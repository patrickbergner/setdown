package payload

import (
	"strings"
	"testing"
)

// The tests need the assets of the platform at hand: go run ./tools/fetch -target <os>/<arch>, or ./run-tests.sh.

func TestEmbedded(t *testing.T) {
	p, err := Embedded()
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, f := range p.Files {
		dir := f.Path[:strings.LastIndex(f.Path, "/")]
		if !strings.HasPrefix(dir, "fonts/") && !strings.HasPrefix(dir, "packages/preview/cmarker/") {
			t.Errorf("unexpected file %s", f.Path)
		}
		counts[dir]++
	}
	for _, dir := range []string{"fonts/source-serif", "fonts/source-sans", "fonts/jetbrains-mono"} {
		if counts[dir] < 4 { // regular, italic, bold and bold italic at least
			t.Errorf("%d files in %s", counts[dir], dir)
		}
	}
	if counts["fonts/noto-emoji"] != 1 { // one variable font
		t.Errorf("%d files in fonts/noto-emoji", counts["fonts/noto-emoji"])
	}
	if len(counts) != 5 || len(p.Typst) < 1<<20 || len(p.ID) != 8 {
		t.Errorf("payload: %v, Typst %d bytes, id %q", counts, len(p.Typst), p.ID)
	}
}

func TestManifestAndLicenses(t *testing.T) {
	m, err := ReadManifest()
	if err != nil {
		t.Fatal(err)
	}
	if m.Typst == "" || m.Cmarker == "" || len(m.Fonts) != 4 {
		t.Errorf("manifest: %+v", m)
	}
	// setdown's, Typst's license and notice, one per font family, cmarker's, then Go's and those of the Go modules
	// (a license for each, and a notice for some).
	if len(m.Licenses) < 1+2+len(m.Fonts)+1+3 || m.Licenses[0].File != "setdown-LICENSE" {
		t.Errorf("%d license texts", len(m.Licenses))
	}
	for _, l := range m.Licenses {
		if text, err := License(l.File); err != nil || len(text) < 500 {
			t.Errorf("%s: %d bytes, %v", l.Title, len(text), err)
		}
	}
	if !strings.Contains(string(Template), `#import "@preview/cmarker:`+m.Cmarker+`"`) {
		t.Errorf("the template does not import cmarker %s", m.Cmarker)
	}
}
