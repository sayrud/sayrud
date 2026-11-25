package db

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecordChanges(t *testing.T) {
	ctx := context.Background()
	tx := newTestDB(t, &SLField{}, &SLRecord{}, &SLRecordChange{}, &SLRecordComment{})
	fields := NewSLFieldsStore(tx)
	title, err := fields.Create(ctx, CreateSLFieldOptions{SLTableID: 1, Label: "Title", Type: TextFieldType})
	require.NoError(t, err)
	score, err := fields.Create(ctx, CreateSLFieldOptions{SLTableID: 1, Label: "Score", Type: NumberFieldType})
	require.NoError(t, err)

	records := NewSLRecordsStore(tx)
	changes := NewSLRecordChangesStore(tx)
	user := SLRecordWriter{UserID: 3}
	record, err := records.Create(ctx, 1, json.RawMessage(`{"`+title.UID+`":"a"}`), user)
	require.NoError(t, err)

	list, err := changes.ListByRecord(ctx, record.ID, 0, 10)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, SLRecordChangeCreated, list[0].Kind)
	assert.Empty(t, list[0].FieldUID, "a creation does not record the values")
	assert.Equal(t, user, list[0].Writer())

	// The key order and number formatting do not count as changes.
	require.NoError(t, records.Update(ctx, record.ID, json.RawMessage(`{"`+score.UID+`":1.50,"`+title.UID+`":"a"}`), SLRecordWriter{WorkflowID: 9}))
	list, err = changes.ListByRecord(ctx, record.ID, 0, 10)
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, SLRecordChangeUpdated, list[0].Kind)
	assert.Equal(t, score.UID, list[0].FieldUID)
	assert.Equal(t, "Score", list[0].FieldLabel)
	assert.Equal(t, NumberFieldType, list[0].FieldType)
	assert.Nil(t, list[0].OldValue)
	assert.JSONEq(t, `1.5`, string(list[0].NewValue))
	assert.Equal(t, SLRecordWriter{WorkflowID: 9}, list[0].Writer())

	latest, err := records.GetByID(ctx, record.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(9), latest.UpdatedByWorkflowID)
	assert.True(t, latest.UpdatedAt.After(record.UpdatedAt) || latest.UpdatedAt.Equal(record.UpdatedAt))

	// The label is a snapshot, renaming or deleting the field later keeps it.
	require.NoError(t, fields.SetLabel(ctx, title.ID, "Renamed"))
	require.NoError(t, records.Update(ctx, record.ID, json.RawMessage(`{"`+score.UID+`":1.5}`), user))
	require.NoError(t, fields.SetLabel(ctx, title.ID, "Again"))
	list, err = changes.ListByRecord(ctx, record.ID, 0, 1)
	require.NoError(t, err)
	assert.Equal(t, "Renamed", list[0].FieldLabel)
	assert.JSONEq(t, `"a"`, string(list[0].OldValue))
	assert.Nil(t, list[0].NewValue)

	// The writes of the server are not recorded.
	require.NoError(t, records.Update(ctx, record.ID, json.RawMessage(`{"`+score.UID+`":2}`), SLRecordWriter{}))
	n, err := changes.ListByRecord(ctx, record.ID, 0, 10)
	require.NoError(t, err)
	assert.Len(t, n, 3)
	latest, err = records.GetByID(ctx, record.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(3), latest.UpdatedByID, "a zero writer keeps the modifier")

	imported, err := records.Import(ctx, 1, ImportSLRecordsOptions{Data: []json.RawMessage{json.RawMessage(`{}`), json.RawMessage(`{}`)}, Writer: user})
	require.NoError(t, err)
	for _, r := range imported {
		list, err := changes.ListByRecord(ctx, r.ID, 0, 10)
		require.NoError(t, err)
		require.Len(t, list, 1)
		assert.Equal(t, SLRecordChangeCreated, list[0].Kind)
	}

	comments := NewSLRecordCommentsStore(tx)
	_, err = comments.Create(ctx, CreateSLRecordCommentOptions{SLTableID: 1, SLRecordID: imported[0].ID, Author: user, Body: "hi"})
	require.NoError(t, err)
	require.NoError(t, records.DeleteByUIDs(ctx, 1, []string{imported[0].UID, imported[1].UID}, user))
	list, err = changes.ListByRecord(ctx, imported[0].ID, 0, 10)
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, SLRecordChangeDeleted, list[0].Kind)
	var left int64
	require.NoError(t, tx.Model(&SLRecordComment{}).Where("sl_record_id = ?", imported[0].ID).Count(&left).Error)
	assert.Zero(t, left, "the comments are deleted with the record")
	_, err = comments.Create(ctx, CreateSLRecordCommentOptions{SLTableID: 1, SLRecordID: imported[0].ID, Author: user, Body: "late"})
	assert.ErrorIs(t, err, ErrSLRecordNotFound)

	got, err := changes.GetByID(ctx, record.ID, n[0].ID)
	require.NoError(t, err)
	assert.Equal(t, n[0].ID, got.ID)
	_, err = changes.GetByID(ctx, imported[0].ID, n[0].ID)
	assert.ErrorIs(t, err, ErrSLRecordChangeNotFound)
}

