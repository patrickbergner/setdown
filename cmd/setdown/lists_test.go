package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// convertText converts Markdown with the given options and returns the text of each page of the PDF.
func convertText(t *testing.T, markdown string, args ...string) [][]string {
	t.Helper()
	work := sandbox(t)
	in := filepath.Join(work, "doc.md")
	if err := os.WriteFile(in, []byte(markdown), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code, sig := run(append(args, in), &stdout, &stderr); code != 0 || sig != nil {
		t.Fatalf("exit %d, signal %v\n%s", code, sig, stderr.String())
	}
	if strings.Contains(stderr.String(), "warning:") {
		t.Errorf("Typst warns:\n%s", stderr.String())
	}
	p, err := readPDF(filepath.Join(work, "doc.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	return p.text
}

// markers returns the runs of text that are list markers, in order.
func markers(pages [][]string, candidates ...string) []string {
	var found []string
	for _, runs := range pages {
		for _, r := range runs {
			for _, c := range candidates {
				if r == c {
					found = append(found, r)
				}
			}
		}
	}
	return found
}

const markersDoc = `---
bullet: •
numbering: 1)
---

From the front matter:

- one
  - nested

1. first
   1. nested

<!--raw-typst #set enum(numbering: "a)")-->

From here on lettered:

1. alpha
2. beta

<!--raw-typst #set enum(numbering: "1.a.i.")-->

1. one
   1. two
      1. three
      2. three again

<!--raw-typst #set list(marker: ([▸], [–]))-->

- arrow
  - dash

- outer
  <!--raw-typst #set list(marker: [★])-->
  - only in this item
- outer again
  - back to the level's marker
`

func TestListMarkers(t *testing.T) {
	got := markers(convertText(t, markersDoc), "•", "–", "▸", "★", "1)", "a)", "b)", "1.", "a.", "i.", "ii.")
	want := []string{
		"•", "•", "1)", "1)", // the front matter's bullet and numbering, at every level
		"a)", "b)", // a set rule in a comment, from there on
		"1.", "a.", "i.", "ii.", // a pattern with a counting symbol per level
		"▸", "–", // a marker per level
		"▸", "★", "▸", "–", // a set rule inside an item applies to that item only
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("markers %q\nwant    %q", got, want)
	}
}

func TestListMarkerOptions(t *testing.T) {
	doc := "---\nbullet: •\nnumbering: i.\n---\n\n- one\n\n1. first\n"
	got := markers(convertText(t, doc, "--bullet", "▸", "--numbering=(a)"), "•", "▸", "i.", "(a)")
	if want := []string{"▸", "(a)"}; !reflect.DeepEqual(got, want) {
		t.Errorf("markers %q, want %q: the options win over the front matter", got, want)
	}
}

func TestListMarkerErrors(t *testing.T) {
	work := sandbox(t)
	in := filepath.Join(work, "doc.md")
	if err := os.WriteFile(in, []byte("---\nnumbering: 1.\n---\n\n1. one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	// YAML reads an unquoted 1. as a number.
	if code, _ := run([]string{in}, &stdout, &stderr); code == 0 || !strings.Contains(stderr.String(), "numbering must be a pattern in quotes") {
		t.Errorf("exit %d, stderr %q", code, stderr.String())
	}
}

// A paragraph ending in a colon stays on the page of the list it introduces, with and without a set rule between
// them. Each case pushes its lead-in further down the page, so some of them land at its foot.
func TestLeadInStaysWithList(t *testing.T) {
	var doc strings.Builder
	cases := 0
	for space := 600; space <= 730; space += 5 {
		for _, rule := range []string{"", "<!--raw-typst #set enum(numbering: \"1.\")-->\n\n"} {
			cases++
			fmt.Fprintf(&doc, "<!--raw-typst #block(width: 100%%, height: %dpt)-->\n\nLead %d:\n\n%s1. item %d\n2. second\n3. third\n\n"+
				"<!--raw-typst #pagebreak()-->\n\n", space, cases, rule, cases)
		}
	}
	pages := convertText(t, doc.String())
	pageOf := func(prefix string) int {
		for i, runs := range pages {
			for _, r := range runs {
				if strings.HasPrefix(r, prefix) {
					return i + 1
				}
			}
		}
		return 0
	}
	for c := 1; c <= cases; c++ {
		lead, item := pageOf(fmt.Sprintf("Lead %d:", c)), pageOf(fmt.Sprintf("item %d", c))
		if lead == 0 || lead != item {
			t.Errorf("case %d: the lead-in is on page %d, its list starts on page %d", c, lead, item)
		}
	}
}
