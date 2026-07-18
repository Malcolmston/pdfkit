package pdfkit

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

// Page is a single page of a Document and the surface onto which text, vector
// graphics and images are drawn.
//
// The coordinate system is native PDF user space: the origin (0, 0) is at the
// bottom-left corner of the page, x increases to the right and y increases
// upward. Distances are in points (1/72 inch). This matches the underlying PDF
// content-stream semantics exactly.
type Page struct {
	doc  *Document
	size PageSize
	buf  bytes.Buffer

	// current text state
	curFont     *Font
	curFontName string
	curSize     float64

	// resources actually referenced by this page's content stream
	usedFonts    map[string]*Font
	usedImages   map[string]*Image
	usedShadings map[string]*Shading
	usedPatterns map[string]*Pattern
	usedGStates  map[string]*ExtGState

	// annots holds link and other annotations placed on this page. Form field
	// widgets are collected separately from the document's AcroForm.
	annots []*annotation
}

func newPage(d *Document, size PageSize) *Page {
	return &Page{
		doc:          d,
		size:         size,
		usedFonts:    map[string]*Font{},
		usedImages:   map[string]*Image{},
		usedShadings: map[string]*Shading{},
		usedPatterns: map[string]*Pattern{},
		usedGStates:  map[string]*ExtGState{},
	}
}

// Size returns the page dimensions in points.
func (p *Page) Size() PageSize { return p.size }

// --- low-level content stream helpers ---

func (p *Page) op(args ...string) {
	for i, a := range args {
		if i > 0 {
			p.buf.WriteByte(' ')
		}
		p.buf.WriteString(a)
	}
	p.buf.WriteByte('\n')
}

func f2s(v float64) string { return formatFloat(v) }

// --- graphics state ---

// SetLineWidth sets the stroke line width in points.
func (p *Page) SetLineWidth(w float64) {
	p.op(f2s(w), "w")
}

// SetStrokeColor sets the color used for stroking paths and text outlines.
func (p *Page) SetStrokeColor(c Color) {
	p.op(f2s(clamp01(c.R)), f2s(clamp01(c.G)), f2s(clamp01(c.B)), "RG")
}

// SetFillColor sets the color used for filling paths and drawing text.
func (p *Page) SetFillColor(c Color) {
	p.op(f2s(clamp01(c.R)), f2s(clamp01(c.G)), f2s(clamp01(c.B)), "rg")
}

// Save pushes a copy of the current graphics state (the PDF "q" operator).
func (p *Page) Save() { p.op("q") }

// Restore pops the graphics state saved by the matching Save ("Q").
func (p *Page) Restore() { p.op("Q") }

// --- path construction ---

// MoveTo begins a new subpath at (x, y).
func (p *Page) MoveTo(x, y float64) { p.op(f2s(x), f2s(y), "m") }

// LineTo appends a straight line segment from the current point to (x, y).
func (p *Page) LineTo(x, y float64) { p.op(f2s(x), f2s(y), "l") }

// CurveTo appends a cubic Bézier curve from the current point to (x3, y3) using
// (x1, y1) and (x2, y2) as control points.
func (p *Page) CurveTo(x1, y1, x2, y2, x3, y3 float64) {
	p.op(f2s(x1), f2s(y1), f2s(x2), f2s(y2), f2s(x3), f2s(y3), "c")
}

// ClosePath closes the current subpath back to its start point ("h").
func (p *Page) ClosePath() { p.op("h") }

// Rect adds a rectangle subpath with its lower-left corner at (x, y).
func (p *Page) Rect(x, y, w, h float64) {
	p.op(f2s(x), f2s(y), f2s(w), f2s(h), "re")
}

// kappa is the constant used to approximate a quarter circle with a cubic Bézier.
const kappa = 0.5522847498307936

// Circle adds a circle subpath centered at (cx, cy) with the given radius,
// approximated by four Bézier curves.
func (p *Page) Circle(cx, cy, r float64) {
	p.Ellipse(cx, cy, r, r)
}

// Ellipse adds an ellipse subpath centered at (cx, cy) with horizontal radius rx
// and vertical radius ry, approximated by four Bézier curves.
func (p *Page) Ellipse(cx, cy, rx, ry float64) {
	ox := rx * kappa
	oy := ry * kappa
	p.MoveTo(cx-rx, cy)
	p.CurveTo(cx-rx, cy+oy, cx-ox, cy+ry, cx, cy+ry)
	p.CurveTo(cx+ox, cy+ry, cx+rx, cy+oy, cx+rx, cy)
	p.CurveTo(cx+rx, cy-oy, cx+ox, cy-ry, cx, cy-ry)
	p.CurveTo(cx-ox, cy-ry, cx-rx, cy-oy, cx-rx, cy)
	p.ClosePath()
}

// --- path painting ---

// Stroke strokes the current path ("S").
func (p *Page) Stroke() { p.op("S") }

// Fill fills the current path using the nonzero winding rule ("f").
func (p *Page) Fill() { p.op("f") }

// FillStroke fills and then strokes the current path ("B").
func (p *Page) FillStroke() { p.op("B") }

