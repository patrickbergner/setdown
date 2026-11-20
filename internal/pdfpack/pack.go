// Package pdfpack makes the PDFs Typst writes smaller without changing what they show or hold. Typst writes every
// object on its own and uncompressed, with a classic cross-reference table. Most objects besides the fonts and page
// contents are small dictionaries (the tags alone are hundreds), and PDF 1.5's object streams compress them together,
// with a compressed cross-reference stream in place of the table. That takes about a third off a typical document.
//
// Pack reads only what Typst writes: one cross-reference table, no incremental updates, no encryption, generation 0
// throughout. It refuses anything else, and the caller keeps the PDF as Typst wrote it.
package pdfpack

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"slices"
	"strconv"
)

// perStream is the number of objects per object stream, as qpdf does: a reader decompresses a whole object stream
// to get one of its objects.
const perStream = 100

// Options say what Pack changes besides the layout of the file.
type Options struct {
	// DropMetadata leaves out the XMP metadata stream, which only PDF/A and PDF/UA require; the document information
	// dictionary keeps the title, author and dates that viewers show.
	DropMetadata bool
	// FacturX, if set, adds the Factur-X metadata to the XMP (see facturx.go).
	FacturX *FacturX
}

// Result says what Pack did.
type Result struct {
	Pages int // the page count, from the page tree
}

// Pack returns the PDF with its objects in object streams and a cross-reference stream, changed as opts say. The
// output is the same for the same input.
func Pack(data []byte, opts Options) ([]byte, Result, error) {
	objects, trailer, err := readObjects(data)
	if err != nil {
		return nil, Result{}, err
	}
	for _, key := range []string{"Prev", "Encrypt", "XRefStm"} {
		if _, ok := dictGet(trailer, key); ok {
			return nil, Result{}, fmt.Errorf("the trailer has /%s, which Typst does not write", key)
		}
	}
	root, ok := refIn(trailer, "Root")
	if !ok {
		return nil, Result{}, errors.New("the trailer names no catalog")
	}
	catalog, ok := objects[root]
	if !ok || catalog.stream {
		return nil, Result{}, errors.New("the catalog is missing")
	}
	var res Result
	if pages, ok := refIn(catalog.body, "Pages"); ok {
		if tree, ok := objects[pages]; ok && !tree.stream {
			if v, ok := dictGet(tree.body, "Count"); ok {
				res.Pages, _ = strconv.Atoi(string(v))
			}
		}
	}
	if res.Pages == 0 {
		return nil, Result{}, errors.New("no page count in the page tree")
	}
	if opts.DropMetadata && opts.FacturX != nil {
		return nil, Result{}, errors.New("Factur-X metadata needs the XMP metadata")
	}
	if opts.DropMetadata {
		if meta, ok := refIn(catalog.body, "Metadata"); ok {
			catalog.body = dictDelete(catalog.body, "Metadata")
			objects[root] = catalog
			delete(objects, meta)
		}
	}
	if opts.FacturX != nil {
		meta, ok := refIn(catalog.body, "Metadata")
		if !ok {
			return nil, Result{}, errors.New("no XMP metadata for the Factur-X metadata")
		}
		o, ok := objects[meta]
		if !ok || !o.stream {
			return nil, Result{}, errors.New("the XMP metadata is not a stream")
		}
		raw, err := rewriteStream(meta, o.raw, opts.FacturX.addTo)
		if err != nil {
			return nil, Result{}, fmt.Errorf("XMP metadata: %w", err)
		}
		o.raw = raw
		objects[meta] = o
	}

	numbers := make([]int, 0, len(objects))
	for n := range objects {
		numbers = append(numbers, n)
	}
	slices.Sort(numbers)
	next := numbers[len(numbers)-1] + 1

	type entry struct {
		kind       byte // 1: at an offset, 2: in an object stream
		field, idx int
	}
	entries := map[int]entry{}
	header := data[:firstOffset(objects)]
	out := bytes.NewBuffer(make([]byte, 0, len(data)))
	out.Write(header)

	// Streams stay as they are; everything else goes into object streams.
	var packed []int
	for _, n := range numbers {
		o := objects[n]
		if !o.stream {
			packed = append(packed, n)
			continue
		}
		entries[n] = entry{1, out.Len(), 0}
		out.Write(o.raw)
		if !bytes.HasSuffix(o.raw, []byte("\n")) {
			out.WriteByte('\n')
		}
	}
	for start := 0; start < len(packed); start += perStream {
		chunk := packed[start:min(start+perStream, len(packed))]
		stm := next
		next++
		var index, body bytes.Buffer
		for i, n := range chunk {
			if i > 0 {
				index.WriteByte(' ')
			}
			fmt.Fprintf(&index, "%d %d", n, body.Len())
			body.Write(objects[n].body)
			body.WriteByte('\n')
			entries[n] = entry{2, stm, i}
		}
		index.WriteByte('\n')
		content := deflate(append(index.Bytes(), body.Bytes()...))
		entries[stm] = entry{1, out.Len(), 0}
		fmt.Fprintf(out, "%d 0 obj\n<</Type/ObjStm/N %d/First %d/Filter/FlateDecode/Length %d>>\nstream\n",
			stm, len(chunk), index.Len(), len(content))
		out.Write(content)
		out.WriteString("\nendstream\nendobj\n")
	}

	// The cross-reference stream lists itself as well. Field widths: type 1 byte, offset or object stream 4 bytes,
	// index 2 bytes, enough for files up to 4 GB.
	xref := next
	size := xref + 1
	entries[xref] = entry{1, out.Len(), 0}
	if out.Len() > 1<<32-1 {
		return nil, Result{}, errors.New("the PDF is too large for 4-byte offsets")
	}
	var table bytes.Buffer
	for n := 0; n < size; n++ {
		var row [7]byte
		e, ok := entries[n]
		switch {
		case ok:
			row[0] = e.kind
			binary.BigEndian.PutUint32(row[1:5], uint32(e.field))
			binary.BigEndian.PutUint16(row[5:7], uint16(e.idx))
		case n == 0:
			binary.BigEndian.PutUint16(row[5:7], 65535) // the head of the free list
		}
		// Any other number (the left out metadata) is free: type 0, generation 0.
		table.Write(row[:])
	}
	content := deflate(table.Bytes())
	fmt.Fprintf(out, "%d 0 obj\n<</Type/XRef/Size %d/W[1 4 2]", xref, size)
	for _, key := range []string{"Root", "Info", "ID"} {
		if v, ok := dictGet(trailer, key); ok {
			fmt.Fprintf(out, "/%s %s", key, v)
		}
	}
	fmt.Fprintf(out, "/Filter/FlateDecode/Length %d>>\nstream\n", len(content))
	out.Write(content)
	fmt.Fprintf(out, "\nendstream\nendobj\nstartxref\n%d\n%%%%EOF\n", entries[xref].field)
	return out.Bytes(), res, nil
}

