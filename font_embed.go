package pdfkit

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// ttfFont is a parsed TrueType (glyf-based) or OpenType font used for embedding
// and glyph subsetting.
type ttfFont struct {
	tables map[string][]byte // table tag -> raw bytes

	unitsPerEm       uint16
	numGlyphs        uint16
	numberOfHMetrics uint16
	indexToLoc       int16
	loca             []uint32 // numGlyphs+1 offsets into glyf
	glyf             []byte
	hmtx             []byte
	cmap             map[rune]uint16

	xMin, yMin, xMax, yMax int16
	ascent, descent        int16
	capHeight              int16
	italicAngle            float64

	postScriptName string

	// used records glyphs referenced by drawn text (for subsetting), and
	// usedRune maps a glyph id back to a source rune (for ToUnicode).
	used     map[uint16]bool
	usedRune map[uint16]rune
}

// LoadTrueType parses a TrueType or glyf-based OpenType font from r and returns
// a Font that will be embedded (with only the glyphs actually used) when the
// document is written. Unicode text drawn with this font is encoded via the
// font's cmap and mapped back for extraction with a ToUnicode CMap.
func LoadTrueType(r io.Reader) (*Font, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	tf, err := parseTTF(data)
	if err != nil {
		return nil, err
	}
	name := tf.postScriptName
	if name == "" {
		name = "EmbeddedFont"
	}
	return &Font{BaseFont: name, embedded: tf}, nil
}

// LoadTrueTypeFile is a convenience wrapper around LoadTrueType that reads from a
// file path.
func LoadTrueTypeFile(path string) (*Font, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return LoadTrueType(f)
}

// parseTTF parses the sfnt table directory and the tables needed for embedding.
func parseTTF(data []byte) (*ttfFont, error) {
	if len(data) < 12 {
		return nil, errors.New("pdfkit: truetype file too short")
	}
	version := binary.BigEndian.Uint32(data)
	if version == 0x74746366 { // 'ttcf'
		return nil, errors.New("pdfkit: TrueType collections are not supported")
	}
	if version == 0x4F54544F { // 'OTTO' (CFF outlines)
		return nil, errors.New("pdfkit: OpenType/CFF fonts cannot be subset for embedding; use a TrueType (glyf) font")
	}
	numTables := int(binary.BigEndian.Uint16(data[4:]))
	tf := &ttfFont{tables: map[string][]byte{}, used: map[uint16]bool{}, usedRune: map[uint16]rune{}}
	off := 12
	for i := 0; i < numTables; i++ {
		if off+16 > len(data) {
			return nil, errors.New("pdfkit: truncated table directory")
		}
		tag := string(data[off : off+4])
		toff := binary.BigEndian.Uint32(data[off+8:])
		tlen := binary.BigEndian.Uint32(data[off+12:])
		if int(toff)+int(tlen) > len(data) {
			return nil, fmt.Errorf("pdfkit: table %q out of range", tag)
		}
		tf.tables[tag] = data[toff : toff+tlen]
		off += 16
	}

	head, ok := tf.tables["head"]
	if !ok || len(head) < 54 {
		return nil, errors.New("pdfkit: missing head table")
	}
	tf.unitsPerEm = binary.BigEndian.Uint16(head[18:])
	tf.xMin = int16(binary.BigEndian.Uint16(head[36:]))
	tf.yMin = int16(binary.BigEndian.Uint16(head[38:]))
	tf.xMax = int16(binary.BigEndian.Uint16(head[40:]))
	tf.yMax = int16(binary.BigEndian.Uint16(head[42:]))
	tf.indexToLoc = int16(binary.BigEndian.Uint16(head[50:]))
	if tf.unitsPerEm == 0 {
		tf.unitsPerEm = 1000
	}

	maxp, ok := tf.tables["maxp"]
	if !ok || len(maxp) < 6 {
		return nil, errors.New("pdfkit: missing maxp table")
	}
	tf.numGlyphs = binary.BigEndian.Uint16(maxp[4:])

	hhea, ok := tf.tables["hhea"]
	if !ok || len(hhea) < 36 {
		return nil, errors.New("pdfkit: missing hhea table")
	}
	tf.ascent = int16(binary.BigEndian.Uint16(hhea[4:]))
	tf.descent = int16(binary.BigEndian.Uint16(hhea[6:]))
	tf.numberOfHMetrics = binary.BigEndian.Uint16(hhea[34:])

	tf.hmtx = tf.tables["hmtx"]
	tf.glyf = tf.tables["glyf"]
	if tf.glyf == nil {
		return nil, errors.New("pdfkit: missing glyf table (not a TrueType outline font)")
	}
	if err := tf.parseLoca(); err != nil {
		return nil, err
	}
	if err := tf.parseCmap(); err != nil {
		return nil, err
	}

	if os2 := tf.tables["OS/2"]; len(os2) >= 90 {
		tf.capHeight = int16(binary.BigEndian.Uint16(os2[88:]))
	}
	if tf.capHeight == 0 {
		tf.capHeight = int16(float64(tf.ascent) * 0.7)
	}
	tf.postScriptName = tf.readPostScriptName()

	// glyph 0 (.notdef) is always embedded.
	tf.used[0] = true
	return tf, nil
}

