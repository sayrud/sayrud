package collab

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cockroachdb/errors"
	"github.com/samber/lo"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/routeutil"
)

// MaxCommentLength is the maximum number of characters of a comment body.
const MaxCommentLength = 10000

// mentionExcerptLength is the number of characters of the body sent to the mentioned users.
const mentionExcerptLength = 140

var (
	ErrCommentEmpty   = errors.New("the comment is empty")
	ErrCommentTooLong = errors.New("the comment is too long")
	ErrCommentDeleted = errors.New("the comment has been deleted")
	// ErrInvalidMention means a mentioned user is not a member of the project.
	ErrInvalidMention = errors.New("invalid mention")
)

// MentionPattern matches a mention `<@userID>` in a comment body, the ID does not change when the user is renamed.
var MentionPattern = regexp.MustCompile(`<@(\d{1,16})>`)

// The message types of the comments sent by the server.
const (
	// MessageCommentChanged tells the subscribers of a table a comment is created, updated or deleted.
	MessageCommentChanged = "COMMENT_CHANGED"
	// MessageMentioned tells a user online it is mentioned by a new comment.
	MessageMentioned = "MENTIONED"
)

// The actions of MessageCommentChanged.
const (
	CommentCreated = "created"
	CommentUpdated = "updated"
	CommentDeleted = "deleted"
)

