package pdfkit

import (
	"fmt"
	"strings"
)

// BlendMode names a PDF separable or non-separable blend mode used when
// compositing transparent content.
type BlendMode string

// The standard PDF blend modes (PDF 1.4+).
const (
	BlendNormal     BlendMode = "Normal"
	BlendMultiply   BlendMode = "Multiply"
	BlendScreen     BlendMode = "Screen"
	BlendOverlay    BlendMode = "Overlay"
	BlendDarken     BlendMode = "Darken"
	BlendLighten    BlendMode = "Lighten"
	BlendColorDodge BlendMode = "ColorDodge"
	BlendColorBurn  BlendMode = "ColorBurn"
	BlendHardLight  BlendMode = "HardLight"
	BlendSoftLight  BlendMode = "SoftLight"
	BlendDifference BlendMode = "Difference"
	BlendExclusion  BlendMode = "Exclusion"
	BlendHue        BlendMode = "Hue"
	BlendSaturation BlendMode = "Saturation"
	BlendColor      BlendMode = "Color"
	BlendLuminosity BlendMode = "Luminosity"
)

// ExtGState is a graphics-state parameter dictionary controlling constant alpha
// (transparency) and blend mode. Build one with NewExtGState and apply it to a
// page with Page.ApplyExtGState, or use the Page.SetAlpha and Page.SetBlendMode
// convenience methods.
type ExtGState struct {
	fillAlpha   float64 // constant non-stroking alpha; <0 means unset
	strokeAlpha float64 // constant stroking alpha; <0 means unset
	blend       BlendMode
}

// NewExtGState builds a graphics-state dictionary. fillAlpha and strokeAlpha are
// constant alpha values in [0, 1]; pass a negative value to leave one unset. An
// empty blend mode is treated as unset.
func NewExtGState(fillAlpha, strokeAlpha float64, blend BlendMode) *ExtGState {
	return &ExtGState{fillAlpha: fillAlpha, strokeAlpha: strokeAlpha, blend: blend}
}

// gstateBody renders an ExtGState dictionary.
func gstateBody(g *ExtGState) string {
	var b strings.Builder
	b.WriteString("<< /Type /ExtGState")
	if g.fillAlpha >= 0 {
		fmt.Fprintf(&b, " /ca %s", formatFloat(clamp01(g.fillAlpha)))
	}
	if g.strokeAlpha >= 0 {
		fmt.Fprintf(&b, " /CA %s", formatFloat(clamp01(g.strokeAlpha)))
	}
	if g.blend != "" {
		fmt.Fprintf(&b, " /BM /%s", g.blend)
	}
	b.WriteString(" >>")
	return b.String()
}

// ApplyExtGState registers g with the document and applies it to the current
// graphics state (the "gs" operator). Wrap it in Save/Restore to scope its
// effect.
func (p *Page) ApplyExtGState(g *ExtGState) {
	name := p.doc.registerGState(g)
	p.usedGStates[name] = g
	p.op("/"+name, "gs")
}

// SetAlpha sets the constant fill and stroke alpha (transparency) for subsequent
// drawing by applying a fresh ExtGState. Values are clamped to [0, 1].
func (p *Page) SetAlpha(fill, stroke float64) {
	p.ApplyExtGState(NewExtGState(fill, stroke, ""))
}

// SetBlendMode sets the blend mode for subsequent compositing by applying a
// fresh ExtGState.
func (p *Page) SetBlendMode(mode BlendMode) {
	p.ApplyExtGState(NewExtGState(-1, -1, mode))
}

// Clip intersects the current clipping path with the current path using the
// nonzero winding rule, then ends the path without painting it (the "W n"
// sequence). Construct a path first, call Clip, then draw the content to be
// clipped, typically inside a Save/Restore pair.
func (p *Page) Clip() {
	p.op("W", "n")
}

// ClipEvenOdd is like Clip but uses the even-odd rule ("W* n").
func (p *Page) ClipEvenOdd() {
	p.op("W*", "n")
}

// FillEvenOdd fills the current path using the even-odd rule ("f*").
func (p *Page) FillEvenOdd() { p.op("f*") }

// SetDash sets the line dash pattern. The pattern is a list of on/off segment
// lengths (in points); an empty pattern restores a solid line. phase offsets the
// start of the pattern.
func (p *Page) SetDash(phase float64, pattern ...float64) {
	var arr strings.Builder
	arr.WriteByte('[')
	for i, v := range pattern {
		if i > 0 {
			arr.WriteByte(' ')
		}
		arr.WriteString(f2s(v))
	}
	arr.WriteByte(']')
	p.op(arr.String(), f2s(phase), "d")
}

// Transform concatenates the affine matrix [a b c d e f] onto the current
// transformation matrix (the "cm" operator).
func (p *Page) Transform(a, b, c, dd, e, f float64) {
	p.op(f2s(a), f2s(b), f2s(c), f2s(dd), f2s(e), f2s(f), "cm")
}

// Translate concatenates a translation onto the current transformation matrix.
func (p *Page) Translate(tx, ty float64) { p.Transform(1, 0, 0, 1, tx, ty) }

// Scale concatenates a scale onto the current transformation matrix.
func (p *Page) Scale(sx, sy float64) { p.Transform(sx, 0, 0, sy, 0, 0) }
