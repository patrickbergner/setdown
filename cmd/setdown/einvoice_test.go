package main

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The sample invoices in testdata/e-invoice are one invoice in the four profiles setdown supports, each valid by
// Mustang (CI checks the PDFs setdown makes of them). BASIC has no seller contact, which its schema lacks.
var sampleProfiles = map[string]profile{
	"basic":     {"BASIC", "factur-x.xml"},
	"en16931":   {"EN 16931", "factur-x.xml"},
	"extended":  {"EXTENDED", "factur-x.xml"},
	"xrechnung": {"XRECHNUNG", "xrechnung.xml"},
}

func sample(t *testing.T, name string) string {
	t.Helper()
	file, err := filepath.Abs(filepath.Join("testdata", "e-invoice", name+".xml"))
	if err != nil {
		t.Fatal(err)
	}
	return file
}

func TestReadEInvoice(t *testing.T) {
	for name, want := range sampleProfiles {
		inv, err := readEInvoice(sample(t, name))
		if err != nil {
			t.Errorf("%s: %v", name, err)
		} else if inv.profile != want {
			t.Errorf("%s: %+v, want %+v", name, inv.profile, want)
		}
	}

	dir := t.TempDir()
	en16931, err := os.ReadFile(sample(t, "en16931"))
	if err != nil {
		t.Fatal(err)
	}
	withID := func(id string) string {
		return strings.Replace(string(en16931), "<ram:ID>urn:cen.eu:en16931:2017</ram:ID>", "<ram:ID>"+id+"</ram:ID>", 1)
	}
	for _, tt := range []struct {
		name, content, err string
	}{
		{"minimum", withID("urn:factur-x.eu:1p0:minimum"), "the profile MINIMUM carries no full invoice"},
		{"basic wl", withID("urn:factur-x.eu:1p0:basicwl"), "the profile BASIC WL carries no full invoice"},
		{"unknown", withID("urn:example:other"), `unknown profile "urn:example:other"`},
		{"xrechnung without a version", withID(xrechnungPrefix), "unknown profile"},
		{"no profile", strings.Replace(string(en16931), "GuidelineSpecifiedDocumentContextParameter", "Other", 2),
			"no profile"},
		{"ubl", `<?xml version="1.0"?><Invoice xmlns="urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"/>`,
			"not a Cross Industry Invoice"},
		{"broken", "<rsm:CrossIndustryInvoice", "not well-formed XML"},
		{"latin-1", "<?xml version=\"1.0\"?><a>Stra\xdfe</a>", "not UTF-8"},
	} {
		file := filepath.Join(dir, "invoice.xml")
		if err := os.WriteFile(file, []byte(tt.content), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := readEInvoice(file); err == nil || !strings.Contains(err.Error(), tt.err) {
			t.Errorf("%s: %v, want %q", tt.name, err, tt.err)
		}
	}
	if _, err := readEInvoice(filepath.Join(dir, "missing.xml")); err == nil || !strings.HasPrefix(err.Error(), "no such file") {
		t.Errorf("a missing file: %v", err)
	}
	// Another XRechnung version and a byte order mark are fine.
	file := filepath.Join(dir, "invoice.xml")
	content := "\xef\xbb\xbf" + withID(xrechnungPrefix+"2.3")
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if inv, err := readEInvoice(file); err != nil || inv.profile.level != "XRECHNUNG" || string(inv.data) != content {
		t.Errorf("XRechnung 2.3 with a BOM: %v %+v", err, inv)
	}
}

func TestTypstString(t *testing.T) {
	for in, want := range map[string]string{
		`a "b" \c`:          `"a \"b\" \\c"`,
		"line\r\nnext\ttab": `"line\r\nnext\ttab"`,
		"\x00\x1f\x7f":      `"\u{0}\u{1f}\u{7f}"`,
		"\xef\xbb\xbfÄ€":    `"\u{feff}Ä€"`,
	} {
		if got := typstString([]byte(in)); got != want {
			t.Errorf("typstString(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestCheckEInvoice(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "invoice.md")
	if err := os.WriteFile(in, []byte("Invoice.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, level := range []string{"", "a3b"} {
		o := options{input: in, eInvoice: sample(t, "en16931"), pdfLevel: level}
		if err := o.check(); err != nil || o.pdfLevel != "a3b" || o.invoice == nil {
			t.Errorf("level %q: %v, level %q", level, err, o.pdfLevel)
		}
	}
	for _, level := range []string{"default", "ua1"} {
		o := options{input: in, eInvoice: sample(t, "en16931"), pdfLevel: level}
		if err := o.check(); err == nil || err.Error() != "--invoice makes a PDF/A-3b and takes no --pdf-level "+level {
			t.Errorf("level %q: %v", level, err)
		}
	}
	o := options{input: in, eInvoice: filepath.Join(dir, "missing.xml")}
	if err := o.check(); err == nil || !strings.HasPrefix(err.Error(), "--invoice: no such file") {
		t.Errorf("a missing file: %v", err)
	}
}

// TestEInvoice makes an e-invoice of each sample and checks what Factur-X asks of the PDF: PDF/A-3b, the XML attached
// unchanged under its name as the pages' alternative, and the fx metadata with its extension schema in the XMP.
func TestEInvoice(t *testing.T) {
	work := sandbox(t)
	in := filepath.Join(work, "invoice.md")
	if err := os.WriteFile(in, []byte("---\ntitle: Invoice 2026-0815\n---\n\nTotal: 155.77 EUR\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(work, "invoice.pdf")
	reEmbedded := regexp.MustCompile(`/Type\s*/EmbeddedFile\b`)
	for name, want := range sampleProfiles {
		var stdout, stderr bytes.Buffer
		if code, _ := run([]string{"--invoice", sample(t, name), in}, &stdout, &stderr); code != 0 {
			t.Fatalf("%s: exit %d, stderr %q", name, code, stderr.String())
		}
		if msg := stderr.String(); strings.Contains(msg, "warning:") || strings.Contains(msg, "could not make") {
			t.Errorf("%s: %s", name, msg)
		}
		content := pdfContent(t, out)
		for _, s := range []string{
			"<pdfaid:part>3</pdfaid:part>", "<pdfaid:conformance>B</pdfaid:conformance>",
			"<fx:DocumentType>INVOICE</fx:DocumentType>", "<fx:Version>1.0</fx:Version>",
			"<fx:DocumentFileName>" + want.fileName + "</fx:DocumentFileName>",
			"<fx:ConformanceLevel>" + want.level + "</fx:ConformanceLevel>",
			"<pdfaSchema:prefix>fx</pdfaSchema:prefix>", "/AFRelationship/Alternative", "(" + want.fileName + ")",
		} {
			if !bytes.Contains(content, []byte(s)) {
				t.Errorf("%s: the PDF lacks %s", name, s)
			}
		}
		if n := bytes.Count(content, []byte("</pdfaExtension:schemas>")); n != 1 {
			t.Errorf("%s: %d pdfaExtension:schemas, want 1", name, n)
		}
		// The attached file is the XML, byte for byte.
		data, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		objects, err := pdfObjects(data)
		if err != nil {
			t.Fatal(err)
		}
		xml, err := os.ReadFile(sample(t, name))
		if err != nil {
			t.Fatal(err)
		}
		found := 0
		for _, o := range objects {
			if reEmbedded.Match(o.dict) {
				found++
				if !bytes.Equal(o.stream, xml) {
					t.Errorf("%s: the attached file differs from the XML", name)
				}
			}
		}
		if found != 1 {
			t.Errorf("%s: %d attached files, want 1", name, found)
		}
	}
}
