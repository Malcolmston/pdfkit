package pdfkit

import (
	"math"
	"testing"
)

// approxEq reports whether a and b are within a small absolute tolerance,
// suitable for comparing the fractional color components PDFKit computes.
func approxEq(a, b, tol float64) bool {
	return math.Abs(a-b) <= tol
}

// TestParityNormalizeColor encodes the known-answer vectors asserted by
// upstream PDFKit's tests/unit/color.spec.js "normalize" test. Each expected
// value is copied verbatim from the upstream assertions (including its
// deliberately unclamped CMYK-array results).
func TestParityNormalizeColor(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want []float64
	}{
		{"#FFF", "#FFF", []float64{1, 1, 1}},
		{"#FFFFFF", "#FFFFFF", []float64{1, 1, 1}},
		{"#000", "#000", []float64{0, 0, 0}},
		{"#000000", "#000000", []float64{0, 0, 0}},
		{"#6F6FEF", "#6F6FEF", []float64{
			0.43529411764705883, 0.43529411764705883, 0.9372549019607843,
		}},
		{"rgb white", []int{255, 255, 255}, []float64{1, 1, 1}},
		{"rgba->cmyk white", []int{255, 255, 255, 255}, []float64{2.55, 2.55, 2.55, 2.55}},
		{"rgb black", []int{0, 0, 0}, []float64{0, 0, 0}},
		{"cmyk zeros", []int{0, 0, 0, 0}, []float64{0, 0, 0, 0}},
		{"rgb dark red", []int{128, 10, 18}, []float64{
			0.5019607843137255, 0.0392156862745098, 0.07058823529411765,
		}},
		{"cmyk 128,10,18,100", []int{128, 10, 18, 100}, []float64{1.28, 0.1, 0.18, 1}},
	}
	for _, c := range cases {
		got := NormalizeColor(c.in)
		if len(got) != len(c.want) {
			t.Errorf("%s: len(got)=%d want %d (%v)", c.name, len(got), len(c.want), got)
			continue
		}
		for i := range c.want {
			if !approxEq(got[i], c.want[i], 1e-12) {
				t.Errorf("%s: got[%d]=%.17g want %.17g", c.name, i, got[i], c.want[i])
			}
		}
	}
}

// TestParityNamedColors verifies that PDFKit's CSS named-color table is ported
// with the exact RGB triples the upstream namedColors map defines. The sample
// spans the alphabet and includes the gray/grey spelling variants and the
// aqua/cyan and fuchsia/magenta aliases that upstream defines identically.
func TestParityNamedColors(t *testing.T) {
	cases := []struct {
		name    string
		r, g, b uint8
	}{
		{"aliceblue", 240, 248, 255},
		{"black", 0, 0, 0},
		{"white", 255, 255, 255},
		{"red", 255, 0, 0},
		{"green", 0, 128, 0},
		{"blue", 0, 0, 255},
		{"cornflowerblue", 100, 149, 237},
		{"rebeccapurple", 0, 0, 0}, // sentinel: not defined upstream (see below)
		{"gray", 128, 128, 128},
		{"grey", 128, 128, 128},
		{"aqua", 0, 255, 255},
		{"cyan", 0, 255, 255},
		{"fuchsia", 255, 0, 255},
		{"magenta", 255, 0, 255},
		{"midnightblue", 25, 25, 112},
		{"yellowgreen", 154, 205, 50},
	}
	for _, c := range cases {
		got, ok := NamedColor(c.name)
		if c.name == "rebeccapurple" {
			// PDFKit's table predates rebeccapurple; confirm it is absent.
			if ok {
				t.Errorf("rebeccapurple should not be defined in PDFKit's table")
			}
			continue
		}
		if !ok {
			t.Errorf("%s: not found", c.name)
			continue
		}
		want := RGB(c.r, c.g, c.b)
		if got != want {
			t.Errorf("%s: got %+v want %+v", c.name, got, want)
		}
	}
	// _normalizeColor resolves named colors to scaled components; spot-check
	// the cornflowerblue path end-to-end.
	got := NormalizeColor("cornflowerblue")
	want := []float64{100.0 / 255, 149.0 / 255, 237.0 / 255}
	for i := range want {
		if !approxEq(got[i], want[i], 1e-12) {
			t.Errorf("normalize cornflowerblue[%d]=%v want %v", i, got[i], want[i])
		}
	}
}