func deflate(b []byte) []byte {
	var buf bytes.Buffer
	w, _ := zlib.NewWriterLevel(&buf, zlib.BestCompression)
	w.Write(b)
	w.Close()
	return buf.Bytes()
}

// An object of the input. A stream is copied as it is (raw: from "n 0 obj" to the next object); any other object
// moves into an object stream with its value alone (body).
type object struct {
	offset int
	raw    []byte
	body   []byte
	stream bool
}

func firstOffset(objects map[int]object) int {
	first := -1
	for _, o := range objects {
		if first < 0 || o.offset < first {
			first = o.offset
		}
	}
	return first
}

// readObjects reads the objects the cross-reference table lists and returns them with the trailer dictionary.
func readObjects(data []byte) (map[int]object, []byte, error) {
	sx := bytes.LastIndex(data, []byte("startxref"))
	if sx < 0 {
		return nil, nil, errors.New("no startxref")
	}
	f := bytes.Fields(data[sx+len("startxref"):])
	if len(f) == 0 {
		return nil, nil, errors.New("no offset after startxref")
	}
	xref, err := strconv.Atoi(string(f[0]))
	if err != nil || xref <= 0 || xref >= sx || !bytes.HasPrefix(data[xref:], []byte("xref")) {
		return nil, nil, errors.New("no cross-reference table where startxref points")
	}
	tr := bytes.Index(data[xref:], []byte("trailer"))
	if tr < 0 {
		return nil, nil, errors.New("no trailer")
	}
	tr += xref
	trailer, _, err := value(data, tr+len("trailer"))
	if err != nil {
		return nil, nil, fmt.Errorf("trailer: %w", err)
	}

	offsets := map[int]int{}
	fields := bytes.Fields(data[xref+len("xref") : tr])
	for i := 0; i < len(fields); {
		if i+1 >= len(fields) {
			return nil, nil, errors.New("a broken cross-reference table")
		}
		first, err1 := strconv.Atoi(string(fields[i]))
		count, err2 := strconv.Atoi(string(fields[i+1]))
		if err1 != nil || err2 != nil || i+2+3*count > len(fields) {
			return nil, nil, errors.New("a broken cross-reference table")
		}
		i += 2
		for k := 0; k < count; k, i = k+1, i+3 {
			if string(fields[i+2]) != "n" {
				continue
			}
			off, err1 := strconv.Atoi(string(fields[i]))
			gen, err2 := strconv.Atoi(string(fields[i+1]))
			if err1 != nil || err2 != nil || off <= 0 || off >= xref {
				return nil, nil, fmt.Errorf("object %d: a broken offset", first+k)
			}
			if gen != 0 {
				return nil, nil, fmt.Errorf("object %d has generation %d", first+k, gen)
			}
			offsets[first+k] = off
		}
	}
	if len(offsets) == 0 {
		return nil, nil, errors.New("no objects")
	}

	starts := []int{xref}
	for _, off := range offsets {
		starts = append(starts, off)
	}
	slices.Sort(starts)
	objects := map[int]object{}
	for n, off := range offsets {
		i, _ := slices.BinarySearch(starts, off+1)
		end := starts[i]
		o := object{offset: off, raw: data[off:end]}
		// "n 0 obj"
		pos := off
		for _, want := range []string{strconv.Itoa(n), "0", "obj"} {
			tok, after, err := token(data[:end], pos)
			if err != nil || string(tok) != want {
				return nil, nil, fmt.Errorf("object %d does not start where the table says", n)
			}
			pos = after
		}
		v, after, err := value(data[:end], pos)
		if err != nil {
			return nil, nil, fmt.Errorf("object %d: %w", n, err)
		}
		kw, _, err := token(data[:end], after)
		switch {
		case err != nil:
			return nil, nil, fmt.Errorf("object %d: %w", n, err)
		case string(kw) == "stream":
			o.stream = true
		case string(kw) == "endobj":
			o.body = v
		default:
			return nil, nil, fmt.Errorf("object %d: %q after its value", n, kw)
		}
		objects[n] = o
	}
	return objects, trailer, nil
}

