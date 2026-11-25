// Package xlsx writes simple Excel workbooks with the standard library only.
//
// It covers what the CRM exports need: several sheets, right-to-left layout
// for Hebrew, a bold frozen header row, Excel auto-filter, column widths,
// shekel amounts and Israeli dates. Strings are written inline, so no shared
// string table is needed.
package xlsx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"time"
)

// Kind tells how a column's cells are written and formatted.
type Kind int

const (
	Text     Kind = iota
	Number        // plain number, e.g. attendees
	Shekel        // amount in ₪ with two decimals
	Date          // dd/mm/yyyy
	DateTime      // dd/mm/yyyy hh:mm
	Time          // hh:mm
)

// Column describes one column of a sheet.
type Column struct {
	Title string
	Kind  Kind
	Width float64 // in characters; 0 means 14
}

// Sheet is one tab of the workbook. Each row holds one value per column:
// string for Text, float64/int for Number and Shekel, time.Time for dates.
// A nil value leaves the cell empty.
type Sheet struct {
	Name    string
	Columns []Column
	Rows    [][]any
	// TotalsLabel, when set, adds a last row that sums every Shekel and
	// Number column with a SUBTOTAL formula (so it follows the auto-filter).
	TotalsLabel string
}

// Workbook is a set of sheets written right-to-left.
type Workbook struct {
	Sheets []Sheet
	// Location used to turn time.Time values into Excel local dates.
	Location *time.Location
}

// Style indexes in styles.xml (cellXfs).
const (
	styleDefault  = 0
	styleHeader   = 1
	styleShekel   = 2
	styleDate     = 3
	styleDateTime = 4
	styleTime     = 5
	styleNumber   = 6
	styleTotal    = 7
	styleTotalSum = 8
)

