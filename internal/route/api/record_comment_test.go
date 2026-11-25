package api

import (
	stdcontext "context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/flamego/flamego"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thanhpk/randstr"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/wuhan005/sayrud/internal/collab"
	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dto"
	"github.com/wuhan005/sayrud/internal/form"
	"github.com/wuhan005/sayrud/internal/i18n"
)

func commentTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	if os.Getenv("PGHOST") == "" {
		t.Skip("PGHOST is not set")
	}
	admin, err := gorm.Open(postgres.New(postgres.Config{DSN: "", PreferSimpleProtocol: true}), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	schema := "test_" + strings.ToLower(randstr.String(10))
	require.NoError(t, admin.Exec("CREATE SCHEMA "+schema).Error)
	t.Cleanup(func() {
		_ = admin.Exec("DROP SCHEMA " + schema + " CASCADE").Error
		if sqlDB, err := admin.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	gormDB, err := gorm.Open(postgres.New(postgres.Config{DSN: "search_path=" + schema, PreferSimpleProtocol: true}), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	t.Cleanup(func() {
		if sqlDB, err := gormDB.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	require.NoError(t, gormDB.AutoMigrate(&db.User{}, &db.Project{}, &db.ProjectMember{}, &db.SLTable{}, &db.SLField{}, &db.SLRecord{},
		&db.SLRecordChange{}, &db.SLRecordComment{}, &db.SLView{}, &db.SLChangeset{}, &db.SLShortcutJob{}, &db.Workflow{}))
	users, projects, members, tables, fields := db.Users, db.Projects, db.ProjectMembers, db.SLTables, db.SLFields
	records, changes, comments, changesets, workflows := db.SLRecords, db.SLRecordChanges, db.SLRecordComments, db.SLChangesets, db.Workflows
	t.Cleanup(func() {
		db.Users, db.Projects, db.ProjectMembers, db.SLTables, db.SLFields = users, projects, members, tables, fields
		db.SLRecords, db.SLRecordChanges, db.SLRecordComments, db.SLChangesets, db.Workflows = records, changes, comments, changesets, workflows
	})
	db.SetDatabaseStore(gormDB)
	return gormDB
}

// commentFixture is a project with a member of each role and a record; requests choose the user by the X-Test-User header.
type commentFixture struct {
	t      *testing.T
	router *flamego.Flame
	users  map[string]*db.User
	db     *gorm.DB
	record *db.SLRecord
	table  *db.SLTable
	field  *db.SLField
}

func newCommentFixture(t *testing.T) *commentFixture {
	ctx := stdcontext.Background()
	gormDB := commentTestDB(t)
	f := &commentFixture{t: t, db: gormDB, users: map[string]*db.User{}}
	roles := map[string]db.ProjectRole{"owner": db.ProjectRoleOwner, "manager": db.ProjectRoleManager, "editor": db.ProjectRoleEditor, "viewer": db.ProjectRoleViewer, "bot": db.ProjectRoleEditor}
	for _, name := range []string{"owner", "manager", "editor", "viewer", "outsider"} {
		user, err := db.Users.Create(ctx, db.CreateUserOptions{Email: name + "@example.com", UserName: name})
		require.NoError(t, err)
		f.users[name] = user
	}
	bot, err := db.Users.CreateBot(ctx, "bot", f.users["owner"].ID)
	require.NoError(t, err)
	f.users["bot"] = bot
	project, err := db.Projects.Create(ctx, db.CreateProjectOptions{OwnerUserID: f.users["owner"].ID, Name: "Tasks"})
	require.NoError(t, err)
	for _, name := range []string{"manager", "editor", "viewer", "bot"} {
		require.NoError(t, db.ProjectMembers.Set(ctx, project.ID, f.users[name].ID, roles[name]))
	}
	f.table, err = db.SLTables.Create(ctx, project.ID, db.CreateSLTableOptions{Name: "Tasks"})
	require.NoError(t, err)
	f.field, err = db.SLFields.Create(ctx, db.CreateSLFieldOptions{SLTableID: f.table.ID, Label: "Title", Type: db.TextFieldType})
	require.NoError(t, err)
	f.record, err = db.SLRecords.Create(ctx, f.table.ID, json.RawMessage(`{}`), db.SLRecordWriter{UserID: f.users["editor"].ID})
	require.NoError(t, err)

	hub := collab.NewHub(gormDB)
	router := flamego.New()
	router.Use(i18n.Middleware(), context.Contexter(gormDB))
	router.Map(hub, project, f.table)
	router.Use(func(c flamego.Context) {
		name := c.Request().Header.Get("X-Test-User")
		c.Map(f.users[name], db.ProjectRole(lo.Ternary(name == "outsider", "", string(roles[name]))))
	})
	router.Post("/records", form.Bind(form.CreateRecord{}), Schemaless.CreateRecord)
	router.Post("/records/batch", form.Bind(form.BatchCreateRecords{}), Schemaless.BatchCreateRecords)
	router.Group("/records/{recordUID}", func() {
		router.Combo("").
			Put(form.Bind(form.UpdateRecord{}), Schemaless.UpdateRecord).
			Delete(Schemaless.DeleteRecord)
		router.Group("/comments", func() {
			router.Combo("").
				Get(Schemaless.ListComments).
				Post(form.Bind(form.CreateRecordComment{}), Schemaless.CreateComment)
			router.Combo("/{commentID}", Schemaless.Commenter).
				Get(Schemaless.GetComment).
				Put(form.Bind(form.UpdateRecordComment{}), Schemaless.UpdateComment).
				Delete(Schemaless.DeleteComment)
		})
		router.Get("/changes", Schemaless.ListChanges)
		router.Get("/changes/{changeID}", Schemaless.GetChange)
		router.Get("/timeline", Schemaless.Timeline)
	}, Schemaless.Recorder)
	f.router = router
	return f
}

// do sends the request as the user and decodes the data of the response into out.
func (f *commentFixture) do(user, method, path string, body interface{}, out interface{}) int {
	f.t.Helper()
	var reader *strings.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		require.NoError(f.t, err)
		reader = strings.NewReader(string(raw))
	} else {
		reader = strings.NewReader("")
	}
	request := httptest.NewRequest(method, "/records/"+f.record.UID+path, reader)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Test-User", user)
	response := httptest.NewRecorder()
	f.router.ServeHTTP(response, request)
	if out != nil && response.Code == http.StatusOK {
		var envelope struct{ Data json.RawMessage }
		require.NoError(f.t, json.Unmarshal(response.Body.Bytes(), &envelope))
		require.NoError(f.t, json.Unmarshal(envelope.Data, out))
	}
	return response.Code
}

func (f *commentFixture) commentCount() int64 {
	record, err := db.SLRecords.GetByID(stdcontext.Background(), f.record.ID)
	require.NoError(f.t, err)
	return record.CommentCount
}

func TestRESTWritesRecordHistory(t *testing.T) {
	f := newCommentFixture(t)
	ctx := stdcontext.Background()
	send := func(method, path string, body interface{}) []byte {
		raw, _ := json.Marshal(body)
		request := httptest.NewRequest(method, path, strings.NewReader(string(raw)))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-Test-User", "bot")
		response := httptest.NewRecorder()
		f.router.ServeHTTP(response, request)
		require.Less(t, response.Code, 300, response.Body.String())
		return response.Body.Bytes()
	}
	kinds := func(recordID int64) []db.SLRecordChangeKind {
		rows, err := db.SLRecordChanges.ListByRecord(ctx, recordID, 0, 100)
		require.NoError(t, err)
		for _, row := range rows {
			assert.Equal(t, db.SLRecordWriter{UserID: f.users["bot"].ID}, row.Writer())
		}
		return lo.Map(lo.Reverse(rows), func(r *db.SLRecordChange, _ int) db.SLRecordChangeKind { return r.Kind })
	}

	var created struct{ Data dto.Record }
	require.NoError(t, json.Unmarshal(send("POST", "/records", map[string]interface{}{"data": map[string]interface{}{f.field.UID: "a"}}), &created))
	record, err := db.SLRecords.GetByUID(ctx, created.Data.UID)
	require.NoError(t, err)
	send("PUT", "/records/"+record.UID, map[string]interface{}{"data": map[string]interface{}{f.field.UID: "b"}})
	assert.Equal(t, []db.SLRecordChangeKind{db.SLRecordChangeCreated, db.SLRecordChangeUpdated}, kinds(record.ID))
	send("DELETE", "/records/"+record.UID, nil)
	assert.Equal(t, []db.SLRecordChangeKind{db.SLRecordChangeCreated, db.SLRecordChangeUpdated, db.SLRecordChangeDeleted}, kinds(record.ID))

	var batch struct{ Data []dto.Record }
	require.NoError(t, json.Unmarshal(send("POST", "/records/batch", map[string]interface{}{"data": []map[string]interface{}{{f.field.UID: "x"}, {f.field.UID: "y"}}}), &batch))
	for _, item := range batch.Data {
		r, err := db.SLRecords.GetByUID(ctx, item.UID)
		require.NoError(t, err)
		assert.Equal(t, []db.SLRecordChangeKind{db.SLRecordChangeCreated}, kinds(r.ID), "a batch creation records only the creation")
	}
}

func TestSharedRecordsHideCommentCount(t *testing.T) {
	records := []*dto.Record{{UID: "rec", TableUID: "tbl", Data: map[string]interface{}{}, CommentCount: 3}}
	prepareSharedRecords(records, "")
	assert.Equal(t, int64(3), records[0].CommentCount)
	prepareSharedRecords(records, "token")
	assert.Zero(t, records[0].CommentCount, "the public shares never show comments")
}

func TestRecordCommentPermissions(t *testing.T) {
	f := newCommentFixture(t)
	mention := func(name string) string { return fmt.Sprintf("<@%d>", f.users[name].ID) }

	var root collab.Comment
	require.Equal(t, 200, f.do("viewer", "POST", "/comments", map[string]interface{}{"body": "viewers comment " + mention("bot")}, &root))
	assert.Equal(t, db.SLRecordWriter{UserID: f.users["viewer"].ID}, root.Author)
	assert.Equal(t, []db.CommentMention{{UserID: f.users["bot"].ID, IsBot: true}}, root.Mentions)
	var botReply collab.Comment
	require.Equal(t, 200, f.do("bot", "POST", "/comments", map[string]interface{}{"body": "on it", "replyToId": root.ID}, &botReply), "bots comment as members")
	assert.Equal(t, root.ID, botReply.RootID)

	assert.Equal(t, 400, f.do("viewer", "POST", "/comments", map[string]interface{}{"body": mention("outsider")}, nil))
	assert.Equal(t, 400, f.do("viewer", "POST", "/comments", map[string]interface{}{"body": "x", "kind": "report"}, nil), "the reserved kinds are rejected")
	assert.Equal(t, 400, f.do("viewer", "POST", "/comments", map[string]interface{}{"body": strings.Repeat("a", collab.MaxCommentLength+1)}, nil))
	assert.Equal(t, 400, f.do("viewer", "POST", "/comments", map[string]interface{}{"body": "   "}, nil))
	assert.Equal(t, 404, f.do("viewer", "POST", "/comments", map[string]interface{}{"body": "x", "replyToId": 99999}, nil))
	assert.Equal(t, int64(2), f.commentCount())

	id := "/comments/" + strconv.FormatInt(root.ID, 10)
	assert.Equal(t, 403, f.do("editor", "PUT", id, map[string]interface{}{"body": "hijack"}, nil), "only the author edits")
	assert.Equal(t, 403, f.do("manager", "PUT", id, map[string]interface{}{"body": "hijack"}, nil), "the managers can not edit others either")
	var edited collab.Comment
	require.Equal(t, 200, f.do("viewer", "PUT", id, map[string]interface{}{"body": "edited " + mention("editor")}, &edited))
	assert.NotNil(t, edited.EditedAt)
	assert.Equal(t, []db.CommentMention{{UserID: f.users["editor"].ID}}, edited.Mentions)

	assert.Equal(t, 403, f.do("editor", "DELETE", id, nil, nil), "an editor can not delete others' comments")
	assert.Equal(t, 204, f.do("manager", "DELETE", id, nil, nil))
	assert.Equal(t, int64(1), f.commentCount(), "the deleted comments are not counted")
	assert.Equal(t, 400, f.do("viewer", "PUT", id, map[string]interface{}{"body": "again"}, nil))

	// The deleted root stays as a placeholder of its reply, and disappears with it.
	var list dto.ListRecordCommentsResp
	require.Equal(t, 200, f.do("viewer", "GET", "/comments", nil, &list))
	require.Len(t, list.Comments, 1)
	assert.True(t, list.Comments[0].Deleted)
	assert.Empty(t, list.Comments[0].Body)
	require.Len(t, list.Comments[0].Replies, 1)
	assert.Equal(t, "on it", list.Comments[0].Replies[0].Body)
	replyID := "/comments/" + strconv.FormatInt(botReply.ID, 10)
	assert.Equal(t, 204, f.do("bot", "DELETE", replyID, nil, nil), "the author deletes its own comment")
	require.Equal(t, 200, f.do("viewer", "GET", "/comments", nil, &list))
	assert.Empty(t, list.Comments)
	assert.Zero(t, f.commentCount())
}

func TestRecordCommentPages(t *testing.T) {
	f := newCommentFixture(t)
	for i := range 5 {
		require.Equal(t, 200, f.do("editor", "POST", "/comments", map[string]interface{}{"body": "c" + strconv.Itoa(i)}, &collab.Comment{}))
	}
	var bodies []string
	cursor := int64(0)
	for {
		var page dto.ListRecordCommentsResp
		require.Equal(t, 200, f.do("viewer", "GET", "/comments?limit=2&cursor="+strconv.FormatInt(cursor, 10), nil, &page))
		bodies = append(bodies, lo.Map(page.Comments, func(c *collab.Comment, _ int) string { return c.Body })...)
		if page.NextCursor == 0 {
			break
		}
		cursor = page.NextCursor
	}
	assert.Equal(t, []string{"c4", "c3", "c2", "c1", "c0"}, bodies)
}

func TestRecordHistoryMerge(t *testing.T) {
	f := newCommentFixture(t)
	ctx := stdcontext.Background()
	now := time.Now()
	minutesAgo := []int{40}
	for i, step := range []struct {
		user, value string
		ago         int
	}{
		{"editor", "a", 30}, {"editor", "b", 27},
		{"editor", "c", 20},
		{"manager", "x", 10}, {"manager", "c", 9},
		{"editor", "y", 2}, {"editor", "z", 1},
	} {
		data, _ := json.Marshal(map[string]string{f.field.UID: step.value})
		require.NoError(t, db.SLRecords.Update(ctx, f.record.ID, data, db.SLRecordWriter{UserID: f.users[step.user].ID}), i)
		minutesAgo = append(minutesAgo, step.ago)
	}
	rows, err := db.SLRecordChanges.ListByRecord(ctx, f.record.ID, 0, 100)
	require.NoError(t, err)
	require.Len(t, rows, len(minutesAgo))
	for i, row := range lo.Reverse(rows) {
		require.NoError(t, f.db.Model(row).Update("created_at", now.Add(-time.Duration(minutesAgo[i])*time.Minute)).Error)
	}

	var entries []*dto.RecordChange
	cursor := int64(0)
	for pages := 0; ; pages++ {
		var page dto.ListRecordChangesResp
		require.Equal(t, 200, f.do("viewer", "GET", "/changes?limit=2&cursor="+strconv.FormatInt(cursor, 10), nil, &page))
		require.LessOrEqual(t, len(page.Changes), 2)
		entries = append(entries, page.Changes...)
		if page.NextCursor == 0 {
			assert.Equal(t, 1, pages, "two pages of two entries")
			break
		}
		cursor = page.NextCursor
	}
	type summary struct{ kind, old, new string }
	got := lo.Map(entries, func(e *dto.RecordChange, _ int) summary {
		if len(e.Fields) == 0 {
			return summary{kind: string(e.Kind)}
		}
		assert.Equal(t, "Title", e.Fields[0].FieldLabel)
		return summary{kind: string(e.Kind), old: string(e.Fields[0].OldValue), new: string(e.Fields[0].NewValue)}
	})
	assert.Equal(t, []summary{
		{"updated", `"c"`, `"z"`},
		// The manager changed the value back, so the entry is dropped.
		{"updated", `"b"`, `"c"`},
		{"updated", "", `"b"`},
		{"created", "", ""},
	}, got)
	assert.Equal(t, db.SLRecordWriter{UserID: f.users["editor"].ID}, *entries[0].Actor)

	var single dto.RecordChange
	require.Equal(t, 200, f.do("viewer", "GET", "/changes/"+strconv.FormatInt(rows[0].ID, 10), nil, &single))
	assert.Equal(t, rows[0].ID, single.ID)
	assert.Equal(t, 404, f.do("viewer", "GET", "/changes/999999", nil, nil))
}

func TestRecordTimeline(t *testing.T) {
	f := newCommentFixture(t)
	ctx := stdcontext.Background()
	for i, value := range []string{"a", "b"} {
		data, _ := json.Marshal(map[string]string{f.field.UID: value})
		require.NoError(t, db.SLRecords.Update(ctx, f.record.ID, data, db.SLRecordWriter{UserID: f.users["editor"].ID}))
		require.Equal(t, 200, f.do("viewer", "POST", "/comments", map[string]interface{}{"body": "comment " + strconv.Itoa(i)}, &collab.Comment{}))
	}
	long := strings.Repeat("长", 600)
	require.Equal(t, 200, f.do("bot", "POST", "/comments", map[string]interface{}{"body": long}, &collab.Comment{}))

	read := func(order string) []*dto.TimelineItem {
		var items []*dto.TimelineItem
		cursor := ""
		for {
			var page dto.RecordTimelineResp
			require.Equal(t, 200, f.do("viewer", "GET", "/timeline?limit=2&order="+order+"&cursor="+cursor, nil, &page))
			items = append(items, page.Items...)
			if page.NextCursor == "" {
				return items
			}
			cursor = page.NextCursor
		}
	}
	desc, asc := read("desc"), read("asc")
	ids := lo.Map(asc, func(item *dto.TimelineItem, _ int) string { return item.ID })
	assert.Len(t, ids, 6, "the creation, two changes and three comments")
	assert.Equal(t, ids, lo.Uniq(ids))
	assert.Equal(t, lo.Reverse(lo.Map(desc, func(item *dto.TimelineItem, _ int) string { return item.ID })), ids)
	assert.Equal(t, dto.TimelineChange, asc[0].Type)
	assert.Equal(t, db.SLRecordChangeCreated, asc[0].Change.Kind)
	assert.Equal(t, "Title: ∅ → a", asc[1].Summary)
	assert.JSONEq(t, `"a"`, string(asc[1].Change.NewValue))
	last := asc[len(asc)-1]
	assert.Equal(t, dto.TimelineComment, last.Type)
	assert.True(t, last.Truncated)
	assert.Len(t, []rune(last.Summary), 500)
	assert.Equal(t, db.SLRecordWriter{UserID: f.users["bot"].ID}, *last.Actor)
	var full collab.Comment
	require.Equal(t, 200, f.do("viewer", "GET", strings.TrimPrefix(last.Ref, "/projects/"+strings.Split(last.Ref, "/")[2]+"/tables/"+f.table.UID+"/records/"+f.record.UID), nil, &full))
	assert.Equal(t, long, full.Body, "the ref reads the full comment")

	assert.Equal(t, 400, f.do("viewer", "GET", "/timeline?cursor=bad!", nil, nil))
}
