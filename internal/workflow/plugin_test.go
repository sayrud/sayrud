package workflow

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/workflow/sandbox"
)

const greetPlugin = `
export const manifest = {
  id: "greet",
  version: "1.2.0",
  name: { en: "Greet", "zh-CN": "问候" },
  description: "Builds a greeting",
  icon: "hand",
  category: "logic",
  outputs: [{ key: "text", label: { en: "Text", "zh-CN": "文本" }, kind: "string" }],
  summary: { parts: ["value:name"] },
  sections: [{
    title: "Greeting",
    fields: [
      { key: "name", widget: "value", label: { en: "Name", "zh-CN": "名字" }, required: true },
      { key: "style", widget: "select", label: "Style", default: "plain", options: [{ value: "plain" }, { value: "loud", label: "Loud" }] },
      { key: "headers", widget: "headers", label: "Headers" },
    ],
  }],
};

export function execute(config, ctx) {
  ctx.log("greeting", config.name);
  const text = "Hello, " + config.name + " from " + ctx.project.name;
  return { text: config.style === "loud" ? text.toUpperCase() : text, headers: config.headers, step: ctx.step.id };
}
`

const notifyPlugin = `
export const manifest = {
  id: "notify",
  version: "1.0.0",
  name: "Notify",
  action: true,
  permissions: { fetch: ["*.hooks.example.com"] },
  outputs: [{ key: "status", label: "Status", kind: "number" }],
  sections: [{ fields: [{ key: "url", widget: "value", label: "URL", required: true }] }],
};

export async function execute(config, ctx) {
  const resp = await ctx.fetch(config.url, { method: "POST", body: { text: "hi" } });
  return { status: resp.status };
}
`

// installTestPlugins replaces the installed plugins of the registry for the test.
func installTestPlugins(t *testing.T, sources map[string]bool) {
	t.Helper()
	r := plugins
	r.syncMu.Lock()
	r.mu.Lock()
	byType, ordered, syncedAt := r.byType, r.ordered, r.syncedAt
	r.byType, r.ordered = map[string]*installedPlugin{}, nil
	// Keeps SyncPlugins from reading the database of other tests.
	r.syncedAt = time.Now().Add(time.Hour)
	r.mu.Unlock()
	r.syncMu.Unlock()
	t.Cleanup(func() {
		r.mu.Lock()
		r.byType, r.ordered, r.syncedAt = byType, ordered, syncedAt
		r.mu.Unlock()
	})

	i := 0
	for source, enabled := range sources {
		program, m, err := InspectPlugin(context.Background(), source, sandbox.JavaScript)
		require.NoError(t, err)
		code := &pluginCode{name: m.ID + ".js", language: sandbox.JavaScript}
		code.once.Do(func() { code.program = program })
		p := newInstalledPlugin(m, PluginChecksum(source), enabled, pluginOrder+i, code)
		r.byType[PluginType(m.ID)] = p
		r.ordered = append(r.ordered, p)
		i++
	}
}

func TestLocalText(t *testing.T) {
	var text LocalText
	require.NoError(t, json.Unmarshal([]byte(`{"en":"Hello","zh-CN":"你好","pt-BR":"Olá"}`), &text))
	assert.Equal(t, "你好", text.In("zh-CN"))
	assert.Equal(t, "你好", text.In("zh-TW"))
	assert.Equal(t, "Olá", text.In("pt"))
	assert.Equal(t, "Hello", text.In("ja"))

	require.NoError(t, json.Unmarshal([]byte(`"Plain"`), &text))
	assert.Equal(t, "Plain", text.In("de"))
	assert.Error(t, json.Unmarshal([]byte(`1`), &text))
}

