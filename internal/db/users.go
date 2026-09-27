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

// Users is the default instance of the UsersStore.
var Users UsersStore

// UsersStore is the persistent interface for users.
type UsersStore interface {
	// GetByID returns the user with the given ID.
	// It returns ErrUserNotFound if the user does not exist.
	GetByID(ctx context.Context, userID uint) (*User, error)
	// GetByUID returns the user with the given UID.
	// It returns ErrUserNotFound if the user does not exist.
	GetByUID(ctx context.Context, userUID string) (*User, error)
	// GetByEmail returns the user with the given email.
	// It returns ErrUserNotFound if the user does not exist.
	GetByEmail(ctx context.Context, email string) (*User, error)
	// GetOrCreateDefault returns the earliest created user as the default user, creating one if there is no user.
	GetOrCreateDefault(ctx context.Context) (*User, error)
	// Create creates a new user with the given options.
	// It returns ErrUserAlreadyExisted if the email or GitHub ID has been used.
	Create(ctx context.Context, options CreateUserOptions) (*User, error)
	// Update updates the user with the given ID.
	// It returns ErrUserNotFound if the user does not exist.
	Update(ctx context.Context, id uint, options UpdateUserOptions) error
	// DeleteByID deletes the user with the given ID.
	DeleteByID(ctx context.Context, id uint) error
}

func NewUsersStore(db *gorm.DB) UsersStore {
	return &users{db}
}

// User is a user of the application.
type User struct {
	// Model contains the primary key and the creation, update and deletion times.
	dbutil.Model
	// Email is the unique email of the user.
	Email string `gorm:"uniqueIndex:idx_user_email, where:deleted_at IS NULL" json:"email"`
	// EmailMd5 is the MD5 hash of the lowercase email, which is used for the Gravatar avatar.
	EmailMd5 string `json:"emailMd5"`
	// UserName is the display name of the user.
	UserName string `json:"userName"`
	// GitHubID is the unique GitHub account ID of the user, empty if the user does not sign in with GitHub.
	GitHubID string `gorm:"uniqueIndex:idx_user_github_id, where:deleted_at IS NULL" json:"githubID"`
	// AccessToken is the OAuth access token of the user, it is never exposed.
	AccessToken string `json:"-"`
}

type users struct {
	// DB is the database connection the store operates on.
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

const (
	DefaultUserEmail = "admin@sayrud.local"
	DefaultUserName  = "admin"
)

// GetOrCreateDefault returns the earliest created user as the default user, creating one if the table is empty.
func (db *users) GetOrCreateDefault(ctx context.Context) (*User, error) {
	var user User
	err := db.WithContext(ctx).Model(&User{}).Order("id ASC").First(&user).Error
	if err == nil {
		return &user, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.Wrap(err, "get first user")
	}

	created, err := db.Create(ctx, CreateUserOptions{
		Email:    DefaultUserEmail,
		UserName: DefaultUserName,
	})
	if err != nil {
		// A concurrent request may have created the default user first.
		if errors.Is(err, ErrUserAlreadyExisted) {
			return db.GetByEmail(ctx, DefaultUserEmail)
		}
		return nil, errors.Wrap(err, "create default user")
	}
	return created, nil
}

// CreateUserOptions are the options of creating a user.
type CreateUserOptions struct {
	// Email is the unique email of the user.
	Email string
	// UserName is the display name of the user.
	UserName string
	// GitHubID is the unique GitHub account ID of the user, it can be empty.
	GitHubID string
	// AccessToken is the OAuth access token of the user, it can be empty.
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

// UpdateUserOptions are the options of updating a user.
type UpdateUserOptions struct {
	// UserName is the new display name of the user.
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
