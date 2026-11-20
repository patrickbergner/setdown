package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// threePages is a document of three pages, each with one line of text.
const threePages = "First page.\n\n<!--raw-typst #pagebreak()-->\n\nSecond page.\n\n<!--raw-typst #pagebreak()-->\n\nThird page.\n"

// footers returns what each page shows besides its line of text: the page number, if any.
func footers(pages [][]string) []string {
	var out []string
	for _, runs := range pages {
		var footer []string
		for _, r := range runs {
			if !strings.HasSuffix(r, " page.") {
				footer = append(footer, r)
			}
		}
		out = append(out, strings.Join(footer, " "))
	}
	return out
}

func withFrontMatter(setting, body string) string {
	return "---\npage-numbers: " + setting + "\n---\n\n" + body
}

func TestPageNumbers(t *testing.T) {
	tests := []struct {
		name string
		doc  string
		args []string
		want []string
	}{
		{"default", threePages, nil, []string{"1 / 3", "2 / 3", "3 / 3"}},
		{"page only", withFrontMatter("1", threePages), nil, []string{"1", "2", "3"}},
		{"of", withFrontMatter("1 of 5", threePages), nil, []string{"1 of 3", "2 of 3", "3 of 3"}},
		{"hyphen", withFrontMatter("1-5", threePages), nil, []string{"1-3", "2-3", "3-3"}},
		{"words", withFrontMatter("Page 1 of 5", threePages), nil, []string{"Page 1 of 3", "Page 2 of 3", "Page 3 of 3"}},
		{"other words", withFrontMatter("Seite 1 von 5", threePages), nil, []string{"Seite 1 von 3", "Seite 2 von 3", "Seite 3 von 3"}},
		{"dashes", withFrontMatter("– 1 –", threePages), nil, []string{"– 1 –", "– 2 –", "– 3 –"}},
		{"none", withFrontMatter("none", threePages), nil, []string{"", "", ""}},
		{"false", withFrontMatter("false", threePages), nil, []string{"", "", ""}},
		{"option", threePages, []string{"--page-numbers", "none"}, []string{"", "", ""}},
		{"option wins", withFrontMatter("none", threePages), []string{"--page-numbers=1 of 9"}, []string{"1 of 3", "2 of 3", "3 of 3"}},
		{"one page", withFrontMatter("Page 1 of 5", "A single page.\n"), nil, []string{""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pages := convertText(t, tt.doc, tt.args...)
			if tt.name == "one page" {
				if len(pages) != 1 || len(pages[0]) != 1 {
					t.Errorf("text %q, want the line alone", pages)
				}
				return
			}
			if got := footers(pages); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("footers %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPageNumbersInvalid(t *testing.T) {
	work := sandbox(t)
	in := filepath.Join(work, "doc.md")
	for _, tt := range []struct {
		doc  string
		args []string
	}{
		{threePages, []string{"--page-numbers", "Page"}},  // no number: caught by setdown
		{threePages, []string{"--page-numbers", "1 2 3"}}, // too many
		{withFrontMatter("Page x", threePages), nil},      // caught by the template
		{withFrontMatter("1 of 2 of 3", threePages), nil}, // too many
	} {
		if err := os.WriteFile(in, []byte(tt.doc), 0o644); err != nil {
			t.Fatal(err)
		}
		var stdout, stderr bytes.Buffer
		code, _ := run(append(tt.args, in), &stdout, &stderr)
		if code == 0 || !strings.Contains(stderr.String(), "page-numbers must be none or show a number for the page") {
			t.Errorf("%q %q: exit %d, stderr %q", tt.args, tt.doc[:12], code, stderr.String())
		}
	}
}
