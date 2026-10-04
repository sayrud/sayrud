// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package db

import (
	"context"
	"strings"

	"github.com/pkg/errors"
	"github.com/thanhpk/randstr"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wuhan005/sayrud/internal/dbutil"
)

var _ ProjectsStore = (*projects)(nil)

// Projects is the default instance of the ProjectsStore.
var Projects ProjectsStore

// ProjectsStore is the persistent interface for projects.
type ProjectsStore interface {
	// ListByUserID returns the paginated projects owned by or shared with the user, along with the total count.
	ListByUserID(ctx context.Context, userID int64, options ListByUserIDOptions) ([]*Project, int64, error)
	// GetByID returns the project with the given ID.
	// It returns ErrProjectNotFound if the project does not exist.
	GetByID(ctx context.Context, projectID int64) (*Project, error)
	// GetByUID returns the project with the given UID.
	// It returns ErrProjectNotFound if the project does not exist.
	GetByUID(ctx context.Context, projectUID string) (*Project, error)
	// Create creates a new project with the given options.
	Create(ctx context.Context, opts CreateProjectOptions) (*Project, error)
	// Update updates the project with the given ID.
	// It returns ErrProjectNotFound if the project does not exist.
	Update(ctx context.Context, projectID int64, opts UpdateProjectOptions) error
	// DeleteByID deletes the project with the given ID.
	DeleteByID(ctx context.Context, projectID int64) error
	// ListAll returns the paginated projects of the whole site along with the total count, the newest first.
	ListAll(ctx context.Context, opts ListAllProjectsOptions) ([]*Project, int64, error)
	// Count returns the number of projects of the whole site.
	Count(ctx context.Context) (int64, error)
	// CountByOwnerIDs returns the number of owned projects keyed by owner ID, the users without projects are omitted.
	CountByOwnerIDs(ctx context.Context, ownerIDs []int64) (map[int64]int64, error)
	// TransferOwner transfers the project to newOwnerID: its collaborator record is removed, and the previous owner becomes a manager if it still exists.
	// It returns ErrProjectNotFound if the project does not exist.
	TransferOwner(ctx context.Context, projectID, newOwnerID int64) error
}

func NewProjectsStore(db *gorm.DB) ProjectsStore {
	return &projects{db}
}

// Project is a collection of tables owned by a user.
type Project struct {
	// Model contains the primary key and the creation, update and deletion times.
	dbutil.Model
	// UID is the unique public identifier of the project, it is generated when creating.
	UID string `gorm:"uniqueIndex:idx_projects_uid, where:deleted_at IS NULL"`
	// OwnerUserID is the ID of the user who owns the project.
	OwnerUserID int64 `json:"-"`
	// Name is the display name of the project.
	Name string

	// Icon is the optional icon identifier displayed with the project name.
	Icon string `gorm:"not null;default:''"`
	// Color is the optional palette color of the project icon.
	Color string `gorm:"not null;default:''"`
}

func (project *Project) BeforeCreate(_ *gorm.DB) error {
	project.UID = "prj" + randstr.String(10)
	return nil
}

type projects struct {
	// DB is the database connection the store operates on.
	*gorm.DB
}

// ProjectScope selects the projects of a user by ownership.
type ProjectScope string

const (
	ProjectScopeAll    ProjectScope = ""
	ProjectScopeOwned  ProjectScope = "owned"
	ProjectScopeShared ProjectScope = "shared"
)

// ListByUserIDOptions are the options of listing the projects of a user.
type ListByUserIDOptions struct {
	// Pagination is the page and page size of the list.
	dbutil.Pagination
	// Scope selects the owned or shared projects, both are listed if empty.
	Scope ProjectScope
}

func (db *projects) ListByUserID(ctx context.Context, userID int64, options ListByUserIDOptions) ([]*Project, int64, error) {
	var total int64
	shared := db.WithContext(ctx).Model(&ProjectMember{}).Select("project_id").Where("user_id = ?", userID)
	q := db.WithContext(ctx).Model(&Project{})
	switch options.Scope {
	case ProjectScopeOwned:
		q = q.Where("owner_user_id = ?", userID)
	case ProjectScopeShared:
		q = q.Where("id IN (?)", shared)
	default:
		q = q.Where("owner_user_id = ? OR id IN (?)", userID, shared)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, errors.Wrap(err, "count")
	}

	limit, offset := dbutil.LimitOffset(options.Page, options.PageSize)
	var projects []*Project
	return projects, total, q.Order("id ASC").Limit(limit).Offset(offset).Find(&projects).Error
}

