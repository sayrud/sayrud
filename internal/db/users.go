// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package db

import (
	"context"
	"strings"

	"github.com/pkg/errors"
	"github.com/wuhan005/gadget"
	"golang.org/x/crypto/bcrypt"
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
	GetByID(ctx context.Context, userID int64) (*User, error)
	// GetByEmail returns the user with the given email, the email is case-insensitive.
	// It returns ErrUserNotFound if the user does not exist.
	GetByEmail(ctx context.Context, email string) (*User, error)
	// ListByIDs returns the users with the given IDs, the missing ones are omitted.
	ListByIDs(ctx context.Context, userIDs []int64) ([]*User, error)
	// Authenticate returns the user with the given email and password.
	// It returns ErrBadCredential if the email does not exist or the password is wrong.
	Authenticate(ctx context.Context, email, password string) (*User, error)
	// Create creates a new user with the given options.
	// It returns ErrUserAlreadyExisted if the email has been used.
	Create(ctx context.Context, options CreateUserOptions) (*User, error)
	// ClaimLegacyDefault lets the new user take over the passwordless default user of the versions without accounts, along with its projects.
	// It returns ErrUserNotFound if there is no such user, and ErrUserAlreadyExisted if the email has been used.
	ClaimLegacyDefault(ctx context.Context, options CreateUserOptions) (*User, error)
	// Update updates the user with the given ID.
	// It returns ErrUserNotFound if the user does not exist.
	Update(ctx context.Context, id int64, options UpdateUserOptions) error
	// UpdatePassword sets the password of the user with the given ID.
	UpdatePassword(ctx context.Context, id int64, password string) error
	// DeleteByID deletes the user with the given ID.
	DeleteByID(ctx context.Context, id int64) error
}

func NewUsersStore(db *gorm.DB) UsersStore {
	return &users{db}
}

// User is a user of the application.
type User struct {
	// Model contains the primary key and the creation, update and deletion times.
	dbutil.Model
	// Email is the unique lowercase email of the user.
	Email string `gorm:"uniqueIndex:idx_user_email, where:deleted_at IS NULL" json:"email"`
	// EmailMd5 is the MD5 hash of the lowercase email, which is used for the Gravatar avatar.
	EmailMd5 string `json:"emailMd5"`
	// UserName is the display name of the user.
	UserName string `json:"userName"`
	// Password is the bcrypt hash of the password, empty if the user can not sign in with a password.
	Password string `json:"-"`
}

type users struct {
	// DB is the database connection the store operates on.
	*gorm.DB
}

// NormalizeEmail returns the email in the stored form.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func (db *users) GetByID(ctx context.Context, userID int64) (*User, error) {
	return db.getBy(ctx, "id = ?", userID)
}

func (db *users) GetByEmail(ctx context.Context, email string) (*User, error) {
	return db.getBy(ctx, "email = ?", NormalizeEmail(email))
}

func (db *users) ListByIDs(ctx context.Context, userIDs []int64) ([]*User, error) {
	var users []*User
	if len(userIDs) == 0 {
		return users, nil
	}
	return users, db.WithContext(ctx).Where("id IN ?", userIDs).Find(&users).Error
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

var ErrBadCredential = errors.New("bad credential")

func (db *users) Authenticate(ctx context.Context, email, password string) (*User, error) {
	user, err := db.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			// Spend the same time as a wrong password, so the response time does not reveal whether the email is registered.
			_ = bcrypt.CompareHashAndPassword(dummyPasswordHash, []byte(password))
			return nil, ErrBadCredential
		}
		return nil, errors.Wrap(err, "get by email")
	}
	if user.Password == "" || bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)) != nil {
		return nil, ErrBadCredential
	}
	return user, nil
}

var dummyPasswordHash, _ = bcrypt.GenerateFromPassword([]byte("sayrud-dummy-password"), bcrypt.DefaultCost)

func hashPassword(password string) (string, error) {
	if password == "" {
		return "", nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", errors.Wrap(err, "hash password")
	}
	return string(hash), nil
}

var ErrUserAlreadyExisted = errors.New("user already existed")

// LegacyDefaultUserEmail is the email of the default user shared by all the visitors before accounts were introduced.
const LegacyDefaultUserEmail = "admin@sayrud.local"

// CreateUserOptions are the options of creating a user.
type CreateUserOptions struct {
	// Email is the unique email of the user.
	Email string
	// UserName is the display name of the user.
	UserName string
	// Password is the plaintext password, it can be empty.
	Password string
}

func (db *users) Create(ctx context.Context, options CreateUserOptions) (*User, error) {
	password, err := hashPassword(options.Password)
	if err != nil {
		return nil, err
	}

	email := NormalizeEmail(options.Email)
	user := &User{
		Email:    email,
		EmailMd5: gadget.Md5(email),
		UserName: options.UserName,
		Password: password,
	}
	if err := db.WithContext(ctx).Create(user).Error; err != nil {
		if dbutil.IsUniqueViolation(err, "idx_user_email") {
			return nil, ErrUserAlreadyExisted
		}
		return nil, err
	}
	return user, nil
}

func (db *users) ClaimLegacyDefault(ctx context.Context, options CreateUserOptions) (*User, error) {
	password, err := hashPassword(options.Password)
	if err != nil {
		return nil, err
	}

	email := NormalizeEmail(options.Email)
	// The conditional update ensures only one of the concurrent sign-ups takes it over.
	result := db.WithContext(ctx).Model(&User{}).
		Where("email = ? AND COALESCE(password, '') = ''", LegacyDefaultUserEmail).
		Updates(map[string]interface{}{
			"email":     email,
			"email_md5": gadget.Md5(email),
			"user_name": options.UserName,
			"password":  password,
		})
	if result.Error != nil {
		if dbutil.IsUniqueViolation(result.Error, "idx_user_email") {
			return nil, ErrUserAlreadyExisted
		}
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, ErrUserNotFound
	}
	return db.GetByEmail(ctx, email)
}

// UpdateUserOptions are the options of updating a user.
type UpdateUserOptions struct {
	// UserName is the new display name of the user.
	UserName string
}

func (db *users) Update(ctx context.Context, id int64, options UpdateUserOptions) error {
	user, err := db.GetByID(ctx, id)
	if err != nil {
		return errors.Wrap(err, "get by ID")
	}

	user.UserName = options.UserName
	return db.WithContext(ctx).Save(user).Error
}

func (db *users) UpdatePassword(ctx context.Context, id int64, password string) error {
	hash, err := hashPassword(password)
	if err != nil {
		return err
	}
	return db.WithContext(ctx).Model(&User{}).Where("id = ?", id).Update("password", hash).Error
}

func (db *users) DeleteByID(ctx context.Context, id int64) error {
	return db.WithContext(ctx).Delete(&User{}, id).Error
}
