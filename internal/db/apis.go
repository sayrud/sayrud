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

// Apis is the default instance of the ApisStore.
var Apis ApisStore

// ApisStore is the persistent interface for the custom APIs of the projects.
type ApisStore interface {
	// List returns the paginated APIs of the project, along with the total count.
	List(ctx context.Context, projectID uint, options ListApiOptions) ([]*Api, int64, error)
	// GetByID returns the API with the given ID.
	// It returns ErrApiNotFound if the API does not exist.
	GetByID(ctx context.Context, apiID uint) (*Api, error)
	// GetByUID returns the API with the given UID.
	// It returns ErrApiNotFound if the API does not exist.
	GetByUID(ctx context.Context, apiUID string) (*Api, error)
	// GetByMethodPath returns the API of the project matching the HTTP method and path, the method "*" matches all methods.
	// It returns ErrApiNotFound if no API matches.
	GetByMethodPath(ctx context.Context, projectID uint, method, path string) (*Api, error)
	// Create creates a new API with the given options.
	Create(ctx context.Context, opts CreateApiOptions) (*Api, error)
	// Update updates the API with the given ID.
	// It returns ErrApiNotFound if the API does not exist.
	Update(ctx context.Context, apiID uint, opts UpdateApiOptions) error
	// DeleteByID deletes the API with the given ID.
	DeleteByID(ctx context.Context, apoID uint) error
}

func NewApisStore(db *gorm.DB) ApisStore {
	return &apis{db}
}

// Api is a custom API of a project, which serves the table data by the configured kind and options.
type Api struct {
	// Model contains the primary key and the creation, update and deletion times.
	dbutil.Model

	// Kind is the operation of the API, e.g. list, view, create, update and delete.
	Kind string
	// ProjectID is the ID of the project the API belongs to.
	ProjectID uint `gorm:"uniqueIndex:idx_sl_table_project_id_name, where:deleted_at IS NULL" json:"-"`
	// Project is the project the API belongs to, it is preloaded when querying.
	Project Project `gorm:"foreignKey:ProjectID" json:"-"`

	// Methods are the HTTP methods the API accepts, "*" accepts all methods.
	Methods pq.StringArray `gorm:"type:text[]"`
	// Path is the request path of the API, relative to the project.
	Path string
	// QueryParams are the definitions of the query parameters.
	QueryParams datatypes.JSON `gorm:"type:jsonb"`
	// BodyParams are the definitions of the request body parameters.
	BodyParams datatypes.JSON `gorm:"type:jsonb"`
	// Options are the kind-specific options, e.g. the datasets to query.
	Options datatypes.JSON `gorm:"type:jsonb"`
	// Middlewares are the middlewares applied before handling the request, e.g. rate limit and captcha.
	Middlewares datatypes.JSON `gorm:"type:jsonb"`
	// Response is the template of the response body.
	Response datatypes.JSON `gorm:"type:jsonb"`
}

type apis struct {
	// DB is the database connection the store operates on.
	*gorm.DB
}

// ListApiOptions are the options of listing the APIs.
type ListApiOptions struct {
	// Pagination is the page and page size of the list.
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

// CreateApiOptions are the options of creating an API, see Api for the meaning of each field.
type CreateApiOptions struct {
	// ProjectID is the ID of the project the API belongs to.
	ProjectID uint
	// Kind is the operation of the API.
	Kind string
	// Methods are the HTTP methods the API accepts.
	Methods pq.StringArray
	// Path is the request path of the API.
	Path string
	// QueryParams are the definitions of the query parameters.
	QueryParams datatypes.JSON
	// BodyParams are the definitions of the request body parameters.
	BodyParams datatypes.JSON
	// Options are the kind-specific options.
	Options datatypes.JSON
	// Middlewares are the middlewares applied before handling the request.
	Middlewares datatypes.JSON
	// Response is the template of the response body.
	Response datatypes.JSON
}

func (db *apis) Create(ctx context.Context, opts CreateApiOptions) (*Api, error) {
	api := &Api{
		ProjectID:   opts.ProjectID,
		Kind:        opts.Kind,
		Methods:     opts.Methods,
		Path:        opts.Path,
		QueryParams: opts.QueryParams,
		BodyParams:  opts.BodyParams,
		Options:     opts.Options,
		Middlewares: opts.Middlewares,
		Response:    opts.Response,
	}
	if err := db.WithContext(ctx).Create(api).Error; err != nil {
		return nil, err
	}
	return api, nil
}

// UpdateApiOptions are the options of updating an API, all the fields are overwritten.
type UpdateApiOptions struct {
	// Kind is the operation of the API.
	Kind string
	// Methods are the HTTP methods the API accepts.
	Methods pq.StringArray
	// Path is the request path of the API.
	Path string
	// QueryParams are the definitions of the query parameters.
	QueryParams datatypes.JSON
	// BodyParams are the definitions of the request body parameters.
	BodyParams datatypes.JSON
	// Options are the kind-specific options.
	Options datatypes.JSON
	// Middlewares are the middlewares applied before handling the request.
	Middlewares datatypes.JSON
	// Response is the template of the response body.
	Response datatypes.JSON
}

func (db *apis) Update(ctx context.Context, apiID uint, opts UpdateApiOptions) error {
	_, err := db.GetByID(ctx, apiID)
	if err != nil {
		return err
	}

	if err := db.WithContext(ctx).Model(&Api{}).Where("id = ?", apiID).Updates(map[string]interface{}{
		"kind":         opts.Kind,
		"methods":      opts.Methods,
		"path":         opts.Path,
		"query_params": opts.QueryParams,
		"body_params":  opts.BodyParams,
		"options":      opts.Options,
		"middlewares":  opts.Middlewares,
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
