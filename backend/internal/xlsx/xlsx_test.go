package xlsx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"strings"
	"testing"
	"time"
)

func unzip(t *testing.T, data []byte) map[string]string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("not a zip: %v", err)
	}
	out := map[string]string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(rc)
		rc.Close()
		out[f.Name] = string(b)
	}
	return out
}

func wellFormed(t *testing.T, name, body string) {
	t.Helper()
	d := xml.NewDecoder(strings.NewReader(body))
	for {
		_, err := d.Token()
		if err == io.EOF {
			return
		}
		if err != nil {
			t.Fatalf("%s is not well-formed XML: %v", name, err)
		}
	}
}

var jerusalem, _ = time.LoadLocation("Asia/Jerusalem")

func sample() Workbook {
	start := time.Date(2026, 11, 30, 18, 30, 0, 0, jerusalem)
	return Workbook{
		Location: jerusalem,
		Sheets: []Sheet{{
			Name: "הזמנות",
			Columns: []Column{
				{Title: "תאריך", Kind: Date},
				{Title: "שעת התחלה", Kind: Time},
				{Title: "חדר", Kind: Text, Width: 20},
				{Title: "משתתפים", Kind: Number},
				{Title: "מחיר", Kind: Shekel},
			},
			Rows: [][]any{
				{start, start, `אולם "גדול" & <קטן>`, 40, 1250.5},
				{start, start, "סטודיו", nil, 0.0},
			},
			TotalsLabel: "סה\"כ",
		}},
	}
}

func TestWorkbookIsAValidPackage(t *testing.T) {
	data, err := sample().Bytes()
	if err != nil {
		t.Fatal(err)
	}
	files := unzip(t, data)
	for _, name := range []string{"[Content_Types].xml", "_rels/.rels", "xl/workbook.xml", "xl/_rels/workbook.xml.rels", "xl/styles.xml", "xl/worksheets/sheet1.xml"} {
		body, ok := files[name]
		if !ok {
			t.Fatalf("missing part %s", name)
		}
		wellFormed(t, name, body)
	}
	if !strings.Contains(files["xl/workbook.xml"], `name="הזמנות"`) {
		t.Error("sheet name not written")
	}
}

func TestSheetIsRightToLeftWithFrozenFilteredHeader(t *testing.T) {
	data, _ := sample().Bytes()
	s := unzip(t, data)["xl/worksheets/sheet1.xml"]
	for _, want := range []string{
		`rightToLeft="1"`,
		`ySplit="1"`, `state="frozen"`,
		`<autoFilter ref="A1:E3"/>`,
		`<c r="A1" t="inlineStr" s="1"><is><t xml:space="preserve">תאריך</t></is></c>`,
		`width="20"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("sheet XML lacks %s", want)
		}
	}
}

func TestCellsAreTypedAndEscaped(t *testing.T) {
	data, _ := sample().Bytes()
	s := unzip(t, data)["xl/worksheets/sheet1.xml"]
	// 30/11/2026 is serial 46356; 18:30 is 0.770833…
	for _, want := range []string{
		`<c r="A2" s="3"><v>46356</v></c>`,
		`<c r="B2" s="5"><v>0.7708333333</v></c>`,
		`אולם &#34;גדול&#34; &amp; &lt;קטן&gt;`,
		`<c r="D2" s="6"><v>40</v></c>`,
		`<c r="E2" s="2"><v>1250.5</v></c>`,
		`<c r="E3" s="2"><v>0</v></c>`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("sheet XML lacks %s", want)
		}
	}
	if strings.Contains(s, `r="D3"`) {
		t.Error("nil value must leave the cell empty")
	}
}

func TestTotalsRowFollowsTheFilter(t *testing.T) {
	data, _ := sample().Bytes()
	s := unzip(t, data)["xl/worksheets/sheet1.xml"]
	for _, want := range []string{
		`<c r="A4" t="inlineStr" s="7"><is><t>סה&#34;כ</t></is></c>`,
		`<c r="D4" s="7"><f>SUBTOTAL(109,D2:D3)</f></c>`,
		`<c r="E4" s="8"><f>SUBTOTAL(109,E2:E3)</f></c>`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("totals row lacks %s", want)
		}
	}
}

func TestEmptySheetStillHasHeader(t *testing.T) {
	wb := Workbook{Sheets: []Sheet{{Name: "תשלומים", Columns: []Column{{Title: "סכום", Kind: Shekel}}, TotalsLabel: "סה\"כ"}}}
	data, err := wb.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	s := unzip(t, data)["xl/worksheets/sheet1.xml"]
	if !strings.Contains(s, `<autoFilter ref="A1:A1"/>`) || strings.Contains(s, "SUBTOTAL") {
		t.Errorf("empty sheet: want header only, got %s", s)
	}
}

func TestInvalidInputIsRejected(t *testing.T) {
	cols := []Column{{Title: "a"}}
	cases := map[string]Workbook{
		"no sheets":      {},
		"bad name":       {Sheets: []Sheet{{Name: "a/b", Columns: cols}}},
		"long name":      {Sheets: []Sheet{{Name: strings.Repeat("א", 32), Columns: cols}}},
		"duplicate name": {Sheets: []Sheet{{Name: "x", Columns: cols}, {Name: "X", Columns: cols}}},
		"no columns":     {Sheets: []Sheet{{Name: "x"}}},
		"ragged row":     {Sheets: []Sheet{{Name: "x", Columns: cols, Rows: [][]any{{"a", "b"}}}}},
		"wrong type":     {Sheets: []Sheet{{Name: "x", Columns: []Column{{Title: "n", Kind: Shekel}}, Rows: [][]any{{"12"}}}}},
	}
	for name, wb := range cases {
		if _, err := wb.Bytes(); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

func TestColName(t *testing.T) {
	for i, want := range map[int]string{0: "A", 25: "Z", 26: "AA", 27: "AB", 701: "ZZ", 702: "AAA"} {
		if got := ColName(i); got != want {
			t.Errorf("ColName(%d) = %s, want %s", i, got, want)
		}
	}
}
