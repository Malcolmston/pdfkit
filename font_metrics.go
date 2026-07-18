package pdfkit

// fontVMetrics holds vertical font metrics in 1000-unit glyph space, matching
// the em scale used by the width tables. Values come from the canonical Adobe
// AFM files for the 14 standard fonts.
type fontVMetrics struct {
	ascent    int
	descent   int // negative
	capHeight int
	xHeight   int
}

// standardVMetrics maps each standard PDF base-font name to its vertical
// metrics. Symbol and ZapfDingbats do not define Ascender/Descender in their
// AFM files, so they are reported as zero.
var standardVMetrics = map[string]fontVMetrics{
	"Helvetica":             {718, -207, 718, 523},
	"Helvetica-Bold":        {718, -207, 718, 532},
	"Helvetica-Oblique":     {718, -207, 718, 523},
	"Helvetica-BoldOblique": {718, -207, 718, 532},
	"Times-Roman":           {683, -217, 662, 450},
	"Times-Bold":            {683, -217, 676, 461},
	"Times-Italic":          {683, -217, 653, 441},
	"Times-BoldItalic":      {683, -217, 669, 462},
	"Courier":               {629, -157, 562, 426},
	"Courier-Bold":          {629, -157, 562, 439},
	"Courier-Oblique":       {629, -157, 562, 426},
	"Courier-BoldOblique":   {629, -157, 562, 439},
	"Symbol":                {0, 0, 0, 0},
	"ZapfDingbats":          {0, 0, 0, 0},
}

// vmetrics returns the font's vertical metrics in 1000-unit glyph space,
// resolving embedded fonts through their head/hhea/OS2 tables and standard fonts
// through the built-in AFM table.
func (f *Font) vmetrics() fontVMetrics {
	if f.embedded != nil {
		tf := f.embedded
		return fontVMetrics{
			ascent:    tf.scale1000(tf.ascent),
			descent:   tf.scale1000(tf.descent),
			capHeight: tf.scale1000(tf.capHeight),
			xHeight:   0,
		}
	}
	return standardVMetrics[f.BaseFont]
}

// Ascent returns the font ascender — the maximum extent above the baseline — in
// points at the given size. This is the recommended distance from the baseline
// to the top of the tallest glyphs.
func (f *Font) Ascent(size float64) float64 {
	return float64(f.vmetrics().ascent) * size / 1000.0
}

// Descent returns the font descender — the maximum extent below the baseline —
// in points at the given size. The value is negative for fonts whose glyphs
// descend below the baseline.
func (f *Font) Descent(size float64) float64 {
	return float64(f.vmetrics().descent) * size / 1000.0
}

// CapHeight returns the cap height (the height of a flat capital letter such as
// 'H' above the baseline) in points at the given size.
func (f *Font) CapHeight(size float64) float64 {
	return float64(f.vmetrics().capHeight) * size / 1000.0
}

// XHeight returns the x-height (the height of the lowercase 'x' above the
// baseline) in points at the given size. It is zero for fonts that do not report
// an x-height, including embedded fonts, Symbol and ZapfDingbats.
func (f *Font) XHeight(size float64) float64 {
	return float64(f.vmetrics().xHeight) * size / 1000.0
}

// LineHeight returns the natural single-line height in points at the given size:
// the ascent minus the (negative) descent. Add extra leading between lines as
// desired; DrawParagraph and DrawTextBox take an explicit leading value.
func (f *Font) LineHeight(size float64) float64 {
	m := f.vmetrics()
	return float64(m.ascent-m.descent) * size / 1000.0
}
