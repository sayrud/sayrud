// Package importer reads the rows of Excel and CSV files and infers the field types of their columns.
package importer

import (
	"bytes"
	"encoding/csv"
	"io"
	"path"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cockroachdb/errors"
	"github.com/xuri/excelize/v2"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/unicode"
)

const (
	// MaxFileSize is the maximum number of bytes of an imported file.
	MaxFileSize = 20 << 20
	// MaxRows is the maximum number of data rows of a sheet, excluding the header row.
	MaxRows = 50000
	// MaxColumns is the maximum number of columns of a sheet.
	MaxColumns = 200
)

var (
	// ErrUnsupportedFormat is returned for the files whose extension is not supported.
	ErrUnsupportedFormat = errors.New("unsupported file format")
	// ErrLegacyExcel is returned for the binary .xls workbooks, which should be saved as .xlsx first.
	ErrLegacyExcel = errors.New("legacy excel format")
	// ErrNoData is returned if all the visible sheets are blank.
	ErrNoData = errors.New("no data")
	// ErrTooManyRows is returned if a sheet has more than MaxRows data rows.
	ErrTooManyRows = errors.New("too many rows")
	// ErrTooManyColumns is returned if a sheet has more than MaxColumns columns.
	ErrTooManyColumns = errors.New("too many columns")
)

// Kind is the type of a cell value read from the file.
type Kind uint8

const (
	// KindText is a text cell, including all the cells of the CSV files.
	KindText Kind = iota
	// KindNumber is a number cell, the value is in Number.
	KindNumber
	// KindBool is a boolean cell, the value is in Bool.
	KindBool
	// KindDate is a number cell with a date format, the value is in Time.
	KindDate
)

// Cell is a non-empty cell: the text as displayed in the file, and the typed value of the spreadsheet cells.
type Cell struct {
	// Text is the trimmed text as displayed, e.g. formatted by the number format of the spreadsheet cell.
	Text string
	// Kind is the type of the value.
	Kind Kind
	// Number is the value of a number cell.
	Number float64
	// Bool is the value of a boolean cell.
	Bool bool
	// Time is the wall clock of a date cell stored in UTC, it is interpreted in the time zone of the user when imported.
	Time time.Time
	// WithTime reports whether the date cell has a time of day.
	WithTime bool
}

// Sheet is a sheet of the file.
type Sheet struct {
	// Name is the sheet name, or the file name without the extension for the CSV files.
	Name string
	// Rows are the non-blank rows, a nil cell is empty.
	Rows [][]*Cell
	// Width is the number of columns, excluding the trailing empty ones.
	Width int
}

var (
	// textExtensions are read as delimiter-separated values.
	textExtensions = []string{".csv", ".tsv", ".txt"}
	// workbookExtensions are read as Office Open XML workbooks.
	workbookExtensions = []string{".xlsx", ".xlsm"}
)

// extension returns the lowercase extension of the file name, including the dot.
func extension(name string) string {
	return strings.ToLower(path.Ext(name))
}

// Supported reports whether the file can be imported by its extension.
func Supported(name string) bool {
	ext := extension(name)
	for _, e := range append(textExtensions, workbookExtensions...) {
		if e == ext {
			return true
		}
	}
	return false
}

// Read reads the visible sheets of the file by its extension, dropping the blank rows.
func Read(name string, data []byte) ([]*Sheet, error) {
	ext := extension(name)
	var sheets []*Sheet
	var err error
	switch ext {
	case ".csv", ".txt":
		sheets, err = readDelimited(strings.TrimSuffix(path.Base(name), path.Ext(name)), data, 0)
	case ".tsv":
		sheets, err = readDelimited(strings.TrimSuffix(path.Base(name), path.Ext(name)), data, '\t')
	case ".xlsx", ".xlsm":
		sheets, err = readWorkbook(data)
	case ".xls":
		return nil, ErrLegacyExcel
	default:
		return nil, ErrUnsupportedFormat
	}
	if err != nil {
		return nil, err
	}

	result := make([]*Sheet, 0, len(sheets))
	for _, s := range sheets {
		if len(s.Rows) == 0 {
			continue
		}
		if len(s.Rows) > MaxRows+1 {
			return nil, ErrTooManyRows
		}
		if s.Width > MaxColumns {
			return nil, ErrTooManyColumns
		}
		result = append(result, s)
	}
	if len(result) == 0 {
		return nil, ErrNoData
	}
	return result, nil
}

// newSheet builds the sheet from the rows, dropping the blank rows and computing the width.
func newSheet(name string, rows [][]*Cell) *Sheet {
	s := &Sheet{Name: name}
	for _, row := range rows {
		last := -1
		for i, c := range row {
			if c != nil {
				last = i
			}
		}
		if last < 0 {
			continue
		}
		s.Rows = append(s.Rows, row[:last+1])
		s.Width = max(s.Width, last+1)
	}
	return s
}

