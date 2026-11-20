package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// invoice attaches its XML as e-invoices do (ZUGFeRD, Factur-X): as the alternative to what the pages show. PDF/A-3
// allows an attached file only with a description. The template's read reads the file relative to the Markdown file;
// pdf.attach would look for it in cmarker's package, where the comment's code is evaluated.
const invoice = "---\ntitle: Invoice 2026-042\n---\n\n" +
	"<!--raw-typst #pdf.attach(\"factur-x.xml\", read(\"factur-x.xml\", encoding: none),\n" +
	"    relationship: \"alternative\", mime-type: \"text/xml\", description: \"Factur-X invoice\")-->\n\n" +
	"## Items\n\n| Item | Amount |\n|---|--:|\n| Consulting, order `PO-17` | 1,200.00 |\n"

// TestPDFLevels converts an invoice with an attached XML at each --pdf-level and checks what each puts into the PDF.
func TestPDFLevels(t *testing.T) {
	work := sandbox(t)
	in := filepath.Join(work, "invoice.md")
	if err := os.WriteFile(in, []byte(invoice), 0o644); err != nil {
		t.Fatal(err)
	}
	xml := "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<invoice/>\n"
	if err := os.WriteFile(filepath.Join(work, "factur-x.xml"), []byte(xml), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(work, "invoice.pdf")

	// Present in every level: the copy and search maps, the bookmarks, the document information, the attachment.
	// Typst marks the attachment as the pages' alternative only in PDF/A-3, which has associated files.
	always := []string{"/ToUnicode", "/Outlines", "/Title(Invoice 2026-042)", "/EmbeddedFiles", "(factur-x.xml)"}
	tests := []struct {
		args          []string
		want, without []string
	}{
		{nil, nil, []string{"/StructTreeRoot", "<x:xmpmeta", "pdfaid:", "pdfuaid:"}},
		{[]string{"--pdf-level", "default"}, nil, []string{"/StructTreeRoot", "<x:xmpmeta"}},
		{[]string{"--pdf-level=a3b"}, []string{"/StructTreeRoot", "<pdfaid:part>3</pdfaid:part>",
			"<pdfaid:conformance>B</pdfaid:conformance>", "/OutputIntents", "/AFRelationship/Alternative"}, []string{"pdfuaid:"}},
		{[]string{"--pdf-level", "ua1"}, []string{"/StructTreeRoot", "<pdfuaid:part>1</pdfuaid:part>", "/S/H1"},
			[]string{"pdfaid:"}},
	}
	for _, tt := range tests {
		var stdout, stderr bytes.Buffer
		if code, _ := run(append(tt.args, in), &stdout, &stderr); code != 0 {
			t.Fatalf("%q: exit %d, stderr %q", tt.args, code, stderr.String())
		}
		if msg := stderr.String(); strings.Contains(msg, "warning:") || strings.Contains(msg, "could not make") {
			t.Errorf("%q: %s", tt.args, msg)
		}
		data, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		// pdfpack's work: a cross-reference stream and object streams, so no dictionary is left readable as text.
		if bytes.Contains(data, []byte("\nxref\n")) || !bytes.Contains(data, []byte("/Type/ObjStm")) ||
			bytes.Contains(data, []byte("/Type/Catalog")) {
			t.Errorf("%q: not packed into object streams", tt.args)
		}
		content := pdfContent(t, out)
		// The code font is TrueType (glyf) without its hinting (fpgm, the hinting's function definitions).
		if !bytes.Contains(content, []byte("glyf")) || bytes.Contains(content, []byte("fpgm")) {
			t.Errorf("%q: no TrueType code font, or one with hinting", tt.args)
		}
		for _, s := range append(always, tt.want...) {
			if !bytes.Contains(content, []byte(s)) {
				t.Errorf("%q: the PDF lacks %s", tt.args, s)
			}
		}
		for _, s := range tt.without {
			if bytes.Contains(content, []byte(s)) {
				t.Errorf("%q: the PDF has %s", tt.args, s)
			}
		}
		if p, err := readPDF(out); err != nil {
			t.Errorf("%q: %v", tt.args, err)
		} else if p.pages != 1 || len(p.text) != 1 || !strings.Contains(strings.Join(p.text[0], " "), "Consulting") {
			t.Errorf("%q: %d pages, text %q", tt.args, p.pages, p.text)
		}
	}

	// setdown refuses an unknown level before Typst runs.
	var stdout, stderr bytes.Buffer
	if code, _ := run([]string{"--pdf-level", "a-3b", in}, &stdout, &stderr); code != 1 ||
		!strings.HasPrefix(stderr.String(), "setdown: --pdf-level must be default, a3b or ua1") {
		t.Errorf("a-3b: exit %d, stderr %q", code, stderr.String())
	}
}

// TestTitleHeading checks that the front matter's title is the document's level 1 heading, as PDF/UA-1 wants when
// the document's own headings start at ##, and that it stays out of a table of contents. The outline has no title,
// which would be a level 1 heading of its own.
func TestTitleHeading(t *testing.T) {
	work := sandbox(t)
	in := filepath.Join(work, "report.md")
	out := filepath.Join(work, "report.pdf")
	body := "<!--raw-typst #outline(title: none)-->\n\n## Alpha\n\nText.\n\n### Beta\n\nMore text.\n"
	if err := os.WriteFile(in, []byte("---\ntitle: Quarterly Report\n---\n\n"+body), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code, _ := run([]string{"--pdf-level", "ua1", in}, &stdout, &stderr); code != 0 {
		t.Fatalf("ua1 with a title: exit %d, stderr %q", code, stderr.String())
	}
	content := pdfContent(t, out)
	// The structure tree: the title's H1, the sections' H2 and H3.
	for tag, want := range map[string]int{"/S/H1": 1, "/S/H2": 1, "/S/H3": 1} {
		if n := bytes.Count(content, []byte(tag)); n != want {
			t.Errorf("%d times %s, want %d", n, tag, want)
		}
	}
	p, err := readPDF(out)
	if err != nil {
		t.Fatal(err)
	}
	var text []string
	for _, runs := range p.text {
		text = append(text, runs...)
	}
	all := strings.Join(text, " ")
	if n := strings.Count(all, "Quarterly Report"); n != 1 {
		t.Errorf("the title shows %d times, want once, not in the table of contents: %q", n, all)
	}
	if n := strings.Count(all, "Alpha"); n != 2 {
		t.Errorf("Alpha shows %d times, want twice, in the table of contents and as the heading: %q", n, all)
	}

	// toc: true puts the outline below the title, with a title of its own, a second level 1 heading, before the ##.
	toc := strings.Replace(body, "<!--raw-typst #outline(title: none)-->\n", "", 1)
	if err := os.WriteFile(in, []byte("---\ntitle: Quarterly Report\ntoc: true\n---\n\n"+toc), 0o644); err != nil {
		t.Fatal(err)
	}
	stderr.Reset()
	if code, _ := run([]string{"--pdf-level", "ua1", in}, &stdout, &stderr); code != 0 {
		t.Fatalf("ua1 with toc: true: exit %d, stderr %q", code, stderr.String())
	}
	content = pdfContent(t, out)
	for tag, want := range map[string]int{"/S/H1": 2, "/S/H2": 1, "/S/H3": 1, "/S/TOCI": 2} {
		if n := len(regexp.MustCompile(tag+`\b`).FindAll(content, -1)); n != want {
			t.Errorf("toc: true: %d times %s, want %d", n, tag, want)
		}
	}
	if !regexp.MustCompile(`/S/TOC\b`).Match(content) {
		t.Error("toc: true: no TOC in the structure tree")
	}
	// The bookmarks are the document's headings, without the contents' title (the metadata's title comes along).
	var titles []string
	for _, m := range regexp.MustCompile(`/Title\s*\(([^)]*)\)`).FindAllSubmatch(content, -1) {
		titles = append(titles, string(m[1]))
	}
	sort.Strings(titles)
	if !reflect.DeepEqual(titles, []string{"Alpha", "Beta", "Quarterly Report"}) {
		t.Errorf("toc: true: titles %q, want the bookmarks Alpha and Beta and the metadata's title", titles)
	}

	// Without a title, the document's first heading is of level 2, which PDF/UA-1 refuses.
	if err := os.WriteFile(in, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(out); err != nil {
		t.Fatal(err)
	}
	stderr.Reset()
	if code, _ := run([]string{"--pdf-level", "ua1", in}, &stdout, &stderr); code == 0 ||
		!strings.Contains(stderr.String(), "the first heading must be of level 1") {
		t.Errorf("ua1 without a title: exit %d, stderr %q", code, stderr.String())
	}
}
