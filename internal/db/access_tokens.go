package db

import (
	"context"
	"crypto/subtle"
	"slices"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/thanhpk/randstr"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/dbutil"
)

// AccessTokenScope is a permission granted to an access token.
type AccessTokenScope string

const (
	// AccessTokenScopeRead allows the GET and HEAD requests, and the POST requests that only read.
	AccessTokenScopeRead AccessTokenScope = "read"
	// AccessTokenScopeWrite allows the other requests.
	AccessTokenScopeWrite AccessTokenScope = "write"
	// AccessTokenScopeEvents is reserved for subscribing to the event stream.
	AccessTokenScopeEvents AccessTokenScope = "events"
)

// AccessTokenScopes are all the scopes in the canonical order.
var AccessTokenScopes = []AccessTokenScope{AccessTokenScopeRead, AccessTokenScopeWrite, AccessTokenScopeEvents}

// NormalizeAccessTokenScopes returns the scopes deduplicated in the canonical order, it reports false if any is unknown or there is none.
func NormalizeAccessTokenScopes(scopes []string) ([]AccessTokenScope, bool) {
	for _, s := range scopes {
		if !slices.Contains(AccessTokenScopes, AccessTokenScope(s)) {
			return nil, false
		}
	}
	normalized := make([]AccessTokenScope, 0, len(AccessTokenScopes))
	for _, s := range AccessTokenScopes {
		if slices.Contains(scopes, string(s)) {
			normalized = append(normalized, s)
		}
	}
	return normalized, len(normalized) > 0
}

// accessTokenPrefix marks the plaintext as a Sayrud access token, so leaked tokens are easy to recognize and scan for.
const accessTokenPrefix = "sat_"

// accessTokenTouchInterval bounds how often the last use of a token is written.
const accessTokenTouchInterval = time.Minute

var _ AccessTokensStore = (*accessTokens)(nil)

// AccessTokens is the default instance of the AccessTokensStore.
var AccessTokens AccessTokensStore

// AccessTokensStore is the persistent interface for the tokens calling the API on behalf of users.
type AccessTokensStore interface {
	// ListByUserID returns the tokens of the user, the newest first.
	ListByUserID(ctx context.Context, userID int64) ([]*AccessToken, error)
	// Create generates a token and returns its plaintext, which cannot be retrieved afterwards.
	Create(ctx context.Context, opts CreateAccessTokenOptions) (*AccessToken, string, error)
	// Verify returns the token of the plaintext.
	// It returns ErrAccessTokenNotFound if no token matches, and ErrAccessTokenExpired if it has expired.
	Verify(ctx context.Context, plaintext string) (*AccessToken, error)
	// Touch records the last use and the IP, at most once a minute per token across the server instances.
	Touch(ctx context.Context, token *AccessToken, ip string) error
	// Delete deletes the token of the user.
	// It returns ErrAccessTokenNotFound if the user has no such token.
	Delete(ctx context.Context, userID, id int64) error
	// LatestByUserIDs returns the newest token of each user keyed by user ID, the users without tokens are omitted.
	LatestByUserIDs(ctx context.Context, userIDs []int64) (map[int64]*AccessToken, error)
	// UpdateAccess changes the scopes, the projects and the expiration of the token, the secret stays the same.
	// It returns ErrAccessTokenNotFound if there is no such token.
	UpdateAccess(ctx context.Context, id int64, opts AccessTokenAccess) error
	// Rotate replaces the secret of the token, forgets its last use and dates it now, the old secret stops working at once.
	// It returns ErrAccessTokenNotFound if there is no such token.
	Rotate(ctx context.Context, id int64) (*AccessToken, string, error)
	// DeleteOthers deletes the tokens of the user except keepID.
	DeleteOthers(ctx context.Context, userID, keepID int64) error
}

func NewAccessTokensStore(db *gorm.DB) AccessTokensStore {
	return &accessTokens{db}
}

