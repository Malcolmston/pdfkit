package pdfkit

// WrapText greedily breaks text into lines no wider than maxWidth points when
// set in font f at the given size, breaking only at spaces and tabs. Existing
// newline characters force a break. A single word longer than maxWidth is kept
// on its own line rather than split mid-word. It is the exported form of the
// word-wrapping used by Page.DrawParagraph.
func WrapText(f *Font, size, maxWidth float64, text string) []string {
	if f == nil {
		return nil
	}
	return wrapText(f, size, maxWidth, text)
}

// HeightOfString returns the total height in points needed to render text set in
// font f at the given size, wrapped to maxWidth, with lineGap points of extra
// leading added between (and after) each line. It measures the number of wrapped
// lines and multiplies by the per-line advance, mirroring the accounting used
// when laying paragraphs out with Page.DrawTextBox.
func (f *Font) HeightOfString(text string, size, maxWidth, lineGap float64) float64 {
	if f == nil {
		return 0
	}
	lines := wrapText(f, size, maxWidth, text)
	if len(lines) == 0 {
		return 0
	}
	return float64(len(lines)) * (f.LineHeight(size) + lineGap)
}

// TruncateToWidth returns text shortened so that, with ellipsis appended, it
// fits within maxWidth points in font f at the given size. If the full text
// already fits it is returned unchanged. Truncation happens at rune boundaries.
// If even the ellipsis alone does not fit, the ellipsis is returned.
func TruncateToWidth(f *Font, size, maxWidth float64, text, ellipsis string) string {
	if f == nil {
		return text
	}
	if f.Width(text, size) <= maxWidth {
		return text
	}
	ell := f.Width(ellipsis, size)
	runes := []rune(text)
	// Grow a prefix while prefix+ellipsis still fits.
	best := 0
	for i := 1; i <= len(runes); i++ {
		if f.Width(string(runes[:i]), size)+ell <= maxWidth {
			best = i
		} else {
			break
		}
	}
	if best == 0 {
		return ellipsis
	}
	return string(runes[:best]) + ellipsis
}
