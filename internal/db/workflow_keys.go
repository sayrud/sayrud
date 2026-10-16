package db

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/thanhpk/randstr"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/dbutil"
)

var _ WorkflowKeysStore = (*workflowKeys)(nil)

// WorkflowKeys is the default instance of the WorkflowKeysStore.
var WorkflowKeys WorkflowKeysStore

// WorkflowKeysStore is the persistent interface for the credentials of calling API workflows.
type WorkflowKeysStore interface {
	// List returns the keys of the workflow in creation order.
	List(ctx context.Context, workflowID int64) ([]*WorkflowKey, error)
	// Create generates a key for the workflow and returns its plaintext, which cannot be retrieved afterwards.
	Create(ctx context.Context, workflowID int64, name string, userID int64) (*WorkflowKey, string, error)
	// Delete deletes the key of the workflow.
	Delete(ctx context.Context, workflowID, keyID int64) error
	// Verify reports whether the plaintext is a key of the workflow, and refreshes its last used time.
	Verify(ctx context.Context, workflowID int64, plaintext string) (bool, error)
}

func NewWorkflowKeysStore(db *gorm.DB) WorkflowKeysStore {
	return &workflowKeys{db}
}

// WorkflowKey is a credential for calling an API workflow, only its hash is stored.
type WorkflowKey struct {
	ID         int64  `gorm:"primarykey"`
	WorkflowID int64  `gorm:"not null;index"`
	Name       string `gorm:"not null;default:''"`
	// Prefix is the leading part used to tell the keys apart.
	Prefix string `gorm:"type:varchar(16);not null"`
	// Hash is the hex-encoded SHA-256 of the key.
	Hash       string `gorm:"type:varchar(64);not null;uniqueIndex:idx_workflow_keys_hash"`
	LastUsedAt *time.Time
	CreatedBy  int64 `gorm:"not null;default:0"`
	CreatedAt  time.Time
}

type workflowKeys struct {
	*gorm.DB
}

func hashWorkflowKey(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

func (db *workflowKeys) List(ctx context.Context, workflowID int64) ([]*WorkflowKey, error) {
	var list []*WorkflowKey
	if err := db.WithContext(ctx).Where("workflow_id = ?", workflowID).Order("id ASC").Find(&list).Error; err != nil {
		return nil, errors.Wrap(err, "find")
	}
	return list, nil
}

func (db *workflowKeys) Create(ctx context.Context, workflowID int64, name string, userID int64) (*WorkflowKey, string, error) {
	plaintext := "sk_" + randstr.String(40)
	key := &WorkflowKey{
		WorkflowID: workflowID,
		Name:       name,
		Prefix:     plaintext[:10],
		Hash:       hashWorkflowKey(plaintext),
		CreatedBy:  userID,
	}
	if err := db.WithContext(ctx).Create(key).Error; err != nil {
		return nil, "", errors.Wrap(err, "create")
	}
	return key, plaintext, nil
}

func (db *workflowKeys) Delete(ctx context.Context, workflowID, keyID int64) error {
	return db.WithContext(ctx).Where("id = ? AND workflow_id = ?", keyID, workflowID).Delete(&WorkflowKey{}).Error
}

func (db *workflowKeys) Verify(ctx context.Context, workflowID int64, plaintext string) (bool, error) {
	if plaintext == "" {
		return false, nil
	}
	hash := hashWorkflowKey(plaintext)
	var key WorkflowKey
	if err := db.WithContext(ctx).Where("workflow_id = ? AND hash = ?", workflowID, hash).First(&key).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, errors.Wrap(err, "get")
	}
	if subtle.ConstantTimeCompare([]byte(key.Hash), []byte(hash)) != 1 {
		return false, nil
	}

	// Refresh at most once a minute to avoid writing to the database on every frequent call.
	now := dbutil.Now()
	if key.LastUsedAt == nil || now.Sub(*key.LastUsedAt) > time.Minute {
		_ = db.WithContext(ctx).Model(&WorkflowKey{}).Where("id = ?", key.ID).Update("last_used_at", now).Error
	}
	return true, nil
}