func TestParsePluginManifest(t *testing.T) {
	m, err := ParsePluginManifest([]byte(`{"id":"echo","version":"1.0.0","name":"Echo","action":true,"permissions":{"fetch":["API.Example.com.","*.hooks.test","*"]},
		"sections":[{"fields":[{"key":"n","widget":"number","default":3}]}]}`))
	require.NoError(t, err)
	assert.Equal(t, []string{"sync", "async", "background"}, m.Flows)
	assert.Equal(t, []string{"*", "api.example.com", "hooks.test"}, m.Permissions.Fetch)
	assert.Equal(t, defaultPluginTimeout, m.Timeout)
	assert.Equal(t, "integration", m.Category)
	assert.Equal(t, "echo", m.IDBase)
	assert.JSONEq(t, `{"n":3}`, string(m.Defaults))

	def := m.ModuleDef("en", 1001, true)
	assert.Equal(t, "plugin:echo", def.Type)
	assert.Equal(t, &ModulePlugin{ID: "echo", Version: "1.0.0", Enabled: true}, def.Plugin)

	// The normalized manifest parses to the same result.
	raw, err := json.Marshal(m)
	require.NoError(t, err)
	again, err := ParsePluginManifest(raw)
	require.NoError(t, err)
	assert.Equal(t, m, again)

	for _, flows := range []string{`["sync"]`, `["async","background"]`} {
		t.Run("explicit flows "+flows, func(t *testing.T) {
			m, err := ParsePluginManifest([]byte(`{"id":"echo","version":"1","name":"Echo","action":true,"flows":` + flows + `}`))
			require.NoError(t, err)
			assert.JSONEq(t, flows, string(marshalJSON(m.Flows)))
		})
	}

	for name, manifest := range map[string]string{
		"bad id":             `{"id":"Echo","version":"1","name":"x"}`,
		"no version":         `{"id":"echo","name":"x"}`,
		"no name":            `{"id":"echo","version":"1"}`,
		"bad category":       `{"id":"echo","version":"1","name":"x","category":"misc"}`,
		"bad flow":           `{"id":"echo","version":"1","name":"x","flows":["web"]}`,
		"fetch without act":  `{"id":"echo","version":"1","name":"x","permissions":{"fetch":["a.com"]}}`,
		"bad host":           `{"id":"echo","version":"1","name":"x","action":true,"permissions":{"fetch":["http://a.com"]}}`,
		"bad widget":         `{"id":"echo","version":"1","name":"x","sections":[{"fields":[{"key":"t","widget":"table"}]}]}`,
		"duplicate key":      `{"id":"echo","version":"1","name":"x","sections":[{"fields":[{"key":"a","widget":"value"},{"key":"a","widget":"value"}]}]}`,
		"select no options":  `{"id":"echo","version":"1","name":"x","sections":[{"fields":[{"key":"a","widget":"select"}]}]}`,
		"own credential":     `{"id":"echo","version":"1","name":"x","sections":[{"fields":[{"key":"c","widget":"credential"}]}]}`,
		"unknown credential": `{"id":"echo","version":"1","name":"x","sections":[{"fields":[{"key":"c","widget":"credential","credentialType":"nope"}]}]}`,
		"bad output kind":    `{"id":"echo","version":"1","name":"x","outputs":[{"key":"r","kind":"record"}]}`,
		"bad summary":        `{"id":"echo","version":"1","name":"x","summary":{"parts":["i18n:x"]}}`,
		"bad showIf":         `{"id":"echo","version":"1","name":"x","sections":[{"showIf":"nope","fields":[]}]}`,
		"bad icon":           `{"id":"echo","version":"1","name":"x","icon":"<svg>"}`,
		"bad color":          `{"id":"echo","version":"1","name":"x","color":"red"}`,
		"bad timeout":        `{"id":"echo","version":"1","name":"x","timeout":120}`,
		"secret summary":     `{"id":"echo","version":"1","name":"x","credential":{"label":"Key","summary":["token"],"sections":[{"fields":[{"key":"token","widget":"password"}]}]}}`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParsePluginManifest([]byte(manifest))
			var manifestErr *ManifestError
			assert.ErrorAs(t, err, &manifestErr)
		})
	}
}

