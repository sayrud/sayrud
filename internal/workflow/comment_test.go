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

func TestCommentTriggerAndStep(t *testing.T) {
	f := newLeadFixture(t)
	ctx := context.Background()
	human := &db.User{Email: "human@example.com", UserName: "Human"}
	require.NoError(t, f.db.Create(human).Error)
	bot := &db.User{Email: "bot@bots.invalid", UserName: "Helper", Kind: db.UserKindBot, OwnerID: human.ID}
	require.NoError(t, f.db.Create(bot).Error)
	members := db.NewProjectMembersStore(f.db)
	require.NoError(t, members.Set(ctx, f.project.ID, human.ID, db.ProjectRoleEditor))
	require.NoError(t, members.Set(ctx, f.project.ID, bot.ID, db.ProjectRoleEditor))
	uid := f.create(t, map[string]interface{}{f.name.UID: "Ann"})
	record, err := db.NewSLRecordsStore(f.db).GetByUID(ctx, uid)
	require.NoError(t, err)

	ackSpec := &Spec{
		Trigger: Trigger{Type: TriggerCommentCreated, Record: &RecordTrigger{TableUID: f.table.UID}, Comment: &CommentTrigger{AuthorKind: CommentAuthorHuman}},
		Steps: []Step{{ID: "ack", Type: StepComment, Config: &CommentConfig{
			TableUID: f.table.UID, RecordUID: vp("trigger.record.uid"),
			Body: &Value{Kind: ValueTemplate, Template: "Got it: {{trigger.comment.body}} from {{trigger.comment.author.name}}"},
		}}},
	}
	issues, err := Check(ctx, f.db, f.project, ackSpec)
	require.NoError(t, err)
	require.Empty(t, issues)
	ack := f.publish(t, "Acknowledge", ackSpec)
	addressed := f.publish(t, "Addressed", &Spec{
		Trigger: Trigger{Type: TriggerCommentCreated, Record: &RecordTrigger{TableUID: f.table.UID}, Comment: &CommentTrigger{MentionedBotID: bot.ID}},
		Steps:   []Step{{ID: "find", Type: StepQuery, Config: &QueryConfig{TableUID: f.table.UID, Limit: 1}}},
	})
	anyAuthor := f.publish(t, "Any", &Spec{
		Trigger: Trigger{Type: TriggerCommentCreated, Record: &RecordTrigger{TableUID: f.table.UID}},
		Steps:   []Step{{ID: "find", Type: StepQuery, Config: &QueryConfig{TableUID: f.table.UID, Limit: 1}}},
	})

	_, err = f.hub.CreateComment(ctx, f.project, f.table, record, collab.Origin{UserID: human.ID}, collab.CommentOptions{Body: "hello"})
	require.NoError(t, err)
	require.Len(t, f.runs(t, ack), 1)
	assert.Empty(t, f.runs(t, addressed), "the comment does not mention the bot")
	require.Len(t, f.runs(t, anyAuthor), 1)
	event := map[string]interface{}{}
	require.NoError(t, json.Unmarshal(f.runs(t, ack)[0].Event, &event))
	comment := event["comment"].(map[string]interface{})
	assert.Equal(t, "hello", comment["body"])
	assert.Equal(t, "Human", comment["author"].(map[string]interface{})["name"])
	assert.Equal(t, uid, event["record"].(map[string]interface{})["uid"])

	_, err = f.hub.CreateComment(ctx, f.project, f.table, record, collab.Origin{UserID: bot.ID}, collab.CommentOptions{Body: fmt.Sprintf("<@%d> ping", bot.ID)})
	require.NoError(t, err)
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
	result, err := f.engine.Debug(ctx, DebugInput{Project: f.project, Workflow: ack, Spec: ackSpec, Trigger: sample, DryRun: true})
	require.NoError(t, err)
	require.Equal(t, db.WorkflowRunSucceeded, result.Status, result.Response)
	assert.Equal(t, int64(3), f.count(t, &db.SLRecordComment{}, "sl_record_id = ?", record.ID))
}

func TestCheckCommentTrigger(t *testing.T) {
	f := newLeadFixture(t)
	ctx := context.Background()
	human := &db.User{Email: "human@example.com", UserName: "Human"}
	require.NoError(t, f.db.Create(human).Error)
	require.NoError(t, db.NewProjectMembersStore(f.db).Set(ctx, f.project.ID, human.ID, db.ProjectRoleEditor))
	spec := &Spec{
		Trigger: Trigger{Type: TriggerCommentCreated, Record: &RecordTrigger{TableUID: f.table.UID}, Comment: &CommentTrigger{AuthorKind: "robot", MentionedBotID: human.ID}},
		Steps:   []Step{{ID: "find", Type: StepQuery, Config: &QueryConfig{TableUID: f.table.UID, Limit: 1, Filters: []Filter{{FieldUID: f.name.UID, Op: "eq", Value: vp("trigger.comment.body")}}}}},
	}
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
