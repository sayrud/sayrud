package db

import (
	"context"
	"time"

	"github.com/cockroachdb/errors"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wuhan005/sayrud/internal/dbutil"
)

var _ SLRecordCommentsStore = (*slRecordComments)(nil)

// SLRecordComments is the default instance of the SLRecordCommentsStore.
var SLRecordComments SLRecordCommentsStore

// SLRecordCommentsStore stores the comments of the records, the comments are deleted with their records.
type SLRecordCommentsStore interface {
	// Create creates the comment and increases the comment count of the record, it must run in a transaction.
	// It returns ErrSLRecordNotFound if the record has been deleted.
	Create(ctx context.Context, opts CreateSLRecordCommentOptions) (*SLRecordComment, error)
	// GetByID returns the comment with the given ID, including the deleted ones.
	// It returns ErrSLRecordCommentNotFound if it does not exist.
	GetByID(ctx context.Context, id int64) (*SLRecordComment, error)
	// ListRoots returns at most limit root comments of the record whose ID is less than beforeID, the newest first;
	// a non-positive beforeID starts from the newest, and the deleted roots are only returned while they have live replies.
	ListRoots(ctx context.Context, slRecordID, beforeID int64, limit int) ([]*SLRecordComment, error)
	// ListReplies returns the live replies of the root comments in creation order.
	ListReplies(ctx context.Context, rootIDs []int64) ([]*SLRecordComment, error)
	// HasReplies reports whether the root comment has live replies.
	HasReplies(ctx context.Context, rootID int64) (bool, error)
	// Update replaces the body and the mentions of the live comment and records the edit time.
	// It returns ErrSLRecordCommentNotFound if the comment has been deleted.
	Update(ctx context.Context, id int64, body string, mentions []CommentMention) error
	// Delete clears the content of the live comment and decreases the comment count of the record, it must run in a transaction.
	Delete(ctx context.Context, comment *SLRecordComment) error
	// ListTimeline returns at most limit live comments of the record after the cursor in the timeline order, a nil cursor starts from the beginning.
	ListTimeline(ctx context.Context, slRecordID int64, cursor *TimelineCursor, asc bool, limit int) ([]*SLRecordComment, error)
	// LatestByTable returns the latest live comment of the live records of the table.
	// It returns ErrSLRecordCommentNotFound if there is none.
	LatestByTable(ctx context.Context, slTableID int64) (*SLRecordComment, error)
}

func NewSLRecordCommentsStore(db *gorm.DB) SLRecordCommentsStore {
	return &slRecordComments{db}
}

// SLRecordCommentKind is the kind of a comment.
type SLRecordCommentKind string

const (
	SLRecordCommentKindComment SLRecordCommentKind = "comment"
	// SLRecordCommentKindReport is reserved for the progress reports of the agents (W7).
	SLRecordCommentKindReport SLRecordCommentKind = "report"
	// SLRecordCommentKindAttention is reserved for the structured questions of the agents (W9).
	SLRecordCommentKindAttention SLRecordCommentKind = "attention"
)

// CommentMention is a user mentioned by a comment as `<@userID>`, IsBot is decided when written so a mentioned bot is addressed without looking it up.
type CommentMention struct {
	UserID int64 `json:"userId"`
	IsBot  bool  `json:"isBot,omitempty"`
} // @name CommentMention

// NewCommentMention returns the mention of the user.
func NewCommentMention(user *User) CommentMention {
	return CommentMention{
		UserID: user.ID,
		IsBot:  user.IsBot(),
	}
}

