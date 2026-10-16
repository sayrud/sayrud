package importer

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
	"golang.org/x/text/encoding/simplifiedchinese"

	"github.com/wuhan005/sayrud/internal/db"
)

func cells(texts ...string) []*Cell {
	out := make([]*Cell, len(texts))
	for i, t := range texts {
		out[i] = textCell(t)
	}
	return out
}

func numberCells(texts ...string) []*Cell {
	out := make([]*Cell, len(texts))
	for i, t := range texts {
		out[i] = &Cell{
			Text: t,
			Kind: KindNumber,
		}
	}
	return out
}

func TestReadDelimited(t *testing.T) {
	t.Run("detects the delimiter and drops blank rows", func(t *testing.T) {
		sheets, err := Read("data.csv", []byte("name;score;\n\nAlice;\"1,5\";\n ; ;\nBob;2;\n"))
		require.NoError(t, err)
		require.Len(t, sheets, 1)
		s := sheets[0]
		assert.Equal(t, "data", s.Name)
		assert.Equal(t, 2, s.Width)
		require.Len(t, s.Rows, 3)
		assert.Equal(t, "1,5", s.Rows[1][1].Text)
	})

	t.Run("decodes GB18030 and UTF-16", func(t *testing.T) {
		gbk, err := simplifiedchinese.GB18030.NewEncoder().Bytes([]byte("姓名,城市\n张三,北京\n"))
		require.NoError(t, err)
		sheets, err := Read("gbk.csv", gbk)
		require.NoError(t, err)
		assert.Equal(t, "北京", sheets[0].Rows[1][1].Text)

		utf16 := []byte{0xff, 0xfe}
		for _, r := range "a\tb\n1\t2\n" {
			utf16 = append(utf16, byte(r), 0)
		}
		sheets, err = Read("export.txt", utf16)
		require.NoError(t, err)
		assert.Equal(t, "b", sheets[0].Rows[0][1].Text)
	})

	t.Run("rejects unsupported and empty files", func(t *testing.T) {
		_, err := Read("old.xls", nil)
		assert.ErrorIs(t, err, ErrLegacyExcel)
		_, err = Read("doc.pdf", nil)
		assert.ErrorIs(t, err, ErrUnsupportedFormat)
		_, err = Read("empty.csv", []byte("\n , \n"))
		assert.ErrorIs(t, err, ErrNoData)
	})
}

func TestReadWorkbook(t *testing.T) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	sheet := "Customers"
	require.NoError(t, f.SetSheetName("Sheet1", sheet))
	require.NoError(t, f.SetSheetRow(sheet, "A1", &[]interface{}{"Name", "Signed", "Rate", "VIP", "Code", "Phone", "Login"}))
	require.NoError(t, f.SetSheetRow(sheet, "A2", &[]interface{}{"Alice", time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC), 0.15, true, "00123", 13800000000, time.Date(2025, 1, 1, 9, 30, 0, 0, time.UTC)}))
	dateStyle, err := f.NewStyle(&excelize.Style{NumFmt: 14})
	require.NoError(t, err)
	customDate := "yyyy-mm-dd hh:mm"
	dateTimeStyle, err := f.NewStyle(&excelize.Style{CustomNumFmt: &customDate})
	require.NoError(t, err)
	percentStyle, err := f.NewStyle(&excelize.Style{NumFmt: 9})
	require.NoError(t, err)
	require.NoError(t, f.SetCellStyle(sheet, "B2", "B2", dateStyle))
	require.NoError(t, f.SetCellStyle(sheet, "G2", "G2", dateTimeStyle))
	require.NoError(t, f.SetCellStyle(sheet, "C2", "C2", percentStyle))

	_, err = f.NewSheet("Hidden")
	require.NoError(t, err)
	require.NoError(t, f.SetCellValue("Hidden", "A1", "secret"))
	require.NoError(t, f.SetSheetVisible("Hidden", false))

	var buf bytes.Buffer
	require.NoError(t, f.Write(&buf))

	sheets, err := Read("book.xlsx", buf.Bytes())
	require.NoError(t, err)
	require.Len(t, sheets, 1)
	row := sheets[0].Rows[1]

	assert.Equal(t, KindDate, row[1].Kind)
	assert.Equal(t, "2024-01-15", row[1].Text)
	assert.False(t, row[1].WithTime)
	assert.Equal(t, KindNumber, row[2].Kind)
	assert.Equal(t, "15%", row[2].Text)
	assert.InDelta(t, 0.15, row[2].Number, 1e-9)
	assert.Equal(t, KindBool, row[3].Kind)
	assert.True(t, row[3].Bool)
	assert.Equal(t, KindText, row[4].Kind)
	assert.Equal(t, "00123", row[4].Text)
	assert.Equal(t, "13800000000", row[5].Text)
	assert.Equal(t, KindDate, row[6].Kind)
	assert.True(t, row[6].WithTime)
	assert.Equal(t, "2025-01-01 09:30", row[6].Text)
}

func TestDetectType(t *testing.T) {
	for _, tc := range []struct {
		cells []*Cell
		want  db.SLFieldType
	}{
		{cells("1", "2.5", "-3", "1,234.00", "15%"), db.NumberFieldType},
		{cells("001", "002"), db.TextFieldType},
		{cells("1234567890123456789"), db.TextFieldType},
		{cells("13800000000", "13900000001"), db.TextFieldType},
		{numberCells("13800000000", "12"), db.NumberFieldType},
		{cells("2024-01-15", "2024/2/1 10:30", "2024年3月1日"), db.DateTimeFieldType},
		{cells("yes", "no", "TRUE", "是", "否"), db.CheckboxFieldType},
		{cells("Open", "Closed", "Open", "Open", "Closed", "Open"), db.SingleSelectFieldType},
		{cells("Note", "Note", "Note", "Note", "Note"), db.TextFieldType},
		{cells("red, blue", "blue", "green、red", "red", "blue, green"), db.MultiSelectFieldType},
		{cells("a", "b", "c", "d", "e"), db.TextFieldType},
		{[]*Cell{nil, nil}, db.TextFieldType},
	} {
		assert.Equal(t, tc.want, DetectType(tc.cells), "%v", tc.cells)
	}
}

func TestNumberAndDateFormats(t *testing.T) {
	assert.Equal(t, "0", NumberFormat(cells("1", "2")))
	assert.Equal(t, "0.00", NumberFormat(cells("1.5", "2.25")))
	assert.Equal(t, "0,000", NumberFormat(cells("1,234", "5")))
	assert.Equal(t, "0,000.00", NumberFormat(cells("1,234.5")))
	assert.Equal(t, "0.00%", NumberFormat(cells("15%", "7.5%", "0.5")))
	assert.Equal(t, "¥0,000.00", NumberFormat(cells("¥1,000.00")))

	format, withTime := DateFormat(cells("2024-01-15", "2024-02-01 10:30"))
	assert.Equal(t, "YYYY-MM-DD", format)
	assert.True(t, withTime)
	format, withTime = DateFormat([]*Cell{{
		Kind: KindDate,
		Text: "2024-01-15",
	}})
	assert.Equal(t, "YYYY/MM/DD", format)
	assert.False(t, withTime)
}
