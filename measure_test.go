package pdfkit

import (
	"testing"
)

func TestWrapText(t *testing.T) {
	// Each word "aaaa" in Helvetica 10 is 4*556*10/1000 = 22.24 pt wide;
	// a space is 278*10/1000 = 2.78 pt.
	lines := WrapText(Helvetica, 10, 50, "aaaa aaaa aaaa")
	if len(lines) != 2 {
		t.Fatalf("want 2 wrapped lines, got %d: %q", len(lines), lines)
	}
	if lines[0] != "aaaa aaaa" || lines[1] != "aaaa" {
		t.Errorf("unexpected wrap: %q", lines)
	}

	// Nil font is a nil result.
	if WrapText(nil, 10, 50, "x") != nil {
		t.Errorf("WrapText(nil,...) should be nil")
	}

	// Existing newlines force breaks.
	if got := WrapText(Helvetica, 10, 1000, "a\nb\nc"); len(got) != 3 {
		t.Errorf("newlines should force 3 lines, got %d", len(got))
	}
}

func TestHeightOfString(t *testing.T) {
	// Two lines, lineGap 0 => 2 * LineHeight(10).
	h := Helvetica.HeightOfString("aaaa aaaa aaaa", 10, 50, 0)
	want := 2 * Helvetica.LineHeight(10)
	if h != want {
		t.Errorf("HeightOfString = %v, want %v", h, want)
	}
	// With lineGap 3 => 2 * (LineHeight + 3).
	h2 := Helvetica.HeightOfString("aaaa aaaa aaaa", 10, 50, 3)
	want2 := 2 * (Helvetica.LineHeight(10) + 3)
	if h2 != want2 {
		t.Errorf("HeightOfString gap = %v, want %v", h2, want2)
	}
}

func TestTruncateToWidth(t *testing.T) {
	full := "Hello, world!"
	// Full text fits in a wide box: unchanged.
	if got := TruncateToWidth(Helvetica, 12, 1000, full, "..."); got != full {
		t.Errorf("wide box should not truncate, got %q", got)
	}
	// Narrow box truncates and appends ellipsis.
	got := TruncateToWidth(Helvetica, 12, 40, full, "...")
	if got == full || len(got) == 0 {
		t.Errorf("expected truncation, got %q", got)
	}
	if got[len(got)-3:] != "..." {
		t.Errorf("truncated text should end with ellipsis, got %q", got)
	}
	// Result width must fit.
	if Helvetica.Width(got, 12) > 40+1e-9 {
		t.Errorf("truncated %q exceeds max width", got)
	}
}

func BenchmarkWrapText(b *testing.B) {
	text := "the quick brown fox jumps over the lazy dog and keeps on running past the fence"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		WrapText(Helvetica, 11, 200, text)
	}
}
