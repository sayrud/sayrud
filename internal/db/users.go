// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package db

import (
	"context"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/wuhan005/gadget"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

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
	// SetAvatar replaces the avatar atomically, returning the updated user and the previous file UID for cleanup.
	SetAvatar(ctx context.Context, id int64, fileUID string) (*User, string, error)
	// UpdatePassword sets the password of the user with the given ID.
	UpdatePassword(ctx context.Context, id int64, password string) error
	// DeleteByID deletes the user with the given ID.
	DeleteByID(ctx context.Context, id int64) error
	// TouchSignIn sets the last sign-in time to now.
	TouchSignIn(ctx context.Context, id int64) error

	// List returns the paginated users along with the total count, the newest first.
	List(ctx context.Context, opts ListUsersOptions) ([]*User, int64, error)
	// Stats returns the user counts.
	Stats(ctx context.Context) (*UserStats, error)
	// Recent returns the limit most recently created users.
	Recent(ctx context.Context, limit int) ([]*User, error)
	// EnsureAdmin makes the user with the smallest ID an admin if there is no enabled admin.
	EnsureAdmin(ctx context.Context) error
	// SetAdmin grants or revokes the admin, it returns ErrLastAdmin when revoking the last enabled admin.
	SetAdmin(ctx context.Context, id int64, isAdmin bool) error
	// SetDisabled disables or enables the user, disabling also deletes all its sessions. It returns ErrLastAdmin when disabling the last enabled admin.
	SetDisabled(ctx context.Context, id int64, disabled bool) error
	// Delete deletes the user along with its collaborator records, sessions, settings and identities, the owned projects are transferred to transferTo.
	// It returns ErrUserOwnsProjects if transferTo is 0 and the user still owns projects, and ErrLastAdmin when deleting the last enabled admin.
	Delete(ctx context.Context, id, transferTo int64) error
}

func NewUsersStore(db *gorm.DB) UsersStore {
	return &users{db}
}

// User is a user of the application.
type User struct {
	// Model contains the primary key and the creation, update and deletion times.
	dbutil.Model
	// Email is the unique lowercase email of the user.
	Email string `gorm:"uniqueIndex:idx_user_email, where:deleted_at IS NULL"`
	// EmailMd5 is the MD5 hash of the lowercase email, which is used for the Gravatar avatar.
	EmailMd5 string
	// UserName is the display name of the user.
	UserName string
	// AvatarFileUID identifies the current uploaded avatar, empty for the default avatar.
	AvatarFileUID string `json:"-"`
	// Password is the bcrypt hash of the password, empty if the user can not sign in with a password.
	Password string `json:"-"`
	// IsAdmin reports whether the user is a system admin.
	IsAdmin bool `gorm:"not null;default:false"`
	// DisabledAt is the time the user was disabled, nil if enabled.
	DisabledAt *time.Time
	// LastSignInAt is the time of the last sign-in.
	LastSignInAt *time.Time
}

// Disabled reports whether the user is disabled.
func (u *User) Disabled() bool {
	return u.DisabledAt != nil
}

// HasPassword reports whether the user has a password, the users created by third-party sign-in have none.
func (u *User) HasPassword() bool {
	return u.Password != ""
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
	result := db.WithContext(ctx).Model(&User{}).Where("id = ?", id).Update("user_name", options.UserName)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrUserNotFound
	}
	return nil
}

func (db *users) SetAvatar(ctx context.Context, id int64, fileUID string) (*User, string, error) {
	var user User
	var previous string

	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrUserNotFound
			}

			return err
		}

		previous = user.AvatarFileUID
		if err := tx.Model(&user).Update("avatar_file_uid", fileUID).Error; err != nil {
			return err
		}
		user.AvatarFileUID = fileUID

		return nil
	})

	return &user, previous, err
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

func (db *users) TouchSignIn(ctx context.Context, id int64) error {
	return db.WithContext(ctx).Model(&User{}).Where("id = ?", id).UpdateColumn("last_sign_in_at", dbutil.Now()).Error
}

// UserStatus filters the user list by status.
type UserStatus string

const (
	UserStatusAll      UserStatus = ""
	UserStatusActive   UserStatus = "active"
	UserStatusDisabled UserStatus = "disabled"
	UserStatusAdmin    UserStatus = "admin"
)

// ListUsersOptions are the options of listing users.
type ListUsersOptions struct {
	dbutil.Pagination
	// Keyword matches the user name or email case-insensitively.
	Keyword string
	Status  UserStatus
}

func (db *users) List(ctx context.Context, opts ListUsersOptions) ([]*User, int64, error) {
	q := db.WithContext(ctx).Model(&User{})
	if k := strings.TrimSpace(opts.Keyword); k != "" {
		pattern := "%" + dbutil.EscapeLike(strings.ToLower(k)) + "%"
		q = q.Where("LOWER(user_name) LIKE ? OR email LIKE ?", pattern, pattern)
	}
	switch opts.Status {
	case UserStatusActive:
		q = q.Where("disabled_at IS NULL")
	case UserStatusDisabled:
		q = q.Where("disabled_at IS NOT NULL")
	case UserStatusAdmin:
		q = q.Where("is_admin = ?", true)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, errors.Wrap(err, "count")
	}
	limit, offset := opts.LimitOffset()
	var users []*User
	return users, total, q.Order("id DESC").Limit(limit).Offset(offset).Find(&users).Error
}

