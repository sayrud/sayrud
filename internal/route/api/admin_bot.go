package api

import (
	"net/http"

	"github.com/cockroachdb/errors"
	"github.com/flamego/binding"
	"github.com/sirupsen/logrus"

	"github.com/wuhan005/sayrud/internal/collab"
	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dbutil"
	"github.com/wuhan005/sayrud/internal/dto"
	"github.com/wuhan005/sayrud/internal/form"
)

// BotTargeter maps the bot of the userID path parameter as *adminTarget, and responds 404 if it is not found or not a bot.
func (adminRoute) BotTargeter(ctx context.Context) error {
	user, err := db.Users.GetByID(ctx.Request().Context(), ctx.ParamInt64("userID"))
	if err != nil && !errors.Is(err, db.ErrUserNotFound) {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get target bot")
		return ctx.ApiServerError()
	}
	if user == nil || !user.IsBot() {
		return ctx.ApiError(http.StatusNotFound, "admin::bot_not_found")
	}
	ctx.Map(&adminTarget{User: user})
	return nil
}

// ListBots
// @Summary List the bots
// @Tags Admin - Bots
// @Description Requires the admin. The newest bots come first.
// @Produce json
// @Param page query int false "Page number, starting from 1"
// @Param pageSize query int false "Page size, defaults to 20"
// @Param keyword query string false "Matches the name or email"
// @Param status query string false "Filter by status, all if empty" Enums(active, disabled)
// @Success 200 {object} dto.ListAdminBotsResp
// @Failure 403 {string} string "Not an admin"
// @Failure 500 {string} string "Internal server error"
// @ID listAdminBots
// @Router /admin/bots [get]
func (adminRoute) ListBots(ctx context.Context) error {
	c := ctx.Request().Context()
	status := db.UserStatus(ctx.Query("status"))
	if status != db.UserStatusActive && status != db.UserStatusDisabled {
		status = db.UserStatusAll
	}
	bots, total, err := db.Users.List(c, db.ListUsersOptions{
		Pagination: dbutil.Pagination{Page: ctx.QueryInt("page"), PageSize: ctx.QueryInt("pageSize")},
		Keyword:    ctx.Query("keyword"),
		Status:     status,
		Kind:       db.UserKindBot,
	})
	if err != nil {
		logrus.WithContext(c).WithError(err).Error("Failed to list bots")
		return ctx.ApiServerError()
	}
	items, err := toBots(ctx, bots)
	if err != nil {
		logrus.WithContext(c).WithError(err).Error("Failed to summarize bots")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess(dto.ListAdminBotsResp{Bots: items, Total: total})
}

// botOwner returns the member who may own a bot, or the message key if the user is not an enabled human.
func botOwner(ctx context.Context, userID int64) (*db.User, string, error) {
	user, err := db.Users.GetByID(ctx.Request().Context(), userID)
	if err != nil {
		if errors.Is(err, db.ErrUserNotFound) {
			return nil, "admin::bot_owner_invalid", nil
		}
		return nil, "", err
	}
	if user.IsBot() || user.Disabled() {
		return nil, "admin::bot_owner_invalid", nil
	}
	return user, "", nil
}

// CreateBot
// @Summary Create a bot
// @Tags Admin - Bots
// @Description Requires the admin. The bot is owned by the given member or the admin, and created together with its only access token whose secret is only returned once. Bots have no password and join the projects by the generated email.
// @Accept json
// @Produce json
// @Param data body form.CreateBot true "Bot to create"
// @Success 200 {object} dto.CreateBotResp
// @Failure 400 {string} string "Invalid name, owner, scopes, projects or expiration"
// @Failure 403 {string} string "Not an admin"
// @Failure 500 {string} string "Internal server error"
// @ID createAdminBot
// @Router /admin/bots [post]
func (adminRoute) CreateBot(ctx context.Context, tx dbutil.Transactor, user *db.User, f form.CreateBot) error {
	owner := user
	if f.OwnerID != 0 && f.OwnerID != user.ID {
		var msg string
		var err error
		if owner, msg, err = botOwner(ctx, f.OwnerID); err != nil {
			logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get bot owner")
			return ctx.ApiServerError()
		} else if msg != "" {
			return ctx.ApiError(http.StatusBadRequest, msg)
		}
	}
	return createBot(ctx, tx, user, owner, f)
}

// GetBot
// @Summary Get a bot
// @Tags Admin - Bots
// @Description Requires the admin. The bot comes with its owner and its only access token without the secret.
// @Produce json
// @Param userID path int true "User ID of the bot"
// @Success 200 {object} dto.Bot
// @Failure 403 {string} string "Not an admin"
// @Failure 404 {string} string "Bot not found"
// @Failure 500 {string} string "Internal server error"
// @ID getAdminBot
// @Router /admin/bots/{userID} [get]
func (adminRoute) GetBot(ctx context.Context, target *adminTarget) error {
	return getBot(ctx, target.User)
}

// UpdateBotAccess
// @Summary Change the access of a bot
// @Tags Admin - Bots
// @Description Requires the admin. Changes the scopes, the projects and the expiration of its access token, the secret stays the same.
// @Accept json
// @Produce json
// @Param userID path int true "User ID of the bot"
// @Param data body form.UpdateBotAccess true "Access"
// @Success 200 {object} dto.Bot
// @Failure 400 {string} string "Invalid scopes, projects or expiration"
// @Failure 403 {string} string "Not an admin"
// @Failure 404 {string} string "Bot or token not found"
// @Failure 500 {string} string "Internal server error"
// @ID updateAdminBotAccess
// @Router /admin/bots/{userID}/access [put]
func (adminRoute) UpdateBotAccess(ctx context.Context, target *adminTarget, f form.UpdateBotAccess) error {
	return updateBotAccess(ctx, target.User, f)
}

// ResetBotToken
// @Summary Reset the access token of a bot
// @Tags Admin - Bots
// @Description Requires the admin. The old secret stops working at once and the access is kept, the new secret is only returned once.
// @Produce json
// @Param userID path int true "User ID of the bot"
// @Success 200 {object} dto.CreateAccessTokenResp
// @Failure 403 {string} string "Not an admin"
// @Failure 404 {string} string "Bot not found"
// @Failure 500 {string} string "Internal server error"
// @ID resetAdminBotToken
// @Router /admin/bots/{userID}/token [post]
func (adminRoute) ResetBotToken(ctx context.Context, tx dbutil.Transactor, user *db.User, target *adminTarget) error {
	return resetBotToken(ctx, tx, user, target.User)
}

// SetBotOwner
// @Summary Hand a bot over to another member
// @Tags Admin - Bots
// @Description Requires the admin. The memberships are kept, but the bot is limited by the access of the new owner and only the new owner can invite it.
// @Accept json
// @Produce json
// @Param userID path int true "User ID of the bot"
// @Param data body form.SetBotOwner true "New owner"
// @Success 200 {object} dto.Bot
// @Failure 400 {string} string "The new owner is not an enabled member"
// @Failure 403 {string} string "Not an admin"
// @Failure 404 {string} string "Bot not found"
// @Failure 500 {string} string "Internal server error"
// @ID setAdminBotOwner
// @Router /admin/bots/{userID}/owner [put]
func (adminRoute) SetBotOwner(ctx context.Context, target *adminTarget, f form.SetBotOwner) error {
	if _, msg, err := botOwner(ctx, f.OwnerID); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get bot owner")
		return ctx.ApiServerError()
	} else if msg != "" {
		return ctx.ApiError(http.StatusBadRequest, msg)
	}
	if err := db.Users.SetBotOwner(ctx.Request().Context(), target.ID, f.OwnerID); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to set bot owner")
		return ctx.ApiServerError()
	}
	target.OwnerID = f.OwnerID
	return getBot(ctx, target.User)
}

