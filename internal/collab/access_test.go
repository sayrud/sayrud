package collab

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dbutil"
)

// Keep the real stores and error mapping, replacing only database query execution.
func accessTestDB(t *testing.T, query func(*gorm.DB)) *gorm.DB {
	t.Helper()
	gormDB, err := gorm.Open(postgres.New(postgres.Config{DSN: "host=127.0.0.1 port=1 sslmode=disable"}), &gorm.Config{
		DisableAutomaticPing: true,
		Logger:               logger.Discard,
	})
	require.NoError(t, err)
	sqlDB, err := gormDB.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, gormDB.Callback().Query().Replace("gorm:query", func(tx *gorm.DB) {
		switch item := tx.Statement.Dest.(type) {
		case *db.UserSession:
			item.UserID = 7
		case *db.User:
			item.ID = 7
		case *db.Project:
			item.ID, item.OwnerUserID = 1, 8
		case *db.ProjectMember:
			item.Role = db.ProjectRoleEditor
		}
		query(tx)
	}))
	return gormDB
}

func TestRefreshAccessErrors(t *testing.T) {
	for _, tc := range []struct {
		name       string
		table      string
		err        error
		disabled   bool
		mismatch   bool
		permission bool
	}{
		{name: "missing session", table: "user_sessions", err: gorm.ErrRecordNotFound, permission: true},
		{name: "missing user", table: "users", err: gorm.ErrRecordNotFound, permission: true},
		{name: "disabled user", disabled: true, permission: true},
		{name: "session belongs to another user", mismatch: true, permission: true},
		{name: "missing project", table: "projects", err: gorm.ErrRecordNotFound, permission: true},
		{name: "missing membership", table: "project_members", err: gorm.ErrRecordNotFound, permission: true},
		{name: "session timeout", table: "user_sessions", err: context.DeadlineExceeded},
		{name: "user query canceled", table: "users", err: context.Canceled},
		{name: "project connection lost", table: "projects", err: io.ErrUnexpectedEOF},
		{name: "membership connection lost", table: "project_members", err: io.ErrUnexpectedEOF},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gormDB := accessTestDB(t, func(tx *gorm.DB) {
				if tx.Statement.Table == tc.table {
					tx.AddError(tc.err)
				}
				if user, ok := tx.Statement.Dest.(*db.User); ok && tc.disabled {
					now := time.Now()
					user.DisabledAt = &now
				}
				if session, ok := tx.Statement.Dest.(*db.UserSession); ok && tc.mismatch {
					session.UserID = 8
				}
			})
			c := &Client{
				hub: NewHub(gormDB), project: &db.Project{Model: dbutil.Model{ID: 1}}, projectUID: "project",
				userID: 7, sessionToken: "token", role: db.ProjectRoleEditor, out: make(chan []byte, 2),
			}
			require.False(t, c.refreshAccess(context.Background()))
			require.Equal(t, db.ProjectRoleEditor, c.role, "failed queries must not change the cached role")
			if tc.permission {
				message, ok := <-c.out
				require.True(t, ok)
				var envelope Message
				require.NoError(t, json.Unmarshal(message, &envelope))
				require.Equal(t, MessagePermissionChanged, envelope.Type)
				var data permissionChangedData
				require.NoError(t, json.Unmarshal(envelope.Data, &data))
				require.Equal(t, "project", data.ProjectUID)
				require.Empty(t, data.Role)
			} else {
				require.Equal(t, websocket.CloseTryAgainLater, c.closeStatus)
			}
			_, ok := <-c.out
			require.False(t, ok, "transient errors must close without a permission notification")
		})
	}
}

func TestAccessFailureClosesWebSocket(t *testing.T) {
	for _, tc := range []struct {
		name       string
		err        error
		permission bool
		heartbeat  bool
	}{
		{name: "inbound transient error", err: io.ErrUnexpectedEOF},
		{name: "inbound revocation", err: gorm.ErrRecordNotFound, permission: true},
		{name: "heartbeat transient error", err: context.DeadlineExceeded, heartbeat: true},
		{name: "heartbeat revocation", err: gorm.ErrRecordNotFound, permission: true, heartbeat: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gormDB := accessTestDB(t, func(tx *gorm.DB) { tx.AddError(tc.err) })
			hub := NewHub(gormDB)
			done := make(chan error, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				done <- hub.Serve(w, r, &db.Project{Model: dbutil.Model{ID: 1}, UID: "project"}, Identity{
					UserID: 7, MemberID: "usr7", Role: db.ProjectRoleEditor, SessionToken: "token",
				}, nil)
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
					t.Error("WebSocket did not stop")
				}
			})
			readClusterMessage(t, conn, MessageHello)
			if !tc.heartbeat {
				require.NoError(t, conn.WriteJSON(Message{Type: MessagePing, ReqID: 1}))
			}
			require.NoError(t, conn.SetReadDeadline(time.Now().Add(pingPeriod+5*time.Second)))
			permission := false
			for {
				var message Message
				err := conn.ReadJSON(&message)
				if err != nil {
					code := websocket.CloseTryAgainLater
					if tc.permission {
						code = websocket.CloseNormalClosure
					}
					require.True(t, websocket.IsCloseError(err, code), "%v", err)
					break
				}
				if message.Type == MessagePermissionChanged {
					permission = true
				}
			}
			require.Equal(t, tc.permission, permission)
		})
	}
}

func TestCommitAccessErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{name: "revoked session", err: gorm.ErrRecordNotFound},
		{name: "database timeout", err: context.DeadlineExceeded},
		{name: "connection lost", err: io.ErrUnexpectedEOF},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gormDB := clusterTestDB(t)
			hub := NewHub(gormDB)
			t.Cleanup(func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				require.NoError(t, hub.WaitBackground(ctx))
				hub.CancelBackground()
			})
			project, table, user := testProject(t, gormDB, hub)
			before, err := db.NewSLTablesStore(gormDB).GetByID(context.Background(), table.ID)
			require.NoError(t, err)
			require.NoError(t, gormDB.Callback().Query().Before("gorm:query").Register("test:access_error", func(tx *gorm.DB) {
				if tx.Statement.Table == "user_sessions" {
					tx.AddError(tc.err)
				}
			}))
			sender := &Client{userID: user.ID, sessionToken: "token", project: project}
			err = hub.Commit(context.Background(), project, table, sender, "", nil, func(CommitResult) {
				t.Error("failed access check accepted the commit")
			})
			if tc.err == gorm.ErrRecordNotFound {
				var rejected *OperationError
				require.ErrorAs(t, err, &rejected)
				require.Equal(t, "collab::no_edit_permission", rejected.Key)
			} else {
				require.ErrorIs(t, err, tc.err)
			}
			after, err := db.NewSLTablesStore(gormDB).GetByID(context.Background(), table.ID)
			require.NoError(t, err)
			require.Equal(t, before.Rev, after.Rev, "failed access checks must not commit a revision")
		})
	}
}
