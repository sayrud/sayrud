// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cast"

	"github.com/wuhan005/sayrud/internal/context"
)

const TypeRateLimit Type = "rate_limit"

var _ Handler = (*rateLimit)(nil)

type rateLimit struct {
	Params
	redis *redis.Client
}

func (l *rateLimit) Handle(ctx context.Context) error {
	ip := ctx.IP()

	policy := cast.ToString(l.Params["policy"])
	period := cast.ToInt(l.Params["period"])
	maxCount := cast.ToInt64(l.Params["value"])

	switch policy {
	case "period":
		key := "rate_limit:" + ip + ":period"
		now := time.Now()

		// Remove the expired data.
		min := "0"
		max := strconv.Itoa(int(now.Add(-time.Duration(period) * time.Second).UnixNano()))
		l.redis.ZRemRangeByScore(ctx.Request().Context(), key, min, max)

		currentCount := l.redis.ZCard(ctx.Request().Context(), key).Val()
		if currentCount >= maxCount {
			return ctx.ApiError(http.StatusTooManyRequests, "请求过快，请稍后再试")
		}

		if err := l.redis.ZAdd(ctx.Request().Context(), key, redis.Z{
			Score:  float64(now.UnixNano()),
			Member: now.UnixNano(),
		}).Err(); err != nil {
			logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to ZAdd")
		}
	}
	return nil
}