// Comment is a record comment returned by the API and the WebSocket.
type Comment struct {
	ID        int64  `json:"id"`
	RecordUID string `json:"recordUID"`
	// Author is the user or the workflow writing the comment.
	Author db.SLRecordWriter `json:"author"`
	// Body is the Markdown text with mentions as `<@userID>`, empty if deleted.
	Body string `json:"body"`
	// RootID is the root comment of a reply, 0 for the roots; ReplyToID is the comment replied to.
	RootID    int64               `json:"rootId,omitempty"`
	ReplyToID int64               `json:"replyToId,omitempty"`
	Kind      string              `json:"kind" enums:"comment,report,attention"`
	Mentions  []db.CommentMention `json:"mentions"`
	// Deleted marks the placeholder of a deleted root comment which still has replies.
	Deleted   bool       `json:"deleted,omitempty"`
	EditedAt  *time.Time `json:"editedAt,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
	// Replies are the live replies of a root comment in creation order, only set in the comment list.
	Replies []*Comment `json:"replies,omitempty"`
} // @name RecordComment

// NewComment returns the comment of the record for the responses.
func NewComment(recordUID string, c *db.SLRecordComment) *Comment {
	mentions := []db.CommentMention(c.Mentions)
	if mentions == nil {
		mentions = []db.CommentMention{}
	}
	return &Comment{
		ID:        c.ID,
		RecordUID: recordUID,
		Author:    c.Author(),
		Body:      c.Body,
		RootID:    c.RootID,
		ReplyToID: c.ReplyToID,
		Kind:      string(c.Kind),
		Mentions:  mentions,
		Deleted:   c.DeletedAt != nil,
		EditedAt:  c.EditedAt,
		CreatedAt: c.CreatedAt,
	}
}

type commentChangedData struct {
	TableUID     string   `json:"tableUID"`
	RecordUID    string   `json:"recordUID"`
	Action       string   `json:"action"`
	Comment      *Comment `json:"comment"`
	CommentCount int64    `json:"commentCount"`
}

type mentionedData struct {
	ProjectUID string            `json:"projectUID"`
	TableUID   string            `json:"tableUID"`
	RecordUID  string            `json:"recordUID"`
	CommentID  int64             `json:"commentId"`
	Author     db.SLRecordWriter `json:"author"`
	Excerpt    string            `json:"excerpt"`
}

// CommentEvent is a comment created in a write transaction, which may trigger workflows.
type CommentEvent struct {
	Record  *db.SLRecord
	Comment *db.SLRecordComment
}

// commentNotice is the notification of a committed comment change.
type commentNotice struct {
	project *db.Project
	table   *db.SLTable
	record  *db.SLRecord
	action  string
	comment *db.SLRecordComment
	count   int64
	// mentioned are the users newly mentioned, who are told if online.
	mentioned []int64
}

// checkCommentBody rejects the empty and too long bodies.
func checkCommentBody(body string) error {
	if strings.TrimSpace(body) == "" {
		return ErrCommentEmpty
	}
	if utf8.RuneCountInString(body) > MaxCommentLength {
		return ErrCommentTooLong
	}
	return nil
}

// parseMentions returns the users mentioned by the body in order of first appearance; each must be a member of the project, or already mentioned in previous so that editing keeps the members who left.
func parseMentions(ctx context.Context, tx *gorm.DB, projectID int64, body string, previous []db.CommentMention) ([]db.CommentMention, error) {
	var ids []int64
	for _, m := range MentionPattern.FindAllStringSubmatch(body, -1) {
		if id, err := strconv.ParseInt(m[1], 10, 64); err == nil && id > 0 && !lo.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	mentions := make([]db.CommentMention, 0, len(ids))
	if len(ids) == 0 {
		return mentions, nil
	}
	directory, err := routeutil.LoadDirectory(ctx, tx, projectID)
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		if old, ok := lo.Find(previous, func(m db.CommentMention) bool { return m.UserID == id }); ok {
			mentions = append(mentions, old)
			continue
		}
		user := directory.User(id)
		if user == nil || !directory.IsMember(id) {
			return nil, errors.Wrapf(ErrInvalidMention, "user %d", id)
		}
		mentions = append(mentions, db.CommentMention{UserID: id, IsBot: user.IsBot()})
	}
	return mentions, nil
}

// newlyMentioned returns the users of mentions who are not in previous, excluding the author.
func newlyMentioned(mentions, previous []db.CommentMention, author db.SLRecordWriter) []int64 {
	var ids []int64
	for _, m := range mentions {
		if m.UserID != author.UserID && !lo.ContainsBy(previous, func(p db.CommentMention) bool { return p.UserID == m.UserID }) {
			ids = append(ids, m.UserID)
		}
	}
	return ids
}

// CommentOptions are the options of creating a comment.
type CommentOptions struct {
	Body string
	// ReplyToID is the live comment of the same record replied to, 0 for a root comment.
	ReplyToID int64
}

// createComment creates the comment in the transaction and enqueues the workflows it triggers.
// It returns db.ErrSLRecordCommentNotFound if the comment replied to does not belong to the record, and ErrCommentDeleted if it is deleted.
func (h *Hub) createComment(ctx context.Context, tx *gorm.DB, project *db.Project, table *db.SLTable, record *db.SLRecord, origin Origin, opts CommentOptions) (*commentNotice, bool, error) {
	if err := checkCommentBody(opts.Body); err != nil {
		return nil, false, err
	}
	comments := db.NewSLRecordCommentsStore(tx)
	var rootID int64
	if opts.ReplyToID != 0 {
		target, err := comments.GetByID(ctx, opts.ReplyToID)
		if err != nil {
			return nil, false, err
		}
		if target.SLRecordID != record.ID {
			return nil, false, db.ErrSLRecordCommentNotFound
		}
		if target.DeletedAt != nil {
			return nil, false, ErrCommentDeleted
		}
		rootID = lo.Ternary(target.RootID != 0, target.RootID, target.ID)
	}
	mentions, err := parseMentions(ctx, tx, table.ProjectID, opts.Body, nil)
	if err != nil {
		return nil, false, err
	}
	author := origin.Writer()
	comment, err := comments.Create(ctx, db.CreateSLRecordCommentOptions{
		SLTableID:  table.ID,
		SLRecordID: record.ID,
		Author:     author,
		Body:       opts.Body,
		RootID:     rootID,
		ReplyToID:  opts.ReplyToID,
		Kind:       db.SLRecordCommentKindComment,
		Mentions:   mentions,
	})
	if err != nil {
		return nil, false, err
	}
	latest, err := db.NewSLRecordsStore(tx).GetByID(ctx, record.ID)
	if err != nil {
		return nil, false, errors.Wrap(err, "get record")
	}

	queued := false
	if h.workflows != nil && project != nil {
		n, err := h.workflows.CommentCreated(ctx, tx, project, table, origin, CommentEvent{Record: latest, Comment: comment})
		if err != nil {
			return nil, false, err
		}
		queued = n > 0
	}
	return &commentNotice{
		project: project, table: table, record: latest, action: CommentCreated, comment: comment, count: latest.CommentCount,
		mentioned: newlyMentioned(mentions, nil, author),
	}, queued, nil
}

// CreateComment creates a comment on the record by the writer of origin, enqueues the workflows it triggers in the same transaction,
// and notifies the subscribers of the table and the mentioned users after committing.
// It returns ErrCommentEmpty, ErrCommentTooLong, ErrInvalidMention, ErrCommentDeleted, db.ErrSLRecordCommentNotFound or db.ErrSLRecordNotFound if rejected.
func (h *Hub) CreateComment(ctx context.Context, project *db.Project, table *db.SLTable, record *db.SLRecord, origin Origin, opts CommentOptions) (*Comment, error) {
	var notice *commentNotice
	queued := false
	if err := h.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		notice, queued, err = h.createComment(ctx, tx, project, table, record, origin, opts)
		return err
	}); err != nil {
		return nil, err
	}
	h.notifyComment(notice)
	h.runsQueued(queued)
	return NewComment(record.UID, notice.comment), nil
}

// UpdateComment replaces the body of the live comment and parses its mentions again, the users newly mentioned are told.
// It returns ErrCommentDeleted if the comment has been deleted, and the same errors as CreateComment for the body.
func (h *Hub) UpdateComment(ctx context.Context, project *db.Project, table *db.SLTable, record *db.SLRecord, commentID int64, body string) (*Comment, error) {
	var notice *commentNotice
	if err := h.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := checkCommentBody(body); err != nil {
			return err
		}
		comments := db.NewSLRecordCommentsStore(tx)
		previous, err := comments.GetByID(ctx, commentID)
		if err != nil {
			return err
		}
		if previous.DeletedAt != nil {
			return ErrCommentDeleted
		}
		mentions, err := parseMentions(ctx, tx, table.ProjectID, body, previous.Mentions)
		if err != nil {
			return err
		}
		if err := comments.Update(ctx, commentID, body, mentions); err != nil {
			return err
		}
		comment, err := comments.GetByID(ctx, commentID)
		if err != nil {
			return err
		}
		notice = &commentNotice{
			project: project, table: table, record: record, action: CommentUpdated, comment: comment, count: record.CommentCount,
			mentioned: newlyMentioned(mentions, previous.Mentions, comment.Author()),
		}
		latest, err := db.NewSLRecordsStore(tx).GetByID(ctx, record.ID)
		if err == nil {
			notice.count = latest.CommentCount
		}
		return err
	}); err != nil {
		return nil, err
	}
	h.notifyComment(notice)
	return NewComment(record.UID, notice.comment), nil
}

// DeleteComment deletes the comment, a deleted root with live replies stays as a placeholder.
func (h *Hub) DeleteComment(ctx context.Context, project *db.Project, table *db.SLTable, record *db.SLRecord, commentID int64) error {
	var notice *commentNotice
	if err := h.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		comments := db.NewSLRecordCommentsStore(tx)
		comment, err := comments.GetByID(ctx, commentID)
		if err != nil {
			return err
		}
		if err := comments.Delete(ctx, comment); err != nil {
			return err
		}
		if comment, err = comments.GetByID(ctx, commentID); err != nil {
			return err
		}
		latest, err := db.NewSLRecordsStore(tx).GetByID(ctx, record.ID)
		if err != nil {
			return err
		}
		notice = &commentNotice{project: project, table: table, record: latest, action: CommentDeleted, comment: comment, count: latest.CommentCount}
		return nil
	}); err != nil {
		return err
	}
	h.notifyComment(notice)
	return nil
}

// CreateComment creates a comment on the record of the table by the writer of the batch, the subscribers are notified after committing.
// It returns db.ErrSLRecordNotFound if the record does not belong to the table, and the same errors as Hub.CreateComment for the body.
func (b *Batch) CreateComment(table *db.SLTable, recordUID, body string) (*db.SLRecordComment, error) {
	record, err := db.NewSLRecordsStore(b.tx).GetByUID(b.ctx, recordUID)
	if err != nil {
		return nil, err
	}
	if record.SLTableID != table.ID {
		return nil, db.ErrSLRecordNotFound
	}
	notice, queued, err := b.hub.createComment(b.ctx, b.tx, b.project, table, record, b.origin, CommentOptions{Body: body})
	if err != nil {
		return nil, err
	}
	b.comments = append(b.comments, notice)
	b.queued = b.queued || queued
	return notice.comment, nil
}

// notifyComment sends the committed comment change to the subscribers of the table and the newly mentioned users of the project, except the public visitors.
func (h *Hub) notifyComment(n *commentNotice) {
	if n == nil || n.project == nil {
		return
	}
	comment := NewComment(n.record.UID, n.comment)
	h.broadcastEvent(clusterEvent{
		Kind: clusterEventComment, ProjectUID: n.project.UID, TableUID: n.table.UID,
		Message: newMessage(MessageCommentChanged, 0, commentChangedData{
			TableUID: n.table.UID, RecordUID: n.record.UID, Action: n.action, Comment: comment, CommentCount: n.count,
		}),
	})
	if len(n.mentioned) == 0 {
		return
	}
	excerpt := []rune(n.comment.Body)
	message := newMessage(MessageMentioned, 0, mentionedData{
		ProjectUID: n.project.UID, TableUID: n.table.UID, RecordUID: n.record.UID, CommentID: n.comment.ID,
		Author: n.comment.Author(), Excerpt: string(excerpt[:min(len(excerpt), mentionExcerptLength)]),
	})
	for _, userID := range n.mentioned {
		h.broadcastEvent(clusterEvent{Kind: clusterEventMention, ProjectUID: n.project.UID, UserID: userID, Message: message})
	}
}

// deliverComment sends the comment message to the members subscribing the table on this instance.
func (h *Hub) deliverComment(projectUID, tableUID string, message []byte) {
	for _, c := range h.clients(projectUID) {
		if c.share == nil && c.subscribed(tableUID) {
			c.send(message)
		}
	}
}

// deliverMention sends the mention message to the connections of the user on this instance.
func (h *Hub) deliverMention(projectUID string, userID int64, message []byte) {
	for _, c := range h.clients(projectUID) {
		if c.share == nil && c.userID == userID {
			c.send(message)
		}
	}
}