func (db *projects) GetByID(ctx context.Context, projectID int64) (*Project, error) {
	return db.getBy(ctx, "id = ?", projectID)
}

func (db *projects) GetByUID(ctx context.Context, projectUID string) (*Project, error) {
	return db.getBy(ctx, "uid = ?", projectUID)
}

// CreateProjectOptions are the options of creating a project.
type CreateProjectOptions struct {
	// OwnerUserID is the ID of the user who owns the project.
	OwnerUserID int64
	// Name is the display name of the project.
	Name string
}

func (db *projects) Create(ctx context.Context, opts CreateProjectOptions) (*Project, error) {
	project := &Project{
		OwnerUserID: opts.OwnerUserID,
		Name:        opts.Name,
	}
	if err := db.WithContext(ctx).Create(project).Error; err != nil {
		return nil, err
	}
	return project, nil
}

// UpdateProjectOptions are the options of updating a project.
type UpdateProjectOptions struct {
	// Name is the new display name of the project.
	Name *string

	// Icon and Color replace the appearance when provided, empty strings clear it.
	Icon  *string
	Color *string
}

func (db *projects) Update(ctx context.Context, projectID int64, opts UpdateProjectOptions) error {
	if _, err := db.GetByID(ctx, projectID); err != nil {
		return err
	}

	updates := map[string]interface{}{"updated_at": dbutil.Now()}
	if opts.Name != nil {
		updates["name"] = *opts.Name
	}

	if opts.Icon != nil {
		updates["icon"] = *opts.Icon
	}
	if opts.Color != nil {
		updates["color"] = *opts.Color
	}

	return db.WithContext(ctx).Model(&Project{}).Where("id = ?", projectID).Updates(updates).Error
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

// ListAllProjectsOptions are the options of listing all the projects.
type ListAllProjectsOptions struct {
	dbutil.Pagination
	// Keyword matches the name case-insensitively.
	Keyword string
}

func (db *projects) ListAll(ctx context.Context, opts ListAllProjectsOptions) ([]*Project, int64, error) {
	q := db.WithContext(ctx).Model(&Project{})
	if k := strings.TrimSpace(opts.Keyword); k != "" {
		q = q.Where("LOWER(name) LIKE ?", "%"+dbutil.EscapeLike(strings.ToLower(k))+"%")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, errors.Wrap(err, "count")
	}
	limit, offset := opts.LimitOffset()
	var projects []*Project
	return projects, total, q.Order("id DESC").Limit(limit).Offset(offset).Find(&projects).Error
}

func (db *projects) Count(ctx context.Context) (int64, error) {
	var count int64
	return count, db.WithContext(ctx).Model(&Project{}).Count(&count).Error
}

func (db *projects) CountByOwnerIDs(ctx context.Context, ownerIDs []int64) (map[int64]int64, error) {
	counts := make(map[int64]int64, len(ownerIDs))
	if len(ownerIDs) == 0 {
		return counts, nil
	}
	var rows []struct {
		OwnerUserID int64
		Count       int64
	}
	if err := db.WithContext(ctx).Model(&Project{}).
		Select("owner_user_id, COUNT(*) AS count").
		Where("owner_user_id IN ?", ownerIDs).
		Group("owner_user_id").
		Scan(&rows).Error; err != nil {
		return nil, errors.Wrap(err, "count")
	}
	for _, r := range rows {
		counts[r.OwnerUserID] = r.Count
	}
	return counts, nil
}

func (db *projects) TransferOwner(ctx context.Context, projectID, newOwnerID int64) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var project Project
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", projectID).First(&project).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrProjectNotFound
			}
			return err
		}
		oldOwnerID := project.OwnerUserID
		if oldOwnerID == newOwnerID {
			return nil
		}

		if err := tx.Where("project_id = ? AND user_id = ?", projectID, newOwnerID).Delete(&ProjectMember{}).Error; err != nil {
			return errors.Wrap(err, "delete membership of the new owner")
		}
		if err := tx.Model(&Project{}).Where("id = ?", projectID).Update("owner_user_id", newOwnerID).Error; err != nil {
			return errors.Wrap(err, "update owner")
		}

		var oldOwnerCount int64
		if err := tx.Model(&User{}).Where("id = ?", oldOwnerID).Count(&oldOwnerCount).Error; err != nil {
			return errors.Wrap(err, "check old owner")
		}
		if oldOwnerCount == 0 {
			return nil
		}
		return NewProjectMembersStore(tx).Set(ctx, projectID, oldOwnerID, ProjectRoleManager)
	})
}
