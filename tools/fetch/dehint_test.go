package main

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"
)

// glyphs of a small font: an empty one, a simple one with instructions, a composite one with instructions after its
// two components (the first with word arguments and a scale).
var (
	simpleGlyph = []byte{
		0, 1, 0, 0, 0, 0, 0, 10, 0, 10, // one contour, bounding box
		0, 2, // end point of the contour: 3 points
		0, 3, 0xB0, 0x01, 0x2C, // three bytes of instructions
		1, 1, 1, 0, 0, 0, 0, 0, 0, // flags and coordinates, kept as they are
	}
	compositeGlyph = []byte{
		0xFF, 0xFF, 0, 0, 0, 0, 0, 10, 0, 10, // -1 contours
		0x00, byte(argsAreWords | haveScale | moreComponents), 0, 1, 0, 5, 0, 6, 0x40, 0x00,
		0x01, 0x00, 0, 1, 7, 8, // haveInstructions, byte arguments
		0, 2, 0xB0, 0x00, // instructions
	}
)

func smallFont(t *testing.T) []byte {
	t.Helper()
	glyf := append(append([]byte{}, simpleGlyph...), 0, 0, 0)
	glyf = append(glyf, compositeGlyph...)
	loca := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, byte(len(simpleGlyph) + 3), 0, 0, 0, byte(len(glyf))}
	head := make([]byte, 54)
	binary.BigEndian.PutUint16(head[50:52], 1)
	maxp := make([]byte, 32)
	binary.BigEndian.PutUint32(maxp[0:4], 0x00010000)
	binary.BigEndian.PutUint16(maxp[4:6], 3)
	binary.BigEndian.PutUint16(maxp[26:28], 3)
	return sfntWrite(0x00010000, map[string][]byte{"head": head, "maxp": maxp, "loca": loca, "glyf": glyf,
		"fpgm": {0xB0, 0x00}, "prep": {0xB0}, "cvt ": {0, 1}, "name": []byte("names stay")})
}

func TestDehint(t *testing.T) {
	out, err := dehint(smallFont(t))
	if err != nil {
		t.Fatal(err)
	}
	tables, err := sfntTables(out)
	if err != nil {
		t.Fatal(err)
	}
	for _, tag := range []string{"fpgm", "prep", "cvt "} {
		if _, ok := tables[tag]; ok {
			t.Errorf("%q is still there", tag)
		}
	}
	if string(tables["name"]) != "names stay" {
		t.Errorf("name: %q", tables["name"])
	}
	if checksum(out) != 0xB1B0AFBA {
		t.Errorf("the file's checksum is %#x, want 0xB1B0AFBA", checksum(out))
	}
	wantSimple := append(append([]byte{}, simpleGlyph[:12]...), 0, 0)
	wantSimple = append(wantSimple, simpleGlyph[17:]...)
	wantComposite := append([]byte{}, compositeGlyph[:len(compositeGlyph)-4]...)
	wantComposite[20] = 0 // haveInstructions cleared
	loca, glyf := tables["loca"], tables["glyf"]
	at := func(i int) int { return int(binary.BigEndian.Uint32(loca[4*i:])) }
	for i, want := range [][]byte{nil, wantSimple, wantComposite} {
		got := glyf[at(i):at(i+1)]
		if !bytes.Equal(bytes.TrimRight(got, "\x00"), bytes.TrimRight(want, "\x00")) || len(got)%4 != 0 {
			t.Errorf("glyph %d: % x, want % x (padded to 4 bytes)", i, got, want)
		}
	}
	if n := binary.BigEndian.Uint16(tables["maxp"][26:28]); n != 0 {
		t.Errorf("maxSizeOfInstructions %d", n)
	}

	// A font with CFF outlines has no TrueType hinting and stays as it is.
	otf := sfntWrite(0x4F54544F, map[string][]byte{"head": make([]byte, 54), "CFF ": {1, 2, 3}})
	if same, err := dehint(otf); err != nil || !bytes.Equal(same, otf) {
		t.Errorf("an OTF font changed: %v", err)
	}
}

