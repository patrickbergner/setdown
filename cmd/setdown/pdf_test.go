package main

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode/utf16"
	"unicode/utf8"
)

// The conversion tests look at what a reader of the PDF would notice (pages, paper, fonts, the amount and size of the
// text, margin, color) instead of comparing bytes, which may differ between platforms and Typst versions without
// anything being wrong. This reads just enough of a PDF for that, as Typst writes it (a classic xref table, fonts as CID
// fonts with two bytes per glyph) and as pdfpack rewrites it (a cross-reference stream, object streams). It does not
// share pdfpack's reader, so that it checks what pdfpack writes.

// pdfProps are the properties of a PDF the tests check.
type pdfProps struct {
	pages         int
	width, height float64            // of every page, in pt
	fonts         []string           // the embedded fonts' PostScript names without the subset tag, sorted
	glyphs        int                // glyphs shown by text operators
	bodySize      float64            // the font size most glyphs are set in, in pt
	sizes         map[string]float64 // per font, as in fonts, the size most of its glyphs are set in, in pt
	left          float64            // the smallest x at which a run of text starts, in pt
	top           float64            // the smallest distance of a run's baseline from the top of its page, in pt
	colored       bool               // some fill is not a gray
	missing       int                // glyphs shown as .notdef (glyph id 0), for characters the fonts lack
	text          [][]string         // per page, the text of each text-showing operator, through the fonts' ToUnicode maps
	tops          []float64          // per page, top's distance for that page alone, +Inf for a page without text
	drawn         [][]drawnXObject   // per page, the XObjects its content draws
	coloredOn     []bool             // per page, as colored
}

// A drawnXObject is an image or form XObject as a page draws it.
type drawnXObject struct {
	obj            int     // the XObject's object number
	subtype        string  // Image or Form
	x0, y0, x1, y1 float64 // the box it covers on the page, in pt from the bottom left
}

func (p pdfProps) String() string {
	return fmt.Sprintf("%d pages of %.2f×%.2f pt, %d glyphs (%d missing), body %.2f pt, left %.2f pt, colored %v, "+
		"fonts %v", p.pages, p.width, p.height, p.glyphs, p.missing, p.bodySize, p.left, p.colored, p.sizes)
}

// A pdfObject is an indirect object: its text up to the stream, and the decoded stream if it has one.
type pdfObject struct {
	dict   []byte
	stream []byte
}

var (
	reStartXref  = regexp.MustCompile(`startxref\s+(\d+)`)
	reStream     = regexp.MustCompile(`>>\s*stream\r?\n`)
	reLength     = regexp.MustCompile(`/Length\s+(\d+)(\s+\d+\s+R)?`)
	rePage       = regexp.MustCompile(`/Type\s*/Page([^A-Za-z]|$)`)
	reMediaBox   = regexp.MustCompile(`/MediaBox\s*\[\s*([-\d.]+)\s+([-\d.]+)\s+([-\d.]+)\s+([-\d.]+)\s*\]`)
	reContents   = regexp.MustCompile(`/Contents\s*(\[[^\]]*\]|\d+\s+\d+\s+R)`)
	reRef        = regexp.MustCompile(`(\d+)\s+\d+\s+R`)
	reFont       = regexp.MustCompile(`/Type\s*/Font\b`)
	reSubtype    = regexp.MustCompile(`/Subtype\s*/(\w+)`)
	reBaseFont   = regexp.MustCompile(`/BaseFont\s*/([^\s/<>\[\]()]+)`)
	reDescriptor = regexp.MustCompile(`/FontDescriptor\s+(\d+)\s+\d+\s+R`)
	reFontFile   = regexp.MustCompile(`/FontFile[23]?\s+\d+\s+\d+\s+R`)
	reSubsetTag  = regexp.MustCompile(`^[A-Z]{6}\+`)
	rePages      = regexp.MustCompile(`/Type\s*/Pages\b`)
	reKids       = regexp.MustCompile(`/Kids\s*\[([^\]]*)\]`)
	reParent     = regexp.MustCompile(`/Parent\s`)
	reResources  = regexp.MustCompile(`/Resources\s+(\d+)\s+\d+\s+R`)
	reFontRes    = regexp.MustCompile(`/Font\s*(<<[^>]*>>|\d+\s+\d+\s+R)`)
	reNamedRef   = regexp.MustCompile(`/([^\s/<>\[\]()]+)\s+(\d+)\s+\d+\s+R`)
	reToUnicode  = regexp.MustCompile(`/ToUnicode\s+(\d+)\s+\d+\s+R`)
	reXObjectRes = regexp.MustCompile(`/XObject\s*(<<[^>]*>>|\d+\s+\d+\s+R)`)
	reBBox       = regexp.MustCompile(`/BBox\s*\[\s*([-\d.]+)\s+([-\d.]+)\s+([-\d.]+)\s+([-\d.]+)\s*\]`)
	reMatrix     = regexp.MustCompile(`/Matrix\s*\[\s*([-\d.]+)\s+([-\d.]+)\s+([-\d.]+)\s+([-\d.]+)\s+([-\d.]+)\s+([-\d.]+)\s*\]`)
)

