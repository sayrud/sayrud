// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package apibuilder

import (
	"context"

	"github.com/wuhan005/sayrud/internal/middleware"
)

var _ ConfigValidator = (*Middleware)(nil)

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
