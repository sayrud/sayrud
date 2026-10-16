package dto

import (
	"strings"

	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/importer"
)

// ImportPreviewRows is the number of the leading rows of each sheet returned for the preview, including the header row.
const ImportPreviewRows = 51

// ImportUpload is the response of uploading a file to import.
type ImportUpload struct {
	// UID identifies the import to start.
	UID string `json:"uid"`
	// FileName is the original name of the uploaded file.
	FileName string `json:"fileName"`
	// Sheets are the visible non-blank sheets of the file.
	Sheets []*ImportSheet `json:"sheets"`
} // @name ImportUpload

// ImportSheet is the preview of a sheet of the uploaded file.
type ImportSheet struct {
	// Name is the sheet name, or the file name without the extension for the CSV files.
	Name string `json:"name"`
	// RowCount is the number of the non-blank rows, including the header row.
	RowCount int `json:"rowCount"`
	// Rows are the displayed texts of the leading rows, each padded to the number of the columns.
	Rows [][]string `json:"rows"`
	// Columns are the inferred schemas of the columns, excluding the trailing empty ones.
	Columns []*ImportColumnSchema `json:"columns"`
} // @name ImportSheet

// ImportColumnSchema is the inferred schema of a column, with or without treating the first row as the header.
type ImportColumnSchema struct {
	// Type is inferred from the rows after the first row.
	Type db.SLFieldType `json:"type" enums:"text,single_select,multi_select,datetime,number,checkbox"`
	// Empty reports whether the rows after the first row are all empty.
	Empty bool `json:"empty"`
	// TypeWithoutHeader is inferred from all the rows.
	TypeWithoutHeader db.SLFieldType `json:"typeWithoutHeader" enums:"text,single_select,multi_select,datetime,number,checkbox"`
	// EmptyWithoutHeader reports whether all the rows are empty.
	EmptyWithoutHeader bool `json:"emptyWithoutHeader"`
} // @name ImportColumnSchema

// ToImportSheets returns the previews of the sheets.
func ToImportSheets(sheets []*importer.Sheet) []*ImportSheet {
	result := make([]*ImportSheet, 0, len(sheets))
	for _, s := range sheets {
		preview := s.Rows[:min(len(s.Rows), ImportPreviewRows)]
		rows := make([][]string, len(preview))
		for i, row := range preview {
			texts := make([]string, s.Width)
			for j, c := range row {
				if c != nil {
					texts[j] = c.Text
				}
			}
			rows[i] = texts
		}

		columns := make([]*ImportColumnSchema, s.Width)
		for i := range columns {
			all := importer.Column(s.Rows, i)
			columns[i] = &ImportColumnSchema{
				Type:               importer.DetectType(all[1:]),
				Empty:              isEmptyColumn(all[1:]),
				TypeWithoutHeader:  importer.DetectType(all),
				EmptyWithoutHeader: isEmptyColumn(all),
			}
		}
		result = append(result, &ImportSheet{
			Name:     strings.TrimSpace(s.Name),
			RowCount: len(s.Rows),
			Rows:     rows,
			Columns:  columns,
		})
	}
	return result
}

// isEmptyColumn reports whether all the cells are empty.
func isEmptyColumn(cells []*importer.Cell) bool {
	for _, c := range cells {
		if c != nil {
			return false
		}
	}
	return true
}

// Import is the state of an import.
type Import struct {
	// UID identifies the import.
	UID string `json:"uid"`
	// Status is the state of the import, a running import without progress for a minute is reported as failed.
	Status db.SLImportStatus `json:"status" enums:"pending,running,succeeded,failed"`
	// TableUID is the table the import writes to, empty before a new table is created.
	TableUID string `json:"tableUID"`
	// Done is the number of the records inserted, they are visible only after the import succeeds.
	Done int `json:"done"`
	// Total is the number of the records to insert.
	Total int `json:"total"`
	// Error is the reason of the failure in the language of the request.
	Error string `json:"error,omitempty"`
} // @name Import