// readPDF reads the properties of a PDF.
func readPDF(file string) (pdfProps, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return pdfProps{}, err
	}
	objects, err := pdfObjects(data)
	if err != nil {
		return pdfProps{}, err
	}
	var p pdfProps
	if err := p.readFonts(objects); err != nil {
		return p, err
	}
	pages, err := pageObjects(objects)
	if err != nil {
		return p, err
	}
	text := &textState{bySize: map[float64]int{}, byFont: map[string]map[float64]int{}, left: math.Inf(1),
		top: math.Inf(1), objects: objects}
	for _, n := range pages {
		page := objects[n]
		p.pages++
		box := reMediaBox.FindSubmatch(page.dict)
		if box == nil {
			return p, fmt.Errorf("page object %d has no MediaBox of its own", n)
		}
		width, height := num(box[3])-num(box[1]), num(box[4])-num(box[2])
		if p.pages == 1 {
			p.width, p.height = width, height
		} else if width != p.width || height != p.height {
			return p, fmt.Errorf("page %d is %.2f×%.2f pt, page 1 %.2f×%.2f pt", p.pages, width, height, p.width, p.height)
		}
		fonts, err := pageFonts(objects, page)
		if err != nil {
			return p, fmt.Errorf("page %d: %w", p.pages, err)
		}
		text.runs, text.height, text.pageTop, text.drawn, text.pageColored = nil, height, math.Inf(1), nil, false
		text.xobjects = pageXObjects(objects, page)
		if contents := reContents.FindSubmatch(page.dict); contents != nil {
			for _, ref := range reRef.FindAllSubmatch(contents[1], -1) {
				stream := objects[atoi(ref[1])].stream
				if stream == nil {
					return p, fmt.Errorf("page %d: no content stream in object %s", p.pages, ref[1])
				}
				if err := text.run(stream, fonts); err != nil {
					return p, fmt.Errorf("page %d: %w", p.pages, err)
				}
			}
		}
		p.text = append(p.text, text.runs)
		p.tops = append(p.tops, text.pageTop)
		p.drawn = append(p.drawn, text.drawn)
		p.coloredOn = append(p.coloredOn, text.pageColored)
	}
	p.glyphs, p.missing, p.left, p.top, p.colored = text.glyphs, text.missing, text.left, text.top, text.colored
	p.bodySize = mostUsed(text.bySize)
	p.sizes = map[string]float64{}
	for font, bySize := range text.byFont {
		p.sizes[font] = mostUsed(bySize)
	}
	return p, nil
}

// mostUsed returns the size with the most glyphs.
func mostUsed(bySize map[float64]int) float64 {
	most := 0.0
	for size, n := range bySize {
		if n > bySize[most] || n == bySize[most] && size > most {
			most = size
		}
	}
	return most
}

// pdfObjects reads the objects that the xref table or stream lists, each up to where the next one begins.
func pdfObjects(data []byte) (map[int]pdfObject, error) {
	m := reStartXref.FindAllSubmatch(data, -1)
	if m == nil {
		return nil, fmt.Errorf("no startxref")
	}
	xref := atoi(m[len(m)-1][1])
	if xref >= len(data) {
		return nil, fmt.Errorf("startxref points past the end")
	}
	if !bytes.HasPrefix(data[xref:], []byte("xref")) {
		return xrefStreamObjects(data, xref)
	}
	end := bytes.Index(data[xref:], []byte("trailer"))
	if end < 0 {
		return nil, fmt.Errorf("no trailer")
	}
	fields := bytes.Fields(data[xref+len("xref") : xref+end])
	offsets := map[int]int{}
	for i := 0; i+1 < len(fields); {
		first, count := atoi(fields[i]), atoi(fields[i+1])
		i += 2
		for k := 0; k < count && i+2 < len(fields); k, i = k+1, i+3 {
			if string(fields[i+2]) == "n" {
				offsets[first+k] = atoi(fields[i])
			}
		}
	}
	return objectsAt(data, offsets, xref)
}

