// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package redis

import (
	"context"

	"github.com/cockroachdb/errors"
	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"

	"github.com/wuhan005/sayrud/internal/conf"
)

var rdb *redis.Client

func Init() (*redis.Client, error) {
	rdb = redis.NewClient(&redis.Options{
		Addr:     conf.Redis.Address,
		Username: conf.Redis.Username,
		Password: conf.Redis.Password,
	})

	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, errors.Wrap(err, "ping")
	}
	if err := redisotel.InstrumentTracing(rdb); err != nil {
		return nil, errors.Wrap(err, "instrument tracing")
	}
	if err := redisotel.InstrumentMetrics(rdb); err != nil {
		return nil, errors.Wrap(err, "instrument metrics")
	}

	SetRedisStore(rdb)
	return rdb, nil
}

// SetRedisStore sets the Redis stores.
func SetRedisStore(client *redis.Client) {
	AuthAttempts = NewAuthAttemptsStore(client)
	SSOStates = NewSSOStatesStore(client)
}

func Get() *redis.Client {
	return rdb
}
