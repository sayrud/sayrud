// Copyright 2023 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package dbutil

import (
	"time"

	"github.com/thanhpk/randstr"
	"gorm.io/gorm"
)

type Model struct {
	ID        uint           `gorm:"primarykey" json:"-"`
	UID       string         `gorm:"uniqueIndex" json:"uid"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"-"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (m *Model) BeforeCreate(_ *gorm.DB) error {
	if m.UID == "" {
		m.UID = randstr.String(8)
	}
	return nil
}
