package main

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/patrickbergner/setdown/internal/payload"
)

// The tests need the assets of the platform at hand: go run ./tools/fetch -target <os>/<arch>, or ./run-tests.sh.

func TestMain(m *testing.M) {
	// The user's own defaults would change what the tests convert, also in the test binary run as setdown below.
	os.Setenv("SETDOWN_CONFIG", "")
	os.Unsetenv("SETDOWN_OPTIONS")
	// The signal tests run the test binary as setdown itself.
	if os.Getenv("SETDOWN_TEST_AS_MAIN") == "1" {
		main()
		return
	}
	os.Exit(m.Run())
}

func TestParseArgs(t *testing.T) {
	tests := []struct {
		args []string
		want options
		err  string
	}{
		{[]string{"in.md"}, options{input: "in.md"}, ""},
		{[]string{"in.md", "out.pdf"}, options{input: "in.md", output: "out.pdf"}, ""},
		{[]string{"in.md", "--sans", "out.pdf", "-l", "de"}, options{input: "in.md", output: "out.pdf", font: "sans", lang: "de"}, ""},
		{[]string{"-s", "--serif", "in.md"}, options{input: "in.md", font: "serif"}, ""},
		{[]string{"--scale", "90%", "--scale=0.8", "in.md"}, options{input: "in.md", scale: "0.8"}, ""},
		{[]string{"--lang=de-CH", "--paper=a5", "--margin=2cm", "x"}, options{input: "x", lang: "de-CH", paper: "a5", margin: "2cm"}, ""},
		{[]string{"-p", "us-letter", "-m", "1in", "x"}, options{input: "x", paper: "us-letter", margin: "1in"}, ""},
		{[]string{"--highlight", "x"}, options{input: "x", highlight: "true"}, ""},
		{[]string{"--highlight", "--no-highlight", "x"}, options{input: "x", highlight: "false"}, ""},
		{[]string{"--bullet", "•", "--numbering=a)", "x"}, options{input: "x", bullet: "•", numbering: "a)"}, ""},
		{[]string{"--numbering"}, options{}, "--numbering needs a value"},
		{[]string{"--landscape", "x"}, options{input: "x", landscape: "true"}, ""},
		{[]string{"--landscape", "--portrait", "x"}, options{input: "x", landscape: "false"}, ""},
		{[]string{"--toc", "x"}, options{input: "x", toc: "true"}, ""},
		{[]string{"x", "--toc=2"}, options{input: "x", toc: "2"}, ""},
		{[]string{"--toc", "--no-toc", "x"}, options{input: "x", toc: "false"}, ""},
		// --toc takes its value only after =, so a 2 after it is the input.
		{[]string{"--toc", "2", "x"}, options{input: "2", output: "x", toc: "true"}, ""},
		{[]string{"--toc="}, options{}, "--toc= needs a value, such as --toc=2"},
		{[]string{"--heading-numbering", "1.1", "x"}, options{input: "x", headingNumbering: "1.1"}, ""},
		{[]string{"--heading-numbering"}, options{}, "--heading-numbering needs a value"},
		{[]string{"-q", "x"}, options{input: "x", quiet: "true"}, ""},
		{[]string{"--quiet", "--no-quiet", "x"}, options{input: "x", quiet: "false"}, ""},
		{[]string{"--quiet=yes"}, options{}, "--quiet takes no value"},
		{[]string{"--portrait=1"}, options{}, "--portrait takes no value"},
		{[]string{"--page-numbers", "Page 1 of 5", "x"}, options{input: "x", pageNumbers: "Page 1 of 5"}, ""},
		{[]string{"x", "--pdf-level=a3b"}, options{input: "x", pdfLevel: "a3b"}, ""},
		{[]string{"--pdf-level"}, options{}, "--pdf-level needs a value"},
		{[]string{"--invoice", "factur-x.xml", "x"}, options{input: "x", eInvoice: "factur-x.xml"}, ""},
		{[]string{"--font", "Inter", "--mono-font=Iosevka", "x"},
			options{input: "x", font: "Inter", monoFont: "Iosevka"}, ""},
		{[]string{"--font", "Inter", "--sans", "x"}, options{input: "x", font: "sans"}, ""},
		{[]string{"--font-path", "a", "x", "--font-path=b c"}, options{input: "x", fontPaths: []string{"a", "b c"}}, ""},
		{[]string{"--style", "corp.typ", "--style=letter.typ", "x"}, options{input: "x", style: "letter.typ"}, ""},
		{[]string{"x", "--font-path"}, options{}, "--font-path needs a value"},
		{[]string{"--style="}, options{}, "--style needs a value"},
		{[]string{"--background", "a.pdf", "--first-background=b.svg", "--first-margin-top", "60mm", "x"},
			options{input: "x", background: "a.pdf", firstBackground: "b.svg", firstMarginTop: "60mm"}, ""},
		{[]string{"--background", "none", "x"}, options{input: "x", background: "none"}, ""},
		{[]string{"--first-background"}, options{}, "--first-background needs a value"},
		{[]string{"--", "-odd.md", "--sans"}, options{input: "-odd.md", output: "--sans"}, ""},
		{[]string{"--version"}, options{showVersion: true}, ""},
		{[]string{"--licenses"}, options{licenses: true}, ""},
		{[]string{"--clean-cache", "x"}, options{input: "x", cleanCache: true}, ""},
		{[]string{"a", "b", "c"}, options{}, "too many arguments"},
		{[]string{"a", "--", "b", "c"}, options{}, "too many arguments"},
		{[]string{"--scale"}, options{}, "--scale needs a value"},
		{[]string{"--lang="}, options{}, "--lang needs a value"},
		{[]string{"-l", ""}, options{}, "-l needs a value"},
		{[]string{"--sans=yes"}, options{}, "--sans takes no value"},
		{[]string{"--frobnicate=1"}, options{}, "unknown option: --frobnicate=1 (see --help)"},
		{[]string{"-x"}, options{}, "unknown option: -x (see --help)"},
		{[]string{"-"}, options{}, "unknown option: - (see --help)"},
		{[]string{"-u"}, options{}, "unknown option: -u (see --help)"},
		{[]string{"--update"}, options{}, "unknown option: --update (see --help)"},
		{[]string{"x", "-h", "--bogus"}, options{}, errHelp.Error()},
		{[]string{"--help=1"}, options{}, "--help takes no value"},
	}
	for _, tt := range tests {
		got, err := parseArgs(tt.args)
		if tt.err != "" {
			if err == nil || err.Error() != tt.err {
				t.Errorf("parseArgs(%q): error %v, want %q", tt.args, err, tt.err)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseArgs(%q): %v", tt.args, err)
		} else if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("parseArgs(%q) = %+v, want %+v", tt.args, got, tt.want)
		}
	}
}