func TestPluginCredential(t *testing.T) {
	m, err := ParsePluginManifest([]byte(`{"id":"chat","version":"1","name":"Chat","action":true,
		"credential":{"label":{"en":"Chat bot","zh-CN":"聊天机器人"},"summary":["workspace"],"sections":[{"fields":[{"key":"workspace","widget":"text","required":true},{"key":"token","widget":"password","required":true}]}]},
		"sections":[{"fields":[{"key":"bot","widget":"credential","required":true},{"key":"mail","widget":"credential","credentialType":"smtp"}]}]}`))
	require.NoError(t, err)
	assert.Equal(t, "plugin:chat", m.Sections[0].Fields[0].CredentialType)

	def := m.CredentialTypeDef("zh-CN")
	require.NotNil(t, def)
	assert.Equal(t, "plugin:chat", def.Type)
	assert.Equal(t, "聊天机器人", def.Label)
	fields := def.Fields()
	require.Len(t, fields, 2)
	assert.False(t, fields[0].Secret)
	assert.Equal(t, FieldDef{Key: "token", Widget: "text", Required: true, Secret: true}, fields[1])
}

func newPluginExecution(actions Actions, preview bool) *execution {
	return &execution{
		ctx: context.Background(), project: &db.Project{UID: "prj1", Name: "Demo"}, tables: map[string]*tableInfo{}, actions: actions, preview: preview,
		scope: Scope{"run": map[string]interface{}{"id": "run1", "timezone": "Asia/Shanghai"}, "steps": map[string]interface{}{}, "input": map[string]interface{}{"name": "Ada"}},
	}
}

func pluginStep(t *testing.T, raw string) Step {
	t.Helper()
	var step Step
	require.NoError(t, json.Unmarshal([]byte(raw), &step))
	return step
}

func TestRunPluginStep(t *testing.T) {
	installTestPlugins(t, map[string]bool{greetPlugin: true})
	step := pluginStep(t, `{"id":"greet1","type":"plugin:greet","config":{
		"name":{"kind":"var","var":"input.name"},
		"headers":[{"name":"X-Id","value":{"kind":"template","template":"{{run.id}}"}}]}}`)

	x := newPluginExecution(nil, false)
	_, err := x.runSteps([]Step{step})
	require.NoError(t, err)
	assert.Equal(t, map[string]interface{}{"text": "Hello, Ada from Demo", "headers": map[string]interface{}{"X-Id": "run1"}, "step": "greet1"}, x.scope["steps"].(map[string]interface{})["greet1"])
	require.Len(t, x.trace, 1)
	assert.Equal(t, TraceSucceeded, x.trace[0].Status)
	assert.Equal(t, []string{"greeting Ada"}, x.trace[0].Logs)

	// A non-action plugin runs for real in simulated debugging.
	step.Config = json.RawMessage(`{"name":{"kind":"const","const":"Bob"},"style":"loud"}`)
	x = newPluginExecution(nil, true)
	_, err = x.runSteps([]Step{step})
	require.NoError(t, err)
	assert.Equal(t, TraceSucceeded, x.trace[0].Status)
	assert.Contains(t, string(x.trace[0].Output), "HELLO, BOB FROM DEMO")
}

