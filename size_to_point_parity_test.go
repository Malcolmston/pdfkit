package pdfkit

import (
	"math"
	"testing"
)

// TestParitySizeToPoint encodes the known-answer vectors from upstream PDFKit's
// tests/unit/font.spec.js "sizeToPoint" test. The upstream document under test
// is configured with font Helvetica, fontSize 12 and page size [250, 500], and
// asserts each conversion to 4 decimal places (toBeCloseTo(expected, 4)).
func TestParitySizeToPoint(t *testing.T) {
	ctx := SizeContext{
		Font:     Helvetica,
		FontSize: 12,
		Page:     PageSize{Width: 250, Height: 500},
	}
	cases := []struct {
		in   string
		want float64
	}{
		{"1", 1},
		{"1em", 12},
		{"1in", 72},
		{"1px", 0.75},
		{"1cm", 28.3465},
		{"1mm", 2.8346},
		{"1pc", 12},
		{"1ex", 11.1},
		{"1ch", 6.672},
		{"1vw", 2.5},
		{"1vh", 5},
		{"1vmin", 2.5},
		{"1vmax", 5},
		{"1%", 0.12},
		{"1pt", 1},
		{"1rem", 12},
	}
	for _, c := range cases {
		got, ok := SizeToPoint(c.in, 0, ctx)
		if !ok {
			t.Errorf("%s: not understood", c.in)
			continue
		}
		// toBeCloseTo(expected, 4) passes when |diff| < 0.5e-4.
		if math.Abs(got-c.want) >= 0.5e-4 {
			t.Errorf("SizeToPoint(%q) = %.6f, want %.6f", c.in, got, c.want)
		}
	}

	// Upstream boolean literals: true -> 1, false -> 0.
	if v, _ := SizeToPoint("true", 0, ctx); v != 1 {
		t.Errorf("true -> %v, want 1", v)
	}
	if v, _ := SizeToPoint("false", 0, ctx); v != 0 {
		t.Errorf("false -> %v, want 0", v)
	}

	// Upstream "1rem -> 12" test: with fontSize 15, "1em" follows the font
	// size (15) while "1rem" stays at the root em size (12).
	ctx15 := ctx
	ctx15.FontSize = 15
	if v, _ := SizeToPoint("1em", 0, ctx15); v != 15 {
		t.Errorf("1em @15 -> %v, want 15", v)
	}
	if v, _ := SizeToPoint("1rem", 0, ctx15); v != 12 {
		t.Errorf("1rem @15 -> %v, want 12", v)
	}
}

// TestParityFontWidthsAFM checks a representative set of the Adobe Font Metrics
// advance widths PDFKit ships and measures against. Widths are in 1000-unit
// glyph space; the expected values are the canonical AFM widths for the named
// glyphs, which PDFKit's afm data reproduces exactly.
func TestParityFontWidthsAFM(t *testing.T) {
	cases := []struct {
		font *Font
		ch   string
		w    int
	}{
		{Helvetica, " ", 278},
		{Helvetica, "A", 667},
		{Helvetica, "W", 944},
		{Helvetica, "0", 556},
		{HelveticaBold, " ", 278},
		{HelveticaBold, "A", 722},
		{TimesRoman, " ", 250},
		{TimesRoman, "A", 722},
		{TimesRoman, "W", 944},
		{Courier, "A", 600},
		{Courier, "i", 600},
	}
	for _, c := range cases {
		// Width at size 1000 recovers the raw AFM advance.
		got := c.font.Width(c.ch, 1000)
		if math.Abs(got-float64(c.w)) > 1e-9 {
			t.Errorf("%s width of %q = %v, want %d", c.font.BaseFont, c.ch, got, c.w)
		}
	}
}
