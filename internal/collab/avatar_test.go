package collab

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"

	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dbutil"
)

func TestUpdateUserAvatarRefreshesAllConnectionsAndIgnoresOlderUpdate(t *testing.T) {
	hub := NewHub(nil)
	first := &Client{userID: 7, projectUID: "first", out: make(chan []byte, 5)}
	second := &Client{userID: 7, projectUID: "second", out: make(chan []byte, 5)}
	other := &Client{userID: 8, projectUID: "first", out: make(chan []byte, 5)}
	hub.projects["first"] = map[*Client]struct{}{first: {}, other: {}}
	hub.projects["second"] = map[*Client]struct{}{second: {}}

	latest := time.Now()
	hub.UpdateUserAvatar(7, "/_/avatars/new", latest)
	require.Equal(t, "/_/avatars/new", first.presence().AvatarURL)
	require.Equal(t, "/_/avatars/new", second.presence().AvatarURL)
	require.Empty(t, other.presence().AvatarURL)
	require.Len(t, first.out, 1)
	require.Len(t, second.out, 1)
	require.Len(t, other.out, 1)

	hub.UpdateUserAvatar(7, "/_/avatars/old", latest.Add(-time.Second))
	require.Equal(t, "/_/avatars/new", first.presence().AvatarURL)
	require.Len(t, first.out, 1)

	hub.UpdateUserAvatar(7, "", latest.Add(time.Second))
	require.Empty(t, first.presence().AvatarURL)
	require.Empty(t, second.presence().AvatarURL)
	require.Len(t, first.out, 2)
}

type avatarRefreshUsers struct {
	db.UsersStore
	user         *db.User
	err          error
	beforeReturn func()
}

func (s avatarRefreshUsers) GetByID(context.Context, int64) (*db.User, error) {
	if s.beforeReturn != nil {
		s.beforeReturn()
	}

	return s.user, s.err
}

func TestServeRefreshesAvatarAfterRegistration(t *testing.T) {
	for _, name := range []string{"changed before registration", "removed before registration", "changed during readback", "readback failure"} {
		t.Run(name, func(t *testing.T) {
			previousUsers := db.Users
			t.Cleanup(func() { db.Users = previousUsers })

			hub := NewHub(nil)
			latest := time.Now()
			initial := latest.Add(-time.Second)
			user := &db.User{Model: dbutil.Model{ID: 7, UpdatedAt: latest}, AvatarFileUID: "new"}
			store := avatarRefreshUsers{user: user}
			want, count := "/_/avatars/new", 3
			switch name {
			case "removed before registration":
				user.AvatarFileUID = ""
				want = ""
			case "changed during readback":
				user.AvatarFileUID, user.UpdatedAt = "old", initial
				store.beforeReturn = func() { hub.UpdateUserAvatar(7, want, latest) }
			case "readback failure":
				store.err = db.ErrUserNotFound
				want, count = "/_/avatars/old", 2
			}
			db.Users = store

			// The upload notification happened while the connection still held its authentication snapshot.
			hub.UpdateUserAvatar(7, want, latest)

			done := make(chan error, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				done <- hub.Serve(w, r, &db.Project{UID: "project"}, Identity{
					UserID: 7, MemberID: "usr7", AvatarURL: "/_/avatars/old", UpdatedAt: initial,
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

			require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
			var last membersData
			for i := 0; i < count; i++ {
				var message Message
				require.NoError(t, conn.ReadJSON(&message))
				if message.Type == MessageMembers {
					require.NoError(t, json.Unmarshal(message.Data, &last))
				}
			}

			require.Len(t, last.Members, 1)
			require.Equal(t, want, last.Members[0].AvatarURL)
		})
	}
}
