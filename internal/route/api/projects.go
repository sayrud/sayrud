// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package api

import (
	"net/http"

	"github.com/flamego/flamego"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/collab"
	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dbutil"
	"github.com/wuhan005/sayrud/internal/dto"
	"github.com/wuhan005/sayrud/internal/form"
)

var Project projectRoute

type projectRoute struct{}

// Projecter maps the project of the projectUID path parameter as *db.Project and the role of the signed-in user on it as db.ProjectRole.
func (projectRoute) Projecter(ctx context.Context, user *db.User) error {
	projectUID := ctx.Param("projectUID")
	project, err := db.Projects.GetByUID(ctx.Request().Context(), projectUID)
	if err != nil {
		if errors.Is(err, db.ErrProjectNotFound) {
			return ctx.ApiError(http.StatusNotFound, "project::not_found")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get project by UID")
		return ctx.ApiServerError()
	}

	role := db.ProjectRoleOwner
	if project.OwnerUserID != user.ID {
		role, err = db.ProjectMembers.GetRole(ctx.Request().Context(), project.ID, user.ID)
		if err != nil {
			if errors.Is(err, db.ErrProjectMemberNotFound) {
				return ctx.ApiError(http.StatusForbidden, "project::no_access")
			}
			logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get project role")
			return ctx.ApiServerError()
		}
	}

	ctx.Map(project)
	ctx.Map(role)
	return nil
}

var roleDeniedKeys = map[db.ProjectRole]string{
	db.ProjectRoleEditor:  "project::editor_required",
	db.ProjectRoleManager: "project::manager_required",
	db.ProjectRoleOwner:   "project::owner_required",
}

// RequireRole responds 403 unless the signed-in user has at least the given role on the project.
func (projectRoute) RequireRole(min db.ProjectRole) flamego.Handler {
	return func(ctx context.Context, role db.ProjectRole) error {
		if !role.AtLeast(min) {
			return ctx.ApiError(http.StatusForbidden, roleDeniedKeys[min])
		}
		return nil
	}
}

// ListProjects
// @Summary List projects
// @Description List the projects owned by or shared with the signed-in user, with the number of tables in each project.
// @Produce json
// @Param page query int false "Page number, starting from 1"
// @Param pageSize query int false "Page size, defaults to 20"
// @Param scope query string false "owned: created by the user, shared: shared with the user, both if empty" Enums(owned, shared)
// @Success 200 {object} dto.ListProjectsResp
// @Failure 401 {string} string "Not signed in"
// @Failure 500 {string} string "Internal server error"
// @ID listProjects
// @Router /projects [get]
func (projectRoute) ListProjects(ctx context.Context, user *db.User) error {
	projects, total, err := db.Projects.ListByUserID(ctx.Request().Context(), user.ID, db.ListByUserIDOptions{
		Pagination: dbutil.Pagination{
			Page:     ctx.QueryInt("page"),
			PageSize: ctx.QueryInt("pageSize"),
		},
		Scope: db.ProjectScope(ctx.Query("scope")),
	})
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to list projects")
		return ctx.ApiServerError()
	}

	projectIDs := make([]int64, 0, len(projects))
	ownerIDs := make([]int64, 0, len(projects))
	for _, project := range projects {
		projectIDs = append(projectIDs, project.ID)
		ownerIDs = append(ownerIDs, project.OwnerUserID)
	}
	roles, err := db.ProjectMembers.RolesByUserID(ctx.Request().Context(), user.ID, projectIDs)
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get project roles")
		return ctx.ApiServerError()
	}
	owners, err := usersByID(ctx, ownerIDs)
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to list project owners")
		return ctx.ApiServerError()
	}

	items := make([]*dto.ProjectListItem, 0, len(projects))
	for _, project := range projects {
		tableCount, err := db.SLTables.CountByProjectID(ctx.Request().Context(), project.ID)
		if err != nil {
			logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to count tables")
			return ctx.ApiServerError()
		}
		role := roles[project.ID]
		if project.OwnerUserID == user.ID {
			role = db.ProjectRoleOwner
		}
		items = append(items, &dto.ProjectListItem{
			Project:    *dto.ToProject(project, role, owners[project.OwnerUserID]),
			TableCount: tableCount,
		})
	}

	return ctx.ApiSuccess(dto.ListProjectsResp{
		Projects: items,
		Total:    total,
	})
}

