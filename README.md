# pdfkit

High-level PDF generation for Go — pure standard library, no cgo, no third-party
dependencies.

pdfkit produces valid `%PDF-1.7` documents with text, vector graphics and
embedded images. It ships built-in Adobe Font Metrics for all 14 standard Type1
fonts so text can be measured and laid out accurately without embedding font
files.

## Features

- Document / Page model over the real PDF object model: indirect objects,
  catalog, page tree, per-page content streams, xref table, trailer and `%%EOF`.
- The 14 standard fonts (Helvetica, Times, Courier families plus Symbol and
  ZapfDingbats) with built-in width tables; text drawing, line/paragraph layout
  with word wrap, and text color.
- Vector graphics: `MoveTo`/`LineTo`/`CurveTo` Bézier paths, rectangles, circles
  and ellipses; stroke, fill and fill-stroke; line width and RGB colors.
- Image embedding: PNG (FlateDecode, with a soft mask for transparency) and JPEG
  (DCTDecode) as image XObjects.
- Standard page sizes (A3/A4/A5, Letter, Legal, Tabloid), custom sizes and
  landscape orientation.
- Document metadata (Title/Author/Subject/Keywords/Creator/Producer) via `/Info`.
- Output to an `io.Writer`, a file, or a byte slice.
- A minimal xref `Reader` for reading the structure back (used in tests).

## Install

```sh
go get github.com/malcolmston/pdfkit
```

Requires Go 1.24 or newer. No external modules are used.

## Quick start

```go
package main

import (
	"log"

	"github.com/malcolmston/pdfkit"
)

func main() {
	doc := pdfkit.New()
	doc.SetTitle("Hello")
	doc.SetAuthor("You")

	page := doc.AddPage(pdfkit.A4)

	// Text
	page.SetFont(pdfkit.Helvetica, 24)
	page.SetFillColor(pdfkit.Black)
	page.DrawText(72, 720, "Hello, PDF!")

	// Vector graphics
	page.SetStrokeColor(pdfkit.RGB(200, 0, 0))
	page.SetLineWidth(2)
	page.Rect(72, 600, 200, 80)
	page.Stroke()

	page.SetFillColor(pdfkit.Blue)
	page.Circle(300, 400, 50)
	page.Fill()

	if err := doc.Save("hello.pdf"); err != nil {
		log.Fatal(err)
	}
}
```

### Coordinate system

Pages use native PDF user space: the origin `(0, 0)` is the bottom-left corner,
`x` increases to the right and `y` increases upward, in points (1/72 inch).

### Measuring text

```go
w := pdfkit.Helvetica.Width("Hello, PDF!", 24) // width in points
```

### Paragraphs

```go
page.SetFont(pdfkit.TimesRoman, 12)
nextY := page.DrawParagraph(72, 700, 400, 15,
	"A long run of text that will be greedily wrapped to the given width...")
```

### Images

```go
f, _ := os.Open("photo.jpg")
img, _ := pdfkit.LoadImage(f) // auto-detects PNG or JPEG
page.DrawImage(img, 72, 72, 200, 150)
```

## License

See repository.
