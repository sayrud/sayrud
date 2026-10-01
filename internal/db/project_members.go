package db

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wuhan005/sayrud/internal/dbutil"
)

// ProjectRole is the permission of a user on a project.
type ProjectRole string

const (
	// ProjectRoleOwner is the creator of the project, it is not stored in project_members.
	ProjectRoleOwner ProjectRole = "owner"
	// ProjectRoleManager can edit the project and manage the collaborators.
	ProjectRoleManager ProjectRole = "manager"
	// ProjectRoleEditor can edit the tables, fields, views and records.
	ProjectRoleEditor ProjectRole = "editor"
	// ProjectRoleViewer can only view the project.
	ProjectRoleViewer ProjectRole = "viewer"
)

var projectRoleLevels = map[ProjectRole]int{
	ProjectRoleViewer:  1,
	ProjectRoleEditor:  2,
	ProjectRoleManager: 3,
	ProjectRoleOwner:   4,
}

// AtLeast reports whether the role has all the permissions of min.
func (r ProjectRole) AtLeast(min ProjectRole) bool {
	return projectRoleLevels[r] > 0 && projectRoleLevels[r] >= projectRoleLevels[min]
}

// IsMemberRole reports whether the role can be granted to a collaborator.
func (r ProjectRole) IsMemberRole() bool {
	return r == ProjectRoleManager || r == ProjectRoleEditor || r == ProjectRoleViewer
}

var _ ProjectMembersStore = (*projectMembers)(nil)

// ProjectMembers is the default instance of the ProjectMembersStore.
var ProjectMembers ProjectMembersStore

// ProjectMembersStore is the persistent interface for the collaborators of projects.
type ProjectMembersStore interface {
	// ListByProjectID returns the collaborators of the project in the order of joining.
	ListByProjectID(ctx context.Context, projectID int64) ([]*ProjectMember, error)
	// GetRole returns the role of the collaborator.
	// It returns ErrProjectMemberNotFound if the user is not a collaborator of the project.
	GetRole(ctx context.Context, projectID, userID int64) (ProjectRole, error)
	// RolesByUserID returns the roles of the user in the given projects, keyed by project ID.
	RolesByUserID(ctx context.Context, userID int64, projectIDs []int64) (map[int64]ProjectRole, error)
	// Set adds the user as a collaborator of the project, or changes the role if it has been one.
	Set(ctx context.Context, projectID, userID int64, role ProjectRole) error
	// Delete removes the collaborator from the project.
	// It returns ErrProjectMemberNotFound if the user is not a collaborator of the project.
	Delete(ctx context.Context, projectID, userID int64) error
	// DeleteByProjectID removes all the collaborators of the project.
	DeleteByProjectID(ctx context.Context, projectID int64) error
	// CountByProjectIDs returns the number of collaborators (excluding the owner) keyed by project ID, the projects without collaborators are omitted.
	CountByProjectIDs(ctx context.Context, projectIDs []int64) (map[int64]int64, error)
}

func NewProjectMembersStore(db *gorm.DB) ProjectMembersStore {
	return &projectMembers{db}
}

// ProjectMember is a collaborator of a project, the owner is not included.
type ProjectMember struct {
	ID        int64       `gorm:"primarykey"`
	ProjectID int64       `gorm:"uniqueIndex:idx_project_members_project_user"`
	UserID    int64       `gorm:"uniqueIndex:idx_project_members_project_user;index"`
	Role      ProjectRole `gorm:"type:varchar(16)"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

type projectMembers struct {
	*gorm.DB
}

var ErrProjectMemberNotFound = errors.New("project member does not exist")

func (db *projectMembers) ListByProjectID(ctx context.Context, projectID int64) ([]*ProjectMember, error) {
	var members []*ProjectMember
	return members, db.WithContext(ctx).Where("project_id = ?", projectID).Order("id ASC").Find(&members).Error
}

func (db *projectMembers) GetRole(ctx context.Context, projectID, userID int64) (ProjectRole, error) {
	var member ProjectMember
	if err := db.WithContext(ctx).Where("project_id = ? AND user_id = ?", projectID, userID).First(&member).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrProjectMemberNotFound
		}
		return "", err
	}
	return member.Role, nil
}

func (db *projectMembers) RolesByUserID(ctx context.Context, userID int64, projectIDs []int64) (map[int64]ProjectRole, error) {
	roles := make(map[int64]ProjectRole, len(projectIDs))
	if len(projectIDs) == 0 {
		return roles, nil
	}
	var members []*ProjectMember
	if err := db.WithContext(ctx).Where("user_id = ? AND project_id IN ?", userID, projectIDs).Find(&members).Error; err != nil {
		return nil, err
	}
	for _, m := range members {
		roles[m.ProjectID] = m.Role
	}
	return roles, nil
}

func (db *projectMembers) Set(ctx context.Context, projectID, userID int64, role ProjectRole) error {
	now := dbutil.Now()
	return db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "project_id"}, {Name: "user_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{"role": role, "updated_at": now}),
	}).Create(&ProjectMember{
		ProjectID: projectID,
		UserID:    userID,
		Role:      role,
		CreatedAt: now,
		UpdatedAt: now,
	}).Error
}

func (db *projectMembers) Delete(ctx context.Context, projectID, userID int64) error {
	result := db.WithContext(ctx).Where("project_id = ? AND user_id = ?", projectID, userID).Delete(&ProjectMember{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrProjectMemberNotFound
	}
	return nil
}

func (db *projectMembers) DeleteByProjectID(ctx context.Context, projectID int64) error {
	return db.WithContext(ctx).Where("project_id = ?", projectID).Delete(&ProjectMember{}).Error
}

func (db *projectMembers) CountByProjectIDs(ctx context.Context, projectIDs []int64) (map[int64]int64, error) {
	counts := make(map[int64]int64, len(projectIDs))
	if len(projectIDs) == 0 {
		return counts, nil
	}
	var rows []struct {
		ProjectID int64
		Count     int64
	}
	if err := db.WithContext(ctx).Model(&ProjectMember{}).
		Select("project_id, COUNT(*) AS count").
		Where("project_id IN ?", projectIDs).
		Group("project_id").
		Scan(&rows).Error; err != nil {
		return nil, errors.Wrap(err, "count")
	}
	for _, r := range rows {
		counts[r.ProjectID] = r.Count
	}
	return counts, nil
}
