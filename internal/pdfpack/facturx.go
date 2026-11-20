package pdfpack

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"strings"
)

// FacturX is the metadata a Factur-X or ZUGFeRD 2.x invoice carries in its XMP, next to the attached XML: the XML's
// file name and the profile (ConformanceLevel). The document type is always INVOICE and the version 1.0, as Factur-X
// 1.0 and ZUGFeRD 2.1 and later define them.
type FacturX struct {
	FileName         string // factur-x.xml, or xrechnung.xml for the XRECHNUNG profile
	ConformanceLevel string // BASIC, EN 16931, EXTENDED or XRECHNUNG
}

const facturXNamespace = "urn:factur-x:pdfa:CrossIndustryDocument:invoice:1p0#"

// facturXSchema declares the fx properties, as PDF/A requires for any namespace it does not know. The descriptions
// are Mustang's.
const facturXSchema = `<rdf:li rdf:parseType="Resource">` +
	`<pdfaSchema:schema>Factur-X PDFA Extension Schema</pdfaSchema:schema>` +
	`<pdfaSchema:namespaceURI>` + facturXNamespace + `</pdfaSchema:namespaceURI>` +
	`<pdfaSchema:prefix>fx</pdfaSchema:prefix>` +
	`<pdfaSchema:property><rdf:Seq>` +
	facturXProperty + `DocumentFileName</pdfaProperty:name>` + facturXText + `name of the embedded XML invoice file` + facturXEnd +
	facturXProperty + `DocumentType</pdfaProperty:name>` + facturXText + `INVOICE` + facturXEnd +
	facturXProperty + `Version</pdfaProperty:name>` + facturXText + `The actual version of the ZUGFeRD XML schema` + facturXEnd +
	facturXProperty + `ConformanceLevel</pdfaProperty:name>` + facturXText + `The selected ZUGFeRD profile completeness` + facturXEnd +
	`</rdf:Seq></pdfaSchema:property></rdf:li>`

const (
	facturXProperty = `<rdf:li rdf:parseType="Resource"><pdfaProperty:name>`
	facturXText     = `<pdfaProperty:valueType>Text</pdfaProperty:valueType>` +
		`<pdfaProperty:category>external</pdfaProperty:category><pdfaProperty:description>`
	facturXEnd = `</pdfaProperty:description></rdf:li>`
)

const extensionNamespaces = ` xmlns:pdfaExtension="http://www.aiim.org/pdfa/ns/extension/"` +
	` xmlns:pdfaSchema="http://www.aiim.org/pdfa/ns/schema#"` +
	` xmlns:pdfaProperty="http://www.aiim.org/pdfa/ns/property#"`

// addTo returns the XMP packet with the Factur-X properties in a description of their own and the schema that
// declares them. An XMP property may appear only once, so the schema joins the pdfaExtension:schemas that Typst
// writes for its own extensions; without one, the new description brings its own.
func (f *FacturX) addTo(xmp []byte) ([]byte, error) {
	for _, s := range []string{f.FileName, f.ConformanceLevel} {
		if s == "" || strings.ContainsAny(s, `<>&"'`) {
			return nil, errors.New("an empty or unsafe Factur-X value")
		}
	}
	if bytes.Contains(xmp, []byte(facturXNamespace)) {
		return nil, errors.New("the XMP already has Factur-X metadata")
	}
	if bytes.Count(xmp, []byte("</rdf:RDF>")) != 1 {
		return nil, errors.New("not one rdf:RDF element")
	}
	description := `<rdf:Description rdf:about="" xmlns:fx="` + facturXNamespace + `">` +
		`<fx:DocumentType>INVOICE</fx:DocumentType>` +
		`<fx:DocumentFileName>` + f.FileName + `</fx:DocumentFileName>` +
		`<fx:Version>1.0</fx:Version>` +
		`<fx:ConformanceLevel>` + f.ConformanceLevel + `</fx:ConformanceLevel>` +
		`</rdf:Description>`

	out := xmp
	switch n := bytes.Count(xmp, []byte("</pdfaExtension:schemas>")); n {
	case 0:
		description += `<rdf:Description rdf:about=""` + extensionNamespaces + `>` +
			`<pdfaExtension:schemas><rdf:Bag>` + facturXSchema + `</rdf:Bag></pdfaExtension:schemas></rdf:Description>`
	case 1:
		end := bytes.Index(xmp, []byte("</pdfaExtension:schemas>"))
		bag := bytes.LastIndex(xmp[:end], []byte("</rdf:Bag>"))
		if bag < 0 || len(bytes.TrimSpace(xmp[bag+len("</rdf:Bag>"):end])) != 0 {
			return nil, errors.New("pdfaExtension:schemas holds no rdf:Bag")
		}
		out = insert(xmp, bag, facturXSchema)
	default:
		return nil, errors.New("more than one pdfaExtension:schemas")
	}
	out = insert(out, bytes.Index(out, []byte("</rdf:RDF>")), description)

	// What comes out must still be well-formed XML.
	d := xml.NewDecoder(bytes.NewReader(out))
	d.Strict = true
	for {
		_, err := d.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, errors.New("the result is not well-formed: " + err.Error())
		}
	}
	return out, nil
}

func insert(b []byte, at int, s string) []byte {
	return append(append(append(make([]byte, 0, len(b)+len(s)), b[:at]...), s...), b[at:]...)
}
