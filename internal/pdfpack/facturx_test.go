package pdfpack

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

const (
	xmpHead = `<?xpacket begin="" id="W5M0MpCehiHzreSzNTczkc9d"?><x:xmpmeta xmlns:x="adobe:ns:meta/"><rdf:RDF ` +
		`xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">`
	xmpTail = `</rdf:RDF></x:xmpmeta><?xpacket end="r"?>`
	// As Typst writes it: one description that declares the extension namespaces and lists its own schemas.
	typstXMP = xmpHead + `<rdf:Description rdf:about="" xmlns:pdfaid="http://www.aiim.org/pdfa/ns/id/"` +
		extensionNamespaces + `><pdfaExtension:schemas><rdf:Bag><rdf:li rdf:parseType="Resource">` +
		`<pdfaSchema:schema>Adobe PDF schema</pdfaSchema:schema></rdf:li></rdf:Bag></pdfaExtension:schemas>` +
		`<pdfaid:part>3</pdfaid:part></rdf:Description>` + xmpTail
	plainXMP = xmpHead + `<rdf:Description rdf:about=""/>` + xmpTail
)

var en16931 = &FacturX{FileName: "factur-x.xml", ConformanceLevel: "EN 16931"}

func TestFacturXMetadata(t *testing.T) {
	for name, xmp := range map[string]string{"Typst's": typstXMP, "without schemas": plainXMP} {
		out, err := en16931.addTo([]byte(xmp))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		s := string(out)
		for _, want := range []string{"<fx:ConformanceLevel>EN 16931</fx:ConformanceLevel>",
			"<fx:DocumentFileName>factur-x.xml</fx:DocumentFileName>", "<fx:DocumentType>INVOICE</fx:DocumentType>",
			"<fx:Version>1.0</fx:Version>", "<pdfaSchema:namespaceURI>" + facturXNamespace + "</pdfaSchema:namespaceURI>"} {
			if strings.Count(s, want) != 1 {
				t.Errorf("%s: %d times %s", name, strings.Count(s, want), want)
			}
		}
		if n := strings.Count(s, "<pdfaExtension:schemas>"); n != 1 {
			t.Errorf("%s: %d pdfaExtension:schemas, want 1", name, n)
		}
		if !strings.HasPrefix(s, xmpHead) || !strings.HasSuffix(s, xmpTail) {
			t.Errorf("%s: the packet's frame changed", name)
		}
	}
	// In Typst's XMP the schema joins the existing list, after Typst's own.
	out, _ := en16931.addTo([]byte(typstXMP))
	if i, j := bytes.Index(out, []byte("Adobe PDF schema")), bytes.Index(out, []byte("Factur-X PDFA Extension Schema")); i < 0 || j < i ||
		j > bytes.Index(out, []byte("</rdf:Bag></pdfaExtension:schemas>")) {
		t.Error("the Factur-X schema is not in Typst's list")
	}

	for name, tt := range map[string]struct {
		f   *FacturX
		xmp string
	}{
		"twice":           {en16931, string(must(en16931.addTo([]byte(typstXMP))))},
		"two lists":       {en16931, strings.Replace(typstXMP, "<pdfaid:part>", `<pdfaExtension:schemas><rdf:Bag/></pdfaExtension:schemas><pdfaid:part>`, 1)},
		"no rdf:RDF":      {en16931, `<x:xmpmeta xmlns:x="adobe:ns:meta/"/>`},
		"an unsafe value": {&FacturX{FileName: "a<b.xml", ConformanceLevel: "BASIC"}, typstXMP},
		"no level":        {&FacturX{FileName: "factur-x.xml"}, typstXMP},
		"broken":          {en16931, strings.Replace(typstXMP, "</pdfaid:part>", "", 1)},
	} {
		if _, err := tt.f.addTo([]byte(tt.xmp)); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

func must(b []byte, err error) []byte {
	if err != nil {
		panic(err)
	}
	return b
}

// TestPackFacturX rewrites the XMP stream of a small PDF and checks the new stream and its /Length.
func TestPackFacturX(t *testing.T) {
	meta := func(xmp string) string {
		return fmt.Sprintf("<</Type/Metadata/Subtype/XML/Length %d>>\nstream\n%s\nendstream", len(xmp), xmp)
	}
	pdf := build([]string{"<</Type/Catalog/Pages 2 0 R/Metadata 3 0 R>>", "<</Type/Pages/Count 1/Kids[]>>",
		meta(typstXMP)}, "<</Size 4/Root 1 0 R>>")
	out, _, err := Pack(pdf, Options{FacturX: en16931})
	if err != nil {
		t.Fatal(err)
	}
	_, raws, _ := unpack(t, out)
	stream := raws[3]
	want, _ := en16931.addTo([]byte(typstXMP))
	if !strings.HasSuffix(stream, "\nstream\n"+string(want)+"\nendstream") ||
		!strings.Contains(stream, fmt.Sprintf("/Length %d>>", len(want))) {
		t.Errorf("the metadata stream: %q", stream)
	}

	for name, objects := range map[string][]string{
		"no metadata": {"<</Type/Catalog/Pages 2 0 R>>", "<</Type/Pages/Count 1/Kids[]>>", "<<>>"},
		"compressed": {"<</Type/Catalog/Pages 2 0 R/Metadata 3 0 R>>", "<</Type/Pages/Count 1/Kids[]>>",
			"<</Type/Metadata/Filter/FlateDecode/Length 3>>\nstream\nabc\nendstream"},
		"indirect length": {"<</Type/Catalog/Pages 2 0 R/Metadata 3 0 R>>", "<</Type/Pages/Count 1/Kids[]>>",
			"<</Type/Metadata/Length 4 0 R>>\nstream\nabc\nendstream", "3"},
	} {
		pdf := build(objects, fmt.Sprintf("<</Size %d/Root 1 0 R>>", len(objects)+1))
		if _, _, err := Pack(pdf, Options{FacturX: en16931}); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	if _, _, err := Pack(pdf, Options{FacturX: en16931, DropMetadata: true}); err == nil {
		t.Error("Factur-X without metadata: no error")
	}
}