// AccessToken is a personal access token of a user or a bot, only its hash is stored.
type AccessToken struct {
	ID     int64  `gorm:"primarykey"`
	UserID int64  `gorm:"not null;index"`
	Name   string `gorm:"type:varchar(64);not null;default:''"`
	// Prefix is the leading part used to tell the tokens apart.
	Prefix string `gorm:"type:varchar(16);not null"`
	// Hash is the hex-encoded SHA-256 of the token.
	Hash   string                                `gorm:"type:varchar(64);not null;uniqueIndex:idx_access_tokens_hash"`
	Scopes datatypes.JSONSlice[AccessTokenScope] `gorm:"type:jsonb;not null"`
	// ProjectIDs restricts the token to these projects, empty to follow the memberships of the user.
	ProjectIDs datatypes.JSONSlice[int64] `gorm:"type:jsonb;not null"`
	// ExpiresAt is nil if the token never expires.
	ExpiresAt  *time.Time
	LastUsedAt *time.Time
	LastUsedIP string `gorm:"type:varchar(64);not null;default:''"`
	// CreatedBy is the user who created the token, an admin for the tokens of bots.
	CreatedBy int64 `gorm:"not null;default:0"`
	CreatedAt time.Time
}

// HasScope reports whether the token is granted the scope.
func (t *AccessToken) HasScope(scope AccessTokenScope) bool {
	return slices.Contains(t.Scopes, scope)
}

// AllowsProject reports whether the token can access the project, the membership of the user is checked separately.
func (t *AccessToken) AllowsProject(projectID int64) bool {
	return len(t.ProjectIDs) == 0 || slices.Contains(t.ProjectIDs, projectID)
}

// Expired reports whether the token has expired at now.
func (t *AccessToken) Expired(now time.Time) bool {
	return t.ExpiresAt != nil && !now.Before(*t.ExpiresAt)
}

type accessTokens struct {
	*gorm.DB
}

var (
	ErrAccessTokenNotFound = errors.New("access token does not exist")
	ErrAccessTokenExpired  = errors.New("access token has expired")
)

func (db *accessTokens) ListByUserID(ctx context.Context, userID int64) ([]*AccessToken, error) {
	var tokens []*AccessToken
	if err := db.WithContext(ctx).Where("user_id = ?", userID).Order("id DESC").Find(&tokens).Error; err != nil {
		return nil, errors.Wrap(err, "find")
	}
	return tokens, nil
}

// CreateAccessTokenOptions are the options of creating an access token.
type CreateAccessTokenOptions struct {
	UserID     int64
	Name       string
	Scopes     []AccessTokenScope
	ProjectIDs []int64
	ExpiresAt  *time.Time
	CreatedBy  int64
}

func newAccessTokenSecret() string {
	return accessTokenPrefix + randstr.String(40)
}

func (db *accessTokens) Create(ctx context.Context, opts CreateAccessTokenOptions) (*AccessToken, string, error) {
	plaintext := newAccessTokenSecret()
	projectIDs := opts.ProjectIDs
	if projectIDs == nil {
		projectIDs = []int64{}
	}
	token := &AccessToken{
		UserID:     opts.UserID,
		Name:       opts.Name,
		Prefix:     plaintext[:12],
		Hash:       hashToken(plaintext),
		Scopes:     opts.Scopes,
		ProjectIDs: projectIDs,
		ExpiresAt:  opts.ExpiresAt,
		CreatedBy:  opts.CreatedBy,
	}
	if err := db.WithContext(ctx).Create(token).Error; err != nil {
		return nil, "", errors.Wrap(err, "create")
	}
	return token, plaintext, nil
}

