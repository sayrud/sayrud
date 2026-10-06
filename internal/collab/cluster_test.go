package collab

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/thanhpk/randstr"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/wuhan005/sayrud/internal/db"
)

func clusterTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	if os.Getenv("PGHOST") == "" {
		t.Skip("PGHOST is not set")
	}
	admin, err := gorm.Open(postgres.New(postgres.Config{DSN: "", PreferSimpleProtocol: true}), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	schema := "test_" + strings.ToLower(randstr.Hex(12))
	require.NoError(t, admin.Exec("CREATE SCHEMA "+schema).Error)
	t.Cleanup(func() {
		_ = admin.Exec("DROP SCHEMA " + schema + " CASCADE").Error
		sqlDB, _ := admin.DB()
		_ = sqlDB.Close()
	})
	gormDB, err := gorm.Open(postgres.New(postgres.Config{DSN: "search_path=" + schema, PreferSimpleProtocol: true}), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	t.Cleanup(func() {
		sqlDB, _ := gormDB.DB()
		_ = sqlDB.Close()
	})
	require.NoError(t, gormDB.AutoMigrate(&db.User{}, &db.UserSession{}, &db.Project{}, &db.ProjectMember{}, &db.SLTable{}, &db.SLField{},
		&db.SLRecord{}, &db.SLView{}, &db.SLChangeset{}, &db.SLShortcutJob{}, &db.File{}))
	return gormDB
}

func testProject(t *testing.T, gormDB *gorm.DB, hub *Hub) (*db.Project, *db.SLTable, *db.User) {
	t.Helper()
	user := &db.User{Email: "owner@example.test", UserName: "Owner"}
	require.NoError(t, gormDB.Create(user).Error)
	project := &db.Project{UID: "prj" + randstr.Hex(12), OwnerUserID: user.ID, Name: "Test"}
	require.NoError(t, gormDB.Create(project).Error)
	table, err := db.NewSLTablesStore(gormDB).Create(context.Background(), project.ID, db.CreateSLTableOptions{Name: "Test"})
	require.NoError(t, err)
	labelA, labelB, kind := "A", "B", "text"
	require.NoError(t, hub.Commit(context.Background(), project, table, nil, "setup", []Operation{{Actions: []Action{
		{Action: ActionAddField, FieldUID: "fldAAAAAAA", Field: &FieldAttrs{Label: &labelA, Type: &kind}},
		{Action: ActionAddField, FieldUID: "fldBBBBBBB", Field: &FieldAttrs{Label: &labelB, Type: &kind}},
		{Action: ActionAddRecord, RecordUID: "recAAAAAAAAAAA", Values: map[string]interface{}{}},
	}}}, func(CommitResult) {}))
	return project, table, user
}

func readClusterMessage(t *testing.T, conn *websocket.Conn, kind string) Message {
	t.Helper()
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	for {
		var message Message
		require.NoError(t, conn.ReadJSON(&message))
		if message.Type == kind {
			return message
		}
	}
}

func connectCluster(t *testing.T, hub *Hub, project *db.Project, table *db.SLTable, user *db.User) *websocket.Conn {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = hub.Serve(w, r, project, Identity{UserID: user.ID, MemberID: fmt.Sprint(user.ID), CanEdit: true}, nil)
	}))
	t.Cleanup(server.Close)
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	readClusterMessage(t, conn, MessageHello)
	require.NoError(t, conn.WriteJSON(Message{Type: MessageSubscribe, ReqID: 1, Data: json.RawMessage(fmt.Sprintf("{\"tableUID\":%q}", table.UID))}))
	readClusterMessage(t, conn, MessageSubscribed)
	return conn
}

