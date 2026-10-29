package db

import (
	"github.com/cockroachdb/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// migrateProjectMembers replaces the legacy pair primary key so soft-deleted members can rejoin.
func migrateProjectMembers(tx *gorm.DB) error {
	var primaryKey string
	if err := tx.Raw(`SELECT c.conname FROM pg_constraint c
		WHERE c.conrelid = to_regclass('project_members') AND c.contype = 'p'
		AND (SELECT array_agg(a.attname::text ORDER BY a.attname) FROM pg_attribute a
			WHERE a.attrelid = c.conrelid AND a.attnum = ANY(c.conkey)) = ARRAY['project_id', 'user_id']`).Scan(&primaryKey).Error; err != nil {
		return errors.Wrap(err, "find legacy primary key")
	}
	if primaryKey == "" {
		return nil
	}

	m := tx.Migrator()
	if !m.HasColumn(&ProjectMember{}, "ID") {
		if err := m.AddColumn(&ProjectMember{}, "ID"); err != nil {
			return errors.Wrap(err, "add member IDs")
		}
	}
	if err := tx.Exec(`ALTER TABLE project_members DROP CONSTRAINT ?, ADD PRIMARY KEY (id)`, clause.Column{Name: primaryKey}).Error; err != nil {
		return errors.Wrap(err, "replace legacy primary key")
	}

	// AutoMigrate recreates this index with the deleted_at IS NULL predicate.
	const index = "idx_project_members_project_user"
	if m.HasIndex(&ProjectMember{}, index) {
		if err := m.DropIndex(&ProjectMember{}, index); err != nil {
			return errors.Wrap(err, "drop legacy member index")
		}
	}
	return nil
}
