package db

import (
	"context"
	"time"

	"github.com/cockroachdb/errors"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

var _ SLRecordChangesStore = (*slRecordChanges)(nil)

// SLRecordChanges is the default instance of the SLRecordChangesStore.
var SLRecordChanges SLRecordChangesStore

// SLRecordChangesStore reads the history of the records, which is written by SLRecordsStore in the transactions writing the records.
type SLRecordChangesStore interface {
	// GetByID returns the change of the record with the given ID.
	// It returns ErrSLRecordChangeNotFound if it does not exist or belongs to another record.
	GetByID(ctx context.Context, slRecordID, id int64) (*SLRecordChange, error)
	// ListByRecord returns at most limit changes of the record whose ID is less than beforeID, the newest first; a non-positive beforeID starts from the newest.
	ListByRecord(ctx context.Context, slRecordID, beforeID int64, limit int) ([]*SLRecordChange, error)
	// ListTimeline returns at most limit changes of the record after the cursor in the timeline order, a nil cursor starts from the beginning.
	ListTimeline(ctx context.Context, slRecordID int64, cursor *TimelineCursor, asc bool, limit int) ([]*SLRecordChange, error)
}

func NewSLRecordChangesStore(db *gorm.DB) SLRecordChangesStore {
	return &slRecordChanges{db}
}

// SLRecordChangeKind is the kind of a record change.
type SLRecordChangeKind string

const (
	SLRecordChangeCreated SLRecordChangeKind = "created"
	SLRecordChangeUpdated SLRecordChangeKind = "updated"
	SLRecordChangeDeleted SLRecordChangeKind = "deleted"
)

// SLRecordChange is the creation, the deletion, or the change of a field value of a record by a user or a workflow.
type SLRecordChange struct {
	// ID is the primary key, it increases with the changes of a record.
	ID int64 `gorm:"primarykey;index:idx_sl_record_changes_record,priority:2"`
	// SLTableID is the table of the record.
	SLTableID int64 `gorm:"not null"`
	// SLRecordID is the record changed, the changes are kept after the record is deleted.
	SLRecordID int64 `gorm:"not null;index:idx_sl_record_changes_record,priority:1"`
	// Kind is created, updated or deleted.
	Kind SLRecordChangeKind `gorm:"type:varchar(16);not null"`
	// FieldUID is the field changed, only for updated.
	FieldUID string `gorm:"not null;default:''"`
	// FieldLabel is the label of the field when written, only for updated.
	FieldLabel string `gorm:"not null;default:''"`
	// FieldType is the type of the field when written, only for updated.
	FieldType SLFieldType `gorm:"type:varchar(32);not null;default:''"`
	// OldValue is the cell value before the change, NULL if empty; only for updated.
	OldValue datatypes.JSON `gorm:"type:jsonb"`
	// NewValue is the cell value after the change, NULL if empty; only for updated.
	NewValue datatypes.JSON `gorm:"type:jsonb"`
	// UserID is the user making the change, 0 if it is made by a workflow.
	UserID int64 `gorm:"not null;default:0"`
	// WorkflowID is the workflow making the change, 0 if it is made by a user.
	WorkflowID int64 `gorm:"not null;default:0"`
	// CreatedAt is the time of the write.
	CreatedAt time.Time
}

// Writer returns the user or the workflow making the change.
func (c *SLRecordChange) Writer() SLRecordWriter {
	return NewSLRecordWriter(c.UserID, c.WorkflowID)
}

// TimelineCursor returns the position of the change in the timeline.
func (c *SLRecordChange) TimelineCursor() TimelineCursor {
	return NewTimelineCursor(c.CreatedAt, TimelineRankChange, c.ID)
}

var ErrSLRecordChangeNotFound = errors.New("sl_record_change does not exist")

type slRecordChanges struct {
	// DB is the database connection the store operates on.
	*gorm.DB
}

func (db *slRecordChanges) GetByID(ctx context.Context, slRecordID, id int64) (*SLRecordChange, error) {
	var change SLRecordChange
	if err := db.WithContext(ctx).Where("id = ? AND sl_record_id = ?", id, slRecordID).First(&change).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSLRecordChangeNotFound
		}
		return nil, errors.Wrap(err, "get")
	}
	return &change, nil
}

func (db *slRecordChanges) ListByRecord(ctx context.Context, slRecordID, beforeID int64, limit int) ([]*SLRecordChange, error) {
	q := db.WithContext(ctx).Where("sl_record_id = ?", slRecordID)
	if beforeID > 0 {
		q = q.Where("id < ?", beforeID)
	}
	var changes []*SLRecordChange
	if err := q.Order("id DESC").Limit(limit).Find(&changes).Error; err != nil {
		return nil, errors.Wrap(err, "find")
	}
	return changes, nil
}

func (db *slRecordChanges) ListTimeline(ctx context.Context, slRecordID int64, cursor *TimelineCursor, asc bool, limit int) ([]*SLRecordChange, error) {
	q := db.WithContext(ctx).Where("sl_record_id = ?", slRecordID)
	if cursor != nil {
		condition, args := cursor.condition(TimelineRankChange, asc)
		q = q.Where(condition, args...)
	}
	var changes []*SLRecordChange
	if err := q.Order(timelineOrder(asc)).Limit(limit).Find(&changes).Error; err != nil {
		return nil, errors.Wrap(err, "find")
	}
	return changes, nil
}

// The ranks order the timeline items written at the same time.
const (
	TimelineRankChange  = 0
	TimelineRankComment = 1
)

// TimelineCursor is the position of the last item read in a timeline ordered by (created_at, rank, id).
type TimelineCursor struct {
	Time time.Time
	Rank int
	ID   int64
}

// NewTimelineCursor returns the position of the item of the rank written at the time.
func NewTimelineCursor(t time.Time, rank int, id int64) TimelineCursor {
	return TimelineCursor{
		Time: t,
		Rank: rank,
		ID:   id,
	}
}

// condition returns the SQL condition of the items of the rank after the cursor.
func (c *TimelineCursor) condition(rank int, asc bool) (string, []interface{}) {
	op := "<"
	if asc {
		op = ">"
	}
	// At the same time, the items of a lower rank come before the cursor in ascending order and after it in descending order.
	switch {
	case rank == c.Rank:
		return "(created_at " + op + " ? OR (created_at = ? AND id " + op + " ?))", []interface{}{c.Time, c.Time, c.ID}
	case (rank < c.Rank) != asc:
		return "created_at " + op + "= ?", []interface{}{c.Time}
	default:
		return "created_at " + op + " ?", []interface{}{c.Time}
	}
}

func timelineOrder(asc bool) string {
	if asc {
		return "created_at ASC, id ASC"
	}
	return "created_at DESC, id DESC"
}