// PDF syntax, as far as Pack needs it.

func isSpace(c byte) bool {
	return c == ' ' || c == '\n' || c == '\r' || c == '\t' || c == '\f' || c == 0
}

func isDelim(c byte) bool {
	return bytes.IndexByte([]byte("()<>[]{}/%"), c) >= 0
}

// token returns the next token at or after pos and the position after it. Strings, hex strings and names are whole
// tokens; << and >> are tokens of their own.
func token(b []byte, pos int) ([]byte, int, error) {
	for pos < len(b) {
		if isSpace(b[pos]) {
			pos++
		} else if b[pos] == '%' {
			for pos < len(b) && b[pos] != '\n' && b[pos] != '\r' {
				pos++
			}
		} else {
			break
		}
	}
	if pos >= len(b) {
		return nil, pos, errors.New("unexpected end")
	}
	start := pos
	switch c := b[pos]; {
	case c == '(':
		depth := 0
		for ; pos < len(b); pos++ {
			switch b[pos] {
			case '\\':
				pos++
			case '(':
				depth++
			case ')':
				depth--
				if depth == 0 {
					return b[start : pos+1], pos + 1, nil
				}
			}
		}
		return nil, pos, errors.New("an unterminated string")
	case c == '<' && pos+1 < len(b) && b[pos+1] == '<', c == '>' && pos+1 < len(b) && b[pos+1] == '>':
		return b[start : pos+2], pos + 2, nil
	case c == '<':
		end := bytes.IndexByte(b[pos:], '>')
		if end < 0 {
			return nil, pos, errors.New("an unterminated hex string")
		}
		return b[start : pos+end+1], pos + end + 1, nil
	case c == '[' || c == ']' || c == '{' || c == '}':
		return b[start : pos+1], pos + 1, nil
	case c == '/':
		pos++
	case c == ')' || c == '>':
		return nil, pos, fmt.Errorf("an unexpected %q", c)
	}
	for pos < len(b) && !isSpace(b[pos]) && !isDelim(b[pos]) {
		pos++
	}
	return b[start:pos], pos, nil
}

