package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf16"
)

// hintingTables are the TrueType tables that only grid-fitting uses: the instructions run before any glyph (fpgm,
// prep), the values they work on (cvt, and cvar for variations) and metrics precomputed with them (hdmx, LTSH,
// VDMX). DSIG, a signature over the file, no longer holds once it changes.
var hintingTables = []string{"fpgm", "prep", "cvt ", "cvar", "hdmx", "LTSH", "VDMX", "DSIG"}

// Composite glyph flags.
const (
	argsAreWords     = 0x0001
	haveScale        = 0x0008
	moreComponents   = 0x0020
	haveXYScale      = 0x0040
	haveTwoByTwo     = 0x0080
	haveInstructions = 0x0100
)

// dehint removes the hinting from a TrueType font: the tables above and every glyph's instructions. Hinting adjusts
// outlines to a screen's pixel grid at small sizes. PDF viewers and printers barely use it, but Typst embeds it with
// every subset, where it makes up half of the code font's bytes. The outlines, metrics and names stay as they are.
// A font with CFF outlines (OTF) has no such hinting and comes back unchanged, as does a TrueType font without any.
func dehint(font []byte) ([]byte, error) {
	tables, err := sfntTables(font)
	if err != nil {
		return nil, err
	}
	if _, ok := tables["glyf"]; !ok {
		return font, nil
	}
	for _, tag := range []string{"head", "maxp", "loca"} {
		if _, ok := tables[tag]; !ok {
			return nil, fmt.Errorf("no %s table", tag)
		}
	}
	head := slices.Clone(tables["head"])
	maxp := slices.Clone(tables["maxp"])
	if len(head) < 54 || len(maxp) < 6 {
		return nil, errors.New("a short head or maxp table")
	}
	numGlyphs := int(binary.BigEndian.Uint16(maxp[4:6]))
	longLoca := binary.BigEndian.Uint16(head[50:52]) == 1
	loca, glyf := tables["loca"], tables["glyf"]
	offset := func(i int) int {
		if longLoca {
			return int(binary.BigEndian.Uint32(loca[4*i:]))
		}
		return 2 * int(binary.BigEndian.Uint16(loca[2*i:]))
	}
	if need := (numGlyphs + 1) * map[bool]int{false: 2, true: 4}[longLoca]; len(loca) < need {
		return nil, errors.New("a short loca table")
	}

	// The new glyf, with long offsets: a glyph table that shrank still fits short ones, but long ones never need a
	// check.
	var newGlyf []byte
	newLoca := make([]byte, 4*(numGlyphs+1))
	changed := false
	for _, tag := range hintingTables {
		_, ok := tables[tag]
		changed = changed || ok
	}
	for i := 0; i < numGlyphs; i++ {
		binary.BigEndian.PutUint32(newLoca[4*i:], uint32(len(newGlyf)))
		start, end := offset(i), offset(i+1)
		if start > end || end > len(glyf) {
			return nil, fmt.Errorf("glyph %d: a broken offset", i)
		}
		g, err := stripGlyph(glyf[start:end])
		if err != nil {
			return nil, fmt.Errorf("glyph %d: %w", i, err)
		}
		changed = changed || !slices.Equal(g, glyf[start:end])
		newGlyf = append(newGlyf, g...)
		for len(newGlyf)%4 != 0 {
			newGlyf = append(newGlyf, 0)
		}
	}
	if !changed {
		// Rewriting would only make loca long (Noto Emoji has no hinting).
		return font, nil
	}
	binary.BigEndian.PutUint32(newLoca[4*numGlyphs:], uint32(len(newGlyf)))
	binary.BigEndian.PutUint16(head[50:52], 1)
	// maxp 1.0 sizes the interpreter for the instructions: zones, twilight points, storage, function and instruction
	// definitions, stack, the longest glyph program. None is needed any more.
	if binary.BigEndian.Uint32(maxp[0:4]) == 0x00010000 && len(maxp) >= 32 {
		binary.BigEndian.PutUint16(maxp[14:16], 1) // maxZones
		// maxTwilightPoints to maxSizeOfInstructions; maxComponentElements (28) and maxComponentDepth (30) stay.
		for _, at := range []int{16, 18, 20, 22, 24, 26} {
			binary.BigEndian.PutUint16(maxp[at:at+2], 0)
		}
	}

	tables["glyf"], tables["loca"], tables["head"], tables["maxp"] = newGlyf, newLoca, head, maxp
	for _, tag := range hintingTables {
		delete(tables, tag)
	}
	return sfntWrite(binary.BigEndian.Uint32(font[0:4]), tables), nil
}

// stripGlyph returns a glyph without its instructions.
func stripGlyph(g []byte) ([]byte, error) {
	if len(g) == 0 {
		return g, nil // an empty glyph, such as the space
	}
	if len(g) < 10 {
		return nil, errors.New("shorter than its header")
	}
	contours := int(int16(binary.BigEndian.Uint16(g[0:2])))
	if contours >= 0 {
		// Header, end points of the contours, the instructions' length, the instructions, then flags and coordinates.
		at := 10 + 2*contours
		if at+2 > len(g) {
			return nil, errors.New("shorter than its contours")
		}
		n := int(binary.BigEndian.Uint16(g[at:]))
		if at+2+n > len(g) {
			return nil, errors.New("shorter than its instructions")
		}
		out := slices.Clone(g[:at+2])
		binary.BigEndian.PutUint16(out[at:], 0)
		return append(out, g[at+2+n:]...), nil
	}
	// A composite glyph: components, then the instructions if a component says so.
	out := slices.Clone(g)
	at, instructions := 10, false
	for {
		if at+4 > len(out) {
			return nil, errors.New("a cut-off component")
		}
		flags := binary.BigEndian.Uint16(out[at:])
		if flags&haveInstructions != 0 {
			instructions = true
			binary.BigEndian.PutUint16(out[at:], flags&^haveInstructions)
		}
		at += 4 // flags, glyph index
		if flags&argsAreWords != 0 {
			at += 4
		} else {
			at += 2
		}
		switch {
		case flags&haveScale != 0:
			at += 2
		case flags&haveXYScale != 0:
			at += 4
		case flags&haveTwoByTwo != 0:
			at += 8
		}
		if at > len(out) {
			return nil, errors.New("a cut-off component")
		}
		if flags&moreComponents == 0 {
			break
		}
	}
	if !instructions {
		return out, nil
	}
	return out[:at], nil
}

