package workflow

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/sirupsen/logrus"

	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/workflow/sandbox"
)

// pluginOrder is the order of the first plugin module, after the built-in modules.
const pluginOrder = 1000

// pluginSyncInterval is how stale the installed plugins of this instance may be before they are read again.
const pluginSyncInterval = 5 * time.Second

// installedPlugin is an installed plugin; it is immutable and replaced as a whole when the plugin changes.
type installedPlugin struct {
	manifest *PluginManifest
	checksum string
	enabled  bool
	order    int
	// base is the module DSL in English, used for checking and executing.
	base *ModuleDef
	code *pluginCode
}

// pluginCode compiles the source on first use; versions share it only when their source and language match.
type pluginCode struct {
	name     string
	source   string
	language sandbox.Language

	once    sync.Once
	program *sandbox.Program
	err     error
}

func (c *pluginCode) compile(ctx context.Context) (*sandbox.Program, error) {
	c.once.Do(func() {
		c.program, c.err = sandbox.Compile(context.WithoutCancel(ctx), c.name, c.source, c.language)
		c.source = ""
	})
	return c.program, c.err
}

func newInstalledPlugin(m *PluginManifest, checksum string, enabled bool, order int, code *pluginCode) *installedPlugin {
	return &installedPlugin{manifest: m, checksum: checksum, enabled: enabled, order: order, base: m.ModuleDef("en", order, enabled), code: code}
}

type pluginRegistry struct {
	mu      sync.RWMutex
	byType  map[string]*installedPlugin
	ordered []*installedPlugin

	syncMu   sync.Mutex
	syncedAt time.Time
}

var plugins = &pluginRegistry{byType: map[string]*installedPlugin{}}

func (r *pluginRegistry) get(stepType string) *installedPlugin {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.byType[stepType]
}

func (r *pluginRegistry) list() []*installedPlugin {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.ordered
}

// SyncPlugins reads the installed plugins again when they may be stale, or always when force is set, e.g. after the admin changed them.
// Plugins whose stored manifest is no longer valid are skipped; the plugins already loaded are kept when the database can not be read.
func SyncPlugins(ctx context.Context, force bool) {
	store := db.WorkflowPlugins
	if store == nil {
		return
	}
	r := plugins
	r.syncMu.Lock()
	defer r.syncMu.Unlock()
	if !force && time.Since(r.syncedAt) < pluginSyncInterval {
		return
	}
	if err := r.sync(ctx, store); err != nil {
		logrus.WithContext(ctx).WithError(err).Warn("Failed to sync workflow plugins")
		return
	}
	r.syncedAt = time.Now()
}

func (r *pluginRegistry) sync(ctx context.Context, store db.WorkflowPluginsStore) error {
	fingerprints, err := store.Fingerprints(ctx)
	if err != nil {
		return err
	}
	byType := make(map[string]*installedPlugin, len(fingerprints))
	ordered := make([]*installedPlugin, 0, len(fingerprints))
	for i, fp := range fingerprints {
		stepType, order := PluginType(fp.PluginID), pluginOrder+i
		var entry *installedPlugin
		if current := r.get(stepType); current != nil && current.checksum == fp.Checksum && current.code.language == sandbox.Language(fp.Language) {
			entry = current
			if current.enabled != fp.Enabled || current.order != order {
				entry = newInstalledPlugin(current.manifest, current.checksum, fp.Enabled, order, current.code)
			}
		} else {
			p, err := store.GetByPluginID(ctx, fp.PluginID)
			if errors.Is(err, db.ErrWorkflowPluginNotFound) {
				continue
			} else if err != nil {
				return err
			}
			m, err := ParsePluginManifest(p.Manifest)
			if err != nil {
				logrus.WithContext(ctx).WithError(err).WithField("plugin", p.PluginID).Warn("Skip the workflow plugin with an invalid manifest")
				continue
			}
			code := &pluginCode{name: p.PluginID + pluginExtension(sandbox.Language(p.Language)), source: p.Source, language: sandbox.Language(p.Language)}
			entry = newInstalledPlugin(m, p.Checksum, p.Enabled, order, code)
		}
		byType[stepType] = entry
		ordered = append(ordered, entry)
	}
	r.mu.Lock()
	r.byType, r.ordered = byType, ordered
	r.mu.Unlock()
	return nil
}

func pluginExtension(lang sandbox.Language) string {
	if lang == sandbox.TypeScript {
		return ".ts"
	}
	return ".js"
}

// PluginChecksum returns the checksum stored with a plugin source.
func PluginChecksum(source string) string {
	sum := sha256.Sum256([]byte(source))
	return hex.EncodeToString(sum[:])
}

// moduleDef returns the DSL of a built-in or installed module in English, or nil if the type is unknown.
func moduleDef(stepType string) *ModuleDef {
	if def := moduleDefs[stepType]; def != nil {
		return def
	}
	if p := plugins.get(stepType); p != nil {
		return p.base
	}
	return nil
}

// ModulesIn returns the definitions of the built-in modules followed by the installed plugins in the language; plugins that are disabled are included with Plugin.Enabled false.
func ModulesIn(lang string) []*ModuleDef {
	list := plugins.list()
	out := make([]*ModuleDef, 0, len(moduleList)+len(list))
	out = append(out, moduleList...)
	for _, p := range list {
		out = append(out, p.manifest.ModuleDef(lang, p.order, p.enabled))
	}
	return out
}

// CredentialTypesIn returns the built-in credential types followed by those of the installed plugins in the language.
func CredentialTypesIn(lang string) []*CredentialTypeDef {
	out := append([]*CredentialTypeDef{}, credentialList...)
	for _, p := range plugins.list() {
		if def := p.manifest.CredentialTypeDef(lang); def != nil {
			out = append(out, def)
		}
	}
	return out
}

// PluginInfo is an installed plugin as seen by this instance.
type PluginInfo struct {
	Manifest *PluginManifest
	Enabled  bool
}

// InstalledPlugin returns the plugin of the step type known by this instance, or nil.
func InstalledPlugin(stepType string) *PluginInfo {
	p := plugins.get(stepType)
	if p == nil {
		return nil
	}
	return &PluginInfo{Manifest: p.manifest, Enabled: p.enabled}
}
