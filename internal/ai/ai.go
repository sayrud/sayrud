// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package ai

import (
	"context"
	"encoding/json"

	"github.com/cockroachdb/errors"
)

type AIAdvice struct {
	RawContent string

	Action      string
	ActionJSON  json.RawMessage
	Description string
}

type Message struct {
	Role    string
	Content string
}

type AIAdvicer interface {
	Advice(ctx context.Context, action ActionType, msg []*Message) (*AIAdvice, error)
}

var ErrActionNotFound = errors.New("action does not exist")
