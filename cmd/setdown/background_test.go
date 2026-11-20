package main

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/patrickbergner/setdown/internal/payload"
)

func TestImageFormat(t *testing.T) {
	for _, tt := range []struct {
		data, file, want string
	}{
		{"%PDF-1.7\n", "a.pdf", "pdf"},
		{"\x89PNG\r\n\x1a\n....", "a.png", "png"},
		{"\xff\xd8\xff\xe0", "a.jpeg", "jpg"},
		{"GIF89a", "a.gif", "gif"},
		{"GIF87a", "a.gif", "gif"},
		{"RIFF\x1e\x00\x00\x00WEBPVP8L", "a.webp", "webp"},
		// By content, whatever the extension says.
		{"\x89PNG\r\n\x1a\n....", "a.jpg", "png"},
		{`<svg xmlns="http://www.w3.org/2000/svg"/>`, "a.svg", "svg"},
		{"\uFEFF<?xml version=\"1.0\" encoding=\"ISO-8859-1\"?>\n<!-- logo -->\n<!DOCTYPE svg [<!ENTITY a \"b\">]>\n" +
			"<svg/>", "logo.xml", "svg"},
		{"\x1f\x8b\x08", "a.svgz", "svg"},
		{"\x1f\x8b\x08", "a.gz", ""},
		{"<html><svg/></html>", "a.svg", ""},
		{"text <svg/>", "a.svg", ""},
		{"RIFF\x1e\x00\x00\x00WAVEfmt ", "a.wav", ""},
		{"", "a.png", ""},
		{"#set page(fill: red)", "a.typ", ""},
	} {
		if got := imageFormat([]byte(tt.data), tt.file); got != tt.want {
			t.Errorf("imageFormat(%q, %s) = %q, want %q", tt.data, tt.file, got, tt.want)
		}
	}
}

func TestBackgroundTypst(t *testing.T) {
	for _, tt := range []struct {
		b    background
		want string
	}{
		{background{data: []byte("<svg a=\"1\">\n</svg>"), format: "svg"},
			`(data: bytes("<svg a=\"1\">\n</svg>"), format: "svg")`},
		// Not UTF-8, so as bytes, as for every other format.
		{background{data: []byte("<svg>\xe4</svg>"), format: "svg"},
			`(data: bytes((60,115,118,103,62,228,60,47,115,118,103,62)), format: "svg")`},
		{background{data: []byte{0x89, 0, 255}, format: "png"}, `(data: bytes((137,0,255)), format: "png")`},
		{background{data: []byte{7}, format: "gif"}, `(data: bytes((7,)), format: "gif")`},
	} {
		if got := tt.b.typst(); got != tt.want {
			t.Errorf("typst() = %s, want %s", got, tt.want)
		}
	}
}

func TestWithBackgrounds(t *testing.T) {
	if n := bytes.Count(payload.Template, backgroundMarker); n != 1 {
		t.Fatalf("the template has %d lines %q, want 1", n, backgroundMarker)
	}
	if src := withBackgrounds(payload.Template, nil, nil); !bytes.Equal(src, payload.Template) {
		t.Error("without backgrounds, the template changed")
	}
	png, svg := &background{data: []byte{1, 2}, format: "png"}, &background{data: []byte("<svg/>"), format: "svg"}
	for _, tt := range []struct {
		first, rest *background
		want        string
	}{
		{nil, png, `#let backgrounds = (first: none, rest: (data: bytes((1,2)), format: "png"))`},
		{svg, png, `#let backgrounds = (first: (data: bytes("<svg/>"), format: "svg"), rest: (data: bytes((1,2)), ` +
			`format: "png"))`},
		// A file for every page comes in once.
		{png, png, `#let backgrounds = { let both = (data: bytes((1,2)), format: "png"); (first: both, rest: both) }`},
	} {
		src := withBackgrounds(payload.Template, tt.first, tt.rest)
		if !bytes.Contains(src, []byte("\n"+tt.want+"\n")) {
			t.Errorf("the template lacks the line %s", tt.want)
		}
		if bytes.Count(src, []byte("\n")) != bytes.Count(payload.Template, []byte("\n")) {
			t.Error("the backgrounds change the template's line count")
		}
	}

	// Before the page's set rule, which draws them, and before the style code, whose line numbers so stay.
	style := "#set text(red)\n"
	src, line := templateWith([]byte(style))
	src = withBackgrounds(src, png, svg)
	if got := strings.Split(string(src), "\n")[line-1] + "\n"; got != style {
		t.Errorf("line %d with backgrounds: %q, want the style's %q", line, got, style)
	}
	text := string(payload.Template)
	marker, page := strings.Index(text, string(backgroundMarker)), strings.Index(text, "#set page(")
	if marker > page || page > strings.Index(text, string(styleMarker)) {
		t.Error("the backgrounds' marker does not come before the page's set rule and the style")
	}
	if n := backgroundCallLine(payload.Template); !strings.Contains(strings.Split(text, "\n")[n-1], "image(bg.data") {
		t.Errorf("line %d does not draw the background", n)
	}
}

