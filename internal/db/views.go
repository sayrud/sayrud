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
)

var _ ViewsStore = (*views)(nil)

var Views ViewsStore

type ViewsStore interface {
	GetByID(ctx context.Context, viewID uint) (*View, error)
	GetByUID(ctx context.Context, viewUID string) (*View, error)
	GetByTableID(ctx context.Context, tableID uint) ([]*View, error)
	Create(ctx context.Context, options CreateViewOptions) (*View, error)
	Update(ctx context.Context, id uint, options UpdateViewOptions) error
	DeleteByID(ctx context.Context, id uint) error
}

func NewViewsStore(db *gorm.DB) ViewsStore {
	return &views{db}
}

type View struct {
	gorm.Model
	SLTableID  uint            `json:"-"`
	SLFieldIDs pq.Int64Array   `gorm:"type:integer[]" json:"-"`
	SLFields   []SLField       `gorm:"-" json:"fields"`
	Filter     json.RawMessage `json:"filter"`
	Order      json.RawMessage `json:"order"`
}

type views struct {
	*gorm.DB
}

func (db *views) GetByID(ctx context.Context, viewID uint) (*View, error) {
	return db.getBy(ctx, "id = ?", viewID)
}

func (db *views) GetByUID(ctx context.Context, viewUID string) (*View, error) {
	return db.getBy(ctx, "uid = ?", viewUID)
}

func (db *views) GetByTableID(ctx context.Context, tableID uint) ([]*View, error) {
	var views []*View
	if err := db.WithContext(ctx).Where("sl_table_id = ?", tableID).Find(&views).Error; err != nil {
		return nil, err
	}
	return views, nil
}

var ErrViewNotFound = errors.New("view does not exist")

func (db *views) getBy(ctx context.Context, query string, args ...interface{}) (*View, error) {
	var view View
	if err := db.WithContext(ctx).Where(query, args...).First(&view).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrViewNotFound
		}
		return nil, err
	}
	return &view, nil
}

type CreateViewOptions struct {
	SLTableID  uint
	SLFieldIDs pq.Int64Array
	Filter     json.RawMessage
	Order      json.RawMessage
}

func (db *views) Create(ctx context.Context, options CreateViewOptions) (*View, error) {
	view := &View{
		SLTableID:  options.SLTableID,
		SLFieldIDs: options.SLFieldIDs,
		Filter:     options.Filter,
		Order:      options.Order,
	}
	if err := db.WithContext(ctx).Create(view).Error; err != nil {
		return nil, err
	}
	return view, nil
}

type UpdateViewOptions struct {
	SLFieldIDs pq.Int64Array
	Filter     json.RawMessage
	Order      json.RawMessage
}

func (db *views) Update(ctx context.Context, id uint, options UpdateViewOptions) error {
	view, err := db.GetByID(ctx, id)
	if err != nil {
		return errors.Wrap(err, "get")
	}

	view.SLFieldIDs = options.SLFieldIDs
	view.Filter = options.Filter
	view.Order = options.Order

	return db.WithContext(ctx).Save(view).Error
}

func (db *views) DeleteByID(ctx context.Context, id uint) error {
	return db.WithContext(ctx).Where("id = ?", id).Delete(&View{}).Error
}
