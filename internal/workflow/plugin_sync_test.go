package workflow

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/workflow/sandbox"
)

func syncPluginSpec(t *testing.T) *Spec {
	t.Helper()
	return &Spec{
		Trigger: Trigger{Type: TriggerAPI, API: &APITrigger{Method: "POST", Path: "/plugin", Auth: Auth{Type: AuthNone}}},
		Steps: []Step{
			pluginStep(t, `{"id":"notify","type":"plugin:notify","config":{"url":{"kind":"const","const":"https://team.hooks.example.com/x"}}}`),
			{ID: "reply", Type: StepResponse, Config: &ResponseConfig{Status: 200, Body: &Body{Type: BodyValue, Value: vp("steps.notify.status")}}},
		},
	}
}

func TestCheckSyncActionPlugin(t *testing.T) {
	for _, tt := range []struct {
		name    string
		source  string
		enabled bool
		code    string
	}{
		{name: "default flows", source: notifyPlugin, enabled: true},
		{name: "explicit sync", source: strings.Replace(notifyPlugin, "action: true,", `action: true, flows: ["sync"],`, 1), enabled: true},
		{name: "async only", source: strings.Replace(notifyPlugin, "action: true,", `action: true, flows: ["async", "background"],`, 1), enabled: true, code: "step.async_only"},
		{name: "disabled", source: notifyPlugin, code: "step.plugin_disabled"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			installTestPlugins(t, map[string]bool{tt.source: tt.enabled})
			issues, err := Check(context.Background(), nil, &db.Project{ID: 1}, syncPluginSpec(t))
			require.NoError(t, err)
			if tt.code == "" {
				assert.Empty(t, issues)
			} else {
				require.Len(t, issues, 1)
				assert.Equal(t, tt.code, issues[0].Code)
			}

			modules := ModulesIn("en")
			def := modules[len(modules)-1]
			assert.Equal(t, "plugin:notify", def.Type)
			assert.Equal(t, tt.code != "step.async_only", contains(def.Flows, "sync"))
			assert.Equal(t, tt.enabled, def.Plugin.Enabled)
		})
	}
}

func TestServeSyncActionPluginWaits(t *testing.T) {
	f := newLeadFixture(t)
	installTestPlugins(t, map[string]bool{notifyPlugin: true})
	f.api = f.publish(t, "Sync plugin", syncPluginSpec(t))
	started, release := make(chan struct{}), make(chan struct{})
	fetcher := &fakePluginFetcher{fetch: func(ctx context.Context, _ sandbox.FetchRequest) (*sandbox.FetchResponse, error) {
		close(started)
		select {
		case <-release:
			return &sandbox.FetchResponse{Status: 202, Body: "{}"}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}}
	f.engine.SetActions(fetcher)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req := httptest.NewRequest(http.MethodPost, "/api/"+f.project.UID+"/plugin", nil).WithContext(ctx)
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		w := httptest.NewRecorder()
		f.engine.Serve(w, req, f.project.UID, "/plugin", "127.0.0.1")
		done <- w
	}()

	select {
	case <-started:
	case w := <-done:
		t.Fatalf("request returned before fetching: %s", w.Body.String())
	case <-ctx.Done():
		t.Fatal("plugin did not start")
	}
	select {
	case <-done:
		t.Fatal("request returned before the external operation completed")
	default:
	}
	close(release)

	select {
	case w := <-done:
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.JSONEq(t, `202`, w.Body.String())
	case <-ctx.Done():
		t.Fatal("request did not finish after the external operation completed")
	}
	assert.Len(t, fetcher.requests, 1)
	assert.Equal(t, int64(0), f.count(t, &db.WorkflowJob{}, "workflow_id = ?", f.api.ID))
	var run db.WorkflowRun
	require.NoError(t, f.db.Where("workflow_id = ?", f.api.ID).First(&run).Error)
	assert.Equal(t, db.WorkflowRunSucceeded, run.Status)
	var trace []TraceStep
	require.NoError(t, json.Unmarshal(run.Trace, &trace))
	require.Len(t, trace, 2)
	assert.Equal(t, TraceSucceeded, trace[0].Status)
	assert.JSONEq(t, `{"status":202}`, string(trace[0].Output))
}

