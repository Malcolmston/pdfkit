package pdfkit

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// infoKeyOrder fixes the serialization order of Info dictionary entries so that
// output is deterministic.
var infoKeyOrder = []string{"Title", "Author", "Subject", "Keywords", "Creator", "Producer"}

// fontObjNums records the object numbers allocated for a single font. For a
// standard Type1 font only primary is used; embedded Type0 fonts also allocate
// descendant, descriptor, font-file and ToUnicode objects.
type fontObjNums struct {
	primary int // the object referenced from page /Font resources
	cid     int // descendant CIDFont (embedded only)
	desc    int // font descriptor (embedded only)
	file    int // FontFile2 stream (embedded only)
	toUni   int // ToUnicode CMap stream (embedded only)
}

// writePlan holds every object number assigned during the numbering pass so the
// body-building pass can emit correct indirect references.
type writePlan struct {
	catalog, pages, info int
	fontObjs             []fontObjNums
	imageNum, smaskNum   []int
	pageNum, contentNum  []int
	pageAnnots           [][]int // per page: object number of each page.annots entry
	fieldObjs            []int   // per document AcroForm field (widget+field merged)
	fieldPage            []int   // page index each field belongs to (-1 if unplaced)
	acroFormObj          int
	outlineObj           int
	outlineItems         []int
	namesObj             int
	shadingObjs          []int
	patternObjs          []int
	gsObjs               []int
	metaObj              int
	encObj               int
	total                int
}

// Write serializes the document as PDF-1.7 to w. The output begins with the
// "%PDF-1.7" header and ends with "%%EOF", and includes a cross-reference table
// and trailer describing every indirect object. Advanced features (annotations,
// outlines, form fields, shadings, patterns, transparency, metadata and
// encryption) are emitted only when used.
func (d *Document) Write(w io.Writer) error {
	plan := d.plan()

	// Prepare encryption. The ID is derived deterministically from the document
	// metadata so output is reproducible, and it also seeds the encryption key.
	id := d.documentID()
	if d.enc != nil {
		d.enc.prepare(id)
	}

	bodies := d.buildBodies(plan, id)

	bw := bufio.NewWriter(w)
	offsets := make([]int, plan.total+1)
	pos := 0
	write := func(s string) error {
		n, err := bw.WriteString(s)
		pos += n
		return err
	}
	writeBytes := func(b []byte) error {
		n, err := bw.Write(b)
		pos += n
		return err
	}

	if err := write("%PDF-1.7\n"); err != nil {
		return err
	}
	if err := writeBytes([]byte("%\xE2\xE3\xCF\xD3\n")); err != nil {
		return err
	}

	for n := 1; n <= plan.total; n++ {
		offsets[n] = pos
		if err := write(strconv.Itoa(n) + " 0 obj\n"); err != nil {
			return err
		}
		if err := writeBytes(bodies[n-1]); err != nil {
			return err
		}
		if err := write("\nendobj\n"); err != nil {
			return err
		}
	}

	xrefStart := pos
	if err := write("xref\n"); err != nil {
		return err
	}
	if err := write(fmt.Sprintf("0 %d\n", plan.total+1)); err != nil {
		return err
	}
	if err := write("0000000000 65535 f \n"); err != nil {
		return err
	}
	for n := 1; n <= plan.total; n++ {
		if err := write(fmt.Sprintf("%010d 00000 n \n", offsets[n])); err != nil {
			return err
		}
	}

	if err := write("trailer\n"); err != nil {
		return err
	}
	if err := write(d.trailerDict(plan, id)); err != nil {
		return err
	}
	if err := write("startxref\n"); err != nil {
		return err
	}
	if err := write(strconv.Itoa(xrefStart) + "\n"); err != nil {
		return err
	}
	if err := write("%%EOF\n"); err != nil {
		return err
	}
	return bw.Flush()
}

