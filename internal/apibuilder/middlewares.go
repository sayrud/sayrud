// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package apibuilder

import (
	"context"
	"encoding/json"

	"github.com/wuhan005/sayrud/internal/middleware"
)

var _ ConfigValidator = (*Middleware)(nil)

type Middlewares []Middleware

func (m Middlewares) ValidateConfig(ctx context.Context) error {
	for _, mw := range m {
		if err := mw.ValidateConfig(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (m Middlewares) ToJSON() []byte {
	if len(m) == 0 {
		return []byte("[]")
	}
	b, _ := json.Marshal(m)
	return b
}

func ParseMiddlewares(middlewares []byte) Middlewares {
	var m Middlewares
	_ = json.Unmarshal(middlewares, &m)
	return m
}

type Middleware struct {
	Type   middleware.Type        `json:"type"`
	Params map[string]interface{} `json:"params"`
}

func (m *Middleware) ValidateConfig(ctx context.Context) error {
	if !m.Type.IsValid() {
		return ErrInvalidMiddlewareType
	}
	return nil
}
