package api

import (
	"net/http"
	"runtime"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/wuhan005/sayrud/internal/appconst"
	"github.com/wuhan005/sayrud/internal/collab"
	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dbutil"
	"github.com/wuhan005/sayrud/internal/dto"
	"github.com/wuhan005/sayrud/internal/form"
)

var Admin adminRoute

type adminRoute struct{}

// adminTarget is the user operated by the admin, mapped apart from the signed-in *db.User.
type adminTarget struct {
	*db.User
}

const lastAdminMessage = "至少需要保留一位启用中的管理员"

// selfOperationError returns the message if the admin performs the action on itself, or an empty string.
func selfOperationError(operatorID, targetID int64, action string) string {
	if operatorID == targetID {
		return "不能" + action + "自己的账号"
	}
	return ""
}

// RequireAdmin responds 403 unless the signed-in user is an admin.
func (adminRoute) RequireAdmin(ctx context.Context, user *db.User) error {
	if !user.IsAdmin {
		return ctx.ApiError(http.StatusForbidden, "只有管理员才能访问管理后台")
	}
	return nil
}

// Targeter maps the user of the userID path parameter as *adminTarget, and responds 404 if not found.
func (adminRoute) Targeter(ctx context.Context) error {
	user, err := db.Users.GetByID(ctx.Request().Context(), ctx.ParamInt64("userID"))
	if err != nil {
		if errors.Is(err, db.ErrUserNotFound) {
			return ctx.ApiError(http.StatusNotFound, "成员不存在")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get target user")
		return ctx.ApiServerError()
	}
	ctx.Map(&adminTarget{User: user})
	return nil
}

// Projecter maps the project of the projectUID path parameter as *db.Project, and responds 404 if not found.
func (adminRoute) Projecter(ctx context.Context) error {
	project, err := db.Projects.GetByUID(ctx.Request().Context(), ctx.Param("projectUID"))
	if err != nil {
		if errors.Is(err, db.ErrProjectNotFound) {
			return ctx.ApiError(http.StatusNotFound, "多维表格不存在")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get project by UID")
		return ctx.ApiServerError()
	}
	ctx.Map(project)
	return nil
}

// Overview
// @Summary Get the overview of the system
// @Description Requires the admin.
// @Produce json
// @Success 200 {object} dto.AdminOverview
// @Failure 403 {string} string "Not an admin"
// @Failure 500 {string} string "Internal server error"
// @ID getAdminOverview
// @Router /admin/overview [get]
func (adminRoute) Overview(ctx context.Context) error {
	c := ctx.Request().Context()
	resp := dto.AdminOverview{}

	stats, err := db.Users.Stats(c)
	if err != nil {
		logrus.WithContext(c).WithError(err).Error("Failed to get user stats")
		return ctx.ApiServerError()
	}
	resp.Users = dto.AdminUserStats{Total: stats.Total, Disabled: stats.Disabled, Admins: stats.Admins, NewLast7Days: stats.NewLast7Days}

	for _, item := range []struct {
		dst   *int64
		count func() (int64, error)
	}{
		{&resp.Projects, func() (int64, error) { return db.Projects.Count(c) }},
		{&resp.Tables, func() (int64, error) { return db.SLTables.Count(c) }},
		{&resp.Records, func() (int64, error) { return db.SLRecords.Count(c) }},
		{&resp.ActiveSessions, func() (int64, error) { return db.UserSessions.CountActive(c) }},
	} {
		if *item.dst, err = item.count(); err != nil {
			logrus.WithContext(c).WithError(err).Error("Failed to count for overview")
			return ctx.ApiServerError()
		}
	}

	recent, err := db.Users.Recent(c, 5)
	if err != nil {
		logrus.WithContext(c).WithError(err).Error("Failed to list recent users")
		return ctx.ApiServerError()
	}
	if resp.RecentUsers, err = toAdminUsers(ctx, recent); err != nil {
		logrus.WithContext(c).WithError(err).Error("Failed to count owned projects")
		return ctx.ApiServerError()
	}

	settings, ok := loadSettings(ctx)
	if !ok {
		return nil
	}
	resp.System = dto.AdminSystemInfo{BuildCommit: appconst.BuildCommit, GoVersion: runtime.Version(), Settings: settings}
	return ctx.ApiSuccess(resp)
}

func toAdminUsers(ctx context.Context, users []*db.User) ([]*dto.AdminUser, error) {
	ids := make([]int64, 0, len(users))
	for _, u := range users {
		ids = append(ids, u.ID)
	}
	counts, err := db.Projects.CountByOwnerIDs(ctx.Request().Context(), ids)
	if err != nil {
		return nil, err
	}
	items := make([]*dto.AdminUser, 0, len(users))
	for _, u := range users {
		items = append(items, dto.ToAdminUser(u, counts[u.ID]))
	}
	return items, nil
}

// ListUsers
// @Summary List the users
// @Description Requires the admin. The newest users come first.
// @Produce json
// @Param page query int false "Page number, starting from 1"
// @Param pageSize query int false "Page size, defaults to 20"
// @Param keyword query string false "Matches the user name or email"
// @Param status query string false "Filter by status, all if empty" Enums(active, disabled, admin)
// @Success 200 {object} dto.ListAdminUsersResp
// @Failure 403 {string} string "Not an admin"
// @Failure 500 {string} string "Internal server error"
// @ID listAdminUsers
// @Router /admin/users [get]
func (adminRoute) ListUsers(ctx context.Context) error {
	users, total, err := db.Users.List(ctx.Request().Context(), db.ListUsersOptions{
		Pagination: dbutil.Pagination{Page: ctx.QueryInt("page"), PageSize: ctx.QueryInt("pageSize")},
		Keyword:    ctx.Query("keyword"),
		Status:     db.UserStatus(ctx.Query("status")),
	})
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to list users")
		return ctx.ApiServerError()
	}
	items, err := toAdminUsers(ctx, users)
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to count owned projects")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess(dto.ListAdminUsersResp{Users: items, Total: total})
}

// CreateUser
// @Summary Create a user
// @Description Requires the admin. It works even if signing up is disabled.
// @Accept json
// @Produce json
// @Param data body form.AdminCreateUser true "User to create"
// @Success 200 {object} dto.AdminUser
// @Failure 400 {string} string "Invalid request body"
// @Failure 403 {string} string "Not an admin"
// @Failure 409 {string} string "Email has been used"
// @Failure 500 {string} string "Internal server error"
// @ID createAdminUser
// @Router /admin/users [post]
func (adminRoute) CreateUser(ctx context.Context, f form.AdminCreateUser) error {
	userName := strings.TrimSpace(f.UserName)
	if userName == "" {
		return ctx.ApiError(http.StatusBadRequest, "用户名不能为空")
	}
	settings, ok := loadSettings(ctx)
	if !ok {
		return nil
	}
	if msg := passwordTooShort(settings, f.Password); msg != "" {
		return ctx.ApiError(http.StatusBadRequest, "%s", msg)
	}

	user, err := db.Users.Create(ctx.Request().Context(), db.CreateUserOptions{Email: f.Email, UserName: userName, Password: f.Password})
	if err != nil {
		if errors.Is(err, db.ErrUserAlreadyExisted) {
			return ctx.ApiError(http.StatusConflict, "该邮箱已注册")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to create user")
		return ctx.ApiServerError()
	}
	if f.IsAdmin {
		if err := db.Users.SetAdmin(ctx.Request().Context(), user.ID, true); err != nil {
			logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to set admin")
			return ctx.ApiServerError()
		}
		user.IsAdmin = true
	}
	return ctx.ApiSuccess(dto.ToAdminUser(user, 0))
}

// UpdateUser
// @Summary Update the user name of a user
// @Description Requires the admin.
// @Accept json
// @Param userID path int true "User ID"
// @Param data body form.AdminUpdateUser true "User name"
// @Success 204 "No Content"
// @Failure 400 {string} string "Invalid request body"
// @Failure 403 {string} string "Not an admin"
// @Failure 404 {string} string "User not found"
// @Failure 500 {string} string "Internal server error"
// @ID updateAdminUser
// @Router /admin/users/{userID} [put]
func (adminRoute) UpdateUser(ctx context.Context, target *adminTarget, f form.AdminUpdateUser) error {
	userName := strings.TrimSpace(f.UserName)
	if userName == "" {
		return ctx.ApiError(http.StatusBadRequest, "用户名不能为空")
	}
	if err := db.Users.Update(ctx.Request().Context(), target.ID, db.UpdateUserOptions{UserName: userName}); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to update user")
		return ctx.ApiServerError()
	}
	return ctx.Status(http.StatusNoContent)
}

// SetUserAdmin
// @Summary Grant or revoke the admin
// @Description Requires the admin. The admin can not revoke itself, and at least one enabled admin is kept.
// @Accept json
// @Param userID path int true "User ID"
// @Param data body form.AdminSetUserAdmin true "Whether the user is an admin"
// @Success 204 "No Content"
// @Failure 400 {string} string "Invalid operation"
// @Failure 403 {string} string "Not an admin"
// @Failure 404 {string} string "User not found"
// @Failure 409 {string} string "The last admin"
// @Failure 500 {string} string "Internal server error"
// @ID setAdminUserAdmin
// @Router /admin/users/{userID}/admin [put]
func (adminRoute) SetUserAdmin(ctx context.Context, user *db.User, target *adminTarget, f form.AdminSetUserAdmin) error {
	if !f.IsAdmin && user.ID == target.ID {
		return ctx.ApiError(http.StatusBadRequest, "不能取消自己的管理员身份")
	}
	if f.IsAdmin && target.Disabled() {
		return ctx.ApiError(http.StatusBadRequest, "已停用的账号不能设为管理员")
	}
	if err := db.Users.SetAdmin(ctx.Request().Context(), target.ID, f.IsAdmin); err != nil {
		if errors.Is(err, db.ErrLastAdmin) {
			return ctx.ApiError(http.StatusConflict, lastAdminMessage)
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to set admin")
		return ctx.ApiServerError()
	}
	return ctx.Status(http.StatusNoContent)
}

// SetUserStatus
// @Summary Disable or enable a user
// @Description Requires the admin. Disabling signs the user out everywhere. The admin can not disable itself, and at least one enabled admin is kept.
// @Accept json
// @Param userID path int true "User ID"
// @Param data body form.AdminSetUserStatus true "Whether the user is disabled"
// @Success 204 "No Content"
// @Failure 400 {string} string "Invalid operation"
// @Failure 403 {string} string "Not an admin"
// @Failure 404 {string} string "User not found"
// @Failure 409 {string} string "The last admin"
// @Failure 500 {string} string "Internal server error"
// @ID setAdminUserStatus
// @Router /admin/users/{userID}/status [put]
func (adminRoute) SetUserStatus(ctx context.Context, hub *collab.Hub, user *db.User, target *adminTarget, f form.AdminSetUserStatus) error {
	if f.Disabled {
		if msg := selfOperationError(user.ID, target.ID, "停用"); msg != "" {
			return ctx.ApiError(http.StatusBadRequest, "%s", msg)
		}
	}
	if err := db.Users.SetDisabled(ctx.Request().Context(), target.ID, f.Disabled); err != nil {
		if errors.Is(err, db.ErrLastAdmin) {
			return ctx.ApiError(http.StatusConflict, lastAdminMessage)
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to set user status")
		return ctx.ApiServerError()
	}
	if f.Disabled {
		hub.DisconnectUser(target.ID)
	}
	return ctx.Status(http.StatusNoContent)
}

// ResetUserPassword
// @Summary Reset the password of a user
// @Description Requires the admin. The user is signed out everywhere.
// @Accept json
// @Param userID path int true "User ID"
// @Param data body form.AdminResetPassword true "New password"
// @Success 204 "No Content"
// @Failure 400 {string} string "Invalid request body"
// @Failure 403 {string} string "Not an admin"
// @Failure 404 {string} string "User not found"
// @Failure 500 {string} string "Internal server error"
// @ID resetAdminUserPassword
// @Router /admin/users/{userID}/password [put]
func (adminRoute) ResetUserPassword(ctx context.Context, hub *collab.Hub, target *adminTarget, f form.AdminResetPassword) error {
	settings, ok := loadSettings(ctx)
	if !ok {
		return nil
	}
	if msg := passwordTooShort(settings, f.Password); msg != "" {
		return ctx.ApiError(http.StatusBadRequest, "%s", msg)
	}
	if err := db.Users.UpdatePassword(ctx.Request().Context(), target.ID, f.Password); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to reset password")
		return ctx.ApiServerError()
	}
	if err := db.UserSessions.DeleteByUserID(ctx.Request().Context(), target.ID, ""); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to delete sessions")
		return ctx.ApiServerError()
	}
	hub.DisconnectUser(target.ID)
	return ctx.Status(http.StatusNoContent)
}

// RevokeUserSessions
// @Summary Sign out a user everywhere
// @Description Requires the admin. Use the account settings for the admin itself.
// @Param userID path int true "User ID"
// @Success 204 "No Content"
// @Failure 400 {string} string "Invalid operation"
// @Failure 403 {string} string "Not an admin"
// @Failure 404 {string} string "User not found"
// @Failure 500 {string} string "Internal server error"
// @ID revokeAdminUserSessions
// @Router /admin/users/{userID}/sessions [delete]
func (adminRoute) RevokeUserSessions(ctx context.Context, hub *collab.Hub, user *db.User, target *adminTarget) error {
	if user.ID == target.ID {
		return ctx.ApiError(http.StatusBadRequest, "请在账号设置中管理自己的登录设备")
	}
	if err := db.UserSessions.DeleteByUserID(ctx.Request().Context(), target.ID, ""); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to delete sessions")
		return ctx.ApiServerError()
	}
	hub.DisconnectUser(target.ID)
	return ctx.Status(http.StatusNoContent)
}

