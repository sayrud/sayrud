package db

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowPluginsStore(t *testing.T) {
	ctx := context.Background()
	gormDB := newTestDB(t, &WorkflowPlugin{}, &Workflow{}, &WorkflowRevision{})
	store := NewWorkflowPluginsStore(gormDB)

	plugin := func(id, version string) *WorkflowPlugin {
		return &WorkflowPlugin{PluginID: id, Name: id, Version: version, Language: "javascript", Source: "export const manifest = {}", Checksum: version, Manifest: []byte(`{}`), Enabled: true}
	}
	require.NoError(t, store.Save(ctx, plugin("greet", "1.0.0")))
	require.NoError(t, store.Save(ctx, plugin("notify", "1.0.0")))
	require.NoError(t, store.SetEnabled(ctx, "greet", false))

	// Upgrading keeps the installation order and the enabled state.
	upgraded := plugin("greet", "2.0.0")
	require.NoError(t, store.Save(ctx, upgraded))
	assert.False(t, upgraded.Enabled)

	list, err := store.List(ctx)
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, "greet", list[0].PluginID)
	assert.Equal(t, "2.0.0", list[0].Version)
	assert.Empty(t, list[0].Source)

	fingerprints, err := store.Fingerprints(ctx)
	require.NoError(t, err)
	require.Len(t, fingerprints, 2)
	assert.Equal(t, "2.0.0", fingerprints[0].Checksum)
	assert.Equal(t, "javascript", fingerprints[0].Language)
	assert.False(t, fingerprints[0].Enabled)

	saved, err := store.GetByPluginID(ctx, "greet")
	require.NoError(t, err)
	assert.Equal(t, "export const manifest = {}", saved.Source)
	assert.ErrorIs(t, store.SetEnabled(ctx, "nope", true), ErrWorkflowPluginNotFound)

	workflows := NewWorkflowsStore(gormDB)
	_, err = workflows.Create(ctx, CreateWorkflowOptions{ProjectID: 1, Name: "uses", Draft: []byte(`{"steps":[{"id":"greet1","type":"plugin:greet","config":{}}]}`)})
	require.NoError(t, err)
	_, err = workflows.Create(ctx, CreateWorkflowOptions{ProjectID: 1, Name: "other", Draft: []byte(`{"steps":[{"id":"greet1","type":"plugin:greeter","config":{}}]}`)})
	require.NoError(t, err)
	count, err := store.CountUsage(ctx, "plugin:greet")
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	require.NoError(t, store.Delete(ctx, "greet"))
	_, err = store.GetByPluginID(ctx, "greet")
	assert.ErrorIs(t, err, ErrWorkflowPluginNotFound)
	// The same plugin can be installed again.
	require.NoError(t, store.Save(ctx, plugin("greet", "3.0.0")))
}

func TestWorkflowPluginUsageMatchesSteps(t *testing.T) {
	ctx := context.Background()
	gormDB := newTestDB(t, &Workflow{}, &WorkflowRevision{})
	store, workflows := NewWorkflowPluginsStore(gormDB), NewWorkflowsStore(gormDB)
	for _, tc := range []struct {
		name      string
		draft     string
		published string
		used      bool
		deleted   bool
	}{
		{name: "main", draft: `{"steps":[{"type":"plugin:greet"}]}`, used: true},
		{name: "async", draft: `{"async":[{"steps":[{"type":"plugin:greet"}]}]}`, used: true},
		{name: "then", draft: `{"steps":[{"type":"condition","then":[{"type":"plugin:greet"}]}]}`, used: true},
		{name: "else", draft: `{"steps":[{"type":"condition","else":[{"type":"plugin:greet"}]}]}`, used: true},
		{name: "nested async", draft: `{"async":[{"steps":[{"type":"condition","then":[{"type":"condition","else":[{"type":"plugin:greet"}]}]}]}]}`, used: true},
		{name: "duplicate steps", draft: `{"steps":[{"type":"plugin:greet"},{"type":"plugin:greet"}]}`, used: true},
		{name: "constant", draft: `{"steps":[{"type":"response","config":{"value":"plugin:greet"}}]}`},
		{name: "config type", draft: `{"steps":[{"type":"response","config":{"type":"plugin:greet","steps":[{"type":"plugin:greet"}]}}]}`},
		{name: "similar type", draft: `{"steps":[{"type":"plugin:greeter"}]}`},
		{name: "root type", draft: `{"type":"plugin:greet","steps":[]}`},
		{name: "async type", draft: `{"async":[{"type":"plugin:greet","steps":[]}]}`},
		{name: "empty", draft: `{}`},
		{name: "null collections", draft: `{"steps":null,"async":null}`},
		{name: "published", draft: `{}`, published: `{"steps":[{"type":"plugin:greet"}]}`, used: true},
		{name: "published nested async", draft: `{}`, published: `{"async":[{"steps":[{"type":"condition","else":[{"type":"plugin:greet"}]}]}]}`, used: true},
		{name: "both versions", draft: `{"steps":[{"type":"plugin:greet"}]}`, published: `{"steps":[{"type":"plugin:greet"}]}`, used: true},
		{name: "published constant", draft: `{}`, published: `{"steps":[{"type":"response","config":{"value":"plugin:greet"}}]}`},
		{name: "deleted", draft: `{"steps":[{"type":"plugin:greet"}]}`, deleted: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, err := workflows.Create(ctx, CreateWorkflowOptions{ProjectID: 1, Name: tc.name, Draft: []byte(tc.draft)})
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, workflows.DeleteByID(ctx, w.ID)) })
			if tc.published != "" {
				_, err = workflows.Publish(ctx, w.ID, PublishWorkflowOptions{DraftVersion: w.DraftVersion, Spec: []byte(tc.published)})
				require.NoError(t, err)
			}
			if tc.deleted {
				require.NoError(t, workflows.DeleteByID(ctx, w.ID))
			}

			count, err := store.CountUsage(ctx, "plugin:greet")
			require.NoError(t, err)
			want := int64(0)
			if tc.used {
				want = 1
			}
			assert.Equal(t, want, count)
		})
	}
}
