package pdfpack

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// build writes a PDF with a classic cross-reference table, as Typst does, from object bodies ("<<...>>" or a
// dictionary followed by "stream ... endstream").
func build(objects []string, trailer string) []byte {
	var b bytes.Buffer
	b.WriteString("%PDF-1.7\n%\x80\x80\x80\x80\n\n")
	offsets := make([]int, len(objects))
	for i, o := range objects {
		offsets[i] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n\n", i+1, o)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f\r\n", len(objects)+1)
	for _, off := range offsets {
		fmt.Fprintf(&b, "%010d 00000 n\r\n", off)
	}
	fmt.Fprintf(&b, "trailer\n%s\nstartxref\n%d\n%%%%EOF\n", trailer, xref)
	return b.Bytes()
}

// Values that a naive reader would get wrong: keywords and delimiters inside strings, comments, a stream whose data
// looks like PDF syntax.
var tricky = []string{
	"<</Type/Catalog/Pages 2 0 R/Metadata 5 0 R/Names<</Dests 4 0 R>>>>",
	"<</Type/Pages/Count 2/Kids[3 0 R 3 0 R]>>",
	"<</Type/Page/Parent 2 0 R/Title(endobj >> \\) stream \\\\ (nested (parens)) endstream)>>",
	"[(a) <414243> /Name#20x 1.5 -2 true null 3 0 R % a comment with endobj and )\n]",
	"<</Type/Metadata/Subtype/XML/Length 26>>\nstream\nendobj 1 0 obj << >> trailer\nendstream",
	"<</Length 12>>\nstream\nxref\n0 1\nendo\nendstream",
	"<</Title(Doc)/Producer<feff0041>>>",
	"42",
}

const trickyTrailer = "<</Size 9/Root 1 0 R/Info 7 0 R/ID[(a)(b)]>>"

