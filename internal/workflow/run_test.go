package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thanhpk/randstr"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/wuhan005/sayrud/internal/collab"
	"github.com/wuhan005/sayrud/internal/db"
)

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	v, err := time.Parse(time.RFC3339, s)
	require.NoError(t, err)
	return v
}

func newTestDB(t *testing.T) *gorm.DB {
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
	require.NoError(t, gormDB.AutoMigrate(
		&db.Setting{}, &db.Project{}, &db.SLTable{}, &db.SLField{}, &db.SLRecord{}, &db.SLChangeset{}, &db.SLShortcutJob{},
		&db.Workflow{}, &db.WorkflowRevision{}, &db.WorkflowRun{}, &db.WorkflowJob{}, &db.WorkflowKey{}, &db.Credential{},
	))
	db.SetDatabaseStore(gormDB)
	return gormDB
}

type fakeActions struct {
	mu      sync.Mutex
	mails   []Mail
	servers []SMTPServer
	fail    int
}

func (f *fakeActions) SendEmail(_ context.Context, server SMTPServer, m Mail) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail > 0 {
		f.fail--
		return &ActionError{Message: "temporarily unavailable", Transient: true}
	}
	f.mails = append(f.mails, m)
	f.servers = append(f.servers, server)
	return nil
}

func (f *fakeActions) Fetch(context.Context, HTTPRequest) (*HTTPResult, error) {
	return &HTTPResult{Status: 200}, nil
}

func (f *fakeActions) sent() []Mail {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Mail{}, f.mails...)
}

type leadFixture struct {
	db      *gorm.DB
	engine  *Engine
	actions *fakeActions
	hub     *collab.Hub
	project *db.Project
	table   *db.SLTable
	name    *db.SLField
	email   *db.SLField
	source  *db.SLField
	api     *db.Workflow
	spec    *Spec
	smtp    *db.Credential
}

func vv(path string) Value  { return Value{Kind: ValueVar, Var: path} }
func vp(path string) *Value { v := vv(path); return &v }

