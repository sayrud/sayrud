package dbutil

import (
	"context"
	"slices"

	"github.com/cockroachdb/errors"
	"gorm.io/gorm"
)

// MovePositions moves the item with the given ID to the zero-based index of the items, and renumbers the positions of all items from 0.
// The index is clamped into the valid range.
func MovePositions[T any](ctx context.Context, db *gorm.DB, model interface{}, items []T, idOf func(T) int64, id int64, index int) error {
	ids := make([]int64, 0, len(items))
	for _, item := range items {
		if idOf(item) != id {
			ids = append(ids, idOf(item))
		}
	}
	if len(ids) == len(items) {
		return errors.Newf("item %d not found", id)
	}

	index = min(max(index, 0), len(ids))
	ids = slices.Insert(ids, index, id)
	for position, id := range ids {
		if err := db.WithContext(ctx).Model(model).Where("id = ?", id).Update("position", position).Error; err != nil {
			return errors.Wrap(err, "update position")
		}
	}
	return nil
}