// unpack reads a PDF Pack wrote: the cross-reference stream, the object streams and the objects at offsets.
func unpack(t *testing.T, data []byte) (bodies map[int]string, raws map[int]string, xrefDict string) {
	t.Helper()
	m := regexp.MustCompile(`startxref\n(\d+)\n%%EOF\n$`).FindSubmatch(data)
	if m == nil {
		t.Fatalf("no startxref at the end")
	}
	xref, _ := strconv.Atoi(string(m[1]))
	stream := func(at int) (dict string, content []byte) {
		obj := data[at:]
		s := bytes.Index(obj, []byte(">>\nstream\n"))
		dict = string(obj[:s+2])
		l := regexp.MustCompile(`/Length (\d+)`).FindStringSubmatch(dict)
		n, _ := strconv.Atoi(l[1])
		raw := obj[s+len(">>\nstream\n") : s+len(">>\nstream\n")+n]
		r, err := zlib.NewReader(bytes.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		content, err = io.ReadAll(r)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.HasPrefix(obj[s+len(">>\nstream\n")+n:], []byte("\nendstream\nendobj\n")) {
			t.Fatalf("object at %d: no endstream after /Length", at)
		}
		return dict, content
	}
	xrefDict, table := stream(xref)
	if !strings.Contains(xrefDict, "/Type/XRef") || !strings.Contains(xrefDict, "/W[1 4 2]") {
		t.Fatalf("not a cross-reference stream: %s", xrefDict)
	}
	bodies, raws = map[int]string{}, map[int]string{}
	stms := map[int][]string{}
	for n := 0; n*7 < len(table); n++ {
		row := table[n*7 : n*7+7]
		kind, f2, f3 := row[0], int(binary.BigEndian.Uint32(row[1:5])), int(binary.BigEndian.Uint16(row[5:7]))
		switch kind {
		case 1:
			if !bytes.HasPrefix(data[f2:], []byte(strconv.Itoa(n)+" 0 obj\n")) {
				t.Fatalf("object %d is not at %d", n, f2)
			}
			end := bytes.Index(data[f2:], []byte("\nendobj\n"))
			raws[n] = string(data[f2 : f2+end])
		case 2:
			if _, ok := stms[f2]; !ok {
				at := int(binary.BigEndian.Uint32(table[f2*7+1 : f2*7+5]))
				dict, content := stream(at)
				first, _ := strconv.Atoi(regexp.MustCompile(`/First (\d+)`).FindStringSubmatch(dict)[1])
				count, _ := strconv.Atoi(regexp.MustCompile(`/N (\d+)`).FindStringSubmatch(dict)[1])
				pairs := strings.Fields(string(content[:first]))
				if len(pairs) != 2*count {
					t.Fatalf("object stream %d: %d numbers for %d objects", f2, len(pairs), count)
				}
				var objs []string
				for i := 0; i < count; i++ {
					start, _ := strconv.Atoi(pairs[2*i+1])
					stop := len(content) - first
					if i+1 < count {
						stop, _ = strconv.Atoi(pairs[2*i+3])
					}
					objs = append(objs, pairs[2*i]+" "+string(content[first+start:first+stop]))
				}
				stms[f2] = objs
			}
			num, body, _ := strings.Cut(stms[f2][f3], " ")
			if num != strconv.Itoa(n) {
				t.Fatalf("object %d: index %d of object stream %d holds object %s", n, f3, f2, num)
			}
			bodies[n] = strings.TrimSuffix(body, "\n")
		}
	}
	return bodies, raws, xrefDict
}

func TestPack(t *testing.T) {
	in := build(tricky, trickyTrailer)
	for _, drop := range []bool{false, true} {
		out, res, err := Pack(in, Options{DropMetadata: drop})
		if err != nil {
			t.Fatal(err)
		}
		if res.Pages != 2 {
			t.Errorf("pages %d, want 2", res.Pages)
		}
		if again, _, _ := Pack(in, Options{DropMetadata: drop}); !bytes.Equal(out, again) {
			t.Error("two runs differ")
		}
		if !bytes.HasPrefix(out, []byte("%PDF-1.7\n%\x80\x80\x80\x80\n")) {
			t.Error("the header changed")
		}
		bodies, raws, xrefDict := unpack(t, out)
		for _, want := range []string{"/Root 1 0 R", "/Info 7 0 R", "/ID [(a)(b)]", "/Size 11"} {
			if !strings.Contains(xrefDict, want) {
				t.Errorf("cross-reference stream %s lacks %s", xrefDict, want)
			}
		}
		for i, o := range tricky {
			n := i + 1
			isStream := strings.Contains(o, "\nstream\n")
			switch {
			case n == 5 && drop:
				if _, ok := raws[n]; ok {
					t.Error("the metadata stream is still there")
				}
			case isStream:
				if want := fmt.Sprintf("%d 0 obj\n%s", n, o); raws[n] != want {
					t.Errorf("stream %d: %q, want %q", n, raws[n], want)
				}
			case n == 1 && drop:
				if want := "<</Type/Catalog/Pages 2 0 R /Names<</Dests 4 0 R>>>>"; bodies[n] != want {
					t.Errorf("catalog %q, want %q", bodies[n], want)
				}
			default:
				if bodies[n] != o {
					t.Errorf("object %d: %q, want %q", n, bodies[n], o)
				}
			}
		}
	}
}

func TestPackRefuses(t *testing.T) {
	plain := []string{"<</Type/Catalog/Pages 2 0 R>>", "<</Type/Pages/Count 1/Kids[]>>"}
	for name, pdf := range map[string][]byte{
		"incremental update": build(plain, "<</Size 3/Root 1 0 R/Prev 10>>"),
		"encrypted":          build(plain, "<</Size 3/Root 1 0 R/Encrypt 2 0 R>>"),
		"no catalog":         build(plain, "<</Size 3>>"),
		"no page count":      build([]string{"<</Type/Catalog/Pages 2 0 R>>", "<</Type/Pages>>"}, "<</Size 3/Root 1 0 R>>"),
		"generation 1":       bytes.Replace(build(plain, "<</Size 3/Root 1 0 R>>"), []byte(" 00000 n"), []byte(" 00001 n"), 1),
		"wrong offset":       bytes.Replace(build(plain, "<</Size 3/Root 1 0 R>>"), []byte("0000000016 00000 n"), []byte("0000000017 00000 n"), 1),
		"unterminated":       build([]string{"<</Type/Catalog/Pages 2 0 R/T(abc>>", "<</Type/Pages/Count 1>>"}, "<</Size 3/Root 1 0 R>>"),
		"stream-xref":        []byte("%PDF-1.7\n1 0 obj\n<</Type/XRef>>\nstream\n\nendstream\nendobj\nstartxref\n9\n%%EOF\n"),
	} {
		if _, _, err := Pack(pdf, Options{}); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}
