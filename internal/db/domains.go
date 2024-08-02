// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package db

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/dbutil"
)

var _ DomainsStore = (*domains)(nil)

var Domains DomainsStore

type DomainsStore interface {
	ListByProjectID(ctx context.Context, projectID uint) ([]*Domain, error)
	Create(ctx context.Context, options CreateDomainOptions) error
	UpdateVerified(ctx context.Context, id uint, verified bool, note string) error
	GetByDomain(ctx context.Context, domain string) (*Domain, error)
	DeleteByID(ctx context.Context, id uint) error
}

func NewDomainsStore(db *gorm.DB) DomainsStore {
	return &domains{db}
}

type Domain struct {
	dbutil.Model
	ProjectID uint    `json:"-"`
	Project   Project `gorm:"foreignKey:ProjectID" json:"-"`

	Domain       string    `json:"domain" gorm:"uniqueIndex:idx_domains_domain, where:deleted_at IS NULL"`
	Verified     bool      `json:"verified"`
	VerifiedNote string    `json:"verifiedNote"`
	LastVerifyAt time.Time `json:"lastVerifyAt"`
}

type domains struct {
	*gorm.DB
}

func (db *domains) ListByProjectID(ctx context.Context, projectID uint) ([]*Domain, error) {
	var domains []*Domain
	if err := db.WithContext(ctx).Where("project_id = ?", projectID).Find(&domains).Error; err != nil {
		return nil, err
	}
	return domains, nil
}

type CreateDomainOptions struct {
	ProjectID uint
	Domain    string
}

var ErrDomainAlreadyExists = errors.New("domain already exists")

func (db *domains) Create(ctx context.Context, options CreateDomainOptions) error {
	domain := &Domain{
		ProjectID: options.ProjectID,
		Domain:    options.Domain,
	}
	if err := db.WithContext(ctx).Create(domain).Error; err != nil {
		if dbutil.IsUniqueViolation(err, "idx_domains_domain") {
			return ErrDomainAlreadyExists
		}
		return err
	}
	return nil
}

var ErrDomainNotFound = errors.New("domain does not exist")

func (db *domains) UpdateVerified(ctx context.Context, id uint, verified bool, note string) error {
	domain := &Domain{}
	if err := db.WithContext(ctx).Where("id = ?", id).First(domain).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrDomainNotFound
		}
		return errors.Wrap(err, "get")
	}

	domain.Verified = verified
	domain.VerifiedNote = note
	domain.LastVerifyAt = time.Now()

	return db.WithContext(ctx).Save(domain).Error
}

func (db *domains) GetByDomain(ctx context.Context, domain string) (*Domain, error) {
	var d Domain
	if err := db.WithContext(ctx).Where("domain = ?", domain).First(&d).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDomainNotFound
		}
		return nil, err
	}
	return &d, nil
}

func (db *domains) DeleteByID(ctx context.Context, id uint) error {
	return db.WithContext(ctx).Where("id = ?", id).Delete(&Domain{}).Error
}
