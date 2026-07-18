package pdfkit

import "testing"

func TestStandardFontMetrics1000(t *testing.T) {
	// Known AFM values at size 1000 return the raw glyph-space numbers.
	tests := []struct {
		f                  *Font
		asc, desc, cap, xh float64
	}{
		{Helvetica, 718, -207, 718, 523},
		{HelveticaBold, 718, -207, 718, 532},
		{TimesRoman, 683, -217, 662, 450},
		{TimesBold, 683, -217, 676, 461},
		{TimesItalic, 683, -217, 653, 441},
		{Courier, 629, -157, 562, 426},
	}
	for _, tt := range tests {
		if got := tt.f.Ascent(1000); got != tt.asc {
			t.Errorf("%s Ascent = %v, want %v", tt.f.BaseFont, got, tt.asc)
		}
		if got := tt.f.Descent(1000); got != tt.desc {
			t.Errorf("%s Descent = %v, want %v", tt.f.BaseFont, got, tt.desc)
		}
		if got := tt.f.CapHeight(1000); got != tt.cap {
			t.Errorf("%s CapHeight = %v, want %v", tt.f.BaseFont, got, tt.cap)
		}
		if got := tt.f.XHeight(1000); got != tt.xh {
			t.Errorf("%s XHeight = %v, want %v", tt.f.BaseFont, got, tt.xh)
		}
	}
}

func TestFontMetricsScaling(t *testing.T) {
	// At size 12, Helvetica ascent = 718 * 12 / 1000 = 8.616.
	if got := Helvetica.Ascent(12); got != 8.616 {
		t.Errorf("Helvetica.Ascent(12) = %v, want 8.616", got)
	}
	// LineHeight = (718 - (-207)) * 12 / 1000 = 11.1.
	if got := Helvetica.LineHeight(12); got != 11.1 {
		t.Errorf("Helvetica.LineHeight(12) = %v, want 11.1", got)
	}
}

func TestLineHeightAllStandardPositive(t *testing.T) {
	for name, f := range standardFonts {
		lh := f.LineHeight(10)
		if name == "Symbol" || name == "ZapfDingbats" {
			if lh != 0 {
				t.Errorf("%s LineHeight expected 0 (no AFM asc/desc), got %v", name, lh)
			}
			continue
		}
		if lh <= 0 {
			t.Errorf("%s LineHeight should be positive, got %v", name, lh)
		}
	}
}
