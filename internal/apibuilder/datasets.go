// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package apibuilder

import (
	"encoding/json"
)

type Datasets []Dataset

var _ ConfigValidator = (*Dataset)(nil)

type Dataset struct {
	TableUID         string    `json:"tableUID"`
	Fields           []string  `json:"fields"`
	Filter           *Operator `json:"filter"`
	Order            []string  `json:"order"`
	LimitExpression  string    `json:"limitExp"`
	OffsetExpression string    `json:"offsetExp"`
}

func (d Dataset) ValidateConfig() error {
	if d.TableUID == "" {
		return ErrEmptyDatasetTableUID
	}
	if len(d.Fields) == 0 {
		return ErrEmptyDatasetFields
	}
	return nil
}

func (d Dataset) ToJSON() []byte {
	b, _ := json.Marshal(d)
	return b
}
