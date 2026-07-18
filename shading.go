package pdfkit

import (
	"fmt"
	"strings"
)

// GradientStop is one color stop of a gradient: a color positioned at an offset
// in [0, 1] along the gradient axis.
type GradientStop struct {
	Offset float64
	Color  Color
}

// Shading is a smooth color transition (an axial/linear or radial gradient)
// usable as a paint source. Create one with LinearGradient or RadialGradient,
// then paint it with Page.Shade (fills the current clip region) or use it as a
// fill via NewShadingPattern and Page.SetFillPattern.
type Shading struct {
	kind   int // 2 = axial (linear), 3 = radial
	coords []float64
	stops  []GradientStop
	extend [2]bool
}

// LinearGradient builds an axial gradient running from (x0, y0) to (x1, y1) with
// the given color stops (at least two, ordered by offset).
func LinearGradient(x0, y0, x1, y1 float64, stops ...GradientStop) *Shading {
	return &Shading{kind: 2, coords: []float64{x0, y0, x1, y1}, stops: normStops(stops), extend: [2]bool{true, true}}
}

// RadialGradient builds a radial gradient between two circles: one centered at
// (x0, y0) with radius r0 and one at (x1, y1) with radius r1.
func RadialGradient(x0, y0, r0, x1, y1, r1 float64, stops ...GradientStop) *Shading {
	return &Shading{kind: 3, coords: []float64{x0, y0, r0, x1, y1, r1}, stops: normStops(stops), extend: [2]bool{true, true}}
}

// normStops guarantees at least two stops so a valid function can be built.
func normStops(stops []GradientStop) []GradientStop {
	switch len(stops) {
	case 0:
		return []GradientStop{{0, Black}, {1, White}}
	case 1:
		return []GradientStop{{0, stops[0].Color}, {1, stops[0].Color}}
	default:
		return stops
	}
}

// shadingBody renders a shading dictionary (with an inline color function).
func shadingBody(s *Shading) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<< /ShadingType %d /ColorSpace /DeviceRGB /Coords [", s.kind)
	for i, c := range s.coords {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(formatFloat(c))
	}
	b.WriteString("] /Function ")
	b.WriteString(stitchFunction(s.stops))
	fmt.Fprintf(&b, " /Extend [%t %t] >>", s.extend[0], s.extend[1])
	return b.String()
}

// stitchFunction renders a PDF function mapping [0,1] to an RGB color following
// the gradient's stops: a single exponential (type 2) for two stops, or a
// stitching function (type 3) of exponential subfunctions for more.
func stitchFunction(stops []GradientStop) string {
	if len(stops) == 2 {
		return expFunction(stops[0].Color, stops[1].Color)
	}
	var fns, bounds, encode strings.Builder
	for i := 0; i < len(stops)-1; i++ {
		if i > 0 {
			fns.WriteByte(' ')
			encode.WriteByte(' ')
		}
		fns.WriteString(expFunction(stops[i].Color, stops[i+1].Color))
		encode.WriteString("0 1")
	}
	for i := 1; i < len(stops)-1; i++ {
		if i > 1 {
			bounds.WriteByte(' ')
		}
		bounds.WriteString(formatFloat(stops[i].Offset))
	}
	return fmt.Sprintf("<< /FunctionType 3 /Domain [0 1] /Functions [%s] /Bounds [%s] /Encode [%s] >>",
		fns.String(), bounds.String(), encode.String())
}

// expFunction renders a type-2 exponential interpolation function between two
// colors (linear, N=1).
func expFunction(c0, c1 Color) string {
	return fmt.Sprintf("<< /FunctionType 2 /Domain [0 1] /C0 [%s %s %s] /C1 [%s %s %s] /N 1 >>",
		formatFloat(clamp01(c0.R)), formatFloat(clamp01(c0.G)), formatFloat(clamp01(c0.B)),
		formatFloat(clamp01(c1.R)), formatFloat(clamp01(c1.G)), formatFloat(clamp01(c1.B)))
}

