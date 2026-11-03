package collab

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dbutil"
	"github.com/wuhan005/sayrud/internal/importer"
	"github.com/wuhan005/sayrud/internal/routeutil"
)

func testDirectory() *routeutil.Directory {
	users := []*db.User{
		{Model: dbutil.Model{ID: 1}, UserName: "Ann", Email: "ann@example.com"},
		{Model: dbutil.Model{ID: 2}, UserName: "Bob", Email: "bob@example.com"},
		{Model: dbutil.Model{ID: 3}, UserName: "Left", Email: "left@example.com"},
	}
	return routeutil.NewDirectory(users, []int64{1, 2}, map[int64]string{7: "Nightly sync"})
}

func TestConvertField_Members(t *testing.T) {
	directory := testDirectory()
	member := newTestField("fldMember1", db.MemberFieldType, map[string]interface{}{"multiple": true})
	records := []*db.SLRecord{
		newTestRecord(t, "rec1", map[string]interface{}{member.UID: []interface{}{1.0, 3.0}}),
		newTestRecord(t, "rec2", map[string]interface{}{}),
	}

	conversion, err := convertField(member, db.TextFieldType, map[string]interface{}{}, records, testBoolText, directory)
	require.NoError(t, err)
	assert.Equal(t, "Ann, Left", conversion.values["rec1"], "the members who left keep their names")
	assert.NotContains(t, conversion.values, "rec2")

	conversion, err = convertField(member, db.MemberFieldType, map[string]interface{}{"multiple": false}, records, testBoolText, directory)
	require.NoError(t, err)
	assert.Equal(t, []interface{}{1.0}, conversion.values["rec1"], "a single member field keeps the first member")

	text := newTestField("fldText001", db.TextFieldType, map[string]interface{}{})
	records = []*db.SLRecord{
		newTestRecord(t, "rec1", map[string]interface{}{text.UID: "BOB@example.com, Ann, Nobody"}),
		newTestRecord(t, "rec2", map[string]interface{}{text.UID: "Left"}),
	}
	conversion, err = convertField(text, db.MemberFieldType, map[string]interface{}{"multiple": true}, records, testBoolText, directory)
	require.NoError(t, err)
	assert.Equal(t, []interface{}{2.0, 1.0}, conversion.values["rec1"], "emails are matched case-insensitively before names")
	assert.Nil(t, conversion.values["rec2"], "the users who left are not matched")
}

func TestConvertField_SystemFields(t *testing.T) {
	directory := testDirectory()
	created := time.Date(2026, 11, 3, 8, 30, 0, 0, time.UTC)
	record := newTestRecord(t, "rec1", map[string]interface{}{})
	record.CreatedAt, record.UpdatedAt = created, created
	record.CreatedByID, record.UpdatedByWorkflowID = 2, 7
	records := []*db.SLRecord{record}

	creator := newTestField("fldCreator1", db.CreatedByFieldType, map[string]interface{}{})
	conversion, err := convertField(creator, db.TextFieldType, map[string]interface{}{}, records, testBoolText, directory)
	require.NoError(t, err)
	assert.Equal(t, "Bob", conversion.values["rec1"])
	conversion, err = convertField(creator, db.MemberFieldType, map[string]interface{}{"multiple": false}, records, testBoolText, directory)
	require.NoError(t, err)
	assert.Equal(t, []interface{}{2.0}, conversion.values["rec1"])

	editor := newTestField("fldEditor01", db.UpdatedByFieldType, map[string]interface{}{})
	conversion, err = convertField(editor, db.TextFieldType, map[string]interface{}{}, records, testBoolText, directory)
	require.NoError(t, err)
	assert.Equal(t, "Nightly sync", conversion.values["rec1"])

	createdAt := newTestField("fldCreated1", db.CreatedAtFieldType, map[string]interface{}{"format": "YYYY/MM/DD", "with_time": false})
	conversion, err = convertField(createdAt, db.DateTimeFieldType, map[string]interface{}{"format": "YYYY/MM/DD"}, records, testBoolText, directory)
	require.NoError(t, err)
	assert.Equal(t, "2026-11-03T08:30:00.000Z", conversion.values["rec1"])

	text := newTestField("fldText001", db.TextFieldType, map[string]interface{}{})
	stored := newTestRecord(t, "rec1", map[string]interface{}{text.UID: "kept"})
	conversion, err = convertField(text, db.CreatedByFieldType, map[string]interface{}{}, []*db.SLRecord{stored}, testBoolText, directory)
	require.NoError(t, err)
	assert.Contains(t, conversion.values, "rec1")
	assert.Nil(t, conversion.values["rec1"], "the system fields store no values")
}

func TestImportMembers(t *testing.T) {
	directory := testDirectory()
	single := newTestField("fldOwner001", db.MemberFieldType, map[string]interface{}{"multiple": false})
	multiple := newTestField("fldTeam0001", db.MemberFieldType, map[string]interface{}{"multiple": true})

	assert.Equal(t, []interface{}{1.0}, importValue(single, &importer.Cell{Text: "ann@example.com、Bob"}, time.UTC, directory))
	assert.Equal(t, []interface{}{1.0, 2.0}, importValue(multiple, &importer.Cell{Text: "ann@example.com、Bob"}, time.UTC, directory))
	assert.Nil(t, importValue(multiple, &importer.Cell{Text: "Nobody"}, time.UTC, directory), "unmatched members are left empty")

	cells := []*importer.Cell{{Text: "Ann"}, nil, {Text: "Ann, Bob"}}
	assert.Equal(t, map[string]interface{}{"multiple": true}, importMetadata(db.MemberFieldType, cells, directory))
	assert.Equal(t, map[string]interface{}{"multiple": false}, importMetadata(db.MemberFieldType, cells[:2], directory))
}

func TestChangeWritten(t *testing.T) {
	var change Change
	change.AddWritten("rec1", false, db.SLRecordWriter{UserID: 1})
	change.AddWritten("rec1", true, db.SLRecordWriter{WorkflowID: 2})
	change.AddWritten("rec2", false, db.SLRecordWriter{UserID: 1})
	change.AddWritten("rec3", true, db.SLRecordWriter{})
	assert.Equal(t, map[string]bool{"rec1": true, "rec2": false}, change.Written, "the writes of the server are not recorded")
}