func newLeadFixture(t *testing.T) *leadFixture {
	ctx := context.Background()
	gormDB := newTestDB(t)
	project, err := db.NewProjectsStore(gormDB).Create(ctx, db.CreateProjectOptions{OwnerUserID: 1, Name: "CRM"})
	require.NoError(t, err)
	table, err := db.NewSLTablesStore(gormDB).Create(ctx, project.ID, db.CreateSLTableOptions{Name: "Leads"})
	require.NoError(t, err)
	fields := db.NewSLFieldsStore(gormDB)
	name, err := fields.Create(ctx, db.CreateSLFieldOptions{SLTableID: table.ID, Label: "Name", Type: db.TextFieldType, Metadata: map[string]interface{}{}})
	require.NoError(t, err)
	email, err := fields.Create(ctx, db.CreateSLFieldOptions{SLTableID: table.ID, Label: "Email", Type: db.TextFieldType, Metadata: map[string]interface{}{}})
	require.NoError(t, err)
	source, err := fields.Create(ctx, db.CreateSLFieldOptions{SLTableID: table.ID, Label: "Source", Type: db.SingleSelectFieldType, Metadata: map[string]interface{}{
		"options": []interface{}{map[string]interface{}{"uid": "optWeb", "name": "Web", "color": 0}, map[string]interface{}{"uid": "optAds", "name": "Ads", "color": 1}},
	}})
	require.NoError(t, err)
	smtp, err := db.NewCredentialsStore(gormDB).Create(ctx, db.CreateCredentialOptions{
		ProjectID: project.ID, Type: "smtp", Name: "Mailer",
		Data: []byte(`{"host":"smtp.example.com","port":465,"security":"tls","fromAddress":"bot@example.com"}`),
	})
	require.NoError(t, err)

	spec := &Spec{
		Trigger: Trigger{Type: TriggerAPI, API: &APITrigger{
			Method: "POST", Path: "/aaaa", Auth: Auth{Type: AuthNone},
			Params: []Param{
				{ID: "p1", Key: "name", Source: SourceBody, Type: TypeString, Required: true},
				{ID: "p2", Key: "email", Source: SourceBody, Type: TypeString, Required: true, Format: FormatEmail},
				{ID: "p3", Key: "source", Source: SourceBody, Type: TypeString},
			},
		}},
		Steps: []Step{
			{ID: "find", Type: StepQuery, Config: &QueryConfig{TableUID: table.UID, Limit: 1, Filters: []Filter{{FieldUID: email.UID, Op: "eq", Value: vp("input.email")}}}},
			{ID: "exists", Type: StepCondition, Config: &ConditionConfig{Conditions: []Condition{{Left: vv("steps.find.found"), Op: "eq", Right: &Value{Kind: ValueConst, Const: json.RawMessage("true")}}}},
				Then: []Step{{ID: "existing", Type: StepResponse, Config: &ResponseConfig{Status: 200, Body: &Body{Type: BodyObject, Fields: []BodyField{
					{Key: "id", Value: Body{Type: BodyValue, Value: vp("steps.find.first.uid")}},
					{Key: "existed", Value: Body{Type: BodyValue, Value: &Value{Kind: ValueConst, Const: json.RawMessage("true")}}},
				}}}}},
			},
			{ID: "lead", Type: StepCreate, Config: &WriteConfig{TableUID: table.UID, Fields: []FieldValue{
				{FieldUID: name.UID, Value: vv("input.name")},
				{FieldUID: email.UID, Value: vv("input.email")},
				{FieldUID: source.UID, Value: vv("input.source")},
			}}},
			{ID: "created", Type: StepResponse, Config: &ResponseConfig{Status: 201, Body: &Body{Type: BodyObject, Fields: []BodyField{
				{Key: "id", Value: Body{Type: BodyValue, Value: vp("steps.lead.record.uid")}},
				{Key: "source", Value: Body{Type: BodyValue, Value: vp("steps.lead.record.fields." + source.UID)}},
			}}}},
		},
		Async: []AsyncFlow{
			{ID: "audit", Event: EventRequestReceived, Steps: []Step{{ID: "ping", Type: StepHTTP, Config: &HTTPConfig{URL: &Value{Kind: ValueConst, Const: json.RawMessage(`"https://example.com/hook"`)}}}}},
			{ID: "notify", Event: EventSucceeded, Steps: []Step{{ID: "mail", Type: StepEmail, Config: &EmailConfig{
				Credential: smtp.UID, To: vp("input.email"),
				Subject: &Value{Kind: ValueTemplate, Template: "Hi {{input.name}}"}, Content: &Value{Kind: ValueTemplate, Template: "Lead {{steps.lead.record.uid}}"},
			}}}},
		},
	}
	issues, err := Check(ctx, gormDB, project, spec)
	require.NoError(t, err)
	require.Empty(t, issues)

	hub := collab.NewHub(gormDB)
	engine := NewEngine(gormDB, hub, nil, 1)
	hub.SetWorkflowHooks(engine)
	actions := &fakeActions{}
	engine.SetActions(actions)
	f := &leadFixture{db: gormDB, engine: engine, hub: hub, actions: actions, project: project, table: table, name: name, email: email, source: source, spec: spec, smtp: smtp}
	f.api = f.publish(t, "Leads", spec)
	return f
}

// publish creates and publishes a workflow the same way the publish API does.
func (f *leadFixture) publish(t *testing.T, name string, spec *Spec) *db.Workflow {
	t.Helper()
	ctx := context.Background()
	raw, _ := json.Marshal(spec)
	wf, err := db.NewWorkflowsStore(f.db).Create(ctx, db.CreateWorkflowOptions{ProjectID: f.project.ID, Name: name, Draft: raw})
	require.NoError(t, err)
	opts, err := PublishOptions(ctx, f.db, spec, 1, 0, time.Now())
	require.NoError(t, err)
	_, err = db.NewWorkflowsStore(f.db).Publish(ctx, wf.ID, opts)
	require.NoError(t, err)
	wf, err = db.NewWorkflowsStore(f.db).GetByID(ctx, wf.ID)
	require.NoError(t, err)
	return wf
}