// TestDehintJetBrainsMono removes the hinting from the fonts in the download cache and checks that every glyph keeps
// its outline: the glyph without its instructions, byte for byte. It skips when nothing was downloaded yet.
func TestDehintJetBrainsMono(t *testing.T) {
	archives, _ := filepath.Glob(filepath.Join("..", "download-cache", "JetBrainsMono-*.zip"))
	if len(archives) == 0 {
		t.Skip("no JetBrains Mono in tools/download-cache; run go run ./tools/fetch")
	}
	fonts := 0
	err := eachFile(archives[0], func(name string, r io.Reader) error {
		if !strings.HasSuffix(name, ".ttf") || !strings.Contains(name, "/ttf/") {
			return nil
		}
		fonts++
		font, err := io.ReadAll(r)
		if err != nil {
			return err
		}
		out, err := dehint(font)
		if err != nil {
			t.Errorf("%s: %v", path.Base(name), err)
			return nil
		}
		before, _ := sfntTables(font)
		after, _ := sfntTables(out)
		for tag, data := range before {
			switch tag {
			case "glyf", "loca", "head", "maxp", "fpgm", "prep", "cvt ", "cvar", "hdmx", "LTSH", "VDMX", "DSIG":
			default:
				if !bytes.Equal(after[tag], data) {
					t.Errorf("%s: table %q changed", path.Base(name), tag)
				}
			}
		}
		n := int(binary.BigEndian.Uint16(before["maxp"][4:6]))
		longBefore := binary.BigEndian.Uint16(before["head"][50:52]) == 1
		offset := func(tables map[string][]byte, long bool, i int) int {
			if long {
				return int(binary.BigEndian.Uint32(tables["loca"][4*i:]))
			}
			return 2 * int(binary.BigEndian.Uint16(tables["loca"][2*i:]))
		}
		for i := 0; i < n; i++ {
			old := before["glyf"][offset(before, longBefore, i):offset(before, longBefore, i+1)]
			want, err := stripGlyph(old)
			if err != nil {
				t.Fatalf("%s glyph %d: %v", path.Base(name), i, err)
			}
			got := after["glyf"][offset(after, true, i):offset(after, true, i+1)]
			if !bytes.Equal(bytes.TrimRight(got, "\x00"), bytes.TrimRight(want, "\x00")) {
				t.Fatalf("%s glyph %d changed beyond its instructions", path.Base(name), i)
			}
		}
		if checksum(out) != 0xB1B0AFBA {
			t.Errorf("%s: wrong checksumAdjustment", path.Base(name))
		}
		if len(out) > len(font)*3/4 {
			t.Errorf("%s: %d of %d bytes left, expected far less", path.Base(name), len(out), len(font))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if fonts == 0 {
		t.Fatal("no TrueType fonts in the archive")
	}
}

// A TrueType font without hinting comes back as it is, with its short loca (Noto Emoji is such a font).
func TestDehintUnhinted(t *testing.T) {
	glyph := append(append([]byte{}, simpleGlyph[:12]...), 0, 0)
	glyph = append(glyph, simpleGlyph[17:]...)
	for len(glyph)%4 != 0 {
		glyph = append(glyph, 0)
	}
	loca := []byte{0, 0, 0, byte(len(glyph) / 2)}
	head := make([]byte, 54)
	maxp := make([]byte, 6)
	binary.BigEndian.PutUint16(maxp[4:6], 1)
	font := sfntWrite(0x00010000, map[string][]byte{"head": head, "maxp": maxp, "loca": loca, "glyf": glyph})
	if out, err := dehint(font); err != nil || !bytes.Equal(out, font) {
		t.Errorf("an unhinted font changed: %v", err)
	}
	cached, _ := filepath.Glob(filepath.Join("..", "download-cache", "NotoEmoji*.ttf"))
	for _, file := range cached {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if out, err := dehint(data); err != nil || !bytes.Equal(out, data) {
			t.Errorf("%s changed: %v", file, err)
		}
	}
}

// nameTable builds a name table with one record for each platform, encoding, language, name ID and value.
func nameTable(records ...[5]any) []byte {
	var storage []byte
	table := []byte{0, 0, 0, byte(len(records)), 0, byte(6 + 12*len(records))}
	for _, r := range records {
		var value []byte
		for _, u := range utf16.Encode([]rune(r[4].(string))) {
			value = binary.BigEndian.AppendUint16(value, u)
		}
		for _, v := range []int{r[0].(int), r[1].(int), r[2].(int), r[3].(int), len(value), len(storage)} {
			table = binary.BigEndian.AppendUint16(table, uint16(v))
		}
		storage = append(storage, value...)
	}
	return append(table, storage...)
}

func TestFontVersionOf(t *testing.T) {
	for want, name := range map[string][]byte{
		"3.002": nameTable([5]any{3, 1, 0x409, 1, "Noto Emoji"}, [5]any{3, 1, 0x409, 5, "Version 3.002;GOOG;noto-emoji"}),
		"2.304": nameTable([5]any{1, 0, 0, 5, "Version 9.9"}, [5]any{3, 1, 0x409, 5, "Version 2.304; ttfautohint (v1.8.4)"}),
		"1.0":   nameTable([5]any{3, 1, 0x409, 5, "1.0"}),
	} {
		font := sfntWrite(0x00010000, map[string][]byte{"head": make([]byte, 54), "name": name})
		if got, err := fontVersionOf(font); err != nil || got != want {
			t.Errorf("fontVersionOf = %q, %v; want %q", got, err, want)
		}
	}
	for _, name := range [][]byte{nil, nameTable([5]any{3, 1, 0x407, 5, "Version 1.0"}), nameTable([5]any{3, 1, 0x409, 5, "Version "})} {
		font := sfntWrite(0x00010000, map[string][]byte{"head": make([]byte, 54), "name": name})
		if got, err := fontVersionOf(font); err == nil {
			t.Errorf("fontVersionOf = %q, want an error", got)
		}
	}
}

func TestFileURL(t *testing.T) {
	f := fontPin{Repo: "google/fonts", Tag: "abc123", Path: "ofl/notoemoji/NotoEmoji[wght].ttf"}
	if got, want := f.fileURL(), "https://raw.githubusercontent.com/google/fonts/abc123/ofl/notoemoji/NotoEmoji%5Bwght%5D.ttf"; got != want {
		t.Errorf("fileURL = %s, want %s", got, want)
	}
}
