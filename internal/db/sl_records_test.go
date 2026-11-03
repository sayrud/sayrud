package db

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// memberFilterFixture is shared with the frontend engine tests, so both implementations of the filters are checked against the same cases.
type memberFilterFixture struct {
	CurrentUserID int64 `json:"currentUserId"`
	Fields        []struct {
		UID      string                 `json:"uid"`
		Type     SLFieldType            `json:"type"`
		Metadata map[string]interface{} `json:"metadata"`
	} `json:"fields"`
	Records []struct {
		UID       string          `json:"uid"`
		Data      json.RawMessage `json:"data"`
		CreatedBy SLRecordWriter  `json:"createdBy"`
		UpdatedBy SLRecordWriter  `json:"updatedBy"`
	} `json:"records"`
	Cases []struct {
		Name          string `json:"name"`
		CurrentUserID *int64 `json:"currentUserId"`
		MatchAny      bool   `json:"matchAny"`
		Filter        []struct {
			FieldUID  string `json:"fieldUID"`
			Operation string `json:"operation"`
			Value     string `json:"value"`
		} `json:"filter"`
		Sort   []QuerySLRecordsSort `json:"sort"`
		Expect []string             `json:"expect"`
	} `json:"cases"`
}

func TestQueryMemberAndWriterFields(t *testing.T) {
	ctx := context.Background()
	gormDB := newTestDB(t, &SLField{}, &SLRecord{})
	raw, err := os.ReadFile("testdata/member_filters.json")
	require.NoError(t, err)
	var fixture memberFilterFixture
	require.NoError(t, json.Unmarshal(raw, &fixture))

	const tableID = 1
	for _, f := range fixture.Fields {
		_, err := NewSLFieldsStore(gormDB).Create(ctx, CreateSLFieldOptions{SLTableID: tableID, UID: f.UID, Label: f.UID, Type: f.Type, Metadata: f.Metadata})
		require.NoError(t, err)
	}
	records := NewSLRecordsStore(gormDB)
	for _, r := range fixture.Records {
		created, err := records.Import(ctx, tableID, ImportSLRecordsOptions{Data: []json.RawMessage{r.Data}, UIDs: []string{r.UID}, Writer: r.CreatedBy})
		require.NoError(t, err)
		if r.UpdatedBy != r.CreatedBy {
			require.NoError(t, records.Update(ctx, created[0].ID, r.Data, r.UpdatedBy))
		}
	}

	for _, c := range fixture.Cases {
		t.Run(c.Name, func(t *testing.T) {
			options := QuerySLRecordsOptions{CurrentUserID: fixture.CurrentUserID, MatchAny: c.MatchAny, Order: c.Sort, Limit: 100}
			if c.CurrentUserID != nil {
				options.CurrentUserID = *c.CurrentUserID
			}
			for _, f := range c.Filter {
				options.Filter = append(options.Filter, QuerySLRecordsFilter{FieldUID: f.FieldUID, Operation: FilterOperation(f.Operation), Value: f.Value})
			}
			got, total, err := records.Query(ctx, tableID, options)
			require.NoError(t, err)
			uids := lo.Map(got, func(r *SLRecord, _ int) string { return r.UID })
			if len(c.Sort) == 0 {
				assert.ElementsMatch(t, c.Expect, uids)
			} else {
				assert.Equal(t, c.Expect, uids)
			}
			assert.Equal(t, int64(len(c.Expect)), total)
		})
	}

	t.Run("invalid user IDs are rejected", func(t *testing.T) {
		_, _, err := records.Query(ctx, tableID, QuerySLRecordsOptions{Filter: []QuerySLRecordsFilter{{FieldUID: "fldTeam0001", Operation: FilterOperationIn, Value: `["ann"]`}}})
		assert.ErrorIs(t, err, ErrInvalidFilterValue)
	})
	t.Run("the trigger conditions do not match the system fields", func(t *testing.T) {
		_, err := records.MatchData(ctx, tableID, []QuerySLRecordsFilter{{FieldUID: "fldCreator1", Operation: FilterOperationEmpty}}, false, map[string]json.RawMessage{"a": json.RawMessage(`{}`)})
		assert.ErrorIs(t, err, ErrSLFieldNotQueryable)
	})
	t.Run("the trigger conditions match the members", func(t *testing.T) {
		keys, err := records.MatchData(ctx, tableID, []QuerySLRecordsFilter{{FieldUID: "fldTeam0001", Operation: FilterOperationIn, Value: `["2"]`}}, false, map[string]json.RawMessage{
			"a": json.RawMessage(`{"fldTeam0001":[1,2]}`), "b": json.RawMessage(`{"fldTeam0001":[3]}`), "c": json.RawMessage(`{}`),
		})
		require.NoError(t, err)
		assert.Equal(t, []string{"a"}, keys)
	})
}