func TestRollback(t *testing.T) {
	f := newLeadFixture(t)
	ctx := context.Background()
	workflows := db.NewWorkflowsStore(f.db)
	revisions := db.NewWorkflowRevisionsStore(f.db)

	// Change the path in the draft and publish it as v2.
	v2 := *f.spec
	api := *v2.Trigger.API
	api.Path = "/bbbb"
	v2.Trigger.API = &api
	raw, _ := json.Marshal(&v2)
	draftVersion, err := workflows.UpdateDraft(ctx, f.api.ID, f.api.DraftVersion, db.UpdateWorkflowDraftOptions{Name: f.api.Name, Draft: raw})
	require.NoError(t, err)
	opts, err := PublishOptions(ctx, f.db, &v2, draftVersion, 0, time.Now())
	require.NoError(t, err)
	_, err = workflows.Publish(ctx, f.api.ID, opts)
	require.NoError(t, err)

	// Roll back to v1, which publishes v3 and overwrites the draft with v1.
	v1, err := revisions.GetByVersion(ctx, f.api.ID, 1)
	require.NoError(t, err)
	spec, err := ParseSpec(v1.Spec)
	require.NoError(t, err)
	opts, err = PublishOptions(ctx, f.db, spec, draftVersion, 0, time.Now())
	require.NoError(t, err)
	opts.RestoredFrom = v1.Version
	v3, err := workflows.Publish(ctx, f.api.ID, opts)
	require.NoError(t, err)
	assert.Equal(t, 3, v3.Version)
	assert.Equal(t, 1, v3.RestoredFrom)

	wf, err := workflows.GetByID(ctx, f.api.ID)
	require.NoError(t, err)
	assert.Equal(t, 3, wf.PublishedVersion)
	assert.Equal(t, "/aaaa", wf.Path)
	assert.Equal(t, draftVersion+1, wf.DraftVersion)
	assert.Equal(t, wf.DraftVersion, wf.PublishedDraftVersion)
	draft, err := ParseSpec(wf.Draft)
	require.NoError(t, err)
	assert.Equal(t, "/aaaa", draft.Trigger.API.Path)

	_, err = revisions.GetByVersion(ctx, f.api.ID, 9)
	assert.ErrorIs(t, err, db.ErrWorkflowRevisionNotFound)
}

func (f *leadFixture) call(t *testing.T, body string, header map[string]string) (*httptest.ResponseRecorder, map[string]interface{}) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/"+f.project.UID+"/aaaa", bytes.NewBufferString(body))
	for k, v := range header {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	f.engine.Serve(w, req, f.project.UID, "/aaaa", "127.0.0.1")
	var out map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w, out
}

func (f *leadFixture) count(t *testing.T, model interface{}, where string, args ...interface{}) int64 {
	t.Helper()
	var n int64
	require.NoError(t, f.db.Model(model).Where(where, args...).Count(&n).Error)
	return n
}

func TestRunQueryOrCreate(t *testing.T) {
	f := newLeadFixture(t)

	w, out := f.call(t, `{"name":"Ann","email":"ann@example.com","source":"Web"}`, nil)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.Equal(t, "Web", out["source"])
	id := out["id"].(string)
	assert.NotEmpty(t, w.Header().Get("X-Request-Id"))

	var record db.SLRecord
	require.NoError(t, f.db.Where("uid = ?", id).First(&record).Error)
	data := map[string]interface{}{}
	require.NoError(t, json.Unmarshal(record.Data, &data))
	assert.Equal(t, "optWeb", data[f.source.UID], "option names are stored as option UIDs")
	assert.Equal(t, int64(1), f.count(t, &db.SLChangeset{}, "sl_table_id = ?", f.table.ID), "the write appends a changeset for the collaborators")
	assert.Equal(t, int64(1), f.count(t, &db.WorkflowJob{}, "event = ?", EventSucceeded))
	assert.Equal(t, int64(1), f.count(t, &db.WorkflowRun{}, "status = ?", db.WorkflowRunSucceeded))

	w, out = f.call(t, `{"name":"Ann","email":"ann@example.com"}`, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, map[string]interface{}{"id": id, "existed": true}, out)
	assert.Equal(t, int64(1), f.count(t, &db.SLRecord{}, "sl_table_id = ?", f.table.ID))
}

func TestRunFailureRollsBack(t *testing.T) {
	f := newLeadFixture(t)

	// Parameter validation fails: no business logic runs and no succeeded event is created, but the request received event is saved.
	w, out := f.call(t, `{"name":"Ann","email":"not-an-email"}`, nil)
	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "invalid_params", out["error"].(map[string]interface{})["code"])
	assert.Equal(t, int64(1), f.count(t, &db.WorkflowRun{}, "status = ?", db.WorkflowRunRejected))
	assert.Equal(t, int64(1), f.count(t, &db.WorkflowJob{}, "event = ?", EventRequestReceived))
	assert.Equal(t, int64(0), f.count(t, &db.WorkflowJob{}, "event = ?", EventSucceeded))

	// Write fails: the record and the succeeded event are rolled back with the transaction.
	w, out = f.call(t, `{"name":"Ann","email":"ann@example.com","source":"Unknown"}`, nil)
	require.Equal(t, http.StatusUnprocessableEntity, w.Code, w.Body.String())
	assert.Equal(t, ErrCodeInvalidFieldValue, out["error"].(map[string]interface{})["code"])
	assert.Equal(t, int64(0), f.count(t, &db.SLRecord{}, "sl_table_id = ?", f.table.ID))
	assert.Equal(t, int64(0), f.count(t, &db.WorkflowJob{}, "event = ?", EventSucceeded))
	assert.Equal(t, int64(0), f.count(t, &db.SLChangeset{}, "sl_table_id = ?", f.table.ID))
	assert.Equal(t, int64(1), f.count(t, &db.WorkflowRun{}, "status = ?", db.WorkflowRunFailed))
}

