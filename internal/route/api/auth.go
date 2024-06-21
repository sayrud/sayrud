// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/go-github/v62/github"
	"github.com/sirupsen/logrus"

	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
)

var Auth authRoute

type authRoute struct{}

func (authRoute) GitHubCallback(ctx context.Context) error {
	code := ctx.Query("code")

	form := url.Values{}
	form.Set("client_id", os.Getenv("GITHUB_CLIENT_ID"))
	form.Set("client_secret", os.Getenv("GITHUB_CLIENT_SECRET"))
	form.Set("code", code)

	req, err := http.NewRequest(http.MethodPost, "https://github.com/login/oauth/access_token", strings.NewReader(form.Encode()))
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to create request")
		return ctx.ApiServerError()
	}
	req.Header.Set("Accept", "application/json")

	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to send request")
		return ctx.ApiServerError()
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		logrus.WithContext(ctx.Request().Context()).Errorf("Failed to get access token, status: %s", resp.Status)
		return ctx.ApiServerError()
	}

	var response struct {
		Error       string `json:"error"`
		AccessToken string `json:"access_token"`
		Scope       string `json:"scope"`
		TokenType   string `json:"token_type"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to decode response")
		return ctx.ApiServerError()
	}

	if response.Error != "" {
		return ctx.ApiError(http.StatusUnauthorized, "凭证无效")
	}

	accessToken := response.AccessToken
	fmt.Println(accessToken)
	githubClient := github.NewClient(nil).WithAuthToken(accessToken)
	githubUser, _, err := githubClient.Users.Get(ctx.Request().Context(), "")
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get user")
		return ctx.ApiServerError()
	}

	email := githubUser.GetEmail()
	userName := githubUser.GetName()
	githubID := githubUser.GetLogin()

	user, err := db.Users.Upsert(ctx.Request().Context(), db.UpsertUserOptions{
		Email:       email,
		UserName:    userName,
		GitHubID:    githubID,
		AccessToken: accessToken,
	})
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to upsert user")
		return ctx.ApiServerError()
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"userUID": user.UID,
	})
	tokenString, err := token.SignedString([]byte(jwtSecret))
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to sign token")
		return ctx.ApiServerError()
	}

	return ctx.ApiSuccess(map[string]interface{}{
		"token": tokenString,
	})
}
