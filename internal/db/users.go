// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package db

import (
	"context"
	"strings"

	"github.com/pkg/errors"
	"github.com/wuhan005/gadget"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/dbutil"
)

var _ UsersStore = (*users)(nil)

var Users UsersStore

type UsersStore interface {
	GetByID(ctx context.Context, userID uint) (*User, error)
	GetByUID(ctx context.Context, userUID string) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	Upsert(ctx context.Context, options UpsertUserOptions) (*User, error)
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
	EmailMd5    string `json:"emailMd5"`
	UserName    string `json:"userName"`
	GitHubID    string `gorm:"uniqueIndex:idx_user_github_id" json:"githubID"`
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

var ErrUserAlreadyExisted = errors.New("user already existed")

type UpsertUserOptions struct {
	Email       string
	UserName    string
	GitHubID    string
	AccessToken string
}

func (db *users) Upsert(ctx context.Context, options UpsertUserOptions) (*User, error) {
	u := &User{
		Email:       options.Email,
		EmailMd5:    gadget.Md5(strings.ToLower(options.Email)),
		UserName:    options.UserName,
		GitHubID:    options.GitHubID,
		AccessToken: options.AccessToken,
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.WithContext(ctx).FirstOrCreate(u, "email = ?", options.Email).Error; err != nil {
			return errors.Wrap(err, "first or create")
		}

		u.AccessToken = options.AccessToken
		if err := tx.WithContext(ctx).Save(u).Error; err != nil {
			return errors.Wrap(err, "save")
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return u, nil
}

type CreateUserOptions struct {
	Email       string
	UserName    string
	GitHubID    string
	AccessToken string
}

func (db *users) Create(ctx context.Context, options CreateUserOptions) (*User, error) {
	user := &User{
		Email:       options.Email,
		EmailMd5:    gadget.Md5(strings.ToLower(options.Email)),
		UserName:    options.UserName,
		GitHubID:    options.GitHubID,
		AccessToken: options.AccessToken,
	}
	if err := db.WithContext(ctx).Create(user).Error; err != nil {
		if dbutil.IsUniqueViolation(err, "idx_user_email") || dbutil.IsUniqueViolation(err, "idx_user_github_id") {
			return nil, ErrUserAlreadyExisted
		}
		return nil, err
	}
	return user, nil
}

type UpdateUserOptions struct {
	UserName string
}

func (db *users) Update(ctx context.Context, id uint, options UpdateUserOptions) error {
	user, err := db.GetByID(ctx, id)
	if err != nil {
		return errors.Wrap(err, "get by ID")
	}

	user.UserName = options.UserName
	return db.WithContext(ctx).Save(user).Error
}

func (db *users) DeleteByID(ctx context.Context, id uint) error {
	return db.WithContext(ctx).Delete(&User{}, id).Error
}