// plan assigns object numbers to every indirect object in a fixed, deterministic
// order. The base-case ordering (catalog, pages, info, fonts, images, then each
// page and its content) is preserved so simple documents keep a stable object
// count; feature objects follow only when used.
func (d *Document) plan() writePlan {
	var p writePlan
	n := 0
	alloc := func() int { n++; return n }

	p.catalog = alloc()
	p.pages = alloc()
	p.info = alloc()

	p.fontObjs = make([]fontObjNums, len(d.fonts))
	for i, f := range d.fonts {
		if f.embedded != nil {
			p.fontObjs[i] = fontObjNums{
				primary: alloc(),
				cid:     alloc(),
				desc:    alloc(),
				file:    alloc(),
				toUni:   alloc(),
			}
		} else {
			p.fontObjs[i] = fontObjNums{primary: alloc()}
		}
	}

	p.imageNum = make([]int, len(d.images))
	p.smaskNum = make([]int, len(d.images))
	for i, img := range d.images {
		p.imageNum[i] = alloc()
		if img.smask != nil {
			p.smaskNum[i] = alloc()
		}
	}

	p.pageNum = make([]int, len(d.pages))
	p.contentNum = make([]int, len(d.pages))
	for i := range d.pages {
		p.pageNum[i] = alloc()
		p.contentNum[i] = alloc()
	}

	// page annotations
	p.pageAnnots = make([][]int, len(d.pages))
	for i, pg := range d.pages {
		p.pageAnnots[i] = make([]int, len(pg.annots))
		for j := range pg.annots {
			p.pageAnnots[i][j] = alloc()
		}
	}

	// form fields (each merges field + widget into one object)
	if len(d.acroFields) > 0 {
		p.acroFormObj = alloc()
		p.fieldObjs = make([]int, len(d.acroFields))
		p.fieldPage = make([]int, len(d.acroFields))
		for i, fld := range d.acroFields {
			p.fieldObjs[i] = alloc()
			p.fieldPage[i] = d.pageIndexOf(fld.page)
		}
	}

	// outline
	if d.outline != nil && len(d.outline.items) > 0 {
		p.outlineObj = alloc()
		p.outlineItems = make([]int, len(d.outline.items))
		for i := range d.outline.items {
			p.outlineItems[i] = alloc()
		}
	}

	// named destinations
	if len(d.namedDests) > 0 {
		p.namesObj = alloc()
	}

	// advanced resources
	p.shadingObjs = make([]int, len(d.shadings))
	for i := range d.shadings {
		p.shadingObjs[i] = alloc()
	}
	p.patternObjs = make([]int, len(d.patterns))
	for i := range d.patterns {
		p.patternObjs[i] = alloc()
	}
	p.gsObjs = make([]int, len(d.extGStates))
	for i := range d.extGStates {
		p.gsObjs[i] = alloc()
	}

	if d.xmp != "" {
		p.metaObj = alloc()
	}
	if d.enc != nil {
		p.encObj = alloc()
	}

	p.total = n
	return p
}

// pageIndexOf returns the index of pg in d.pages, or -1.
func (d *Document) pageIndexOf(pg *Page) int {
	for i, p := range d.pages {
		if p == pg {
			return i
		}
	}
	return -1
}

// buildBodies constructs the byte body of every indirect object.
func (d *Document) buildBodies(p writePlan, id []byte) [][]byte {
	_ = id
	bodies := make([][]byte, p.total)
	set := func(num int, b []byte) { bodies[num-1] = b }

	set(p.catalog, []byte(d.catalogBody(p)))
	set(p.pages, []byte(d.pagesBody(p)))
	set(p.info, d.infoBody(p.info))

	for i, f := range d.fonts {
		on := p.fontObjs[i]
		if f.embedded != nil {
			d.embeddedFontBodies(f, on, set)
		} else {
			set(on.primary, []byte(fontBody(f)))
		}
	}

	for i, img := range d.images {
		var sm int
		if img.smask != nil {
			sm = p.smaskNum[i]
			set(sm, smaskBody(img, sm, d.enc))
		}
		set(p.imageNum[i], imageBody(img, sm, p.imageNum[i], d.enc))
	}

	for i, pg := range d.pages {
		set(p.pageNum[i], []byte(d.pageBody(pg, i, p)))
		set(p.contentNum[i], contentBody(pg.content(), p.contentNum[i], d.enc))
	}

	// annotations
	for i, pg := range d.pages {
		for j, a := range pg.annots {
			set(p.pageAnnots[i][j], []byte(d.annotBody(a, p.pageAnnots[i][j], p)))
		}
	}

	// form fields
	if len(d.acroFields) > 0 {
		set(p.acroFormObj, []byte(d.acroFormBody(p)))
		for i, fld := range d.acroFields {
			set(p.fieldObjs[i], []byte(d.fieldBody(fld, i, p)))
		}
	}

	// outline
	if p.outlineObj != 0 {
		d.outlineBodies(p, set)
	}

	// named destinations
	if p.namesObj != 0 {
		set(p.namesObj, []byte(d.namesBody(p)))
	}

	// shadings, patterns, extgstates
	for i, s := range d.shadings {
		set(p.shadingObjs[i], []byte(shadingBody(s)))
	}
	for i, pat := range d.patterns {
		set(p.patternObjs[i], patternBody(pat, p.patternObjs[i], d.enc))
	}
	for i, g := range d.extGStates {
		set(p.gsObjs[i], []byte(gstateBody(g)))
	}

	if p.metaObj != 0 {
		set(p.metaObj, metadataBody(d.xmp, p.metaObj, d.enc))
	}
	if p.encObj != 0 {
		set(p.encObj, []byte(d.enc.dict()))
	}
	return bodies
}