// --- convenience shapes ---

// DrawLine strokes a straight line between two points using the current stroke
// color and line width.
func (p *Page) DrawLine(x1, y1, x2, y2 float64) {
	p.MoveTo(x1, y1)
	p.LineTo(x2, y2)
	p.Stroke()
}

// --- text ---

// SetFont selects the font and size (in points) for subsequent text drawing.
// The font is registered with the document and included in the page resources.
func (p *Page) SetFont(f *Font, size float64) {
	name := p.doc.registerFont(f)
	p.usedFonts[name] = f
	p.curFont = f
	p.curFontName = name
	p.curSize = size
}

// DrawText draws a single line of text with its baseline origin at (x, y) using
// the font and size set by SetFont and the current fill color. It is a no-op if
// SetFont has not been called.
func (p *Page) DrawText(x, y float64, text string) {
	if p.curFont == nil {
		return
	}
	p.buf.WriteString("BT\n")
	p.buf.WriteString("/" + p.curFontName + " " + f2s(p.curSize) + " Tf\n")
	p.buf.WriteString(f2s(x) + " " + f2s(y) + " Td\n")
	p.buf.WriteString(p.showText(text) + " Tj\n")
	p.buf.WriteString("ET\n")
}

// showText renders text as the appropriate PDF string operand for the current
// font: a WinAnsi literal string for standard fonts, or a hex string of 2-byte
// glyph identifiers (Identity-H) for an embedded font, marking each glyph used
// for subsetting.
func (p *Page) showText(text string) string {
	if p.curFont.embedded == nil {
		return "(" + escapeString(text) + ")"
	}
	tf := p.curFont.embedded
	var b strings.Builder
	b.WriteByte('<')
	for _, r := range text {
		gid := tf.mark(r)
		fmt.Fprintf(&b, "%04X", gid)
	}
	b.WriteByte('>')
	return b.String()
}

// TextWidth returns the width in points of text in the current font and size.
// It returns 0 if no font has been set.
func (p *Page) TextWidth(text string) float64 {
	if p.curFont == nil {
		return 0
	}
	return p.curFont.Width(text, p.curSize)
}

// DrawLines draws multiple lines of text starting with the first baseline at
// (x, y) and advancing downward by leading points per line.
func (p *Page) DrawLines(x, y, leading float64, lines []string) {
	for i, line := range lines {
		p.DrawText(x, y-float64(i)*leading, line)
	}
}

// DrawParagraph lays out text into lines no wider than maxWidth points, breaking
// on spaces, and draws it starting at (x, y) advancing downward by leading
// points per line. It returns the y coordinate of the baseline following the
// last drawn line.
func (p *Page) DrawParagraph(x, y, maxWidth, leading float64, text string) float64 {
	if p.curFont == nil {
		return y
	}
	lines := wrapText(p.curFont, p.curSize, maxWidth, text)
	p.DrawLines(x, y, leading, lines)
	return y - float64(len(lines))*leading
}

// --- images ---

// DrawImage draws img so that it fills the rectangle with lower-left corner at
// (x, y) and the given width and height (in points). The image is registered
// with the document and referenced from the page resources.
func (p *Page) DrawImage(img *Image, x, y, width, height float64) {
	name := p.doc.registerImage(img)
	p.usedImages[name] = img
	p.op("q")
	// cm matrix: scale unit square to width x height, translate to (x, y).
	p.op(f2s(width), "0", "0", f2s(height), f2s(x), f2s(y), "cm")
	p.op("/"+name, "Do")
	p.op("Q")
}

// content returns the page's content-stream bytes.
func (p *Page) content() []byte { return p.buf.Bytes() }

// resource name helpers -----------------------------------------------------

func fontResourceName(idx int) string    { return "F" + strconv.Itoa(idx+1) }
func imageResourceName(idx int) string   { return "Im" + strconv.Itoa(idx+1) }
func shadingResourceName(idx int) string { return "Sh" + strconv.Itoa(idx+1) }
func patternResourceName(idx int) string { return "P" + strconv.Itoa(idx+1) }
func gstateResourceName(idx int) string  { return "GS" + strconv.Itoa(idx+1) }

// wrapText greedily wraps text into lines that fit within maxWidth points for
// the given font and size. Existing newlines force line breaks.
func wrapText(f *Font, size, maxWidth float64, text string) []string {
	var lines []string
	for _, para := range splitLines(text) {
		words := splitSpaces(para)
		if len(words) == 0 {
			lines = append(lines, "")
			continue
		}
		cur := words[0]
		for _, w := range words[1:] {
			if f.Width(cur+" "+w, size) <= maxWidth {
				cur += " " + w
			} else {
				lines = append(lines, cur)
				cur = w
			}
		}
		lines = append(lines, cur)
	}
	return lines
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}

func splitSpaces(s string) []string {
	var out []string
	start := -1
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' || s[i] == '\t' {
			if start >= 0 {
				out = append(out, s[start:i])
				start = -1
			}
		} else if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		out = append(out, s[start:])
	}
	return out
}