func TestRunPluginStepErrors(t *testing.T) {
	failing := `export const manifest = { id: "fail", version: "1", name: "Fail" };
export function execute() { const e = new Error("rate limited"); e.retryable = true; throw e }`
	installTestPlugins(t, map[string]bool{greetPlugin: false, failing: true})

	x := newPluginExecution(nil, false)
	_, err := x.runSteps([]Step{pluginStep(t, `{"id":"greet1","type":"plugin:greet","config":{}}`)})
	var stepErr *StepError
	require.ErrorAs(t, err, &stepErr)
	assert.Equal(t, ErrCodeAction, stepErr.Code)
	assert.Contains(t, x.trace[0].Error, "disabled")

	x = newPluginExecution(nil, false)
	_, err = x.runSteps([]Step{pluginStep(t, `{"id":"fail1","type":"plugin:fail","config":{}}`)})
	require.ErrorAs(t, err, &stepErr)
	assert.True(t, stepErr.Transient)
	assert.Equal(t, "Error: rate limited", x.trace[0].Error)

	x = newPluginExecution(nil, false)
	_, err = x.runSteps([]Step{pluginStep(t, `{"id":"gone1","type":"plugin:gone","config":{}}`)})
	require.ErrorAs(t, err, &stepErr)
	assert.Equal(t, ErrCodeConfig, stepErr.Code)
}

type fakePluginFetcher struct {
	fakeActions
	requests []sandbox.FetchRequest
	fetch    func(context.Context, sandbox.FetchRequest) (*sandbox.FetchResponse, error)
}

func (f *fakePluginFetcher) PluginFetch(ctx context.Context, req sandbox.FetchRequest) (*sandbox.FetchResponse, error) {
	f.requests = append(f.requests, req)
	if f.fetch != nil {
		return f.fetch(ctx, req)
	}
	return &sandbox.FetchResponse{Status: 202, Body: "{}"}, nil
}

func TestRunActionPlugin(t *testing.T) {
	installTestPlugins(t, map[string]bool{notifyPlugin: true})
	step := pluginStep(t, `{"id":"notify1","type":"plugin:notify","config":{"url":{"kind":"const","const":"https://team.hooks.example.com/x"}}}`)

	fetcher := &fakePluginFetcher{}
	x := newPluginExecution(fetcher, false)
	_, err := x.runSteps([]Step{step})
	require.NoError(t, err)
	assert.Equal(t, map[string]interface{}{"status": float64(202)}, x.scope["steps"].(map[string]interface{})["notify1"])
	require.Len(t, fetcher.requests, 1)
	assert.Equal(t, sandbox.FetchRequest{Method: "POST", URL: "https://team.hooks.example.com/x", Body: `{"text":"hi"}`, JSONBody: true}, fetcher.requests[0])

	// Simulated debugging only computes the config of an action plugin without preview.
	x = newPluginExecution(fetcher, true)
	_, err = x.runSteps([]Step{step})
	require.NoError(t, err)
	assert.Equal(t, TracePlanned, x.trace[0].Status)
	assert.JSONEq(t, `{"url":"https://team.hooks.example.com/x"}`, string(x.trace[0].Output))
	assert.Len(t, fetcher.requests, 1)

	step.Config = json.RawMessage(`{"url":{"kind":"const","const":"https://evil.test/x"}}`)
	x = newPluginExecution(fetcher, false)
	_, err = x.runSteps([]Step{step})
	require.Error(t, err)
	assert.Contains(t, x.trace[0].Error, "not allowed by the plugin permissions")
	assert.Len(t, fetcher.requests, 1)
}

func TestRunActionPluginCancellation(t *testing.T) {
	installTestPlugins(t, map[string]bool{notifyPlugin: true})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fetcher := &fakePluginFetcher{fetch: func(fetchCtx context.Context, _ sandbox.FetchRequest) (*sandbox.FetchResponse, error) {
		cancel()
		<-fetchCtx.Done()
		return nil, fetchCtx.Err()
	}}
	x := newPluginExecution(fetcher, false)
	x.ctx = ctx
	_, err := x.runSteps([]Step{pluginStep(t, `{"id":"notify","type":"plugin:notify","config":{"url":{"kind":"const","const":"https://team.hooks.example.com/x"}}}`)})
	var stepErr *StepError
	require.ErrorAs(t, err, &stepErr)
	assert.Equal(t, ErrCodeAction, stepErr.Code)
	assert.True(t, stepErr.Uncertain)
	assert.False(t, stepErr.Transient)
	assert.Len(t, fetcher.requests, 1)
	assert.Equal(t, TraceFailed, x.trace[0].Status)
	assert.Equal(t, "the node was canceled", x.trace[0].Error)
}