func TestCheck(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "notes.v2.md")
	if err := os.WriteFile(in, []byte("# Notes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	latin1 := filepath.Join(dir, "latin1.typ")
	if err := os.WriteFile(latin1, []byte("// Gr\xfc\xdfe\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	o := options{input: in}
	if err := o.check(); err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "notes.v2.pdf"); o.output != want {
		t.Errorf("output %s, want %s", o.output, want)
	}

	bad := []struct {
		o   options
		err string
	}{
		{options{input: filepath.Join(dir, "missing.md")}, "no such file: "},
		{options{input: dir}, "no such file: "},
		{options{input: in, output: in}, "the output would overwrite the input"},
		{options{input: in, scale: "90 %"}, "--scale must be a factor"},
		{options{input: in, scale: "-1"}, "--scale must be a factor"},
		{options{input: in, margin: "2"}, "--margin must be a length"},
		{options{input: in, margin: "2em"}, "--margin must be a length"},
		{options{input: in, margin: "30mm 20mm 2"}, "--margin must be a length"},
		{options{input: in, margin: "1mm 2mm 3mm 4mm 5mm"}, "--margin must be a length"},
		{options{input: in, margin: " "}, "--margin must be a length"},
		{options{input: in, pageNumbers: "Page"}, "--page-numbers must be none or show a number"},
		{options{input: in, pageNumbers: "1/2/3"}, "--page-numbers must be none or show a number"},
		{options{input: in, pdfLevel: "a-3b"}, "--pdf-level must be default, a3b or ua1, got: a-3b"},
		{options{input: in, pdfLevel: "UA1"}, "--pdf-level must be default, a3b or ua1"},
		{options{input: in, pdfLevel: "a3b,ua1"}, "--pdf-level must be default, a3b or ua1"},
		{options{input: in, fontPaths: []string{dir, filepath.Join(dir, "fonts")}}, "--font-path: no such directory: "},
		{options{input: in, fontPaths: []string{in}}, "--font-path: no such directory: "},
		{options{input: in, style: filepath.Join(dir, "missing.typ")}, "--style: no such file: "},
		{options{input: in, style: dir}, "--style: no such file: "},
		{options{input: in, style: latin1}, "--style: not UTF-8 text"},
		{options{input: in, firstMarginTop: "2em"}, "--first-margin-top must be a length such as 60mm, got: 2em"},
		{options{input: in, firstMarginTop: "30mm 20mm"}, "--first-margin-top must be a length"},
		{options{input: in, background: filepath.Join(dir, "missing.png")}, "--background: no such file: "},
		{options{input: in, firstBackground: dir}, "--first-background: no such file: "},
		{options{input: in, background: in}, "--background: not a PDF, SVG, PNG, JPEG, GIF or WebP file: "},
		{options{input: in, toc: "7"}, "--toc must be true, false or the number of heading levels from 1 to 6, got: 7"},
		{options{input: in, toc: "0"}, "--toc must be"},
		{options{input: in, toc: "deep"}, "--toc must be"},
		// Values such as these come only from a config file.
		{options{input: in, highlight: "maybe"}, "--highlight must be true or false, got: maybe"},
		{options{input: in, landscape: "1"}, "--landscape must be true or false"},
		{options{input: in, quiet: "si"}, "--quiet must be true or false"},
	}
	for _, b := range bad {
		if err := b.o.check(); err == nil || !strings.HasPrefix(err.Error(), b.err) {
			t.Errorf("check(%+v): %v, want %q", b.o, err, b.err)
		}
	}
	for _, good := range []options{{input: in, scale: "0.9"}, {input: in, scale: "90%"}, {input: in, scale: "1"},
		{input: in, margin: "20mm"}, {input: in, margin: "1.5cm"}, {input: in, margin: "1in"}, {input: in, margin: "36pt"},
		{input: in, margin: "30mm 20mm"}, {input: in, margin: "30mm  1in 2cm"}, {input: in, margin: "1mm 2mm 3mm 4.5pt"},
		{input: in, pageNumbers: "1"}, {input: in, pageNumbers: "Seite 1 von 5"}, {input: in, pageNumbers: "None"},
		{input: in, pdfLevel: "default"}, {input: in, pdfLevel: "a3b"}, {input: in, pdfLevel: "ua1"},
		{input: in, fontPaths: []string{dir, dir}}, {input: in, toc: "true"}, {input: in, toc: "6"},
		{input: in, toc: "No"}, {input: in, highlight: "yes"}, {input: in, landscape: "off"}, {input: in, quiet: "on"},
		{input: in, firstMarginTop: "2.5in"}, {input: in, background: "none", firstBackground: "None"}} {
		if err := good.check(); err != nil {
			t.Errorf("check(%+v): %v", good, err)
		}
	}

	// The style code loses a byte order mark and ends in a newline, so the template goes on in a line of its own.
	for content, want := range map[string]string{"\uFEFF#set text(red)": "#set text(red)\n", "a\r\nb\r\n": "a\r\nb\r\n",
		"": ""} {
		style := filepath.Join(dir, "style.typ")
		if err := os.WriteFile(style, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		o := options{input: in, style: style}
		if err := o.check(); err != nil {
			t.Fatal(err)
		} else if string(o.styleCode) != want || o.styleCode == nil {
			t.Errorf("style code of %q: %q, want %q", content, o.styleCode, want)
		}
	}
}

func TestTypstArgs(t *testing.T) {
	o := &options{input: filepath.Join("docs", "report.md"), output: "-report.pdf", font: "sans", lang: "de", highlight: "false",
		bullet: "•", numbering: "1)", pdfLevel: "a3b"}
	got := typstArgs(o, "ROOT", "FONTS", "PACKAGES")
	want := []string{
		"compile", "--root", "ROOT", "--font-path", "FONTS", "--ignore-system-fonts", "--package-path", "PACKAGES",
		"--input", "md=report.md", "--input", "name=report", "--input", "font=sans", "--input", "lang=de",
		"--input", "highlight=false", "--input", "bullet=•", "--input", "numbering=1)", "--pdf-standard", "a-3b",
		"-", "." + string(filepath.Separator) + "-report.pdf",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("typstArgs:\n got %q\nwant %q", got, want)
	}

	// The user's fonts come after setdown's, the style file does not reach Typst's arguments.
	o = &options{input: "report.md", output: "report.pdf", font: "Inter", monoFont: "Iosevka",
		fontPaths: []string{"my fonts", "more"}, style: "corp.typ"}
	got = typstArgs(o, "ROOT", "FONTS", "PACKAGES")
	want = []string{
		"compile", "--root", "ROOT", "--font-path", "FONTS", "--font-path", "my fonts", "--font-path", "more",
		"--ignore-system-fonts", "--package-path", "PACKAGES", "--input", "md=report.md", "--input", "name=report",
		"--input", "font=Inter", "--input", "mono-font=Iosevka", "--no-pdf-tags", "-", "report.pdf",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("typstArgs with fonts:\n got %q\nwant %q", got, want)
	}

	// The new settings in their place, and the user's defaults in one input after them.
	o = &options{input: "r.md", output: "r.pdf", landscape: "true", headingNumbering: "1.1", toc: "2", pageNumbers: "1",
		firstMarginTop: "60mm", background: "bg.pdf", quiet: "true",
		defaults: map[string]any{"lang": "de", "bullet": []string{"•", "◦"}}}
	got = typstArgs(o, "ROOT", "FONTS", "PACKAGES")
	want = []string{
		"compile", "--root", "ROOT", "--font-path", "FONTS", "--ignore-system-fonts", "--package-path", "PACKAGES",
		"--input", "md=r.md", "--input", "name=r", "--input", "landscape=true", "--input", "first-margin-top=60mm",
		"--input", "heading-numbering=1.1",
		"--input", "toc=2", "--input", "page-numbers=1", "--input", `defaults={"bullet":["•","◦"],"lang":"de"}`,
		"--no-pdf-tags", "-", "r.pdf",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("typstArgs with the new settings:\n got %q\nwant %q", got, want)
	}
}

func TestTemplateWith(t *testing.T) {
	if src, line := templateWith(nil); !bytes.Equal(src, payload.Template) || line != 0 {
		t.Errorf("without style code: line %d, template changed: %v", line, !bytes.Equal(src, payload.Template))
	}
	if n := bytes.Count(payload.Template, styleMarker); n != 1 {
		t.Fatalf("the template has %d lines %q, want 1", n, styleMarker)
	}
	style := "#set text(red)\n#show heading: set text(blue)\n"
	src, line := templateWith([]byte(style))
	lines := strings.Split(string(src), "\n")
	if got := strings.Join(lines[line-1:line+1], "\n") + "\n"; got != style {
		t.Errorf("lines %d and %d: %q, want %q", line, line+1, got, style)
	}
	if bytes.Contains(src, styleMarker) || len(src) != len(payload.Template)-len(styleMarker)+len(style) {
		t.Error("the style code does not replace the marker alone")
	}
	// The rules that the style code should be able to override come before it, the body after it.
	before, after, _ := strings.Cut(string(src), style)
	for _, rule := range []string{"#set page(", "#set text(font: (body-font", "#show heading:",
		"#show raw: set text(font:"} {
		if !strings.Contains(before, rule) {
			t.Errorf("%q does not come before the style code", rule)
		}
	}
	if !strings.Contains(after, "keep-with-next(body)") || !strings.Contains(after, "#if title != none") {
		t.Error("the title and the body do not come after the style code")
	}
}

func TestFormatSize(t *testing.T) {
	for b, want := range map[int64]string{0: "0 B", 1023: "1023 B", 1024: "1 KB", 1536: "2 KB", 50208: "49 KB",
		1<<20 - 1: "1024 KB", 1 << 20: "1.0 MB", 2_411_724: "2.3 MB"} {
		if got := formatSize(b); got != want {
			t.Errorf("formatSize(%d) = %s, want %s", b, got, want)
		}
	}
}

func TestReport(t *testing.T) {
	dir := t.TempDir()
	// The page count comes from pdfpack, or from the page tree when pdfpack could not read the PDF.
	for _, tt := range []struct {
		content string
		pages   int
		want    string
	}{
		{"%PDF-1.7\n1 0 obj\n<</Type/Pages/Count 3/Kids[2 0 R]>>\n", 0, "(3 pages, %d B)"},
		{"%PDF-1.7\n<< /Type /Pages /Count 1 /Kids [2 0 R] >>\n", 0, "(1 page, %d B)"},
		{"%PDF-1.7\nno page tree\n", 0, "(%d B)"},
		{"%PDF-1.7\ncompressed\n", 12, "(12 pages, %d B)"},
	} {
		content, want := tt.content, tt.want
		file := filepath.Join(dir, "out.pdf")
		if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		var stderr bytes.Buffer
		report(&stderr, file, tt.pages)
		if line := "setdown: wrote " + file + " " + fmt.Sprintf(want, len(content)) + "\n"; stderr.String() != line {
			t.Errorf("report: %q, want %q", stderr.String(), line)
		}
	}
}

// sandbox points the places setdown and Typst write to into a test's own directories and returns a working directory
// with a copy of the overview example as example.md.
func sandbox(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	for _, dir := range []string{"work", "cache", "tmp", "typst-packages"} {
		if err := os.Mkdir(filepath.Join(base, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("SETDOWN_CACHE", filepath.Join(base, "cache"))
	t.Setenv("TMPDIR", filepath.Join(base, "tmp"))
	t.Setenv("XDG_RUNTIME_DIR", "")
	// Typst would download a package it does not find in --package-path into this cache.
	t.Setenv("TYPST_PACKAGE_CACHE_PATH", filepath.Join(base, "typst-packages"))
	example, err := os.ReadFile(filepath.Join("..", "..", "examples", "01-overview", "overview.md"))
	if err != nil {
		t.Fatal(err)
	}
	work := filepath.Join(base, "work")
	if err := os.WriteFile(filepath.Join(work, "example.md"), example, 0o644); err != nil {
		t.Fatal(err)
	}
	return work
}

// isEmpty reports whether a directory has no entries.
func isEmpty(t *testing.T, dir string) bool {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	return len(entries) == 0
}

// pdfWant is what a conversion of the overview example should look like.
type pdfWant struct {
	pages         int
	width, height float64 // in pt
	family        string  // of the body text, as in the fonts' PostScript names
	mono          string  // of code, likewise
	size          float64 // of the body text, in pt
	left          float64 // the margin, in pt
	colored       bool
}

const (
	a4Width, a4Height = 595.28, 841.89
	a5Width, a5Height = 419.53, 595.28
	mm20, mm15        = 56.69, 42.52
	jbMono            = "JetBrainsMono"
	// The glyphs the overview comes to, give or take hyphens, ligatures and the byline the front matter variant lacks.
	overviewGlyphs = 2070
)

// TestConvert converts the overview example with each option, and with front matter that sets them, and checks what
// a reader would notice rather than bytes, which may differ between platforms without anything being wrong.
func TestConvert(t *testing.T) {
	work := sandbox(t)
	base := filepath.Dir(work)
	in := filepath.Join(work, "example.md")
	example, err := os.ReadFile(in)
	if err != nil {
		t.Fatal(err)
	}
	// The overview without its own front matter, under front matter that sets everything the options do.
	_, body, ok := strings.Cut(strings.TrimPrefix(string(example), "---\n"), "\n---\n")
	if !ok {
		t.Fatal("the overview has no front matter")
	}
	fm := filepath.Join(work, "front-matter.md")
	settings := "---\ntitle: Front Matter\nfont: sans\nscale: 0.95\npaper: a5\nmargin: 15mm\nhighlight: true\n---\n"
	if err := os.WriteFile(fm, []byte(settings+body), 0o644); err != nil {
		t.Fatal(err)
	}

	// Fonts that setdown does not bundle: Source Sans 3 renamed, in a directory of its own, and DejaVu Sans Mono,
	// which is built into Typst.
	fonts := customFonts(t, base)
	custom := filepath.Join(work, "custom-fonts.md")
	settings = "---\nfont: Custom Sans 3\nmono-font: DejaVu Sans Mono\n---\n"
	if err := os.WriteFile(custom, []byte(settings+body), 0o644); err != nil {
		t.Fatal(err)
	}

	defaults := pdfWant{3, a4Width, a4Height, "SourceSerif4", jbMono, 11, mm20, false}
	tests := []struct {
		name string
		args []string
		want pdfWant
	}{
		{"defaults", []string{in}, defaults},
		{"sans", []string{"--sans", in}, pdfWant{3, a4Width, a4Height, "SourceSans3", jbMono, 11, mm20, false}},
		{"highlight", []string{"--highlight", in}, pdfWant{3, a4Width, a4Height, "SourceSerif4", jbMono, 11, mm20, true}},
		{"scale", []string{"--scale=90%", in}, pdfWant{3, a4Width, a4Height, "SourceSerif4", jbMono, 9.9, mm20, false}},
		{"paper", []string{"--paper", "a5", in}, pdfWant{4, a5Width, a5Height, "SourceSerif4", jbMono, 11, mm20, false}},
		{"margin", []string{"-m", "15mm", in}, pdfWant{3, a4Width, a4Height, "SourceSerif4", jbMono, 11, mm15, false}},
		{"lang", []string{"--lang", "de", in}, defaults},
		{"front matter", []string{fm}, pdfWant{3, a5Width, a5Height, "SourceSans3", jbMono, 10.45, mm15, true}},
		{"options win", []string{"--serif", "--no-highlight", fm},
			pdfWant{3, a5Width, a5Height, "SourceSerif4", jbMono, 10.45, mm15, false}},
		{"font", []string{"--font-path", fonts, "--font", "Custom Sans 3", in},
			pdfWant{3, a4Width, a4Height, "CustomSans3", jbMono, 11, mm20, false}},
		{"mono font", []string{"--mono-font", "DejaVu Sans Mono", in},
			pdfWant{3, a4Width, a4Height, "SourceSerif4", "DejaVuSansMono", 11, mm20, false}},
		{"fonts from front matter", []string{"--font-path=" + fonts, custom},
			pdfWant{3, a4Width, a4Height, "CustomSans3", "DejaVuSansMono", 11, mm20, false}},
		{"font options win", []string{"--font-path=" + fonts, "--serif", "--mono-font", "JetBrains Mono", custom},
			pdfWant{3, a4Width, a4Height, "SourceSerif4", jbMono, 11, mm20, false}},
	}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := filepath.Join(work, fmt.Sprintf("out-%d.pdf", i))
			var stdout, stderr bytes.Buffer
			code, sig := run(append(tt.args, out), &stdout, &stderr)
			if code != 0 || sig != nil {
				t.Fatalf("run(%q): exit %d, signal %v\n%s", tt.args, code, sig, stderr.String())
			}
			// On Windows and macOS, the first run says that it unpacks the cache first.
			if msg, want := stderr.String(), fmt.Sprintf("setdown: wrote %s (%d pages, ", out, tt.want.pages); !strings.Contains(msg, want) {
				t.Errorf("stderr %q, want a line starting with %q", msg, want)
			}
			// Typst warns about a font family it does not find.
			if strings.Contains(stderr.String(), "warning:") {
				t.Errorf("Typst warns:\n%s", stderr.String())
			}
			got, err := readPDF(out)
			if err != nil {
				t.Fatalf("reading the PDF: %v", err)
			}
			checkPDF(t, got, tt.want)
		})
	}
	if !isEmpty(t, filepath.Join(base, "typst-packages")) {
		t.Error("Typst downloaded a package instead of using the embedded cmarker")
	}
	if !isEmpty(t, filepath.Join(base, "tmp")) {
		t.Error("a run left files in the temporary directory")
	}
}

// customFonts writes Source Sans 3 in the four styles the overview needs, renamed to Custom Sans 3, into a directory
// below base and returns it: a family setdown does not bundle. The names are replaced by ones of the same length, so
// the tables keep their offsets; Typst does not check the tables' checksums.
func customFonts(t *testing.T, base string) string {
	t.Helper()
	dir := filepath.Join(base, "fonts")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	utf16 := func(s string) []byte {
		var b []byte
		for _, r := range s {
			b = append(b, 0, byte(r))
		}
		return b
	}
	for _, style := range []string{"Regular", "It", "Bold", "Semibold"} {
		name := "SourceSans3-" + style + ".otf"
		data, err := os.ReadFile(filepath.Join("..", "..", "internal", "payload", "assets", "files", "fonts",
			"source-sans", name))
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range [][2]string{{"Source Sans 3", "Custom Sans 3"}, {"SourceSans3", "CustomSans3"}} {
			data = bytes.ReplaceAll(data, []byte(r[0]), []byte(r[1]))
			data = bytes.ReplaceAll(data, utf16(r[0]), utf16(r[1]))
		}
		if bytes.Contains(data, []byte("SourceSans")) || bytes.Contains(data, utf16("SourceSans")) {
			t.Fatalf("%s still names Source Sans", name)
		}
		if err := os.WriteFile(filepath.Join(dir, "Custom"+name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// TestConvertStyle runs a style file from outside the Markdown file's directory: its rules win over the template's,
// it sees the template's definitions, and an error in it is traced back to its own lines.
func TestConvertStyle(t *testing.T) {
	work := sandbox(t)
	base := filepath.Dir(work)
	in := filepath.Join(work, "example.md")
	style := filepath.Join(base, "style.typ")
	code := "#assert.eq(meta.title, \"An Overview of setdown\")\n#assert.eq(setting(\"paper\", \"a4\"), \"a4\")\n" +
		"#set page(paper: \"a5\")\n#show heading: set text(fill: rgb(\"#1849a8\"))\n"
	if err := os.WriteFile(style, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(work, "styled.pdf")
	var stdout, stderr bytes.Buffer
	if code, sig := run([]string{"--style", style, in, out}, &stdout, &stderr); code != 0 || sig != nil {
		t.Fatalf("exit %d, signal %v\n%s", code, sig, stderr.String())
	}
	got, err := readPDF(out)
	if err != nil {
		t.Fatalf("reading the PDF: %v", err)
	}
	checkPDF(t, got, pdfWant{4, a5Width, a5Height, "SourceSerif4", jbMono, 11, mm20, true})

	// An error in the third line of the style file.
	if err := os.WriteFile(style, []byte("#set text(fill: blue)\n\n#no-such-function()"), 0o644); err != nil {
		t.Fatal(err)
	}
	stderr.Reset()
	if code, _ := run([]string{"--style", style, in, out}, &stdout, &stderr); code != 1 {
		t.Fatalf("exit %d, want Typst's 1\n%s", code, stderr.String())
	}
	_, line := templateWith([]byte("x"))
	for _, want := range []string{fmt.Sprintf("<stdin>:%d:", line+2), fmt.Sprintf("setdown: in Typst's messages, "+
		"lines %d to %d of <stdin> are lines 1 to 3 of %s\n", line, line+2, style)} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("stderr lacks %q:\n%s", want, stderr.String())
		}
	}
}

// TestMargins sets each side's margin on its own, from the command line and from the front matter, and checks where
// the text starts against a run with the default margin.
func TestMargins(t *testing.T) {
	work := sandbox(t)
	in := filepath.Join(work, "example.md")
	example, err := os.ReadFile(in)
	if err != nil {
		t.Fatal(err)
	}
	fm := filepath.Join(work, "margins.md")
	if err := os.WriteFile(fm, bytes.Replace(example, []byte("---\n"), []byte("---\nmargin: 35mm 15mm\n"), 1),
		0o644); err != nil {
		t.Fatal(err)
	}
	convert := func(args ...string) pdfProps {
		t.Helper()
		out := filepath.Join(work, "margins.pdf")
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
	near := func(a, b float64) bool { return math.Abs(a-b) <= 0.5 }
	base := convert(in)
	const mm = 72 / 25.4
	for _, tt := range []struct {
		args      []string
		top, left float64 // in mm
	}{
		{[]string{"--margin", "30mm 20mm 25mm 10mm", in}, 30, 10},
		{[]string{"--margin=35mm 15mm", in}, 35, 15},
		{[]string{"--margin", "1in 15mm 20mm", in}, 25.4, 15},
		{[]string{fm}, 35, 15},
	} {
		got := convert(tt.args...)
		if !near(got.left, tt.left*mm) {
			t.Errorf("%q: text starts %.2f pt from the left, want %.2f", tt.args, got.left, tt.left*mm)
		}
		// The first baseline moves down by as much as the top margin grows.
		if want := base.top + (tt.top-20)*mm; !near(got.top, want) {
			t.Errorf("%q: first baseline %.2f pt from the top, want %.2f", tt.args, got.top, want)
		}
	}
}

func TestConvertFails(t *testing.T) {
	work := sandbox(t)
	var stdout, stderr bytes.Buffer
	out := filepath.Join(work, "bad.pdf")
	code, sig := run([]string{"--paper", "no-such-paper", filepath.Join(work, "example.md"), out}, &stdout, &stderr)
	if code != 1 || sig != nil {
		t.Errorf("exit %d, signal %v; want Typst's exit code 1", code, sig)
	}
	if msg := stderr.String(); !strings.Contains(msg, "error:") || strings.Contains(msg, "wrote") {
		t.Errorf("stderr %q: want Typst's error and no report", msg)
	}
	if _, err := os.Stat(out); err == nil {
		t.Error("a PDF was written")
	}
	if !isEmpty(t, filepath.Join(filepath.Dir(work), "tmp")) {
		t.Error("a failed run left files in the temporary directory")
	}
}

func TestUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code, _ := run(nil, &stdout, &stderr); code != 1 || !strings.HasPrefix(stderr.String(), "Usage: setdown") {
		t.Errorf("no arguments: exit %d, stderr %q", code, stderr.String())
	}
	stdout.Reset()
	if code, _ := run([]string{"--help"}, &stdout, &stderr); code != 0 || !strings.HasPrefix(stdout.String(), "Usage: setdown") {
		t.Errorf("--help: exit %d, stdout %q", code, stdout.String())
	}
	// The usage names this system's config file.
	t.Setenv("SETDOWN_CONFIG", filepath.Join("my", "setdown.yaml"))
	stdout.Reset()
	if run([]string{"--help"}, &stdout, &stderr); !strings.Contains(stdout.String(), "here "+filepath.Join("my",
		"setdown.yaml")+",\n") || strings.Contains(stdout.String(), "%CONFIG%") {
		t.Errorf("--help does not name the config file:\n%s", stdout.String())
	}
}

func TestVersionAndLicenses(t *testing.T) {
	m, err := payload.ReadManifest()
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code, _ := run([]string{"--version"}, &stdout, &stderr); code != 0 {
		t.Fatalf("--version: exit %d, %s", code, stderr.String())
	}
	for _, want := range []string{"setdown " + version, "Typst " + m.Typst, "Source Serif ", "Source Sans ", "JetBrains Mono ",
		"Noto Emoji ", "cmarker " + m.Cmarker} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("--version lacks %q:\n%s", want, stdout.String())
		}
	}
	stdout.Reset()
	if code, _ := run([]string{"--licenses"}, &stdout, &stderr); code != 0 {
		t.Fatalf("--licenses: exit %d, %s", code, stderr.String())
	}
	for _, want := range []string{"MIT License", "Patrick Bergner", "Apache License", "SIL OPEN FONT LICENSE Version 1.1",
		"Sabrina Jewson", "The Go Authors", "Kirill Simonov"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("--licenses lacks %q", want)
		}
	}
	// setdown's, Typst's license and notice, one per font family, cmarker's, then Go's and its modules' (two or more).
	if n := strings.Count(stdout.String(), "\n==> "); n != len(m.Licenses) || n < 1+2+len(m.Fonts)+1+3 {
		t.Errorf("--licenses shows %d texts, the manifest lists %d", n, len(m.Licenses))
	}
}
