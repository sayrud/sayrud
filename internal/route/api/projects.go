// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package api

import (
	"net/http"

	"github.com/cockroachdb/errors"
	"github.com/flamego/flamego"
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

// projectAccess describes how the signed-in user accesses the project.
type projectAccess struct {
	// ViaAdmin reports whether the effective role is raised by the system admin.
	ViaAdmin bool
}

// Projecter maps the project of the projectUID path parameter as *db.Project, the effective role of the signed-in user on it as db.ProjectRole, and how the role is granted as projectAccess.
// An access token must allow the project and its role is never raised by the system admin, a bot never exceeds the own role of its owner.
func (projectRoute) Projecter(ctx context.Context, user *db.User, token *db.AccessToken) error {
	projectUID := ctx.Param("projectUID")
	project, err := db.Projects.GetByUID(ctx.Request().Context(), projectUID)
	if err != nil {
		if errors.Is(err, db.ErrProjectNotFound) {
			return ctx.ApiError(http.StatusNotFound, "project::not_found")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get project by UID")
		return ctx.ApiServerError()
	}
	if token != nil && !token.AllowsProject(project.ID) {
		return ctx.ApiError(http.StatusForbidden, "auth::token_project_denied")
	}

	ownRole, err := ownProjectRole(ctx, project, user.ID)
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get project role")
		return ctx.ApiServerError()
	}
	role := db.EffectiveRole(ownRole, user.IsAdmin && token == nil)
	if role == "" {
		return ctx.ApiError(http.StatusForbidden, "project::no_access")
	}
	if user.IsBot() {
		ownerRole, err := ownProjectRole(ctx, project, user.OwnerID)
		if err != nil {
			logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get project role of bot owner")
			return ctx.ApiServerError()
		}
		if role = db.LowerRole(role, ownerRole); role == "" {
			return ctx.ApiError(http.StatusForbidden, "project::bot_owner_no_access")
		}
		ownRole = role
	}

	ctx.Map(project)
	ctx.Map(role)
	ctx.Map(projectAccess{ViaAdmin: role != ownRole})
	return nil
}

// ownProjectRole returns the role of the user as the owner or a collaborator of the project, empty if neither.
func ownProjectRole(ctx context.Context, project *db.Project, userID int64) (db.ProjectRole, error) {
	if project.OwnerUserID == userID {
		return db.ProjectRoleOwner, nil
	}
	role, err := db.ProjectMembers.GetRole(ctx.Request().Context(), project.ID, userID)
	if errors.Is(err, db.ErrProjectMemberNotFound) {
		return "", nil
	}
	return role, err
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
// @Tags Projects
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
func (projectRoute) ListProjects(ctx context.Context, user *db.User, token *db.AccessToken) error {
	var restrictTo []int64
	if token != nil && len(token.ProjectIDs) > 0 {
		restrictTo = token.ProjectIDs
	}
	var ownerID int64
	if user.IsBot() {
		ownerID = user.OwnerID
	}
	projects, total, err := db.Projects.ListByUserID(ctx.Request().Context(), user.ID, db.ListByUserIDOptions{
		Pagination: dbutil.Pagination{
			Page:     ctx.QueryInt("page"),
			PageSize: ctx.QueryInt("pageSize"),
		},
		Scope:            db.ProjectScope(ctx.Query("scope")),
		IDs:              restrictTo,
		AlsoAccessibleBy: ownerID,
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
// @Tags Projects
// @Description Create a project owned by the signed-in user. Bots can not own projects.
// @Accept json
// @Produce json
// @Param data body form.CreateProject true "Project to create"
// @Success 200 {object} dto.Project
// @Failure 400 {string} string "Invalid request body"
// @Failure 401 {string} string "Not signed in"
// @Failure 403 {string} string "Called by a bot"
// @Failure 500 {string} string "Internal server error"
// @ID createProject
// @Router /projects [post]
func (projectRoute) CreateProject(ctx context.Context, user *db.User, f form.CreateProject) error {
	if user.IsBot() {
		return ctx.ApiError(http.StatusForbidden, "project::bot_cannot_own")
	}
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
// @Tags Projects
// @Description System admins can open every project with at least the manager role, adminAccess is true if the role is granted by the admin.
// @Produce json
// @Param projectUID path string true "Project UID"
// @Success 200 {object} dto.Project
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project not found"
// @Failure 500 {string} string "Internal server error"
// @ID getProject
// @Router /projects/{projectUID} [get]
func (projectRoute) GetProject(ctx context.Context, project *db.Project, role db.ProjectRole, access projectAccess) error {
	owner, err := db.Users.GetByID(ctx.Request().Context(), project.OwnerUserID)
	if err != nil && !errors.Is(err, db.ErrUserNotFound) {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get project owner")
		return ctx.ApiServerError()
	}
	resp := dto.ToProject(project, role, owner)
	resp.AdminAccess = access.ViaAdmin
	return ctx.ApiSuccess(resp)
}

// UpdateProject
// @Summary Update a project
// @Tags Projects
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
	if !validAppearanceUpdate(f.Name, f.Icon, f.Color) {
		return ctx.ApiError(http.StatusBadRequest, "common::invalid_body")
	}

	if err := db.Projects.Update(ctx.Request().Context(), project.ID, db.UpdateProjectOptions{
		Name:  f.Name,
		Icon:  f.Icon,
		Color: f.Color,
	}); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to update project")
		return ctx.ApiServerError()
	}

	hub.NotifyProject(project.UID, collab.MessageProjectChange)
	return ctx.Status(http.StatusNoContent)
}

// DeleteProject
// @Summary Delete a project
// @Tags Projects
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
// @Tags Projects
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
	if user.IsBot() {
		return "project::bot_cannot_own", nil
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
// @Tags Project members
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
	users, err := db.Users.ListByIDs(ctx.Request().Context(), userIDs)
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to list member users")
		return ctx.ApiServerError()
	}
	briefs, err := userBriefs(ctx, users)
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to list owners of member bots")
		return ctx.ApiServerError()
	}

	resp := make([]*dto.ProjectMember, 0, len(members)+1)
	if owner, ok := briefs[project.OwnerUserID]; ok {
		resp = append(resp, &dto.ProjectMember{User: owner, Role: string(db.ProjectRoleOwner)})
	}
	for _, m := range members {
		if u, ok := briefs[m.UserID]; ok {
			resp = append(resp, &dto.ProjectMember{User: u, Role: string(m.Role)})
		}
	}
	return ctx.ApiSuccess(resp)
}

// userBriefs converts the users keyed by ID, naming the owners of the bots.
func userBriefs(ctx context.Context, users []*db.User) (map[int64]*dto.UserBrief, error) {
	var ownerIDs []int64
	for _, u := range users {
		if u.IsBot() {
			ownerIDs = append(ownerIDs, u.OwnerID)
		}
	}
	owners, err := usersByID(ctx, ownerIDs)
	if err != nil {
		return nil, err
	}
	briefs := make(map[int64]*dto.UserBrief, len(users))
	for _, u := range users {
		brief := dto.ToUserBrief(u)
		if owner, ok := owners[u.OwnerID]; ok && u.IsBot() {
			brief.OwnerName = owner.UserName
		}
		briefs[u.ID] = brief
	}
	return briefs, nil
}

// visibleCandidate reports whether the operator may find and invite the user, the bots are only visible to their owners.
func visibleCandidate(operator, user *db.User) bool {
	return !user.IsBot() || user.OwnerID == operator.ID
}

// LookupMemberCandidate
// @Summary Find a user to invite
// @Tags Project members
// @Description Find the registered user by the exact email, requires the manager role. The bots of the others are not found.
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param email query string true "Email of the user"
// @Success 200 {object} dto.UserBrief
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "User not found"
// @Failure 500 {string} string "Internal server error"
// @ID lookupProjectMemberCandidate
// @Router /projects/{projectUID}/members/lookup [get]
func (projectRoute) LookupMemberCandidate(ctx context.Context, operator *db.User) error {
	user, err := db.Users.GetByEmail(ctx.Request().Context(), ctx.Query("email"))
	if err != nil && !errors.Is(err, db.ErrUserNotFound) {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get user by email")
		return ctx.ApiServerError()
	}
	if user == nil || !visibleCandidate(operator, user) {
		return ctx.ApiError(http.StatusNotFound, "project::email_not_registered")
	}
	brief := dto.ToUserBrief(user)
	if user.IsBot() {
		brief.OwnerName = operator.UserName
	}
	return ctx.ApiSuccess(brief)
}

// SearchMemberBots
// @Summary Find the bots to invite
// @Tags Project members
// @Description Search the own enabled bots by name or email, at most 20, requires the manager role. Add them by the email like the other users.
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param keyword query string false "Matches the name or email of the bot"
// @Success 200 {array} dto.UserBrief
// @Failure 403 {string} string "Permission denied"
// @Failure 500 {string} string "Internal server error"
// @ID searchProjectMemberBots
// @Router /projects/{projectUID}/members/bots [get]
func (projectRoute) SearchMemberBots(ctx context.Context, operator *db.User) error {
	bots, _, err := db.Users.List(ctx.Request().Context(), db.ListUsersOptions{
		Pagination: dbutil.Pagination{PageSize: 20},
		Keyword:    ctx.Query("keyword"),
		Status:     db.UserStatusActive,
		Kind:       db.UserKindBot,
		OwnerID:    operator.ID,
	})
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to search bots")
		return ctx.ApiServerError()
	}
	resp := make([]*dto.UserBrief, 0, len(bots))
	for _, b := range bots {
		brief := dto.ToUserBrief(b)
		if b.IsBot() {
			brief.OwnerName = operator.UserName
		}
		resp = append(resp, brief)
	}
	return ctx.ApiSuccess(resp)
}

// AddMember
// @Summary Add a collaborator
// @Tags Project members
// @Description Add the registered user as a collaborator, or change the role if it has been one. Requires the manager role, a bot can only be added by its owner.
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
func (projectRoute) AddMember(ctx context.Context, hub *collab.Hub, operator *db.User, project *db.Project, f form.AddProjectMember) error {
	role := db.ProjectRole(f.Role)
	if !role.IsMemberRole() {
		return ctx.ApiError(http.StatusBadRequest, "project::unsupported_role")
	}
	user, err := db.Users.GetByEmail(ctx.Request().Context(), f.Email)
	if err != nil && !errors.Is(err, db.ErrUserNotFound) {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get user by email")
		return ctx.ApiServerError()
	}
	if user == nil || !visibleCandidate(operator, user) {
		return ctx.ApiError(http.StatusNotFound, "project::email_not_registered")
	}
	if user.ID == project.OwnerUserID {
		return ctx.ApiError(http.StatusBadRequest, "project::user_is_owner")
	}

	if err := db.ProjectMembers.Set(ctx.Request().Context(), project.ID, user.ID, role); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to add project member")
		return ctx.ApiServerError()
	}
	hub.SetUserRole(project.UID, user.ID, db.EffectiveRole(role, user.IsAdmin))
	brief := dto.ToUserBrief(user)
	if user.IsBot() {
		brief.OwnerName = operator.UserName
	}
	return ctx.ApiSuccess(&dto.ProjectMember{User: brief, Role: string(role)})
}

// UpdateMember
// @Summary Change the role of a collaborator
// @Tags Project members
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
	notifyMemberRole(ctx, hub, project, userID, role)
	return ctx.Status(http.StatusNoContent)
}

// notifyMemberRole applies the new role of the collaborator to its online connections, an empty role means it is removed. The role of system admins is raised by db.EffectiveRole.
func notifyMemberRole(ctx context.Context, hub *collab.Hub, project *db.Project, userID int64, role db.ProjectRole) {
	user, err := db.Users.GetByID(ctx.Request().Context(), userID)
	if err == nil {
		role = db.EffectiveRole(role, user.IsAdmin)
	} else if !errors.Is(err, db.ErrUserNotFound) {
		// The notification is a hint, the connections recheck the access on the next message or heartbeat.
		logrus.WithContext(ctx.Request().Context()).WithError(err).Warn("Failed to get collaborator for role notification")
	}
	hub.SetUserRole(project.UID, userID, role)
}

// RemoveMember
// @Summary Remove a collaborator
// @Tags Project members
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
	notifyMemberRole(ctx, hub, project, userID, "")
	return ctx.Status(http.StatusNoContent)
}