func TestCheckPluginSteps(t *testing.T) {
	installTestPlugins(t, map[string]bool{greetPlugin: true, notifyPlugin: false})
	spec := &Spec{
		Trigger: Trigger{Type: TriggerSchedule, Schedule: &ScheduleTrigger{Start: "2026-01-01T09:00", Timezone: "UTC", Repeat: RepeatDaily}},
		Steps: []Step{
			pluginStep(t, `{"id":"greet1","type":"plugin:greet","config":{"name":{"kind":"var","var":"trigger.time"}}}`),
			pluginStep(t, `{"id":"greet2","type":"plugin:greet","config":{"name":{"kind":"var","var":"steps.greet1.text"}}}`),
			pluginStep(t, `{"id":"greet3","type":"plugin:greet","config":{"name":{"kind":"var","var":"steps.greet1.nope"}}}`),
			pluginStep(t, `{"id":"greet4","type":"plugin:greet","config":{}}`),
			pluginStep(t, `{"id":"notify1","type":"plugin:notify","config":{"url":{"kind":"const","const":"https://x.hooks.example.com"}}}`),
			pluginStep(t, `{"id":"gone1","type":"plugin:gone","config":{}}`),
		},
	}
	spec.Normalize()
	issues, err := Check(context.Background(), nil, &db.Project{ID: 1}, spec)
	require.NoError(t, err)
	assert.ElementsMatch(t, []Issue{
		{Code: "value.unknown_var", Target: "step:greet3", Args: map[string]string{"var": "steps.greet1.nope"}},
		{Code: "step.required", Target: "step:greet4", Args: map[string]string{"field": "Name"}},
		{Code: "step.plugin_disabled", Target: "step:notify1", Args: map[string]string{"plugin": "Notify"}},
		{Code: "step.plugin_missing", Target: "step:gone1", Args: map[string]string{"plugin": "gone"}},
	}, issues)

	issues, err = Check(WithLanguage(context.Background(), "zh-CN"), nil, &db.Project{ID: 1}, spec)
	require.NoError(t, err)
	assert.Contains(t, issues, Issue{Code: "step.required", Target: "step:greet4", Args: map[string]string{"field": "名字"}})
}

func TestStarterTemplate(t *testing.T) {
	source, err := os.ReadFile("../../frontend/src/assets/templates/workflow-plugin.js")
	require.NoError(t, err)
	program, m, err := InspectPlugin(context.Background(), string(source), sandbox.JavaScript)
	require.NoError(t, err)
	assert.True(t, program.Has(sandbox.HookPreview))
	assert.NotNil(t, m.CredentialTypeDef("zh-CN"))
}

func TestTestPlugin(t *testing.T) {
	program, m, err := InspectPlugin(context.Background(), greetPlugin, sandbox.JavaScript)
	require.NoError(t, err)
	result, err := TestPlugin(context.Background(), program, m, map[string]interface{}{"name": "Ada", "style": "loud"})
	require.NoError(t, err)
	assert.Equal(t, map[string]interface{}{"text": "HELLO, ADA FROM TEST", "step": "greet"}, result.Value)
	assert.Equal(t, []string{"greeting Ada"}, result.Logs)
}

func TestModulesIn(t *testing.T) {
	installTestPlugins(t, map[string]bool{greetPlugin: true})
	modules := ModulesIn("zh-CN")
	last := modules[len(modules)-1]
	assert.Equal(t, "plugin:greet", last.Type)
	assert.Equal(t, "问候", last.Label)
	assert.Equal(t, "文本", last.Outputs[0].Label)
	assert.Equal(t, &SummaryDef{Parts: []string{"value:name"}}, last.Summary)
	assert.Len(t, modules, len(Modules())+1)
}