// DeleteUser
// @Summary Delete a user
// @Description Requires the admin. The projects owned by the user are transferred to transferTo, which is required if there are any.
// @Param userID path int true "User ID"
// @Param transferTo query int false "User ID to receive the owned projects"
// @Success 204 "No Content"
// @Failure 400 {string} string "Invalid operation"
// @Failure 403 {string} string "Not an admin"
// @Failure 404 {string} string "User not found"
// @Failure 409 {string} string "The last admin"
// @Failure 500 {string} string "Internal server error"
// @ID deleteAdminUser
// @Router /admin/users/{userID} [delete]
func (adminRoute) DeleteUser(ctx context.Context, hub *collab.Hub, user *db.User, target *adminTarget) error {
	if msg := selfOperationError(user.ID, target.ID, "删除"); msg != "" {
		return ctx.ApiError(http.StatusBadRequest, "%s", msg)
	}
	transferTo := ctx.QueryInt64("transferTo")
	if transferTo != 0 {
		if transferTo == target.ID {
			return ctx.ApiError(http.StatusBadRequest, "接收人不能是被删除的成员")
		}
		if msg, err := transferTargetError(ctx, transferTo); err != nil {
			logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get transfer target")
			return ctx.ApiServerError()
		} else if msg != "" {
			return ctx.ApiError(http.StatusBadRequest, "%s", msg)
		}
	}

	if err := db.Users.Delete(ctx.Request().Context(), target.ID, transferTo); err != nil {
		switch {
		case errors.Is(err, db.ErrUserOwnsProjects):
			return ctx.ApiError(http.StatusBadRequest, "该成员拥有多维表格，请选择接收人")
		case errors.Is(err, db.ErrLastAdmin):
			return ctx.ApiError(http.StatusConflict, lastAdminMessage)
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to delete user")
		return ctx.ApiServerError()
	}
	hub.DisconnectUser(target.ID)
	return ctx.Status(http.StatusNoContent)
}

// ListProjects
// @Summary List all the projects
// @Description Requires the admin. The newest projects come first.
// @Produce json
// @Param page query int false "Page number, starting from 1"
// @Param pageSize query int false "Page size, defaults to 20"
// @Param keyword query string false "Matches the project name"
// @Success 200 {object} dto.ListAdminProjectsResp
// @Failure 403 {string} string "Not an admin"
// @Failure 500 {string} string "Internal server error"
// @ID listAdminProjects
// @Router /admin/projects [get]
func (adminRoute) ListProjects(ctx context.Context) error {
	c := ctx.Request().Context()
	projects, total, err := db.Projects.ListAll(c, db.ListAllProjectsOptions{
		Pagination: dbutil.Pagination{Page: ctx.QueryInt("page"), PageSize: ctx.QueryInt("pageSize")},
		Keyword:    ctx.Query("keyword"),
	})
	if err != nil {
		logrus.WithContext(c).WithError(err).Error("Failed to list projects")
		return ctx.ApiServerError()
	}

	projectIDs := make([]int64, 0, len(projects))
	ownerIDs := make([]int64, 0, len(projects))
	for _, p := range projects {
		projectIDs = append(projectIDs, p.ID)
		ownerIDs = append(ownerIDs, p.OwnerUserID)
	}
	owners, err := usersByID(ctx, ownerIDs)
	if err != nil {
		logrus.WithContext(c).WithError(err).Error("Failed to list project owners")
		return ctx.ApiServerError()
	}
	memberCounts, err := db.ProjectMembers.CountByProjectIDs(c, projectIDs)
	if err != nil {
		logrus.WithContext(c).WithError(err).Error("Failed to count project members")
		return ctx.ApiServerError()
	}
	tableCounts, err := db.SLTables.CountByProjectIDs(c, projectIDs)
	if err != nil {
		logrus.WithContext(c).WithError(err).Error("Failed to count tables")
		return ctx.ApiServerError()
	}

	items := make([]*dto.AdminProject, 0, len(projects))
	for _, p := range projects {
		item := &dto.AdminProject{
			UID:         p.UID,
			Name:        p.Name,
			MemberCount: memberCounts[p.ID],
			TableCount:  tableCounts[p.ID],
			CreatedAt:   p.CreatedAt,
		}
		if owner, ok := owners[p.OwnerUserID]; ok {
			item.Owner = dto.ToUserBrief(owner)
		}
		items = append(items, item)
	}
	return ctx.ApiSuccess(dto.ListAdminProjectsResp{Projects: items, Total: total})
}

// TransferProject
// @Summary Transfer the ownership of a project
// @Description Requires the admin. The new owner can be any enabled user, the previous owner becomes a manager.
// @Accept json
// @Param projectUID path string true "Project UID"
// @Param data body form.AdminTransferProject true "New owner"
// @Success 204 "No Content"
// @Failure 400 {string} string "Invalid new owner"
// @Failure 403 {string} string "Not an admin"
// @Failure 404 {string} string "Project not found"
// @Failure 500 {string} string "Internal server error"
// @ID transferAdminProject
// @Router /admin/projects/{projectUID}/owner [put]
func (adminRoute) TransferProject(ctx context.Context, hub *collab.Hub, project *db.Project, f form.AdminTransferProject) error {
	if f.UserID == project.OwnerUserID {
		return ctx.ApiError(http.StatusBadRequest, "该成员已是所有者")
	}
	if msg, err := transferTargetError(ctx, f.UserID); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get new owner")
		return ctx.ApiServerError()
	} else if msg != "" {
		return ctx.ApiError(http.StatusBadRequest, "%s", msg)
	}
	if err := transferProject(ctx, hub, project, f.UserID); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to transfer project")
		return ctx.ApiServerError()
	}
	return ctx.Status(http.StatusNoContent)
}

