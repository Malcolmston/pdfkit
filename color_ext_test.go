package pdfkit

import (
	"math"
	"strings"
	"testing"
)

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestCMYKToRGB(t *testing.T) {
	tests := []struct {
		c       CMYKColor
		r, g, b float64
	}{
		{CMYK(0, 0, 0, 0), 1, 1, 1}, // white
		{CMYK(0, 0, 0, 1), 0, 0, 0}, // black
		{CMYK(1, 0, 0, 0), 0, 1, 1}, // cyan
		{CMYK(0, 1, 0, 0), 1, 0, 1}, // magenta
		{CMYK(0, 0, 1, 0), 1, 1, 0}, // yellow
		{CMYK(0, 0, 0, 0.5), 0.5, 0.5, 0.5},
	}
	for _, tt := range tests {
		got := tt.c.RGB()
		if !approx(got.R, tt.r) || !approx(got.G, tt.g) || !approx(got.B, tt.b) {
			t.Errorf("%v.RGB() = %v, want (%v,%v,%v)", tt.c, got, tt.r, tt.g, tt.b)
		}
	}
}

func TestRGBToCMYK(t *testing.T) {
	// Round-trip primaries.
	tests := []Color{{1, 1, 1}, {0, 0, 0}, {1, 0, 0}, {0, 1, 0}, {0, 0, 1}, {0.5, 0.25, 0.75}}
	for _, c := range tests {
		back := c.CMYK().RGB()
		if !approx(back.R, c.R) || !approx(back.G, c.G) || !approx(back.B, c.B) {
			t.Errorf("CMYK round-trip of %v = %v", c, back)
		}
	}
	if k := (Color{0, 0, 0}).CMYK().K; !approx(k, 1) {
		t.Errorf("black K = %v, want 1", k)
	}
}

func TestHSL(t *testing.T) {
	tests := []struct {
		h, s, l float64
		r, g, b float64
	}{
		{0, 0, 0, 0, 0, 0},         // black
		{0, 0, 1, 1, 1, 1},         // white
		{0, 0, 0.5, 0.5, 0.5, 0.5}, // gray
		{0, 1, 0.5, 1, 0, 0},       // red
		{120, 1, 0.5, 0, 1, 0},     // green
		{240, 1, 0.5, 0, 0, 1},     // blue
		{360, 1, 0.5, 1, 0, 0},     // wraps to red
	}
	for _, tt := range tests {
		got := HSL(tt.h, tt.s, tt.l)
		if !approx(got.R, tt.r) || !approx(got.G, tt.g) || !approx(got.B, tt.b) {
			t.Errorf("HSL(%v,%v,%v) = %v, want (%v,%v,%v)", tt.h, tt.s, tt.l, got, tt.r, tt.g, tt.b)
		}
	}
}

func TestHexParse(t *testing.T) {
	tests := []struct {
		in      string
		r, g, b uint8
		wantErr bool
	}{
		{"#ff0000", 255, 0, 0, false},
		{"00ff00", 0, 255, 0, false},
		{"#0000ff", 0, 0, 255, false},
		{"#f80", 255, 136, 0, false},
		{"abc", 170, 187, 204, false},
		{"#fff", 255, 255, 255, false},
		{"#12345", 0, 0, 0, true},
		{"#gggggg", 0, 0, 0, true},
	}
	for _, tt := range tests {
		got, err := Hex(tt.in)
		if tt.wantErr {
			if err == nil {
				t.Errorf("Hex(%q) expected error", tt.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("Hex(%q) unexpected error: %v", tt.in, err)
			continue
		}
		want := RGB(tt.r, tt.g, tt.b)
		if !approx(got.R, want.R) || !approx(got.G, want.G) || !approx(got.B, want.B) {
			t.Errorf("Hex(%q) = %v, want %v", tt.in, got, want)
		}
	}
}

func TestColorHexString(t *testing.T) {
	tests := []struct {
		c    Color
		want string
	}{
		{Red, "#ff0000"},
		{Green, "#00ff00"},
		{Blue, "#0000ff"},
		{White, "#ffffff"},
		{Black, "#000000"},
		{RGB(255, 136, 0), "#ff8800"},
	}
	for _, tt := range tests {
		if got := tt.c.Hex(); got != tt.want {
			t.Errorf("%v.Hex() = %q, want %q", tt.c, got, tt.want)
		}
	}
}

func TestHexRoundTrip(t *testing.T) {
	for _, s := range []string{"#ff8800", "#123456", "#abcdef", "#000000"} {
		c, err := Hex(s)
		if err != nil {
			t.Fatal(err)
		}
		if got := c.Hex(); got != s {
			t.Errorf("round trip %q -> %q", s, got)
		}
	}
}

func TestSetCMYKOps(t *testing.T) {
	doc := New()
	pg := doc.AddPage(A4)
	pg.SetFillCMYK(CMYK(0.1, 0.2, 0.3, 0.4))
	pg.SetStrokeCMYK(CMYK(1, 0, 0, 0))
	content := string(pg.content())
	if !strings.Contains(content, "0.1 0.2 0.3 0.4 k") {
		t.Errorf("missing fill CMYK op in:\n%s", content)
	}
	if !strings.Contains(content, "1 0 0 0 K") {
		t.Errorf("missing stroke CMYK op in:\n%s", content)
	}
}
