package form

// StartImport starts importing a sheet of the uploaded file into a table, or into a new table if TableUID is empty.
type StartImport struct {
	// TableUID is the existing table to append the records to.
	TableUID string `json:"tableUID,omitempty"`
	// TableName is the name of the new table, required if TableUID is empty.
	TableName string `json:"tableName,omitempty"`
	// ViewName is the name of the grid view of the new table.
	ViewName string `json:"viewName,omitempty"`
	// Sheet is the index of the sheet in the upload response.
	Sheet int `json:"sheet"`
	// HeaderRow uses the first row as the field titles instead of importing it.
	HeaderRow bool `json:"headerRow"`
	// Timezone is the IANA time zone of the dates without a time zone, e.g. Asia/Shanghai.
	Timezone string `json:"timezone,omitempty"`
	// Columns are the imported columns and their target fields.
	Columns []StartImportColumn `json:"columns"`
} // @name StartImport

// StartImportColumn imports a column into an existing field, or into a new field with the label and type if FieldUID is empty.
type StartImportColumn struct {
	// Index is the zero-based index of the column in the sheet.
	Index int `json:"index"`
	// FieldUID is the existing target field.
	FieldUID string `json:"fieldUID,omitempty"`
	// Label is the title of the new field.
	Label string `json:"label,omitempty"`
	// Type is the type of the new field.
	Type string `json:"type,omitempty" enums:"text,single_select,multi_select,datetime,number,checkbox"`
} // @name StartImportColumn
