package pdfkit

import (
	"math"
	"strings"
	"testing"
)

func TestLineCapJoinMiterOps(t *testing.T) {
	doc := New()
	pg := doc.AddPage(A4)
	pg.SetLineCap(CapRound)
	pg.SetLineJoin(JoinBevel)
	pg.SetMiterLimit(4)
	pg.SetMiterLimit(0.5) // clamps to 1
	content := string(pg.content())
	for _, want := range []string{"1 J", "2 j", "4 M", "1 M"} {
		if !strings.Contains(content, want) {
			t.Errorf("content missing %q:\n%s", want, content)
		}
	}
}

func TestRoundedRectPathOps(t *testing.T) {
	doc := New()
	pg := doc.AddPage(A4)
	pg.RoundedRect(10, 20, 100, 50, 10)
	content := string(pg.content())
	// 4 corner curves and 4 straight edges, closed.
	if got := strings.Count(content, " c\n"); got != 4 {
		t.Errorf("want 4 curves, got %d:\n%s", got, content)
	}
	if got := strings.Count(content, " l\n"); got != 4 {
		t.Errorf("want 4 lines, got %d", got)
	}
	if !strings.Contains(content, "h\n") {
		t.Errorf("rounded rect not closed")
	}
	if !strings.HasPrefix(content, "20 20 m") { // MoveTo(x+r, y) = (10+10, 20)
		t.Errorf("unexpected start op: %q", content[:20])
	}
}

func TestRoundedRectRadiusClampAndZero(t *testing.T) {
	// Zero radius falls back to a plain rectangle.
	doc := New()
	pg := doc.AddPage(A4)
	pg.RoundedRect(0, 0, 40, 40, 0)
	if !strings.Contains(string(pg.content()), "0 0 40 40 re") {
		t.Errorf("zero-radius rounded rect should emit re operator")
	}

	// Radius larger than half the short side is clamped; still 4 curves.
	pg2 := doc.AddPage(A4)
	pg2.RoundedRect(0, 0, 40, 20, 1000)
	if got := strings.Count(string(pg2.content()), " c\n"); got != 4 {
		t.Errorf("clamped rounded rect should still have 4 curves, got %d", got)
	}
}

func TestPolygonPolyline(t *testing.T) {
	doc := New()
	pg := doc.AddPage(A4)
	pg.Polyline(0, 0, 10, 0, 10, 10)
	c1 := string(pg.content())
	if strings.Count(c1, " l\n") != 2 || strings.Contains(c1, "h\n") {
		t.Errorf("polyline should have 2 lines and no close:\n%s", c1)
	}

	pg2 := doc.AddPage(A4)
	pg2.Polygon(0, 0, 10, 0, 10, 10)
	c2 := string(pg2.content())
	if strings.Count(c2, " l\n") != 2 || !strings.Contains(c2, "h\n") {
		t.Errorf("polygon should have 2 lines and a close:\n%s", c2)
	}

	// No-ops for degenerate input.
	pg3 := doc.AddPage(A4)
	pg3.Polygon(0, 0, 1, 1) // only 2 points
	pg3.Polyline(0, 0)      // 1 point
	if len(pg3.content()) != 0 {
		t.Errorf("degenerate polygon/polyline should be no-ops, got: %q", pg3.content())
	}
}

func TestRegularPolygonVertices(t *testing.T) {
	doc := New()
	pg := doc.AddPage(A4)
	pg.RegularPolygon(100, 100, 50, 4) // square, first vertex at top
	content := string(pg.content())
	// A square has 1 MoveTo + 3 LineTo + close.
	if strings.Count(content, " m\n") != 1 || strings.Count(content, " l\n") != 3 {
		t.Errorf("square regular polygon op count wrong:\n%s", content)
	}
	if !strings.HasPrefix(content, "100 150 m") {
		t.Errorf("first vertex should be top (100,150), got: %q", content[:12])
	}

	// Fewer than 3 sides is a no-op.
	pg2 := doc.AddPage(A4)
	pg2.RegularPolygon(0, 0, 10, 2)
	if len(pg2.content()) != 0 {
		t.Errorf("regular polygon with <3 sides should be a no-op")
	}
}

func TestTrigApprox(t *testing.T) {
	tests := []struct{ x, sin, cos float64 }{
		{0, 0, 1},
		{pi / 2, 1, 0},
		{pi, 0, -1},
		{3 * pi / 2, -1, 0},
		{pi / 6, 0.5, math.Sqrt(3) / 2},
	}
	for _, tt := range tests {
		if s := sinApprox(tt.x); math.Abs(s-tt.sin) > 1e-6 {
			t.Errorf("sinApprox(%v) = %v, want %v", tt.x, s, tt.sin)
		}
		if c := cosApprox(tt.x); math.Abs(c-tt.cos) > 1e-6 {
			t.Errorf("cosApprox(%v) = %v, want %v", tt.x, c, tt.cos)
		}
	}
}

func BenchmarkRoundedRect(b *testing.B) {
	doc := New()
	pg := doc.AddPage(A4)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pg.RoundedRect(0, 0, 100, 50, 8)
	}
}

func BenchmarkRegularPolygon(b *testing.B) {
	doc := New()
	pg := doc.AddPage(A4)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pg.RegularPolygon(100, 100, 40, 7)
	}
}