// SLRecordComment is a comment on a record written by a user or a workflow, with one level of replies.
type SLRecordComment struct {
	// ID is the primary key.
	ID int64 `gorm:"primarykey"`
	// SLTableID is the table of the record.
	SLTableID int64 `gorm:"not null;index"`
	// SLRecordID is the record commented on, the comments are deleted with it.
	SLRecordID int64 `gorm:"not null;index"`
	// AuthorID is the user writing the comment, 0 if it is written by a workflow.
	AuthorID int64 `gorm:"not null;default:0"`
	// AuthorWorkflowID is the workflow writing the comment, 0 if it is written by a user.
	AuthorWorkflowID int64 `gorm:"not null;default:0"`
	// Body is the Markdown text, cleared when deleted.
	Body string `gorm:"type:text;not null;default:''"`
	// RootID is the root comment of a reply, 0 for the roots.
	RootID int64 `gorm:"not null;default:0;index"`
	// ReplyToID is the comment replied to, the root or another reply; 0 for the roots.
	ReplyToID int64 `gorm:"not null;default:0"`
	// Kind is comment, or the reserved report and attention.
	Kind SLRecordCommentKind `gorm:"type:varchar(16);not null;default:'comment'"`
	// Payload is the structured content of the reserved kinds, NULL for the plain comments.
	Payload datatypes.JSON `gorm:"type:jsonb"`
	// Mentions are the users mentioned by the body, cleared when deleted.
	Mentions datatypes.JSONSlice[CommentMention] `gorm:"type:jsonb;not null;default:'[]'"`
	// EditedAt is the time of the last edit, nil if never edited.
	EditedAt *time.Time
	// DeletedAt keeps the deleted root as a placeholder of its replies, it is not a gorm soft deletion.
	DeletedAt *time.Time
	// CreatedAt is the time the comment is posted.
	CreatedAt time.Time
}

// Author returns the user or the workflow writing the comment.
func (c *SLRecordComment) Author() SLRecordWriter {
	return NewSLRecordWriter(c.AuthorID, c.AuthorWorkflowID)
}

// TimelineCursor returns the position of the comment in the timeline.
func (c *SLRecordComment) TimelineCursor() TimelineCursor {
	return NewTimelineCursor(c.CreatedAt, TimelineRankComment, c.ID)
}

// CreateSLRecordCommentOptions are the options of creating a comment.
type CreateSLRecordCommentOptions struct {
	SLTableID  int64
	SLRecordID int64
	Author     SLRecordWriter
	Body       string
	RootID     int64
	ReplyToID  int64
	Kind       SLRecordCommentKind
	Mentions   []CommentMention
}

var ErrSLRecordCommentNotFound = errors.New("sl_record_comment does not exist")

type slRecordComments struct {
	// DB is the database connection the store operates on.
	*gorm.DB
}

func (db *slRecordComments) Create(ctx context.Context, opts CreateSLRecordCommentOptions) (*SLRecordComment, error) {
	// Locking the live record row serializes with its deletion, which deletes the comments after marking the record deleted.
	result := db.WithContext(ctx).Model(&SLRecord{}).Where("id = ?", opts.SLRecordID).
		UpdateColumn("comment_count", gorm.Expr("comment_count + 1"))
	if result.Error != nil {
		return nil, errors.Wrap(result.Error, "increase comment count")
	}
	if result.RowsAffected == 0 {
		return nil, ErrSLRecordNotFound
	}

	kind := opts.Kind
	if kind == "" {
		kind = SLRecordCommentKindComment
	}
	mentions := opts.Mentions
	if mentions == nil {
		mentions = []CommentMention{}
	}
	comment := &SLRecordComment{
		SLTableID:        opts.SLTableID,
		SLRecordID:       opts.SLRecordID,
		AuthorID:         opts.Author.UserID,
		AuthorWorkflowID: opts.Author.WorkflowID,
		Body:             opts.Body,
		RootID:           opts.RootID,
		ReplyToID:        opts.ReplyToID,
		Kind:             kind,
		Mentions:         mentions,
	}
	if err := db.WithContext(ctx).Create(comment).Error; err != nil {
		return nil, errors.Wrap(err, "create")
	}
	return comment, nil
}

func (db *slRecordComments) GetByID(ctx context.Context, id int64) (*SLRecordComment, error) {
	var comment SLRecordComment
	if err := db.WithContext(ctx).Where("id = ?", id).First(&comment).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSLRecordCommentNotFound
		}
		return nil, errors.Wrap(err, "get")
	}
	return &comment, nil
}

func (db *slRecordComments) ListRoots(ctx context.Context, slRecordID, beforeID int64, limit int) ([]*SLRecordComment, error) {
	q := db.WithContext(ctx).Where("sl_record_id = ? AND root_id = 0", slRecordID).
		Where("deleted_at IS NULL OR EXISTS (SELECT 1 FROM sl_record_comments r WHERE r.root_id = sl_record_comments.id AND r.deleted_at IS NULL)")
	if beforeID > 0 {
		q = q.Where("id < ?", beforeID)
	}
	var comments []*SLRecordComment
	if err := q.Order("id DESC").Limit(limit).Find(&comments).Error; err != nil {
		return nil, errors.Wrap(err, "find")
	}
	return comments, nil
}

