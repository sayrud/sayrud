// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package middleware

import (
	"github.com/wuhan005/sayrud/internal/context"
)

const TypeSendEmail Type = "send_email"

var _ Handler = (*sendEmail)(nil)

type sendEmail struct {
	Params
}

func (l *sendEmail) Handle(ctx context.Context) error {
	return nil
}
