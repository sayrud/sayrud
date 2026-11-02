package api

import (
	"net/http"
	"strings"

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

// ownBot is owned by the signed-in member, mapped apart from the signed-in *db.User.
type ownBot struct {
	*db.User
}

// OwnBotTargeter maps the bot of the userID path parameter as *ownBot, and responds 404 unless it is owned by the signed-in member.
func (accountRoute) OwnBotTargeter(ctx context.Context, user *db.User) error {
	bot, err := db.Users.GetByID(ctx.Request().Context(), ctx.ParamInt64("userID"))
	if err != nil && !errors.Is(err, db.ErrUserNotFound) {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get own bot")
		return ctx.ApiServerError()
	}
	if bot == nil || !bot.IsBot() || bot.OwnerID != user.ID {
		return ctx.ApiError(http.StatusNotFound, "admin::bot_not_found")
	}
	ctx.Map(&ownBot{User: bot})
	return nil
}

// botQuota returns how many bots the member may own, 0 for no limit, and whether one more can be created now.
func botQuota(ctx context.Context, user *db.User, settings *db.SystemSettings) (int, bool, error) {
	if user.IsAdmin {
		return 0, true, nil
	}
	if !settings.AllowMemberBots {
		return settings.MemberBotLimit, false, nil
	}
	count, err := db.Users.CountBotsByOwner(ctx.Request().Context(), user.ID)
	if err != nil {
		return 0, false, errors.Wrap(err, "count bots")
	}
	return settings.MemberBotLimit, count < int64(settings.MemberBotLimit), nil
}

// ListBots
// @Summary List the bots of the signed-in member
// @Tags My bots
// @Description The newest first, along with the limit of the own bots. Not available to access tokens.
// @Produce json
// @Success 200 {object} dto.ListMyBotsResp
// @Failure 401 {string} string "Not signed in"
// @Failure 403 {string} string "Called with an access token"
// @Failure 500 {string} string "Internal server error"
// @ID listMyBots
// @Router /auth/bots [get]
func (accountRoute) ListBots(ctx context.Context, user *db.User) error {
	c := ctx.Request().Context()
	settings, ok := loadSettings(ctx)
	if !ok {
		return nil
	}
	limit, canCreate, err := botQuota(ctx, user, settings)
	if err != nil {
		logrus.WithContext(c).WithError(err).Error("Failed to get bot quota")
		return ctx.ApiServerError()
	}
	bots, _, err := db.Users.List(c, db.ListUsersOptions{
		Pagination: dbutil.Pagination{PageSize: 100},
		Kind:       db.UserKindBot,
		OwnerID:    user.ID,
	})
	if err != nil {
		logrus.WithContext(c).WithError(err).Error("Failed to list own bots")
		return ctx.ApiServerError()
	}
	items, err := toBots(ctx, bots)
	if err != nil {
		logrus.WithContext(c).WithError(err).Error("Failed to summarize own bots")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess(dto.ListMyBotsResp{Bots: items, Limit: limit, CanCreate: canCreate})
}

// CreateBot
// @Summary Create an own bot
// @Tags My bots
// @Description The bot is only visible to the member, its access never exceeds the own role of the member in each project, and it is deleted with the member. It comes with its only access token, whose secret is only returned once. Not available to access tokens.
// @Accept json
// @Produce json
// @Param data body form.CreateBot true "Bot to create"
// @Success 200 {object} dto.CreateBotResp
// @Failure 400 {string} string "Invalid name, scopes, projects or expiration"
// @Failure 401 {string} string "Not signed in"
// @Failure 403 {string} string "Member bots are turned off, or called with an access token"
// @Failure 409 {string} string "Too many bots"
// @Failure 500 {string} string "Internal server error"
// @ID createMyBot
// @Router /auth/bots [post]
func (accountRoute) CreateBot(ctx context.Context, tx dbutil.Transactor, user *db.User, f form.CreateBot) error {
	c := ctx.Request().Context()
	settings, ok := loadSettings(ctx)
	if !ok {
		return nil
	}
	limit, canCreate, err := botQuota(ctx, user, settings)
	if err != nil {
		logrus.WithContext(c).WithError(err).Error("Failed to get bot quota")
		return ctx.ApiServerError()
	}
	if !canCreate {
		if !settings.AllowMemberBots {
			return ctx.ApiError(http.StatusForbidden, "account::member_bots_disabled")
		}
		return ctx.ApiError(http.StatusConflict, "account::too_many_bots", limit)
	}
	return createBot(ctx, tx, user, user, f)
}

// GetBot
// @Summary Get an own bot
// @Tags My bots
// @Description The bot comes with its only access token without the secret. Not available to access tokens.
// @Produce json
// @Param userID path int true "User ID of the bot"
// @Success 200 {object} dto.Bot
// @Failure 401 {string} string "Not signed in"
// @Failure 404 {string} string "Bot not found"
// @Failure 500 {string} string "Internal server error"
// @ID getMyBot
// @Router /auth/bots/{userID} [get]
func (accountRoute) GetBot(ctx context.Context, bot *ownBot) error {
	return getBot(ctx, bot.User)
}

// UpdateBotAccess
// @Summary Change the access of an own bot
// @Tags My bots
// @Description Changes the scopes, the projects and the expiration of its access token, the secret stays the same. Not available to access tokens.
// @Accept json
// @Produce json
// @Param userID path int true "User ID of the bot"
// @Param data body form.UpdateBotAccess true "Access"
// @Success 200 {object} dto.Bot
// @Failure 400 {string} string "Invalid scopes, projects or expiration"
// @Failure 401 {string} string "Not signed in"
// @Failure 404 {string} string "Bot or token not found"
// @Failure 500 {string} string "Internal server error"
// @ID updateMyBotAccess
// @Router /auth/bots/{userID}/access [put]
func (accountRoute) UpdateBotAccess(ctx context.Context, bot *ownBot, f form.UpdateBotAccess) error {
	return updateBotAccess(ctx, bot.User, f)
}

// ResetBotToken
// @Summary Reset the access token of an own bot
// @Tags My bots
// @Description The old secret stops working at once and the access is kept, the new secret is only returned once. Not available to access tokens.
// @Produce json
// @Param userID path int true "User ID of the bot"
// @Success 200 {object} dto.CreateAccessTokenResp
// @Failure 401 {string} string "Not signed in"
// @Failure 404 {string} string "Bot not found"
// @Failure 500 {string} string "Internal server error"
// @ID resetMyBotToken
// @Router /auth/bots/{userID}/token [post]
func (accountRoute) ResetBotToken(ctx context.Context, tx dbutil.Transactor, user *db.User, bot *ownBot) error {
	return resetBotToken(ctx, tx, user, bot.User)
}

// UpdateBot
// @Summary Rename an own bot
// @Tags My bots
// @Description Not available to access tokens.
// @Accept json
// @Param userID path int true "User ID of the bot"
// @Param data body form.UpdateBot true "Name"
// @Success 204 "No Content"
// @Failure 400 {string} string "Invalid request body"
// @Failure 401 {string} string "Not signed in"
// @Failure 404 {string} string "Bot not found"
// @Failure 500 {string} string "Internal server error"
// @ID updateMyBot
// @Router /auth/bots/{userID} [put]
func (accountRoute) UpdateBot(ctx context.Context, bot *ownBot, f form.UpdateBot) error {
	userName := strings.TrimSpace(f.UserName)
	if userName == "" {
		return ctx.ApiError(http.StatusBadRequest, "auth::user_name_required")
	}
	if err := db.Users.Update(ctx.Request().Context(), bot.ID, db.UpdateUserOptions{UserName: userName}); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to rename own bot")
		return ctx.ApiServerError()
	}
	return ctx.Status(http.StatusNoContent)
}

// SetBotStatus
// @Summary Disable or enable an own bot
// @Tags My bots
// @Description Its access tokens stop working while it is disabled. Not available to access tokens.
// @Accept json
// @Param userID path int true "User ID of the bot"
// @Param data body form.SetBotStatus true "Whether the bot is disabled"
// @Success 204 "No Content"
// @Failure 401 {string} string "Not signed in"
// @Failure 404 {string} string "Bot not found"
// @Failure 500 {string} string "Internal server error"
// @ID setMyBotStatus
// @Router /auth/bots/{userID}/status [put]
func (accountRoute) SetBotStatus(ctx context.Context, bot *ownBot, f form.SetBotStatus) error {
	if err := db.Users.SetDisabled(ctx.Request().Context(), bot.ID, f.Disabled); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to set own bot status")
		return ctx.ApiServerError()
	}
	return ctx.Status(http.StatusNoContent)
}