func (db *accessTokens) Verify(ctx context.Context, plaintext string) (*AccessToken, error) {
	if !strings.HasPrefix(plaintext, accessTokenPrefix) {
		return nil, ErrAccessTokenNotFound
	}
	hash := hashToken(plaintext)
	var token AccessToken
	if err := db.WithContext(ctx).Where("hash = ?", hash).First(&token).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAccessTokenNotFound
		}
		return nil, errors.Wrap(err, "get")
	}
	if subtle.ConstantTimeCompare([]byte(token.Hash), []byte(hash)) != 1 {
		return nil, ErrAccessTokenNotFound
	}
	if token.Expired(dbutil.Now()) {
		return nil, ErrAccessTokenExpired
	}
	return &token, nil
}

func (db *accessTokens) Touch(ctx context.Context, token *AccessToken, ip string) error {
	now := dbutil.Now()
	if token.LastUsedAt != nil && now.Sub(*token.LastUsedAt) < accessTokenTouchInterval {
		return nil
	}
	// The condition keeps the concurrent requests from writing more than once in an interval.
	return db.WithContext(ctx).Model(&AccessToken{}).
		Where("id = ? AND (last_used_at IS NULL OR last_used_at < ?)", token.ID, now.Add(-accessTokenTouchInterval)).
		Updates(map[string]interface{}{"last_used_at": now, "last_used_ip": truncate(ip, 64)}).Error
}

func (db *accessTokens) Delete(ctx context.Context, userID, id int64) error {
	result := db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).Delete(&AccessToken{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrAccessTokenNotFound
	}
	return nil
}

func (db *accessTokens) LatestByUserIDs(ctx context.Context, userIDs []int64) (map[int64]*AccessToken, error) {
	latest := make(map[int64]*AccessToken, len(userIDs))
	if len(userIDs) == 0 {
		return latest, nil
	}
	var tokens []*AccessToken
	if err := db.WithContext(ctx).Where("user_id IN ?", userIDs).Order("id DESC").Find(&tokens).Error; err != nil {
		return nil, errors.Wrap(err, "find")
	}
	for _, t := range tokens {
		if _, ok := latest[t.UserID]; !ok {
			latest[t.UserID] = t
		}
	}
	return latest, nil
}

// AccessTokenAccess is what an access token may do.
type AccessTokenAccess struct {
	Scopes     []AccessTokenScope
	ProjectIDs []int64
	ExpiresAt  *time.Time
}

func (db *accessTokens) UpdateAccess(ctx context.Context, id int64, opts AccessTokenAccess) error {
	projectIDs := opts.ProjectIDs
	if projectIDs == nil {
		projectIDs = []int64{}
	}
	result := db.WithContext(ctx).Model(&AccessToken{}).Where("id = ?", id).Updates(map[string]interface{}{
		"scopes":      datatypes.JSONSlice[AccessTokenScope](opts.Scopes),
		"project_ids": datatypes.JSONSlice[int64](projectIDs),
		"expires_at":  opts.ExpiresAt,
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrAccessTokenNotFound
	}
	return nil
}

func (db *accessTokens) Rotate(ctx context.Context, id int64) (*AccessToken, string, error) {
	plaintext := newAccessTokenSecret()
	result := db.WithContext(ctx).Model(&AccessToken{}).Where("id = ?", id).Updates(map[string]interface{}{
		"prefix":       plaintext[:12],
		"hash":         hashToken(plaintext),
		"last_used_at": nil,
		"last_used_ip": "",
		"created_at":   dbutil.Now(),
	})
	if result.Error != nil {
		return nil, "", errors.Wrap(result.Error, "update")
	}
	if result.RowsAffected == 0 {
		return nil, "", ErrAccessTokenNotFound
	}
	var token AccessToken
	if err := db.WithContext(ctx).First(&token, id).Error; err != nil {
		return nil, "", errors.Wrap(err, "reload")
	}
	return &token, plaintext, nil
}

func (db *accessTokens) DeleteOthers(ctx context.Context, userID, keepID int64) error {
	return db.WithContext(ctx).Where("user_id = ? AND id <> ?", userID, keepID).Delete(&AccessToken{}).Error
}
