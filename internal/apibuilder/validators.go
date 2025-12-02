// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package apibuilder

const (
	ValidatorTypeTextModeration = "text_moderation"
)

type Validators []Validator

func (v Validators) ValidateConfig() error {
	for _, validator := range v {
		if err := validator.ValidateConfig(); err != nil {
			return err
		}
	}
	return nil
}

type Validator struct {
	Type       string `json:"type"`
	Expression string `json:"expression"`
	Message    string `json:"message"`
}

func (v Validator) ValidateConfig() error {
	if v.Expression == "" {
		return ErrEmptyValidatorExpression
	}
	return nil
}

type ValidatorError struct {
	Message string
}

func (e ValidatorError) Error() string {
	return e.Message
}
