package importer

import (
	"regexp"
	"strings"

	"github.com/samber/lo"

	"github.com/wuhan005/sayrud/internal/db"
)

// detectRows is the number of leading data rows inspected when inferring the column types.
const detectRows = 2000

var (
	// CheckboxTrueTexts are the lowercase texts of a checked checkbox, the same as textToValue of the frontend.
	CheckboxTrueTexts = []string{"true", "1", "是", "yes", "y", "✓", "√", "checked"}
	// checkboxFalseTexts are the lowercase texts of an unchecked checkbox, only used when inferring the column types.
	checkboxFalseTexts = []string{"false", "0", "否", "no", "n", "✗", "×", "unchecked"}

	// numberPattern matches the plain numbers with an optional sign, currency symbol, thousands separators and percent sign.
	numberPattern = regexp.MustCompile(`^[-+]?[¥$]?[-+]?(?:\d{1,3}(?:,\d{3})+|\d+)(?:\.\d+)?%?$`)
	// datePattern also matches the Chinese dates such as 2024年1月15日, the characters are escaped as \x{5e74} (year), \x{6708} (month) and \x{65e5} (day).
	datePattern = regexp.MustCompile(`^\d{4}[-/.\x{5e74}]\d{1,2}[-/.\x{6708}]\d{1,2}\x{65e5}?(?:[ T]\d{1,2}:\d{2}(?::\d{2}(?:\.\d+)?)?(?:Z|[+-]\d{2}:?\d{2})?)?$`)
	// codePattern matches the integers of 11+ digits, such as phone numbers and IDs.
	codePattern = regexp.MustCompile(`^[-+]?\d{11,}$`)
	// MultiSelectSeparator splits the option names of a multiple select cell, the same as the frontend.
	MultiSelectSeparator = regexp.MustCompile(`[,，、;；\n]`)
)

// Column returns the cells of the column in the rows.
func Column(rows [][]*Cell, index int) []*Cell {
	cells := make([]*Cell, len(rows))
	for i, row := range rows {
		if index < len(row) {
			cells[i] = row[index]
		}
	}
	return cells
}

// nonEmpty returns at most limit non-empty cells.
func nonEmpty(cells []*Cell, limit int) []*Cell {
	values := make([]*Cell, 0, min(len(cells), limit))
	for _, c := range cells {
		if c == nil {
			continue
		}
		values = append(values, c)
		if len(values) >= limit {
			break
		}
	}
	return values
}

// isNumberText reports whether the text is a plain number, codes with leading zeros or more than 15 digits are kept as texts.
func isNumberText(text string) bool {
	if !numberPattern.MatchString(text) {
		return false
	}
	digits := strings.Map(func(r rune) rune {
		if (r >= '0' && r <= '9') || r == '.' {
			return r
		}
		return -1
	}, text)
	if len(digits) > 1 && digits[0] == '0' && digits[1] != '.' {
		return false
	}
	return len(strings.Replace(digits, ".", "", 1)) <= 15
}

// IsDateText reports whether the text is a date such as 2024-01-15, 2024/1/15 10:30 or 2024年1月15日.
func IsDateText(text string) bool {
	return datePattern.MatchString(text)
}

// isBooleanText reports whether the text is a checked or unchecked checkbox.
func isBooleanText(text string) bool {
	t := strings.ToLower(text)
	return lo.Contains(CheckboxTrueTexts, t) || lo.Contains(checkboxFalseTexts, t)
}

// looksLikeOptions reports whether the repeated short values look like options: 2 to 20 distinct ones, each used twice on average.
func looksLikeOptions(names []string) bool {
	distinct := lo.Uniq(names)
	if len(names) < 5 || len(distinct) < 2 || len(distinct) > 20 || len(distinct)*2 > len(names) {
		return false
	}
	return lo.EveryBy(distinct, func(s string) bool { return len([]rune(s)) <= 40 && !strings.Contains(s, "\n") })
}

