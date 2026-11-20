package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/patrickbergner/setdown/internal/pdfpack"
)

// An eInvoice is the XML of a Factur-X or ZUGFeRD 2.x invoice (UN/CEFACT Cross Industry Invoice) that --invoice
// attaches to the PDF/A-3b, with the profile its guideline ID names.
type eInvoice struct {
	data    []byte
	profile profile
}

// A profile is a Factur-X/ZUGFeRD profile as the PDF's XMP names it, with the attachment's file name.
type profile struct {
	level, fileName string
}

const ciiNamespace = "urn:un:unece:uncefact:data:standard:CrossIndustryInvoice:100"

// xrechnungPrefix starts the guideline ID of every XRechnung version (2.x, 3.x), as the XRECHNUNG profile carries it.
const xrechnungPrefix = "urn:cen.eu:en16931:2017#compliant#urn:xeinkauf.de:kosit:xrechnung_"

// profiles maps the guideline IDs (BT-24) of the supported profiles. MINIMUM and BASIC WL carry no full invoice, so
// setdown leaves them out (see unsupportedProfiles).
var profiles = map[string]profile{
	"urn:cen.eu:en16931:2017#compliant#urn:factur-x.eu:1p0:basic":     {"BASIC", "factur-x.xml"},
	"urn:cen.eu:en16931:2017":                                         {"EN 16931", "factur-x.xml"},
	"urn:cen.eu:en16931:2017#conformant#urn:factur-x.eu:1p0:extended": {"EXTENDED", "factur-x.xml"},
}

var unsupportedProfiles = map[string]string{
	"urn:factur-x.eu:1p0:minimum": "MINIMUM",
	"urn:factur-x.eu:1p0:basicwl": "BASIC WL",
}

func profileOf(id string) (profile, error) {
	if p, ok := profiles[id]; ok {
		return p, nil
	}
	if strings.HasPrefix(id, xrechnungPrefix) && len(id) > len(xrechnungPrefix) {
		return profile{"XRECHNUNG", "xrechnung.xml"}, nil
	}
	if name, ok := unsupportedProfiles[id]; ok {
		return profile{}, fmt.Errorf("the profile %s carries no full invoice; "+
			"use BASIC, EN 16931, EXTENDED or XRECHNUNG", name)
	}
	return profile{}, fmt.Errorf("unknown profile %q; setdown supports BASIC, EN 16931, EXTENDED and XRECHNUNG", id)
}

// readEInvoice reads the XML and finds its profile: rsm:CrossIndustryInvoice, then in rsm:ExchangedDocumentContext
// the ram:ID of ram:GuidelineSpecifiedDocumentContextParameter. It checks no more: Mustang or the receiver validates
// the invoice itself.
func readEInvoice(file string) (*eInvoice, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("no such file: %s", file)
	}
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("not UTF-8, which Factur-X requires: %s", file)
	}
	d := xml.NewDecoder(bytes.NewReader(bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})))
	var path []string
	var id strings.Builder
	found := false
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("not well-formed XML: %s: %v", file, err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if len(path) == 0 && (t.Name.Space != ciiNamespace || t.Name.Local != "CrossIndustryInvoice") {
				return nil, fmt.Errorf("not a Cross Industry Invoice, the XML of Factur-X and ZUGFeRD: %s", file)
			}
			path = append(path, t.Name.Local)
		case xml.EndElement:
			if strings.Join(path, "/") == guidelineID {
				found = true
			}
			path = path[:len(path)-1]
		case xml.CharData:
			if strings.Join(path, "/") == guidelineID && !found {
				id.Write(t)
			}
		}
	}
	if !found {
		return nil, fmt.Errorf("no profile (GuidelineSpecifiedDocumentContextParameter) in %s", file)
	}
	p, err := profileOf(strings.TrimSpace(id.String()))
	if err != nil {
		return nil, err
	}
	return &eInvoice{data: data, profile: p}, nil
}

const guidelineID = "CrossIndustryInvoice/ExchangedDocumentContext/GuidelineSpecifiedDocumentContextParameter/ID"

// typstLine returns the Typst code that attaches the XML, for the end of the template. The XML comes in as a string
// literal, so it reaches the PDF byte for byte from wherever the file lies; a path would have to lie below the
// Markdown file's directory, Typst's root. Its relationship to the pages is Alternative: it holds the same invoice.
func (e *eInvoice) typstLine() []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "#pdf.attach(%q, bytes(%s), relationship: \"alternative\", mime-type: \"text/xml\", "+
		"description: \"The invoice as XML (%s, %s)\")\n", e.profile.fileName, typstString(e.data),
		map[bool]string{true: "XRechnung", false: "Factur-X"}[e.profile.level == "XRECHNUNG"], e.profile.level)
	return b.Bytes()
}

// typstString returns s as a Typst string literal on one line.
func typstString(s []byte) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range string(s) {
		switch {
		case r == '\\':
			b.WriteString(`\\`)
		case r == '"':
			b.WriteString(`\"`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		case r < 0x20 || r == 0x7f || r == 0xFEFF:
			fmt.Fprintf(&b, `\u{%x}`, r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// facturX is what pdfpack writes into the XMP for the invoice.
func (e *eInvoice) facturX() *pdfpack.FacturX {
	return &pdfpack.FacturX{FileName: e.profile.fileName, ConformanceLevel: e.profile.level}
}
