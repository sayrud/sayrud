package db

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"time"

	"github.com/pkg/errors"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/dbutil"
)

var _ UserSessionsStore = (*userSessions)(nil)

// UserSessions is the default instance of the UserSessionsStore.
var UserSessions UserSessionsStore

// UserSessionsStore is the persistent interface for the sign-in sessions.
type UserSessionsStore interface {
	// Create creates a session of the user and returns the plaintext token, only its hash is stored.
	Create(ctx context.Context, opts CreateUserSessionOptions) (string, error)
	// GetByToken returns the unexpired session of the token.
	// It returns ErrUserSessionNotFound if the session does not exist or has expired.
	GetByToken(ctx context.Context, token string) (*UserSession, error)
	// DeleteByToken deletes the session of the token.
	DeleteByToken(ctx context.Context, token string) error
	// DeleteByUserID deletes all the sessions of the user except the one of exceptToken, which can be empty.
	DeleteByUserID(ctx context.Context, userID int64, exceptToken string) error
}

func NewUserSessionsStore(db *gorm.DB) UserSessionsStore {
	return &userSessions{db}
}

// UserSession is a sign-in session of a user.
type UserSession struct {
	ID        int64  `gorm:"primarykey"`
	UserID    int64  `gorm:"index"`
	TokenHash string `gorm:"uniqueIndex:idx_user_sessions_token_hash"`
	UserAgent string
	IP        string
	ExpiresAt time.Time `gorm:"index"`
	CreatedAt time.Time
}

type userSessions struct {
	*gorm.DB
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// CreateUserSessionOptions are the options of creating a session.
type CreateUserSessionOptions struct {
	UserID    int64
	UserAgent string
	IP        string
	TTL       time.Duration
}

func (db *userSessions) Create(ctx context.Context, opts CreateUserSessionOptions) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", errors.Wrap(err, "generate token")
	}
	token := base64.RawURLEncoding.EncodeToString(b)

	now := dbutil.Now()
	if err := db.WithContext(ctx).Where("user_id = ? AND expires_at < ?", opts.UserID, now).Delete(&UserSession{}).Error; err != nil {
		return "", errors.Wrap(err, "delete expired sessions")
	}
	if err := db.WithContext(ctx).Create(&UserSession{
		UserID:    opts.UserID,
		TokenHash: hashToken(token),
		UserAgent: truncate(opts.UserAgent, 255),
		IP:        truncate(opts.IP, 64),
		ExpiresAt: now.Add(opts.TTL),
	}).Error; err != nil {
		return "", errors.Wrap(err, "create session")
	}
	return token, nil
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

var ErrUserSessionNotFound = errors.New("user session does not exist")

func (db *userSessions) GetByToken(ctx context.Context, token string) (*UserSession, error) {
	if token == "" {
		return nil, ErrUserSessionNotFound
	}
	var session UserSession
	if err := db.WithContext(ctx).Where("token_hash = ? AND expires_at > ?", hashToken(token), dbutil.Now()).First(&session).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserSessionNotFound
		}
		return nil, err
	}
	return &session, nil
}

func (db *userSessions) DeleteByToken(ctx context.Context, token string) error {
	return db.WithContext(ctx).Where("token_hash = ?", hashToken(token)).Delete(&UserSession{}).Error
}

func (db *userSessions) DeleteByUserID(ctx context.Context, userID int64, exceptToken string) error {
	q := db.WithContext(ctx).Where("user_id = ?", userID)
	if exceptToken != "" {
		q = q.Where("token_hash <> ?", hashToken(exceptToken))
	}
	return q.Delete(&UserSession{}).Error
}
