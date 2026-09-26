// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package db

import (
	"context"
	"strings"

	"github.com/pkg/errors"
	"github.com/thanhpk/randstr"
	escape "github.com/tj/go-pg-escape"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/dbutil"
)

var _ ProjectsStore = (*projects)(nil)

var Projects ProjectsStore

type ProjectsStore interface {
	ListByUserID(ctx context.Context, userID int64, options ListByUserIDOptions) ([]*Project, int64, error)
	GetByID(ctx context.Context, projectID int64) (*Project, error)
	GetByUID(ctx context.Context, projectUID string) (*Project, error)
	Create(ctx context.Context, opts CreateProjectOptions) (*Project, error)
	Update(ctx context.Context, projectID int64, opts UpdateProjectOptions) error
	DeleteByID(ctx context.Context, projectID int64) error
	CreateSchema(ctx context.Context, projectID int64) error
	DeleteSchema(ctx context.Context, projectID int64) error
}

func NewProjectsStore(db *gorm.DB) ProjectsStore {
	return &projects{db}
}

type Project struct {
	dbutil.Model
	UID         string `gorm:"uniqueIndex:idx_projects_uid, where:deleted_at IS NULL" json:"uid"`
	OwnerUserID int64  `json:"-"`
	Name        string `json:"name"`
	SchemaName  string `gorm:"uniqueIndex:idx_projects_schema_name, where:deleted_at IS NULL" json:"schemaName"`

	CustomDomain string `json:"customDomain"`
	RoutePrefix  string `json:"routePrefix"`
}

func (project *Project) BeforeCreate(_ *gorm.DB) error {
	project.UID = "prj" + randstr.String(10)
	return nil
}

type projects struct {
	*gorm.DB
}

type ListByUserIDOptions struct {
	dbutil.Pagination
}

func (db *projects) ListByUserID(ctx context.Context, userID int64, options ListByUserIDOptions) ([]*Project, int64, error) {
	var total int64
	q := db.WithContext(ctx).Model(&Project{}).Where("owner_user_id = ?", userID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, errors.Wrap(err, "count")
	}

	limit, offset := dbutil.LimitOffset(options.Page, options.PageSize)
	var projects []*Project
	return projects, total, q.Limit(limit).Offset(offset).Find(&projects).Error
}

func (db *projects) GetByID(ctx context.Context, projectID int64) (*Project, error) {
	return db.getBy(ctx, "id = ?", projectID)
}

func (db *projects) GetByUID(ctx context.Context, projectUID string) (*Project, error) {
	return db.getBy(ctx, "uid = ?", projectUID)
}

type CreateProjectOptions struct {
	OwnerUserID int64
	Name        string
	SchemaName  string
}

var ErrProjectSchemaNameExists = errors.New("project schema name exists")

func (db *projects) Create(ctx context.Context, opts CreateProjectOptions) (*Project, error) {
	project := &Project{
		OwnerUserID: opts.OwnerUserID,
		Name:        opts.Name,
		SchemaName:  opts.SchemaName,
	}
	if err := db.WithContext(ctx).Create(project).Error; err != nil {
		if dbutil.IsUniqueViolation(err, "idx_projects_schema_name") {
			return nil, ErrProjectSchemaNameExists
		}
		return nil, err
	}
	return project, nil
}

type UpdateProjectOptions struct {
	Name string
}

func (db *projects) Update(ctx context.Context, projectID int64, opts UpdateProjectOptions) error {
	project, err := db.GetByID(ctx, projectID)
	if err != nil {
		return err
	}

	project.Name = opts.Name
	if err := db.WithContext(ctx).Save(project).Error; err != nil {
		return err
	}
	return nil
}

var ErrProjectNotFound = errors.New("project does not exist")

func (db *projects) getBy(ctx context.Context, where string, args ...interface{}) (*Project, error) {
	var project Project
	if err := db.WithContext(ctx).Where(where, args...).First(&project).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProjectNotFound
		}
		return nil, err
	}
	return &project, nil
}

func (db *projects) DeleteByID(ctx context.Context, projectID int64) error {
	if err := db.WithContext(ctx).Where("id = ?", projectID).Delete(&Project{}).Error; err != nil {
		return err
	}
	return nil
}

func (db *projects) CreateSchema(ctx context.Context, projectID int64) error {
	project, err := db.GetByID(ctx, projectID)
	if err != nil {
		return errors.Wrap(err, "get by ID")
	}

	schemaName := project.SchemaName
	if err := db.Exec(escape.Escape("CREATE SCHEMA %I", schemaName)).Error; err != nil {
		if strings.Contains(err.Error(), "already exists") {
			return ErrProjectSchemaNameExists
		}
		return err
	}
	return nil
}

func (db *projects) DeleteSchema(ctx context.Context, projectID int64) error {
	project, err := db.GetByID(ctx, projectID)
	if err != nil {
		return errors.Wrap(err, "get by ID")
	}

	schemaName := project.SchemaName
	if schemaName == "public" {
		return errors.New("can't delete public schema")
	}

	return db.Transaction(func(tx *gorm.DB) error {

		// Drop all the tables in schema.
		if err := tx.Debug().Exec(`
do $$ declare
    r record;
begin
    for r in (select viewname, schemaname from pg_catalog.pg_views where schemaname = ?) loop
        execute 'drop view if exists ' || quote_ident(r.schemaname) || '.' || quote_ident(r.viewname) || ' cascade';
    end loop;
end $$;
`, schemaName).Error; err != nil {
			return errors.Wrap(err, "drop schema tables")
		}

		if err := tx.Exec(escape.Escape("DROP SCHEMA IF EXISTS %I", schemaName)).Error; err != nil {
			return errors.Wrap(err, "drop schema")
		}

		return nil
	})
}
