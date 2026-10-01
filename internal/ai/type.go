// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package ai

type ActionType string

const (
	ActionTypeTables = "tables"
)

type ApplyTables []*ApplyTable

type ApplyTable struct {
	TableName  string `json:"tableName" valid:"required"`
	TableLabel string `json:"tableLabel" valid:"required"`
}
