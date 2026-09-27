package collab

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/thanhpk/randstr"

	"github.com/wuhan005/sayrud/internal/db"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = 25 * time.Second
	maxMessageSize = 8 << 20
	sendBufferSize = 256
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	// The management API allows all origins as well, see context.Contexter.
	CheckOrigin: func(*http.Request) bool { return true },
}

// Identity is the anonymous collaborator identity provided by the browser.
type Identity struct {
	MemberID string
	Name     string
	Color    string
}

// Client is a WebSocket connection of a project.
type Client struct {
	hub        *Hub
	conn       *websocket.Conn
	project    *db.Project
	projectUID string
	out        chan []byte
	closeOnce  sync.Once

	mu     sync.Mutex
	member Member
	tables map[string]*db.SLTable
}

// Serve upgrades the request to WebSocket and serves the connection until it is closed.
func (h *Hub) Serve(w http.ResponseWriter, r *http.Request, project *db.Project, identity Identity) error {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return errors.Wrap(err, "upgrade")
	}

	c := &Client{
		hub:        h,
		conn:       conn,
		project:    project,
		projectUID: project.UID,
		out:        make(chan []byte, sendBufferSize),
		member: Member{
			ClientID: "cli" + randstr.String(12),
			MemberID: identity.MemberID,
			Name:     identity.Name,
			Color:    identity.Color,
		},
		tables: make(map[string]*db.SLTable),
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
			c.send(newMessage(MessageError, 0, errorData{Message: "消息格式错误"}))
			continue
		}
		c.handle(ctx, message)
	}
}

func (c *Client) handle(ctx context.Context, message Message) {
	replyError := func(msg string) {
		c.send(newMessage(MessageError, message.ReqID, errorData{Message: msg}))
	}

	switch message.Type {
	case MessagePing:
		c.send(newMessage(MessagePong, message.ReqID, nil))

	case MessageSubscribe:
		var data subscribeData
		if err := json.Unmarshal(message.Data, &data); err != nil {
			replyError("消息格式错误")
			return
		}
		table, err := db.SLTables.GetByUID(ctx, data.TableUID)
		if err != nil || table.ProjectID != c.project.ID {
			replyError("数据表不存在")
			return
		}
		rev, err := c.hub.subscribe(ctx, c, table)
		if err != nil {
			logrus.WithContext(ctx).WithError(err).Error("Failed to subscribe table")
			replyError("服务器内部错误")
			return
		}
		c.mu.Lock()
		c.tables[table.UID] = table
		c.mu.Unlock()
		c.send(newMessage(MessageSubscribed, message.ReqID, subscribedData{TableUID: table.UID, Rev: rev}))

	case MessageUnsubscribe:
		var data subscribeData
		if err := json.Unmarshal(message.Data, &data); err != nil {
			replyError("消息格式错误")
			return
		}
		c.mu.Lock()
		delete(c.tables, data.TableUID)
		c.mu.Unlock()

	case MessageUserChanges:
		var data userChangesData
		if err := json.Unmarshal(message.Data, &data); err != nil {
			replyError("消息格式错误")
			return
		}
		c.commit(ctx, message.ReqID, data)

	case MessagePresence:
		var data presenceData
		if err := json.Unmarshal(message.Data, &data); err != nil {
			replyError("消息格式错误")
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
		replyError("不支持的消息类型")
	}
}

func (c *Client) commit(ctx context.Context, reqID int64, data userChangesData) {
	reject := func(msg string) {
		c.send(newMessage(MessageRejectCommit, reqID, rejectCommitData{TableUID: data.TableUID, Signature: data.Signature, Message: msg}))
	}

	table := c.subscribedTable(data.TableUID)
	if table == nil {
		reject("请先订阅数据表")
		return
	}
	if data.Signature == "" {
		reject("缺少提交签名")
		return
	}
	for _, operation := range data.Operations {
		for _, action := range operation.Actions {
			if action.Action == ActionDirty {
				reject("不支持的操作：" + ActionDirty)
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
			reject(operationErr.Message)
			return
		}
		logrus.WithContext(ctx).WithError(err).WithField("tableUID", table.UID).Error("Failed to commit changeset")
		reject("服务器内部错误")
	}
}
