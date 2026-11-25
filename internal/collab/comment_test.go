package collab

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wuhan005/sayrud/internal/db"
)

func TestComments(t *testing.T) {
	ctx := context.Background()
	gormDB := clusterTestDB(t)
	hub := NewHub(gormDB)
	project, table, owner := testProject(t, gormDB, hub)
	member := &db.User{
		Email:    "member@example.test",
		UserName: "Member",
	}
	bot := &db.User{
		Email:    "bot@bots.invalid",
		UserName: "Bot",
		Kind:     db.UserKindBot,
		OwnerID:  owner.ID,
	}
	outsider := &db.User{
		Email:    "outsider@example.test",
		UserName: "Outsider",
	}
	require.NoError(t, gormDB.Create([]*db.User{member, bot, outsider}).Error)
	members := db.NewProjectMembersStore(gormDB)
	require.NoError(t, members.Set(ctx, project.ID, member.ID, db.ProjectRoleViewer))
	require.NoError(t, members.Set(ctx, project.ID, bot.ID, db.ProjectRoleEditor))
	record, err := db.NewSLRecordsStore(gormDB).GetByUID(ctx, "recAAAAAAAAAAA")
	require.NoError(t, err)

	conn := connectCluster(t, hub, project, table, member)
	// A public visitor subscribing the same table never receives the comments.
	visitor := &Client{
		hub:        hub,
		projectUID: project.UID,
		share:      &ShareSession{TableID: table.ID},
		out:        make(chan []byte, 8),
		tables:     map[string]*db.SLTable{table.UID: table},
	}
	hub.mu.Lock()
	hub.projects[project.UID][visitor] = struct{}{}
	hub.mu.Unlock()

	mention := func(u *db.User) string { return fmt.Sprintf("<@%d>", u.ID) }
	comment := func(author *db.User, body string, replyTo int64) (*Comment, error) {
		return hub.CreateComment(ctx, project, table, record, Origin{UserID: author.ID}, CommentOptions{
			Body:      body,
			ReplyToID: replyTo,
		})
	}
	root, err := comment(owner, "hi "+mention(member)+" "+mention(bot)+" "+mention(member), 0)
	require.NoError(t, err)
	assert.Equal(t, []db.CommentMention{db.NewCommentMention(member), db.NewCommentMention(bot)}, root.Mentions)
	assert.True(t, root.Mentions[1].IsBot)

	var changed commentChangedData
	require.NoError(t, json.Unmarshal(readClusterMessage(t, conn, MessageCommentChanged).Data, &changed))
	assert.Equal(t, CommentCreated, changed.Action)
	assert.Equal(t, int64(1), changed.CommentCount)
	assert.Equal(t, root.ID, changed.Comment.ID)
	var mentioned mentionedData
	require.NoError(t, json.Unmarshal(readClusterMessage(t, conn, MessageMentioned).Data, &mentioned))
	assert.Equal(t, root.ID, mentioned.CommentID)
	assert.Len(t, visitor.out, 0)

	_, err = comment(member, mention(outsider), 0)
	assert.ErrorIs(t, err, ErrInvalidMention)
	_, err = comment(member, "  \n", 0)
	assert.ErrorIs(t, err, ErrCommentEmpty)
	_, err = comment(member, strings.Repeat("评", MaxCommentLength+1), 0)
	assert.ErrorIs(t, err, ErrCommentTooLong)
	_, err = comment(member, strings.Repeat("评", MaxCommentLength), 0)
	require.NoError(t, err, "the limit counts characters rather than bytes")

	// The replies of a reply belong to the same root.
	reply, err := comment(bot, "reply", root.ID)
	require.NoError(t, err)
	nested, err := comment(member, "nested", reply.ID)
	require.NoError(t, err)
	assert.Equal(t, root.ID, nested.RootID)
	assert.Equal(t, reply.ID, nested.ReplyToID)
	_, err = comment(member, "x", 999999)
	assert.ErrorIs(t, err, db.ErrSLRecordCommentNotFound)

	// Editing keeps a mentioned member who left and rejects a new outsider.
	require.NoError(t, gormDB.Where("project_id = ? AND user_id = ?", project.ID, member.ID).Delete(&db.ProjectMember{}).Error)
	edited, err := hub.UpdateComment(ctx, project, table, record, root.ID, "still "+mention(member))
	require.NoError(t, err)
	assert.NotNil(t, edited.EditedAt)
	assert.Equal(t, []db.CommentMention{db.NewCommentMention(member)}, edited.Mentions)
	_, err = hub.UpdateComment(ctx, project, table, record, root.ID, mention(outsider))
	assert.ErrorIs(t, err, ErrInvalidMention)

	require.NoError(t, hub.DeleteComment(ctx, project, table, record, root.ID))
	latest, err := db.NewSLRecordsStore(gormDB).GetByID(ctx, record.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(3), latest.CommentCount)
	_, err = comment(owner, "x", root.ID)
	assert.ErrorIs(t, err, ErrCommentDeleted)
	_, err = hub.UpdateComment(ctx, project, table, record, root.ID, "again")
	assert.ErrorIs(t, err, ErrCommentDeleted)
}

func TestSharedMessagesDropComments(t *testing.T) {
	hub, _, project, session := shareFixture(t)
	c := &Client{
		hub:     hub,
		project: project,
		share:   session,
	}
	for _, kind := range []string{MessageCommentChanged, MessageMentioned} {
		message, allowed := c.sharedMessage(context.Background(), newMessage(kind, 0, map[string]string{"tableUID": "tbl"}))
		assert.True(t, allowed)
		assert.Nil(t, message, kind)
	}
}
