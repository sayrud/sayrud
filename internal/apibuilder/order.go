// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package apibuilder

import (
	"encoding/json"
)

type Order struct {
	FieldUID  string `json:"fieldUID" valid:"required" label:"字段 UID"`
	OrderType string `json:"orderType" valid:"required;list:asc,desc" label:"排序类型"`
}

func ParseOrder(data []byte) (*Order, error) {
	var o Order
	if err := json.Unmarshal(data, &o); err != nil {
		return nil, err
	}
	return &o, nil
}

func ParseOrders(data []byte) ([]*Order, error) {
	var o []*Order
	if err := json.Unmarshal(data, &o); err != nil {
		return nil, err
	}
	return o, nil
}
