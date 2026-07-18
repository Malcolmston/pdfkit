package pdfkit

import (
	"strconv"
	"strings"
)

// Unit-conversion constants matching PDFKit's lib/mixins/fonts.js.
const (
	inToPt = 72.0       // 1 inch = 72 PDF points
	pxToIn = 1.0 / 96.0 // 1 CSS pixel = 1/96 inch
	cmToIn = 1.0 / 2.54 // 1 centimetre = 1/2.54 inch
	mmToCm = 0.1        // 1 millimetre = 0.1 centimetre
	pcToPt = 12.0       // 1 pica = 12 points
	defRem = 12.0       // PDFKit's default _remSize (root em) is 12pt
)

// SizeContext supplies the document/page state that PDFKit's sizeToPoint uses to
// resolve relative CSS units. It mirrors the fields the upstream method reads
// off the document and current page.
type SizeContext struct {
	// Font is the current font, used to resolve the "ex" unit (its line
	// height) and the "ch" unit (the advance width of '0'). If nil, those
	// units evaluate to zero, as they would with no font selected.
	Font *Font
	// FontSize is the current font size in points, used for the "em" unit and
	// as the default percentage basis for "%".
	FontSize float64
	// Page is the current page size, used for the viewport units "vw", "vh",
	// "vmin" and "vmax".
	Page PageSize
	// RemSize is the root em size for the "rem" unit. When zero, PDFKit's
	// default of 12 points is used.
	RemSize float64
}

// SizeToPoint converts a CSS-style size specifier to PDF points, reproducing
// PDFKit's sizeToPoint (lib/mixins/fonts.js). The size may be a bare number
// string ("1", "12.5"), a boolean literal ("true"→1, "false"→0), or a number
// with a unit suffix drawn from:
//
//	em in px cm mm pc ex ch rem vw vh vmin vmax % pt
//
// The units resolve as PDFKit specifies: em and % are relative to ctx.FontSize,
// rem to ctx.RemSize (defaulting to 12), ex to the current font's line height,
// ch to the advance of '0', and vw/vh/vmin/vmax to the page dimensions. An
// unrecognised or missing unit is treated as points. It returns defaultValue for
// an empty input and an ok flag reporting whether the input was understood.
func SizeToPoint(size string, defaultValue float64, ctx SizeContext) (float64, bool) {
	if size == "" {
		return defaultValue, true
	}
	switch size {
	case "true":
		return 1, true
	case "false":
		return 0, true
	}

	num, unit, ok := splitSize(size)
	if !ok {
		return defaultValue, false
	}

	remSize := ctx.RemSize
	if remSize == 0 {
		remSize = defRem
	}

	var multiplier float64
	switch unit {
	case "em":
		multiplier = ctx.FontSize
	case "in":
		multiplier = inToPt
	case "px":
		multiplier = pxToIn * inToPt
	case "cm":
		multiplier = cmToIn * inToPt
	case "mm":
		multiplier = mmToCm * cmToIn * inToPt
	case "pc":
		multiplier = pcToPt
	case "ex":
		if ctx.Font != nil {
			multiplier = ctx.Font.LineHeight(ctx.FontSize)
		}
	case "ch":
		if ctx.Font != nil {
			multiplier = ctx.Font.Width("0", ctx.FontSize)
		}
	case "rem":
		multiplier = remSize
	case "vw":
		multiplier = ctx.Page.Width / 100
	case "vh":
		multiplier = ctx.Page.Height / 100
	case "vmin":
		multiplier = minf(ctx.Page.Width, ctx.Page.Height) / 100
	case "vmax":
		multiplier = maxf(ctx.Page.Width, ctx.Page.Height) / 100
	case "%":
		multiplier = ctx.FontSize / 100
	default: // "pt" or none
		multiplier = 1
	}

	return multiplier * num, true
}

// splitSize parses the leading number and trailing unit of a size specifier,
// matching the numeric grammar of PDFKit's regexp: an optional integer part and
// an optional fractional part, followed by an optional known unit. It reports
// ok=false if there is no numeric part at all.
func splitSize(s string) (num float64, unit string, ok bool) {
	i := 0
	for i < len(s) && (s[i] >= '0' && s[i] <= '9') {
		i++
	}
	if i < len(s) && s[i] == '.' {
		i++
		for i < len(s) && (s[i] >= '0' && s[i] <= '9') {
			i++
		}
	}
	numStr := s[:i]
	if numStr == "" || numStr == "." {
		return 0, "", false
	}
	n, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, "", false
	}
	return n, strings.TrimSpace(s[i:]), true
}

// minf returns the smaller of two float64 values.
func minf(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