// DeleteBot
// @Summary Delete an own bot
// @Tags My bots
// @Description Its access tokens are revoked and it leaves every project. Not available to access tokens.
// @Param userID path int true "User ID of the bot"
// @Success 204 "No Content"
// @Failure 401 {string} string "Not signed in"
// @Failure 404 {string} string "Bot not found"
// @Failure 500 {string} string "Internal server error"
// @ID deleteMyBot
// @Router /auth/bots/{userID} [delete]
func (accountRoute) DeleteBot(ctx context.Context, bot *ownBot) error {
	if err := db.Users.Delete(ctx.Request().Context(), bot.ID, 0); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to delete own bot")
		return ctx.ApiServerError()
	}
	return ctx.Status(http.StatusNoContent)
}

// UploadBotAvatar
// @Summary Upload the avatar of an own bot
// @Tags My bots
// @Description Accept a PNG, JPEG or GIF up to 2 MiB and 4096 pixels per edge. Not available to access tokens.
// @Accept multipart/form-data
// @Produce json
// @Param userID path int true "User ID of the bot"
// @Param file formData file true "Avatar image"
// @Success 200 {object} dto.UserBrief
// @Failure 400 {string} string "Invalid image or multipart body"
// @Failure 401 {string} string "Not signed in"
// @Failure 404 {string} string "Bot not found"
// @Failure 413 {string} string "Image is too large"
// @Failure 500 {string} string "Internal server error"
// @ID uploadMyBotAvatar
// @Router /auth/bots/{userID}/avatar [post]
func (accountRoute) UploadBotAvatar(ctx context.Context, hub *collab.Hub, bot *ownBot, f form.UploadAvatar, errs binding.Errors) error {
	updated, ok := saveAvatar(ctx, hub, bot.ID, f, errs)
	if !ok {
		return nil
	}
	return ctx.ApiSuccess(dto.ToUserBrief(updated))
}

// RemoveBotAvatar
// @Summary Restore the default avatar of an own bot
// @Tags My bots
// @Description Not available to access tokens.
// @Produce json
// @Param userID path int true "User ID of the bot"
// @Success 200 {object} dto.UserBrief
// @Failure 401 {string} string "Not signed in"
// @Failure 404 {string} string "Bot not found"
// @Failure 500 {string} string "Internal server error"
// @ID removeMyBotAvatar
// @Router /auth/bots/{userID}/avatar [delete]
func (accountRoute) RemoveBotAvatar(ctx context.Context, hub *collab.Hub, bot *ownBot) error {
	updated, err := clearAvatar(ctx, hub, bot.ID)
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to remove own bot avatar")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess(dto.ToUserBrief(updated))
}
