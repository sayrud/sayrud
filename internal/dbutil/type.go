// Copyright 2023 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package dbutil

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"

	"github.com/pkg/errors"
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

func (o *Options) Value() (driver.Value, error) {
	return json.Marshal(o)
}

var _ sql.Scanner = (*Options)(nil)
var _ driver.Valuer = (*Options)(nil)
