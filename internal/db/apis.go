// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package db

import (
	"context"

	"github.com/lib/pq"
	"github.com/pkg/errors"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/dbutil"
)

var _ ApisStore = (*apis)(nil)

var Apis ApisStore

type ApisStore interface {
	List(ctx context.Context, projectID uint, options ListApiOptions) ([]*Api, int64, error)
	GetByID(ctx context.Context, apiID uint) (*Api, error)
	GetByUID(ctx context.Context, apiUID string) (*Api, error)
	GetByMethodPath(ctx context.Context, projectID uint, method, path string) (*Api, error)
	Create(ctx context.Context, opts CreateApiOptions) (*Api, error)
	Update(ctx context.Context, fieldID uint, opts UpdateApiOptions) error
	DeleteByID(ctx context.Context, fieldID uint) error
}

func NewApisStore(db *gorm.DB) ApisStore {
	return &apis{db}
}

type Api struct {
	dbutil.Model

	ProjectID uint    `gorm:"uniqueIndex:idx_sl_table_project_id_name, where:deleted_at IS NULL" json:"-"`
	Project   Project `gorm:"foreignKey:ProjectID" json:"-"`

	Methods     pq.StringArray `gorm:"type:text[]" json:"methods"`
	Path        string         `json:"path"`
	QueryParams datatypes.JSON `gorm:"type:jsonb" json:"queryParams"`
	BodyParams  datatypes.JSON `gorm:"type:jsonb" json:"bodyParams"`
	Datasets    datatypes.JSON `gorm:"type:jsonb" json:"datasets"`
	Response    datatypes.JSON `gorm:"type:jsonb" json:"response"`
}

type apis struct {
	*gorm.DB
}

type ListApiOptions struct {
	dbutil.Pagination
}

func (db *apis) List(ctx context.Context, projectID uint, options ListApiOptions) ([]*Api, int64, error) {
	var total int64
	q := db.WithContext(ctx).Model(&Api{}).Preload("Project").Where("project_id = ?", projectID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, errors.Wrap(err, "count")
	}

	limit, offset := dbutil.LimitOffset(options.Page, options.PageSize)
	var apis []*Api
	return apis, total, q.Limit(limit).Offset(offset).Find(&apis).Error
}

var ErrApiNotFound = errors.New("api does not exist")

func (db *apis) getBy(ctx context.Context, where string, args ...interface{}) (*Api, error) {
	var api Api
	if err := db.WithContext(ctx).Model(&Api{}).Preload("Project").Where(where, args...).First(&api).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrApiNotFound
		}
		return nil, errors.Wrap(err, "get by")
	}
	return &api, nil
}

func (db *apis) GetByID(ctx context.Context, apiID uint) (*Api, error) {
	return db.getBy(ctx, "id = ?", apiID)
}

func (db *apis) GetByUID(ctx context.Context, apiUID string) (*Api, error) {
	return db.getBy(ctx, "uid = ?", apiUID)
}

func (db *apis) GetByMethodPath(ctx context.Context, projectID uint, method, path string) (*Api, error) {
	return db.getBy(ctx, `project_id = ? AND (methods @> ? OR methods @> ?)  AND path = ?`, projectID,
		pq.StringArray{method}, pq.StringArray{"*"},
		path,
	)
}

type CreateApiOptions struct {
	ProjectID   uint
	Methods     pq.StringArray
	Path        string
	QueryParams datatypes.JSON
	BodyParams  datatypes.JSON
	Datasets    datatypes.JSON
	Response    datatypes.JSON
}

func (db *apis) Create(ctx context.Context, opts CreateApiOptions) (*Api, error) {
	api := &Api{
		ProjectID:   opts.ProjectID,
		Methods:     opts.Methods,
		Path:        opts.Path,
		QueryParams: opts.QueryParams,
		BodyParams:  opts.BodyParams,
		Datasets:    opts.Datasets,
		Response:    opts.Response,
	}
	if err := db.WithContext(ctx).Create(api).Error; err != nil {
		return nil, err
	}
	return api, nil
}

type UpdateApiOptions struct {
	Methods     pq.StringArray
	Path        string
	QueryParams datatypes.JSON
	BodyParams  datatypes.JSON
	Datasets    datatypes.JSON
	Response    string
}

func (db *apis) Update(ctx context.Context, apiID uint, opts UpdateApiOptions) error {
	_, err := db.GetByID(ctx, apiID)
	if err != nil {
		return err
	}

	if err := db.WithContext(ctx).Model(&Api{}).Where("id = ?", apiID).Updates(map[string]interface{}{
		"methods":      opts.Methods,
		"path":         opts.Path,
		"query_params": opts.QueryParams,
		"body_params":  opts.BodyParams,
		"datasets":     opts.Datasets,
		"response":     opts.Response,
	}).Error; err != nil {
		return err
	}
	return nil
}

func (db *apis) DeleteByID(ctx context.Context, apiID uint) error {
	if err := db.WithContext(ctx).Where("id = ?", apiID).Delete(&Api{}).Error; err != nil {
		return err
	}
	return nil
}
