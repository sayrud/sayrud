// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package apibuilder

import (
	"context"

	"github.com/pkg/errors"
	"github.com/spf13/cast"
)

type ConfigValidator interface {
	ValidateConfig(ctx context.Context) error
}

type ctxProjectID struct{}

func WithProjectID(ctx context.Context, projectID int64) context.Context {
	return context.WithValue(ctx, ctxProjectID(struct{}{}), projectID)
}

func ParseProjectID(ctx context.Context) int64 {
	return cast.ToInt64(ctx.Value(ctxProjectID(struct{}{})))
}

type ctxKind struct{}

func WithKind(ctx context.Context, kind Kind) context.Context {
	return context.WithValue(ctx, ctxKind(struct{}{}), string(kind))
}

func ParseKind(ctx context.Context) Kind {
	return Kind(cast.ToString(ctx.Value(ctxKind(struct{}{}))))
}

var (
	ErrEmptyParamKey      = errors.New("empty param key")
	ErrInvalidParamType   = errors.New("invalid param type")
	ErrDuplicateParamKey  = errors.New("duplicate param key")
	ErrParamValueRequired = errors.New("param value required")

	ErrEmptyValidatorExpression = errors.New("empty validator expression")

	ErrDatasetTableNotFound = errors.New("dataset table not found")
	ErrEmptyDatasets        = errors.New("empty datasets")
	ErrEmptyDatasetTableUID = errors.New("empty dataset table UID")
	ErrEmptyDatasetFields   = errors.New("empty dataset fields")
	ErrFieldNotFound        = errors.New("field not found")

	ErrInvalidKind = errors.New("invalid kind")

	ErrInvalidMiddlewareType = errors.New("invalid middleware type")
)