// textCell returns the text cell of the trimmed text, or nil if it is blank.
func textCell(text string) *Cell {
	t := strings.TrimSpace(text)
	if t == "" {
		return nil
	}
	return &Cell{Text: t}
}

// DecodeText decodes a text file: UTF-16 with a BOM, UTF-8 (BOM stripped), or GB18030 as saved by Excel on Chinese systems.
func DecodeText(data []byte) string {
	switch {
	case bytes.HasPrefix(data, []byte{0xff, 0xfe}), bytes.HasPrefix(data, []byte{0xfe, 0xff}):
		if decoded, err := unicode.UTF16(unicode.LittleEndian, unicode.ExpectBOM).NewDecoder().Bytes(data); err == nil {
			return string(decoded)
		}
	case bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}):
		return string(data[3:])
	}
	if utf8.Valid(data) {
		return string(data)
	}
	if decoded, err := simplifiedchinese.GB18030.NewDecoder().Bytes(data); err == nil {
		return string(decoded)
	}
	return string(data)
}

// newCSVReader returns a lenient reader accepting bare quotes and rows of different lengths.
func newCSVReader(text string, delimiter rune) *csv.Reader {
	r := csv.NewReader(strings.NewReader(text))
	r.Comma = delimiter
	r.LazyQuotes = true
	r.FieldsPerRecord = -1
	return r
}

// DetectDelimiter picks the delimiter splitting the leading lines into the most consistent number of columns, comma by default.
func DetectDelimiter(text string) rune {
	head := text
	if len(head) > 64<<10 {
		head = head[:64<<10]
	}
	if lines := strings.SplitN(head, "\n", 21); len(lines) > 20 {
		head = strings.Join(lines[:20], "\n")
	}

	best, bestScore := ',', 0
	for _, d := range []rune{',', '\t', ';', '|'} {
		r := newCSVReader(head, d)
		width, consistent := 0, 0
		for {
			record, err := r.Read()
			if err != nil {
				break
			}
			if width == 0 {
				width = len(record)
			}
			if len(record) == width {
				consistent++
			}
		}
		if width < 2 {
			continue
		}
		if score := consistent*1000 + width; score > bestScore {
			best, bestScore = d, score
		}
	}
	return best
}

// readDelimited reads a CSV / TSV file as a sheet, the delimiter is detected if it is zero.
func readDelimited(name string, data []byte, delimiter rune) ([]*Sheet, error) {
	text := DecodeText(data)
	if delimiter == 0 {
		delimiter = DetectDelimiter(text)
	}
	r := newCSVReader(text, delimiter)
	var rows [][]*Cell
	for {
		record, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, errors.Wrap(err, "read csv")
		}
		row := make([]*Cell, len(record))
		for i, v := range record {
			row[i] = textCell(v)
		}
		rows = append(rows, row)
		if len(rows) > MaxRows+1 {
			return nil, ErrTooManyRows
		}
	}
	return []*Sheet{newSheet(name, rows)}, nil
}

// builtinDateFormats are the IDs of the built-in number formats of dates and times, including the CJK ones.
var builtinDateFormats = map[int]bool{
	14: true, 15: true, 16: true, 17: true, 18: true, 19: true, 20: true, 21: true, 22: true,
	27: true, 28: true, 29: true, 30: true, 31: true, 32: true, 33: true, 34: true, 35: true, 36: true,
	45: true, 46: true, 47: true, 50: true, 51: true, 52: true, 53: true, 54: true, 55: true, 56: true, 57: true, 58: true,
}

// isDateFormat reports whether the custom number format shows a date or time, ignoring the quoted, bracketed and escaped literals.
func isDateFormat(code string) bool {
	var b strings.Builder
	quoted, bracketed := false, false
	for i := 0; i < len(code); i++ {
		c := code[i]
		switch {
		case quoted:
			quoted = c != '"'
		case bracketed:
			bracketed = c != ']'
		case c == '"':
			quoted = true
		case c == '[':
			bracketed = true
		case c == '\\' || c == '_' || c == '*':
			i++
		default:
			b.WriteByte(c)
		}
	}
	return strings.ContainsAny(strings.ToLower(b.String()), "ymdhs")
}

// workbookReader reads the cells of a workbook.
type workbookReader struct {
	// f is the opened workbook.
	f *excelize.File
	// date1904 reports whether the dates count from 1904 instead of 1900.
	date1904 bool
	// dateStyles caches whether the number format of each style ID shows a date.
	dateStyles map[int]bool
}

