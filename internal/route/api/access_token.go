package api

import (
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/sirupsen/logrus"

	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dbutil"
	"github.com/wuhan005/sayrud/internal/dto"
	"github.com/wuhan005/sayrud/internal/form"
)

const (
	maxAccessTokensPerUser = 50
	maxAccessTokenProjects = 100
)

// ListTokens
// @Summary List the access tokens of the signed-in user
// @Tags Access tokens
// @Description The newest first. Not available to access tokens.
// @Produce json
// @Success 200 {array} dto.AccessToken
// @Failure 401 {string} string "Not signed in"
// @Failure 403 {string} string "Called with an access token"
// @Failure 500 {string} string "Internal server error"
// @ID listAccessTokens
// @Router /auth/tokens [get]
func (accountRoute) ListTokens(ctx context.Context, user *db.User) error {
	return listAccessTokens(ctx, user.ID)
}

// CreateToken
// @Summary Create an access token of the signed-in user
// @Tags Access tokens
// @Description The token acts as the user with the granted scopes, the secret is only returned once. Not available to access tokens.
// @Accept json
// @Produce json
// @Param data body form.CreateAccessToken true "Token"
// @Success 200 {object} dto.CreateAccessTokenResp
// @Failure 400 {string} string "Invalid name, scopes, projects or expiration"
// @Failure 401 {string} string "Not signed in"
// @Failure 403 {string} string "Called with an access token"
// @Failure 409 {string} string "Too many tokens"
// @Failure 500 {string} string "Internal server error"
// @ID createAccessToken
// @Router /auth/tokens [post]
func (accountRoute) CreateToken(ctx context.Context, user *db.User, f form.CreateAccessToken) error {
	return createAccessToken(ctx, user.ID, user.ID, f)
}

// RevokeToken
// @Summary Revoke an access token of the signed-in user
// @Tags Access tokens
// @Description Not available to access tokens.
// @Param tokenID path int true "Token ID"
// @Success 204 "No Content"
// @Failure 401 {string} string "Not signed in"
// @Failure 403 {string} string "Called with an access token"
// @Failure 404 {string} string "Token not found"
// @Failure 500 {string} string "Internal server error"
// @ID revokeAccessToken
// @Router /auth/tokens/{tokenID} [delete]
func (accountRoute) RevokeToken(ctx context.Context, user *db.User) error {
	return revokeAccessToken(ctx, user.ID, ctx.ParamInt64("tokenID"))
}

func listAccessTokens(ctx context.Context, userID int64) error {
	c := ctx.Request().Context()
	tokens, err := db.AccessTokens.ListByUserID(c, userID)
	if err != nil {
		logrus.WithContext(c).WithError(err).Error("Failed to list access tokens")
		return ctx.ApiServerError()
	}
	resp, err := toAccessTokens(ctx, tokens)
	if err != nil {
		logrus.WithContext(c).WithError(err).Error("Failed to list projects of access tokens")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess(resp)
}

func toAccessTokens(ctx context.Context, tokens []*db.AccessToken) ([]*dto.AccessToken, error) {
	var ids []int64
	for _, t := range tokens {
		ids = append(ids, t.ProjectIDs...)
	}
	projects, err := db.Projects.ListByIDs(ctx.Request().Context(), ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[int64]*db.Project, len(projects))
	for _, p := range projects {
		byID[p.ID] = p
	}
	now := dbutil.Now()
	resp := make([]*dto.AccessToken, 0, len(tokens))
	for _, t := range tokens {
		resp = append(resp, dto.ToAccessToken(t, byID, now))
	}
	return resp, nil
}

// tokenAccess is the validated access of a token along with the projects it is restricted to.
type tokenAccess struct {
	db.AccessTokenAccess
	projects map[int64]*db.Project
}

// parseTokenAccess validates the scopes, the project UIDs and the expiration, the error response has been written if ok is false.
func parseTokenAccess(ctx context.Context, scopes, projectUIDs []string, expiresAt *time.Time) (_ *tokenAccess, ok bool) {
	c := ctx.Request().Context()
	fail := func(status int, key string, args ...interface{}) (*tokenAccess, bool) {
		_ = ctx.ApiError(status, key, args...)
		return nil, false
	}
	normalized, ok := db.NormalizeAccessTokenScopes(scopes)
	if !ok {
		return fail(http.StatusBadRequest, "account::token_scope_invalid")
	}
	if expiresAt != nil && !expiresAt.After(dbutil.Now()) {
		return fail(http.StatusBadRequest, "account::token_expiry_past")
	}
	uids := slices.Compact(slices.Sorted(slices.Values(projectUIDs)))
	if len(uids) > maxAccessTokenProjects {
		return fail(http.StatusBadRequest, "account::token_too_many_projects", maxAccessTokenProjects)
	}
	projects, err := db.Projects.ListByUIDs(c, uids)
	if err != nil {
		logrus.WithContext(c).WithError(err).Error("Failed to list projects of access token")
		return fail(http.StatusInternalServerError, "common::internal_error")
	}
	if len(projects) != len(uids) {
		return fail(http.StatusBadRequest, "account::token_project_not_found")
	}
	access := &tokenAccess{
		AccessTokenAccess: db.AccessTokenAccess{Scopes: normalized, ProjectIDs: make([]int64, 0, len(projects)), ExpiresAt: expiresAt},
		projects:          make(map[int64]*db.Project, len(projects)),
	}
	for _, p := range projects {
		access.ProjectIDs = append(access.ProjectIDs, p.ID)
		access.projects[p.ID] = p
	}
	return access, true
}

// createAccessToken validates the form and creates a token of the user on behalf of createdBy.
func createAccessToken(ctx context.Context, userID, createdBy int64, f form.CreateAccessToken) error {
	c := ctx.Request().Context()
	name := strings.TrimSpace(f.Name)
	if name == "" {
		return ctx.ApiError(http.StatusBadRequest, "account::token_name_required")
	}
	access, ok := parseTokenAccess(ctx, f.Scopes, f.Projects, f.ExpiresAt)
	if !ok {
		return nil
	}

	existing, err := db.AccessTokens.ListByUserID(c, userID)
	if err != nil {
		logrus.WithContext(c).WithError(err).Error("Failed to list access tokens")
		return ctx.ApiServerError()
	}
	if len(existing) >= maxAccessTokensPerUser {
		return ctx.ApiError(http.StatusConflict, "account::too_many_tokens", maxAccessTokensPerUser)
	}

	token, secret, err := db.AccessTokens.Create(c, db.CreateAccessTokenOptions{
		UserID:     userID,
		Name:       name,
		Scopes:     access.Scopes,
		ProjectIDs: access.ProjectIDs,
		ExpiresAt:  access.ExpiresAt,
		CreatedBy:  createdBy,
	})
	if err != nil {
		logrus.WithContext(c).WithError(err).Error("Failed to create access token")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess(dto.CreateAccessTokenResp{Token: dto.ToAccessToken(token, access.projects, dbutil.Now()), Secret: secret})
}

func revokeAccessToken(ctx context.Context, userID, tokenID int64) error {
	if err := db.AccessTokens.Delete(ctx.Request().Context(), userID, tokenID); err != nil {
		if errors.Is(err, db.ErrAccessTokenNotFound) {
			return ctx.ApiError(http.StatusNotFound, "account::token_not_found")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to revoke access token")
		return ctx.ApiServerError()
	}
	return ctx.Status(http.StatusNoContent)
}
