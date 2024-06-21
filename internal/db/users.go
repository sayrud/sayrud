// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package db

import (
	"context"

	"github.com/pkg/errors"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/dbutil"
)

var _ UsersStore = (*users)(nil)

var Users UsersStore

type UsersStore interface {
	GetByID(ctx context.Context, userID uint) (*User, error)
	GetByUID(ctx context.Context, userUID string) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	Create(ctx context.Context, options CreateUserOptions) (*User, error)
	Update(ctx context.Context, id uint, options UpdateUserOptions) error
	DeleteByID(ctx context.Context, id uint) error
}

func NewUsersStore(db *gorm.DB) UsersStore {
	return &users{db}
}

type User struct {
	dbutil.Model
	Email       string `gorm:"uniqueIndex:idx_user_email" json:"email"`
	AccessToken string `json:"-"`
}

type users struct {
	*gorm.DB
}

func (db *users) GetByID(ctx context.Context, userID uint) (*User, error) {
	return db.getBy(ctx, "id = ?", userID)
}

func (db *users) GetByUID(ctx context.Context, userUID string) (*User, error) {
	return db.getBy(ctx, "uid = ?", userUID)
}

func (db *users) GetByEmail(ctx context.Context, email string) (*User, error) {
	return db.getBy(ctx, "email = ?", email)
}

var ErrUserNotFound = errors.New("user dose not exist")

func (db *users) getBy(ctx context.Context, where string, args ...interface{}) (*User, error) {
	var user User
	if err := db.WithContext(ctx).Model(&User{}).Where(where, args...).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

type CreateUserOptions struct {
	Email       string
	AccessToken string
}

var ErrUserAlreadyExisted = errors.New("user already existed")

func (db *users) Create(ctx context.Context, options CreateUserOptions) (*User, error) {
	user := &User{
		Email:       options.Email,
		AccessToken: options.AccessToken,
	}
	if err := db.WithContext(ctx).Create(user).Error; err != nil {
		if dbutil.IsUniqueViolation(err, "idx_user_email") {
			return nil, ErrUserAlreadyExisted
		}
		return nil, err
	}
	return user, nil
}

type UpdateUserOptions struct {
	AccessToken string
}

func (db *users) Update(ctx context.Context, id uint, options UpdateUserOptions) error {
	user, err := db.GetByID(ctx, id)
	if err != nil {
		return errors.Wrap(err, "get by ID")
	}

	user.AccessToken = options.AccessToken
	return db.WithContext(ctx).Save(user).Error
}

func (db *users) DeleteByID(ctx context.Context, id uint) error {
	return db.WithContext(ctx).Delete(&User{}, id).Error
}
