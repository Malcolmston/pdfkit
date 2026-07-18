package pdfkit

import (
	"fmt"
	"strings"
)

// FormField is an interactive AcroForm field (a text field, checkbox or push
// button) placed on a page as a widget annotation. Create fields with
// Page.AddTextField, Page.AddCheckbox or Page.AddPushButton.
type FormField struct {
	kind     string // "text", "checkbox", "button"
	name     string
	page     *Page
	rect     [4]float64
	value    string // text value or button caption
	checked  bool   // checkbox default state
	fontSize float64
}

// AddTextField adds a text input field named name over the rectangle at (x, y)
// with width w and height h, pre-filled with value. It returns the field.
func (p *Page) AddTextField(name string, x, y, w, h float64, value string) *FormField {
	f := &FormField{kind: "text", name: name, page: p, rect: [4]float64{x, y, x + w, y + h}, value: value, fontSize: 0}
	p.doc.acroFields = append(p.doc.acroFields, f)
	return f
}

// AddCheckbox adds a checkbox named name at (x, y) with the given side length,
// initially checked or not. It returns the field.
func (p *Page) AddCheckbox(name string, x, y, size float64, checked bool) *FormField {
	f := &FormField{kind: "checkbox", name: name, page: p, rect: [4]float64{x, y, x + size, y + size}, checked: checked}
	p.doc.acroFields = append(p.doc.acroFields, f)
	return f
}

// AddPushButton adds a push button named name over the rectangle at (x, y) with
// the given caption. It returns the field.
func (p *Page) AddPushButton(name string, x, y, w, h float64, caption string) *FormField {
	f := &FormField{kind: "button", name: name, page: p, rect: [4]float64{x, y, x + w, y + h}, value: caption}
	p.doc.acroFields = append(p.doc.acroFields, f)
	return f
}

// acroFormBody renders the /AcroForm dictionary referenced from the catalog. It
// enables NeedAppearances so viewers synthesize field appearances, and supplies
// default resources (Helvetica and ZapfDingbats) inline.
func (d *Document) acroFormBody(plan writePlan) string {
	var b strings.Builder
	b.WriteString("<< /Fields [")
	for i := range d.acroFields {
		if i > 0 {
			b.WriteByte(' ')
		}
		fmt.Fprintf(&b, "%d 0 R", plan.fieldObjs[i])
	}
	b.WriteString("] /NeedAppearances true /DA (/Helv 0 Tf 0 g)")
	b.WriteString(" /DR << /Font << /Helv << /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>")
	b.WriteString(" /ZaDb << /Type /Font /Subtype /Type1 /BaseFont /ZapfDingbats >> >> >>")
	b.WriteString(" >>")
	return b.String()
}

// fieldBody renders a merged field/widget annotation dictionary for field i.
func (d *Document) fieldBody(f *FormField, i int, plan writePlan) string {
	num := plan.fieldObjs[i]
	var b strings.Builder
	fmt.Fprintf(&b, "<< /Type /Annot /Subtype /Widget /T %s /F 4 /Rect [%s %s %s %s]",
		d.pdfString(f.name, num),
		formatFloat(f.rect[0]), formatFloat(f.rect[1]), formatFloat(f.rect[2]), formatFloat(f.rect[3]))
	switch f.kind {
	case "text":
		b.WriteString(" /FT /Tx /DA (/Helv 0 Tf 0 g)")
		if f.value != "" {
			fmt.Fprintf(&b, " /V %s", d.pdfString(f.value, num))
		}
		b.WriteString(" /MK << /BC [0] /BG [1 1 1] >>")
	case "checkbox":
		b.WriteString(" /FT /Btn /DA (/ZaDb 0 Tf 0 g) /MK << /CA (4) >>")
		state := "Off"
		if f.checked {
			state = "Yes"
		}
		fmt.Fprintf(&b, " /V /%s /AS /%s", state, state)
	case "button":
		b.WriteString(" /FT /Btn /Ff 65536 /DA (/Helv 0 Tf 0 g)")
		fmt.Fprintf(&b, " /MK << /CA %s >>", d.pdfString(f.value, num))
	}
	b.WriteString(" >>")
	return b.String()
}