// objectsAt reads the objects at the offsets, each up to where the next one begins or to end.
func objectsAt(data []byte, offsets map[int]int, end int) (map[int]pdfObject, error) {
	starts := []int{end}
	for _, off := range offsets {
		starts = append(starts, off)
	}
	sort.Ints(starts)
	objects := map[int]pdfObject{}
	lengths := map[int][]byte{} // objects whose stream length is another object, for a second pass
	for n, off := range offsets {
		next := starts[sort.SearchInts(starts, off+1)]
		text := data[off:next]
		s := reStream.FindIndex(text)
		if s == nil {
			objects[n] = pdfObject{dict: text}
			continue
		}
		objects[n] = pdfObject{dict: text[:s[0]+2]}
		lengths[n] = text[s[1]:]
	}
	for n, rest := range lengths {
		obj := objects[n]
		l := reLength.FindSubmatch(obj.dict)
		if l == nil {
			return nil, fmt.Errorf("object %d: stream without /Length", n)
		}
		length := atoi(l[1])
		if len(l[2]) > 0 { // an indirect length: "<n> 0 obj <length> endobj"
			ref := reRef.FindSubmatch(l[0])
			fields := bytes.Fields(objects[atoi(ref[1])].dict)
			if len(fields) < 4 {
				return nil, fmt.Errorf("object %d: unreadable stream length", n)
			}
			length = atoi(fields[3])
		}
		if length > len(rest) {
			return nil, fmt.Errorf("object %d: stream longer than the object", n)
		}
		obj.stream = rest[:length]
		if bytes.Contains(obj.dict, []byte("/FlateDecode")) {
			r, err := zlib.NewReader(bytes.NewReader(obj.stream))
			if err != nil {
				return nil, fmt.Errorf("object %d: %w", n, err)
			}
			if obj.stream, err = io.ReadAll(r); err != nil {
				return nil, fmt.Errorf("object %d: %w", n, err)
			}
		}
		objects[n] = obj
	}
	return objects, nil
}

var (
	reW     = regexp.MustCompile(`/W\s*\[\s*(\d+)\s+(\d+)\s+(\d+)\s*\]`)
	reSize  = regexp.MustCompile(`/Size\s+(\d+)`)
	reIndex = regexp.MustCompile(`/Index\s*\[([\d\s]*)\]`)
	reFirst = regexp.MustCompile(`/First\s+(\d+)`)
	reN     = regexp.MustCompile(`/N\s+(\d+)`)
	reObjNo = regexp.MustCompile(`^(\d+)\s+0\s+obj`)
)

// xrefStreamObjects reads the objects a cross-reference stream at offset xref lists, those in object streams included.
// An object from an object stream has its value as dict, without "n 0 obj".
func xrefStreamObjects(data []byte, xref int) (map[int]pdfObject, error) {
	no := reObjNo.FindSubmatch(data[xref:])
	if no == nil {
		return nil, fmt.Errorf("neither an xref table nor an object at %d", xref)
	}
	end := bytes.LastIndex(data, []byte("startxref"))
	self, err := objectsAt(data, map[int]int{atoi(no[1]): xref}, end)
	if err != nil {
		return nil, err
	}
	x := self[atoi(no[1])]
	if !bytes.Contains(x.dict, []byte("/Type/XRef")) && !bytes.Contains(x.dict, []byte("/Type /XRef")) {
		return nil, fmt.Errorf("object %s at %d is no cross-reference stream", no[1], xref)
	}
	w := reW.FindSubmatch(x.dict)
	size := reSize.FindSubmatch(x.dict)
	if w == nil || size == nil {
		return nil, fmt.Errorf("cross-reference stream without /W or /Size")
	}
	widths := []int{atoi(w[1]), atoi(w[2]), atoi(w[3])}
	index := []int{0, atoi(size[1])}
	if ix := reIndex.FindSubmatch(x.dict); ix != nil {
		index = nil
		for _, f := range bytes.Fields(ix[1]) {
			index = append(index, atoi(f))
		}
	}
	field := func(b []byte) int {
		v := 0
		for _, c := range b {
			v = v<<8 | int(c)
		}
		return v
	}
	row := widths[0] + widths[1] + widths[2]
	offsets := map[int]int{}
	type inStream struct{ stream, idx int }
	compressed := map[int]inStream{}
	pos := 0
	for i := 0; i+1 < len(index); i += 2 {
		for k := 0; k < index[i+1]; k++ {
			if pos+row > len(x.stream) {
				return nil, fmt.Errorf("cross-reference stream shorter than its /Index")
			}
			r := x.stream[pos : pos+row]
			pos += row
			kind := 1 // a missing type field means type 1
			if widths[0] > 0 {
				kind = field(r[:widths[0]])
			}
			a, b := field(r[widths[0]:widths[0]+widths[1]]), field(r[widths[0]+widths[1]:])
			switch kind {
			case 1:
				offsets[index[i]+k] = a
			case 2:
				compressed[index[i]+k] = inStream{a, b}
			}
		}
	}
	objects, err := objectsAt(data, offsets, end)
	if err != nil {
		return nil, err
	}
	for n, c := range compressed {
		stm, ok := objects[c.stream]
		if !ok || stm.stream == nil {
			return nil, fmt.Errorf("object %d: no object stream %d", n, c.stream)
		}
		first, count := reFirst.FindSubmatch(stm.dict), reN.FindSubmatch(stm.dict)
		if first == nil || count == nil {
			return nil, fmt.Errorf("object stream %d without /First or /N", c.stream)
		}
		f := atoi(first[1])
		pairs := bytes.Fields(stm.stream[:f])
		if len(pairs) != 2*atoi(count[1]) || c.idx >= atoi(count[1]) || atoi(pairs[2*c.idx]) != n {
			return nil, fmt.Errorf("object %d: not at index %d of object stream %d", n, c.idx, c.stream)
		}
		start, stop := f+atoi(pairs[2*c.idx+1]), len(stm.stream)
		if 2*c.idx+3 < len(pairs) {
			stop = f + atoi(pairs[2*c.idx+3])
		}
		objects[n] = pdfObject{dict: stm.stream[start:stop]}
	}
	return objects, nil
}

