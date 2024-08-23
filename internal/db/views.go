// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package db

import (
	"context"
	"encoding/json"

	"github.com/lib/pq"
	"github.com/pkg/errors"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/dbutil"
)

var _ ViewsStore = (*views)(nil)

var Views ViewsStore

type ViewsStore interface {
	List(ctx context.Context, projectID uint, options ListViewOptions) ([]*View, int64, error)
	GetByID(ctx context.Context, viewID uint) (*View, error)
	GetByUID(ctx context.Context, viewUID string) (*View, error)
	GetByProjectID(ctx context.Context, projectID uint) ([]*View, error)
	GetByTableID(ctx context.Context, tableID uint) ([]*View, error)
	Create(ctx context.Context, options CreateViewOptions) (*View, error)
	Update(ctx context.Context, id uint, options UpdateViewOptions) error
	DeleteByID(ctx context.Context, id uint) error
}

func NewViewsStore(db *gorm.DB) ViewsStore {
	return &views{db}
}

type View struct {
	dbutil.Model
	ProjectID   uint            `json:"-"`
	SLTableID   uint            `json:"-"`
	SLTable     SLTable         `gorm:"foreignKey:SLTableID" json:"table"`
	Name        string          `json:"name"`
	SLFieldUIDs pq.StringArray  `gorm:"type:text[]" json:"fieldUIDs"`
	Filter      json.RawMessage `json:"filter"`
	Order       json.RawMessage `json:"order"`
}

type views struct {
	*gorm.DB
}

type ListViewOptions struct {
	dbutil.Pagination
}

func (db *views) List(ctx context.Context, projectID uint, options ListViewOptions) ([]*View, int64, error) {
	var total int64
	q := db.WithContext(ctx).Model(&View{}).Where("project_id = ?", projectID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, errors.Wrap(err, "count")
	}

	limit, offset := dbutil.LimitOffset(options.Page, options.PageSize)
	var views []*View
	return views, total, q.Limit(limit).Offset(offset).Find(&views).Error
}

func (db *views) GetByID(ctx context.Context, viewID uint) (*View, error) {
	return db.getBy(ctx, "id = ?", viewID)
}

func (db *views) GetByUID(ctx context.Context, viewUID string) (*View, error) {
	return db.getBy(ctx, "uid = ?", viewUID)
}

func (db *views) GetByProjectID(ctx context.Context, projectID uint) ([]*View, error) {
	return db.queryBy(ctx, "project_id = ?", projectID)
}

func (db *views) GetByTableID(ctx context.Context, tableID uint) ([]*View, error) {
	return db.queryBy(ctx, "sl_table_id = ?", tableID)
}

func (db *views) queryBy(ctx context.Context, query string, args ...interface{}) ([]*View, error) {
	var views []*View
	if err := db.WithContext(ctx).Preload("SLTable.Project").Where(query, args...).Find(&views).Error; err != nil {
		return nil, err
	}
	return views, nil
}

var ErrViewNotFound = errors.New("view does not exist")

func (db *views) getBy(ctx context.Context, query string, args ...interface{}) (*View, error) {
	var view View
	if err := db.WithContext(ctx).Preload("SLTable.Project").Where(query, args...).First(&view).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrViewNotFound
		}
		return nil, err
	}
	return &view, nil
}

type CreateViewOptions struct {
	ProjectID   uint
	SLTableID   uint
	Name        string
	SLFieldUIDs pq.StringArray
	Filter      json.RawMessage
	Order       json.RawMessage
}

func (db *views) Create(ctx context.Context, options CreateViewOptions) (*View, error) {
	view := &View{
		ProjectID:   options.ProjectID,
		SLTableID:   options.SLTableID,
		Name:        options.Name,
		SLFieldUIDs: options.SLFieldUIDs,
		Filter:      options.Filter,
		Order:       options.Order,
	}
	if err := db.WithContext(ctx).Create(view).Error; err != nil {
		return nil, err
	}
	return view, nil
}

type UpdateViewOptions struct {
	Name        string
	SLFieldUIDs pq.StringArray
	Filter      json.RawMessage
	Order       json.RawMessage
}

func (db *views) Update(ctx context.Context, id uint, options UpdateViewOptions) error {
	view, err := db.GetByID(ctx, id)
	if err != nil {
		return errors.Wrap(err, "get")
	}

	view.Name = options.Name
	view.SLFieldUIDs = options.SLFieldUIDs
	view.Filter = options.Filter
	view.Order = options.Order

	return db.WithContext(ctx).Save(view).Error
}

func (db *views) DeleteByID(ctx context.Context, id uint) error {
	return db.WithContext(ctx).Where("id = ?", id).Delete(&View{}).Error
}