// UserStats are the user counts.
type UserStats struct {
	Total        int64
	Disabled     int64
	Admins       int64
	NewLast7Days int64
}

func (db *users) Stats(ctx context.Context) (*UserStats, error) {
	var stats UserStats
	count := func(dst *int64, where string, args ...interface{}) error {
		q := db.WithContext(ctx).Model(&User{})
		if where != "" {
			q = q.Where(where, args...)
		}
		return q.Count(dst).Error
	}
	if err := count(&stats.Total, ""); err != nil {
		return nil, errors.Wrap(err, "count total")
	}
	if err := count(&stats.Disabled, "disabled_at IS NOT NULL"); err != nil {
		return nil, errors.Wrap(err, "count disabled")
	}
	if err := count(&stats.Admins, "is_admin = ?", true); err != nil {
		return nil, errors.Wrap(err, "count admins")
	}
	if err := count(&stats.NewLast7Days, "created_at > ?", dbutil.Now().Add(-7*24*time.Hour)); err != nil {
		return nil, errors.Wrap(err, "count new")
	}
	return &stats, nil
}

func (db *users) Recent(ctx context.Context, limit int) ([]*User, error) {
	var users []*User
	return users, db.WithContext(ctx).Order("id DESC").Limit(limit).Find(&users).Error
}

func (db *users) EnsureAdmin(ctx context.Context) error {
	return db.WithContext(ctx).Exec(`UPDATE users SET is_admin = TRUE
WHERE id = (SELECT MIN(id) FROM users WHERE deleted_at IS NULL AND disabled_at IS NULL)
  AND NOT EXISTS (SELECT 1 FROM users WHERE is_admin AND deleted_at IS NULL AND disabled_at IS NULL)`).Error
}

var ErrLastAdmin = errors.New("at least one enabled admin is required")

// lockAdmins locks all the admin rows, and returns ErrLastAdmin if no enabled admin would be left after target loses the admin or is disabled or deleted.
func lockAdmins(tx *gorm.DB, target int64) error {
	var admins []*User
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("is_admin = ?", true).Find(&admins).Error; err != nil {
		return errors.Wrap(err, "lock admins")
	}
	targetIsAdmin := false
	for _, u := range admins {
		if u.ID == target {
			targetIsAdmin = true
		} else if !u.Disabled() {
			return nil
		}
	}
	if targetIsAdmin {
		return ErrLastAdmin
	}
	return nil
}

func (db *users) SetAdmin(ctx context.Context, id int64, isAdmin bool) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if !isAdmin {
			if err := lockAdmins(tx, id); err != nil {
				return err
			}
		}
		return updateUser(tx, id, map[string]interface{}{"is_admin": isAdmin})
	})
}

func (db *users) SetDisabled(ctx context.Context, id int64, disabled bool) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if !disabled {
			return updateUser(tx, id, map[string]interface{}{"disabled_at": nil})
		}
		if err := lockAdmins(tx, id); err != nil {
			return err
		}
		if err := updateUser(tx, id, map[string]interface{}{"disabled_at": dbutil.Now()}); err != nil {
			return err
		}
		return tx.Where("user_id = ?", id).Delete(&UserSession{}).Error
	})
}

func updateUser(tx *gorm.DB, id int64, values map[string]interface{}) error {
	result := tx.Model(&User{}).Where("id = ?", id).Updates(values)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrUserNotFound
	}
	return nil
}

var ErrUserOwnsProjects = errors.New("user still owns projects")

func (db *users) Delete(ctx context.Context, id, transferTo int64) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockAdmins(tx, id); err != nil {
			return err
		}

		var owned []int64
		if err := tx.Model(&Project{}).Where("owner_user_id = ?", id).Pluck("id", &owned).Error; err != nil {
			return errors.Wrap(err, "list owned projects")
		}
		if len(owned) > 0 {
			if transferTo == 0 {
				return ErrUserOwnsProjects
			}
			if err := tx.Where("user_id = ? AND project_id IN ?", transferTo, owned).Delete(&ProjectMember{}).Error; err != nil {
				return errors.Wrap(err, "delete memberships of the new owner")
			}
			if err := tx.Model(&Project{}).Where("id IN ?", owned).Update("owner_user_id", transferTo).Error; err != nil {
				return errors.Wrap(err, "transfer projects")
			}
		}

		if err := tx.Where("user_id = ?", id).Delete(&ProjectMember{}).Error; err != nil {
			return errors.Wrap(err, "delete memberships")
		}
		if err := tx.Where("user_id = ?", id).Delete(&UserSession{}).Error; err != nil {
			return errors.Wrap(err, "delete sessions")
		}
		if err := tx.Where("user_id = ?", id).Delete(&Setting{}).Error; err != nil {
			return errors.Wrap(err, "delete settings")
		}
		if err := tx.Where("user_id = ?", id).Delete(&UserIdentity{}).Error; err != nil {
			return errors.Wrap(err, "delete identities")
		}
		result := tx.Delete(&User{}, id)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrUserNotFound
		}
		return nil
	})
}
