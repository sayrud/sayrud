package collab

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"

	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dbutil"
)

type sharedTables struct {
	db.SLTablesStore
	mu     sync.Mutex
	tables map[int64]db.SLTable
}

func (s *sharedTables) GetByID(_ context.Context, id int64) (*db.SLTable, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	table, ok := s.tables[id]
	if !ok || table.DeletedAt.Valid {
		return nil, db.ErrSLTableNotFound
	}
	return &table, nil
}

func (s *sharedTables) GetByUID(_ context.Context, uid string) (*db.SLTable, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, table := range s.tables {
		if table.UID == uid && !table.DeletedAt.Valid {
			return &table, nil
		}
	}
	return nil, db.ErrSLTableNotFound
}

func (s *sharedTables) update(id int64, change func(*db.SLTable)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	table := s.tables[id]
	change(&table)
	s.tables[id] = table
}

type sharedProjects struct{ db.ProjectsStore }

func (sharedProjects) GetByID(_ context.Context, id int64) (*db.Project, error) {
	return &db.Project{Model: dbutil.Model{ID: id}, UID: "project"}, nil
}

func shareFixture(t *testing.T) (*Hub, *sharedTables, *db.Project, *ShareSession) {
	t.Helper()
	oldTables, oldProjects := db.SLTables, db.Projects
	t.Cleanup(func() { db.SLTables, db.Projects = oldTables, oldProjects })
	tables := &sharedTables{tables: map[int64]db.SLTable{
		1: {Model: dbutil.Model{ID: 1}, UID: "root", ProjectID: 1, ShareToken: "token", ShareEnabled: true, Rev: 4},
		2: {Model: dbutil.Model{ID: 2}, UID: "child", ProjectID: 1, Rev: 8},
		3: {Model: dbutil.Model{ID: 3}, UID: "outside", ProjectID: 2},
	}}
	db.SLTables, db.Projects = tables, sharedProjects{}
	return NewHub(nil), tables, &db.Project{Model: dbutil.Model{ID: 1}, UID: "project"}, &ShareSession{TableID: 1, Token: "token"}
}

func dialShared(t *testing.T, hub *Hub, project *db.Project, session *ShareSession) *websocket.Conn {
	t.Helper()
	done := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		done <- hub.Serve(w, r, project, Identity{MemberID: "guest", Name: "Visitor", Role: db.ProjectRoleEditor, Share: session}, nil)
	}))
	t.Cleanup(server.Close)
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = conn.Close()
		select {
		case err := <-done:
			require.NoError(t, err)
		case <-time.After(5 * time.Second):
			t.Error("Shared WebSocket did not close")
		}
	})
	return conn
}

func readShared(t *testing.T, conn *websocket.Conn, kind string, reqID int64) Message {
	t.Helper()
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	for {
		var message Message
		require.NoError(t, conn.ReadJSON(&message))
		require.NotEqual(t, MessageShortcutJobs, message.Type, "Public visitors received private job data")
		if kind == MessagePong || kind == MessageShareChanged {
			require.NotEqual(t, MessageNewChanges, message.Type, "A revoked or out-of-scope notification was delivered")
		}
		if message.Type == kind && message.ReqID == reqID {
			return message
		}
	}
}