// parseLoca decodes the loca table into absolute glyf offsets.
func (tf *ttfFont) parseLoca() error {
	loca, ok := tf.tables["loca"]
	if !ok {
		return errors.New("pdfkit: missing loca table")
	}
	n := int(tf.numGlyphs) + 1
	tf.loca = make([]uint32, n)
	if tf.indexToLoc == 0 { // short
		if len(loca) < n*2 {
			return errors.New("pdfkit: short loca table too small")
		}
		for i := 0; i < n; i++ {
			tf.loca[i] = uint32(binary.BigEndian.Uint16(loca[i*2:])) * 2
		}
	} else { // long
		if len(loca) < n*4 {
			return errors.New("pdfkit: long loca table too small")
		}
		for i := 0; i < n; i++ {
			tf.loca[i] = binary.BigEndian.Uint32(loca[i*4:])
		}
	}
	return nil
}

// parseCmap builds a Unicode-to-glyph map from the best available cmap subtable.
func (tf *ttfFont) parseCmap() error {
	cmap, ok := tf.tables["cmap"]
	if !ok || len(cmap) < 4 {
		return errors.New("pdfkit: missing cmap table")
	}
	numSub := int(binary.BigEndian.Uint16(cmap[2:]))
	bestOff := -1
	bestScore := -1
	for i := 0; i < numSub; i++ {
		rec := 4 + i*8
		if rec+8 > len(cmap) {
			break
		}
		plat := binary.BigEndian.Uint16(cmap[rec:])
		enc := binary.BigEndian.Uint16(cmap[rec+2:])
		sub := binary.BigEndian.Uint32(cmap[rec+4:])
		score := 0
		switch {
		case plat == 3 && enc == 10:
			score = 5
		case plat == 3 && enc == 1:
			score = 4
		case plat == 0:
			score = 3
		case plat == 3 && enc == 0:
			score = 2
		default:
			score = 1
		}
		if score > bestScore && int(sub) < len(cmap) {
			bestScore = score
			bestOff = int(sub)
		}
	}
	if bestOff < 0 {
		return errors.New("pdfkit: no usable cmap subtable")
	}
	tf.cmap = map[rune]uint16{}
	format := binary.BigEndian.Uint16(cmap[bestOff:])
	switch format {
	case 4:
		tf.parseCmap4(cmap[bestOff:])
	case 12:
		tf.parseCmap12(cmap[bestOff:])
	case 6:
		tf.parseCmap6(cmap[bestOff:])
	case 0:
		tf.parseCmap0(cmap[bestOff:])
	default:
		return fmt.Errorf("pdfkit: unsupported cmap format %d", format)
	}
	return nil
}

func (tf *ttfFont) parseCmap0(s []byte) {
	if len(s) < 262 {
		return
	}
	for i := 0; i < 256; i++ {
		g := s[6+i]
		if g != 0 {
			tf.cmap[rune(i)] = uint16(g)
		}
	}
}

