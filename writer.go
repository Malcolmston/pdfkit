package pdfkit

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// infoKeyOrder fixes the serialization order of Info dictionary entries so that
// output is deterministic.
var infoKeyOrder = []string{"Title", "Author", "Subject", "Keywords", "Creator", "Producer"}

// Write serializes the document as PDF-1.7 to w. The output begins with the
// "%PDF-1.7" header and ends with "%%EOF", and includes a cross-reference table
// and trailer describing every indirect object.
func (d *Document) Write(w io.Writer) error {
	bw := bufio.NewWriter(w)

	// --- assign object numbers ---
	const (
		catalogNum = 1
		pagesNum   = 2
		infoNum    = 3
	)
	next := 4

	fontNum := make([]int, len(d.fonts))
	for i := range d.fonts {
		fontNum[i] = next
		next++
	}

	imageNum := make([]int, len(d.images))
	smaskNum := make([]int, len(d.images))
	for i, img := range d.images {
		imageNum[i] = next
		next++
		if img.smask != nil {
			smaskNum[i] = next
			next++
		}
	}

	pageNum := make([]int, len(d.pages))
	contentNum := make([]int, len(d.pages))
	for i := range d.pages {
		pageNum[i] = next
		next++
		contentNum[i] = next
		next++
	}
	totalObjects := next - 1

	// --- build object bodies in number order ---
	bodies := make([][]byte, totalObjects) // index = objNum-1

	// 1: catalog
	bodies[catalogNum-1] = []byte(fmt.Sprintf("<< /Type /Catalog /Pages %d 0 R >>", pagesNum))

	// 2: pages tree
	var kids strings.Builder
	for i, n := range pageNum {
		if i > 0 {
			kids.WriteByte(' ')
		}
		fmt.Fprintf(&kids, "%d 0 R", n)
	}
	bodies[pagesNum-1] = []byte(fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", kids.String(), len(d.pages)))

	// 3: info
	bodies[infoNum-1] = []byte(d.infoBody())

	// fonts
	for i, f := range d.fonts {
		bodies[fontNum[i]-1] = []byte(fontBody(f))
	}

	// images (+ soft masks)
	for i, img := range d.images {
		var sm int
		if img.smask != nil {
			sm = smaskNum[i]
			bodies[sm-1] = smaskBody(img)
		}
		bodies[imageNum[i]-1] = imageBody(img, sm)
	}

	// pages + content streams
	for i, p := range d.pages {
		bodies[pageNum[i]-1] = []byte(d.pageBody(p, pagesNum, contentNum[i], fontNum, imageNum))
		bodies[contentNum[i]-1] = contentBody(p.content())
	}

	// --- serialize with offset tracking ---
	offsets := make([]int, totalObjects+1) // offsets[n] for object n
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
	// binary marker comment so tools treat the file as binary.
	if err := writeBytes([]byte("%\xE2\xE3\xCF\xD3\n")); err != nil {
		return err
	}

	for n := 1; n <= totalObjects; n++ {
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

	// --- cross-reference table ---
	xrefStart := pos
	if err := write("xref\n"); err != nil {
		return err
	}
	if err := write(fmt.Sprintf("0 %d\n", totalObjects+1)); err != nil {
		return err
	}
	if err := write("0000000000 65535 f \n"); err != nil {
		return err
	}
	for n := 1; n <= totalObjects; n++ {
		if err := write(fmt.Sprintf("%010d 00000 n \n", offsets[n])); err != nil {
			return err
		}
	}

	// --- trailer ---
	if err := write("trailer\n"); err != nil {
		return err
	}
	if err := write(fmt.Sprintf("<< /Size %d /Root %d 0 R /Info %d 0 R >>\n", totalObjects+1, catalogNum, infoNum)); err != nil {
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

// infoBody renders the /Info dictionary body.
func (d *Document) infoBody() string {
	var b strings.Builder
	b.WriteString("<<")
	for _, k := range infoKeyOrder {
		if v, ok := d.info[k]; ok && v != "" {
			b.WriteString(" /" + k + " (" + escapeString(v) + ")")
		}
	}
	b.WriteString(" >>")
	return b.String()
}

// fontBody renders a Type1 font dictionary body.
func fontBody(f *Font) string {
	if f.symbolic {
		return fmt.Sprintf("<< /Type /Font /Subtype /Type1 /BaseFont /%s >>", f.BaseFont)
	}
	return fmt.Sprintf("<< /Type /Font /Subtype /Type1 /BaseFont /%s /Encoding /WinAnsiEncoding >>", f.BaseFont)
}

// imageBody renders an image XObject (dictionary + stream). smaskObj is the
// object number of an associated soft mask, or 0 for none.
func imageBody(img *Image, smaskObj int) []byte {
	filter := "/FlateDecode"
	if img.filter == filterDCT {
		filter = "/DCTDecode"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "<< /Type /XObject /Subtype /Image /Width %d /Height %d /ColorSpace /%s /BitsPerComponent %d /Filter %s /Length %d",
		img.Width, img.Height, img.colorSpace, img.bpc, filter, len(img.data))
	if smaskObj != 0 {
		fmt.Fprintf(&b, " /SMask %d 0 R", smaskObj)
	}
	b.WriteString(" >>\nstream\n")
	out := append([]byte(b.String()), img.data...)
	out = append(out, []byte("\nendstream")...)
	return out
}

// smaskBody renders the soft-mask (alpha) XObject for an image.
func smaskBody(img *Image) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "<< /Type /XObject /Subtype /Image /Width %d /Height %d /ColorSpace /DeviceGray /BitsPerComponent 8 /Filter /FlateDecode /Length %d >>\nstream\n",
		img.Width, img.Height, len(img.smask))
	out := append([]byte(b.String()), img.smask...)
	out = append(out, []byte("\nendstream")...)
	return out
}

// contentBody renders a page content stream object.
func contentBody(content []byte) []byte {
	head := fmt.Sprintf("<< /Length %d >>\nstream\n", len(content))
	out := append([]byte(head), content...)
	out = append(out, []byte("\nendstream")...)
	return out
}

// pageBody renders a page dictionary, including only the resources the page
// actually references.
func (d *Document) pageBody(p *Page, parentNum, contentObj int, fontNum, imageNum []int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<< /Type /Page /Parent %d 0 R /MediaBox [0 0 %s %s]",
		parentNum, formatFloat(p.size.Width), formatFloat(p.size.Height))

	b.WriteString(" /Resources << /ProcSet [/PDF /Text /ImageB /ImageC /ImageI]")

	// fonts used by this page, in document font order
	var fontEntries strings.Builder
	for idx, f := range d.fonts {
		name := fontResourceName(idx)
		if _, ok := p.usedFonts[name]; ok {
			_ = f
			fmt.Fprintf(&fontEntries, " /%s %d 0 R", name, fontNum[idx])
		}
	}
	if fontEntries.Len() > 0 {
		b.WriteString(" /Font <<" + fontEntries.String() + " >>")
	}

	// images used by this page, in document image order
	var xobjEntries strings.Builder
	for idx := range d.images {
		name := imageResourceName(idx)
		if _, ok := p.usedImages[name]; ok {
			fmt.Fprintf(&xobjEntries, " /%s %d 0 R", name, imageNum[idx])
		}
	}
	if xobjEntries.Len() > 0 {
		b.WriteString(" /XObject <<" + xobjEntries.String() + " >>")
	}

	b.WriteString(" >>") // close Resources
	fmt.Fprintf(&b, " /Contents %d 0 R >>", contentObj)
	return b.String()
}
