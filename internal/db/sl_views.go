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

var SLViews SLViewsStore

type SLViewsStore interface {
	ListByTableID(ctx context.Context, tableID int64) ([]*SLView, error)
	GetByUID(ctx context.Context, tableID int64, viewUID string) (*SLView, error)
	Create(ctx context.Context, options CreateSLViewOptions) (*SLView, error)
	Update(ctx context.Context, viewID int64, options UpdateSLViewOptions) error
	// Move moves the view to the zero-based index among the table views, and renumbers all the view positions.
	Move(ctx context.Context, tableID, viewID int64, index int) error
	DeleteByID(ctx context.Context, viewID int64) error
}

func NewSLViewsStore(db *gorm.DB) SLViewsStore {
	return &slViews{db}
}

type SLViewType string

const (
	GridViewType    SLViewType = "grid"
	KanbanViewType  SLViewType = "kanban"
	GalleryViewType SLViewType = "gallery"
	FormViewType    SLViewType = "form"
)

func (t SLViewType) Check() bool {
	return lo.Contains([]SLViewType{GridViewType, KanbanViewType, GalleryViewType, FormViewType}, t)
}

// SLView represents a view of the schemaless table. The config (filter, sort, group, layout, etc.) is maintained by the frontend.
type SLView struct {
	dbutil.Model

	SLTableID int64  `gorm:"index;uniqueIndex:idx_sl_view_table_id_uid, where:deleted_at IS NULL"`
	UID       string `gorm:"uniqueIndex:idx_sl_view_table_id_uid, where:deleted_at IS NULL"`
	Name      string
	Type      SLViewType
	Config    datatypes.JSON `gorm:"type:jsonb"`
	Position  int
}

func (slView *SLView) BeforeCreate(_ *gorm.DB) error {
	if slView.UID == "" {
		slView.UID = "viw" + randstr.String(10)
	}
	return nil
}

type slViews struct {
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

type CreateSLViewOptions struct {
	// UID is generated randomly if empty.
	UID       string
	SLTableID int64
	Name      string
	Type      SLViewType
	Config    json.RawMessage
	Position  int
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
	Name   *string
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