// Shade paints shading s across the current clipping region using the "sh"
// operator. Because "sh" fills the whole clip, first construct a path and call
// Clip (inside a Save/Restore pair) to bound the gradient.
func (p *Page) Shade(s *Shading) {
	name := p.doc.registerShading(s)
	p.usedShadings[name] = s
	p.op("/"+name, "sh")
}

// Pattern is a fill source: either a shading pattern (a gradient usable to fill
// arbitrary paths) built with NewShadingPattern, or a tiling pattern built with
// NewTilingPattern (a small cell repeated across the fill area).
type Pattern struct {
	shading *Shading   // set for a shading (gradient) pattern
	matrix  [6]float64 // pattern matrix
	bbox    [4]float64 // tiling cell bounding box
	xstep   float64    // tiling horizontal step
	ystep   float64    // tiling vertical step
	content []byte     // tiling cell content stream
}

// NewShadingPattern wraps a gradient as a fill pattern so it can fill arbitrary
// paths (via Page.SetFillPattern followed by Page.Fill).
func NewShadingPattern(s *Shading) *Pattern {
	return &Pattern{shading: s, matrix: [6]float64{1, 0, 0, 1, 0, 0}}
}

// NewTilingPattern builds a tiling pattern whose cell is width x height points.
// The draw callback receives a detached page onto which vector graphics and
// colors may be drawn (text and images are not supported in a tiling cell). The
// cell is repeated on a width x height grid to fill any path painted with the
// pattern.
func NewTilingPattern(width, height float64, draw func(*Page)) *Pattern {
	cell := &Page{
		usedFonts:    map[string]*Font{},
		usedImages:   map[string]*Image{},
		usedShadings: map[string]*Shading{},
		usedPatterns: map[string]*Pattern{},
		usedGStates:  map[string]*ExtGState{},
	}
	if draw != nil {
		draw(cell)
	}
	return &Pattern{
		matrix:  [6]float64{1, 0, 0, 1, 0, 0},
		bbox:    [4]float64{0, 0, width, height},
		xstep:   width,
		ystep:   height,
		content: cell.content(),
	}
}

// patternBody renders a pattern object (a dictionary for a shading pattern, or a
// stream for a tiling pattern).
func patternBody(pat *Pattern, num int, enc *encryptConfig) []byte {
	m := pat.matrix
	matrix := fmt.Sprintf("[%s %s %s %s %s %s]",
		formatFloat(m[0]), formatFloat(m[1]), formatFloat(m[2]),
		formatFloat(m[3]), formatFloat(m[4]), formatFloat(m[5]))
	if pat.shading != nil {
		body := fmt.Sprintf("<< /Type /Pattern /PatternType 2 /Shading %s /Matrix %s >>",
			shadingBody(pat.shading), matrix)
		return []byte(body)
	}
	entries := fmt.Sprintf("/Type /Pattern /PatternType 1 /PaintType 1 /TilingType 1 /BBox [%s %s %s %s] /XStep %s /YStep %s /Resources << >> /Matrix %s",
		formatFloat(pat.bbox[0]), formatFloat(pat.bbox[1]), formatFloat(pat.bbox[2]), formatFloat(pat.bbox[3]),
		formatFloat(pat.xstep), formatFloat(pat.ystep), matrix)
	return streamObject(num, entries, pat.content, enc)
}

// SetFillPattern selects pat as the non-stroking (fill) color source. Subsequent
// Fill calls paint with the pattern until another fill color or pattern is set.
func (p *Page) SetFillPattern(pat *Pattern) {
	name := p.doc.registerPattern(pat)
	p.usedPatterns[name] = pat
	p.op("/Pattern", "cs")
	p.op("/"+name, "scn")
}

// SetStrokePattern selects pat as the stroking color source.
func (p *Page) SetStrokePattern(pat *Pattern) {
	name := p.doc.registerPattern(pat)
	p.usedPatterns[name] = pat
	p.op("/Pattern", "CS")
	p.op("/"+name, "SCN")
}
