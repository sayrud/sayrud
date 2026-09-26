// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package form

import (
	"encoding/json"

	"github.com/wuhan005/sayrud/internal/ai"
)

type AIAdvice struct {
	Action   ai.ActionType `json:"action" swaggertype:"string" enums:"tables"`
	Messages []*AIMessage  `json:"messages"`
} // @name AIAdvice

type AIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
} // @name AIMessage

type AIApply struct {
	Action ai.ActionType `json:"action" swaggertype:"string" enums:"tables"`
	// ActionJson is the actionJson returned by the advice API.
	ActionJson json.RawMessage `json:"actionJson" swaggertype:"object"`
} // @name AIApply
