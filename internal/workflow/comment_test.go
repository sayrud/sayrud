package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wuhan005/sayrud/internal/collab"
	"github.com/wuhan005/sayrud/internal/db"
)

// commentSpec returns a comment_created workflow of the leads table with the filters.
func (f *leadFixture) commentSpec(filters *CommentTrigger, steps ...Step) *Spec {
	return &Spec{
		Trigger: Trigger{
			Type:    TriggerCommentCreated,
			Record:  &RecordTrigger{TableUID: f.table.UID},
			Comment: filters,
		},
		Steps: steps,
	}
}

// queryStep returns a step reading a lead, which has no side effects.
func (f *leadFixture) queryStep(filters ...Filter) Step {
	return Step{
		ID:   "find",
		Type: StepQuery,
		Config: &QueryConfig{
			TableUID: f.table.UID,
			Limit:    1,
			Filters:  filters,
		},
	}
}

// addUser creates a user of the kind and adds it to the project as an editor.
func (f *leadFixture) addUser(t *testing.T, name string, kind db.UserKind, ownerID int64) *db.User {
	t.Helper()
	user := &db.User{
		Email:    name + "@example.com",
		UserName: name,
		Kind:     kind,
		OwnerID:  ownerID,
	}
	require.NoError(t, f.db.Create(user).Error)
	require.NoError(t, db.NewProjectMembersStore(f.db).Set(context.Background(), f.project.ID, user.ID, db.ProjectRoleEditor))
	return user
}

func TestCommentTriggerAndStep(t *testing.T) {
	f := newLeadFixture(t)
	ctx := context.Background()
	human := f.addUser(t, "Human", db.UserKindHuman, 0)
	bot := f.addUser(t, "Helper", db.UserKindBot, human.ID)
	uid := f.create(t, map[string]interface{}{f.name.UID: "Ann"})
	record, err := db.NewSLRecordsStore(f.db).GetByUID(ctx, uid)
	require.NoError(t, err)

	ackSpec := f.commentSpec(&CommentTrigger{AuthorKind: CommentAuthorHuman}, Step{
		ID:   "ack",
		Type: StepComment,
		Config: &CommentConfig{
			TableUID:  f.table.UID,
			RecordUID: vp("trigger.record.uid"),
			Body: &Value{
				Kind:     ValueTemplate,
				Template: "Got it: {{trigger.comment.body}} from {{trigger.comment.author.name}}",
			},
		},
	})
	issues, err := Check(ctx, f.db, f.project, ackSpec)
	require.NoError(t, err)
	require.Empty(t, issues)
	assert.Equal(t, []string{f.table.UID}, ackSpec.WriteTables(), "a synchronous flow locks the tables it comments on")
	ack := f.publish(t, "Acknowledge", ackSpec)
	addressed := f.publish(t, "Addressed", f.commentSpec(&CommentTrigger{MentionedBotID: bot.ID}, f.queryStep()))
	anyAuthor := f.publish(t, "Any", f.commentSpec(nil, f.queryStep()))

	comment := func(author *db.User, body string) {
		_, err := f.hub.CreateComment(ctx, f.project, f.table, record, collab.Origin{UserID: author.ID}, collab.CommentOptions{Body: body})
		require.NoError(t, err)
	}
	comment(human, "hello")
	require.Len(t, f.runs(t, ack), 1)
	assert.Empty(t, f.runs(t, addressed), "the comment does not mention the bot")
	require.Len(t, f.runs(t, anyAuthor), 1)
	event := map[string]interface{}{}
	require.NoError(t, json.Unmarshal(f.runs(t, ack)[0].Event, &event))
	trigger := event["comment"].(map[string]interface{})
	assert.Equal(t, "hello", trigger["body"])
	assert.Equal(t, "Human", trigger["author"].(map[string]interface{})["name"])
	assert.Equal(t, uid, event["record"].(map[string]interface{})["uid"])

	comment(bot, fmt.Sprintf("<@%d> ping", bot.ID))
	assert.Len(t, f.runs(t, ack), 1, "the comments of bots are not written by humans")
	require.Len(t, f.runs(t, addressed), 1)
	assert.Len(t, f.runs(t, anyAuthor), 2)

	runCtx, cancel := context.WithCancel(context.Background())
	f.engine.Start(runCtx)
	defer func() {
		cancel()
		_ = f.engine.Shutdown(context.Background(), context.Background())
	}()
	require.Eventually(t, func() bool { return f.runs(t, ack)[0].Status == db.WorkflowRunSucceeded }, 15*time.Second, 100*time.Millisecond)
	var written db.SLRecordComment
	require.NoError(t, f.db.Where("author_workflow_id = ?", ack.ID).First(&written).Error)
	assert.Equal(t, "Got it: hello from Human", written.Body)
	assert.Zero(t, written.AuthorID)
	require.Eventually(t, func() bool { return len(f.runs(t, anyAuthor)) == 3 }, 5*time.Second, 50*time.Millisecond, "the comments of a workflow trigger the others")
	assert.Len(t, f.runs(t, ack), 1, "a workflow is not triggered by its own comments")
	latest, err := db.NewSLRecordsStore(f.db).GetByID(ctx, record.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(3), latest.CommentCount)

	// Debugging runs the comment step in a rolled-back transaction.
	sample, err := f.engine.SampleTrigger(ctx, f.project, ackSpec, "")
	require.NoError(t, err)
	assert.Equal(t, written.Body, sample["comment"].(map[string]interface{})["body"], "the sample is the latest comment of the table")
	result, err := f.engine.Debug(ctx, DebugInput{
		Project:  f.project,
		Workflow: ack,
		Spec:     ackSpec,
		Trigger:  sample,
		DryRun:   true,
	})
	require.NoError(t, err)
	require.Equal(t, db.WorkflowRunSucceeded, result.Status, result.Response)
	assert.Equal(t, int64(3), f.count(t, &db.SLRecordComment{}, "sl_record_id = ?", record.ID))
}

func TestCheckCommentTrigger(t *testing.T) {
	f := newLeadFixture(t)
	ctx := context.Background()
	human := f.addUser(t, "Human", db.UserKindHuman, 0)
	filters := &CommentTrigger{
		AuthorKind:     "robot",
		MentionedBotID: human.ID,
	}
	spec := f.commentSpec(filters, f.queryStep(Filter{
		FieldUID: f.name.UID,
		Op:       "eq",
		Value:    vp("trigger.comment.body"),
	}))
	issues, err := Check(ctx, f.db, f.project, spec)
	require.NoError(t, err)
	codes := map[string]bool{}
	for _, issue := range issues {
		codes[issue.Code] = true
	}
	assert.True(t, codes["trigger.invalid_condition"], issues)
	assert.True(t, codes["trigger.bot_missing"], "the mentioned user must be a bot of the project")
	assert.False(t, codes["value.unknown_var"], "the comment is a trigger variable")
}