// pdfContent returns every object's text and decoded stream of a PDF, to look for something anywhere in it. Object
// streams count by the objects in them.
func pdfContent(t *testing.T, file string) []byte {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	objects, err := pdfObjects(data)
	if err != nil {
		t.Fatal(err)
	}
	numbers := make([]int, 0, len(objects))
	for n := range objects {
		numbers = append(numbers, n)
	}
	sort.Ints(numbers)
	var all []byte
	for _, n := range numbers {
		o := objects[n]
		if bytes.Contains(o.dict, []byte("/Type/ObjStm")) {
			o.stream = nil
		}
		all = append(append(append(all, o.dict...), o.stream...), '\n')
	}
	return all
}

// pageObjects returns the page objects in the order of the page tree.
func pageObjects(objects map[int]pdfObject) ([]int, error) {
	var walk func(n, depth int) ([]int, error)
	walk = func(n, depth int) ([]int, error) {
		obj, ok := objects[n]
		if !ok || depth > 32 {
			return nil, fmt.Errorf("broken page tree at object %d", n)
		}
		if !rePages.Match(obj.dict) {
			return []int{n}, nil
		}
		kids := reKids.FindSubmatch(obj.dict)
		if kids == nil {
			return nil, fmt.Errorf("page tree node %d has no kids", n)
		}
		var pages []int
		for _, ref := range reRef.FindAllSubmatch(kids[1], -1) {
			sub, err := walk(atoi(ref[1]), depth+1)
			if err != nil {
				return nil, err
			}
			pages = append(pages, sub...)
		}
		return pages, nil
	}
	for n, obj := range objects {
		if obj.stream == nil && rePages.Match(obj.dict) && !reParent.Match(obj.dict) {
			return walk(n, 0)
		}
	}
	return nil, fmt.Errorf("no page tree")
}

// pageXObjects maps the XObject names a page's content uses to their objects.
func pageXObjects(objects map[int]pdfObject, page pdfObject) map[string]int {
	resources := page.dict
	if ref := reResources.FindSubmatch(page.dict); ref != nil {
		resources = objects[atoi(ref[1])].dict
	}
	names := map[string]int{}
	res := reXObjectRes.FindSubmatch(resources)
	if res == nil {
		return names
	}
	dict := res[1]
	if !bytes.HasPrefix(dict, []byte("<<")) {
		dict = objects[atoi(reRef.FindSubmatch(dict)[1])].dict
	}
	for _, m := range reNamedRef.FindAllSubmatch(dict, -1) {
		names[string(m[1])] = atoi(m[2])
	}
	return names
}

// A pageFont is a font as a page's content uses it.
type pageFont struct {
	name string            // the PostScript name without the subset tag, as in pdfProps.fonts
	cmap map[uint16]string // its ToUnicode map
}

