package db

import (
	"context"
	"time"

	"github.com/cockroachdb/errors"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wuhan005/sayrud/internal/dbutil"
)

var _ WorkflowPluginsStore = (*workflowPlugins)(nil)

// WorkflowPlugins is the default instance of the WorkflowPluginsStore.
var WorkflowPlugins WorkflowPluginsStore

// WorkflowPluginsStore persists the workflow node plugins installed by the admins.
type WorkflowPluginsStore interface {
	// List returns all the plugins in installation order, without their sources.
	List(ctx context.Context) ([]*WorkflowPlugin, error)
	// Fingerprints returns what tells whether the plugins changed, in installation order.
	Fingerprints(ctx context.Context) ([]*WorkflowPluginFingerprint, error)
	// GetByPluginID returns the plugin with the manifest ID, it returns ErrWorkflowPluginNotFound if not found.
	GetByPluginID(ctx context.Context, pluginID string) (*WorkflowPlugin, error)
	// Save installs the plugin, or replaces the installed plugin of the same manifest ID; the installation order and the enabled state of a replaced plugin are kept.
	Save(ctx context.Context, p *WorkflowPlugin) error
	// SetEnabled enables or disables the plugin, it returns ErrWorkflowPluginNotFound if not found.
	SetEnabled(ctx context.Context, pluginID string, enabled bool) error
	// Delete uninstalls the plugin.
	Delete(ctx context.Context, pluginID string) error
	// CountUsage returns the number of workflows whose draft or published revision uses the step type.
	CountUsage(ctx context.Context, stepType string) (int64, error)
}

func NewWorkflowPluginsStore(db *gorm.DB) WorkflowPluginsStore {
	return &workflowPlugins{db}
}

// WorkflowPlugin is a workflow node implemented by a JavaScript or TypeScript module, see the workflow/sandbox package.
type WorkflowPlugin struct {
	dbutil.Model

	// PluginID is the id of the manifest, the steps refer to the plugin by the type "plugin:<id>".
	PluginID string `gorm:"type:varchar(64);not null;uniqueIndex:idx_workflow_plugins_plugin_id, where:deleted_at IS NULL"`
	// Name is the name of the manifest in English, used for searching and sorting.
	Name    string `gorm:"type:varchar(128);not null"`
	Version string `gorm:"type:varchar(32);not null"`
	// Language is javascript or typescript.
	Language string `gorm:"type:varchar(16);not null"`
	Source   string `gorm:"type:text;not null"`
	// SourceURL is the address the source was downloaded from, empty if uploaded or written by hand.
	SourceURL string `gorm:"type:text;not null;default:''"`
	// Checksum is the hex SHA-256 of the source.
	Checksum string `gorm:"type:varchar(64);not null"`
	// Manifest is the exported manifest, kept so that the nodes can be listed without running the source.
	Manifest datatypes.JSON `gorm:"type:jsonb;not null"`
	// Enabled reports whether the node can be added to workflows and executed.
	Enabled bool `gorm:"not null;default:true"`
}

// WorkflowPluginFingerprint is the version of an installed plugin.
type WorkflowPluginFingerprint struct {
	PluginID  string
	Checksum  string
	Language  string
	Enabled   bool
	UpdatedAt time.Time
}

var ErrWorkflowPluginNotFound = errors.New("workflow plugin does not exist")

type workflowPlugins struct {
	*gorm.DB
}

func (db *workflowPlugins) List(ctx context.Context) ([]*WorkflowPlugin, error) {
	var list []*WorkflowPlugin
	if err := db.WithContext(ctx).Omit("source").Order("id ASC").Find(&list).Error; err != nil {
		return nil, errors.Wrap(err, "find")
	}
	return list, nil
}