func TestSharedWebSocketLiveChangesPresenceAndWriteDenial(t *testing.T) {
	hub, _, project, session := shareFixture(t)
	conn := dialShared(t, hub, project, session)
	readShared(t, conn, MessageHello, 0)

	require.NoError(t, conn.WriteJSON(Message{Type: MessageSubscribe, ReqID: 1, Data: json.RawMessage(`{"tableUID":"root"}`)}))
	readShared(t, conn, MessageSubscribed, 1)
	for i, uid := range []string{"child", "outside"} {
		reqID := int64(i + 2)
		require.NoError(t, conn.WriteJSON(Message{Type: MessageSubscribe, ReqID: reqID, Data: json.RawMessage(`{"tableUID":"` + uid + `"}`)}))
		readShared(t, conn, MessageError, reqID)
	}

	require.NoError(t, conn.WriteJSON(Message{Type: MessageUserChanges, ReqID: 4, Data: json.RawMessage(`{"tableUID":"root","signature":"attempt","operations":[]}`)}))
	readShared(t, conn, MessageRejectCommit, 4)

	rootMember := &Client{hub: hub, projectUID: project.UID, out: make(chan []byte, 8), member: Member{ClientID: "editor", TableUID: "root", RecordUID: "record", FieldUID: "field", Name: "Editor"}}
	privateMember := &Client{hub: hub, projectUID: project.UID, out: make(chan []byte, 8), member: Member{ClientID: "private", TableUID: "child", Name: "Private"}}
	hub.register(rootMember)
	hub.register(privateMember)
	var members membersData
	for {
		message := readShared(t, conn, MessageMembers, 0)
		require.NoError(t, json.Unmarshal(message.Data, &members))
		if len(members.Members) == 2 {
			break
		}
	}
	var editor Member
	for _, member := range members.Members {
		if member.ClientID == "editor" {
			editor = member
		}
	}
	require.Equal(t, "record", editor.RecordUID)
	require.NotContains(t, string(newMessage(MessageMembers, 0, members)), "Private")

	var changeset Changeset
	require.NoError(t, json.Unmarshal([]byte(`{"tableUID":"root","rev":5,"signature":"signature","clientId":"editor","operations":[{"command":"secret-shortcut-inputs","actions":[{"action":"field.add","fieldUID":"newField","field":{"label":"Visible field","type":"text","metadata":{"optionsReference":"private","options":["visible"]},"shortcut":{"id":"private"}}},{"action":"field.setShortcut","fieldUID":"field","shortcut":{"id":"private"}},{"action":"record.set","recordUID":"record","values":{"field":"live value","files":[{"uid":"file","url":"private-url"}]}},{"action":"view.set","viewUID":"view","view":{"name":"Visible view"}},{"action":"table.dirty","dirty":{"records":["record"]}}]}]}`), &changeset))
	hub.broadcastChangeset(project, &changeset, nil)
	message := readShared(t, conn, MessageNewChanges, 0)
	var shared Changeset
	require.NoError(t, json.Unmarshal(message.Data, &shared))
	require.Equal(t, int64(5), shared.Rev)
	require.Equal(t, "signature", shared.Signature)
	require.Len(t, shared.Operations, 1)
	require.Len(t, shared.Operations[0].Actions, 4)
	require.Equal(t, "live value", shared.Operations[0].Actions[1].Values["field"])
	require.Equal(t, "/_/shares/token/tables/root/attachments/file", shared.Operations[0].Actions[1].Values["files"].([]interface{})[0].(map[string]interface{})["url"])
	require.NotContains(t, string(message.Data), "private")
	require.NotContains(t, string(message.Data), "secret-shortcut-inputs")
	require.Equal(t, "private", changeset.Operations[0].Actions[0].Field.Shortcut.ID)
	require.Equal(t, "private-url", changeset.Operations[0].Actions[2].Values["files"].([]interface{})[0].(map[string]interface{})["url"])

	hub.broadcastChangeset(project, &Changeset{TableUID: "root", Rev: 6, Operations: []Operation{{Actions: []Action{{Action: ActionSetFieldShortcut, Shortcut: &db.FieldShortcut{ID: "private"}}}}}}, nil)
	message = readShared(t, conn, MessageNewChanges, 0)
	require.NoError(t, json.Unmarshal(message.Data, &shared))
	require.Equal(t, int64(6), shared.Rev)
	require.NotNil(t, shared.Operations)
	require.Empty(t, shared.Operations)
	hub.NotifyTable(project.UID, "root", MessageShortcutJobs, func(func(string, ...interface{}) string) interface{} { return "private-job-config" })
	require.NoError(t, conn.WriteJSON(Message{Type: MessagePing, ReqID: 5}))
	readShared(t, conn, MessagePong, 5)
}

func TestSharedWebSocketScopeAndRevocationApplyToOpenConnections(t *testing.T) {
	for _, change := range []string{"disabled", "new token", "new password", "expired grant", "deleted table"} {
		t.Run(change, func(t *testing.T) {
			hub, tables, project, session := shareFixture(t)
			if change == "expired grant" {
				session.ExpiresAt = time.Now().Add(150 * time.Millisecond)
			}
			conn := dialShared(t, hub, project, session)
			readShared(t, conn, MessageHello, 0)
			require.NoError(t, conn.WriteJSON(Message{Type: MessageSubscribe, ReqID: 1, Data: json.RawMessage(`{"tableUID":"root"}`)}))
			readShared(t, conn, MessageSubscribed, 1)

			tables.update(1, func(table *db.SLTable) {
				switch change {
				case "disabled":
					table.ShareEnabled = false
				case "new token":
					table.ShareToken = "rotated"
				case "new password":
					table.SharePasswordHash = "new-hash"
				case "deleted table":
					table.DeletedAt.Valid = true
				}
			})
			if change == "expired grant" {
				time.Sleep(time.Until(session.ExpiresAt) + time.Millisecond)
			}

			hub.broadcastChangeset(project, &Changeset{TableUID: "root", Rev: 5}, nil)
			readShared(t, conn, MessageShareChanged, 0)
			_, _, err := conn.ReadMessage()
			require.Error(t, err)
		})
	}

	t.Run("scope narrowed", func(t *testing.T) {
		hub, tables, project, session := shareFixture(t)
		tables.update(1, func(table *db.SLTable) { table.ShareIncludeChildren = true })
		conn := dialShared(t, hub, project, session)
		readShared(t, conn, MessageHello, 0)
		require.NoError(t, conn.WriteJSON(Message{Type: MessageSubscribe, ReqID: 1, Data: json.RawMessage(`{"tableUID":"child"}`)}))
		readShared(t, conn, MessageSubscribed, 1)
		tables.update(1, func(table *db.SLTable) { table.ShareIncludeChildren = false })
		hub.broadcastChangeset(project, &Changeset{TableUID: "child", Rev: 9}, nil)
		require.NoError(t, conn.WriteJSON(Message{Type: MessagePing, ReqID: 2}))
		readShared(t, conn, MessagePong, 2)
		require.NoError(t, conn.WriteJSON(Message{Type: MessageSubscribe, ReqID: 3, Data: json.RawMessage(`{"tableUID":"child"}`)}))
		readShared(t, conn, MessageError, 3)
		hub.NotifyShareChanged(project.UID, 1)
		readShared(t, conn, MessageShareChanged, 0)
	})
}