func usersByID(ctx context.Context, userIDs []int64) (map[int64]*db.User, error) {
	users, err := db.Users.ListByIDs(ctx.Request().Context(), userIDs)
	if err != nil {
		return nil, err
	}
	m := make(map[int64]*db.User, len(users))
	for _, u := range users {
		m[u.ID] = u
	}
	return m, nil
}

// CreateProject
// @Summary Create a project
// @Description Create a project owned by the signed-in user.
// @Accept json
// @Produce json
// @Param data body form.CreateProject true "Project to create"
// @Success 200 {object} dto.Project
// @Failure 400 {string} string "Invalid request body"
// @Failure 401 {string} string "Not signed in"
// @Failure 500 {string} string "Internal server error"
// @ID createProject
// @Router /projects [post]
func (projectRoute) CreateProject(ctx context.Context, user *db.User, f form.CreateProject) error {
	project, err := db.Projects.Create(ctx.Request().Context(), db.CreateProjectOptions{
		OwnerUserID: user.ID,
		Name:        f.Name,
	})
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to create project")
		return ctx.ApiServerError()
	}

	return ctx.ApiSuccess(dto.ToProject(project, db.ProjectRoleOwner, user))
}

// GetProject
// @Summary Get a project
// @Produce json
// @Param projectUID path string true "Project UID"
// @Success 200 {object} dto.Project
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project not found"
// @Failure 500 {string} string "Internal server error"
// @ID getProject
// @Router /projects/{projectUID} [get]
func (projectRoute) GetProject(ctx context.Context, project *db.Project, role db.ProjectRole) error {
	owner, err := db.Users.GetByID(ctx.Request().Context(), project.OwnerUserID)
	if err != nil && !errors.Is(err, db.ErrUserNotFound) {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get project owner")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess(dto.ToProject(project, role, owner))
}

// UpdateProject
// @Summary Update a project
// @Description Requires the editor role.
// @Accept json
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param data body form.UpdateProject true "Project properties"
// @Success 204 "No Content"
// @Failure 400 {string} string "Invalid request body"
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project not found"
// @Failure 500 {string} string "Internal server error"
// @ID updateProject
// @Router /projects/{projectUID} [put]
func (projectRoute) UpdateProject(ctx context.Context, hub *collab.Hub, project *db.Project, f form.UpdateProject) error {
	if err := db.Projects.Update(ctx.Request().Context(), project.ID, db.UpdateProjectOptions{
		Name: f.Name,
	}); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to update project")
		return ctx.ApiServerError()
	}

	hub.NotifyProject(project.UID, collab.MessageProjectChange)
	return ctx.Status(http.StatusNoContent)
}

// DeleteProject
// @Summary Delete a project
// @Description Delete the project along with its collaborators, only the owner can delete it.
// @Produce json
// @Param projectUID path string true "Project UID"
// @Success 204 "No Content"
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project not found"
// @Failure 500 {string} string "Internal server error"
// @ID deleteProject
// @Router /projects/{projectUID} [delete]
func (projectRoute) DeleteProject(ctx context.Context, hub *collab.Hub, project *db.Project, tx dbutil.Transactor) error {
	if err := tx.Transaction(func(tx *gorm.DB) error {
		if err := db.NewProjectsStore(tx).DeleteByID(ctx.Request().Context(), project.ID); err != nil {
			return errors.Wrap(err, "delete project")
		}

		if err := db.NewProjectMembersStore(tx).DeleteByProjectID(ctx.Request().Context(), project.ID); err != nil {
			return errors.Wrap(err, "delete project members")
		}

		// TODO: the tables, fields, records are not deleted.
		return nil
	}); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to delete project")
		return ctx.ApiServerError()
	}

	hub.NotifyProject(project.UID, collab.MessageProjectChange)
	return ctx.Status(http.StatusNoContent)
}

