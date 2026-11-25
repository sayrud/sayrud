package dto

import (
	"encoding/json"
	"time"

	"github.com/wuhan005/sayrud/internal/collab"
	"github.com/wuhan005/sayrud/internal/db"
)

// ListRecordCommentsResp is a page of the root comments of a record, the newest first.
type ListRecordCommentsResp struct {
	Comments []*collab.Comment `json:"comments"`
	// NextCursor reads the older root comments, 0 if there are none.
	NextCursor int64 `json:"nextCursor"`
} // @name ListRecordCommentsResp

// RecordFieldChange is the change of a field value, the label and the type are the ones when written.
type RecordFieldChange struct {
	FieldUID   string         `json:"fieldUID"`
	FieldLabel string         `json:"fieldLabel"`
	FieldType  db.SLFieldType `json:"fieldType"`
	// OldValue and NewValue are the cell values, absent if empty.
	OldValue json.RawMessage `json:"oldValue,omitempty" swaggertype:"object"`
	NewValue json.RawMessage `json:"newValue,omitempty" swaggertype:"object"`
} // @name RecordFieldChange

// RecordChange is an entry of the history of a record: its creation, deletion, or the changes of the fields by a writer.
type RecordChange struct {
	// ID is the latest change merged into the entry.
	ID   int64                 `json:"id"`
	Kind db.SLRecordChangeKind `json:"kind" enums:"created,updated,deleted"`
	// Actor is the user or the workflow making the change, absent if unknown.
	Actor     *db.SLRecordWriter `json:"actor,omitempty"`
	CreatedAt time.Time          `json:"createdAt"`
	// Fields are the changed fields of an update, empty for the creation and the deletion.
	Fields []*RecordFieldChange `json:"fields"`
} // @name RecordChange

// ListRecordChangesResp is a page of the history of a record, the newest first.
type ListRecordChangesResp struct {
	Changes []*RecordChange `json:"changes"`
	// NextCursor reads the older history, 0 if there is none.
	NextCursor int64 `json:"nextCursor"`
} // @name ListRecordChangesResp

// ToRecordChange returns a single change without merging.
func ToRecordChange(c *db.SLRecordChange) *RecordChange {
	item := &RecordChange{ID: c.ID, Kind: c.Kind, Actor: writerOf(c.Writer()), CreatedAt: c.CreatedAt, Fields: []*RecordFieldChange{}}
	if c.Kind == db.SLRecordChangeUpdated {
		item.Fields = append(item.Fields, &RecordFieldChange{
			FieldUID: c.FieldUID, FieldLabel: c.FieldLabel, FieldType: c.FieldType, OldValue: json.RawMessage(c.OldValue), NewValue: json.RawMessage(c.NewValue),
		})
	}
	return item
}

// The types of the timeline items.
const (
	TimelineComment = "comment"
	TimelineChange  = "change"
)

// TimelineItem is a comment or a field change in the timeline of a record.
type TimelineItem struct {
	// ID is unique in the timeline and never changes, e.g. comment:12 or change:34.
	ID        string             `json:"id"`
	Type      string             `json:"type" enums:"comment,change"`
	CreatedAt time.Time          `json:"createdAt"`
	Actor     *db.SLRecordWriter `json:"actor,omitempty"`
	// Summary is the comment text or the change as text, truncated to 500 characters.
	Summary string `json:"summary"`
	// Truncated reports the summary or the values are shortened, the full item is read from Ref.
	Truncated bool `json:"truncated"`
	// Ref is the API path of the full comment or change, relative to /_.
	Ref     string                `json:"ref"`
	Comment *TimelineCommentInfo  `json:"comment,omitempty"`
	Change  *TimelineChangeDetail `json:"change,omitempty"`
} // @name RecordTimelineItem

// TimelineCommentInfo is the structure of a comment item.
type TimelineCommentInfo struct {
	ID        int64               `json:"id"`
	RootID    int64               `json:"rootId"`
	ReplyToID int64               `json:"replyToId"`
	Kind      string              `json:"kind" enums:"comment,report,attention"`
	Mentions  []db.CommentMention `json:"mentions"`
	EditedAt  *time.Time          `json:"editedAt"`
} // @name RecordTimelineComment

// TimelineChangeDetail is the structure of a change item, the values are absent when they are empty or truncated.
type TimelineChangeDetail struct {
	ID         int64                 `json:"id"`
	Kind       db.SLRecordChangeKind `json:"kind" enums:"created,updated,deleted"`
	FieldUID   string                `json:"fieldUID,omitempty"`
	FieldLabel string                `json:"fieldLabel,omitempty"`
	FieldType  db.SLFieldType        `json:"fieldType,omitempty"`
	OldValue   json.RawMessage       `json:"oldValue,omitempty" swaggertype:"object"`
	NewValue   json.RawMessage       `json:"newValue,omitempty" swaggertype:"object"`
} // @name RecordTimelineChange

// RecordTimelineResp is a page of the timeline of a record.
type RecordTimelineResp struct {
	Items []*TimelineItem `json:"items"`
	// NextCursor reads the following items in the same order, empty if there are none.
	NextCursor string `json:"nextCursor"`
} // @name RecordTimelineResp
