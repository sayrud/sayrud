package dbutil

import (
	"context"
	"database/sql"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// LockTable serializes table mutations across server instances. Call it before
// reading fields or records, inside the transaction which writes the changeset.
func LockTable(ctx context.Context, tx *gorm.DB, tableID int64) error {
	var table Model
	return tx.WithContext(ctx).Table("sl_tables").Select("id").
		Clauses(clause.Locking{Strength: "UPDATE"}).Take(&table, tableID).Error
}

type Transactor interface {
	Transaction(fc func(tx *gorm.DB) error, opts ...*sql.TxOptions) (err error)
}

func IsUniqueViolation(err error, constraint string) bool {
	errMsg := err.Error()
	return strings.Contains(errMsg, "duplicate key value violates unique constraint") && strings.Contains(errMsg, constraint)
}

// EscapeLike escapes the LIKE wildcards so the keyword matches literally.
func EscapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