// TransferOwner
// @Summary Transfer the ownership of a project
// @Description Only the owner can transfer it to an existing collaborator, the previous owner becomes a manager.
// @Accept json
// @Param projectUID path string true "Project UID"
// @Param data body form.TransferProjectOwner true "New owner"
// @Success 204 "No Content"
// @Failure 400 {string} string "The user is not a collaborator or is disabled"
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project not found"
// @Failure 500 {string} string "Internal server error"
// @ID transferProjectOwner
// @Router /projects/{projectUID}/owner [put]
func (projectRoute) TransferOwner(ctx context.Context, hub *collab.Hub, project *db.Project, f form.TransferProjectOwner) error {
	if _, err := db.ProjectMembers.GetRole(ctx.Request().Context(), project.ID, f.UserID); err != nil {
		if errors.Is(err, db.ErrProjectMemberNotFound) {
			return ctx.ApiError(http.StatusBadRequest, "project::transfer_to_collaborator")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get project role")
		return ctx.ApiServerError()
	}
	if msg, err := transferTargetError(ctx, f.UserID); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get new owner")
		return ctx.ApiServerError()
	} else if msg != "" {
		return ctx.ApiError(http.StatusBadRequest, msg)
	}

	if err := transferProject(ctx, hub, project, f.UserID); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to transfer project")
		return ctx.ApiServerError()
	}
	return ctx.Status(http.StatusNoContent)
}

// transferTargetError returns the message key if the user can not receive projects, or an empty string.
func transferTargetError(ctx context.Context, userID int64) (string, error) {
	user, err := db.Users.GetByID(ctx.Request().Context(), userID)
	if err != nil {
		if errors.Is(err, db.ErrUserNotFound) {
			return "project::recipient_not_found", nil
		}
		return "", err
	}
	if user.Disabled() {
		return "project::recipient_disabled", nil
	}
	return "", nil
}

// transferProject transfers the ownership and applies the new roles to the online connections.
func transferProject(ctx context.Context, hub *collab.Hub, project *db.Project, newOwnerID int64) error {
	if err := db.Projects.TransferOwner(ctx.Request().Context(), project.ID, newOwnerID); err != nil {
		return err
	}
	if project.OwnerUserID != newOwnerID {
		hub.SetUserRole(project.UID, newOwnerID, db.ProjectRoleOwner)
		hub.SetUserRole(project.UID, project.OwnerUserID, db.ProjectRoleManager)
	}
	hub.NotifyProject(project.UID, collab.MessageProjectChange)
	return nil
}

// ListMembers
// @Summary List the collaborators
// @Description List the owner and the collaborators of the project, the owner comes first.
// @Produce json
// @Param projectUID path string true "Project UID"
// @Success 200 {array} dto.ProjectMember
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project not found"
// @Failure 500 {string} string "Internal server error"
// @ID listProjectMembers
// @Router /projects/{projectUID}/members [get]
func (projectRoute) ListMembers(ctx context.Context, project *db.Project) error {
	members, err := db.ProjectMembers.ListByProjectID(ctx.Request().Context(), project.ID)
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to list project members")
		return ctx.ApiServerError()
	}

	userIDs := []int64{project.OwnerUserID}
	for _, m := range members {
		userIDs = append(userIDs, m.UserID)
	}
	users, err := usersByID(ctx, userIDs)
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to list member users")
		return ctx.ApiServerError()
	}

	resp := make([]*dto.ProjectMember, 0, len(members)+1)
	if owner, ok := users[project.OwnerUserID]; ok {
		resp = append(resp, &dto.ProjectMember{User: dto.ToUserBrief(owner), Role: string(db.ProjectRoleOwner)})
	}
	for _, m := range members {
		if u, ok := users[m.UserID]; ok {
			resp = append(resp, &dto.ProjectMember{User: dto.ToUserBrief(u), Role: string(m.Role)})
		}
	}
	return ctx.ApiSuccess(resp)
}

