package pdfkit

// TextAlign selects how lines of text are positioned horizontally within a box
// of a given width by Page.DrawTextAligned and Page.DrawTextBox.
type TextAlign int

// The supported horizontal text alignments.
const (
	// AlignLeft aligns each line to the left edge of the box (default).
	AlignLeft TextAlign = iota
	// AlignCenter centers each line within the box.
	AlignCenter
	// AlignRight aligns each line to the right edge of the box.
	AlignRight
	// AlignJustify stretches inter-word spacing so every line except the last
	// of each paragraph fills the full box width.
	AlignJustify
)

// TextOptions controls how Page.DrawTextBox lays out and decorates a block of
// wrapped text.
type TextOptions struct {
	// Align sets the horizontal alignment of each line.
	Align TextAlign
	// Underline draws a line beneath each rendered line of text.
	Underline bool
	// Strike draws a line through the middle of each rendered line of text.
	Strike bool
	// LineGap adds extra leading (in points) between lines on top of the font's
	// natural line height. It is used only when leading is not supplied to
	// DrawTextBox (a leading of 0 means "use the font line height plus LineGap").
	LineGap float64
}

// alignOffset returns the x displacement to apply to a line of the given width
// so that it is aligned within a box of boxWidth points under align.
func alignOffset(align TextAlign, boxWidth, lineWidth float64) float64 {
	switch align {
	case AlignCenter:
		return (boxWidth - lineWidth) / 2
	case AlignRight:
		return boxWidth - lineWidth
	default:
		return 0
	}
}

// DrawTextAligned draws a single line of text within a box of the given width
// whose left edge is at x, positioning the baseline at y and aligning the line
// horizontally per align. AlignJustify behaves like AlignLeft for a single line.
// It is a no-op if no font has been set.
func (p *Page) DrawTextAligned(x, y, width float64, text string, align TextAlign) {
	if p.curFont == nil {
		return
	}
	tw := p.curFont.Width(text, p.curSize)
	p.DrawText(x+alignOffset(align, width, tw), y, text)
}

// DrawTextCentered draws a single line of text centered horizontally on cx, with
// its baseline at y. It is a no-op if no font has been set.
func (p *Page) DrawTextCentered(cx, y float64, text string) {
	if p.curFont == nil {
		return
	}
	tw := p.curFont.Width(text, p.curSize)
	p.DrawText(cx-tw/2, y, text)
}

// DrawTextRight draws a single line of text so that its right edge ends at x,
// with the baseline at y. It is a no-op if no font has been set.
func (p *Page) DrawTextRight(x, y float64, text string) {
	if p.curFont == nil {
		return
	}
	tw := p.curFont.Width(text, p.curSize)
	p.DrawText(x-tw, y, text)
}

// DrawUnderline strokes a horizontal underline of the given width starting at
// (x, y), using the current stroke color. The stroke width scales with the
// current font size. It draws relative to a baseline at y, placing the line
// slightly below it.
func (p *Page) DrawUnderline(x, y, width float64) {
	thickness := p.curSize * 0.05
	if thickness <= 0 {
		thickness = 0.5
	}
	off := p.curSize * 0.1
	p.op("q")
	p.op(f2s(thickness), "w")
	p.MoveTo(x, y-off)
	p.LineTo(x+width, y-off)
	p.Stroke()
	p.op("Q")
}

// DrawStrikethrough strokes a horizontal line through the middle of a line of
// text of the given width starting at baseline (x, y), using the current stroke
// color. The line is positioned near the font's x-height midpoint.
func (p *Page) DrawStrikethrough(x, y, width float64) {
	thickness := p.curSize * 0.05
	if thickness <= 0 {
		thickness = 0.5
	}
	off := p.curSize * 0.3
	p.op("q")
	p.op(f2s(thickness), "w")
	p.MoveTo(x, y+off)
	p.LineTo(x+width, y+off)
	p.Stroke()
	p.op("Q")
}

// DrawTextBox lays text out into a box whose left edge is at x and top baseline
// is at y, wrapping to width points and advancing downward by leading points per
// line. If leading is 0 the font's natural line height plus opts.LineGap is
// used. Lines are aligned per opts.Align, and optionally underlined or struck
// through. It returns the y coordinate of the baseline just past the last drawn
// line. It is a no-op returning y unchanged if no font has been set.
func (p *Page) DrawTextBox(x, y, width, leading float64, text string, opts TextOptions) float64 {
	if p.curFont == nil {
		return y
	}
	if leading <= 0 {
		leading = p.curFont.LineHeight(p.curSize) + opts.LineGap
	}
	paras := splitLines(text)
	cy := y
	for _, para := range paras {
		lines := wrapText(p.curFont, p.curSize, width, para)
		for li, line := range lines {
			isLastLine := li == len(lines)-1
			if opts.Align == AlignJustify && !isLastLine {
				p.drawJustifiedLine(x, cy, width, line)
			} else {
				tw := p.curFont.Width(line, p.curSize)
				lx := x + alignOffset(opts.Align, width, tw)
				p.DrawText(lx, cy, line)
				if opts.Underline {
					p.DrawUnderline(lx, cy, tw)
				}
				if opts.Strike {
					p.DrawStrikethrough(lx, cy, tw)
				}
			}
			cy -= leading
		}
	}
	return cy
}

// drawJustifiedLine draws a single line whose words are spread so their combined
// advance plus inter-word gaps exactly fills width. Lines with one or zero words
// are drawn left-aligned.
func (p *Page) drawJustifiedLine(x, y, width float64, line string) {
	words := splitSpaces(line)
	if len(words) <= 1 {
		p.DrawText(x, y, line)
		return
	}
	var wordsWidth float64
	for _, w := range words {
		wordsWidth += p.curFont.Width(w, p.curSize)
	}
	gap := (width - wordsWidth) / float64(len(words)-1)
	if gap < 0 {
		gap = 0
	}
	cx := x
	for _, w := range words {
		p.DrawText(cx, y, w)
		cx += p.curFont.Width(w, p.curSize) + gap
	}
}