// DeleteProject
// @Summary Delete a project
// @Description Requires the admin.
// @Param projectUID path string true "Project UID"
// @Success 204 "No Content"
// @Failure 403 {string} string "Not an admin"
// @Failure 404 {string} string "Project not found"
// @Failure 500 {string} string "Internal server error"
// @ID deleteAdminProject
// @Router /admin/projects/{projectUID} [delete]
func (adminRoute) DeleteProject(ctx context.Context, hub *collab.Hub, project *db.Project, tx dbutil.Transactor) error {
	return Project.DeleteProject(ctx, hub, project, tx)
}

// GetSettings
// @Summary Get the system settings
// @Description Requires the admin.
// @Produce json
// @Success 200 {object} db.SystemSettings
// @Failure 403 {string} string "Not an admin"
// @Failure 500 {string} string "Internal server error"
// @ID getSystemSettings
// @Router /admin/settings [get]
func (adminRoute) GetSettings(ctx context.Context) error {
	settings, ok := loadSettings(ctx)
	if !ok {
		return nil
	}
	return ctx.ApiSuccess(settings)
}

// UpdateSettings
// @Summary Update the system settings
// @Description Requires the admin. The password and session settings apply to the later sign-ups and sign-ins.
// @Accept json
// @Produce json
// @Param data body form.UpdateSystemSettings true "System settings"
// @Success 200 {object} db.SystemSettings
// @Failure 400 {string} string "Invalid settings"
// @Failure 403 {string} string "Not an admin"
// @Failure 500 {string} string "Internal server error"
// @ID updateSystemSettings
// @Router /admin/settings [put]
func (adminRoute) UpdateSettings(ctx context.Context, f form.UpdateSystemSettings) error {
	settings := db.SystemSettings{
		SiteName:          strings.TrimSpace(f.SiteName),
		AllowSignUp:       f.AllowSignUp,
		PasswordMinLength: f.PasswordMinLength,
		SessionTTLDays:    f.SessionTTLDays,
	}
	if err := settings.Validate(); err != nil {
		return ctx.ApiError(http.StatusBadRequest, "%s", err.Error())
	}
	if err := db.Settings.SaveSystem(ctx.Request().Context(), settings); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to save system settings")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess(settings)
}
