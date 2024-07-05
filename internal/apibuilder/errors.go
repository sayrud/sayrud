// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package apibuilder

import (
	"github.com/pkg/errors"
)

type ConfigValidator interface {
	ValidateConfig() error
}

var (
	ErrEmptyParamKey      = errors.New("empty param key")
	ErrInvalidParamType   = errors.New("invalid param type")
	ErrDuplicateParamKey  = errors.New("duplicate param key")
	ErrParamValueRequired = errors.New("param value required")

	ErrEmptyValidatorExpression = errors.New("empty validator expression")

	ErrEmptyDatasetTableUID = errors.New("empty dataset table UID")
	ErrEmptyDatasetFields   = errors.New("empty dataset fields")

	ErrInvalidKind = errors.New("invalid kind")

	ErrInvalidMiddlewareType = errors.New("invalid middleware type")
)