// UploadBotAvatar
// @Summary Upload the avatar of a bot
// @Tags Admin - Bots
// @Description Requires the admin. Accept a PNG, JPEG or GIF up to 2 MiB and 4096 pixels per edge.
// @Accept multipart/form-data
// @Produce json
// @Param userID path int true "User ID of the bot"
// @Param file formData file true "Avatar image"
// @Success 200 {object} dto.UserBrief
// @Failure 400 {string} string "Invalid image or multipart body"
// @Failure 403 {string} string "Not an admin"
// @Failure 404 {string} string "Bot not found"
// @Failure 413 {string} string "Image is too large"
// @Failure 500 {string} string "Internal server error"
// @ID uploadAdminBotAvatar
// @Router /admin/bots/{userID}/avatar [post]
func (adminRoute) UploadBotAvatar(ctx context.Context, hub *collab.Hub, target *adminTarget, f form.UploadAvatar, errs binding.Errors) error {
	updated, ok := saveAvatar(ctx, hub, target.ID, f, errs)
	if !ok {
		return nil
	}
	return ctx.ApiSuccess(dto.ToUserBrief(updated))
}

// RemoveBotAvatar
// @Summary Restore the default avatar of a bot
// @Tags Admin - Bots
// @Description Requires the admin.
// @Produce json
// @Param userID path int true "User ID of the bot"
// @Success 200 {object} dto.UserBrief
// @Failure 403 {string} string "Not an admin"
// @Failure 404 {string} string "Bot not found"
// @Failure 500 {string} string "Internal server error"
// @ID removeAdminBotAvatar
// @Router /admin/bots/{userID}/avatar [delete]
func (adminRoute) RemoveBotAvatar(ctx context.Context, hub *collab.Hub, target *adminTarget) error {
	updated, err := clearAvatar(ctx, hub, target.ID)
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to remove bot avatar")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess(dto.ToUserBrief(updated))
}
