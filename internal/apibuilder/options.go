// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package apibuilder

import (
	"encoding/json"
)

type Kind string

func (k Kind) ValidateConfig() error {
	switch k {
	case KindList, KindView, KindCreate, KindUpdate, KindDelete:
		return nil
	default:
		return ErrInvalidKind
	}
}

const (
	KindList   Kind = "list"
	KindView   Kind = "view"
	KindCreate Kind = "create"
	KindUpdate Kind = "update"
	KindDelete Kind = "delete"
)

type Options[T ListOptions | ViewOptions | CreateOptions | UpdateOptions | DeleteOptions] struct {
	value T
}

func (o Options[T]) ParseOptions(options []byte) T {
	var v T
	_ = json.Unmarshal(options, &v)
	return v
}

type ListOptions struct {
	Datasets Datasets `json:"datasets"`
}

type ViewOptions struct {
	Dataset Dataset `json:"dataset"`
}

type CreateOptions struct {
	TableUID string `json:"tableUID"`
	// FieldMapping maps given request key to actual database field uid.
	FieldMapping map[string]string `json:"fieldMapping"`
}

type UpdateOptions struct {
	TableUID     string            `json:"tableUID"`
	FieldMapping map[string]string `json:"fieldMapping"`
	Filter       *Operator         `json:"filter"`
}

type DeleteOptions struct {
	TableUID string    `json:"tableUID"`
	Filter   *Operator `json:"filter"`
}
