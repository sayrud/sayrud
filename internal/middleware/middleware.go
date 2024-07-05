// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package middleware

import (
	"context"

	"github.com/pkg/errors"
)

type Type string

func (t Type) IsValid() bool {
	switch t {
	case TypeLog:
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
		return &log{params}, nil
	default:
		return nil, ErrUnknownMiddlewareType
	}
}
