package db

import (
	"context"
	"encoding/json"

	"github.com/cockroachdb/errors"
	"github.com/samber/lo"
	"github.com/thanhpk/randstr"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/dbutil"
)

var _ SLViewsStore = (*slViews)(nil)

// SLViews is the default instance of the SLViewsStore.
var SLViews SLViewsStore

// SLViewsStore is the persistent interface for the views of schemaless tables.
type SLViewsStore interface {
	// ListByTableID returns all the views of the table ordered by position.
	ListByTableID(ctx context.Context, tableID int64) ([]*SLView, error)
	// GetByUID returns the view of the table with the given UID.
	// It returns ErrSLViewNotFound if the view does not exist.
	GetByUID(ctx context.Context, tableID int64, viewUID string) (*SLView, error)
	// Create creates a new view in the table.
	// It returns ErrUnexpectedSLView if the view type is unknown, and ErrSLViewExists if the UID has been used in the table.
	Create(ctx context.Context, options CreateSLViewOptions) (*SLView, error)
	// Update updates the name and config of the view with the given ID.
	Update(ctx context.Context, viewID int64, options UpdateSLViewOptions) error
	// Move moves the view to the zero-based index among the table views, and renumbers all the view positions.
	Move(ctx context.Context, tableID, viewID int64, index int) error
	// DeleteByID deletes the view with the given ID.
	DeleteByID(ctx context.Context, viewID int64) error
}

func NewSLViewsStore(db *gorm.DB) SLViewsStore {
	return &slViews{db}
}

// SLViewType is the layout of a view.
type SLViewType string

const (
	GridViewType    SLViewType = "grid"
	KanbanViewType  SLViewType = "kanban"
	GalleryViewType SLViewType = "gallery"
	FormViewType    SLViewType = "form"
)

// Check reports whether the view type is known.
func (t SLViewType) Check() bool {
	return lo.Contains([]SLViewType{GridViewType, KanbanViewType, GalleryViewType, FormViewType}, t)
}

// SLView represents a view of the schemaless table. The config (filter, sort, group, layout, etc.) is maintained by the frontend.
type SLView struct {
	// Model contains the primary key and the creation, update and deletion times.
	dbutil.Model

	// SLTableID is the ID of the table the view belongs to.
	SLTableID int64 `gorm:"index;uniqueIndex:idx_sl_view_table_id_uid, where:deleted_at IS NULL"`
	// UID is the public identifier of the view unique in the table, e.g. "viw" followed by 10 random characters.
	UID string `gorm:"uniqueIndex:idx_sl_view_table_id_uid, where:deleted_at IS NULL"`
	// Name is the display name of the view.
	Name string
	// Type is the layout of the view.
	Type SLViewType
	// Config is the view configuration maintained by the frontend, e.g. filter, sort, group, hidden fields and field widths.
	Config datatypes.JSON `gorm:"type:jsonb"`
	// Position is the order of the view in the table, starting from 0.
	Position int
}

func (slView *SLView) BeforeCreate(_ *gorm.DB) error {
	if slView.UID == "" {
		slView.UID = "viw" + randstr.String(10)
	}
	return nil
}

type slViews struct {
	// DB is the database connection the store operates on.
	*gorm.DB
}

func (db *slViews) ListByTableID(ctx context.Context, tableID int64) ([]*SLView, error) {
	var views []*SLView
	if err := db.WithContext(ctx).Model(&SLView{}).Where("sl_table_id = ?", tableID).Order("position ASC, id ASC").Find(&views).Error; err != nil {
		return nil, errors.Wrap(err, "find")
	}
	return views, nil
}

var (
	ErrSLViewNotFound   = errors.New("sl_view does not exist")
	ErrSLViewExists     = errors.New("sl_view exists")
	ErrUnexpectedSLView = errors.New("unexpected view type")
)

func (db *slViews) GetByUID(ctx context.Context, tableID int64, viewUID string) (*SLView, error) {
	var view SLView
	if err := db.WithContext(ctx).Model(&SLView{}).Where("sl_table_id = ? AND uid = ?", tableID, viewUID).First(&view).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSLViewNotFound
		}
		return nil, errors.Wrap(err, "get")
	}
	return &view, nil
}

// CreateSLViewOptions are the options of creating a view.
type CreateSLViewOptions struct {
	// UID is generated randomly if empty.
	UID string
	// SLTableID is the ID of the table the view belongs to.
	SLTableID int64
	// Name is the display name of the view.
	Name string
	// Type is the layout of the view.
	Type SLViewType
	// Config is the view configuration, it defaults to an empty object.
	Config json.RawMessage
	// Position is the order of the view in the table.
	Position int
}

func (db *slViews) Create(ctx context.Context, options CreateSLViewOptions) (*SLView, error) {
	if !options.Type.Check() {
		return nil, ErrUnexpectedSLView
	}
	config := options.Config
	if len(config) == 0 {
		config = json.RawMessage("{}")
	}

	view := &SLView{
		UID:       options.UID,
		SLTableID: options.SLTableID,
		Name:      options.Name,
		Type:      options.Type,
		Config:    datatypes.JSON(config),
		Position:  options.Position,
	}
	if err := db.WithContext(ctx).Create(view).Error; err != nil {
		if dbutil.IsUniqueViolation(err, "idx_sl_view_table_id_uid") {
			return nil, ErrSLViewExists
		}
		return nil, errors.Wrap(err, "create")
	}
	return view, nil
}

// UpdateSLViewOptions only updates the non-empty options.
type UpdateSLViewOptions struct {
	// Name is the new display name of the view, nil keeps it unchanged.
	Name *string
	// Config replaces the view configuration, empty keeps it unchanged.
	Config json.RawMessage
}

func (db *slViews) Update(ctx context.Context, viewID int64, options UpdateSLViewOptions) error {
	fields := map[string]interface{}{
		"updated_at": dbutil.Now(),
	}
	if options.Name != nil {
		fields["name"] = *options.Name
	}
	if len(options.Config) > 0 {
		fields["config"] = datatypes.JSON(options.Config)
	}
	if err := db.WithContext(ctx).Model(&SLView{}).Where("id = ?", viewID).Updates(fields).Error; err != nil {
		return errors.Wrap(err, "update")
	}
	return nil
}

func (db *slViews) Move(ctx context.Context, tableID, viewID int64, index int) error {
	views, err := db.ListByTableID(ctx, tableID)
	if err != nil {
		return errors.Wrap(err, "list views")
	}
	return movePositions(ctx, db.DB, &SLView{}, views, func(v *SLView) int64 { return v.ID }, viewID, index)
}

func (db *slViews) DeleteByID(ctx context.Context, viewID int64) error {
	if err := db.WithContext(ctx).Delete(&SLView{}, viewID).Error; err != nil {
		return errors.Wrap(err, "delete")
	}
	return nil
}
