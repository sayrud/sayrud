// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package form

import (
	"encoding/json"

	"github.com/wuhan005/sayrud/internal/ai"
)

type AIAdvice struct {
	Action   ai.ActionType `json:"action"`
	Messages []*struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
}

type AIApply struct {
	Action     ai.ActionType   `json:"action"`
	ActionJson json.RawMessage `json:"actionJson"`
}