func TestRecordWriters(t *testing.T) {
	ctx := context.Background()
	gormDB := newTestDB(t, &SLField{}, &SLRecord{})
	records := NewSLRecordsStore(gormDB)

	record, err := records.Create(ctx, 1, json.RawMessage(`{}`), SLRecordWriter{UserID: 3})
	require.NoError(t, err)
	assert.Equal(t, SLRecordWriter{UserID: 3}, record.CreatedBy())
	assert.Equal(t, SLRecordWriter{UserID: 3}, record.UpdatedBy())

	require.NoError(t, records.Update(ctx, record.ID, json.RawMessage(`{"a":1}`), SLRecordWriter{WorkflowID: 9}))
	stored, err := records.GetByID(ctx, record.ID)
	require.NoError(t, err)
	assert.Equal(t, SLRecordWriter{UserID: 3}, stored.CreatedBy())
	assert.Equal(t, SLRecordWriter{WorkflowID: 9}, stored.UpdatedBy())

	require.NoError(t, records.Update(ctx, record.ID, json.RawMessage(`{"a":2}`), SLRecordWriter{}))
	stored, err = records.GetByID(ctx, record.ID)
	require.NoError(t, err)
	assert.Equal(t, SLRecordWriter{WorkflowID: 9}, stored.UpdatedBy(), "the writes of the server keep the modifier")
	assert.JSONEq(t, `{"a":2}`, string(stored.Data))
}

func TestQuerySystemTimes(t *testing.T) {
	ctx := context.Background()
	gormDB := newTestDB(t, &SLField{}, &SLRecord{})
	_, err := NewSLFieldsStore(gormDB).Create(ctx, CreateSLFieldOptions{SLTableID: 1, UID: "fldCreated1", Label: "Created", Type: CreatedAtFieldType, Metadata: map[string]interface{}{}})
	require.NoError(t, err)
	records := NewSLRecordsStore(gormDB)
	old, err := records.Create(ctx, 1, json.RawMessage(`{}`), SLRecordWriter{})
	require.NoError(t, err)
	require.NoError(t, gormDB.Model(&SLRecord{}).Where("id = ?", old.ID).UpdateColumn("created_at", time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)).Error)
	recent, err := records.Create(ctx, 1, json.RawMessage(`{}`), SLRecordWriter{})
	require.NoError(t, err)

	got, _, err := records.Query(ctx, 1, QuerySLRecordsOptions{Filter: []QuerySLRecordsFilter{{FieldUID: "fldCreated1", Operation: FilterOperationGreaterThan, Value: "2021-01-01T00:00:00Z"}}})
	require.NoError(t, err)
	assert.Equal(t, []string{recent.UID}, lo.Map(got, func(r *SLRecord, _ int) string { return r.UID }))

	got, _, err = records.Query(ctx, 1, QuerySLRecordsOptions{Order: []QuerySLRecordsSort{{FieldUID: "fldCreated1", Order: "asc"}}})
	require.NoError(t, err)
	assert.Equal(t, []string{old.UID, recent.UID}, lo.Map(got, func(r *SLRecord, _ int) string { return r.UID }))

	got, _, err = records.Query(ctx, 1, QuerySLRecordsOptions{Filter: []QuerySLRecordsFilter{{FieldUID: "fldCreated1", Operation: FilterOperationEmpty}}})
	require.NoError(t, err)
	assert.Empty(t, got, "the creation time is never empty")
}
