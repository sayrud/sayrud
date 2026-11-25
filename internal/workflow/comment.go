package workflow

import (
	"context"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/samber/lo"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/collab"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/routeutil"
)

// CommentCreated enqueues, within the transaction writing the comment, the runs of the comment_created workflows of the table whose filters match; a workflow is never triggered by its own comments.
func (e *Engine) CommentCreated(ctx context.Context, tx *gorm.DB, project *db.Project, table *db.SLTable, origin collab.Origin, ev collab.CommentEvent) (int, error) {
	if origin.Depth >= maxTriggerDepth {
		return 0, nil
	}
	list, err := db.NewWorkflowsStore(tx).ListByTriggerTable(ctx, table.ID, []string{TriggerCommentCreated})
	if err != nil {
		return 0, errors.Wrap(err, "list triggered workflows")
	}
	list = lo.Filter(list, func(wf *db.Workflow, _ int) bool { return wf.ID != origin.WorkflowID })
	if len(list) == 0 {
		return 0, nil
	}
	fields, err := db.NewSLFieldsStore(tx).ListByTableID(ctx, table.ID)
	if err != nil {
		return 0, errors.Wrap(err, "list fields")
	}
	info := newTableInfo(table, fields)
	if info.directory, err = routeutil.LoadDirectory(ctx, tx, table.ProjectID); err != nil {
		return 0, errors.Wrap(err, "load members")
	}
	authorKind := commentAuthorKind(info.directory, ev.Comment)

	total := 0
	for _, wf := range list {
		log := logrus.WithContext(ctx).WithField("workflow", wf.UID)
		spec, err := e.PublishedSpec(ctx, wf.PublishedRevisionID)
		if err != nil {
			log.WithError(err).Error("Failed to load workflow revision")
			continue
		}
		if !matchComment(spec.Trigger.Comment, authorKind, ev.Comment) {
			continue
		}
		trigger := commentTriggerData(info, ev.Record, ev.Comment, formulaLocation(spec.FormulaTimezone()))
		if _, err := e.enqueue(ctx, tx, enqueueOptions{
			ProjectID:   project.ID,
			Workflow:    wf,
			RevisionID:  wf.PublishedRevisionID,
			Version:     wf.PublishedVersion,
			TriggerType: spec.Trigger.Type,
			Timezone:    spec.FormulaTimezone(),
			Depth:       origin.Depth + 1,
			Triggers:    []map[string]interface{}{trigger},
		}); err != nil {
			return 0, err
		}
		total++
	}
	return total, nil
}

// commentAuthorKind returns human or bot for the comments of the users, and empty for the comments of the workflows.
func commentAuthorKind(directory *routeutil.Directory, comment *db.SLRecordComment) string {
	if comment.AuthorID == 0 {
		return ""
	}
	if user := directory.User(comment.AuthorID); user != nil && user.IsBot() {
		return CommentAuthorBot
	}
	return CommentAuthorHuman
}

// matchComment reports whether the comment written by the kind of author passes the filters of the trigger.
func matchComment(t *CommentTrigger, authorKind string, comment *db.SLRecordComment) bool {
	if t == nil {
		return true
	}
	if t.AuthorKind != "" && t.AuthorKind != authorKind {
		return false
	}
	return t.MentionedBotID == 0 || lo.ContainsBy(comment.Mentions, func(m db.CommentMention) bool { return m.UserID == t.MentionedBotID && m.IsBot })
}

// commentTriggerData returns the trigger variables of comment_created: record, table and comment.
func commentTriggerData(info *tableInfo, record *db.SLRecord, comment *db.SLRecordComment, loc *time.Location) map[string]interface{} {
	data := map[string]interface{}{
		"record":  recordOutput(info, record, loc),
		"table":   tableData(info.table),
		"comment": commentOutput(info.directory, comment),
	}
	return normalize(data).(map[string]interface{})
}

// commentOutput returns the variable of a comment, the author is the same as the writer variable.
func commentOutput(directory *routeutil.Directory, comment *db.SLRecordComment) map[string]interface{} {
	mentions := make([]interface{}, 0, len(comment.Mentions))
	for _, m := range comment.Mentions {
		u := userOutput(directory, m.UserID)
		mentions = append(mentions, map[string]interface{}{
			"id":    u["id"],
			"name":  u["name"],
			"isBot": m.IsBot,
		})
	}
	return map[string]interface{}{
		"id":        float64(comment.ID),
		"body":      comment.Body,
		"author":    writerOutput(directory, comment.Author()),
		"mentions":  mentions,
		"rootId":    float64(comment.RootID),
		"replyToId": float64(comment.ReplyToID),
		"createdAt": comment.CreatedAt.UTC().Format(time.RFC3339),
	}
}

// checkCommentTrigger checks the table and the filters of a comment_created trigger.
func (c *checker) checkCommentTrigger() error {
	info, err := c.triggerTable()
	if err != nil {
		return err
	}
	if info == nil {
		c.add("trigger.table_missing", "trigger")
		return nil
	}
	t := c.spec.Trigger.Comment
	if t == nil {
		return nil
	}
	if t.AuthorKind != "" && t.AuthorKind != CommentAuthorHuman && t.AuthorKind != CommentAuthorBot {
		c.add("trigger.invalid_condition", "trigger")
	}
	if t.MentionedBotID != 0 {
		directory, err := routeutil.LoadDirectory(c.ctx, c.db, c.project.ID)
		if err != nil {
			return errors.Wrap(err, "load members")
		}
		if user := directory.User(t.MentionedBotID); user == nil || !user.IsBot() || !directory.IsMember(t.MentionedBotID) {
			c.add("trigger.bot_missing", "trigger")
		}
	}
	return nil
}

// comment writes a comment on the record by the workflow.
func (x *execution) comment(step *Step) (interface{}, error) {
	cfg := configOf[CommentConfig](step)
	info, err := x.table(cfg.TableUID)
	if err != nil {
		return nil, err
	}
	v, err := x.scope.Resolve(cfg.RecordUID)
	if err != nil {
		return nil, err
	}
	recordUID := Text(v)
	if recordUID == "" {
		return nil, db.ErrSLRecordNotFound
	}
	body, err := x.scope.Resolve(cfg.Body)
	if err != nil {
		return nil, err
	}

	apply := func(b *collab.Batch) (interface{}, error) {
		comment, err := b.CreateComment(info.table, recordUID, Text(body))
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"comment": map[string]interface{}{
				"id":        float64(comment.ID),
				"body":      comment.Body,
				"createdAt": comment.CreatedAt.UTC().Format(time.RFC3339),
			},
		}, nil
	}
	if x.batch != nil {
		return apply(x.batch)
	}
	var output interface{}
	err = x.hub.Batch(x.ctx, x.project, nil, collab.BatchOptions{Origin: x.origin}, func(b *collab.Batch) error {
		out, err := apply(b)
		if err != nil {
			return err
		}
		output = normalize(out)
		if x.persist != nil {
			return x.persist(b.Tx(), step.ID, output)
		}
		return nil
	})
	return output, err
}