// catalogBody renders the document catalog, wiring in optional features.
func (d *Document) catalogBody(p writePlan) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<< /Type /Catalog /Pages %d 0 R", p.pages)
	if p.outlineObj != 0 {
		fmt.Fprintf(&b, " /Outlines %d 0 R /PageMode /UseOutlines", p.outlineObj)
	}
	if p.namesObj != 0 {
		fmt.Fprintf(&b, " /Names << /Dests %d 0 R >>", p.namesObj)
	}
	if p.acroFormObj != 0 {
		fmt.Fprintf(&b, " /AcroForm %d 0 R", p.acroFormObj)
	}
	if p.metaObj != 0 {
		fmt.Fprintf(&b, " /Metadata %d 0 R", p.metaObj)
	}
	b.WriteString(" >>")
	return b.String()
}

// pagesBody renders the page-tree node.
func (d *Document) pagesBody(p writePlan) string {
	var kids strings.Builder
	for i, n := range p.pageNum {
		if i > 0 {
			kids.WriteByte(' ')
		}
		fmt.Fprintf(&kids, "%d 0 R", n)
	}
	return fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", kids.String(), len(d.pages))
}

// infoBody renders the /Info dictionary body, encrypting string values when
// the document is encrypted.
func (d *Document) infoBody(num int) []byte {
	var b strings.Builder
	b.WriteString("<<")
	for _, k := range infoKeyOrder {
		if v, ok := d.info[k]; ok && v != "" {
			b.WriteString(" /" + k + " " + d.pdfString(v, num))
		}
	}
	b.WriteString(" >>")
	return []byte(b.String())
}

// pdfString renders s as a PDF literal string, applying document encryption for
// object number num when enabled.
func (d *Document) pdfString(s string, num int) string {
	if d.enc == nil {
		return "(" + escapeString(s) + ")"
	}
	enc := d.enc.encrypt(num, 0, []byte(s))
	return "(" + escapeString(string(enc)) + ")"
}

// fontBody renders a standard Type1 font dictionary body.
func fontBody(f *Font) string {
	if f.symbolic {
		return fmt.Sprintf("<< /Type /Font /Subtype /Type1 /BaseFont /%s >>", f.BaseFont)
	}
	return fmt.Sprintf("<< /Type /Font /Subtype /Type1 /BaseFont /%s /Encoding /WinAnsiEncoding >>", f.BaseFont)
}

// imageBody renders an image XObject (dictionary + stream). smaskObj is the
// object number of an associated soft mask, or 0 for none.
func imageBody(img *Image, smaskObj, num int, enc *encryptConfig) []byte {
	filter := "/FlateDecode"
	if img.filter == filterDCT {
		filter = "/DCTDecode"
	}
	entries := fmt.Sprintf("/Type /XObject /Subtype /Image /Width %d /Height %d /ColorSpace /%s /BitsPerComponent %d /Filter %s",
		img.Width, img.Height, img.colorSpace, img.bpc, filter)
	if smaskObj != 0 {
		entries += fmt.Sprintf(" /SMask %d 0 R", smaskObj)
	}
	return streamObject(num, entries, img.data, enc)
}

// smaskBody renders the soft-mask (alpha) XObject for an image.
func smaskBody(img *Image, num int, enc *encryptConfig) []byte {
	entries := fmt.Sprintf("/Type /XObject /Subtype /Image /Width %d /Height %d /ColorSpace /DeviceGray /BitsPerComponent 8 /Filter /FlateDecode",
		img.Width, img.Height)
	return streamObject(num, entries, img.smask, enc)
}

// contentBody renders a page content stream object.
func contentBody(content []byte, num int, enc *encryptConfig) []byte {
	return streamObject(num, "", content, enc)
}

// streamObject builds an indirect stream object body given the dictionary
// entries (without the surrounding "<<" ">>" and without /Length) and the raw
// stream data. When enc is non-nil the data is encrypted for object num and the
// /Length reflects the encrypted size.
func streamObject(num int, entries string, data []byte, enc *encryptConfig) []byte {
	if enc != nil {
		data = enc.encrypt(num, 0, data)
	}
	var b bytes.Buffer
	b.WriteString("<<")
	if entries != "" {
		b.WriteString(" ")
		b.WriteString(entries)
	}
	fmt.Fprintf(&b, " /Length %d >>\nstream\n", len(data))
	b.Write(data)
	b.WriteString("\nendstream")
	return b.Bytes()
}

