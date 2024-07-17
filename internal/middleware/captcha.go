// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cast"

	"github.com/wuhan005/sayrud/internal/context"
)

const TypeTurnstileCaptcha Type = "turnstile_captcha"

var _ Handler = (*turnstileCaptcha)(nil)

type turnstileCaptcha struct {
	Params
}

func (t *turnstileCaptcha) Handle(ctx context.Context) error {
	secretKey := cast.ToString(t.Params["secretKey"])
	response := ctx.Query("turnstile")

	requestBody, err := json.Marshal(map[string]string{
		"response": response,
		"secret":   secretKey,
	})
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to marshal request body")
		return ctx.ApiServerError()
	}

	request, err := http.NewRequest(http.MethodPost, "https://challenges.cloudflare.com/turnstile/v0/siteverify", bytes.NewBuffer(requestBody))
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to create request")
		return ctx.ApiServerError()
	}
	request.Header.Set("Content-Type", "application/json")

	client := http.Client{}
	resp, err := client.Do(request)
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to send request")
		return ctx.ApiServerError()
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return ctx.ApiError(http.StatusForbidden, "验证码校验失败，请尝试刷新页面后提交")
	}

	var responseBody = struct {
		Success bool `json:"success"`
	}{}
	if err := json.NewDecoder(resp.Body).Decode(&responseBody); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to decode response")
		return ctx.ApiServerError()
	}

	if !responseBody.Success {
		return ctx.ApiError(http.StatusForbidden, "验证码校验失败，请尝试刷新页面后提交")
	}
	return nil
}
