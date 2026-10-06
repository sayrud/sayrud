package db

import (
	"context"
	"time"

	"github.com/cockroachdb/errors"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/dbutil"
)

var _ UserIdentitiesStore = (*userIdentities)(nil)

// UserIdentities is the default instance of the UserIdentitiesStore.
var UserIdentities UserIdentitiesStore

// UserIdentitiesStore is the persistent interface for the third-party accounts bound to the users.
type UserIdentitiesStore interface {
	// GetByProviderSubject returns the identity of the provider and the subject, it returns ErrUserIdentityNotFound if not found.
	GetByProviderSubject(ctx context.Context, providerID int64, subject string) (*UserIdentity, error)
	// ListByUserID returns the identities of the user in the order of ID.
	ListByUserID(ctx context.Context, userID int64) ([]*UserIdentity, error)
	// ListByUserIDs returns the identities of the users.
	ListByUserIDs(ctx context.Context, userIDs []int64) ([]*UserIdentity, error)
	// Create creates an identity, it returns ErrUserIdentityTaken if the account is bound or the user has bound the provider.
	Create(ctx context.Context, identity *UserIdentity) error
	// Touch updates the email and the last used time of the identity.
	Touch(ctx context.Context, id int64, email string) error
	// Delete deletes the identity of the user, it returns ErrUserIdentityNotFound if not found.
	Delete(ctx context.Context, userID, id int64) error
	// CountByUserID returns the number of identities of the user.
	CountByUserID(ctx context.Context, userID int64) (int64, error)
	// CountByProviderIDs returns the number of identities of each provider, the providers without any are omitted.
	CountByProviderIDs(ctx context.Context, providerIDs []int64) (map[int64]int64, error)
}

func NewUserIdentitiesStore(db *gorm.DB) UserIdentitiesStore {
	return &userIdentities{db}
}

// UserIdentity is a third-party account bound to a user.
type UserIdentity struct {
	// Model contains the primary key and the creation, update and deletion times.
	dbutil.Model

	UserID     int64 `gorm:"not null;index;uniqueIndex:idx_user_identities_user_provider, where:deleted_at IS NULL"`
	ProviderID int64 `gorm:"not null;uniqueIndex:idx_user_identities_provider_subject, where:deleted_at IS NULL;uniqueIndex:idx_user_identities_user_provider, where:deleted_at IS NULL"`
	// Subject is the unique ID of the account in the provider.
	Subject    string `gorm:"type:varchar(255);not null;uniqueIndex:idx_user_identities_provider_subject, where:deleted_at IS NULL"`
	Email      string `gorm:"type:varchar(254);not null;default:''"`
	LastUsedAt *time.Time
}

type userIdentities struct {
	*gorm.DB
}

var (
	ErrUserIdentityNotFound = errors.New("user identity does not exist")
	ErrUserIdentityTaken    = errors.New("user identity has been bound")
)

func (db *userIdentities) GetByProviderSubject(ctx context.Context, providerID int64, subject string) (*UserIdentity, error) {
	var identity UserIdentity
	if err := db.WithContext(ctx).Where("provider_id = ? AND subject = ?", providerID, subject).First(&identity).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserIdentityNotFound
		}
		return nil, err
	}
	return &identity, nil
}

func (db *userIdentities) ListByUserID(ctx context.Context, userID int64) ([]*UserIdentity, error) {
	var identities []*UserIdentity
	return identities, db.WithContext(ctx).Where("user_id = ?", userID).Order("id ASC").Find(&identities).Error
}

func (db *userIdentities) ListByUserIDs(ctx context.Context, userIDs []int64) ([]*UserIdentity, error) {
	var identities []*UserIdentity
	if len(userIDs) == 0 {
		return identities, nil
	}
	return identities, db.WithContext(ctx).Where("user_id IN ?", userIDs).Order("id ASC").Find(&identities).Error
}

func (db *userIdentities) Create(ctx context.Context, identity *UserIdentity) error {
	now := dbutil.Now()
	identity.LastUsedAt = &now
	if err := db.WithContext(ctx).Create(identity).Error; err != nil {
		if dbutil.IsUniqueViolation(err, "idx_user_identities_") {
			return ErrUserIdentityTaken
		}
		return err
	}
	return nil
}

func (db *userIdentities) Touch(ctx context.Context, id int64, email string) error {
	return db.WithContext(ctx).Model(&UserIdentity{}).Where("id = ?", id).Updates(map[string]interface{}{
		"email":        email,
		"last_used_at": dbutil.Now(),
	}).Error
}

func (db *userIdentities) Delete(ctx context.Context, userID, id int64) error {
	result := db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).Delete(&UserIdentity{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrUserIdentityNotFound
	}
	return nil
}

func (db *userIdentities) CountByUserID(ctx context.Context, userID int64) (int64, error) {
	var count int64
	return count, db.WithContext(ctx).Model(&UserIdentity{}).Where("user_id = ?", userID).Count(&count).Error
}

func (db *userIdentities) CountByProviderIDs(ctx context.Context, providerIDs []int64) (map[int64]int64, error) {
	counts := make(map[int64]int64, len(providerIDs))
	if len(providerIDs) == 0 {
		return counts, nil
	}
	var rows []struct {
		ProviderID int64
		Count      int64
	}
	if err := db.WithContext(ctx).Model(&UserIdentity{}).
		Select("provider_id, COUNT(*) AS count").
		Where("provider_id IN ?", providerIDs).
		Group("provider_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		counts[row.ProviderID] = row.Count
	}
	return counts, nil
}
