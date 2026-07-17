package pdfkit

import (
	"bytes"
	"errors"
	"strconv"
	"strings"
)

// Reader is a deliberately minimal cross-reference reader. It parses the trailer,
// the classic (uncompressed) xref table and the object offsets of a PDF produced
// by this package. It is intended for validation and testing rather than as a
// general-purpose PDF parser.
type Reader struct {
	data []byte

	// Size is the /Size entry from the trailer (object count including the
	// free object 0).
	Size int
	// Root is the object number of the document catalog (/Root).
	Root int
	// Info is the object number of the /Info dictionary (0 if absent).
	Info int
	// Offsets maps object number -> byte offset of "N 0 obj". Index 0 is the
	// free list head and is always 0.
	Offsets []int
}

// Parse parses the cross-reference structure of a PDF document held in data.
func Parse(data []byte) (*Reader, error) {
	if !bytes.HasPrefix(data, []byte("%PDF-")) {
		return nil, errors.New("pdfkit: missing %PDF header")
	}
	r := &Reader{data: data}

	idx := bytes.LastIndex(data, []byte("startxref"))
	if idx < 0 {
		return nil, errors.New("pdfkit: missing startxref")
	}
	rest := strings.TrimSpace(string(data[idx+len("startxref"):]))
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return nil, errors.New("pdfkit: malformed startxref")
	}
	xrefOff, err := strconv.Atoi(fields[0])
	if err != nil || xrefOff < 0 || xrefOff >= len(data) {
		return nil, errors.New("pdfkit: invalid startxref offset")
	}

	if err := r.parseXref(xrefOff); err != nil {
		return nil, err
	}
	if err := r.parseTrailer(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *Reader) parseXref(off int) error {
	s := r.data[off:]
	if !bytes.HasPrefix(s, []byte("xref")) {
		return errors.New("pdfkit: xref keyword not found at offset")
	}
	lines := strings.Split(string(s), "\n")
	// lines[0] == "xref"
	if len(lines) < 2 {
		return errors.New("pdfkit: truncated xref")
	}
	header := strings.Fields(lines[1])
	if len(header) != 2 {
		return errors.New("pdfkit: bad xref subsection header")
	}
	start, err := strconv.Atoi(header[0])
	if err != nil {
		return err
	}
	count, err := strconv.Atoi(header[1])
	if err != nil {
		return err
	}
	r.Offsets = make([]int, start+count)
	for i := 0; i < count; i++ {
		line := lines[2+i]
		f := strings.Fields(line)
		if len(f) < 3 {
			return errors.New("pdfkit: bad xref entry")
		}
		o, err := strconv.Atoi(f[0])
		if err != nil {
			return err
		}
		r.Offsets[start+i] = o
	}
	return nil
}

func (r *Reader) parseTrailer() error {
	idx := bytes.LastIndex(r.data, []byte("trailer"))
	if idx < 0 {
		return errors.New("pdfkit: missing trailer")
	}
	seg := r.data[idx:]
	open := bytes.Index(seg, []byte("<<"))
	closeIdx := bytes.Index(seg, []byte(">>"))
	if open < 0 || closeIdx < 0 || closeIdx < open {
		return errors.New("pdfkit: malformed trailer dictionary")
	}
	dict := string(seg[open+2 : closeIdx])
	r.Size = intAfter(dict, "/Size")
	r.Root = refAfter(dict, "/Root")
	r.Info = refAfter(dict, "/Info")
	return nil
}

// ObjectCount returns the number of indirect objects (excluding free object 0).
func (r *Reader) ObjectCount() int {
	if len(r.Offsets) == 0 {
		return 0
	}
	return len(r.Offsets) - 1
}

// Object returns the raw bytes of indirect object n, from "N 0 obj" up to (but
// not including) "endobj". It returns nil if n is out of range.
func (r *Reader) Object(n int) []byte {
	if n <= 0 || n >= len(r.Offsets) {
		return nil
	}
	off := r.Offsets[n]
	if off <= 0 || off >= len(r.data) {
		return nil
	}
	end := bytes.Index(r.data[off:], []byte("endobj"))
	if end < 0 {
		return r.data[off:]
	}
	return r.data[off : off+end]
}

// intAfter finds "key" in s and returns the following integer, or 0.
func intAfter(s, key string) int {
	i := strings.Index(s, key)
	if i < 0 {
		return 0
	}
	f := strings.Fields(s[i+len(key):])
	if len(f) == 0 {
		return 0
	}
	v, _ := strconv.Atoi(f[0])
	return v
}

// refAfter finds "key" in s and returns the object number of a following
// indirect reference "N 0 R", or 0.
func refAfter(s, key string) int {
	i := strings.Index(s, key)
	if i < 0 {
		return 0
	}
	f := strings.Fields(s[i+len(key):])
	if len(f) == 0 {
		return 0
	}
	v, _ := strconv.Atoi(f[0])
	return v
}