// readWorkbook reads the visible sheets of the workbook, or all the sheets if none is visible.
func readWorkbook(data []byte) ([]*Sheet, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data), excelize.Options{UnzipSizeLimit: 512 << 20})
	if err != nil {
		return nil, errors.Wrap(err, "open workbook")
	}
	defer func() { _ = f.Close() }()

	w := &workbookReader{
		f:          f,
		dateStyles: map[int]bool{},
	}
	if props, err := f.GetWorkbookProps(); err == nil && props.Date1904 != nil {
		w.date1904 = *props.Date1904
	}

	names := f.GetSheetList()
	visible := make([]string, 0, len(names))
	for _, name := range names {
		if ok, err := f.GetSheetVisible(name); err == nil && ok {
			visible = append(visible, name)
		}
	}
	if len(visible) == 0 {
		visible = names
	}

	sheets := make([]*Sheet, 0, len(visible))
	for _, name := range visible {
		s, err := w.readSheet(name)
		if err != nil {
			return nil, err
		}
		sheets = append(sheets, s)
	}
	return sheets, nil
}

// readSheet reads the formatted and the raw values of the sheet, and converts them to the cells.
func (w *workbookReader) readSheet(name string) (*Sheet, error) {
	formatted, err := w.f.GetRows(name)
	if err != nil {
		return nil, errors.Wrapf(err, "read sheet %q", name)
	}
	if len(formatted) > MaxRows+1 && countNonBlank(formatted) > MaxRows+1 {
		return nil, ErrTooManyRows
	}
	raw, err := w.f.GetRows(name, excelize.Options{RawCellValue: true})
	if err != nil {
		return nil, errors.Wrapf(err, "read sheet %q", name)
	}

	rows := make([][]*Cell, len(formatted))
	for r, values := range formatted {
		row := make([]*Cell, len(values))
		for c, text := range values {
			var rawValue string
			if r < len(raw) && c < len(raw[r]) {
				rawValue = raw[r][c]
			}
			row[c] = w.cell(name, r, c, text, rawValue)
		}
		rows[r] = row
	}
	return newSheet(name, rows), nil
}

// countNonBlank returns the number of the rows with any non-blank value.
func countNonBlank(rows [][]string) int {
	n := 0
	for _, row := range rows {
		for _, v := range row {
			if strings.TrimSpace(v) != "" {
				n++
				break
			}
		}
	}
	return n
}

// cell converts the formatted and raw values of a cell, the numbers with a date format are read as dates.
func (w *workbookReader) cell(sheet string, r, c int, text, raw string) *Cell {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	number, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return &Cell{Text: text}
	}

	axis, err := excelize.CoordinatesToCellName(c+1, r+1)
	if err != nil {
		return &Cell{Text: text}
	}
	switch typ, _ := w.f.GetCellType(sheet, axis); typ {
	case excelize.CellTypeSharedString, excelize.CellTypeInlineString:
		// Numeric-looking texts such as codes with leading zeros stay texts.
		return &Cell{Text: text}
	case excelize.CellTypeBool:
		return &Cell{
			Text: text,
			Kind: KindBool,
			Bool: number != 0,
		}
	}

	if text != raw && w.isDateCell(sheet, axis) {
		t, err := excelize.ExcelDateToTime(number, w.date1904)
		// Time-only values fall on the epoch day and are kept as texts.
		if err == nil && number >= 1 {
			withTime := t.Hour()+t.Minute()+t.Second() > 0
			return &Cell{
				Text:     t.Format(dateLayout(withTime)),
				Kind:     KindDate,
				Time:     t,
				WithTime: withTime,
			}
		}
		return &Cell{Text: text}
	}
	// The General format abbreviates long numbers such as phone numbers and IDs, e.g. 1.23457E+14.
	if strings.ContainsAny(text, "Ee") && !strings.ContainsAny(raw, "Ee") {
		text = strconv.FormatFloat(number, 'f', -1, 64)
	}
	return &Cell{
		Text:   text,
		Kind:   KindNumber,
		Number: number,
	}
}

// isDateCell reports whether the number format of the cell shows a date or time.
func (w *workbookReader) isDateCell(sheet, axis string) bool {
	styleID, err := w.f.GetCellStyle(sheet, axis)
	if err != nil {
		return false
	}
	if v, ok := w.dateStyles[styleID]; ok {
		return v
	}
	isDate := false
	if style, err := w.f.GetStyle(styleID); err == nil {
		isDate = builtinDateFormats[style.NumFmt] || (style.CustomNumFmt != nil && isDateFormat(*style.CustomNumFmt))
	}
	w.dateStyles[styleID] = isDate
	return isDate
}

// dateLayout returns the layout of the displayed text of a date cell.
func dateLayout(withTime bool) string {
	if withTime {
		return "2006-01-02 15:04"
	}
	return "2006-01-02"
}
