package workflow

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/db"
)

const branchingPlugin = `
export const manifest = {
  id: "moderate", version: "1", name: "Moderate", action: true,
  branches: { output: "passed", then: { en: "Approved", "zh-CN": "审核通过" }, else: "Rejected" },
  outputs: [{ key: "passed", kind: "boolean" }],
  sections: [{ fields: [{ key: "passed", widget: "value" }] }]
};
export function execute(config, ctx) { ctx.log("checked"); return { passed: config.passed }; }
export function preview() { return { passed: false, preview: true }; }
`

func TestPluginBranchesManifest(t *testing.T) {
	installTestPlugins(t, map[string]bool{branchingPlugin: true})
	def := moduleDef("plugin:moderate")
	assert.True(t, def.Branches)
	assert.Equal(t, "Approved", def.BranchLabels["then"])
	assert.Equal(t, "审核通过", plugins.get(def.Type).manifest.ModuleDef("zh-CN", 1000, true).BranchLabels["then"])

	for _, branches := range []string{
		`{"output":"missing","then":"Approved","else":"Rejected"}`,
		`{"output":"text","then":"Approved","else":"Rejected"}`,
		`{"output":"passed","else":"Rejected"}`,
		`{"output":"passed","then":"Approved"}`,
	} {
		_, err := ParsePluginManifest([]byte(`{"id":"moderate","version":"1","name":"Moderate","outputs":[{"key":"passed","kind":"boolean"},{"key":"text","kind":"string"}],"branches":` + branches + `}`))
		require.Error(t, err)
	}
}

func TestRunPluginBranches(t *testing.T) {
	installTestPlugins(t, map[string]bool{branchingPlugin: true, greetPlugin: true})
	for _, tc := range []struct {
		name    string
		value   string
		preview bool
		reuse   bool
		branch  string
	}{
		{name: "approved", value: "true", branch: "then"},
		{name: "rejected", value: "false", branch: "else"},
		{name: "preview", value: "true", preview: true, branch: "else"},
		{name: "retry approved", value: "true", reuse: true, branch: "then"},
		{name: "retry rejected", value: "false", reuse: true, branch: "else"},
		{name: "missing output", value: "null"},
		{name: "truthy output", value: `"false"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			step := pluginStep(t, `{"id":"moderate","type":"plugin:moderate","config":{"passed":{"kind":"const","const":`+tc.value+`}},
"then":[{"id":"approved","type":"plugin:greet","config":{"name":{"kind":"var","var":"steps.moderate.passed"}}}],
"else":[{"id":"rejected","type":"plugin:greet","config":{"name":{"kind":"var","var":"steps.moderate.passed"}}}]}`)
			x := newPluginExecution(nil, tc.preview)
			persisted := map[string]interface{}{}
			x.persist = func(_ *gorm.DB, id string, output interface{}) error {
				persisted[id] = output
				return nil
			}
			if tc.reuse {
				x.done = map[string]json.RawMessage{"moderate": json.RawMessage(`{"passed":` + tc.value + `}`)}
			}

			_, err := x.runSteps([]Step{step})
			if tc.branch == "" {
				require.Error(t, err)
				require.Len(t, x.trace, 1)
				assert.Equal(t, TraceFailed, x.trace[0].Status)
				assert.Empty(t, persisted)
				return
			}

			require.NoError(t, err)
			require.Len(t, x.trace, 2)
			assert.Equal(t, tc.branch, x.trace[0].Branch)
			assert.Equal(t, tc.reuse, x.trace[0].Reused)
			assert.Equal(t, map[string]string{"then": "approved", "else": "rejected"}[tc.branch], x.trace[1].StepID)
			if tc.preview {
				assert.Equal(t, TracePlanned, x.trace[0].Status)
				assert.Empty(t, persisted["moderate"])
			} else if !tc.reuse {
				assert.Equal(t, []string{"checked"}, x.trace[0].Logs)
				assert.NotNil(t, persisted["moderate"])
			}
		})
	}
}

func TestBranchingPluginScopeAndNormalization(t *testing.T) {
	installTestPlugins(t, map[string]bool{branchingPlugin: true, greetPlugin: true})
	spec := &Spec{
		Trigger: Trigger{Type: TriggerSchedule, Schedule: &ScheduleTrigger{Start: "2026-01-01T09:00", Timezone: "UTC", Repeat: RepeatDaily}},
		Steps: []Step{
			pluginStep(t, `{"id":"moderate","type":"plugin:moderate","config":{"passed":{"kind":"const","const":true}},
"then":[{"id":"inside","type":"plugin:greet","config":{"name":{"kind":"var","var":"steps.moderate.passed"}}}],
"else":[{"id":"other","type":"plugin:greet","config":{"name":{"kind":"var","var":"steps.moderate.passed"}}}]}`),
			pluginStep(t, `{"id":"after","type":"plugin:greet","config":{"name":{"kind":"var","var":"steps.moderate.passed"}}}`),
			pluginStep(t, `{"id":"unavailable","type":"plugin:greet","config":{"name":{"kind":"var","var":"steps.inside.text"}}}`),
		},
	}
	spec.Normalize()
	require.Len(t, spec.Steps[0].Then, 1)
	require.Len(t, spec.Steps[0].Else, 1)

	issues, err := Check(context.Background(), nil, &db.Project{ID: 1}, spec)
	require.NoError(t, err)
	assert.Equal(t, []Issue{{Code: "value.unknown_var", Target: "step:unavailable", Args: map[string]string{"var": "steps.inside.text"}}}, issues)
}

func TestPluginBranchResponse(t *testing.T) {
	installTestPlugins(t, map[string]bool{branchingPlugin: true, greetPlugin: true})
	for _, value := range []string{"true", "false"} {
		step := pluginStep(t, `{"id":"moderate","type":"plugin:moderate","config":{"passed":{"kind":"const","const":`+value+`}},
"then":[{"id":"approved","type":"response","config":{"status":200,"body":{"type":"value","value":{"kind":"var","var":"steps.moderate.passed"}}}}],
"else":[{"id":"rejected","type":"response","config":{"status":422,"body":{"type":"value","value":{"kind":"var","var":"steps.moderate.passed"}}}}]}`)
		x := newPluginExecution(nil, false)
		stop, err := x.runSteps([]Step{step, pluginStep(t, `{"id":"unreachable","type":"plugin:greet","config":{}}`)})
		require.NoError(t, err)
		assert.True(t, stop)
		require.NotNil(t, x.response)
		assert.Equal(t, value == "true", x.response.Body)
		if value == "true" {
			assert.Equal(t, 200, x.response.Status)
		} else {
			assert.Equal(t, 422, x.response.Status)
		}
		assert.Len(t, x.trace, 2)
	}
}
