package main

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// allText joins the text of every page, run by run.
func allText(pages [][]string) string {
	var runs []string
	for _, page := range pages {
		runs = append(runs, page...)
	}
	return strings.Join(runs, " ")
}

const contentsDoc = `---
title: Report
%s---

## Alpha

Text.

### Beta

More text.

#### Gamma

Even more.

## Delta

The end.
`

// TestTableOfContents turns the table of contents on and off from the front matter and the options, with its depth,
// and checks which headings it lists: each listed heading shows twice, once there and once in the text.
func TestTableOfContents(t *testing.T) {
	for _, tt := range []struct {
		name, frontMatter string
		args              []string
		want              map[string]int
	}{
		{"none", "", nil, map[string]int{"Contents": 0, "Alpha": 1, "Beta": 1}},
		{"front matter", "toc: true\n", nil, map[string]int{"Contents": 1, "Alpha": 2, "Beta": 2, "Gamma": 1, "Delta": 2}},
		{"option", "", []string{"--toc"}, map[string]int{"Contents": 1, "Alpha": 2, "Beta": 2, "Gamma": 1}},
		{"depth", "toc: 2\n", nil, map[string]int{"Contents": 1, "Alpha": 2, "Beta": 1, "Delta": 2}},
		{"option depth", "toc: true\n", []string{"--toc=4"}, map[string]int{"Alpha": 2, "Beta": 2, "Gamma": 2}},
		{"option off", "toc: true\n", []string{"--no-toc"}, map[string]int{"Contents": 0, "Alpha": 1}},
		{"language", "toc: yes\nlang: de\n", nil, map[string]int{"Inhaltsverzeichnis": 1, "Alpha": 2}},
		{"title stays out", "toc: 6\n", nil, map[string]int{"Report": 1, "Gamma": 2}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			text := allText(convertText(t, strings.Replace(contentsDoc, "%s", tt.frontMatter, 1), tt.args...))
			for word, n := range tt.want {
				if got := strings.Count(text, word); got != n {
					t.Errorf("%s shows %d times, want %d: %q", word, got, n, text)
				}
			}
		})
	}
}

// TestHeadingNumbering numbers the headings from the document's top level on, here ##, in the text and in the table
// of contents; the title stays unnumbered.
func TestHeadingNumbering(t *testing.T) {
	for _, tt := range []struct {
		name, frontMatter string
		args              []string
		want              []string // in order
		not               []string
	}{
		{"none", "", nil, []string{"Alpha", "Beta"}, []string{"1.1", "0."}},
		{"front matter", "heading-numbering: \"1.1\"\n", nil,
			[]string{"1", "Alpha", "1.1", "Beta", "1.1.1", "Gamma", "2", "Delta"}, []string{"0.", "1 Report"}},
		{"a number in YAML", "heading-numbering: 1.1\n", nil, []string{"1.1", "Beta"}, []string{"0."}},
		{"option", "", []string{"--heading-numbering", "I.a"}, []string{"I", "Alpha", "I.a", "Beta", "II", "Delta"}, nil},
		{"option off", "heading-numbering: 1.1\n", []string{"--heading-numbering", "none"}, []string{"Alpha"},
			[]string{"1.1"}},
		{"in the contents", "toc: true\nheading-numbering: 1.\n", nil,
			[]string{"Contents", "1.", "Alpha", "1.1.", "Beta", "2.", "Delta", "1.", "Alpha"}, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			text := allText(convertText(t, strings.Replace(contentsDoc, "%s", tt.frontMatter, 1), tt.args...))
			rest := text
			for _, want := range tt.want {
				i := strings.Index(rest, want)
				if i < 0 {
					t.Fatalf("%q missing or out of order in %q", want, text)
				}
				rest = rest[i+len(want):]
			}
			for _, not := range tt.not {
				if strings.Contains(text, not) {
					t.Errorf("%q in %q", not, text)
				}
			}
		})
	}
}

