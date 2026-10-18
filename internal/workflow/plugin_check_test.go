package workflow

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wuhan005/sayrud/internal/db"
)

func TestCheckPluginZeroValues(t *testing.T) {
	source := `
export const manifest = {
  id: "zero", version: "1", name: "Zero",
  sections: [{ fields: [
    { key: "count", label: "Count", widget: "number", required: true, min: 0, max: 10 },
    { key: "mode", label: "Mode", widget: "select", required: true, options: [{ value: 0, label: "Zero" }] },
    { key: "optional", label: "Optional", widget: "number", min: 1, max: 10 }
  ] }]
};
export function execute(config) { return config; }
`
	installTestPlugins(t, map[string]bool{source: true})
	for _, tc := range []struct {
		name   string
		config string
		code   string
		field  string
	}{
		{name: "zero", config: `{"count":0,"mode":0}`},
		{name: "positive", config: `{"count":1,"mode":0}`},
		{name: "out of range", config: `{"count":11,"mode":0}`, code: "step.out_of_range", field: "Count"},
		{name: "optional zero out of range", config: `{"count":0,"mode":0,"optional":0}`, code: "step.out_of_range", field: "Optional"},
		{name: "invalid option", config: `{"count":0,"mode":1}`, code: "step.invalid_option", field: "Mode"},
		{name: "missing", config: `{"mode":0}`, code: "step.required", field: "Count"},
		{name: "null", config: `{"count":null,"mode":0}`, code: "step.required", field: "Count"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			spec := &Spec{
				Trigger: Trigger{Type: TriggerSchedule, Schedule: &ScheduleTrigger{Start: "2026-01-01T09:00", Timezone: "UTC", Repeat: RepeatDaily}},
				Steps:   []Step{pluginStep(t, `{"id":"zero1","type":"plugin:zero","config":`+tc.config+`}`)},
			}
			spec.Normalize()
			issues, err := Check(context.Background(), nil, &db.Project{ID: 1}, spec)
			require.NoError(t, err)
			if tc.code == "" {
				assert.Empty(t, issues)
			} else {
				require.Len(t, issues, 1)
				assert.Equal(t, tc.code, issues[0].Code)
				assert.Equal(t, tc.field, issues[0].Args["field"])
			}
		})
	}
}

func TestCredentialExistsCachesType(t *testing.T) {
	ctx := context.Background()
	gormDB := newTestDB(t)
	credential, err := db.NewCredentialsStore(gormDB).Create(ctx, db.CreateCredentialOptions{ProjectID: 1, Type: "smtp", Data: []byte(`{}`)})
	require.NoError(t, err)

	for _, types := range [][]string{{"smtp", "plugin:token", ""}, {"plugin:token", "smtp", ""}} {
		c := &checker{ctx: ctx, db: gormDB, project: &db.Project{ID: 1}, credentials: map[[2]string]bool{}}
		for _, credType := range types {
			for range 2 {
				usable, err := c.credentialExists(credential.UID, credType)
				require.NoError(t, err)
				assert.Equal(t, credType != "plugin:token", usable, "required type %q", credType)
			}
		}
	}
}