// sfntTables returns the tables of an sfnt font file by tag.
func sfntTables(font []byte) (map[string][]byte, error) {
	if len(font) < 12 {
		return nil, errors.New("not a font file")
	}
	switch binary.BigEndian.Uint32(font[0:4]) {
	case 0x00010000, 0x74727565, 0x4F54544F: // TrueType, "true", "OTTO"
	default:
		return nil, errors.New("not a TrueType or OpenType font (a collection?)")
	}
	n := int(binary.BigEndian.Uint16(font[4:6]))
	if 12+16*n > len(font) {
		return nil, errors.New("a cut-off table directory")
	}
	tables := map[string][]byte{}
	for i := 0; i < n; i++ {
		rec := font[12+16*i:]
		tag := string(rec[0:4])
		off, length := int(binary.BigEndian.Uint32(rec[8:12])), int(binary.BigEndian.Uint32(rec[12:16]))
		if off+length > len(font) || off < 0 || length < 0 {
			return nil, fmt.Errorf("table %q lies outside the file", tag)
		}
		tables[tag] = font[off : off+length]
	}
	return tables, nil
}

// sfntWrite builds a font file from tables, sorted by tag and 4-byte aligned, with their checksums and head's
// checksumAdjustment.
func sfntWrite(version uint32, tables map[string][]byte) []byte {
	tags := make([]string, 0, len(tables))
	for tag := range tables {
		tags = append(tags, tag)
	}
	slices.Sort(tags)
	n := len(tags)
	pow, log := 1, 0
	for pow*2 <= n {
		pow, log = pow*2, log+1
	}
	out := make([]byte, 12+16*n)
	binary.BigEndian.PutUint32(out[0:4], version)
	binary.BigEndian.PutUint16(out[4:6], uint16(n))
	binary.BigEndian.PutUint16(out[6:8], uint16(16*pow))
	binary.BigEndian.PutUint16(out[8:10], uint16(log))
	binary.BigEndian.PutUint16(out[10:12], uint16(16*n-16*pow))
	headAt := -1
	for i, tag := range tags {
		data := tables[tag]
		if tag == "head" {
			data = slices.Clone(data)
			binary.BigEndian.PutUint32(data[8:12], 0) // checksumAdjustment, set below
			headAt = len(out)
		}
		rec := out[12+16*i:]
		copy(rec[0:4], tag)
		binary.BigEndian.PutUint32(rec[4:8], checksum(data))
		binary.BigEndian.PutUint32(rec[8:12], uint32(len(out)))
		binary.BigEndian.PutUint32(rec[12:16], uint32(len(data)))
		out = append(out, data...)
		for len(out)%4 != 0 {
			out = append(out, 0)
		}
	}
	if headAt >= 0 {
		binary.BigEndian.PutUint32(out[headAt+8:headAt+12], 0xB1B0AFBA-checksum(out))
	}
	return out
}

func checksum(b []byte) uint32 {
	var sum uint32
	for i := 0; i < len(b); i += 4 {
		var word [4]byte
		copy(word[:], b[i:min(i+4, len(b))])
		sum += binary.BigEndian.Uint32(word[:])
	}
	return sum
}

// fontVersionOf returns a font's version as its name table gives it in English for Windows (name ID 5), without the
// word Version and what follows the number: 3.002 for "Version 3.002;GOOG;noto-emoji".
func fontVersionOf(font []byte) (string, error) {
	tables, err := sfntTables(font)
	if err != nil {
		return "", err
	}
	name := tables["name"]
	if len(name) < 6 {
		return "", errors.New("no name table")
	}
	count, storage := int(binary.BigEndian.Uint16(name[2:4])), int(binary.BigEndian.Uint16(name[4:6]))
	for i := 0; i < count && 6+12*i+12 <= len(name); i++ {
		rec := name[6+12*i:]
		platform, encoding := binary.BigEndian.Uint16(rec[0:2]), binary.BigEndian.Uint16(rec[2:4])
		language, id := binary.BigEndian.Uint16(rec[4:6]), binary.BigEndian.Uint16(rec[6:8])
		length, offset := int(binary.BigEndian.Uint16(rec[8:10])), int(binary.BigEndian.Uint16(rec[10:12]))
		if platform != 3 || encoding != 1 || language != 0x409 || id != 5 {
			continue
		}
		raw := name[min(storage+offset, len(name)):min(storage+offset+length, len(name))]
		units := make([]uint16, len(raw)/2)
		for j := range units {
			units[j] = binary.BigEndian.Uint16(raw[2*j:])
		}
		v := strings.TrimSpace(strings.TrimPrefix(string(utf16.Decode(units)), "Version "))
		v, _, _ = strings.Cut(v, ";")
		v, _, _ = strings.Cut(v, " ")
		if v == "" {
			break
		}
		return v, nil
	}
	return "", errors.New("no version in the name table")
}