func TestHeadingNumberingInvalid(t *testing.T) {
	work := sandbox(t)
	in := filepath.Join(work, "doc.md")
	if err := os.WriteFile(in, []byte(strings.Replace(contentsDoc, "%s", "", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"--heading-numbering", "§"}, {"--toc=7"}, {"--toc=deep"}} {
		var stdout, stderr bytes.Buffer
		if code, _ := run(append(args, in), &stdout, &stderr); code == 0 {
			t.Errorf("%q: exit 0, want an error", args)
		}
	}
	// A value from the front matter only the template checks.
	if err := os.WriteFile(in, []byte(strings.Replace(contentsDoc, "%s", "toc: lots\n", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code, _ := run([]string{in}, &stdout, &stderr); code == 0 || !strings.Contains(stderr.String(), "toc must be") {
		t.Errorf("toc: lots: exit %d, stderr %q", code, stderr.String())
	}
}

// TestLandscape turns the page sideways from the options and the front matter.
func TestLandscape(t *testing.T) {
	work := sandbox(t)
	in := filepath.Join(work, "example.md")
	example, err := os.ReadFile(in)
	if err != nil {
		t.Fatal(err)
	}
	fm := filepath.Join(work, "landscape.md")
	if err := os.WriteFile(fm, bytes.Replace(example, []byte("---\n"), []byte("---\nlandscape: true\npaper: a5\n"), 1),
		0o644); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		args          []string
		width, height float64
	}{
		{[]string{in}, a4Width, a4Height},
		{[]string{"--landscape", in}, a4Height, a4Width},
		{[]string{"--landscape", "--portrait", in}, a4Width, a4Height},
		{[]string{fm}, a5Height, a5Width},
		{[]string{"--portrait", fm}, a5Width, a5Height},
	} {
		out := filepath.Join(work, "out.pdf")
		var stdout, stderr bytes.Buffer
		if code, sig := run(append(tt.args, out), &stdout, &stderr); code != 0 || sig != nil {
			t.Fatalf("run(%q): exit %d, signal %v\n%s", tt.args, code, sig, stderr.String())
		}
		p, err := readPDF(out)
		if err != nil {
			t.Fatal(err)
		}
		if p.width < tt.width-0.5 || p.width > tt.width+0.5 || p.height < tt.height-0.5 || p.height > tt.height+0.5 {
			t.Errorf("%q: pages of %.2f×%.2f pt, want %.2f×%.2f", tt.args, p.width, p.height, tt.width, tt.height)
		}
		if p.missing > 0 {
			t.Errorf("%q: %d glyphs missing", tt.args, p.missing)
		}
	}
}

// TestEmoji sets emoji in text, a heading, a table, code and a list from Noto Emoji, sequences included, while
// symbols that the text fonts have stay in those.
func TestEmoji(t *testing.T) {
	work := sandbox(t)
	convert := func(markdown string) pdfProps {
		t.Helper()
		in := filepath.Join(work, "emoji.md")
		if err := os.WriteFile(in, []byte(markdown), 0o644); err != nil {
			t.Fatal(err)
		}
		var stdout, stderr bytes.Buffer
		if code, sig := run([]string{in}, &stdout, &stderr); code != 0 || sig != nil {
			t.Fatalf("exit %d, signal %v\n%s", code, sig, stderr.String())
		}
		if strings.Contains(stderr.String(), "warning:") {
			t.Errorf("Typst warns:\n%s", stderr.String())
		}
		p, err := readPDF(filepath.Join(work, "emoji.pdf"))
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	hasEmojiFont := func(p pdfProps) bool {
		return slices.ContainsFunc(p.fonts, func(f string) bool { return strings.HasPrefix(f, "NotoEmoji") })
	}

	p := convert("# Release 🚀\n\nDone ✅, failed ❌, careful ⚠️, love ❤️, a family 👨‍👩‍👧, a thumb 👍🏽, flags 🇩🇪 🇯🇵, " +
		"a keycap 1️⃣ and *bold 🎉*.\n\n| State | |\n|---|---|\n| ok | ✅ |\n\n- 🍕 pizza\n\n```\nfix 🐛\n```\n\n" +
		"Inline `code 🔥` too.\n")
	if !hasEmojiFont(p) {
		t.Errorf("no Noto Emoji among %v", p.fonts)
	}
	if p.missing > 0 {
		t.Errorf("%d glyphs missing", p.missing)
	}
	for _, font := range p.fonts {
		family, _, _ := strings.Cut(font, "-")
		if !slices.Contains([]string{"SourceSerif4", "JetBrainsMono", "NotoEmoji"}, family) {
			t.Errorf("unexpected font %s", font)
		}
	}

	// Symbols asked to look like an emoji, and joined emoji made of them, come from Noto Emoji as a whole, not split
	// between it and the fonts that have the symbol (Libertinus Serif, Source Sans 3).
	p = convert("❤️ ✔️ ☀️ ©️ ⚠️ 🧑‍⚕️ 🏳️‍🌈 👁️‍🗨️\n")
	for _, font := range p.fonts {
		if family, _, _ := strings.Cut(font, "-"); family != "NotoEmoji" && family != "SourceSerif4" {
			t.Errorf("emoji sequences: unexpected font %s", font)
		}
	}
	if !hasEmojiFont(p) || p.missing > 0 {
		t.Errorf("emoji sequences: fonts %v, %d missing", p.fonts, p.missing)
	}

	// Symbols that are no emoji by default keep to the text fonts.
	if p := convert("Stars ★, ticks ✓, © 2026, Brand™, arrows → ↔, and ✗.\n"); hasEmojiFont(p) || p.missing > 0 {
		t.Errorf("fonts %v, %d missing; want no Noto Emoji", p.fonts, p.missing)
	}
}
