package dto

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/wuhan005/sayrud/internal/collab"
	"github.com/wuhan005/sayrud/internal/db"
)

// ListRecordCommentsResp is a page of the root comments of a record, the newest first.
type ListRecordCommentsResp struct {
	// Comments are the root comments with their replies.
	Comments []*collab.Comment `json:"comments"`
	// NextCursor reads the older root comments, 0 if there are none.
	NextCursor int64 `json:"nextCursor"`
} // @name ListRecordCommentsResp

// RecordFieldChange is the change of a field value.
type RecordFieldChange struct {
	// FieldUID is the field changed.
	FieldUID string `json:"fieldUID"`
	// FieldLabel is the label of the field when written.
	FieldLabel string `json:"fieldLabel"`
	// FieldType is the type of the field when written.
	FieldType db.SLFieldType `json:"fieldType"`
	// OldValue is the cell value before the change, absent if empty.
	OldValue json.RawMessage `json:"oldValue,omitempty" swaggertype:"object"`
	// NewValue is the cell value after the change, absent if empty.
	NewValue json.RawMessage `json:"newValue,omitempty" swaggertype:"object"`
} // @name RecordFieldChange

// NewRecordFieldChange returns the field change of an updated change.
func NewRecordFieldChange(c *db.SLRecordChange) *RecordFieldChange {
	return &RecordFieldChange{
		FieldUID:   c.FieldUID,
		FieldLabel: c.FieldLabel,
		FieldType:  c.FieldType,
		OldValue:   json.RawMessage(c.OldValue),
		NewValue:   json.RawMessage(c.NewValue),
	}
}

// RecordChange is an entry of the history of a record: its creation, deletion, or the changes of the fields by a writer.
type RecordChange struct {
	// ID is the latest change merged into the entry.
	ID int64 `json:"id"`
	// Kind is created, updated or deleted.
	Kind db.SLRecordChangeKind `json:"kind" enums:"created,updated,deleted"`
	// Actor is the user or the workflow making the change, absent if unknown.
	Actor *db.SLRecordWriter `json:"actor,omitempty"`
	// CreatedAt is the time of the latest change merged into the entry.
	CreatedAt time.Time `json:"createdAt"`
	// Fields are the changed fields of an update, empty for the creation and the deletion.
	Fields []*RecordFieldChange `json:"fields"`
} // @name RecordChange

// NewRecordChange returns the history entry starting from the change, without the field changes.
func NewRecordChange(c *db.SLRecordChange) *RecordChange {
	return &RecordChange{
		ID:        c.ID,
		Kind:      c.Kind,
		Actor:     writerOf(c.Writer()),
		CreatedAt: c.CreatedAt,
		Fields:    []*RecordFieldChange{},
	}
}

// ToRecordChange returns a single change without merging.
func ToRecordChange(c *db.SLRecordChange) *RecordChange {
	item := NewRecordChange(c)
	if c.Kind == db.SLRecordChangeUpdated {
		item.Fields = append(item.Fields, NewRecordFieldChange(c))
	}
	return item
}

// ListRecordChangesResp is a page of the history of a record, the newest first.
type ListRecordChangesResp struct {
	// Changes are the merged history entries.
	Changes []*RecordChange `json:"changes"`
	// NextCursor reads the older history, 0 if there is none.
	NextCursor int64 `json:"nextCursor"`
} // @name ListRecordChangesResp

// The types of the timeline items.
const (
	TimelineComment = "comment"
	TimelineChange  = "change"
)

