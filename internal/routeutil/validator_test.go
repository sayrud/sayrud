package routeutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"

	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dbutil"
)

func TestMemberValues(t *testing.T) {
	single := &db.SLField{UID: "fldOwner001", Type: db.MemberFieldType, Metadata: datatypes.NewJSONType[db.SLFieldMetadata](map[string]interface{}{"multiple": false})}
	multiple := &db.SLField{UID: "fldTeam0001", Type: db.MemberFieldType, Metadata: datatypes.NewJSONType[db.SLFieldMetadata](map[string]interface{}{"multiple": true})}

	assert.True(t, CheckValue(single, []interface{}{12.0}))
	assert.True(t, CheckValue(multiple, []interface{}{12.0, 34.0}))
	assert.False(t, CheckValue(single, []interface{}{12.0, 34.0}), "a single member field holds one member")
	for _, invalid := range []interface{}{12.0, "12", []interface{}{"12"}, []interface{}{1.5}, []interface{}{0.0}, []interface{}{12.0, 12.0}, []interface{}{float64(1 << 54)}} {
		assert.False(t, CheckValue(multiple, invalid), "%v", invalid)
	}
	assert.Equal(t, []int64{12, 34}, MemberIDs([]interface{}{12.0, "x", 34.0}))
}

func TestSystemFieldsAreReadOnly(t *testing.T) {
	text := &db.SLField{UID: "fldText0001", Label: "Name", Type: db.TextFieldType}
	creator := &db.SLField{UID: "fldCreator1", Label: "Creator", Type: db.CreatedByFieldType}
	created := &db.SLField{UID: "fldCreated1", Label: "Created", Type: db.CreatedAtFieldType}
	validator := NewRecordValidator([]*db.SLField{text, creator, created})

	result, err := validator.Validate(map[string]interface{}{text.UID: "Ann", creator.UID: nil, created.UID: ""})
	require.NoError(t, err)
	assert.Equal(t, map[string]interface{}{text.UID: "Ann"}, result, "empty system values are ignored")

	_, err = validator.Validate(map[string]interface{}{created.UID: "2026-11-03T00:00:00Z"})
	assert.ErrorIs(t, err, ErrFieldReadOnly)
	assert.Equal(t, creator, validator.ReadOnlyField(map[string]interface{}{creator.UID: map[string]interface{}{"userId": 1.0}}))
	assert.Nil(t, validator.ReadOnlyField(map[string]interface{}{text.UID: "Ann", creator.UID: nil}))
	assert.False(t, IsStoredValue(created, "2026-11-03T00:00:00Z"))
}

func TestDirectory(t *testing.T) {
	users := []*db.User{
		{Model: dbutil.Model{ID: 1}, UserName: "Ann", Email: "ann@example.com"},
		{Model: dbutil.Model{ID: 2}, UserName: "Ann", Email: "other@example.com"},
		{Model: dbutil.Model{ID: 3}, UserName: "Left", Email: "left@example.com"},
	}
	d := NewDirectory(users, []int64{1, 2}, map[int64]string{7: "Nightly sync"})

	id, ok := d.Match("OTHER@example.com")
	assert.True(t, ok)
	assert.Equal(t, int64(2), id)
	id, _ = d.Match("Ann")
	assert.Equal(t, int64(1), id, "names match the first member")
	_, ok = d.Match("Left")
	assert.False(t, ok, "only the members are matched")

	for _, v := range []interface{}{1.0, "1", "ann@example.com"} {
		id, ok := d.ResolveMember(v)
		assert.True(t, ok, "%v", v)
		assert.Equal(t, int64(1), id)
	}
	_, ok = d.ResolveMember(3.0)
	assert.False(t, ok, "the users who left can not be written")

	assert.Equal(t, "Ann, Left", d.MemberNames([]interface{}{1.0, 3.0, 99.0}))
	assert.Equal(t, "Nightly sync", d.WriterName(db.SLRecordWriter{WorkflowID: 7}))
	assert.Equal(t, "", d.WriterName(db.SLRecordWriter{}))
	var none *Directory
	assert.Equal(t, "", none.MemberNames([]interface{}{1.0}))
	assert.Nil(t, none.ParseMembers("Ann", true))
}