// pageFonts maps the font names a page's content uses to the fonts.
func pageFonts(objects map[int]pdfObject, page pdfObject) (map[string]pageFont, error) {
	resources := page.dict
	if ref := reResources.FindSubmatch(page.dict); ref != nil {
		resources = objects[atoi(ref[1])].dict
	}
	fonts := map[string]pageFont{}
	res := reFontRes.FindSubmatch(resources)
	if res == nil {
		return fonts, nil
	}
	dict := res[1]
	if !bytes.HasPrefix(dict, []byte("<<")) {
		dict = objects[atoi(reRef.FindSubmatch(dict)[1])].dict
	}
	for _, m := range reNamedRef.FindAllSubmatch(dict, -1) {
		cmap := map[uint16]string{}
		if ref := reToUnicode.FindSubmatch(objects[atoi(m[2])].dict); ref != nil {
			var err error
			if cmap, err = parseCMap(objects[atoi(ref[1])].stream); err != nil {
				return nil, fmt.Errorf("font %s: %w", m[1], err)
			}
		}
		font := pageFont{cmap: cmap}
		if base := reBaseFont.FindSubmatch(objects[atoi(m[2])].dict); base != nil {
			// The Type0 font's name is its descendant's with the encoding after it.
			font.name = strings.TrimSuffix(reSubsetTag.ReplaceAllString(string(base[1]), ""), "-Identity-H")
		}
		fonts[string(m[1])] = font
	}
	return fonts, nil
}

// parseCMap reads a ToUnicode CMap's bfchar and bfrange sections, for two-byte codes.
func parseCMap(cmap []byte) (map[uint16]string, error) {
	codes := map[uint16]string{}
	code := func(b []byte) uint16 {
		if len(b) == 1 {
			return uint16(b[0])
		}
		return uint16(b[0])<<8 | uint16(b[1])
	}
	utf16be := func(b []byte) string {
		units := make([]uint16, len(b)/2)
		for i := range units {
			units[i] = uint16(b[2*i])<<8 | uint16(b[2*i+1])
		}
		return string(utf16.Decode(units))
	}
	var section string
	var strs [][]byte
	inArray := false
	var array [][]byte
	err := pdfTokens(cmap, func(kind byte, tok, str []byte) error {
		switch {
		case kind == 's' && inArray:
			array = append(array, str)
		case kind == 's':
			strs = append(strs, str)
		case kind == '[':
			inArray, array = true, nil
		case kind == ']':
			inArray = false
			if section == "bfrange" && len(strs) == 2 {
				for k, dst := range array {
					codes[code(strs[0])+uint16(k)] = utf16be(dst)
				}
				strs = nil
			}
		case kind == 'o' && (string(tok) == "beginbfchar" || string(tok) == "beginbfrange"):
			section, strs = strings.TrimPrefix(string(tok), "begin"), nil
		case kind == 'o' && (string(tok) == "endbfchar" || string(tok) == "endbfrange"):
			section = ""
		}
		if section == "bfchar" && len(strs) == 2 {
			codes[code(strs[0])] = utf16be(strs[1])
			strs = nil
		}
		if section == "bfrange" && len(strs) == 3 {
			lo, hi, dst := code(strs[0]), code(strs[1]), []rune(utf16be(strs[2]))
			for c := lo; c >= lo && c <= hi && len(dst) > 0; c++ {
				codes[c] = string(dst[:len(dst)-1]) + string(dst[len(dst)-1]+rune(c-lo))
			}
			strs = nil
		}
		return nil
	})
	return codes, err
}

// readFonts lists the embedded fonts and makes sure every one is a CID font with its font program in the file.
func (p *pdfProps) readFonts(objects map[int]pdfObject) error {
	seen := map[string]bool{}
	for n, obj := range objects {
		if obj.stream != nil || !reFont.Match(obj.dict) {
			continue
		}
		subtype := reSubtype.FindSubmatch(obj.dict)
		if subtype == nil {
			return fmt.Errorf("font object %d has no subtype", n)
		}
		switch string(subtype[1]) {
		case "Type0": // its descendant is listed on its own
		case "CIDFontType0", "CIDFontType2":
			base := reBaseFont.FindSubmatch(obj.dict)
			descriptor := reDescriptor.FindSubmatch(obj.dict)
			if base == nil || descriptor == nil {
				return fmt.Errorf("font object %d has no name or descriptor", n)
			}
			name := reSubsetTag.ReplaceAllString(string(base[1]), "")
			if !reFontFile.Match(objects[atoi(descriptor[1])].dict) {
				return fmt.Errorf("font %s is not embedded", name)
			}
			seen[name] = true
		default:
			// Glyphs are counted as two bytes each, which holds for CID fonts only.
			return fmt.Errorf("font object %d is a %s font", n, subtype[1])
		}
	}
	for name := range seen {
		p.fonts = append(p.fonts, name)
	}
	sort.Strings(p.fonts)
	return nil
}

