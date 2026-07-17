package pdfkit

import (
	"bytes"
	"os"
)

// Document is a PDF document under construction. Create one with New, add pages
// with AddPage, draw on each returned Page, then serialize with Write, Bytes or
// Save.
//
// The zero value is not usable; always start from New.
type Document struct {
	pages []*Page

	// info holds document metadata keyed by PDF /Info entry name
	// (Title, Author, Subject, Keywords, Creator, Producer).
	info map[string]string

	// fonts is the ordered, de-duplicated set of fonts used anywhere in the
	// document. A font's index determines its resource name (/F1, /F2, ...).
	fonts     []*Font
	fontIndex map[string]int

	// images is the ordered set of images used in the document; a resource
	// name /Im1, /Im2, ... is derived from the index.
	images []*Image
}

// New creates an empty document with default metadata (a Producer/Creator of
// "pdfkit").
func New() *Document {
	return &Document{
		info: map[string]string{
			"Producer": "pdfkit",
			"Creator":  "pdfkit",
		},
		fontIndex: map[string]int{},
	}
}

// SetTitle sets the document title (/Title in the Info dictionary).
func (d *Document) SetTitle(s string) { d.info["Title"] = s }

// SetAuthor sets the document author (/Author).
func (d *Document) SetAuthor(s string) { d.info["Author"] = s }

// SetSubject sets the document subject (/Subject).
func (d *Document) SetSubject(s string) { d.info["Subject"] = s }

// SetKeywords sets the document keywords (/Keywords).
func (d *Document) SetKeywords(s string) { d.info["Keywords"] = s }

// SetCreator sets the creating application name (/Creator).
func (d *Document) SetCreator(s string) { d.info["Creator"] = s }

// SetProducer sets the producing application name (/Producer).
func (d *Document) SetProducer(s string) { d.info["Producer"] = s }

// AddPage appends a new page of the given size (portrait orientation as given)
// and returns it for drawing.
func (d *Document) AddPage(size PageSize) *Page {
	p := newPage(d, size)
	d.pages = append(d.pages, p)
	return p
}

// PageCount returns the number of pages added so far.
func (d *Document) PageCount() int { return len(d.pages) }

// registerFont records a font as used by the document and returns its resource
// name (e.g. "F1"). Repeated calls for the same base font share a name.
func (d *Document) registerFont(f *Font) string {
	if idx, ok := d.fontIndex[f.BaseFont]; ok {
		return fontResourceName(idx)
	}
	idx := len(d.fonts)
	d.fonts = append(d.fonts, f)
	d.fontIndex[f.BaseFont] = idx
	return fontResourceName(idx)
}

// registerImage records an image and returns its resource name (e.g. "Im1").
func (d *Document) registerImage(img *Image) string {
	for i, existing := range d.images {
		if existing == img {
			return imageResourceName(i)
		}
	}
	idx := len(d.images)
	d.images = append(d.images, img)
	return imageResourceName(idx)
}

// Bytes serializes the document to a byte slice of valid PDF-1.7 content.
func (d *Document) Bytes() ([]byte, error) {
	var buf bytes.Buffer
	if err := d.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Save writes the document to the named file, creating or truncating it.
func (d *Document) Save(filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	if err := d.Write(f); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}