func TestConcurrentHubsPreserveCellsAndDeduplicate(t *testing.T) {
	gormDB := clusterTestDB(t)
	first, second := NewHub(gormDB), NewHub(gormDB)
	project, table, _ := testProject(t, gormDB, first)
	ctx := context.Background()
	for iteration := range 20 {
		start := make(chan struct{})
		errs := make(chan error, 2)
		for i, hub := range []*Hub{first, second} {
			go func() {
				<-start
				field := []string{"fldAAAAAAA", "fldBBBBBBB"}[i]
				errs <- hub.Commit(ctx, project, table, nil, fmt.Sprintf("%d-%d", iteration, i), []Operation{{Actions: []Action{{
					Action: ActionSetRecord, RecordUID: "recAAAAAAAAAAA", Values: map[string]interface{}{field: fmt.Sprint(iteration)},
				}}}}, func(CommitResult) {})
			}()
		}
		close(start)
		require.NoError(t, <-errs)
		require.NoError(t, <-errs)
		record, err := db.NewSLRecordsStore(gormDB).GetByUID(ctx, "recAAAAAAAAAAA")
		require.NoError(t, err)
		values := map[string]interface{}{}
		require.NoError(t, json.Unmarshal(record.Data, &values))
		require.Equal(t, fmt.Sprint(iteration), values["fldAAAAAAA"])
		require.Equal(t, fmt.Sprint(iteration), values["fldBBBBBBB"])
	}
	var wg sync.WaitGroup
	results, errs := make(chan CommitResult, 2), make(chan error, 2)
	for _, hub := range []*Hub{first, second} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- hub.Commit(ctx, project, table, nil, "same-signature", []Operation{}, func(result CommitResult) { results <- result })
		}()
	}
	wg.Wait()
	require.NoError(t, <-errs)
	require.NoError(t, <-errs)
	a, b := <-results, <-results
	require.Equal(t, a.Rev, b.Rev)
	require.NotEqual(t, a.Duplicate, b.Duplicate)
	for _, hub := range []*Hub{first, second} {
		require.NoError(t, hub.WaitBackground(ctx))
		hub.CancelBackground()
	}
}

func TestClusterBroadcastPresenceRevocationAndDrain(t *testing.T) {
	address := os.Getenv("REDIS_TEST_ADDRESS")
	if address == "" {
		t.Skip("REDIS_TEST_ADDRESS is not set")
	}
	gormDB := clusterTestDB(t)
	rdb := redis.NewClient(&redis.Options{Addr: address})
	t.Cleanup(func() { _ = rdb.Close() })
	first, second := NewHub(gormDB), NewHub(gormDB)
	project, table, owner := testProject(t, gormDB, first)
	viewer := &db.User{Email: "member@example.test", UserName: "Member"}
	require.NoError(t, gormDB.Create(viewer).Error)
	require.NoError(t, gormDB.Create(&db.ProjectMember{ProjectID: project.ID, UserID: viewer.ID, Role: db.ProjectRoleEditor}).Error)
	ctx := context.Background()
	for _, hub := range []*Hub{first, second} {
		require.NoError(t, hub.StartCluster(ctx, rdb))
		t.Cleanup(func() {
			cleanup, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			_ = hub.Shutdown(cleanup)
			_ = hub.WaitBackground(cleanup)
			hub.CancelBackground()
			hub.StopCluster()
		})
	}
	a := connectCluster(t, first, project, table, owner)
	b := connectCluster(t, second, project, table, viewer)
	for {
		message := readClusterMessage(t, a, MessageMembers)
		var members membersData
		require.NoError(t, json.Unmarshal(message.Data, &members))
		if len(members.Members) == 2 {
			break
		}
	}
	require.NoError(t, first.Commit(ctx, project, table, nil, "remote-edit", []Operation{{Actions: []Action{{
		Action: ActionSetRecord, RecordUID: "recAAAAAAAAAAA", Values: map[string]interface{}{"fldAAAAAAA": "remote"},
	}}}}, func(CommitResult) {}))
	message := readClusterMessage(t, b, MessageNewChanges)
	var changes Changeset
	require.NoError(t, json.Unmarshal(message.Data, &changes))
	require.Equal(t, "remote", changes.Operations[0].Actions[0].Values["fldAAAAAAA"])
	require.NoError(t, gormDB.Model(&db.ProjectMember{}).Where("project_id = ? AND user_id = ?", project.ID, viewer.ID).Update("role", db.ProjectRoleManager).Error)
	first.SetUserRole(project.UID, viewer.ID, db.ProjectRoleManager)
	promotion := readClusterMessage(t, b, MessagePermissionChanged)
	var promoted permissionChangedData
	require.NoError(t, json.Unmarshal(promotion.Data, &promoted))
	require.Equal(t, string(db.ProjectRoleManager), promoted.Role)
	require.NoError(t, gormDB.Model(&db.ProjectMember{}).Where("project_id = ? AND user_id = ?", project.ID, viewer.ID).Update("role", db.ProjectRoleViewer).Error)
	first.SetUserRole(project.UID, viewer.ID, db.ProjectRoleViewer)
	readClusterMessage(t, b, MessagePermissionChanged)
	require.NoError(t, b.WriteJSON(Message{Type: MessageUserChanges, ReqID: 2, Data: json.RawMessage(fmt.Sprintf("{\"tableUID\":%q,\"signature\":\"denied\",\"operations\":[]}", table.UID))}))
	readClusterMessage(t, b, MessageRejectCommit)
	first.BeginDrain()
	require.NoError(t, first.Shutdown(ctx))
	for {
		_, _, err := a.ReadMessage()
		if err != nil {
			require.True(t, websocket.IsCloseError(err, websocket.CloseServiceRestart), "%v", err)
			break
		}
	}
	// The surviving node remains usable and reports durable revisions.
	require.NoError(t, b.WriteJSON(Message{Type: MessagePing, ReqID: 3}))
	pong := readClusterMessage(t, b, MessagePong)
	var data struct{ Revisions map[string]int64 }
	require.NoError(t, json.Unmarshal(pong.Data, &data))
	require.Equal(t, changes.Rev, data.Revisions[table.UID])
	_, response, err := websocket.DefaultDialer.Dial("ws://"+a.RemoteAddr().String(), nil)
	require.Error(t, err)
	require.Equal(t, http.StatusServiceUnavailable, response.StatusCode)
	_ = response.Body.Close()
}