func TestReadBackgrounds(t *testing.T) {
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a.png"), filepath.Join(dir, "b.svg")
	if err := os.WriteFile(a, []byte("\x89PNG\r\n\x1a\nrest"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("<svg/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		background, first string
		wantFirst         string // the file on the first page, "" for none
		wantRest          string
	}{
		{a, "", a, a},
		{"", b, b, ""},
		{a, b, b, a},
		{a, "none", "", a},
		{"None", b, b, ""},
		{a, a, a, a},
		{"", "", "", ""},
	} {
		o := options{background: tt.background, firstBackground: tt.first}
		if err := o.readBackgrounds(); err != nil {
			t.Fatal(err)
		}
		file := func(bg *background) string {
			if bg == nil {
				return ""
			}
			return bg.file
		}
		if file(o.firstBg) != tt.wantFirst || file(o.restBg) != tt.wantRest {
			t.Errorf("--background %q --first-background %q: first page %q, others %q; want %q and %q",
				tt.background, tt.first, file(o.firstBg), file(o.restBg), tt.wantFirst, tt.wantRest)
		}
		// A file on every page is read once.
		if tt.wantFirst != "" && tt.wantFirst == tt.wantRest && (o.firstBg != o.restBg || len(o.backgrounds()) != 1) {
			t.Errorf("--background %q --first-background %q: the file is read twice", tt.background, tt.first)
		}
	}
	o := options{firstBackground: filepath.Join(dir, "missing.png")}
	if err := o.readBackgrounds(); err == nil || !strings.HasPrefix(err.Error(), "--first-background: ") {
		t.Errorf("a missing file: %v", err)
	}
}

// backgroundFiles writes a background of each format into dir: 40 by 10 pixels (or 400 by 100 pt) in blue, so
// that each covers a quarter of an A4 page's width in height.
func backgroundFiles(t *testing.T, dir string) map[string]string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 40, 10))
	for i := range img.Pix {
		img.Pix[i] = []byte{0x33, 0x66, 0xcc, 0xff}[i%4]
	}
	var pngData, jpegData, gifData bytes.Buffer
	if err := png.Encode(&pngData, img); err != nil {
		t.Fatal(err)
	}
	if err := jpeg.Encode(&jpegData, img, nil); err != nil {
		t.Fatal(err)
	}
	palette := image.NewPaletted(img.Rect, color.Palette{color.RGBA{0x33, 0x66, 0xcc, 0xff}})
	if err := gif.Encode(&gifData, palette, nil); err != nil {
		t.Fatal(err)
	}
	// 40 by 10 pixels of #3366cc, lossless, as ImageMagick writes it; Go writes no WebP.
	webpData, err := hex.DecodeString("524946461e000000574542505650384c110000002f274002000750b3ce94b9ff8188e87f0000")
	if err != nil {
		t.Fatal(err)
	}
	// An image in an SVG is drawn as an image XObject, which the PDF reader can measure; the SVG's own shapes are
	// drawn in the page's content.
	svg := `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="400" ` +
		`height="100" viewBox="0 0 400 100"><image width="400" height="100" xlink:href="data:image/png;base64,` +
		base64.StdEncoding.EncodeToString(pngData.Bytes()) + `"/></svg>`
	var svgz bytes.Buffer
	zw := gzip.NewWriter(&svgz)
	zw.Write([]byte(svg))
	zw.Close()
	files := map[string][]byte{"bg.png": pngData.Bytes(), "bg.jpg": jpegData.Bytes(), "bg.gif": gifData.Bytes(),
		"bg.webp": webpData, "bg.svg": []byte(svg), "bg.svgz": svgz.Bytes(), "bg.pdf": bluePDF(400, 100),
		"broken.png": pngData.Bytes()[:40]}
	paths := map[string]string{}
	for name, data := range files {
		paths[name] = filepath.Join(dir, name)
		if err := os.WriteFile(paths[name], data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return paths
}

// bluePDF returns a PDF of one page of the size, in pt, filled in blue.
func bluePDF(width, height int) []byte {
	content := fmt.Sprintf("0.2 0.4 0.8 rg 0 0 %d %d re f\n", width, height)
	objects := []string{
		"<</Type/Catalog/Pages 2 0 R>>",
		"<</Type/Pages/Kids[3 0 R]/Count 1>>",
		fmt.Sprintf("<</Type/Page/Parent 2 0 R/MediaBox[0 0 %d %d]/Contents 4 0 R/Resources<<>>>>", width, height),
		fmt.Sprintf("<</Length %d>>\nstream\n%sendstream", len(content), content),
	}
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects))
	for i, obj := range objects {
		offsets[i] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, obj)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, off := range offsets {
		fmt.Fprintf(&b, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&b, "trailer\n<</Size %d/Root 1 0 R>>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	return b.Bytes()
}

// TestBackgrounds converts the overview with backgrounds from outside its directory and checks where they are drawn.
func TestBackgrounds(t *testing.T) {
	work := sandbox(t)
	base := filepath.Dir(work)
	in := filepath.Join(work, "example.md")
	files := backgroundFiles(t, base)
	convert := func(t *testing.T, args ...string) (pdfProps, string) {
		t.Helper()
		out := filepath.Join(work, "background.pdf")
		var stdout, stderr bytes.Buffer
		if code, sig := run(append(args, in, out), &stdout, &stderr); code != 0 || sig != nil {
			t.Fatalf("run(%q): exit %d, signal %v\n%s", args, code, sig, stderr.String())
		}
		for _, line := range strings.Split(stderr.String(), "\n") {
			if strings.HasPrefix(line, "warning:") {
				t.Errorf("Typst warns:\n%s", stderr.String())
			}
		}
		p, err := readPDF(out)
		if err != nil {
			t.Fatal(err)
		}
		return p, stderr.String()
	}
	near := func(a, b float64) bool { return math.Abs(a-b) <= 0.5 }
	// drawnAt checks that a page draws exactly one XObject, at the page's width from its top and a quarter of that
	// in height, and returns its object number.
	drawnAt := func(t *testing.T, p pdfProps, page int, subtype string) int {
		t.Helper()
		drawn := p.drawn[page-1]
		if len(drawn) != 1 {
			t.Errorf("page %d draws %+v, want one %s XObject", page, drawn, subtype)
			return 0
		}
		d := drawn[0]
		if d.subtype != subtype || !near(d.x0, 0) || !near(d.x1, p.width) || !near(d.y1, p.height) ||
			!near(d.y0, p.height-p.width/4) {
			t.Errorf("page %d draws %+v, want %s XObject from 0 to %.2f pt across, %.2f to %.2f pt up", page, d,
				subtype, p.width, p.height-p.width/4, p.height)
		}
		return d.obj
	}
	noneAt := func(t *testing.T, p pdfProps, pages ...int) {
		t.Helper()
		for _, page := range pages {
			if len(p.drawn[page-1]) > 0 {
				t.Errorf("page %d draws %+v, want nothing", page, p.drawn[page-1])
			}
		}
	}

	t.Run("every page", func(t *testing.T) {
		p, _ := convert(t, "--background", files["bg.png"])
		// The same object on every page: the file is in the PDF once.
		if a, b, c := drawnAt(t, p, 1, "Image"), drawnAt(t, p, 2, "Image"), drawnAt(t, p, 3, "Image"); a != b || b != c {
			t.Errorf("pages draw objects %d, %d and %d, want one", a, b, c)
		}
		// Behind the text, which stays as it is.
		checkPDF(t, p, pdfWant{3, a4Width, a4Height, "SourceSerif4", jbMono, 11, mm20, false})
	})
	t.Run("first page", func(t *testing.T) {
		p, _ := convert(t, "--first-background", files["bg.png"])
		drawnAt(t, p, 1, "Image")
		noneAt(t, p, 2, 3)
	})
	t.Run("first page and others", func(t *testing.T) {
		p, _ := convert(t, "--first-background", files["bg.jpg"], "--background="+files["bg.gif"])
		a, b, c := drawnAt(t, p, 1, "Image"), drawnAt(t, p, 2, "Image"), drawnAt(t, p, 3, "Image")
		if a == b || b != c {
			t.Errorf("pages draw objects %d, %d and %d, want one for the first page and one for the others", a, b, c)
		}
	})
	for _, name := range []string{"bg.webp", "bg.svg", "bg.svgz"} {
		t.Run(name, func(t *testing.T) {
			p, _ := convert(t, "--background", files[name])
			drawnAt(t, p, 1, "Image")
		})
	}
	t.Run("pdf", func(t *testing.T) {
		p, msg := convert(t, "--background", files["bg.pdf"])
		drawnAt(t, p, 1, "Form")
		if strings.Contains(msg, "warning") {
			t.Errorf("a PDF background at the default level warns:\n%s", msg)
		}
	})
	t.Run("landscape", func(t *testing.T) {
		p, _ := convert(t, "--landscape", "--background", files["bg.png"])
		if !near(p.width, a4Height) {
			t.Fatalf("the page is %.2f pt wide, want A4 landscape", p.width)
		}
		drawnAt(t, p, 1, "Image")
	})
	t.Run("none", func(t *testing.T) {
		writeConfig(t, "background: "+files["bg.png"]+"\n")
		p, _ := convert(t, "--background", "none")
		noneAt(t, p, 1, 2, 3)
		p, _ = convert(t, "--first-background", "none")
		noneAt(t, p, 1)
		drawnAt(t, p, 2, "Image")
	})
	// Typst cannot embed a PDF in these, so setdown converts it to SVG, which Typst draws in the page's content: the
	// blue fill.
	for _, level := range []string{"a3b", "ua1"} {
		t.Run(level, func(t *testing.T) {
			p, msg := convert(t, "--pdf-level", level, "--first-background", files["bg.pdf"])
			if want := "setdown: warning: Typst cannot embed a PDF image in a " + standardNames[level] +
				", so --first-background " + files["bg.pdf"] + " is drawn from an SVG"; !strings.Contains(msg, want) {
				t.Errorf("stderr %q, want %q", msg, want)
			}
			noneAt(t, p, 1, 2, 3)
			if want := []bool{true, false, false}; fmt.Sprint(p.coloredOn) != fmt.Sprint(want) {
				t.Errorf("colored pages %v, want %v", p.coloredOn, want)
			}
		})
	}
	t.Run("invoice", func(t *testing.T) {
		p, msg := convert(t, "--invoice", sample(t, "en16931"), "--background", files["bg.pdf"], "--quiet")
		if !strings.Contains(msg, "warning: Typst cannot embed a PDF image in a PDF/A-3b") {
			t.Errorf("stderr %q, want the warning, which --quiet keeps", msg)
		}
		if fmt.Sprint(p.coloredOn) != "[true true true]" {
			t.Errorf("colored pages %v, want all", p.coloredOn)
		}
	})

	t.Run("broken", func(t *testing.T) {
		out := filepath.Join(work, "broken.pdf")
		var stdout, stderr bytes.Buffer
		if code, _ := run([]string{"--background", files["broken.png"], in, out}, &stdout, &stderr); code != 1 {
			t.Fatalf("exit %d, want Typst's 1\n%s", code, stderr.String())
		}
		line := backgroundCallLine(payload.Template)
		for _, want := range []string{fmt.Sprintf("<stdin>:%d:", line), fmt.Sprintf("setdown: in Typst's messages, "+
			"line %d of <stdin> draws the background %s (--background)\n", line, files["broken.png"])} {
			if !strings.Contains(stderr.String(), want) {
				t.Errorf("stderr lacks %q:\n%s", want, stderr.String())
			}
		}
		stderr.Reset()
		bad := filepath.Join(base, "bad.pdf")
		if err := os.WriteFile(bad, []byte("%PDF-1.7\nnot a PDF\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if code, _ := run([]string{"--pdf-level", "a3b", "--background", bad, in, out}, &stdout, &stderr); code != 1 {
			t.Fatalf("exit %d, want Typst's 1\n%s", code, stderr.String())
		}
		if want := "setdown: could not convert " + bad + " (--background) to SVG, which PDF/A-3b needs\n"; !strings.Contains(
			stderr.String(), want) {
			t.Errorf("stderr lacks %q:\n%s", want, stderr.String())
		}
	})

	if !isEmpty(t, filepath.Join(base, "typst-packages")) {
		t.Error("Typst downloaded a package")
	}
	if !isEmpty(t, filepath.Join(base, "tmp")) {
		t.Error("a run left files in the temporary directory")
	}
}

// TestFirstMarginTop moves the text of the first page down, from the command line and the front matter, and leaves the
// other pages' top margin as it is.
func TestFirstMarginTop(t *testing.T) {
	work := sandbox(t)
	in := filepath.Join(work, "example.md")
	example, err := os.ReadFile(in)
	if err != nil {
		t.Fatal(err)
	}
	fm := filepath.Join(work, "first.md")
	if err := os.WriteFile(fm, bytes.Replace(example, []byte("---\n"), []byte("---\nfirst-margin-top: 50mm\n"), 1),
		0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(work, "first.pdf")
	convert := func(args ...string) pdfProps {
		t.Helper()
		var stdout, stderr bytes.Buffer
		if code, sig := run(append(args, out), &stdout, &stderr); code != 0 || sig != nil {
			t.Fatalf("run(%q): exit %d, signal %v\n%s", args, code, sig, stderr.String())
		}
		p, err := readPDF(out)
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	const mm = 72 / 25.4
	base := convert(in)
	for _, tt := range []struct {
		args  []string
		first float64 // in mm, the first page's top margin
	}{
		{[]string{"--first-margin-top", "50mm", in}, 50},
		{[]string{fm}, 50},
		{[]string{"--first-margin-top=1in", fm}, 25.4},
		{[]string{"--first-margin-top", "20mm", in}, 20},
	} {
		got := convert(tt.args...)
		if want := base.tops[0] + (tt.first-20)*mm; math.Abs(got.tops[0]-want) > 0.5 {
			t.Errorf("%q: first baseline of page 1 %.2f pt from the top, want %.2f", tt.args, got.tops[0], want)
		}
		// The second page starts at the top margin of 20 mm, with a heading or a line of text.
		if got.tops[1] > 20*mm+20 {
			t.Errorf("%q: first baseline of page 2 %.2f pt from the top, want it at the 20 mm margin", tt.args,
				got.tops[1])
		}
	}

	var stdout, stderr bytes.Buffer
	if code, _ := run([]string{"--margin", "30mm 20mm", "--first-margin-top", "25mm", in, out}, &stdout, &stderr); code != 1 ||
		!strings.Contains(stderr.String(), "first-margin-top must be at least the other pages' top margin, 30mm, got 25mm") {
		t.Errorf("a first margin smaller than the others: exit %d\n%s", code, stderr.String())
	}
}