// textState follows a content stream's graphics state as far as the properties need it.
type textState struct {
	glyphs      int
	bySize      map[float64]int
	byFont      map[string]map[float64]int // glyphs per font and size
	left        float64
	top         float64
	height      float64 // of the current page
	colored     bool
	missing     int
	runs        []string // the current page's text, one entry per text-showing operator
	pageTop     float64  // as top, for the current page
	pageColored bool     // as colored, for the current page

	objects  map[int]pdfObject
	xobjects map[string]int // the current page's XObjects by name
	drawn    []drawnXObject // those the current page drew
}

// An operand of a content stream operator.
type operand struct {
	kind byte // 'n', 's' or '/' as from pdfTokens
	num  float64
	str  []byte
}

type matrix [6]float64

var identity = matrix{1, 0, 0, 1, 0, 0}

func (a matrix) times(b matrix) matrix {
	return matrix{
		a[0]*b[0] + a[1]*b[2], a[0]*b[1] + a[1]*b[3],
		a[2]*b[0] + a[3]*b[2], a[2]*b[1] + a[3]*b[3],
		a[4]*b[0] + a[5]*b[2] + b[4], a[4]*b[1] + a[5]*b[3] + b[5],
	}
}

// run interprets one content stream. It knows the operators Typst uses for text and fills; any others it skips.
// fonts maps the page's font names to the fonts.
func (s *textState) run(content []byte, fonts map[string]pageFont) error {
	ctm, text, size := identity, identity, 0.0
	var stack []matrix
	var font pageFont
	var operands []operand
	return pdfTokens(content, func(kind byte, tok []byte, str []byte) error {
		switch kind {
		case 'n':
			operands = append(operands, operand{kind: 'n', num: num(tok)})
			return nil
		case 's':
			operands = append(operands, operand{kind: 's', str: str})
			return nil
		case '/':
			operands = append(operands, operand{kind: '/', str: tok[1:]})
			return nil
		case '[', ']', '<', '>':
			return nil
		}
		var nums []float64
		for _, o := range operands {
			if o.kind == 'n' {
				nums = append(nums, o.num)
			}
		}
		last := func(k int) []float64 {
			if len(nums) < k {
				return make([]float64, k)
			}
			return nums[len(nums)-k:]
		}
		switch string(tok) {
		case "q":
			stack = append(stack, ctm)
		case "Q":
			if len(stack) == 0 {
				return fmt.Errorf("Q without q")
			}
			ctm, stack = stack[len(stack)-1], stack[:len(stack)-1]
		case "cm":
			ctm = matrix(last(6)).times(ctm)
		case "BT":
			text = identity
		case "Tm":
			text = matrix(last(6))
		case "Tf":
			size = last(1)[0]
			for _, o := range operands {
				if o.kind == '/' {
					font = fonts[string(o.str)]
				}
			}
		case "TJ", "Tj":
			var b strings.Builder
			n := 0
			for _, o := range operands {
				if o.kind == 'n' && o.num < -100 { // a gap of more than a tenth of an em: Typst's word space
					b.WriteByte(' ')
				}
				if o.kind != 's' {
					continue
				}
				for k := 0; k+1 < len(o.str); k += 2 {
					glyph := uint16(o.str[k])<<8 | uint16(o.str[k+1])
					if glyph == 0 { // glyph id 0 is .notdef, in a subset as in the font
						s.missing++
					}
					if t, ok := font.cmap[glyph]; ok {
						b.WriteString(t)
					} else {
						b.WriteRune(utf8.RuneError)
					}
					n++
				}
			}
			s.runs = append(s.runs, b.String())
			s.glyphs += n
			s.bySize[math.Round(size*100)/100] += n
			if s.byFont[font.name] == nil {
				s.byFont[font.name] = map[float64]int{}
			}
			s.byFont[font.name][math.Round(size*100)/100] += n
			at := text.times(ctm)
			if at[4] < s.left {
				s.left = at[4]
			}
			if y := s.height - at[5]; y < s.top {
				s.top = y
			}
			if y := s.height - at[5]; y < s.pageTop {
				s.pageTop = y
			}
		case "Do":
			for _, o := range operands {
				if o.kind == '/' {
					s.draw(o.str, ctm)
				}
			}
		case "scn", "sc", "rg":
			if len(nums) == 3 && !(nums[0] == nums[1] && nums[1] == nums[2]) {
				s.colored, s.pageColored = true, true
			}
		case "k": // CMYK
			if len(nums) == 4 && (nums[0] != 0 || nums[1] != 0 || nums[2] != 0) {
				s.colored, s.pageColored = true, true
			}
		}
		operands = operands[:0]
		return nil
	})
}