// pageBody renders a page dictionary, including only the resources the page
// actually references plus any annotations.
func (d *Document) pageBody(pg *Page, pageIdx int, plan writePlan) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<< /Type /Page /Parent %d 0 R /MediaBox [0 0 %s %s]",
		plan.pages, formatFloat(pg.size.Width), formatFloat(pg.size.Height))

	b.WriteString(" /Resources << /ProcSet [/PDF /Text /ImageB /ImageC /ImageI]")

	var fontEntries strings.Builder
	for idx := range d.fonts {
		name := fontResourceName(idx)
		if _, ok := pg.usedFonts[name]; ok {
			fmt.Fprintf(&fontEntries, " /%s %d 0 R", name, plan.fontObjs[idx].primary)
		}
	}
	if fontEntries.Len() > 0 {
		b.WriteString(" /Font <<" + fontEntries.String() + " >>")
	}

	var xobjEntries strings.Builder
	for idx := range d.images {
		name := imageResourceName(idx)
		if _, ok := pg.usedImages[name]; ok {
			fmt.Fprintf(&xobjEntries, " /%s %d 0 R", name, plan.imageNum[idx])
		}
	}
	if xobjEntries.Len() > 0 {
		b.WriteString(" /XObject <<" + xobjEntries.String() + " >>")
	}

	var shEntries strings.Builder
	for idx := range d.shadings {
		name := shadingResourceName(idx)
		if _, ok := pg.usedShadings[name]; ok {
			fmt.Fprintf(&shEntries, " /%s %d 0 R", name, plan.shadingObjs[idx])
		}
	}
	if shEntries.Len() > 0 {
		b.WriteString(" /Shading <<" + shEntries.String() + " >>")
	}

	var patEntries strings.Builder
	for idx := range d.patterns {
		name := patternResourceName(idx)
		if _, ok := pg.usedPatterns[name]; ok {
			fmt.Fprintf(&patEntries, " /%s %d 0 R", name, plan.patternObjs[idx])
		}
	}
	if patEntries.Len() > 0 {
		b.WriteString(" /Pattern <<" + patEntries.String() + " >>")
	}

	var gsEntries strings.Builder
	for idx := range d.extGStates {
		name := gstateResourceName(idx)
		if _, ok := pg.usedGStates[name]; ok {
			fmt.Fprintf(&gsEntries, " /%s %d 0 R", name, plan.gsObjs[idx])
		}
	}
	if gsEntries.Len() > 0 {
		b.WriteString(" /ExtGState <<" + gsEntries.String() + " >>")
	}

	b.WriteString(" >>") // close Resources

	// Annots: page annotations plus any form-field widgets on this page.
	var annots []int
	annots = append(annots, plan.pageAnnots[pageIdx]...)
	for i := range d.acroFields {
		if len(plan.fieldPage) > i && plan.fieldPage[i] == pageIdx {
			annots = append(annots, plan.fieldObjs[i])
		}
	}
	if len(annots) > 0 {
		b.WriteString(" /Annots [")
		for i, n := range annots {
			if i > 0 {
				b.WriteByte(' ')
			}
			fmt.Fprintf(&b, "%d 0 R", n)
		}
		b.WriteString("]")
	}

	fmt.Fprintf(&b, " /Contents %d 0 R >>", plan.contentNum[pageIdx])
	return b.String()
}

// trailerDict renders the trailer dictionary, including /Encrypt and /ID when
// the document is encrypted.
func (d *Document) trailerDict(p writePlan, id []byte) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<< /Size %d /Root %d 0 R /Info %d 0 R", p.total+1, p.catalog, p.info)
	if p.encObj != 0 {
		hexID := fmt.Sprintf("%x", id)
		fmt.Fprintf(&b, " /Encrypt %d 0 R /ID [<%s> <%s>]", p.encObj, hexID, hexID)
	}
	b.WriteString(" >>\n")
	return b.String()
}

// documentID derives a deterministic 16-byte document identifier from the
// metadata and structure, used for the trailer /ID and encryption key seed.
func (d *Document) documentID() []byte {
	h := md5.New()
	for _, k := range infoKeyOrder {
		fmt.Fprintf(h, "%s=%s;", k, d.info[k])
	}
	fmt.Fprintf(h, "pages=%d", len(d.pages))
	return h.Sum(nil)
}