// LookupMemberCandidate
// @Summary Find a user to invite
// @Description Find the registered user by the exact email, requires the manager role.
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param email query string true "Email of the user"
// @Success 200 {object} dto.UserBrief
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "User not found"
// @Failure 500 {string} string "Internal server error"
// @ID lookupProjectMemberCandidate
// @Router /projects/{projectUID}/members/lookup [get]
func (projectRoute) LookupMemberCandidate(ctx context.Context) error {
	user, err := db.Users.GetByEmail(ctx.Request().Context(), ctx.Query("email"))
	if err != nil {
		if errors.Is(err, db.ErrUserNotFound) {
			return ctx.ApiError(http.StatusNotFound, "project::email_not_registered")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get user by email")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess(dto.ToUserBrief(user))
}

// AddMember
// @Summary Add a collaborator
// @Description Add the registered user as a collaborator, or change the role if it has been one. Requires the manager role.
// @Accept json
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param data body form.AddProjectMember true "Collaborator"
// @Success 200 {object} dto.ProjectMember
// @Failure 400 {string} string "Invalid request body"
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "User not found"
// @Failure 500 {string} string "Internal server error"
// @ID addProjectMember
// @Router /projects/{projectUID}/members [post]
func (projectRoute) AddMember(ctx context.Context, hub *collab.Hub, project *db.Project, f form.AddProjectMember) error {
	role := db.ProjectRole(f.Role)
	if !role.IsMemberRole() {
		return ctx.ApiError(http.StatusBadRequest, "project::unsupported_role")
	}
	user, err := db.Users.GetByEmail(ctx.Request().Context(), f.Email)
	if err != nil {
		if errors.Is(err, db.ErrUserNotFound) {
			return ctx.ApiError(http.StatusNotFound, "project::email_not_registered")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get user by email")
		return ctx.ApiServerError()
	}
	if user.ID == project.OwnerUserID {
		return ctx.ApiError(http.StatusBadRequest, "project::user_is_owner")
	}

	if err := db.ProjectMembers.Set(ctx.Request().Context(), project.ID, user.ID, role); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to add project member")
		return ctx.ApiServerError()
	}
	hub.SetUserRole(project.UID, user.ID, role)
	return ctx.ApiSuccess(&dto.ProjectMember{User: dto.ToUserBrief(user), Role: string(role)})
}

// UpdateMember
// @Summary Change the role of a collaborator
// @Description Requires the manager role, the owner and the signed-in user itself can not be changed.
// @Accept json
// @Param projectUID path string true "Project UID"
// @Param userID path int true "User ID of the collaborator"
// @Param data body form.UpdateProjectMember true "Role"
// @Success 204 "No Content"
// @Failure 400 {string} string "Invalid request body"
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Collaborator not found"
// @Failure 500 {string} string "Internal server error"
// @ID updateProjectMember
// @Router /projects/{projectUID}/members/{userID} [put]
func (projectRoute) UpdateMember(ctx context.Context, hub *collab.Hub, user *db.User, project *db.Project, f form.UpdateProjectMember) error {
	role := db.ProjectRole(f.Role)
	if !role.IsMemberRole() {
		return ctx.ApiError(http.StatusBadRequest, "project::unsupported_role")
	}
	userID := ctx.ParamInt64("userID")
	if userID == project.OwnerUserID {
		return ctx.ApiError(http.StatusBadRequest, "project::cannot_change_owner_role")
	}
	if userID == user.ID {
		return ctx.ApiError(http.StatusBadRequest, "project::cannot_change_own_role")
	}

	if _, err := db.ProjectMembers.GetRole(ctx.Request().Context(), project.ID, userID); err != nil {
		if errors.Is(err, db.ErrProjectMemberNotFound) {
			return ctx.ApiError(http.StatusNotFound, "project::collaborator_not_found")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get project role")
		return ctx.ApiServerError()
	}
	if err := db.ProjectMembers.Set(ctx.Request().Context(), project.ID, userID, role); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to update project member")
		return ctx.ApiServerError()
	}
	hub.SetUserRole(project.UID, userID, role)
	return ctx.Status(http.StatusNoContent)
}

// RemoveMember
// @Summary Remove a collaborator
// @Description Requires the manager role, except that a collaborator can remove itself to leave the project. The owner can not be removed.
// @Param projectUID path string true "Project UID"
// @Param userID path int true "User ID of the collaborator"
// @Success 204 "No Content"
// @Failure 400 {string} string "The owner can not be removed"
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Collaborator not found"
// @Failure 500 {string} string "Internal server error"
// @ID removeProjectMember
// @Router /projects/{projectUID}/members/{userID} [delete]
func (projectRoute) RemoveMember(ctx context.Context, hub *collab.Hub, user *db.User, project *db.Project, role db.ProjectRole) error {
	userID := ctx.ParamInt64("userID")
	if userID == project.OwnerUserID {
		return ctx.ApiError(http.StatusBadRequest, "project::cannot_remove_owner")
	}
	if userID != user.ID && !role.AtLeast(db.ProjectRoleManager) {
		return ctx.ApiError(http.StatusForbidden, roleDeniedKeys[db.ProjectRoleManager])
	}

	if err := db.ProjectMembers.Delete(ctx.Request().Context(), project.ID, userID); err != nil {
		if errors.Is(err, db.ErrProjectMemberNotFound) {
			return ctx.ApiError(http.StatusNotFound, "project::collaborator_not_found")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to remove project member")
		return ctx.ApiServerError()
	}
	hub.SetUserRole(project.UID, userID, "")
	return ctx.Status(http.StatusNoContent)
}
