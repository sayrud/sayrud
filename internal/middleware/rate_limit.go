// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package middleware

import (
	"github.com/spf13/cast"

	"github.com/wuhan005/sayrud/internal/context"
)

const TypeRateLimit Type = "rate_limit"

var _ Handler = (*rateLimit)(nil)

type rateLimit struct {
	Params
}

func (l *rateLimit) Handle(ctx context.Context) error {
	// TODO: Get IP from context defined function.
	ip := ctx.RemoteAddr()
	_ = ip

	policy := cast.ToString(l.Params["policy"])
	countValue := cast.ToInt(l.Params["value"])
	_ = countValue

	switch policy {
	case "per_second":

	case "per_minute":

	}
	return nil
}