func TestRecordComments(t *testing.T) {
	ctx := context.Background()
	tx := newTestDB(t, &SLField{}, &SLRecord{}, &SLRecordChange{}, &SLRecordComment{})
	records := NewSLRecordsStore(tx)
	record, err := records.Create(ctx, 1, json.RawMessage(`{}`), SLRecordWriter{UserID: 1})
	require.NoError(t, err)
	comments := NewSLRecordCommentsStore(tx)
	create := func(body string, root, replyTo int64) *SLRecordComment {
		c, err := comments.Create(ctx, CreateSLRecordCommentOptions{SLTableID: 1, SLRecordID: record.ID, Author: SLRecordWriter{UserID: 1}, Body: body, RootID: root, ReplyToID: replyTo})
		require.NoError(t, err)
		return c
	}
	count := func() int64 {
		r, err := records.GetByID(ctx, record.ID)
		require.NoError(t, err)
		return r.CommentCount
	}
	first := create("first", 0, 0)
	second := create("second", 0, 0)
	reply := create("reply", first.ID, first.ID)
	assert.Equal(t, int64(3), count())
	updatedAt := record.UpdatedAt
	r, err := records.GetByID(ctx, record.ID)
	require.NoError(t, err)
	assert.Equal(t, updatedAt.UnixMicro(), r.UpdatedAt.UnixMicro(), "comments do not touch the record")

	roots, err := comments.ListRoots(ctx, record.ID, 0, 10)
	require.NoError(t, err)
	assert.Equal(t, []int64{second.ID, first.ID}, lo.Map(roots, func(c *SLRecordComment, _ int) int64 { return c.ID }))
	roots, err = comments.ListRoots(ctx, record.ID, second.ID, 10)
	require.NoError(t, err)
	assert.Equal(t, []int64{first.ID}, lo.Map(roots, func(c *SLRecordComment, _ int) int64 { return c.ID }))

	// A deleted root stays as a placeholder while it has replies, and disappears after them.
	require.NoError(t, comments.Delete(ctx, first))
	require.NoError(t, comments.Delete(ctx, first), "deleting twice is a no-op")
	assert.Equal(t, int64(2), count())
	roots, err = comments.ListRoots(ctx, record.ID, 0, 10)
	require.NoError(t, err)
	require.Len(t, roots, 2)
	assert.NotNil(t, roots[1].DeletedAt)
	assert.Empty(t, roots[1].Body)
	replies, err := comments.ListReplies(ctx, []int64{first.ID})
	require.NoError(t, err)
	assert.Len(t, replies, 1)
	require.NoError(t, comments.Delete(ctx, reply))
	roots, err = comments.ListRoots(ctx, record.ID, 0, 10)
	require.NoError(t, err)
	assert.Len(t, roots, 1)
	assert.Equal(t, int64(1), count())

	require.NoError(t, comments.Update(ctx, second.ID, "edited <@2>", []CommentMention{{UserID: 2, IsBot: true}}))
	got, err := comments.GetByID(ctx, second.ID)
	require.NoError(t, err)
	assert.Equal(t, "edited <@2>", got.Body)
	assert.NotNil(t, got.EditedAt)
	assert.Equal(t, []CommentMention{{UserID: 2, IsBot: true}}, []CommentMention(got.Mentions))

	latest, err := comments.LatestByTable(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, second.ID, latest.ID)

	// The timeline pages by time, rank and ID in both directions.
	all, err := comments.ListTimeline(ctx, record.ID, nil, true, 10)
	require.NoError(t, err)
	require.Len(t, all, 1)
	cursor := &TimelineCursor{Time: all[0].CreatedAt, Rank: TimelineRankComment, ID: all[0].ID}
	after, err := comments.ListTimeline(ctx, record.ID, cursor, true, 10)
	require.NoError(t, err)
	assert.Empty(t, after)
	before, err := NewSLRecordChangesStore(tx).ListTimeline(ctx, record.ID, &TimelineCursor{Time: all[0].CreatedAt.Add(time.Second), Rank: TimelineRankComment}, false, 10)
	require.NoError(t, err)
	assert.Len(t, before, 1, "the created change is before the cursor")
}