func TestRunSyncActionPluginFailure(t *testing.T) {
	for _, failure := range []string{"external error", "timeout", "later write error"} {
		t.Run(failure, func(t *testing.T) {
			f := newLeadFixture(t)
			source := strings.Replace(notifyPlugin, "action: true,", "action: true, timeout: 1,", 1)
			installTestPlugins(t, map[string]bool{source: true})
			spec := syncPluginSpec(t)
			create := Step{ID: "lead", Type: StepCreate, Config: &WriteConfig{TableUID: f.table.UID, Fields: []FieldValue{
				{FieldUID: f.name.UID, Value: Value{Kind: ValueConst, Const: json.RawMessage(`"Ann"`)}},
			}}}
			spec.Steps = append([]Step{create}, spec.Steps...)
			fetcher := &fakePluginFetcher{fetch: func(ctx context.Context, _ sandbox.FetchRequest) (*sandbox.FetchResponse, error) {
				switch failure {
				case "external error":
					return nil, &ActionError{Message: "temporarily unavailable", Transient: true}
				case "timeout":
					<-ctx.Done()
					return nil, ctx.Err()
				default:
					return &sandbox.FetchResponse{Status: 202}, nil
				}
			}}
			f.engine.SetActions(fetcher)
			status, code := http.StatusBadGateway, ErrCodeAction
			if failure == "later write error" {
				invalid := Step{ID: "invalid", Type: StepCreate, Config: &WriteConfig{TableUID: f.table.UID, Fields: []FieldValue{
					{FieldUID: f.source.UID, Value: Value{Kind: ValueConst, Const: json.RawMessage(`"Unknown"`)}},
				}}}
				spec.Steps = append(spec.Steps[:2], invalid, spec.Steps[2])
				status, code = http.StatusUnprocessableEntity, ErrCodeInvalidFieldValue
			}
			spec.Async = []AsyncFlow{{ID: "succeeded", Event: EventSucceeded, Steps: []Step{spec.Steps[1]}}}

			result, err := f.engine.Run(context.Background(), RunInput{Project: f.project, Workflow: f.api, Spec: spec})
			require.NoError(t, err)
			assert.Equal(t, db.WorkflowRunFailed, result.Status)
			assert.Equal(t, status, result.Response.Status)
			assert.Equal(t, code, result.Response.Body.(map[string]interface{})["error"].(map[string]interface{})["code"])
			assert.Len(t, fetcher.requests, 1, "synchronous external operations are never retried")
			assert.Empty(t, result.Jobs)
			assert.Equal(t, int64(0), f.count(t, &db.SLRecord{}, "1 = 1"))
			assert.Equal(t, int64(0), f.count(t, &db.SLChangeset{}, "1 = 1"))
			assert.Equal(t, int64(0), f.count(t, &db.WorkflowJob{}, "1 = 1"))
			assert.Equal(t, int64(1), f.count(t, &db.WorkflowRun{}, "status = ?", db.WorkflowRunFailed))
			if failure == "timeout" {
				assert.Contains(t, result.Trace[1].Error, "timed out after 1 seconds")
			}
		})
	}
}

func TestRunSyncActionPluginDryRun(t *testing.T) {
	for _, preview := range []bool{false, true} {
		t.Run(map[bool]string{false: "config only", true: "preview hook"}[preview], func(t *testing.T) {
			f := newLeadFixture(t)
			source, output := notifyPlugin, `{"url":"https://team.hooks.example.com/x"}`
			if preview {
				source += `export function preview() { return { status: 299 }; }`
				output = `{"status":299}`
			}
			installTestPlugins(t, map[string]bool{source: true})
			fetcher := &fakePluginFetcher{}
			f.engine.SetActions(fetcher)
			spec := syncPluginSpec(t)
			spec.Steps = append([]Step{f.spec.Steps[2]}, spec.Steps...)
			if !preview {
				spec.Steps[2].Config = &ResponseConfig{Status: 200}
			}
			spec.Trigger.API.Params = f.spec.Params()
			result, err := f.engine.Run(context.Background(), RunInput{
				Project: f.project, Workflow: f.api, Spec: spec, Trigger: db.WorkflowRunTriggerDebug, DryRun: true,
				Raw: RawInput{Body: map[string]json.RawMessage{"name": json.RawMessage(`"Ann"`), "email": json.RawMessage(`"ann@example.com"`)}},
			})
			require.NoError(t, err)
			require.Equal(t, db.WorkflowRunSucceeded, result.Status, "%+v", result.Trace)
			assert.Empty(t, fetcher.requests)
			assert.Equal(t, TracePlanned, result.Trace[1].Status)
			assert.JSONEq(t, output, string(result.Trace[1].Output))
			if preview {
				assert.Equal(t, float64(299), result.Response.Body)
			}
			assert.Equal(t, int64(0), f.count(t, &db.SLRecord{}, "1 = 1"))
			assert.Equal(t, int64(0), f.count(t, &db.WorkflowRun{}, "1 = 1"))
			assert.Equal(t, int64(0), f.count(t, &db.WorkflowJob{}, "1 = 1"))
		})
	}
}
