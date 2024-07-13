// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package middleware

import (
	"github.com/pkg/errors"

	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/redis"
)

type Type string

const TypeMain = "main"

func (t Type) IsValid() bool {
	switch t {
	case TypeMain, TypeLog, TypeRateLimit, TypeSendEmail:
		return true
	default:
		return false
	}
}

type Params map[string]any

type Handler interface {
	Handle(ctx context.Context) error
}

var ErrUnknownMiddlewareType = errors.New("unknown middleware type")

func Get(typ Type, params Params) (Handler, error) {
	switch typ {
	case TypeLog:
		return &log{Params: params}, nil
	case TypeRateLimit:
		return &rateLimit{Params: params, redis: redis.Get()}, nil
	case TypeSendEmail:
		return &sendEmail{Params: params}, nil
	default:
		return nil, ErrUnknownMiddlewareType
	}
}
