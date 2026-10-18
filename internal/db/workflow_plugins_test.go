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