// draw notes the XObject a Do operator draws with the current transformation matrix: an image fills the unit square,
// a form its bounding box under its own matrix.
func (s *textState) draw(name []byte, ctm matrix) {
	n, ok := s.xobjects[string(name)]
	if !ok {
		return
	}
	d := drawnXObject{obj: n}
	if sub := reSubtype.FindSubmatch(s.objects[n].dict); sub != nil {
		d.subtype = string(sub[1])
	}
	box, m := [4]float64{0, 0, 1, 1}, identity
	if d.subtype == "Form" {
		if b := reBBox.FindSubmatch(s.objects[n].dict); b != nil {
			box = [4]float64{num(b[1]), num(b[2]), num(b[3]), num(b[4])}
		}
		if f := reMatrix.FindSubmatch(s.objects[n].dict); f != nil {
			m = matrix{num(f[1]), num(f[2]), num(f[3]), num(f[4]), num(f[5]), num(f[6])}
		}
	}
	d.x0, d.y0, d.x1, d.y1 = math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, c := range [][2]float64{{box[0], box[1]}, {box[2], box[1]}, {box[0], box[3]}, {box[2], box[3]}} {
		at := matrix{1, 0, 0, 1, c[0], c[1]}.times(m).times(ctm)
		d.x0, d.y0 = math.Min(d.x0, at[4]), math.Min(d.y0, at[5])
		d.x1, d.y1 = math.Max(d.x1, at[4]), math.Max(d.y1, at[5])
	}
	s.drawn = append(s.drawn, d)
}

// pdfTokens splits a content stream into numbers ('n'), strings ('s', decoded), names ('/'), array and dictionary
// delimiters, and operators ('o').
func pdfTokens(s []byte, fn func(kind byte, tok, str []byte) error) error {
	isSpace := func(c byte) bool { return c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == '\f' || c == 0 }
	isDelim := func(c byte) bool { return bytes.IndexByte([]byte("()<>[]{}/%"), c) >= 0 }
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case isSpace(c):
			i++
		case c == '%':
			for i < len(s) && s[i] != '\r' && s[i] != '\n' {
				i++
			}
		case c == '(':
			var str []byte
			depth := 1
			for i++; i < len(s) && depth > 0; i++ {
				switch s[i] {
				case '\\':
					i++
					if i >= len(s) {
						break
					}
					if s[i] >= '0' && s[i] <= '7' {
						v, k := 0, 0
						for ; k < 3 && i < len(s) && s[i] >= '0' && s[i] <= '7'; k, i = k+1, i+1 {
							v = v*8 + int(s[i]-'0')
						}
						i--
						str = append(str, byte(v))
					} else if j := bytes.IndexByte([]byte("nrtbf"), s[i]); j >= 0 {
						str = append(str, "\n\r\t\b\f"[j])
					} else if s[i] != '\r' && s[i] != '\n' {
						str = append(str, s[i])
					}
				case '(':
					depth++
					str = append(str, '(')
				case ')':
					depth--
					if depth > 0 {
						str = append(str, ')')
					}
				default:
					str = append(str, s[i])
				}
			}
			if err := fn('s', nil, str); err != nil {
				return err
			}
		case c == '<' && i+1 < len(s) && s[i+1] == '<', c == '>' && i+1 < len(s) && s[i+1] == '>':
			if err := fn(c, s[i:i+2], nil); err != nil {
				return err
			}
			i += 2
		case c == '<':
			end := bytes.IndexByte(s[i:], '>')
			if end < 0 {
				return fmt.Errorf("unterminated hex string")
			}
			hex := bytes.Map(func(r rune) rune {
				if isSpace(byte(r)) {
					return -1
				}
				return r
			}, s[i+1:i+end])
			if len(hex)%2 == 1 {
				hex = append(hex, '0')
			}
			str := make([]byte, len(hex)/2)
			for k := range str {
				v, err := strconv.ParseUint(string(hex[2*k:2*k+2]), 16, 8)
				if err != nil {
					return fmt.Errorf("bad hex string: %w", err)
				}
				str[k] = byte(v)
			}
			if err := fn('s', nil, str); err != nil {
				return err
			}
			i += end + 1
		case c == '[' || c == ']' || c == '{' || c == '}':
			if err := fn(c, s[i:i+1], nil); err != nil {
				return err
			}
			i++
		default:
			j := i + 1
			for j < len(s) && !isSpace(s[j]) && !isDelim(s[j]) {
				j++
			}
			tok, kind := s[i:j], byte('o')
			switch {
			case c == '/':
				kind = '/'
			case (c >= '0' && c <= '9') || c == '-' || c == '+' || c == '.':
				kind = 'n'
			}
			if err := fn(kind, tok, nil); err != nil {
				return err
			}
			i = j
		}
	}
	return nil
}

