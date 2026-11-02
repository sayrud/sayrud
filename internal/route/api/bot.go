package api

import (
	"net/http"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dbutil"
	"github.com/wuhan005/sayrud/internal/dto"
	"github.com/wuhan005/sayrud/internal/form"
)

// botTokenName names the only access token of a bot, which the bot pages never show.
const botTokenName = "bot"

// toBots converts the bots along with their owners and access tokens.
func toBots(ctx context.Context, bots []*db.User) ([]*dto.Bot, error) {
	ids := make([]int64, 0, len(bots))
	ownerIDs := make([]int64, 0, len(bots))
	for _, b := range bots {
		ids = append(ids, b.ID)
		ownerIDs = append(ownerIDs, b.OwnerID)
	}
	latest, err := db.AccessTokens.LatestByUserIDs(ctx.Request().Context(), ids)
	if err != nil {
		return nil, errors.Wrap(err, "list tokens")
	}
	tokens := make([]*db.AccessToken, 0, len(latest))
	for _, b := range bots {
		if t, ok := latest[b.ID]; ok {
			tokens = append(tokens, t)
		}
	}
	converted, err := toAccessTokens(ctx, tokens)
	if err != nil {
		return nil, errors.Wrap(err, "convert tokens")
	}
	tokenByUser := make(map[int64]*dto.AccessToken, len(tokens))
	for i, t := range tokens {
		tokenByUser[t.UserID] = converted[i]
	}
	owners, err := usersByID(ctx, ownerIDs)
	if err != nil {
		return nil, errors.Wrap(err, "list owners")
	}
	items := make([]*dto.Bot, 0, len(bots))
	for _, b := range bots {
		items = append(items, dto.ToBot(b, owners[b.OwnerID], tokenByUser[b.ID]))
	}
	return items, nil
}

// getBot responds the bot with its owner and access token.
func getBot(ctx context.Context, bot *db.User) error {
	items, err := toBots(ctx, []*db.User{bot})
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get bot")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess(items[0])
}

// createBot validates the form and creates the bot owned by the member together with its only access token.
func createBot(ctx context.Context, tx dbutil.Transactor, operator, owner *db.User, f form.CreateBot) error {
	c := ctx.Request().Context()
	userName := strings.TrimSpace(f.UserName)
	if userName == "" {
		return ctx.ApiError(http.StatusBadRequest, "auth::user_name_required")
	}
	access, ok := parseTokenAccess(ctx, f.Scopes, f.Projects, f.ExpiresAt)
	if !ok {
		return nil
	}

	var bot *db.User
	var token *db.AccessToken
	var secret string
	if err := tx.Transaction(func(tx *gorm.DB) error {
		var err error
		if bot, err = db.NewUsersStore(tx).CreateBot(c, userName, owner.ID); err != nil {
			return errors.Wrap(err, "create bot")
		}
		token, secret, err = db.NewAccessTokensStore(tx).Create(c, db.CreateAccessTokenOptions{
			UserID:     bot.ID,
			Name:       botTokenName,
			Scopes:     access.Scopes,
			ProjectIDs: access.ProjectIDs,
			ExpiresAt:  access.ExpiresAt,
			CreatedBy:  operator.ID,
		})
		return errors.Wrap(err, "create token")
	}); err != nil {
		logrus.WithContext(c).WithError(err).Error("Failed to create bot")
		return ctx.ApiServerError()
	}

	return ctx.ApiSuccess(dto.CreateBotResp{
		Bot:    dto.ToBot(bot, owner, dto.ToAccessToken(token, access.projects, dbutil.Now())),
		Secret: secret,
	})
}

// updateBotAccess changes what the only access token of the bot may do, its secret stays the same.
func updateBotAccess(ctx context.Context, bot *db.User, f form.UpdateBotAccess) error {
	c := ctx.Request().Context()
	access, ok := parseTokenAccess(ctx, f.Scopes, f.Projects, f.ExpiresAt)
	if !ok {
		return nil
	}
	latest, err := db.AccessTokens.LatestByUserIDs(c, []int64{bot.ID})
	if err != nil {
		logrus.WithContext(c).WithError(err).Error("Failed to get bot token")
		return ctx.ApiServerError()
	}
	token, ok := latest[bot.ID]
	if !ok {
		return ctx.ApiError(http.StatusNotFound, "account::token_not_found")
	}
	if err := db.AccessTokens.UpdateAccess(c, token.ID, access.AccessTokenAccess); err != nil {
		logrus.WithContext(c).WithError(err).Error("Failed to update bot access")
		return ctx.ApiServerError()
	}
	return getBot(ctx, bot)
}

// resetBotToken replaces the secret of the only access token of the bot, issuing a read-only one if it has none.
func resetBotToken(ctx context.Context, tx dbutil.Transactor, operator, bot *db.User) error {
	c := ctx.Request().Context()
	var token *db.AccessToken
	var secret string
	if err := tx.Transaction(func(tx *gorm.DB) error {
		tokens := db.NewAccessTokensStore(tx)
		latest, err := tokens.LatestByUserIDs(c, []int64{bot.ID})
		if err != nil {
			return errors.Wrap(err, "get token")
		}
		current, ok := latest[bot.ID]
		if !ok {
			token, secret, err = tokens.Create(c, db.CreateAccessTokenOptions{
				UserID:    bot.ID,
				Name:      botTokenName,
				Scopes:    []db.AccessTokenScope{db.AccessTokenScopeRead},
				CreatedBy: operator.ID,
			})
			return errors.Wrap(err, "create token")
		}
		if err := tokens.DeleteOthers(c, bot.ID, current.ID); err != nil {
			return errors.Wrap(err, "delete extra tokens")
		}
		token, secret, err = tokens.Rotate(c, current.ID)
		return errors.Wrap(err, "rotate token")
	}); err != nil {
		logrus.WithContext(c).WithError(err).Error("Failed to reset bot token")
		return ctx.ApiServerError()
	}

	converted, err := toAccessTokens(ctx, []*db.AccessToken{token})
	if err != nil {
		logrus.WithContext(c).WithError(err).Error("Failed to convert bot token")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess(dto.CreateAccessTokenResp{Token: converted[0], Secret: secret})
}