func (tf *ttfFont) parseCmap6(s []byte) {
	if len(s) < 10 {
		return
	}
	first := binary.BigEndian.Uint16(s[6:])
	count := binary.BigEndian.Uint16(s[8:])
	for i := 0; i < int(count); i++ {
		if 10+i*2+2 > len(s) {
			break
		}
		g := binary.BigEndian.Uint16(s[10+i*2:])
		if g != 0 {
			tf.cmap[rune(first)+rune(i)] = g
		}
	}
}

func (tf *ttfFont) parseCmap4(s []byte) {
	if len(s) < 14 {
		return
	}
	segX2 := int(binary.BigEndian.Uint16(s[6:]))
	segCount := segX2 / 2
	endOff := 14
	startOff := endOff + segX2 + 2
	deltaOff := startOff + segX2
	rangeOff := deltaOff + segX2
	for i := 0; i < segCount; i++ {
		if rangeOff+i*2+2 > len(s) {
			return
		}
		end := binary.BigEndian.Uint16(s[endOff+i*2:])
		start := binary.BigEndian.Uint16(s[startOff+i*2:])
		delta := binary.BigEndian.Uint16(s[deltaOff+i*2:])
		ro := binary.BigEndian.Uint16(s[rangeOff+i*2:])
		for c := int(start); c <= int(end); c++ {
			if c == 0xFFFF {
				continue
			}
			var g uint16
			if ro == 0 {
				g = uint16(c) + delta
			} else {
				idx := rangeOff + i*2 + int(ro) + (c-int(start))*2
				if idx+2 > len(s) {
					continue
				}
				g = binary.BigEndian.Uint16(s[idx:])
				if g != 0 {
					g += delta
				}
			}
			if g != 0 {
				tf.cmap[rune(c)] = g
			}
		}
	}
}

func (tf *ttfFont) parseCmap12(s []byte) {
	if len(s) < 16 {
		return
	}
	nGroups := binary.BigEndian.Uint32(s[12:])
	for i := 0; i < int(nGroups); i++ {
		base := 16 + i*12
		if base+12 > len(s) {
			return
		}
		start := binary.BigEndian.Uint32(s[base:])
		end := binary.BigEndian.Uint32(s[base+4:])
		gid := binary.BigEndian.Uint32(s[base+8:])
		for c := start; c <= end && c-start < 0x10000; c++ {
			tf.cmap[rune(c)] = uint16(gid + (c - start))
		}
	}
}

// readPostScriptName extracts a PostScript name (name ID 6) from the name table.
func (tf *ttfFont) readPostScriptName() string {
	name, ok := tf.tables["name"]
	if !ok || len(name) < 6 {
		return ""
	}
	count := int(binary.BigEndian.Uint16(name[2:]))
	storage := int(binary.BigEndian.Uint16(name[4:]))
	best := ""
	for i := 0; i < count; i++ {
		rec := 6 + i*12
		if rec+12 > len(name) {
			break
		}
		nameID := binary.BigEndian.Uint16(name[rec+6:])
		if nameID != 6 {
			continue
		}
		length := int(binary.BigEndian.Uint16(name[rec+8:]))
		offset := int(binary.BigEndian.Uint16(name[rec+10:]))
		start := storage + offset
		if start+length > len(name) {
			continue
		}
		raw := name[start : start+length]
		platform := binary.BigEndian.Uint16(name[rec:])
		if platform == 3 || platform == 0 { // UTF-16BE
			var sb strings.Builder
			for j := 0; j+1 < len(raw); j += 2 {
				sb.WriteRune(rune(binary.BigEndian.Uint16(raw[j:])))
			}
			best = sb.String()
		} else {
			best = string(raw)
		}
		if best != "" {
			break
		}
	}
	return sanitizeName(best)
}

