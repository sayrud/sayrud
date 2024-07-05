// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package middleware

import (
	"context"
)

const TypeLog Type = "log"

var _ Handler = (*log)(nil)

type log struct {
	Params
}

func (l *log) Handle(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}