func (db *workflowPlugins) Fingerprints(ctx context.Context) ([]*WorkflowPluginFingerprint, error) {
	var list []*WorkflowPluginFingerprint
	if err := db.WithContext(ctx).Model(&WorkflowPlugin{}).Select("plugin_id", "checksum", "language", "enabled", "updated_at").Order("id ASC").Find(&list).Error; err != nil {
		return nil, errors.Wrap(err, "find")
	}
	return list, nil
}

func (db *workflowPlugins) GetByPluginID(ctx context.Context, pluginID string) (*WorkflowPlugin, error) {
	var p WorkflowPlugin
	if err := db.WithContext(ctx).Where("plugin_id = ?", pluginID).First(&p).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrWorkflowPluginNotFound
		}
		return nil, errors.Wrap(err, "get")
	}
	return &p, nil
}

func (db *workflowPlugins) Save(ctx context.Context, p *WorkflowPlugin) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var saved WorkflowPlugin
		err := tx.Where("plugin_id = ?", p.PluginID).Clauses(clause.Locking{Strength: "UPDATE"}).First(&saved).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.Wrap(tx.Create(p).Error, "create")
		}
		if err != nil {
			return errors.Wrap(err, "get")
		}
		p.ID, p.CreatedAt, p.Enabled = saved.ID, saved.CreatedAt, saved.Enabled
		p.UpdatedAt = dbutil.Now()
		return errors.Wrap(tx.Model(&WorkflowPlugin{}).Where("id = ?", saved.ID).Updates(map[string]interface{}{
			"name":       p.Name,
			"version":    p.Version,
			"language":   p.Language,
			"source":     p.Source,
			"source_url": p.SourceURL,
			"checksum":   p.Checksum,
			"manifest":   p.Manifest,
			"updated_at": p.UpdatedAt,
		}).Error, "update")
	})
}

func (db *workflowPlugins) SetEnabled(ctx context.Context, pluginID string, enabled bool) error {
	result := db.WithContext(ctx).Model(&WorkflowPlugin{}).Where("plugin_id = ?", pluginID).
		Updates(map[string]interface{}{"enabled": enabled, "updated_at": dbutil.Now()})
	if result.Error != nil {
		return errors.Wrap(result.Error, "update")
	}
	if result.RowsAffected == 0 {
		return ErrWorkflowPluginNotFound
	}
	return nil
}

func (db *workflowPlugins) Delete(ctx context.Context, pluginID string) error {
	// The row is removed so that the same plugin can be installed again.
	if err := db.WithContext(ctx).Unscoped().Where("plugin_id = ?", pluginID).Delete(&WorkflowPlugin{}).Error; err != nil {
		return errors.Wrap(err, "delete")
	}
	return nil
}

func (db *workflowPlugins) CountUsage(ctx context.Context, stepType string) (int64, error) {
	var count int64
	err := db.WithContext(ctx).Model(&Workflow{}).
		Joins("LEFT JOIN workflow_revisions ON workflow_revisions.id = workflows.published_revision_id").
		Where(`EXISTS (
			WITH RECURSIVE steps(value) AS (
				SELECT value FROM jsonb_array_elements(
					jsonb_path_query_array(workflows.draft, '$.steps[*]') ||
					jsonb_path_query_array(workflows.draft, '$.async[*].steps[*]') ||
					COALESCE(jsonb_path_query_array(workflow_revisions.spec, '$.steps[*]'), '[]'::jsonb) ||
					COALESCE(jsonb_path_query_array(workflow_revisions.spec, '$.async[*].steps[*]'), '[]'::jsonb)
				)
				UNION ALL
				SELECT branch.value FROM steps
				CROSS JOIN LATERAL jsonb_array_elements(
					jsonb_path_query_array(steps.value, '$.then[*]') ||
					jsonb_path_query_array(steps.value, '$.else[*]')
				) AS branch(value)
			)
			SELECT 1 FROM steps WHERE value ->> 'type' = ?
		)`, stepType).
		Count(&count).Error
	if err != nil {
		return 0, errors.Wrap(err, "count")
	}
	return count, nil
}
