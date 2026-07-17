// Package pdfkit is a small, dependency-free library for generating PDF
// documents in pure Go. It uses only the Go standard library: no cgo and no
// third-party modules.
//
// # Overview
//
// A [Document] is a container of [Page] values plus document metadata. You
// build a document by adding pages, drawing text, vector graphics and images
// onto each page, and then serializing the result:
//
//	doc := pdfkit.New()
//	doc.SetTitle("Hello")
//	page := doc.AddPage(pdfkit.A4)
//	page.SetFont(pdfkit.Helvetica, 24)
//	page.DrawText(72, 720, "Hello, PDF!")
//	data, _ := doc.Bytes() // or doc.Write(w) / doc.Save("out.pdf")
//
// # The PDF object model
//
// The generated file is a conforming PDF 1.7 document. Internally pdfkit emits
// the standard indirect-object structure:
//
//   - a %PDF-1.7 header and a binary marker comment;
//   - a document catalog (/Type /Catalog) referencing the page tree;
//   - a page tree (/Type /Pages) with one page node per [Page];
//   - one content stream per page holding the drawing operators;
//   - font, image (XObject) and /Info dictionaries as needed;
//   - a cross-reference (xref) table, a trailer and %%EOF.
//
// Output is deterministic: given the same drawing calls, the byte stream is
// identical, which makes golden-file testing straightforward. The [Reader] type
// provides a minimal xref parser that can read the structure back.
//
// # Coordinate system
//
// Pages use native PDF user space. The origin (0, 0) is the bottom-left corner,
// x increases rightward and y increases upward, with distances in points
// (1/72 inch). Standard page sizes ([A4], [Letter], [Legal] and others) are
// provided, along with [Custom] sizes and [PageSize.Landscape] orientation.
//
// # Fonts and text
//
// All 14 standard PDF Type1 fonts are available as package variables
// ([Helvetica], [TimesRoman], [Courier], [Symbol], [ZapfDingbats] and their
// bold/italic variants) and by name via [StandardFont]. Each font ships with a
// built-in Adobe Font Metrics width table, so [Font.Width] and [Page.TextWidth]
// return accurate text widths for layout. [Page.DrawText] draws a single line,
// [Page.DrawLines] draws several, and [Page.DrawParagraph] performs greedy word
// wrapping to a maximum width. Text color follows the current fill color set
// with [Page.SetFillColor].
//
// # Vector graphics
//
// Paths are built with [Page.MoveTo], [Page.LineTo], [Page.CurveTo] (cubic
// Bézier), [Page.Rect], [Page.Circle] and [Page.Ellipse], then painted with
// [Page.Stroke], [Page.Fill] or [Page.FillStroke]. Line width is set with
// [Page.SetLineWidth] and colors with [Page.SetStrokeColor] and
// [Page.SetFillColor]. Colors are RGB values constructed with [RGB] or [Gray].
//
// # Images
//
// PNG and JPEG images are embedded as image XObjects. JPEG data is stored
// directly using the DCTDecode filter; PNG pixels are re-encoded as
// zlib-compressed RGB samples (FlateDecode) with an automatically generated soft
// mask when the source has transparency. Load images with [LoadImage],
// [LoadPNG] or [LoadJPEG] and place them with [Page.DrawImage].
package pdfkit