// sanitizeName strips characters not allowed in a PDF name.
func sanitizeName(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r > 32 && r < 127 && r != '/' && r != '(' && r != ')' && r != '<' && r != '>' && r != '[' && r != ']' && r != '{' && r != '}' && r != '%' && r != '#' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// runeGID returns the glyph id for r, or 0 (.notdef) if absent.
func (tf *ttfFont) runeGID(r rune) uint16 {
	if g, ok := tf.cmap[r]; ok {
		return g
	}
	return 0
}

// advance returns the advance width of gid in font units.
func (tf *ttfFont) advance(gid uint16) uint16 {
	if len(tf.hmtx) == 0 || tf.numberOfHMetrics == 0 {
		return tf.unitsPerEm
	}
	i := int(gid)
	if i >= int(tf.numberOfHMetrics) {
		i = int(tf.numberOfHMetrics) - 1
	}
	if i*4+2 > len(tf.hmtx) {
		return tf.unitsPerEm
	}
	return binary.BigEndian.Uint16(tf.hmtx[i*4:])
}

// runeWidth1000 returns the advance of r scaled to a 1000-unit em.
func (tf *ttfFont) runeWidth1000(r rune) int {
	return int(tf.advance(tf.runeGID(r))) * 1000 / int(tf.unitsPerEm)
}

// mark records the glyph for r (and any component glyphs) as used, returning the
// glyph id so text can be encoded.
func (tf *ttfFont) mark(r rune) uint16 {
	g := tf.runeGID(r)
	tf.markGID(g)
	tf.usedRune[g] = r
	return g
}

// markGID records gid and, for composite glyphs, its components recursively.
func (tf *ttfFont) markGID(gid uint16) {
	if tf.used[gid] {
		return
	}
	tf.used[gid] = true
	tf.markComponents(gid)
}

func (tf *ttfFont) markComponents(gid uint16) {
	if int(gid)+1 >= len(tf.loca) {
		return
	}
	start, end := tf.loca[gid], tf.loca[gid+1]
	if start >= end || int(end) > len(tf.glyf) {
		return
	}
	g := tf.glyf[start:end]
	if len(g) < 10 {
		return
	}
	if int16(binary.BigEndian.Uint16(g)) >= 0 {
		return // simple glyph
	}
	pos := 10
	for pos+4 <= len(g) {
		flags := binary.BigEndian.Uint16(g[pos:])
		comp := binary.BigEndian.Uint16(g[pos+2:])
		pos += 4
		tf.markGID(comp)
		if flags&0x0001 != 0 { // ARG_1_AND_2_ARE_WORDS
			pos += 4
		} else {
			pos += 2
		}
		switch {
		case flags&0x0008 != 0: // WE_HAVE_A_SCALE
			pos += 2
		case flags&0x0040 != 0: // X_AND_Y_SCALE
			pos += 4
		case flags&0x0080 != 0: // TWO_BY_TWO
			pos += 8
		}
		if flags&0x0020 == 0 { // no MORE_COMPONENTS
			break
		}
	}
}

// subsetGIDs returns the used glyph ids in ascending order.
func (tf *ttfFont) subsetGIDs() []uint16 {
	gids := make([]uint16, 0, len(tf.used))
	for g := range tf.used {
		gids = append(gids, g)
	}
	sort.Slice(gids, func(i, j int) bool { return gids[i] < gids[j] })
	return gids
}

// subsetTag derives a deterministic 6-uppercase-letter subset tag from the set
// of used glyphs.
func (tf *ttfFont) subsetTag() string {
	var h uint32 = 2166136261
	for _, g := range tf.subsetGIDs() {
		h = (h ^ uint32(g)) * 16777619
	}
	var b [6]byte
	for i := 0; i < 6; i++ {
		b[i] = 'A' + byte(h%26)
		h /= 26
		h += uint32(i) * 7
	}
	return string(b[:])
}

// buildSubset builds a stripped sfnt containing only the used glyph outlines,
// preserving glyph numbering (so CIDToGIDMap can stay Identity). Unused glyphs
// become zero-length entries.
func (tf *ttfFont) buildSubset() []byte {
	var newGlyf bytes.Buffer
	newLoca := make([]uint32, int(tf.numGlyphs)+1)
	for gid := 0; gid < int(tf.numGlyphs); gid++ {
		newLoca[gid] = uint32(newGlyf.Len())
		if tf.used[uint16(gid)] {
			start, end := tf.loca[gid], tf.loca[gid+1]
			if start < end && int(end) <= len(tf.glyf) {
				newGlyf.Write(tf.glyf[start:end])
				if newGlyf.Len()%2 == 1 {
					newGlyf.WriteByte(0)
				}
			}
		}
	}
	newLoca[tf.numGlyphs] = uint32(newGlyf.Len())

	// long loca
	locaBuf := make([]byte, len(newLoca)*4)
	for i, v := range newLoca {
		binary.BigEndian.PutUint32(locaBuf[i*4:], v)
	}

	head := append([]byte(nil), tf.tables["head"]...)
	if len(head) >= 54 {
		binary.BigEndian.PutUint32(head[8:], 0)  // checksumAdjustment = 0
		binary.BigEndian.PutUint16(head[50:], 1) // indexToLocFormat = long
	}

	tables := map[string][]byte{
		"head": head,
		"maxp": tf.tables["maxp"],
		"hhea": tf.tables["hhea"],
		"hmtx": tf.hmtx,
		"loca": locaBuf,
		"glyf": newGlyf.Bytes(),
	}
	for _, opt := range []string{"cvt ", "fpgm", "prep", "gasp"} {
		if t, ok := tf.tables[opt]; ok {
			tables[opt] = t
		}
	}
	return writeSFNT(tables)
}

// writeSFNT assembles an sfnt font file from the given tables.
func writeSFNT(tables map[string][]byte) []byte {
	tags := make([]string, 0, len(tables))
	for t := range tables {
		tags = append(tags, t)
	}
	sort.Strings(tags)

	n := len(tags)
	entrySelector := 0
	for (1 << (entrySelector + 1)) <= n {
		entrySelector++
	}
	searchRange := (1 << entrySelector) * 16
	rangeShift := n*16 - searchRange

	var buf bytes.Buffer
	hdr := make([]byte, 12)
	binary.BigEndian.PutUint32(hdr[0:], 0x00010000)
	binary.BigEndian.PutUint16(hdr[4:], uint16(n))
	binary.BigEndian.PutUint16(hdr[6:], uint16(searchRange))
	binary.BigEndian.PutUint16(hdr[8:], uint16(entrySelector))
	binary.BigEndian.PutUint16(hdr[10:], uint16(rangeShift))
	buf.Write(hdr)

	offset := 12 + n*16
	dir := make([]byte, n*16)
	body := &bytes.Buffer{}
	for i, tag := range tags {
		data := tables[tag]
		rec := dir[i*16 : i*16+16]
		copy(rec[0:4], tag)
		binary.BigEndian.PutUint32(rec[4:], tableChecksum(data))
		binary.BigEndian.PutUint32(rec[8:], uint32(offset))
		binary.BigEndian.PutUint32(rec[12:], uint32(len(data)))
		body.Write(data)
		for body.Len()%4 != 0 {
			body.WriteByte(0)
		}
		offset = 12 + n*16 + body.Len()
	}
	buf.Write(dir)
	buf.Write(body.Bytes())
	return buf.Bytes()
}

// tableChecksum computes the sfnt table checksum (sum of big-endian uint32s).
func tableChecksum(b []byte) uint32 {
	var sum uint32
	for i := 0; i < len(b); i += 4 {
		var v uint32
		for j := 0; j < 4; j++ {
			v <<= 8
			if i+j < len(b) {
				v |= uint32(b[i+j])
			}
		}
		sum += v
	}
	return sum
}

// scale1000 scales a font-unit value to a 1000-unit em.
func (tf *ttfFont) scale1000(v int16) int {
	return int(v) * 1000 / int(tf.unitsPerEm)
}

// embeddedFontBodies emits the Type0, CIDFont, FontDescriptor, FontFile2 and
// ToUnicode objects for an embedded font.
func (d *Document) embeddedFontBodies(f *Font, on fontObjNums, set func(int, []byte)) {
	tf := f.embedded
	tag := tf.subsetTag()
	baseName := tag + "+" + f.BaseFont
	gids := tf.subsetGIDs()

	// Type0
	type0 := fmt.Sprintf("<< /Type /Font /Subtype /Type0 /BaseFont /%s /Encoding /Identity-H /DescendantFonts [%d 0 R] /ToUnicode %d 0 R >>",
		baseName, on.cid, on.toUni)
	set(on.primary, []byte(type0))

	// CIDFont with a /W width array.
	var w strings.Builder
	w.WriteString("[")
	for i, g := range gids {
		if i > 0 {
			w.WriteByte(' ')
		}
		fmt.Fprintf(&w, "%d [%d]", g, tf.scale1000(int16(tf.advance(g))))
	}
	w.WriteString("]")
	cid := fmt.Sprintf("<< /Type /Font /Subtype /CIDFontType2 /BaseFont /%s /CIDSystemInfo << /Registry (Adobe) /Ordering (Identity) /Supplement 0 >> /FontDescriptor %d 0 R /CIDToGIDMap /Identity /DW 1000 /W %s >>",
		baseName, on.desc, w.String())
	set(on.cid, []byte(cid))

	// FontDescriptor
	desc := fmt.Sprintf("<< /Type /FontDescriptor /FontName /%s /Flags 32 /FontBBox [%d %d %d %d] /ItalicAngle %s /Ascent %d /Descent %d /CapHeight %d /StemV 80 /FontFile2 %d 0 R >>",
		baseName,
		tf.scale1000(tf.xMin), tf.scale1000(tf.yMin), tf.scale1000(tf.xMax), tf.scale1000(tf.yMax),
		formatFloat(tf.italicAngle), tf.scale1000(tf.ascent), tf.scale1000(tf.descent), tf.scale1000(tf.capHeight),
		on.file)
	set(on.desc, []byte(desc))

	// FontFile2 (subset, Flate-compressed)
	subset := tf.buildSubset()
	comp, err := zlibCompress(subset)
	if err != nil {
		comp = subset
	}
	entries := fmt.Sprintf("/Length1 %d /Filter /FlateDecode", len(subset))
	set(on.file, streamObject(on.file, entries, comp, d.enc))

	// ToUnicode CMap
	set(on.toUni, toUnicodeBody(tf, on.toUni, d.enc))
}

// toUnicodeBody builds a ToUnicode CMap stream mapping glyph ids to Unicode.
func toUnicodeBody(tf *ttfFont, num int, enc *encryptConfig) []byte {
	gids := tf.subsetGIDs()
	type pair struct {
		gid uint16
		r   rune
	}
	var pairs []pair
	for _, g := range gids {
		if r, ok := tf.usedRune[g]; ok {
			pairs = append(pairs, pair{g, r})
		}
	}
	var b strings.Builder
	b.WriteString("/CIDInit /ProcSet findresource begin\n12 dict begin\nbegincmap\n")
	b.WriteString("/CIDSystemInfo << /Registry (Adobe) /Ordering (UCS) /Supplement 0 >> def\n")
	b.WriteString("/CMapName /Adobe-Identity-UCS def\n/CMapType 2 def\n")
	b.WriteString("1 begincodespacerange\n<0000> <FFFF>\nendcodespacerange\n")
	if len(pairs) > 0 {
		fmt.Fprintf(&b, "%d beginbfchar\n", len(pairs))
		for _, p := range pairs {
			fmt.Fprintf(&b, "<%04X> <%s>\n", p.gid, utf16Hex(p.r))
		}
		b.WriteString("endbfchar\n")
	}
	b.WriteString("endcmap\nCMapResource_endcmap\nend\nend")
	return streamObject(num, "", []byte(b.String()), enc)
}

// utf16Hex renders r as a big-endian UTF-16 hex string.
func utf16Hex(r rune) string {
	if r > 0xFFFF {
		r -= 0x10000
		hi := 0xD800 + (r >> 10)
		lo := 0xDC00 + (r & 0x3FF)
		return fmt.Sprintf("%04X%04X", hi, lo)
	}
	return fmt.Sprintf("%04X", r)
}