func (db *slRecordComments) ListReplies(ctx context.Context, rootIDs []int64) ([]*SLRecordComment, error) {
	var comments []*SLRecordComment
	if len(rootIDs) == 0 {
		return comments, nil
	}
	if err := db.WithContext(ctx).Where("root_id IN ? AND deleted_at IS NULL", rootIDs).Order("id ASC").Find(&comments).Error; err != nil {
		return nil, errors.Wrap(err, "find")
	}
	return comments, nil
}

func (db *slRecordComments) HasReplies(ctx context.Context, rootID int64) (bool, error) {
	var found bool
	if err := db.WithContext(ctx).Raw("SELECT EXISTS (SELECT 1 FROM sl_record_comments WHERE root_id = ? AND deleted_at IS NULL)", rootID).
		Scan(&found).Error; err != nil {
		return false, errors.Wrap(err, "exists")
	}
	return found, nil
}

func (db *slRecordComments) Update(ctx context.Context, id int64, body string, mentions []CommentMention) error {
	if mentions == nil {
		mentions = []CommentMention{}
	}
	result := db.WithContext(ctx).Model(&SLRecordComment{}).Where("id = ? AND deleted_at IS NULL", id).Updates(map[string]interface{}{
		"body":      body,
		"mentions":  datatypes.JSONSlice[CommentMention](mentions),
		"edited_at": dbutil.Now(),
	})
	if result.Error != nil {
		return errors.Wrap(result.Error, "update")
	}
	if result.RowsAffected == 0 {
		return ErrSLRecordCommentNotFound
	}
	return nil
}

func (db *slRecordComments) Delete(ctx context.Context, comment *SLRecordComment) error {
	// Locking the record row before the comment keeps the lock order of deleting the record, which marks the record deleted before deleting the comments.
	var locked []int64
	if err := db.WithContext(ctx).Model(&SLRecord{}).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", comment.SLRecordID).Pluck("id", &locked).Error; err != nil {
		return errors.Wrap(err, "lock record")
	}
	result := db.WithContext(ctx).Model(&SLRecordComment{}).Where("id = ? AND deleted_at IS NULL", comment.ID).Updates(map[string]interface{}{
		"body":       "",
		"mentions":   datatypes.JSONSlice[CommentMention]{},
		"payload":    nil,
		"deleted_at": dbutil.Now(),
	})
	if result.Error != nil {
		return errors.Wrap(result.Error, "delete")
	}
	if result.RowsAffected == 0 {
		return nil
	}
	if err := db.WithContext(ctx).Model(&SLRecord{}).Where("id = ?", comment.SLRecordID).
		UpdateColumn("comment_count", gorm.Expr("GREATEST(comment_count - 1, 0)")).Error; err != nil {
		return errors.Wrap(err, "decrease comment count")
	}
	return nil
}

func (db *slRecordComments) ListTimeline(ctx context.Context, slRecordID int64, cursor *TimelineCursor, asc bool, limit int) ([]*SLRecordComment, error) {
	q := db.WithContext(ctx).Where("sl_record_id = ? AND deleted_at IS NULL", slRecordID)
	if cursor != nil {
		condition, args := cursor.condition(TimelineRankComment, asc)
		q = q.Where(condition, args...)
	}
	var comments []*SLRecordComment
	if err := q.Order(timelineOrder(asc)).Limit(limit).Find(&comments).Error; err != nil {
		return nil, errors.Wrap(err, "find")
	}
	return comments, nil
}

func (db *slRecordComments) LatestByTable(ctx context.Context, slTableID int64) (*SLRecordComment, error) {
	var comment SLRecordComment
	err := db.WithContext(ctx).Where("sl_table_id = ? AND deleted_at IS NULL", slTableID).
		Where("EXISTS (SELECT 1 FROM sl_records r WHERE r.id = sl_record_comments.sl_record_id AND r.deleted_at IS NULL)").
		Order("id DESC").First(&comment).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSLRecordCommentNotFound
		}
		return nil, errors.Wrap(err, "get")
	}
	return &comment, nil
}
