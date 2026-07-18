package pdfkit

import (
	"strings"
	"testing"
)

func TestAlignOffset(t *testing.T) {
	tests := []struct {
		align           TextAlign
		box, line, want float64
	}{
		{AlignLeft, 100, 40, 0},
		{AlignCenter, 100, 40, 30},
		{AlignRight, 100, 40, 60},
		{AlignJustify, 100, 40, 0},
	}
	for _, tt := range tests {
		if got := alignOffset(tt.align, tt.box, tt.line); got != tt.want {
			t.Errorf("alignOffset(%v,%v,%v) = %v, want %v", tt.align, tt.box, tt.line, got, tt.want)
		}
	}
}

func TestDrawTextAlignedPositions(t *testing.T) {
	doc := New()
	pg := doc.AddPage(A4)
	pg.SetFont(Helvetica, 12)
	text := "Hi"
	tw := Helvetica.Width(text, 12)

	pg.DrawTextAligned(0, 700, 200, text, AlignRight)
	content := string(pg.content())
	wantX := f2s(200 - tw)
	if !strings.Contains(content, wantX+" 700 Td") {
		t.Errorf("right-aligned Td x should be %s:\n%s", wantX, content)
	}
}

func TestDrawTextCenteredAndRight(t *testing.T) {
	doc := New()
	pg := doc.AddPage(A4)
	pg.SetFont(Helvetica, 12)
	tw := Helvetica.Width("X", 12)

	pg.DrawTextCentered(100, 500, "X")
	if !strings.Contains(string(pg.content()), f2s(100-tw/2)+" 500 Td") {
		t.Errorf("centered text x wrong:\n%s", pg.content())
	}

	pg2 := doc.AddPage(A4)
	pg2.SetFont(Helvetica, 12)
	pg2.DrawTextRight(300, 500, "X")
	if !strings.Contains(string(pg2.content()), f2s(300-tw)+" 500 Td") {
		t.Errorf("right text x wrong:\n%s", pg2.content())
	}
}

func TestDrawTextBoxReturnsNextY(t *testing.T) {
	doc := New()
	pg := doc.AddPage(A4)
	pg.SetFont(Helvetica, 10)
	next := pg.DrawTextBox(50, 700, 60, 12, "aaaa aaaa aaaa", TextOptions{Align: AlignLeft})
	// 3 wrapped words at width 60: "aaaa aaaa" fits (~47.26) then "aaaa".
	lines := WrapText(Helvetica, 10, 60, "aaaa aaaa aaaa")
	wantY := 700 - float64(len(lines))*12
	if next != wantY {
		t.Errorf("DrawTextBox next y = %v, want %v", next, wantY)
	}
	// One Tj per line.
	if got := strings.Count(string(pg.content()), " Tj\n"); got != len(lines) {
		t.Errorf("want %d Tj, got %d", len(lines), got)
	}
}

func TestDrawTextBoxDefaultLeading(t *testing.T) {
	doc := New()
	pg := doc.AddPage(A4)
	pg.SetFont(Helvetica, 10)
	// leading 0 => use LineHeight + LineGap.
	next := pg.DrawTextBox(0, 100, 1000, 0, "single line", TextOptions{LineGap: 2})
	want := 100 - (Helvetica.LineHeight(10) + 2)
	if next != want {
		t.Errorf("default-leading next y = %v, want %v", next, want)
	}
}

func TestDrawTextBoxDecorations(t *testing.T) {
	doc := New()
	pg := doc.AddPage(A4)
	pg.SetFont(Helvetica, 10)
	pg.DrawTextBox(0, 100, 1000, 12, "underlined", TextOptions{Underline: true, Strike: true})
	content := string(pg.content())
	// Underline + strike each add a stroked segment.
	if strings.Count(content, "\nS\n") < 2 {
		t.Errorf("expected underline and strike strokes:\n%s", content)
	}
}

func TestDrawTextBoxJustify(t *testing.T) {
	doc := New()
	pg := doc.AddPage(A4)
	pg.SetFont(Helvetica, 10)
	// Narrow box forces wrapping; non-last lines are justified word-by-word.
	pg.DrawTextBox(0, 100, 60, 12, "aa bb cc dd ee ff gg hh", TextOptions{Align: AlignJustify})
	content := string(pg.content())
	// Justify draws each word of a filled line individually, so there are many Tj ops.
	if strings.Count(content, " Tj\n") < 4 {
		t.Errorf("justify should emit multiple word Tj ops:\n%s", content)
	}
}

func TestTextStyleNoFontNoop(t *testing.T) {
	doc := New()
	pg := doc.AddPage(A4)
	pg.DrawTextAligned(0, 0, 100, "x", AlignCenter)
	pg.DrawTextCentered(0, 0, "x")
	pg.DrawTextRight(0, 0, "x")
	if y := pg.DrawTextBox(0, 50, 100, 10, "x", TextOptions{}); y != 50 {
		t.Errorf("no-font DrawTextBox should return y unchanged, got %v", y)
	}
	if len(pg.content()) != 0 {
		t.Errorf("no-font text ops should produce no content")
	}
}
