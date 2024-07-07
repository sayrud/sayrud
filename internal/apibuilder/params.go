// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package apibuilder

import (
	"context"
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/spf13/cast"
)

type ParamType string

func (t ParamType) ValidateConfig() error {
	switch t {
	case ParamTypeString, ParamTypeNumber, ParamTypeBoolean:
		return nil
	default:
		return ErrInvalidParamType
	}
}

func (t ParamType) String() string {
	return string(t)
}

const (
	ParamTypeString  ParamType = "string"
	ParamTypeNumber  ParamType = "number"
	ParamTypeBoolean ParamType = "boolean"
)

type Param struct {
	Key              string     `json:"key"`
	Label            string     `json:"label"`
	Type             ParamType  `json:"type"`
	Required         bool       `json:"required"`
	CustomValidators Validators `json:"customValidators"`
}

func (p Param) ValidateValue(inputValue string) (interface{}, error) {
	if inputValue == "" && p.Required {
		return nil, ErrParamValueRequired
	}

	var v interface{}
	switch p.Type {
	case ParamTypeString:
		v = inputValue
	case ParamTypeNumber:

		v = cast.ToInt64(inputValue)
	case ParamTypeBoolean:
		v = cast.ToBool(inputValue)
	default:
		v = inputValue
	}

	// TODO: custom validator

	return v, nil
}

func (p Param) ValidateConfig() error {
	if p.Key == "" {
		return ErrEmptyParamKey
	}
	if err := p.Type.ValidateConfig(); err != nil {
		return errors.Wrap(err, "validate type")
	}
	if err := p.CustomValidators.ValidateConfig(); err != nil {
		return errors.Wrap(err, "validate custom validators")
	}
	return nil
}

type QueryParams []QueryParam

func ParseQueryParams(b []byte) (QueryParams, error) {
	var params QueryParams
	if err := json.Unmarshal(b, &params); err != nil {
		return nil, errors.Wrap(err, "unmarshal")
	}
	return params, nil
}

func (p QueryParams) ValidateConfig(ctx context.Context) error {
	keys := make(map[string]struct{})
	for _, param := range p {
		if err := param.ValidateConfig(); err != nil {
			return err
		}
		if _, ok := keys[param.Key]; ok {
			return ErrDuplicateParamKey
		}
		keys[param.Key] = struct{}{}
	}
	return nil
}

func (p QueryParams) ToJSON() []byte {
	b, _ := json.Marshal(p)
	return b
}

type QueryParam struct {
	Param `json:",inline"`
}

type BodyParams []BodyParam

func ParseBodyParams(b []byte) (BodyParams, error) {
	var params BodyParams
	if err := json.Unmarshal(b, &params); err != nil {
		return nil, errors.Wrap(err, "unmarshal")
	}
	return params, nil
}

func (p BodyParams) ValidateConfig(ctx context.Context) error {
	keys := make(map[string]struct{})
	for _, param := range p {
		if err := param.ValidateConfig(); err != nil {
			return err
		}
		if _, ok := keys[param.Key]; ok {
			return ErrDuplicateParamKey
		}
		keys[param.Key] = struct{}{}
	}
	return nil
}

func (p BodyParams) ToJSON() []byte {
	b, _ := json.Marshal(p)
	return b
}

type BodyParam struct {
	Param `json:",inline"`
}
