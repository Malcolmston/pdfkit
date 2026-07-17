package pdfkit_test

import (
	"fmt"

	"github.com/malcolmston/pdfkit"
)

// Example builds a one-page document and reports structural facts about the
// generated PDF bytes.
func Example() {
	doc := pdfkit.New()
	doc.SetTitle("Quick Start")

	page := doc.AddPage(pdfkit.A4)
	page.SetFont(pdfkit.Helvetica, 24)
	page.DrawText(72, 720, "Hello, PDF!")

	data, err := doc.Bytes()
	if err != nil {
		panic(err)
	}

	r, err := pdfkit.Parse(data)
	if err != nil {
		panic(err)
	}

	fmt.Printf("header: %s\n", string(data[:8]))
	fmt.Printf("pages: %d\n", doc.PageCount())
	fmt.Printf("objects: %d\n", r.ObjectCount())
	fmt.Printf("ends with EOF: %t\n", string(data[len(data)-6:]) == "%%EOF\n")

	// Output:
	// header: %PDF-1.7
	// pages: 1
	// objects: 6
	// ends with EOF: true
}

// ExampleFont_Width demonstrates measuring text width using the built-in AFM
// metrics.
func ExampleFont_Width() {
	w := pdfkit.Helvetica.Width("AV", 10)
	fmt.Printf("%.2f", w)
	// Output: 13.34
}
