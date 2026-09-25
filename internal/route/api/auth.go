// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package api

import (
	"github.com/sirupsen/logrus"

	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
)

var Auth authRoute

type authRoute struct{}

// Authenticator treats every request as signed in by the default user and maps it as *db.User.
func (authRoute) Authenticator(ctx context.Context) error {
	user, err := db.Users.GetOrCreateDefault(ctx.Request().Context())
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get default user")
		return ctx.ApiServerError()
	}

	ctx.Map(user)
	return nil
}

func (authRoute) Profile(ctx context.Context, user *db.User) error {
	return ctx.ApiSuccess(user)
}
