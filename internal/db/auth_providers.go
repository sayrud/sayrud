package db

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/dbutil"
)

var _ AuthProvidersStore = (*authProviders)(nil)

// AuthProviders is the default instance of the AuthProvidersStore.
var AuthProviders AuthProvidersStore

// AuthProvidersStore is the persistent interface for the third-party sign-in providers.
type AuthProvidersStore interface {
	// List returns all the providers in the order of position and ID.
	List(ctx context.Context) ([]*AuthProvider, error)
	// ListEnabled returns the enabled providers in the order of position and ID.
	ListEnabled(ctx context.Context) ([]*AuthProvider, error)
	// GetByID returns the provider with the given ID, it returns ErrAuthProviderNotFound if not found.
	GetByID(ctx context.Context, id int64) (*AuthProvider, error)
	// GetBySlug returns the provider with the given slug, it returns ErrAuthProviderNotFound if not found.
	GetBySlug(ctx context.Context, slug string) (*AuthProvider, error)
	// Create creates the provider at the end, it returns ErrAuthProviderSlugTaken if the slug has been used.
	Create(ctx context.Context, p *AuthProvider) error
	// Update updates the fields except slug, type and position, it returns ErrAuthProviderNotFound if not found.
	Update(ctx context.Context, p *AuthProvider) error
	// Delete deletes the provider along with its identities.
	Delete(ctx context.Context, id int64) error
	// SetPositions reorders the providers as ids, the unlisted ones keep their positions.
	SetPositions(ctx context.Context, ids []int64) error
	// CountEnabled returns the number of enabled providers.
	CountEnabled(ctx context.Context) (int64, error)
}

func NewAuthProvidersStore(db *gorm.DB) AuthProvidersStore {
	return &authProviders{db}
}

// AuthProviderType is the protocol of a provider.
type AuthProviderType string

const (
	AuthProviderOAuth2 AuthProviderType = "oauth2"
	AuthProviderOIDC   AuthProviderType = "oidc"
	AuthProviderSAML   AuthProviderType = "saml"
	AuthProviderLDAP   AuthProviderType = "ldap"
)

// Valid reports whether the type is supported.
func (t AuthProviderType) Valid() bool {
	switch t {
	case AuthProviderOAuth2, AuthProviderOIDC, AuthProviderSAML, AuthProviderLDAP:
		return true
	}
	return false
}

// Redirect reports whether the type signs in by redirecting to the identity provider, LDAP uses a form instead.
func (t AuthProviderType) Redirect() bool {
	return t == AuthProviderOAuth2 || t == AuthProviderOIDC || t == AuthProviderSAML
}

// AuthProvider is a third-party sign-in provider.
type AuthProvider struct {
	ID int64 `gorm:"primarykey"`
	// Slug is part of the callback URL and can not be changed after creating.
	Slug     string           `gorm:"type:varchar(32);not null;uniqueIndex:idx_auth_providers_slug"`
	Name     string           `gorm:"type:varchar(32);not null"`
	Icon     string           `gorm:"type:varchar(32);not null;default:''"`
	Type     AuthProviderType `gorm:"type:varchar(16);not null"`
	Enabled  bool             `gorm:"not null;default:false"`
	Position int              `gorm:"not null;default:0"`
	// Config is the JSON of sso.Config without the secrets.
	Config datatypes.JSON `gorm:"type:jsonb;not null"`
	// Secrets is the encrypted JSON of sso.Secrets, empty if there is no secret.
	Secrets             string                      `gorm:"type:text;not null;default:''"`
	AutoCreateUser      bool                        `gorm:"not null;default:false"`
	LinkByEmail         bool                        `gorm:"not null;default:false"`
	AllowedEmailDomains datatypes.JSONSlice[string] `gorm:"type:jsonb;not null;default:'[]'"`
	AllowedGroups       datatypes.JSONSlice[string] `gorm:"type:jsonb;not null;default:'[]'"`
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type authProviders struct {
	*gorm.DB
}

var (
	ErrAuthProviderNotFound  = errors.New("auth provider does not exist")
	ErrAuthProviderSlugTaken = errors.New("auth provider slug has been used")
)

func (db *authProviders) List(ctx context.Context) ([]*AuthProvider, error) {
	var providers []*AuthProvider
	return providers, db.WithContext(ctx).Order("position ASC, id ASC").Find(&providers).Error
}

func (db *authProviders) ListEnabled(ctx context.Context) ([]*AuthProvider, error) {
	var providers []*AuthProvider
	return providers, db.WithContext(ctx).Where("enabled = ?", true).Order("position ASC, id ASC").Find(&providers).Error
}

func (db *authProviders) GetByID(ctx context.Context, id int64) (*AuthProvider, error) {
	return db.getBy(ctx, "id = ?", id)
}

func (db *authProviders) GetBySlug(ctx context.Context, slug string) (*AuthProvider, error) {
	return db.getBy(ctx, "slug = ?", slug)
}

func (db *authProviders) getBy(ctx context.Context, where string, args ...interface{}) (*AuthProvider, error) {
	var p AuthProvider
	if err := db.WithContext(ctx).Where(where, args...).First(&p).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAuthProviderNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (db *authProviders) Create(ctx context.Context, p *AuthProvider) error {
	var maxPosition *int
	if err := db.WithContext(ctx).Model(&AuthProvider{}).Select("MAX(position)").Scan(&maxPosition).Error; err != nil {
		return errors.Wrap(err, "get max position")
	}
	p.Position = 0
	if maxPosition != nil {
		p.Position = *maxPosition + 1
	}
	if err := db.WithContext(ctx).Create(p).Error; err != nil {
		if dbutil.IsUniqueViolation(err, "idx_auth_providers_slug") {
			return ErrAuthProviderSlugTaken
		}
		return err
	}
	return nil
}

func (db *authProviders) Update(ctx context.Context, p *AuthProvider) error {
	result := db.WithContext(ctx).Model(&AuthProvider{}).Where("id = ?", p.ID).Updates(map[string]interface{}{
		"name":                  p.Name,
		"icon":                  p.Icon,
		"enabled":               p.Enabled,
		"config":                p.Config,
		"secrets":               p.Secrets,
		"auto_create_user":      p.AutoCreateUser,
		"link_by_email":         p.LinkByEmail,
		"allowed_email_domains": p.AllowedEmailDomains,
		"allowed_groups":        p.AllowedGroups,
		"updated_at":            dbutil.Now(),
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrAuthProviderNotFound
	}
	return nil
}

func (db *authProviders) Delete(ctx context.Context, id int64) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("provider_id = ?", id).Delete(&UserIdentity{}).Error; err != nil {
			return errors.Wrap(err, "delete identities")
		}
		result := tx.Delete(&AuthProvider{}, id)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrAuthProviderNotFound
		}
		return nil
	})
}

func (db *authProviders) SetPositions(ctx context.Context, ids []int64) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i, id := range ids {
			if err := tx.Model(&AuthProvider{}).Where("id = ?", id).UpdateColumn("position", i).Error; err != nil {
				return errors.Wrap(err, "update position")
			}
		}
		return nil
	})
}

func (db *authProviders) CountEnabled(ctx context.Context) (int64, error) {
	var count int64
	return count, db.WithContext(ctx).Model(&AuthProvider{}).Where("enabled = ?", true).Count(&count).Error
}