// SplitOptionNames splits the text of a multiple select cell into the option names.
func SplitOptionNames(text string) []string {
	names := make([]string, 0)
	for _, part := range MultiSelectSeparator.Split(text, -1) {
		if name := strings.TrimSpace(part); name != "" {
			names = append(names, name)
		}
	}
	return names
}

// DetectType infers the field type of the column from its non-empty cells.
func DetectType(cells []*Cell) db.SLFieldType {
	values := nonEmpty(cells, detectRows)
	if len(values) == 0 {
		return db.TextFieldType
	}
	if lo.EveryBy(values, func(c *Cell) bool { return c.Kind == KindDate || (c.Kind == KindText && IsDateText(c.Text)) }) {
		return db.DateTimeFieldType
	}
	if lo.EveryBy(values, func(c *Cell) bool { return c.Kind == KindNumber || (c.Kind == KindText && isNumberText(c.Text)) }) {
		// Integers of 11+ digits such as phone numbers and IDs are codes rather than quantities.
		if lo.EveryBy(values, func(c *Cell) bool { return codePattern.MatchString(c.Text) }) {
			return db.TextFieldType
		}
		return db.NumberFieldType
	}
	if lo.EveryBy(values, func(c *Cell) bool { return c.Kind == KindBool || (c.Kind == KindText && isBooleanText(c.Text)) }) {
		return db.CheckboxFieldType
	}
	lists := lo.Map(values, func(c *Cell, _ int) []string { return SplitOptionNames(c.Text) })
	if lo.SomeBy(lists, func(names []string) bool { return len(names) > 1 }) && looksLikeOptions(lo.Flatten(lists)) {
		return db.MultiSelectFieldType
	}
	if looksLikeOptions(lo.Map(values, func(c *Cell, _ int) string { return c.Text })) {
		return db.SingleSelectFieldType
	}
	return db.TextFieldType
}

var (
	// decimalsPattern captures the decimal digits of a number text.
	decimalsPattern = regexp.MustCompile(`\.(\d+)`)
	// thousandsPattern matches a number text with the thousands separator.
	thousandsPattern = regexp.MustCompile(`\d,\d{3}`)
)

// NumberFormat infers the number format from the displayed texts: thousands separator, decimals, percent and currency.
func NumberFormat(cells []*Cell) string {
	decimals, percentDecimals := 0, -1
	thousands := false
	currency := ""
	for _, c := range nonEmpty(cells, detectRows) {
		d := 0
		if m := decimalsPattern.FindStringSubmatch(c.Text); m != nil {
			d = len(m[1])
		}
		if strings.HasSuffix(c.Text, "%") {
			percentDecimals = max(percentDecimals, d)
		} else {
			decimals = max(decimals, d)
		}
		thousands = thousands || thousandsPattern.MatchString(c.Text)
		if currency == "" && strings.ContainsAny(c.Text, "¥$") {
			currency = lo.Ternary(strings.Contains(c.Text, "¥"), "¥", "$")
		}
	}
	switch {
	case currency != "":
		return currency + "0,000.00"
	case percentDecimals >= 0:
		return lo.Ternary(percentDecimals > 0, "0.00%", "0%")
	}
	d := min(decimals, 3)
	if thousands {
		return lo.Ternary(d > 0, "0,000.00", "0,000")
	}
	if d == 0 {
		return "0"
	}
	return "0." + strings.Repeat("0", d)
}

// DateFormat infers the date format and whether the values have a time of day.
func DateFormat(cells []*Cell) (format string, withTime bool) {
	dashed := false
	for _, c := range nonEmpty(cells, detectRows) {
		if c.Kind == KindDate {
			withTime = withTime || c.WithTime
			continue
		}
		withTime = withTime || strings.Contains(c.Text, ":")
		dashed = dashed || (len(c.Text) > 4 && c.Text[4] == '-')
	}
	return lo.Ternary(dashed, "YYYY-MM-DD", "YYYY/MM/DD"), withTime
}
