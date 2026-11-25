package docx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"strings"
	"testing"
)

func readPart(t *testing.T, b []byte, name string) string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		t.Fatalf("not a zip: %v", err)
	}
	for _, f := range zr.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			defer rc.Close()
			data, _ := io.ReadAll(rc)
			return string(data)
		}
	}
	t.Fatalf("part %s missing", name)
	return ""
}

func TestWriteHebrewDocument(t *testing.T) {
	doc := Document{
		Title: "הסכם רכז/ת <חוגים> & שות׳",
		Body:  "סעיף 1: התחייבות\n\nסעיף 2: תשלום של 100 ₪",
		Pages: []Page{{Heading: "אישור וחתימה", Lines: []string{"שם: נועה", "דוא״ל: noa@example.com"}}},
	}
	var buf bytes.Buffer
	if err := doc.Write(&buf); err != nil {
		t.Fatal(err)
	}
	b := buf.Bytes()
	for _, part := range []string{"[Content_Types].xml", "_rels/.rels", "word/_rels/document.xml.rels", "word/styles.xml"} {
		readPart(t, b, part)
	}
	body := readPart(t, b, "word/document.xml")
	// Well-formed XML (special characters escaped).
	dec := xml.NewDecoder(strings.NewReader(body))
	for {
		if _, err := dec.Token(); err == io.EOF {
			break
		} else if err != nil {
			t.Fatalf("document.xml is not well-formed: %v", err)
		}
	}
	for _, want := range []string{
		"הסכם רכז/ת &lt;חוגים&gt; &amp; שות׳",
		"סעיף 2: תשלום של 100 ₪",
		"noa@example.com",
		`<w:bidi/>`, `<w:rtl/>`,
		`<w:br w:type="page"/>`,
		`w:w="11906" w:h="16838"`, // A4
	} {
		if !strings.Contains(body, want) {
			t.Errorf("document.xml lacks %q", want)
		}
	}
	// Each body line is its own paragraph; the blank line is kept.
	if n := strings.Count(body, "<w:p>"); n < 6 {
		t.Errorf("expected one paragraph per line, got %d", n)
	}
	if !strings.Contains(readPart(t, b, "word/styles.xml"), "Arial") {
		t.Error("default font should be set")
	}
}
