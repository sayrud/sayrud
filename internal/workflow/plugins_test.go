package workflow

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/workflow/sandbox"
)

func TestPluginRegistryLanguageChange(t *testing.T) {
	ctx := context.Background()
	store := db.NewWorkflowPluginsStore(newTestDB(t))
	source := `
export const manifest = { id: "language", version: "1", name: "Language" };
export function execute() { return Math.max<number>(1, 2); }
`
	_, manifest, err := InspectPlugin(ctx, source, sandbox.JavaScript)
	require.NoError(t, err)
	raw, err := json.Marshal(manifest)
	require.NoError(t, err)
	plugin := &db.WorkflowPlugin{PluginID: manifest.ID, Name: "Language", Version: "1", Language: string(sandbox.JavaScript), Source: source, Checksum: PluginChecksum(source), Manifest: raw, Enabled: true}
	require.NoError(t, store.Save(ctx, plugin))

	registry := &pluginRegistry{byType: map[string]*installedPlugin{}}
	require.NoError(t, registry.sync(ctx, store))
	first := registry.get(PluginType(manifest.ID))
	require.NotNil(t, first)
	program, err := first.code.compile(ctx)
	require.NoError(t, err)
	_, err = program.Run(ctx, sandbox.Call{Hook: sandbox.HookExecute})
	require.Error(t, err)

	plugin.Language = string(sandbox.TypeScript)
	require.NoError(t, store.Save(ctx, plugin))
	require.NoError(t, registry.sync(ctx, store))
	second := registry.get(PluginType(manifest.ID))
	require.NotNil(t, second)
	assert.NotSame(t, first.code, second.code)
	assert.Equal(t, sandbox.TypeScript, second.code.language)
	program, err = second.code.compile(ctx)
	require.NoError(t, err)
	result, err := program.Run(ctx, sandbox.Call{Hook: sandbox.HookExecute})
	require.NoError(t, err)
	assert.Equal(t, float64(2), result.Value)

	require.NoError(t, store.SetEnabled(ctx, manifest.ID, false))
	require.NoError(t, registry.sync(ctx, store))
	third := registry.get(PluginType(manifest.ID))
	require.NotNil(t, third)
	assert.False(t, third.enabled)
	assert.Same(t, second.code, third.code)
}
