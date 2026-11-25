// Package docx writes simple right-to-left Word documents with the standard
// library only. The CRM uses it for contracts: OpenSign converts the file to
// PDF (LibreOffice) before it is signed.
//
// Layout: A4, 1 inch (72 pt) margins, Arial 12 pt. The title is centred, each
// body line becomes a justified paragraph, and every extra Page starts on a
// new page.
package docx

import (
	"archive/zip"
	"encoding/xml"
	"io"
	"strconv"
	"strings"
)

// Page geometry in PDF points, for placing widgets on the converted PDF.
const (
	PageWidthPt  = 595.0
	PageHeightPt = 842.0
	MarginPt     = 72.0
)

// Document is a title, a free-text body and optional extra pages.
type Document struct {
	Title string
	Body  string
	Pages []Page
}

// Page starts on a new page with a bold heading and one paragraph per line.
type Page struct {
	Heading string
	Lines   []string
}

// Write encodes the document as a .docx file.
func (d Document) Write(w io.Writer) error {
	zw := zip.NewWriter(w)
	parts := []struct{ name, body string }{
		{"[Content_Types].xml", contentTypes},
		{"_rels/.rels", rootRels},
		{"word/_rels/document.xml.rels", documentRels},
		{"word/styles.xml", styles},
		{"word/document.xml", d.documentXML()},
	}
	for _, p := range parts {
		f, err := zw.Create(p.name)
		if err != nil {
			return err
		}
		if _, err := io.WriteString(f, p.body); err != nil {
			return err
		}
	}
	return zw.Close()
}

func (d Document) documentXML() string {
	var b strings.Builder
	b.WriteString(xml.Header)
	b.WriteString(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>`)
	paragraph(&b, d.Title, "center", true, 32)
	paragraph(&b, "", "both", false, 0)
	for _, line := range strings.Split(strings.ReplaceAll(d.Body, "\r\n", "\n"), "\n") {
		paragraph(&b, line, "both", false, 0)
	}
	for _, p := range d.Pages {
		b.WriteString(`<w:p><w:r><w:br w:type="page"/></w:r></w:p>`)
		paragraph(&b, p.Heading, "center", true, 28)
		paragraph(&b, "", "both", false, 0)
		for _, line := range p.Lines {
			paragraph(&b, line, "both", false, 0)
		}
	}
	b.WriteString(`<w:sectPr><w:pgSz w:w="11906" w:h="16838"/>` +
		`<w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440" w:header="708" w:footer="708" w:gutter="0"/>` +
		`<w:bidi/></w:sectPr></w:body></w:document>`)
	return b.String()
}

// paragraph writes one right-to-left paragraph. size is in half-points (0: default).
func paragraph(b *strings.Builder, text, jc string, bold bool, size int) {
	b.WriteString(`<w:p><w:pPr><w:bidi/><w:jc w:val="` + jc + `"/></w:pPr>`)
	if text != "" {
		b.WriteString(`<w:r><w:rPr><w:rtl/>`)
		if bold {
			b.WriteString(`<w:b/><w:bCs/>`)
		}
		if size > 0 {
			s := strconv.Itoa(size)
			b.WriteString(`<w:sz w:val="` + s + `"/><w:szCs w:val="` + s + `"/>`)
		}
		b.WriteString(`</w:rPr><w:t xml:space="preserve">`)
		xml.EscapeText(b, []byte(text)) //nolint:errcheck // strings.Builder never fails
		b.WriteString(`</w:t></w:r>`)
	}
	b.WriteString(`</w:p>`)
}

const contentTypes = xml.Header + `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
	`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
	`<Default Extension="xml" ContentType="application/xml"/>` +
	`<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>` +
	`<Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>` +
	`</Types>`

const rootRels = xml.Header + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
	`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>` +
	`</Relationships>`

const documentRels = xml.Header + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
	`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>` +
	`</Relationships>`

// Arial is replaced by a metric-compatible font with Hebrew glyphs
// (Liberation Sans / DejaVu) when LibreOffice renders the PDF.
const styles = xml.Header + `<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
	`<w:docDefaults><w:rPrDefault><w:rPr>` +
	`<w:rFonts w:ascii="Arial" w:hAnsi="Arial" w:eastAsia="Arial" w:cs="Arial"/>` +
	`<w:sz w:val="24"/><w:szCs w:val="24"/><w:lang w:val="he-IL" w:bidi="he-IL"/>` +
	`</w:rPr></w:rPrDefault>` +
	`<w:pPrDefault><w:pPr><w:bidi/><w:spacing w:after="120" w:line="300" w:lineRule="auto"/></w:pPr></w:pPrDefault>` +
	`</w:docDefaults>` +
	`<w:style w:type="paragraph" w:default="1" w:styleId="Normal"><w:name w:val="Normal"/><w:pPr><w:bidi/></w:pPr><w:rPr><w:rtl/></w:rPr></w:style>` +
	`</w:styles>`
