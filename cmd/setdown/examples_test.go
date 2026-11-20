package main

import (
	"bytes"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestExamples renders every example and compares the result with the PDF committed next to it, by what a reader
// would notice: pages, paper, fonts, the amount and size of the text, margin and color. A change to an example, the
// template or a pinned version that changes any of these needs the PDFs rendered again with ./render-examples.sh.
// An example with an options.txt is rendered with the options in it (one line, split at spaces, paths relative to the
// example's directory, where setdown runs), as render-examples.sh and CI do. An example with a style.typ must show it
// verbatim, and one at a --pdf-level of a standard, or an e-invoice, must carry the standard's marker.
func TestExamples(t *testing.T) {
	sandbox(t)
	dirs, err := filepath.Glob(filepath.Join("..", "..", "examples", "[0-9][0-9]-*"))
	if err != nil || len(dirs) == 0 {
		t.Fatalf("no examples found (%v)", err)
	}
	for _, dir := range dirs {
		example := filepath.Base(dir)
		name := example[len("00-"):]
		// Absolute, since each run changes into its copy of the example.
		dir, err := filepath.Abs(dir)
		if err != nil {
			t.Fatal(err)
		}
		t.Run(example, func(t *testing.T) {
			want, err := readPDF(filepath.Join(dir, name+".pdf"))
			if err != nil {
				t.Fatalf("the committed PDF: %v (render it with ./render-examples.sh)", err)
			}
			// A copy, so that the test writes nothing into the repository.
			work := filepath.Join(t.TempDir(), example)
			if err := os.CopyFS(work, os.DirFS(dir)); err != nil {
				t.Fatal(err)
			}
			out := filepath.Join(work, name+".pdf")
			if err := os.Remove(out); err != nil {
				t.Fatal(err)
			}
			var args []string
			if opts, err := os.ReadFile(filepath.Join(work, "options.txt")); err == nil {
				args = strings.Fields(string(opts))
			}
			args = append(args, name+".md")
			t.Chdir(work)
			if style, err := os.ReadFile(filepath.Join(work, "style.typ")); err == nil {
				// The example shows its style file, so the two must not drift apart.
				md, err := os.ReadFile(filepath.Join(work, name+".md"))
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Contains(md, style) {
					t.Errorf("%s.md does not show style.typ as it is", name)
				}
			}
			var stdout, stderr bytes.Buffer
			if code, sig := run(args, &stdout, &stderr); code != 0 || sig != nil {
				t.Fatalf("exit %d, signal %v\n%s", code, sig, stderr.String())
			}
			// An example shows what works; one that makes Typst warn shows something that may soon stop working.
			if strings.Contains(stderr.String(), "warning:") {
				t.Errorf("Typst warns:\n%s", stderr.String())
			}
			got, err := readPDF(out)
			if err != nil {
				t.Fatalf("the new PDF: %v", err)
			}
			compareExample(t, got, want)
			markers := map[string]string{"a3b": "<pdfaid:part>3</pdfaid:part>", "ua1": "<pdfuaid:part>1</pdfuaid:part>",
				"e-invoice": "<fx:DocumentType>INVOICE</fx:DocumentType>"}
			for i, a := range args {
				key := ""
				if a == "--pdf-level" && i+1 < len(args) {
					key = args[i+1]
				} else if a == "--invoice" {
					key = "e-invoice"
				}
				if markers[key] == "" {
					continue
				}
				for _, file := range []string{out, filepath.Join(dir, name+".pdf")} {
					if !bytes.Contains(pdfContent(t, file), []byte(markers[key])) {
						t.Errorf("%s lacks %s", file, markers[key])
					}
				}
			}
		})
	}
	// examples/ holds nothing but NN-name directories, each with name.md and name.pdf in it.
	entries, err := os.ReadDir(filepath.Join("..", "..", "examples"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !(e.IsDir() && len(e.Name()) > 3 && strings.Contains("0123456789", e.Name()[:1])) {
			t.Errorf("examples/%s is not an NN-name directory", e.Name())
		}
	}
	for _, dir := range dirs {
		name := filepath.Base(dir)[len("00-"):]
		if name != strings.ToLower(name) || strings.ContainsAny(name, " _") {
			t.Errorf("%s: names are lowercase kebab case", dir)
		}
		_ = fs.WalkDir(os.DirFS(dir), ".", func(path string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() && strings.HasSuffix(path, ".md") && path != name+".md" {
				t.Errorf("%s: %s is not the example's %s.md", dir, path, name)
			}
			return nil
		})
	}
}

// compareExample compares a fresh render with the committed one. The glyph count may move a little with a new
// version of Typst or the fonts (hyphens, ligatures), the rest may not.
func compareExample(t *testing.T, got, want pdfProps) {
	t.Helper()
	near := func(a, b, tolerance float64) bool { return math.Abs(a-b) <= tolerance }
	if got.missing > 0 {
		t.Errorf("%d glyphs missing from the fonts", got.missing)
	}
	if got.pages != want.pages || !near(got.width, want.width, 0.5) || !near(got.height, want.height, 0.5) {
		t.Errorf("%d pages of %.2f×%.2f pt, the committed PDF has %d of %.2f×%.2f", got.pages, got.width, got.height,
			want.pages, want.width, want.height)
	}
	if !reflect.DeepEqual(got.fonts, want.fonts) {
		t.Errorf("fonts %v, the committed PDF has %v", got.fonts, want.fonts)
	}
	for font, size := range want.sizes {
		if !near(got.sizes[font], size, 0.01) {
			t.Errorf("%s in %.2f pt, the committed PDF has %.2f", font, got.sizes[font], size)
		}
	}
	if !near(float64(got.glyphs), float64(want.glyphs), float64(want.glyphs)/50) {
		t.Errorf("%d glyphs, the committed PDF has %d", got.glyphs, want.glyphs)
	}
	if !near(got.bodySize, want.bodySize, 0.01) || !near(got.left, want.left, 0.5) || got.colored != want.colored {
		t.Errorf("body %.2f pt, left %.2f pt, colored %v; the committed PDF: body %.2f pt, left %.2f pt, colored %v",
			got.bodySize, got.left, got.colored, want.bodySize, want.left, want.colored)
	}
	// Images and backgrounds drawn as XObjects, by kind and place; object numbers may differ.
	for i := 0; i < min(len(got.drawn), len(want.drawn)); i++ {
		same := len(got.drawn[i]) == len(want.drawn[i])
		for k := 0; same && k < len(got.drawn[i]); k++ {
			g, w := got.drawn[i][k], want.drawn[i][k]
			same = g.subtype == w.subtype && near(g.x0, w.x0, 0.5) && near(g.y0, w.y0, 0.5) && near(g.x1, w.x1, 0.5) &&
				near(g.y1, w.y1, 0.5)
		}
		if !same {
			t.Errorf("page %d draws %+v, the committed PDF %+v", i+1, got.drawn[i], want.drawn[i])
		}
	}
	if t.Failed() {
		t.Logf("new:       %v", got)
		t.Logf("committed: %v", want)
	}
}