// Write encodes the workbook as .xlsx into w.
func (wb Workbook) Write(w io.Writer) error {
	if len(wb.Sheets) == 0 {
		return fmt.Errorf("xlsx: no sheets")
	}
	loc := wb.Location
	if loc == nil {
		loc = time.UTC
	}
	seen := map[string]bool{}
	for _, s := range wb.Sheets {
		if s.Name == "" || len([]rune(s.Name)) > 31 || strings.ContainsAny(s.Name, `[]:*?/\`) {
			return fmt.Errorf("xlsx: invalid sheet name %q", s.Name)
		}
		if seen[strings.ToLower(s.Name)] {
			return fmt.Errorf("xlsx: duplicate sheet name %q", s.Name)
		}
		seen[strings.ToLower(s.Name)] = true
		if len(s.Columns) == 0 {
			return fmt.Errorf("xlsx: sheet %q has no columns", s.Name)
		}
		for i, r := range s.Rows {
			if len(r) != len(s.Columns) {
				return fmt.Errorf("xlsx: sheet %q row %d has %d cells, want %d", s.Name, i+1, len(r), len(s.Columns))
			}
		}
	}

	zw := zip.NewWriter(w)
	files := []struct{ name, body string }{
		{"[Content_Types].xml", contentTypes(len(wb.Sheets))},
		{"_rels/.rels", rootRels},
		{"xl/workbook.xml", workbookXML(wb.Sheets)},
		{"xl/_rels/workbook.xml.rels", workbookRels(len(wb.Sheets))},
		{"xl/styles.xml", stylesXML},
	}
	for i, s := range wb.Sheets {
		body, err := sheetXML(s, loc)
		if err != nil {
			return err
		}
		files = append(files, struct{ name, body string }{fmt.Sprintf("xl/worksheets/sheet%d.xml", i+1), body})
	}
	for _, f := range files {
		fw, err := zw.Create(f.name)
		if err != nil {
			return err
		}
		if _, err := io.WriteString(fw, f.body); err != nil {
			return err
		}
	}
	return zw.Close()
}

// Bytes is Write into a buffer.
func (wb Workbook) Bytes() ([]byte, error) {
	var b bytes.Buffer
	if err := wb.Write(&b); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// ColName turns a 0-based index into A, B, …, Z, AA, …
func ColName(i int) string {
	name := ""
	for i++; i > 0; i = (i - 1) / 26 {
		name = string(rune('A'+(i-1)%26)) + name
	}
	return name
}

func esc(s string) string {
	var b strings.Builder
	// xml.EscapeText also drops characters that are invalid in XML.
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

// excelSerial converts a time to Excel's day serial in loc (1900 date system).
func excelSerial(t time.Time, loc *time.Location) float64 {
	lt := t.In(loc)
	naive := time.Date(lt.Year(), lt.Month(), lt.Day(), lt.Hour(), lt.Minute(), lt.Second(), 0, time.UTC)
	epoch := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)
	return naive.Sub(epoch).Hours() / 24
}

func num(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	}
	return 0, false
}

func fmtNum(f float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.10f", f), "0"), ".")
}

func cellXML(ref string, c Column, v any, loc *time.Location) (string, error) {
	if v == nil {
		return "", nil
	}
	switch c.Kind {
	case Text:
		s, ok := v.(string)
		if !ok {
			s = fmt.Sprint(v)
		}
		if s == "" {
			return "", nil
		}
		return fmt.Sprintf(`<c r="%s" t="inlineStr"><is><t xml:space="preserve">%s</t></is></c>`, ref, esc(s)), nil
	case Number, Shekel:
		f, ok := num(v)
		if !ok {
			return "", fmt.Errorf("xlsx: cell %s: %T is not a number", ref, v)
		}
		style := styleNumber
		if c.Kind == Shekel {
			style = styleShekel
		}
		return fmt.Sprintf(`<c r="%s" s="%d"><v>%s</v></c>`, ref, style, fmtNum(f)), nil
	case Date, DateTime, Time:
		t, ok := v.(time.Time)
		if !ok {
			return "", fmt.Errorf("xlsx: cell %s: %T is not a time", ref, v)
		}
		if t.IsZero() {
			return "", nil
		}
		serial := excelSerial(t, loc)
		style := styleDate
		switch c.Kind {
		case DateTime:
			style = styleDateTime
		case Time:
			style = styleTime
			serial -= float64(int(serial))
		default:
			serial = float64(int(serial))
		}
		return fmt.Sprintf(`<c r="%s" s="%d"><v>%s</v></c>`, ref, style, fmtNum(serial)), nil
	}
	return "", fmt.Errorf("xlsx: unknown column kind %d", c.Kind)
}

func sheetXML(s Sheet, loc *time.Location) (string, error) {
	var b strings.Builder
	last := ColName(len(s.Columns) - 1)
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	b.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">`)
	b.WriteString(`<sheetViews><sheetView rightToLeft="1" workbookViewId="0"><pane ySplit="1" topLeftCell="A2" activePane="bottomLeft" state="frozen"/><selection pane="bottomLeft" activeCell="A2" sqref="A2"/></sheetView></sheetViews>`)
	b.WriteString(`<cols>`)
	for i, c := range s.Columns {
		w := c.Width
		if w <= 0 {
			w = 14
		}
		fmt.Fprintf(&b, `<col min="%d" max="%d" width="%s" customWidth="1"/>`, i+1, i+1, fmtNum(w))
	}
	b.WriteString(`</cols><sheetData>`)

	b.WriteString(`<row r="1">`)
	for i, c := range s.Columns {
		fmt.Fprintf(&b, `<c r="%s1" t="inlineStr" s="%d"><is><t xml:space="preserve">%s</t></is></c>`, ColName(i), styleHeader, esc(c.Title))
	}
	b.WriteString(`</row>`)

	for ri, row := range s.Rows {
		r := ri + 2
		fmt.Fprintf(&b, `<row r="%d">`, r)
		for ci, v := range row {
			cell, err := cellXML(fmt.Sprintf("%s%d", ColName(ci), r), s.Columns[ci], v, loc)
			if err != nil {
				return "", err
			}
			b.WriteString(cell)
		}
		b.WriteString(`</row>`)
	}

	lastData := len(s.Rows) + 1
	if s.TotalsLabel != "" && len(s.Rows) > 0 {
		r := lastData + 1
		fmt.Fprintf(&b, `<row r="%d">`, r)
		labelDone := false
		for ci, c := range s.Columns {
			ref := fmt.Sprintf("%s%d", ColName(ci), r)
			switch {
			case c.Kind == Shekel || c.Kind == Number:
				style := styleTotalSum
				if c.Kind == Number {
					style = styleTotal
				}
				fmt.Fprintf(&b, `<c r="%s" s="%d"><f>SUBTOTAL(109,%s2:%s%d)</f></c>`, ref, style, ColName(ci), ColName(ci), lastData)
			case !labelDone:
				fmt.Fprintf(&b, `<c r="%s" t="inlineStr" s="%d"><is><t>%s</t></is></c>`, ref, styleTotal, esc(s.TotalsLabel))
				labelDone = true
			default:
				fmt.Fprintf(&b, `<c r="%s" s="%d"/>`, ref, styleTotal)
			}
		}
		b.WriteString(`</row>`)
	}
	b.WriteString(`</sheetData>`)
	fmt.Fprintf(&b, `<autoFilter ref="A1:%s%d"/>`, last, lastData)
	b.WriteString(`<pageMargins left="0.5" right="0.5" top="0.75" bottom="0.75" header="0.3" footer="0.3"/>`)
	b.WriteString(`<pageSetup orientation="landscape" fitToWidth="1" fitToHeight="0"/>`)
	b.WriteString(`</worksheet>`)
	return b.String(), nil
}

func contentTypes(n int) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">`)
	b.WriteString(`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/>`)
	b.WriteString(`<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>`)
	b.WriteString(`<Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>`)
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, `<Override PartName="/xl/worksheets/sheet%d.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>`, i)
	}
	b.WriteString(`</Types>`)
	return b.String()
}

const rootRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`

func workbookXML(sheets []Sheet) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">`)
	b.WriteString(`<bookViews><workbookView/></bookViews><sheets>`)
	for i, s := range sheets {
		fmt.Fprintf(&b, `<sheet name="%s" sheetId="%d" r:id="rId%d"/>`, esc(s.Name), i+1, i+1)
	}
	b.WriteString(`</sheets><definedNames>`)
	for i, s := range sheets {
		rows := len(s.Rows) + 1
		fmt.Fprintf(&b, `<definedName name="_xlnm._FilterDatabase" localSheetId="%d" hidden="1">'%s'!$A$1:$%s$%d</definedName>`,
			i, esc(strings.ReplaceAll(s.Name, "'", "''")), ColName(len(s.Columns)-1), rows)
	}
	b.WriteString(`</definedNames></workbook>`)
	return b.String()
}

func workbookRels(n int) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, `<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet%d.xml"/>`, i, i)
	}
	fmt.Fprintf(&b, `<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>`, n+1)
	b.WriteString(`</Relationships>`)
	return b.String()
}

// Number formats: 164 ₪ amount, 165 date, 166 date-time, 167 time.
const stylesXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
	`<numFmts count="4">` +
	`<numFmt numFmtId="164" formatCode="&quot;₪&quot;\ #,##0.00;\-&quot;₪&quot;\ #,##0.00"/>` +
	`<numFmt numFmtId="165" formatCode="dd/mm/yyyy"/>` +
	`<numFmt numFmtId="166" formatCode="dd/mm/yyyy\ hh:mm"/>` +
	`<numFmt numFmtId="167" formatCode="hh:mm"/>` +
	`</numFmts>` +
	`<fonts count="2"><font><sz val="11"/><name val="Arial"/><family val="2"/></font><font><b/><sz val="11"/><name val="Arial"/><family val="2"/></font></fonts>` +
	`<fills count="3"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill>` +
	`<fill><patternFill patternType="solid"><fgColor rgb="FFE8F1F0"/><bgColor indexed="64"/></patternFill></fill></fills>` +
	`<borders count="2"><border><left/><right/><top/><bottom/><diagonal/></border>` +
	`<border><left/><right/><top style="thin"><color auto="1"/></top><bottom style="thin"><color auto="1"/></bottom><diagonal/></border></borders>` +
	`<cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs>` +
	`<cellXfs count="9">` +
	`<xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/>` +
	`<xf numFmtId="0" fontId="1" fillId="2" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1"/>` +
	`<xf numFmtId="164" fontId="0" fillId="0" borderId="0" xfId="0" applyNumberFormat="1"/>` +
	`<xf numFmtId="165" fontId="0" fillId="0" borderId="0" xfId="0" applyNumberFormat="1"/>` +
	`<xf numFmtId="166" fontId="0" fillId="0" borderId="0" xfId="0" applyNumberFormat="1"/>` +
	`<xf numFmtId="167" fontId="0" fillId="0" borderId="0" xfId="0" applyNumberFormat="1"/>` +
	`<xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/>` +
	`<xf numFmtId="0" fontId="1" fillId="2" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1"/>` +
	`<xf numFmtId="164" fontId="1" fillId="2" borderId="1" xfId="0" applyNumberFormat="1" applyFont="1" applyFill="1" applyBorder="1"/>` +
	`</cellXfs>` +
	`<cellStyles count="1"><cellStyle name="Normal" xfId="0" builtinId="0"/></cellStyles>` +
	`</styleSheet>`
