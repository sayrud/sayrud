// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package apibuilder

type Validators []Validator

func (v Validators) ValidateConfig() error {
	// No custom validator implementations remain; reject persisted rules as well.
	if len(v) > 0 {
		return ErrUnsupportedCustomValidators
	}
	return nil
}

type Validator struct {
	Type       string `json:"type"`
	Expression string `json:"expression"`
	Message    string `json:"message"`
}
