// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package apibuilder

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/dop251/goja"
	"github.com/pkg/errors"
	"github.com/spf13/cast"
	"gorm.io/gorm/clause"
)

type OperatorType string

const (
	// Binary operators
	OperatorTypeAnd          OperatorType = "AND"
	OperatorTypeOr           OperatorType = "OR"
	OperatorTypeNotEqual     OperatorType = "<>"
	OperatorTypeEqual        OperatorType = "="
	OperatorTypeGreater      OperatorType = ">"
	OperatorTypeLess         OperatorType = "<"
	OperatorTypeGreaterEqual OperatorType = ">="
	OperatorTypeLessEqual    OperatorType = "<="
	OperatorTypeLike         OperatorType = "LIKE"
	OperatorTypeIn           OperatorType = "IN"

	// Unary operators
	OperatorTypeNot OperatorType = "NOT"

	OperatorTypeField   OperatorType = "FIELD"
	OperatorTypeLiteral OperatorType = "LITERAL"
)

type Operator struct {
	Type  OperatorType    `json:"t"`
	Value json.RawMessage `json:"v,omitempty"`
	Left  *Operator       `json:"l,omitempty"`
	Right *Operator       `json:"r,omitempty"`
}

func (o *Operator) ValidateConfig(ctx context.Context) error {
	if o == nil {
		return nil
	}

	switch o.Type {
	case OperatorTypeAnd, OperatorTypeOr:
		if o.Left == nil || o.Right == nil {
			return errors.New("empty left or right")
		}
		if err := o.Left.ValidateConfig(ctx); err != nil {
			return err
		}
		if err := o.Right.ValidateConfig(ctx); err != nil {
			return err
		}

	case OperatorTypeNotEqual, OperatorTypeEqual, OperatorTypeGreater, OperatorTypeLess, OperatorTypeGreaterEqual, OperatorTypeLessEqual, OperatorTypeLike, OperatorTypeIn:
		if o.Left == nil || o.Right == nil {
			return errors.New("empty left or right")
		}
		if err := o.Left.ValidateConfig(ctx); err != nil {
			return err
		}
		if err := o.Right.ValidateConfig(ctx); err != nil {
			return err
		}

	case OperatorTypeNot:
		if o.Left == nil {
			return errors.New("empty left")
		}
		if err := o.Left.ValidateConfig(ctx); err != nil {
			return err
		}

	case OperatorTypeField:
		var field string
		if err := json.Unmarshal(o.Value, &field); err != nil {
			return err
		}
		if field == "" {
			return errors.New("empty field")
		}

	case OperatorTypeLiteral:
		var value interface{}
		if err := json.Unmarshal(o.Value, &value); err != nil {
			return err
		}
		if cast.ToString(value) == "" {
			return errors.New("empty value")
		}
	}
	return nil
}

func (o *Operator) SetFieldUIDToName(fieldUIDNameSets map[string]string) error {
	if o == nil {
		return nil
	}

	if o.Type == OperatorTypeField {
		var fieldUID string
		if err := json.Unmarshal(o.Value, &fieldUID); err != nil {
			return err
		}
		if fieldName, ok := fieldUIDNameSets[fieldUID]; ok {
			o.Value = json.RawMessage(`"` + fieldName + `"`)
		}
	}
	if err := o.Left.SetFieldUIDToName(fieldUIDNameSets); err != nil {
		return errors.Wrap(err, "left")
	}
	if err := o.Right.SetFieldUIDToName(fieldUIDNameSets); err != nil {
		return errors.Wrap(err, "right")
	}

	return nil
}

func (o *Operator) ToClauseExpression(vm *goja.Runtime) (clause.Expression, error) {
	if o == nil {
		return nil, nil
	}

	left, err := o.Left.ToClauseExpression(vm)
	if err != nil {
		return nil, err
	}
	right, err := o.Right.ToClauseExpression(vm)
	if err != nil {
		return nil, err
	}

	switch o.Type {
	case OperatorTypeAnd:
		return &clause.AndConditions{Exprs: []clause.Expression{left, right}}, nil

	case OperatorTypeOr:
		return &clause.OrConditions{Exprs: []clause.Expression{left, right}}, nil

	case OperatorTypeNotEqual:
		return clause.Neq{Column: left, Value: right}, nil

	case OperatorTypeEqual:
		return clause.Eq{Column: left, Value: right}, nil

	case OperatorTypeGreater:
		return clause.Gt{Column: left, Value: right}, nil

	case OperatorTypeLess:
		return clause.Lt{Column: left, Value: right}, nil

	case OperatorTypeGreaterEqual:
		return clause.Gte{Column: left, Value: right}, nil

	case OperatorTypeLessEqual:
		return clause.Lte{Column: left, Value: right}, nil

	case OperatorTypeLike:
		return clause.Like{Column: left, Value: right}, nil

	case OperatorTypeIn:
		return clause.IN{Column: left, Values: []interface{}{right}}, nil

	case OperatorTypeNot:
		return clause.NotConditions{Exprs: []clause.Expression{left}}, nil

	case OperatorTypeField:
		var field string
		if err := json.Unmarshal(o.Value, &field); err != nil {
			return nil, err
		}
		return clause.Expr{SQL: field}, nil

	case OperatorTypeLiteral:
		var value interface{}
		if err := json.Unmarshal(o.Value, &value); err != nil {
			return nil, err
		}

		strValue := cast.ToString(value)
		if strings.HasPrefix(strValue, "$") {
			result, err := vm.RunString(strValue)
			if err != nil {
				return nil, errors.Wrap(err, "run string")
			}
			value = result.Export()
		}

		return clause.Expr{
			SQL:  "?",
			Vars: []interface{}{value},
		}, nil
	}
	return nil, nil
}
