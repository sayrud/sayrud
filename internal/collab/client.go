package collab

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"github.com/sirupsen/logrus"
	"github.com/thanhpk/randstr"

	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/i18n"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = 25 * time.Second
	maxMessageSize = 8 << 20
	sendBufferSize = 256
)

// upgrader uses the default same-origin check of gorilla, so other sites can not connect with the session cookie of the user.
var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
}

// Identity is the signed-in collaborator of a connection.
type Identity struct {
	UserID   int64
	MemberID string
	Name     string
	Color    string
	CanEdit  bool
}

// Client is a WebSocket connection of a project.
type Client struct {
	hub        *Hub
	conn       *websocket.Conn
	project    *db.Project
	projectUID string
	userID     int64
	locale     i18n.Locale
	out        chan []byte
	closeOnce  sync.Once

	mu      sync.Mutex
	member  Member
	canEdit bool
	tables  map[string]*db.SLTable
}

// Serve upgrades the request to WebSocket and serves the connection until it is closed.
func (h *Hub) Serve(w http.ResponseWriter, r *http.Request, project *db.Project, identity Identity, locale i18n.Locale) error {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return errors.Wrap(err, "upgrade")
	}

	c := &Client{
		hub:        h,
		conn:       conn,
		project:    project,
		projectUID: project.UID,
		userID:     identity.UserID,
		locale:     locale,
		out:        make(chan []byte, sendBufferSize),
		member: Member{
			ClientID: "cli" + randstr.String(12),
			MemberID: identity.MemberID,
			Name:     identity.Name,
			Color:    identity.Color,
		},
		canEdit: identity.CanEdit,
		tables:  make(map[string]*db.SLTable),
	}

	h.register(c)
	go c.writePump()
	c.readPump(r.Context())
	h.unregister(c)
	return nil
}

func (c *Client) close() {
	c.closeOnce.Do(func() {
		close(c.out)
	})
}

// send queues the message, the slow client whose buffer is full is disconnected, and it will catch up after reconnecting.
func (c *Client) send(message []byte) {
	defer func() {
		// The channel is closed by a concurrent disconnection.
		_ = recover()
	}()
	select {
	case c.out <- message:
	default:
		c.close()
	}
}

func (c *Client) presence() Member {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.member
}

func (c *Client) subscribe(tableUID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.tables[tableUID]; !ok {
		c.tables[tableUID] = nil
	}
}

func (c *Client) subscribed(tableUID string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.tables[tableUID]
	return ok
}

func (c *Client) subscribedTable(tableUID string) *db.SLTable {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.tables[tableUID]
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.out:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) readPump(ctx context.Context) {
	defer func() {
		c.close()
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))

		var message Message
		if err := json.Unmarshal(raw, &message); err != nil {
			c.send(newMessage(MessageError, 0, errorData{Message: c.tr("collab::invalid_message")}))
			continue
		}
		c.handle(ctx, message)
	}
}

func (c *Client) handle(ctx context.Context, message Message) {
	replyError := func(key string) {
		c.send(newMessage(MessageError, message.ReqID, errorData{Message: c.tr(key)}))
	}

	switch message.Type {
	case MessagePing:
		c.send(newMessage(MessagePong, message.ReqID, nil))

	case MessageSubscribe:
		var data subscribeData
		if err := json.Unmarshal(message.Data, &data); err != nil {
			replyError("collab::invalid_message")
			return
		}
		table, err := db.SLTables.GetByUID(ctx, data.TableUID)
		if err != nil || table.ProjectID != c.project.ID {
			replyError("collab::table_not_found")
			return
		}
		rev, err := c.hub.subscribe(ctx, c, table)
		if err != nil {
			logrus.WithContext(ctx).WithError(err).Error("Failed to subscribe table")
			replyError("common::internal_error")
			return
		}
		c.mu.Lock()
		c.tables[table.UID] = table
		c.mu.Unlock()
		c.send(newMessage(MessageSubscribed, message.ReqID, subscribedData{TableUID: table.UID, Rev: rev}))

	case MessageUnsubscribe:
		var data subscribeData
		if err := json.Unmarshal(message.Data, &data); err != nil {
			replyError("collab::invalid_message")
			return
		}
		c.mu.Lock()
		delete(c.tables, data.TableUID)
		c.mu.Unlock()

	case MessageUserChanges:
		var data userChangesData
		if err := json.Unmarshal(message.Data, &data); err != nil {
			replyError("collab::invalid_message")
			return
		}
		c.commit(ctx, message.ReqID, data)

	case MessagePresence:
		var data presenceData
		if err := json.Unmarshal(message.Data, &data); err != nil {
			replyError("collab::invalid_message")
			return
		}
		c.mu.Lock()
		c.member.TableUID = data.TableUID
		c.member.ViewUID = data.ViewUID
		c.member.RecordUID = data.RecordUID
		c.member.FieldUID = data.FieldUID
		c.mu.Unlock()
		c.hub.broadcastMembers(c.projectUID)

	default:
		replyError("collab::unsupported_message")
	}
}

func (c *Client) commit(ctx context.Context, reqID int64, data userChangesData) {
	reject := func(key string, args ...interface{}) {
		c.send(newMessage(MessageRejectCommit, reqID, rejectCommitData{TableUID: data.TableUID, Signature: data.Signature, Message: c.tr(key, args...)}))
	}

	c.mu.Lock()
	canEdit := c.canEdit
	c.mu.Unlock()
	if !canEdit {
		reject("collab::no_edit_permission")
		return
	}

	table := c.subscribedTable(data.TableUID)
	if table == nil {
		reject("collab::subscribe_first")
		return
	}
	if data.Signature == "" {
		reject("collab::signature_required")
		return
	}
	for _, operation := range data.Operations {
		for _, action := range operation.Actions {
			if action.Action == ActionDirty {
				reject("collab::unsupported_action", ActionDirty)
				return
			}
		}
	}

	err := c.hub.Commit(ctx, c.project, table, c, data.Signature, data.Operations, func(result CommitResult) {
		c.send(newMessage(MessageAcceptCommit, reqID, acceptCommitData{TableUID: table.UID, Rev: result.Rev, Signature: data.Signature}))
	})
	if err != nil {
		var operationErr *OperationError
		if errors.As(err, &operationErr) {
			reject(operationErr.Key, operationErr.Args...)
			return
		}
		logrus.WithContext(ctx).WithError(err).WithField("tableUID", table.UID).Error("Failed to commit changeset")
		reject("common::internal_error")
	}
}

// tr translates the message key in the language of the connection.
func (c *Client) tr(key string, args ...interface{}) string {
	if c.locale == nil {
		return key
	}
	return c.locale.Translate(key, args...)
}

// boolText returns the text of the checkbox value in the language of the connection, or Simplified Chinese without a sender.
func (c *Client) boolText(v bool) string {
	if c == nil {
		return lo.Ternary(v, "是", "否")
	}
	return c.tr(lo.Ternary(v, "common::yes", "common::no"))
}
