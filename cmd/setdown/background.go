package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"
)

// A background is a file drawn behind the pages (--background) or the first page (--first-background), at the page's
// width from its top. It reaches Typst on stdin as part of the template, like an --invoice's XML, so it can lie
// anywhere; a path would have to lie below the Markdown file's directory, Typst's root.
type background struct {
	file   string
	data   []byte
	format string // as Typst's image takes it: pdf, svg, png, jpg, gif or webp
}

// noBackground reports whether a --background or --first-background value turns the background off.
func noBackground(v string) bool {
	return strings.EqualFold(v, "none")
}

// readBackground reads a background file and tells its format by its content, as a file's extension may be wrong.
func readBackground(file string) (*background, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	format := imageFormat(data, file)
	if format == "" {
		return nil, fmt.Errorf("not a PDF, SVG, PNG, JPEG, GIF or WebP file: %s", file)
	}
	return &background{file: file, data: data, format: format}, nil
}

// imageFormat returns the format of an image by its first bytes, "" for none that Typst takes.
func imageFormat(data []byte, file string) string {
	switch {
	case bytes.HasPrefix(data, []byte("%PDF-")):
		return "pdf"
	case bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")):
		return "png"
	case bytes.HasPrefix(data, []byte("\xff\xd8\xff")):
		return "jpg"
	case bytes.HasPrefix(data, []byte("GIF87a")), bytes.HasPrefix(data, []byte("GIF89a")):
		return "gif"
	case len(data) >= 12 && bytes.HasPrefix(data, []byte("RIFF")) && string(data[8:12]) == "WEBP":
		return "webp"
	case bytes.HasPrefix(data, []byte("\x1f\x8b")) && strings.EqualFold(filepath.Ext(file), ".svgz"):
		return "svg" // Typst unpacks a gzipped SVG itself
	case isSVG(data):
		return "svg"
	}
	return ""
}

// isSVG reports whether data is XML whose root element is svg.
func isSVG(data []byte) bool {
	d := xml.NewDecoder(bytes.NewReader(bytes.TrimPrefix(data, bom)))
	// The encoding does not matter for finding the root element's name.
	d.CharsetReader = func(_ string, r io.Reader) (io.Reader, error) { return r, nil }
	for {
		tok, err := d.Token()
		if err != nil {
			return false
		}
		switch t := tok.(type) {
		case xml.StartElement:
			return t.Name.Local == "svg"
		case xml.CharData:
			if len(bytes.TrimSpace(t)) > 0 {
				return false
			}
		}
	}
}

// typst returns the background as a Typst dictionary on one line. An SVG in UTF-8 goes in as a string, which is as
// long as the file; anything else as a list of its bytes, about three and a half times as long, which Typst still
// reads at several MB per second.
func (b *background) typst() string {
	var s strings.Builder
	s.WriteString("(data: bytes(")
	if b.format == "svg" && utf8.Valid(b.data) {
		s.WriteString(typstString(b.data))
	} else {
		buf := make([]byte, 0, 4*len(b.data)+2)
		buf = append(buf, '(')
		for i, c := range b.data {
			if i > 0 {
				buf = append(buf, ',')
			}
			buf = strconv.AppendInt(buf, int64(c), 10)
		}
		if len(b.data) == 1 {
			buf = append(buf, ',') // (1) would be a number, not a list
		}
		s.Write(append(buf, ')'))
	}
	fmt.Fprintf(&s, "), format: %q)", b.format)
	return s.String()
}

// backgroundMarker is the line of the template that the backgrounds replace.
var backgroundMarker = []byte("// setdown: --background\n")

// backgroundLine returns the Typst line that defines the backgrounds for the template, in place of its marker: one
// line, so that the template's line numbers stay, and a style file's with them. A file that is both backgrounds
// comes in once.
func backgroundLine(first, rest *background) []byte {
	value := func(b *background) string {
		if b == nil {
			return "none"
		}
		return b.typst()
	}
	if first != nil && first == rest {
		return []byte("#let backgrounds = { let both = " + value(first) + "; (first: both, rest: both) }\n")
	}
	return []byte("#let backgrounds = (first: " + value(first) + ", rest: " + value(rest) + ")\n")
}

// withBackgrounds returns the template with the backgrounds' line in place of its marker.
func withBackgrounds(template []byte, first, rest *background) []byte {
	if first == nil && rest == nil {
		return template
	}
	before, after, ok := bytes.Cut(template, backgroundMarker)
	if !ok {
		panic("the template lacks the line " + strings.TrimSpace(string(backgroundMarker)))
	}
	line := backgroundLine(first, rest)
	src := make([]byte, 0, len(before)+len(line)+len(after))
	return append(append(append(src, before...), line...), after...)
}

// backgroundCall is the template's call that draws a background, where Typst reports an image it cannot read.
var backgroundCall = []byte("image(bg.data,")

// backgroundCallLine returns the line of the template with backgroundCall.
func backgroundCallLine(template []byte) int {
	i := bytes.Index(template, backgroundCall)
	if i < 0 {
		panic("the template lacks " + string(backgroundCall))
	}
	return bytes.Count(template[:i], []byte("\n")) + 1
}

// backgrounds returns the distinct backgrounds, first page first.
func (o *options) backgrounds() []*background {
	var list []*background
	for _, b := range []*background{o.firstBg, o.restBg} {
		if b != nil && (len(list) == 0 || list[0] != b) {
			list = append(list, b)
		}
	}
	return list
}

// readBackgrounds reads the background files for check: --first-background for the first page, else --background,
// which is for the other pages. none turns either off.
func (o *options) readBackgrounds() error {
	if o.background != "" && !noBackground(o.background) {
		b, err := readBackground(o.background)
		if err != nil {
			return fmt.Errorf("--background: %v", err)
		}
		o.restBg = b
	}
	switch {
	case o.firstBackground == "":
		o.firstBg = o.restBg
	case !noBackground(o.firstBackground):
		if o.firstBackground == o.background {
			o.firstBg = o.restBg
			break
		}
		b, err := readBackground(o.firstBackground)
		if err != nil {
			return fmt.Errorf("--first-background: %v", err)
		}
		o.firstBg = b
	}
	return nil
}

// optionOf names the option a background came from, for messages.
func (o *options) optionOf(b *background) string {
	switch {
	case b == o.firstBg && b == o.restBg:
		return "--background"
	case b == o.firstBg:
		return "--first-background"
	}
	return "--background"
}

// pdfToSVG is the template that turns the first page of a PDF into an SVG of the same size, for the PDF levels that
// cannot embed a PDF. Transparent parts stay transparent.
const pdfToSVG = `#set page(width: auto, height: auto, margin: 0pt, fill: none)
#image("/" + sys.inputs.file, format: "pdf")
`

// svgArgs returns the arguments of the Typst run that converts a PDF background to SVG, with the template on stdin
// and the SVG on stdout.
func svgArgs(file string) ([]string, error) {
	abs, err := filepath.Abs(file)
	if err != nil {
		return nil, err
	}
	return []string{"compile", "--root", filepath.Dir(abs), "--ignore-system-fonts", "--input",
		"file=" + filepath.Base(abs), "--format", "svg", "-", "-"}, nil
}

// bom is the byte order mark that may start a UTF-8 file.
var bom = []byte{0xef, 0xbb, 0xbf}