func TestPresenceIgnoresDelayedSnapshotsAndExpires(t *testing.T) {
	hub := NewHub(nil)
	hub.cluster = &cluster{members: make(map[string]map[string]remotePresence)}
	client := &Client{out: make(chan []byte, 10), member: Member{ClientID: "local"}}
	hub.projects["project"] = map[*Client]struct{}{client: {}}
	hub.receiveEvent(clusterEvent{Kind: clusterEventMembers, ProjectUID: "project", Source: "other", Version: 2, Members: []Member{{ClientID: "new"}}})
	hub.receiveEvent(clusterEvent{Kind: clusterEventMembers, ProjectUID: "project", Source: "other", Version: 1, Members: []Member{{ClientID: "old"}}})
	require.Equal(t, "new", hub.cluster.members["project"]["other"].members[0].ClientID)
	presence := hub.cluster.members["project"]["other"]
	presence.expires = time.Now().Add(-time.Second)
	hub.cluster.members["project"]["other"] = presence
	hub.deliverMembers("project")
	require.Empty(t, hub.cluster.members["project"])
}

func TestShareNotificationReachesBothNodesWithoutRepublishing(t *testing.T) {
	address := os.Getenv("REDIS_TEST_ADDRESS")
	if address == "" {
		t.Skip("REDIS_TEST_ADDRESS is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rdb := redis.NewClient(&redis.Options{Addr: address, ContextTimeoutEnabled: true})
	t.Cleanup(func() { _ = rdb.Close() })
	observer := rdb.Subscribe(ctx, clusterChannel)
	t.Cleanup(func() { _ = observer.Close() })
	_, err := observer.ReceiveTimeout(ctx, time.Second)
	require.NoError(t, err)
	messages := observer.Channel()

	projectUID := "prj" + randstr.Hex(12)
	first, second := NewHub(nil), NewHub(nil)
	for _, hub := range []*Hub{first, second} {
		visitor := &Client{share: &ShareSession{TableID: 1}, out: make(chan []byte, 4)}
		member := &Client{out: make(chan []byte, 4)}
		hub.projects[projectUID] = map[*Client]struct{}{visitor: {}, member: {}}
		require.NoError(t, hub.StartCluster(ctx, rdb))
		t.Cleanup(hub.StopCluster)
	}

	first.NotifyShareChanged(projectUID, 1)
	for _, hub := range []*Hub{first, second} {
		for client := range hub.projects[projectUID] {
			if client.share == nil {
				continue
			}

			select {
			case raw := <-client.out:
				var message Message
				require.NoError(t, json.Unmarshal(raw, &message))
				require.Equal(t, MessageShareChanged, message.Type)
			case <-ctx.Done():
				t.Fatal("share notification was not delivered")
			}
		}
	}

	count := 0
	timer := time.NewTimer(200 * time.Millisecond)
	defer timer.Stop()
	for {
		select {
		case message := <-messages:
			var event clusterEvent
			require.NoError(t, json.Unmarshal([]byte(message.Payload), &event))
			if event.Kind == clusterEventShare && event.ProjectUID == projectUID {
				count++
				require.Equal(t, first.cluster.id, event.Source)
			}
		case <-timer.C:
			require.Equal(t, 1, count, "received notifications must not be republished")
			return
		}
	}
}

func TestSubscriptionRecoveryRequiresSocketReauthentication(t *testing.T) {
	if os.Getenv("REDIS_TEST_ADDRESS") == "" {
		t.Skip("REDIS_TEST_ADDRESS is not set")
	}
	gormDB := clusterTestDB(t)
	hub := NewHub(gormDB)
	project, table, user := testProject(t, gormDB, hub)
	rdb := redis.NewClient(&redis.Options{Addr: os.Getenv("REDIS_TEST_ADDRESS"), ContextTimeoutEnabled: true})
	t.Cleanup(func() { _ = rdb.Close() })
	require.NoError(t, hub.StartCluster(context.Background(), rdb))
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = hub.Shutdown(ctx)
		_ = hub.WaitBackground(ctx)
		hub.CancelBackground()
		hub.StopCluster()
	})
	conn := connectCluster(t, hub, project, table, user)
	hub.cluster.mu.Lock()
	require.NoError(t, hub.cluster.sub.Close())
	hub.cluster.mu.Unlock()
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			require.True(t, websocket.IsCloseError(err, websocket.CloseServiceRestart), "%v", err)
			break
		}
	}
	require.Eventually(t, hub.ClusterReady, 5*time.Second, 10*time.Millisecond)
	reconnected := connectCluster(t, hub, project, table, user)
	require.NoError(t, reconnected.WriteJSON(Message{Type: MessagePing, ReqID: 2}))
	readClusterMessage(t, reconnected, MessagePong)
}

