package pdfkit

// Font represents one of the 14 standard PDF Type1 fonts. Each font carries a
// built-in glyph-width table (in 1000-unit glyph space) so that text widths can
// be measured without embedding or parsing external font files.
//
// Fonts are immutable and safe for concurrent use. Obtain them through the
// package-level font variables (for example Helvetica or TimesBold) or via
// StandardFont.
type Font struct {
	// BaseFont is the PDF /BaseFont name, e.g. "Helvetica" or "Times-Bold".
	BaseFont string
	// symbolic reports whether the font uses its own built-in encoding
	// (Symbol and ZapfDingbats) rather than WinAnsiEncoding.
	symbolic bool
	// widths holds glyph widths for character codes 32..126.
	widths *[95]int16
	// defWidth is used for character codes outside the table's range.
	defWidth int16
	// fixed, when non-zero, is used for every glyph (monospaced Courier).
	fixed int16
}

// The 14 standard fonts. These are always available in every conforming PDF
// viewer and never need to be embedded.
var (
	Helvetica            = &Font{BaseFont: "Helvetica", widths: &helveticaWidths, defWidth: 278}
	HelveticaBold        = &Font{BaseFont: "Helvetica-Bold", widths: &helveticaBoldWidths, defWidth: 278}
	HelveticaOblique     = &Font{BaseFont: "Helvetica-Oblique", widths: &helveticaWidths, defWidth: 278}
	HelveticaBoldOblique = &Font{BaseFont: "Helvetica-BoldOblique", widths: &helveticaBoldWidths, defWidth: 278}

	TimesRoman      = &Font{BaseFont: "Times-Roman", widths: &timesRomanWidths, defWidth: 250}
	TimesBold       = &Font{BaseFont: "Times-Bold", widths: &timesBoldWidths, defWidth: 250}
	TimesItalic     = &Font{BaseFont: "Times-Italic", widths: &timesItalicWidths, defWidth: 250}
	TimesBoldItalic = &Font{BaseFont: "Times-BoldItalic", widths: &timesBoldItalicWidths, defWidth: 250}

	Courier            = &Font{BaseFont: "Courier", fixed: courierWidth, defWidth: courierWidth}
	CourierBold        = &Font{BaseFont: "Courier-Bold", fixed: courierWidth, defWidth: courierWidth}
	CourierOblique     = &Font{BaseFont: "Courier-Oblique", fixed: courierWidth, defWidth: courierWidth}
	CourierBoldOblique = &Font{BaseFont: "Courier-BoldOblique", fixed: courierWidth, defWidth: courierWidth}

	Symbol       = &Font{BaseFont: "Symbol", symbolic: true, widths: &symbolWidths, defWidth: 500}
	ZapfDingbats = &Font{BaseFont: "ZapfDingbats", symbolic: true, widths: &zapfDingbatsWidths, defWidth: 500}
)

// standardFonts maps the canonical PDF base-font name to its Font.
var standardFonts = map[string]*Font{
	"Helvetica":             Helvetica,
	"Helvetica-Bold":        HelveticaBold,
	"Helvetica-Oblique":     HelveticaOblique,
	"Helvetica-BoldOblique": HelveticaBoldOblique,
	"Times-Roman":           TimesRoman,
	"Times-Bold":            TimesBold,
	"Times-Italic":          TimesItalic,
	"Times-BoldItalic":      TimesBoldItalic,
	"Courier":               Courier,
	"Courier-Bold":          CourierBold,
	"Courier-Oblique":       CourierOblique,
	"Courier-BoldOblique":   CourierBoldOblique,
	"Symbol":                Symbol,
	"ZapfDingbats":          ZapfDingbats,
}

// StandardFont returns the standard font with the given PDF base-font name
// (for example "Helvetica-Bold" or "Times-Roman"). It returns nil if name does
// not identify one of the 14 standard fonts.
func StandardFont(name string) *Font {
	return standardFonts[name]
}

// glyphWidth returns the width of a single rune in 1000-unit glyph space.
func (f *Font) glyphWidth(r rune) int16 {
	if f.fixed != 0 {
		return f.fixed
	}
	if r >= widthFirst && int(r) < widthFirst+len(f.widths) {
		return f.widths[int(r)-widthFirst]
	}
	return f.defWidth
}

// Width returns the rendered width, in points, of text set in this font at the
// given size (in points). It sums the built-in glyph advances; characters
// outside the built-in table fall back to the font's default width.
func (f *Font) Width(text string, size float64) float64 {
	var total int
	for _, r := range text {
		total += int(f.glyphWidth(r))
	}
	return float64(total) * size / 1000.0
}
