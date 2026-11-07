package workflow

import (
	"encoding/json"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"

	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dbutil"
	"github.com/wuhan005/sayrud/internal/routeutil"
)

// memberFormulaFixture is shared with the frontend engine tests, so both implementations of the formulas read the members alike.
type memberFormulaFixture struct {
	Users []struct {
		ID       int64  `json:"id"`
		UserName string `json:"userName"`
		Email    string `json:"email"`
		IsBot    bool   `json:"isBot"`
	} `json:"users"`
	MemberIDs []int64           `json:"memberIds"`
	Workflows map[string]string `json:"workflows"`
	Fields    []struct {
		UID      string                 `json:"uid"`
		Type     db.SLFieldType         `json:"type"`
		Metadata map[string]interface{} `json:"metadata"`
	} `json:"fields"`
	Record struct {
		Data      json.RawMessage   `json:"data"`
		CreatedAt time.Time         `json:"createdAt"`
		CreatedBy db.SLRecordWriter `json:"createdBy"`
		UpdatedBy db.SLRecordWriter `json:"updatedBy"`
	} `json:"record"`
	Cases []struct {
		Exp    string      `json:"exp"`
		Expect interface{} `json:"expect"`
	} `json:"cases"`
}

func loadMemberFormulaFixture(t *testing.T) (*memberFormulaFixture, *routeutil.Directory) {
	raw, err := os.ReadFile("../db/testdata/member_formulas.json")
	require.NoError(t, err)
	var fixture memberFormulaFixture
	require.NoError(t, json.Unmarshal(raw, &fixture))
	users := lo.Map(fixture.Users, func(u struct {
		ID       int64  `json:"id"`
		UserName string `json:"userName"`
		Email    string `json:"email"`
		IsBot    bool   `json:"isBot"`
	}, _ int) *db.User {
		return &db.User{Model: dbutil.Model{ID: u.ID}, UserName: u.UserName, Email: u.Email, Kind: lo.Ternary(u.IsBot, db.UserKindBot, db.UserKindHuman)}
	})
	workflows := map[int64]string{}
	for id, name := range fixture.Workflows {
		n, _ := strconv.ParseInt(id, 10, 64)
		workflows[n] = name
	}
	return &fixture, routeutil.NewDirectory(users, fixture.MemberIDs, workflows)
}

func TestMemberFormulas(t *testing.T) {
	fixture, directory := loadMemberFormulaFixture(t)
	fields := make([]*db.SLField, 0, len(fixture.Fields)+len(fixture.Cases))
	for _, f := range fixture.Fields {
		fields = append(fields, &db.SLField{UID: f.UID, Type: f.Type, Metadata: datatypes.NewJSONType[db.SLFieldMetadata](f.Metadata)})
	}
	for i, c := range fixture.Cases {
		fields = append(fields, &db.SLField{UID: "fldFormula" + strconv.Itoa(i), Type: db.FormulaFieldType, Metadata: datatypes.NewJSONType[db.SLFieldMetadata](map[string]interface{}{"exp": c.Exp})})
	}
	info := newTableInfo(&db.SLTable{}, fields)
	info.directory = directory
	record := &db.SLRecord{UID: "rec1", Data: datatypes.JSON(fixture.Record.Data)}
	record.CreatedAt, record.UpdatedAt = fixture.Record.CreatedAt, fixture.Record.CreatedAt
	record.CreatedByID, record.CreatedByWorkflowID = fixture.Record.CreatedBy.UserID, fixture.Record.CreatedBy.WorkflowID
	record.UpdatedByID, record.UpdatedByWorkflowID = fixture.Record.UpdatedBy.UserID, fixture.Record.UpdatedBy.WorkflowID

	out := recordOutput(info, record, time.UTC)["fields"].(map[string]interface{})
	for i, c := range fixture.Cases {
		assert.Equal(t, c.Expect, out["fldFormula"+strconv.Itoa(i)], c.Exp)
	}

	assert.Equal(t, []interface{}{
		map[string]interface{}{"id": 1.0, "name": "Ann", "email": "ann@example.com", "isBot": false},
		map[string]interface{}{"id": 2.0, "name": "Bob", "email": "bob@example.com", "isBot": false},
		map[string]interface{}{"id": 3.0, "name": "Builder", "email": "bot-builder@bots.invalid", "isBot": true},
		map[string]interface{}{"id": 99.0, "name": "", "email": "", "isBot": false},
	}, out["fldTeam0001"])
	assert.Equal(t, map[string]interface{}{"id": 2.0, "name": "Bob", "email": "bob@example.com", "isBot": false, "workflowId": 0.0}, out["fldCreator1"])
	assert.Equal(t, map[string]interface{}{"id": 0.0, "name": "Nightly sync", "email": "", "isBot": false, "workflowId": 7.0}, out["fldEditor01"])
	assert.Equal(t, "2026-11-03T08:30:00Z", out["fldCreated1"])
}

func TestMemberCellValues(t *testing.T) {
	_, directory := loadMemberFormulaFixture(t)
	single := &db.SLField{UID: "fldOwner001", Label: "Owner", Type: db.MemberFieldType, Metadata: datatypes.NewJSONType[db.SLFieldMetadata](map[string]interface{}{"multiple": false})}
	multiple := &db.SLField{UID: "fldTeam0001", Label: "Team", Type: db.MemberFieldType, Metadata: datatypes.NewJSONType[db.SLFieldMetadata](map[string]interface{}{"multiple": true})}

	for _, v := range []interface{}{1.0, "1", "ANN@example.com", map[string]interface{}{"id": 1.0, "name": "Ann"}} {
		got, err := toCellValue(single, v, directory)
		require.NoError(t, err, "%v", v)
		assert.Equal(t, []interface{}{1.0}, got)
	}
	got, err := toCellValue(multiple, []interface{}{"bot-builder@bots.invalid", 1.0, "1"}, directory)
	require.NoError(t, err)
	assert.Equal(t, []interface{}{3.0, 1.0}, got)

	_, err = toCellValue(multiple, []interface{}{"bob@example.com"}, directory)
	var fieldErr *FieldValueError
	require.ErrorAs(t, err, &fieldErr, "the users who left can not be written")
	_, err = toCellValue(single, []interface{}{1.0, 3.0}, directory)
	require.ErrorAs(t, err, &fieldErr)
	got, err = toCellValue(single, "", directory)
	require.NoError(t, err)
	assert.Nil(t, got)

	assert.False(t, WritableField(&db.SLField{Type: db.UpdatedByFieldType}))
	value, err := filterValue(multiple, "in", []interface{}{"@me", "ann@example.com", 2.0}, directory)
	require.NoError(t, err)
	assert.Equal(t, `["@me","1","2"]`, value, "the users who left can still be filtered by ID")
}
