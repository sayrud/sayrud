// Copyright 2023 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package dbutil

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"strings"

	"github.com/pkg/errors"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

type Options map[string]interface{}

func (o *Options) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	tmp, ok := value.([]byte)
	if !ok {
		return errors.Errorf("dbutil.Options: Scan wants json.RawMessage but got %T", value)
	}

	data := make(map[string]interface{})
	err := json.Unmarshal(tmp, &data)
	if err != nil {
		return err
	}

	*o = data
	return nil
}

func (o Options) Value() (driver.Value, error) {
	if len(o) == 0 {
		return nil, nil
	}
	b, err := json.Marshal(o)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func (o Options) String() string {
	b, _ := json.Marshal(o)
	return string(b)
}

func (Options) GormDataType() string {
	return "json"
}

func (Options) GormDBDataType(db *gorm.DB, field *schema.Field) string {
	switch db.Dialector.Name() {
	case "sqlite":
		return "JSON"
	case "mysql":
		return "JSON"
	case "postgres":
		return "JSONB"
	}
	return ""
}

func (o Options) GormValue(ctx context.Context, db *gorm.DB) clause.Expr {
	if len(o) == 0 {
		return gorm.Expr("'{}'")
	}

	data, _ := json.Marshal(o)

	switch db.Dialector.Name() {
	case "mysql":
		if v, ok := db.Dialector.(*mysql.Dialector); ok && !strings.Contains(v.ServerVersion, "MariaDB") {
			return gorm.Expr("CAST(? AS JSON)", string(data))
		}
	}

	return gorm.Expr("?", string(data))
}

var _ sql.Scanner = (*Options)(nil)
var _ driver.Valuer = (*Options)(nil)