func num(b []byte) float64 {
	v, _ := strconv.ParseFloat(string(b), 64)
	return v
}

func atoi(b []byte) int {
	v, _ := strconv.Atoi(string(b))
	return v
}

// checkPDF compares a PDF's properties with what a conversion of the overview example should give.
func checkPDF(t *testing.T, got pdfProps, want pdfWant) {
	t.Helper()
	near := func(a, b, tolerance float64) bool { return math.Abs(a-b) <= tolerance }
	if got.pages != want.pages || !near(got.width, want.width, 0.5) || !near(got.height, want.height, 0.5) {
		t.Errorf("%d pages of %.2f×%.2f pt, want %d of %.2f×%.2f", got.pages, got.width, got.height, want.pages,
			want.width, want.height)
	}
	// The body family in its four styles and the code font, and nothing else: another font would be a fallback for
	// glyphs the fonts lack. A code font without styles (DejaVu Sans Mono) has no -Regular in its name.
	mono := want.mono + "-Regular"
	if want.mono != jbMono {
		mono = want.mono
	}
	needed := map[string]bool{want.family + "-Regular": false, want.family + "-It": false, want.family + "-Bold": false,
		want.family + "-Semibold": false, mono: false}
	for _, font := range got.fonts {
		family, _, _ := strings.Cut(font, "-")
		if family != want.family && family != want.mono {
			t.Errorf("unexpected font %s", font)
		}
		if _, ok := needed[font]; ok {
			needed[font] = true
		}
	}
	for font, found := range needed {
		if !found {
			t.Errorf("font %s missing; the PDF has %v", font, got.fonts)
		}
	}
	if !near(float64(got.glyphs), overviewGlyphs, overviewGlyphs/10) {
		t.Errorf("%d glyphs, want about %d", got.glyphs, overviewGlyphs)
	}
	if !near(got.bodySize, want.size, 0.01) {
		t.Errorf("body text in %.2f pt, want %.2f", got.bodySize, want.size)
	}
	// Code at the size where its x-height matches the body text's. Within 1 %, for the rounding of the sizes.
	if body, code := xHeights[want.family], xHeights[want.mono]; body == 0 || code == 0 {
		t.Errorf("no x-height known for %s or %s", want.family, want.mono)
	} else if size, wantSize := got.sizes[mono], want.size*body/code; !near(size, wantSize, wantSize/100) {
		t.Errorf("code in %.2f pt, want %.2f, where its x-height matches the body text's", size, wantSize)
	}
	if !near(got.left, want.left, 0.5) {
		t.Errorf("text starts at %.2f pt from the left, want the margin, %.2f", got.left, want.left)
	}
	if got.missing > 0 {
		t.Errorf("%d glyphs missing from the fonts", got.missing)
	}
	if got.colored != want.colored {
		t.Errorf("colored %v, want %v", got.colored, want.colored)
	}
	if t.Failed() {
		t.Logf("the PDF: %v", got)
	}
}

// xHeights are the fonts' x-heights in em, measured on the outline of an x: for DejaVu Sans Mono, Typst's metric
// gives 0.76, its cap height.
var xHeights = map[string]float64{"SourceSerif4": 0.475, "SourceSans3": 0.486, "CustomSans3": 0.486,
	"JetBrainsMono": 0.55, "DejaVuSansMono": 0.547}

// The checks must notice what they are there for: a character the bundled fonts lack comes out as .notdef.
func TestReadPDFMissingGlyphs(t *testing.T) {
	work := sandbox(t)
	in := filepath.Join(work, "missing.md")
	if err := os.WriteFile(in, []byte("Schriftzeichen 漢字, die keiner der Schriften hat.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code, _ := run([]string{in}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d\n%s", code, stderr.String())
	}
	got, err := readPDF(filepath.Join(work, "missing.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	if got.missing != 2 || got.pages != 1 || got.glyphs < 40 {
		t.Errorf("the PDF: %v; want 2 missing glyphs", got)
	}
}