func TestRESTMutationRollsBackDataAndRevisionTogether(t *testing.T) {
	gormDB := clusterTestDB(t)
	hub := NewHub(gormDB)
	project, table, _ := testProject(t, gormDB, hub)
	ctx := context.Background()
	before, err := db.NewSLTablesStore(gormDB).GetByID(ctx, table.ID)
	require.NoError(t, err)
	rejected := fmt.Errorf("invalid mutation")
	err = hub.Mutate(ctx, project, table, func(tx *gorm.DB) error {
		record, err := db.NewSLRecordsStore(tx).GetByUID(ctx, "recAAAAAAAAAAA")
		if err != nil {
			return err
		}
		if err := db.NewSLRecordsStore(tx).Update(ctx, record.ID, []byte("{\"fldAAAAAAA\":\"discarded\"}")); err != nil {
			return err
		}
		return rejected
	}, func() DirtyScope { return DirtyScope{AllRecords: true} })
	require.ErrorIs(t, err, rejected)
	after, err := db.NewSLTablesStore(gormDB).GetByID(ctx, table.ID)
	require.NoError(t, err)
	require.Equal(t, before.Rev, after.Rev)
	record, err := db.NewSLRecordsStore(gormDB).GetByUID(ctx, "recAAAAAAAAAAA")
	require.NoError(t, err)
	require.JSONEq(t, "{}", string(record.Data))
	require.NoError(t, hub.WaitBackground(ctx))
	hub.CancelBackground()
}