func TestRunDryRunWritesNothing(t *testing.T) {
	f := newLeadFixture(t)
	result, err := f.engine.Run(context.Background(), RunInput{
		Project: f.project, Workflow: f.api, Spec: f.spec, Trigger: db.WorkflowRunTriggerDebug, DryRun: true,
		Raw: RawInput{Body: map[string]json.RawMessage{"name": json.RawMessage(`"Ann"`), "email": json.RawMessage(`"ann@example.com"`)}},
	})
	require.NoError(t, err)
	assert.Equal(t, db.WorkflowRunSucceeded, result.Status)
	assert.Equal(t, 201, result.Response.Status)
	require.Len(t, result.Jobs, 2)
	assert.False(t, result.Jobs[0].Queued)
	assert.Equal(t, int64(0), f.count(t, &db.SLRecord{}, "1 = 1"))
	assert.Equal(t, int64(0), f.count(t, &db.WorkflowRun{}, "1 = 1"))
	assert.Equal(t, int64(0), f.count(t, &db.WorkflowJob{}, "1 = 1"))
}

func TestWorkerRetriesAndSucceeds(t *testing.T) {
	f := newLeadFixture(t)
	f.actions.fail = 1
	jobRetryDelays = []time.Duration{0, 0}

	w, _ := f.call(t, `{"name":"Ann","email":"ann@example.com"}`, nil)
	require.Equal(t, http.StatusCreated, w.Code)

	ctx, cancel := context.WithCancel(context.Background())
	f.engine.Start(ctx)
	defer func() {
		cancel()
		_ = f.engine.Shutdown(context.Background(), context.Background())
	}()

	require.Eventually(t, func() bool {
		return f.count(t, &db.WorkflowJob{}, "status = ?", db.WorkflowJobSucceeded) == 2
	}, 15*time.Second, 100*time.Millisecond)
	mails := f.actions.sent()
	require.Len(t, mails, 1)
	assert.Equal(t, []string{"ann@example.com"}, mails[0].To)
	assert.Equal(t, "Hi Ann", mails[0].Subject)
	assert.True(t, strings.HasPrefix(mails[0].Content, "Lead rec"))
	assert.Equal(t, "smtp.example.com", f.actions.servers[0].Host, "the email is sent with the referenced credential")
	assert.Equal(t, "bot@example.com", f.actions.servers[0].FromAddress)

	var job db.WorkflowJob
	require.NoError(t, f.db.Where("flow_id = ?", "notify").First(&job).Error)
	assert.Equal(t, 2, job.Attempts)
}

func TestFindReference(t *testing.T) {
	f := newLeadFixture(t)
	api, err := db.NewWorkflowsStore(f.db).FindReference(context.Background(), f.project.ID, f.source.UID)
	require.NoError(t, err)
	assert.Equal(t, f.api.ID, api.ID)
	_, err = db.NewWorkflowsStore(f.db).FindReference(context.Background(), f.project.ID, "fldMissing")
	assert.ErrorIs(t, err, db.ErrWorkflowNotFound)
}

func TestCheckReportsIssues(t *testing.T) {
	f := newLeadFixture(t)
	spec := *f.spec
	api := *f.spec.Trigger.API
	spec.Trigger.API = &api
	spec.Steps = []Step{
		{ID: "lead", Type: StepCreate, Config: &WriteConfig{TableUID: f.table.UID, Fields: []FieldValue{{FieldUID: f.email.UID, Value: vv("input.missing")}}}},
		{ID: "mail", Type: StepEmail, Config: &EmailConfig{Credential: "crdMissing", To: vp("input.email")}},
	}
	api.Params = append(append([]Param{}, api.Params...), Param{ID: "p9", Key: "name", Source: SourceBody, Type: TypeString})
	issues, err := Check(context.Background(), f.db, f.project, &spec)
	require.NoError(t, err)
	got := map[string]bool{}
	for _, i := range issues {
		got[i.Code] = true
	}
	for _, code := range []string{"value.unknown_var", "step.async_only", "flow.missing_response", "param.duplicate_key", "step.credential_missing", "step.required"} {
		assert.True(t, got[code], code)
	}
}