// TimelineItem is a comment or a field change in the timeline of a record.
type TimelineItem struct {
	// ID is unique in the timeline and never changes, e.g. comment:12 or change:34.
	ID string `json:"id"`
	// Type is comment or change.
	Type string `json:"type" enums:"comment,change"`
	// CreatedAt is the time the comment is posted or the change is written.
	CreatedAt time.Time `json:"createdAt"`
	// Actor is the author of the comment or the writer of the change, absent if unknown.
	Actor *db.SLRecordWriter `json:"actor,omitempty"`
	// Summary is the comment text or the change as text, truncated to 500 characters.
	Summary string `json:"summary"`
	// Truncated reports the summary or the values are shortened, the full item is read from Ref.
	Truncated bool `json:"truncated"`
	// Ref is the API path of the full comment or change, relative to /_.
	Ref string `json:"ref"`
	// Comment is the structure of a comment item.
	Comment *TimelineCommentInfo `json:"comment,omitempty"`
	// Change is the structure of a change item.
	Change *TimelineChangeDetail `json:"change,omitempty"`
} // @name RecordTimelineItem

// NewCommentTimelineItem returns the timeline item of the comment, ref reads the full comment.
func NewCommentTimelineItem(ref string, c *db.SLRecordComment, summary string, truncated bool) *TimelineItem {
	return &TimelineItem{
		ID:        TimelineComment + ":" + strconv.FormatInt(c.ID, 10),
		Type:      TimelineComment,
		CreatedAt: c.CreatedAt,
		Actor:     writerOf(c.Author()),
		Summary:   summary,
		Truncated: truncated,
		Ref:       ref,
		Comment:   NewTimelineCommentInfo(c),
	}
}

// NewChangeTimelineItem returns the timeline item of the change, ref reads the full change; withValues inlines its values.
func NewChangeTimelineItem(ref string, c *db.SLRecordChange, summary string, truncated, withValues bool) *TimelineItem {
	return &TimelineItem{
		ID:        TimelineChange + ":" + strconv.FormatInt(c.ID, 10),
		Type:      TimelineChange,
		CreatedAt: c.CreatedAt,
		Actor:     writerOf(c.Writer()),
		Summary:   summary,
		Truncated: truncated,
		Ref:       ref,
		Change:    NewTimelineChangeDetail(c, withValues),
	}
}

// TimelineCommentInfo is the structure of a comment item.
type TimelineCommentInfo struct {
	ID        int64               `json:"id"`
	RootID    int64               `json:"rootId"`
	ReplyToID int64               `json:"replyToId"`
	Kind      string              `json:"kind" enums:"comment,report,attention"`
	Mentions  []db.CommentMention `json:"mentions"`
	EditedAt  *time.Time          `json:"editedAt"`
} // @name RecordTimelineComment

// NewTimelineCommentInfo returns the structure of the comment.
func NewTimelineCommentInfo(c *db.SLRecordComment) *TimelineCommentInfo {
	mentions := []db.CommentMention(c.Mentions)
	if mentions == nil {
		mentions = []db.CommentMention{}
	}
	return &TimelineCommentInfo{
		ID:        c.ID,
		RootID:    c.RootID,
		ReplyToID: c.ReplyToID,
		Kind:      string(c.Kind),
		Mentions:  mentions,
		EditedAt:  c.EditedAt,
	}
}

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

// NewTimelineChangeDetail returns the structure of the change, withValues inlines its values.
func NewTimelineChangeDetail(c *db.SLRecordChange, withValues bool) *TimelineChangeDetail {
	detail := &TimelineChangeDetail{
		ID:         c.ID,
		Kind:       c.Kind,
		FieldUID:   c.FieldUID,
		FieldLabel: c.FieldLabel,
		FieldType:  c.FieldType,
	}
	if withValues {
		detail.OldValue = json.RawMessage(c.OldValue)
		detail.NewValue = json.RawMessage(c.NewValue)
	}
	return detail
}

// RecordTimelineResp is a page of the timeline of a record.
type RecordTimelineResp struct {
	// Items are the comments and the changes in the requested order.
	Items []*TimelineItem `json:"items"`
	// NextCursor reads the following items in the same order, empty if there are none.
	NextCursor string `json:"nextCursor"`
} // @name RecordTimelineResp
