package pdfkit

import (
	"fmt"
	"strconv"
	"strings"
)

// CMYKColor is a color in the subtractive CMYK (cyan, magenta, yellow, key/black)
// color space, with each component in the range [0, 1]. CMYK is the native color
// model of most print workflows; PDF supports it directly through the
// DeviceCMYK color space and the "k"/"K" content operators. Construct one with
// CMYK and paint with Page.SetFillCMYK or Page.SetStrokeCMYK.
type CMYKColor struct {
	C, M, Y, K float64
}

// CMYK constructs a CMYKColor from four components, each clamped to [0, 1].
func CMYK(c, m, y, k float64) CMYKColor {
	return CMYKColor{C: clamp01(c), M: clamp01(m), Y: clamp01(y), K: clamp01(k)}
}

// RGB converts the CMYK color to its RGB equivalent using the standard naive
// conversion R=(1-C)(1-K), G=(1-M)(1-K), B=(1-Y)(1-K). The result is suitable
// for on-screen preview; exact print reproduction depends on the output device's
// color profile.
func (c CMYKColor) RGB() Color {
	return Color{
		R: (1 - c.C) * (1 - c.K),
		G: (1 - c.M) * (1 - c.K),
		B: (1 - c.Y) * (1 - c.K),
	}
}

// CMYK converts an RGB color to CMYK. The black component K is derived as
// 1-max(R,G,B) and the chromatic components are computed relative to it. Pure
// black (0,0,0) maps to K=1 with zero chroma.
func (c Color) CMYK() CMYKColor {
	r, g, b := clamp01(c.R), clamp01(c.G), clamp01(c.B)
	k := 1 - maxf(r, maxf(g, b))
	if k >= 1 {
		return CMYKColor{C: 0, M: 0, Y: 0, K: 1}
	}
	return CMYKColor{
		C: (1 - r - k) / (1 - k),
		M: (1 - g - k) / (1 - k),
		Y: (1 - b - k) / (1 - k),
		K: k,
	}
}

// SetFillCMYK sets the non-stroking (fill) color to a DeviceCMYK color for
// subsequent fills and text, emitting the "k" operator.
func (p *Page) SetFillCMYK(c CMYKColor) {
	p.op(f2s(clamp01(c.C)), f2s(clamp01(c.M)), f2s(clamp01(c.Y)), f2s(clamp01(c.K)), "k")
}

// SetStrokeCMYK sets the stroking color to a DeviceCMYK color for subsequent
// strokes, emitting the "K" operator.
func (p *Page) SetStrokeCMYK(c CMYKColor) {
	p.op(f2s(clamp01(c.C)), f2s(clamp01(c.M)), f2s(clamp01(c.Y)), f2s(clamp01(c.K)), "K")
}

// HSL constructs an RGB Color from hue (in degrees, wrapped to [0, 360)),
// saturation and lightness (each in [0, 1]). HSL(0,1,0.5) is pure red,
// HSL(120,1,0.5) is green and HSL(240,1,0.5) is blue.
func HSL(h, s, l float64) Color {
	s = clamp01(s)
	l = clamp01(l)
	h = h - 360*floorDiv(h, 360) // wrap to [0,360)
	if s == 0 {
		return Color{R: l, G: l, B: l}
	}
	var q float64
	if l < 0.5 {
		q = l * (1 + s)
	} else {
		q = l + s - l*s
	}
	pp := 2*l - q
	hk := h / 360
	return Color{
		R: hueToRGB(pp, q, hk+1.0/3.0),
		G: hueToRGB(pp, q, hk),
		B: hueToRGB(pp, q, hk-1.0/3.0),
	}
}

// Hex parses a CSS-style hex color string such as "#ff8800", "ff8800", "#f80"
// or "f80" into a Color. A leading '#' is optional; both 3-digit shorthand and
// 6-digit forms are accepted. It returns an error for any other length or for
// non-hexadecimal digits.
func Hex(s string) (Color, error) {
	t := strings.TrimSpace(s)
	t = strings.TrimPrefix(t, "#")
	switch len(t) {
	case 3:
		r, err1 := strconv.ParseUint(string([]byte{t[0], t[0]}), 16, 8)
		g, err2 := strconv.ParseUint(string([]byte{t[1], t[1]}), 16, 8)
		b, err3 := strconv.ParseUint(string([]byte{t[2], t[2]}), 16, 8)
		if err1 != nil || err2 != nil || err3 != nil {
			return Color{}, fmt.Errorf("pdfkit: invalid hex color %q", s)
		}
		return RGB(uint8(r), uint8(g), uint8(b)), nil
	case 6:
		v, err := strconv.ParseUint(t, 16, 32)
		if err != nil {
			return Color{}, fmt.Errorf("pdfkit: invalid hex color %q", s)
		}
		return RGB(uint8(v>>16), uint8(v>>8), uint8(v)), nil
	default:
		return Color{}, fmt.Errorf("pdfkit: invalid hex color %q: want 3 or 6 digits", s)
	}
}

// Hex formats the color as a lowercase 6-digit "#rrggbb" hex string. Each
// component is rounded to the nearest 8-bit value.
func (c Color) Hex() string {
	return fmt.Sprintf("#%02x%02x%02x", to255(c.R), to255(c.G), to255(c.B))
}

// hueToRGB is the helper used by HSL to convert a hue fraction to a channel
// value given the two intermediate coefficients p and q.
func hueToRGB(p, q, t float64) float64 {
	if t < 0 {
		t += 1
	}
	if t > 1 {
		t -= 1
	}
	switch {
	case t < 1.0/6.0:
		return p + (q-p)*6*t
	case t < 1.0/2.0:
		return q
	case t < 2.0/3.0:
		return p + (q-p)*(2.0/3.0-t)*6
	default:
		return p
	}
}

// to255 rounds a [0,1] component to an 8-bit integer.
func to255(v float64) uint8 {
	v = clamp01(v)
	return uint8(v*255 + 0.5)
}

// maxf returns the larger of two float64 values.
func maxf(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// floorDiv returns the mathematical floor of a/b as a whole number, used to wrap
// hue angles without importing math.
func floorDiv(a, b float64) float64 {
	q := a / b
	fq := float64(int64(q))
	if q < 0 && fq != q {
		fq--
	}
	return fq
}
