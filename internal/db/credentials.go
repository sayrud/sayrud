package db

import (
	"context"

	"github.com/cockroachdb/errors"
	"github.com/thanhpk/randstr"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/dbutil"
)

var _ CredentialsStore = (*credentials)(nil)

// Credentials is the default instance of the CredentialsStore.
var Credentials CredentialsStore

// CredentialsStore is the persistent interface for project credentials (e.g. SMTP accounts, third-party tokens); all credential types share one table.
type CredentialsStore interface {
	// List returns the credentials of the project in creation order, only of credType if it is not empty.
	List(ctx context.Context, projectID int64, credType string) ([]*Credential, error)
	// GetByUID returns the credential, it returns ErrCredentialNotFound if it does not exist.
	GetByUID(ctx context.Context, uid string) (*Credential, error)
	Create(ctx context.Context, opts CreateCredentialOptions) (*Credential, error)
	// Update overwrites the name, the plain data and the sealed secrets of the credential.
	Update(ctx context.Context, id int64, opts UpdateCredentialOptions) error
	DeleteByID(ctx context.Context, id int64) error
}

func NewCredentialsStore(db *gorm.DB) CredentialsStore {
	return &credentials{db}
}

// Credential is a project credential referenced by workflow steps, whose fields are defined by the DSL of its type.
type Credential struct {
	dbutil.Model

	// UID is the public identifier, "crd" followed by 13 random characters, which step configs reference.
	UID       string `gorm:"uniqueIndex:idx_credentials_uid, where:deleted_at IS NULL"`
	ProjectID int64  `gorm:"not null;index"`
	Type      string `gorm:"type:varchar(32);not null"`
	Name      string `gorm:"not null;default:''"`
	// Data holds the non-secret fields, which may be returned to the frontend.
	Data datatypes.JSON `gorm:"type:jsonb;not null"`
	// Sealed is the JSON of the secret fields encrypted with auth.secret_key, empty if there are no secrets.
	Sealed    string `gorm:"not null;default:''"`
	CreatedBy int64  `gorm:"not null;default:0"`
	UpdatedBy int64  `gorm:"not null;default:0"`
}

func (c *Credential) BeforeCreate(_ *gorm.DB) error {
	if c.UID == "" {
		c.UID = "crd" + randstr.String(13)
	}
	return nil
}

var ErrCredentialNotFound = errors.New("credential does not exist")

type credentials struct {
	*gorm.DB
}

func (db *credentials) List(ctx context.Context, projectID int64, credType string) ([]*Credential, error) {
	q := db.WithContext(ctx).Where("project_id = ?", projectID)
	if credType != "" {
		q = q.Where("type = ?", credType)
	}
	var list []*Credential
	if err := q.Order("id ASC").Find(&list).Error; err != nil {
		return nil, errors.Wrap(err, "find")
	}
	return list, nil
}

func (db *credentials) GetByUID(ctx context.Context, uid string) (*Credential, error) {
	var c Credential
	if err := db.WithContext(ctx).Where("uid = ?", uid).First(&c).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCredentialNotFound
		}
		return nil, errors.Wrap(err, "get")
	}
	return &c, nil
}

type CreateCredentialOptions struct {
	ProjectID int64
	Type      string
	Name      string
	Data      []byte
	Sealed    string
	UserID    int64
}

func (db *credentials) Create(ctx context.Context, opts CreateCredentialOptions) (*Credential, error) {
	c := &Credential{
		ProjectID: opts.ProjectID, Type: opts.Type, Name: opts.Name, Data: datatypes.JSON(opts.Data), Sealed: opts.Sealed,
		CreatedBy: opts.UserID, UpdatedBy: opts.UserID,
	}
	if err := db.WithContext(ctx).Create(c).Error; err != nil {
		return nil, errors.Wrap(err, "create")
	}
	return c, nil
}

type UpdateCredentialOptions struct {
	Name   string
	Data   []byte
	Sealed string
	UserID int64
}

func (db *credentials) Update(ctx context.Context, id int64, opts UpdateCredentialOptions) error {
	return db.WithContext(ctx).Model(&Credential{}).Where("id = ?", id).Updates(map[string]interface{}{
		"name": opts.Name, "data": datatypes.JSON(opts.Data), "sealed": opts.Sealed, "updated_by": opts.UserID,
	}).Error
}

func (db *credentials) DeleteByID(ctx context.Context, id int64) error {
	return db.WithContext(ctx).Where("id = ?", id).Delete(&Credential{}).Error
}