// value reads one object (a dictionary, an array, a reference "n 0 R" or a single token) at or after pos and returns
// its text and the position after it.
func value(b []byte, pos int) ([]byte, int, error) {
	tok, after, err := token(b, pos)
	if err != nil {
		return nil, pos, err
	}
	start := after - len(tok)
	switch string(tok) {
	case "<<", "[":
		closer := ">>"
		if string(tok) == "[" {
			closer = "]"
		}
		pos = after
		for {
			t, a, err := token(b, pos)
			if err != nil {
				return nil, pos, err
			}
			if string(t) == closer {
				return b[start:a], a, nil
			}
			if _, pos, err = value(b, pos); err != nil {
				return nil, pos, err
			}
		}
	case ">>", "]":
		return nil, pos, fmt.Errorf("an unexpected %s", tok)
	}
	if isInt(tok) {
		// "n g R" is one value.
		if t2, a2, err := token(b, after); err == nil && isInt(t2) {
			if t3, a3, err := token(b, a2); err == nil && string(t3) == "R" {
				return b[start:a3], a3, nil
			}
		}
	}
	return tok, after, nil
}

func isInt(t []byte) bool {
	if len(t) == 0 {
		return false
	}
	for _, c := range t {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// dictEntry finds key at the top level of a dictionary and returns where its entry (key and value) starts and ends
// and the value's text.
func dictEntry(dict []byte, key string) (start, end int, v []byte, ok bool) {
	tok, pos, err := token(dict, 0)
	if err != nil || string(tok) != "<<" {
		return 0, 0, nil, false
	}
	for {
		k, after, err := token(dict, pos)
		if err != nil || string(k) == ">>" {
			return 0, 0, nil, false
		}
		v, vend, err := value(dict, after)
		if err != nil {
			return 0, 0, nil, false
		}
		if string(k) == "/"+key {
			return after - len(k), vend, v, true
		}
		pos = vend
	}
}

func dictGet(dict []byte, key string) ([]byte, bool) {
	_, _, v, ok := dictEntry(dict, key)
	return v, ok
}

// rewriteStream returns the object "n 0 obj" with its stream's data changed by edit and the /Length to match. It takes
// only an unfiltered stream with a direct length, as Typst writes the XMP metadata.
func rewriteStream(n int, raw []byte, edit func([]byte) ([]byte, error)) ([]byte, error) {
	pos := 0
	for range 3 { // "n 0 obj"
		var err error
		if _, pos, err = token(raw, pos); err != nil {
			return nil, err
		}
	}
	dict, pos, err := value(raw, pos)
	if err != nil {
		return nil, err
	}
	if _, ok := dictGet(dict, "Filter"); ok {
		return nil, errors.New("the stream is compressed")
	}
	lv, ok := dictGet(dict, "Length")
	length, err := strconv.Atoi(string(lv))
	if !ok || err != nil {
		return nil, errors.New("no direct /Length")
	}
	kw, pos, err := token(raw, pos)
	if err != nil || string(kw) != "stream" {
		return nil, errors.New("no stream")
	}
	// The data starts after the end of line that follows the keyword.
	if bytes.HasPrefix(raw[pos:], []byte("\r\n")) {
		pos += 2
	} else if pos < len(raw) && raw[pos] == '\n' {
		pos++
	} else {
		return nil, errors.New("no end of line after stream")
	}
	if pos+length > len(raw) {
		return nil, errors.New("the stream is longer than its object")
	}
	after := bytes.TrimLeft(raw[pos+length:], "\r\n")
	if !bytes.HasPrefix(after, []byte("endstream")) {
		return nil, errors.New("no endstream after /Length bytes")
	}
	data, err := edit(raw[pos : pos+length])
	if err != nil {
		return nil, err
	}
	start, end, _, _ := dictEntry(dict, "Length")
	newDict := append(append(append([]byte{}, dict[:start]...), "/Length "+strconv.Itoa(len(data))...), dict[end:]...)
	var out bytes.Buffer
	fmt.Fprintf(&out, "%d 0 obj\n%s\nstream\n", n, newDict)
	out.Write(data)
	out.WriteString("\nendstream\nendobj\n")
	return out.Bytes(), nil
}

// dictDelete returns the dictionary without key's entry.
func dictDelete(dict []byte, key string) []byte {
	start, end, _, ok := dictEntry(dict, key)
	if !ok {
		return dict
	}
	return append(append(append([]byte{}, dict[:start]...), ' '), dict[end:]...)
}

// refIn returns the object number key refers to.
func refIn(dict []byte, key string) (int, bool) {
	v, ok := dictGet(dict, key)
	if !ok {
		return 0, false
	}
	f := bytes.Fields(v)
	if len(f) != 3 || string(f[2]) != "R" {
		return 0, false
	}
	n, err := strconv.Atoi(string(f[0]))
	return n, err == nil
}
