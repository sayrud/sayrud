package db

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/dbutil"
)

func TestLockTableRequiresLiveTable(t *testing.T) {
	database := newTestDB(t, &SLTable{})
	table := &SLTable{}
	require.NoError(t, database.Create(table).Error)
	lock := func() error {
		return database.Transaction(func(tx *gorm.DB) error {
			return dbutil.LockTable(context.Background(), tx, table.ID)
		})
	}
	require.NoError(t, lock())
	require.NoError(t, database.Delete(table).Error)
	require.ErrorIs(t, lock(), gorm.ErrRecordNotFound)
}
