package pdfkit

import (
	"fmt"
	"sort"
	"strings"
)

// annotation is an internal representation of a PDF annotation placed on a page.
type annotation struct {
	subtype  string // "Link" or "Text"
	rect     [4]float64
	uri      string  // URI action target (Link)
	destName string  // named destination (Link, GoTo)
	destPage *Page   // explicit destination page (Link/GoTo)
	destY    float64 // explicit destination top y
	contents string  // note text (Text)
}

// AddLinkURI adds a hyperlink over the rectangle at (x, y) with width w and
// height h that opens the given URI when clicked.
func (p *Page) AddLinkURI(x, y, w, h float64, uri string) {
	p.annots = append(p.annots, &annotation{
		subtype: "Link",
		rect:    [4]float64{x, y, x + w, y + h},
		uri:     uri,
	})
}

// AddLinkTo adds an internal hyperlink over the rectangle at (x, y) that jumps
// to target page, scrolled so that atY is near the top of the view.
func (p *Page) AddLinkTo(x, y, w, h float64, target *Page, atY float64) {
	p.annots = append(p.annots, &annotation{
		subtype:  "Link",
		rect:     [4]float64{x, y, x + w, y + h},
		destPage: target,
		destY:    atY,
	})
}

// AddLinkToDest adds an internal hyperlink over the rectangle at (x, y) that
// jumps to the named destination (see Document.AddNamedDest).
func (p *Page) AddLinkToDest(x, y, w, h float64, name string) {
	p.annots = append(p.annots, &annotation{
		subtype:  "Link",
		rect:     [4]float64{x, y, x + w, y + h},
		destName: name,
	})
}

// AddTextAnnotation adds a "sticky note" text annotation with its icon at
// (x, y) showing the given contents when opened.
func (p *Page) AddTextAnnotation(x, y float64, contents string) {
	p.annots = append(p.annots, &annotation{
		subtype:  "Text",
		rect:     [4]float64{x, y, x + 20, y + 20},
		contents: contents,
	})
}

// annotBody renders an annotation dictionary for object number num.
func (d *Document) annotBody(a *annotation, num int, plan writePlan) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<< /Type /Annot /Subtype /%s /Rect [%s %s %s %s]",
		a.subtype, formatFloat(a.rect[0]), formatFloat(a.rect[1]),
		formatFloat(a.rect[2]), formatFloat(a.rect[3]))
	switch a.subtype {
	case "Link":
		b.WriteString(" /Border [0 0 0]")
		switch {
		case a.uri != "":
			fmt.Fprintf(&b, " /A << /S /URI /URI %s >>", d.pdfString(a.uri, num))
		case a.destName != "":
			fmt.Fprintf(&b, " /A << /S /GoTo /D %s >>", d.pdfString(a.destName, num))
		case a.destPage != nil:
			idx := d.pageIndexOf(a.destPage)
			if idx >= 0 {
				fmt.Fprintf(&b, " /Dest [%d 0 R /XYZ null %s null]", plan.pageNum[idx], formatFloat(a.destY))
			}
		}
	case "Text":
		fmt.Fprintf(&b, " /Contents %s /Open false", d.pdfString(a.contents, num))
	}
	b.WriteString(" >>")
	return b.String()
}

// namedDest associates a name with a destination on a page.
type namedDest struct {
	name string
	page *Page
	y    float64
}

// AddNamedDest registers a named destination that points at (implicitly the top
// of) page, scrolled so y is near the top. Named destinations can be targeted by
// Page.AddLinkToDest and are exposed in the document's /Names /Dests tree.
func (d *Document) AddNamedDest(name string, page *Page, y float64) {
	d.namedDests = append(d.namedDests, namedDest{name: name, page: page, y: y})
}

// namesBody renders the /Dests name tree (a single leaf node with a sorted
// /Names array), as referenced from the catalog.
func (d *Document) namesBody(plan writePlan) string {
	dests := make([]namedDest, len(d.namedDests))
	copy(dests, d.namedDests)
	sort.Slice(dests, func(i, j int) bool { return dests[i].name < dests[j].name })
	var b strings.Builder
	b.WriteString("<< /Names [")
	for i, nd := range dests {
		if i > 0 {
			b.WriteByte(' ')
		}
		idx := d.pageIndexOf(nd.page)
		fmt.Fprintf(&b, "(%s) [%d 0 R /XYZ null %s null]", escapeString(nd.name), plan.pageNum[idx], formatFloat(nd.y))
	}
	b.WriteString("] >>")
	return b.String()
}

// Outline is a document outline (bookmark tree). Obtain the document's outline
// with Document.Outline, then add top-level bookmarks with Add; each returned
// Bookmark can nest further bookmarks beneath it.
type Outline struct {
	items []*outlineItem
}

// outlineItem is one bookmark node.
type outlineItem struct {
	title    string
	page     *Page
	y        float64
	parent   int // index into Outline.items, or -1 for a top-level item
	children []int
}

// Bookmark is a handle to a single outline entry, used to attach child entries.
type Bookmark struct {
	o   *Outline
	idx int
}

// Outline returns the document outline, creating it on first use.
func (d *Document) Outline() *Outline {
	if d.outline == nil {
		d.outline = &Outline{}
	}
	return d.outline
}

// Add appends a top-level bookmark titled title that jumps to page at vertical
// position y, and returns a handle for nesting children beneath it.
func (o *Outline) Add(title string, page *Page, y float64) *Bookmark {
	return o.add(title, page, y, -1)
}

// Add appends a child bookmark beneath this one.
func (bm *Bookmark) Add(title string, page *Page, y float64) *Bookmark {
	return bm.o.add(title, page, y, bm.idx)
}

func (o *Outline) add(title string, page *Page, y float64, parent int) *Bookmark {
	idx := len(o.items)
	o.items = append(o.items, &outlineItem{title: title, page: page, y: y, parent: parent})
	if parent >= 0 {
		o.items[parent].children = append(o.items[parent].children, idx)
	}
	return &Bookmark{o: o, idx: idx}
}

// topLevel returns the indices of items with no parent, in insertion order.
func (o *Outline) topLevel() []int {
	var top []int
	for i, it := range o.items {
		if it.parent < 0 {
			top = append(top, i)
		}
	}
	return top
}

// outlineBodies emits the outline root and every bookmark item object.
func (d *Document) outlineBodies(plan writePlan, set func(int, []byte)) {
	o := d.outline
	top := o.topLevel()
	// root
	var root strings.Builder
	root.WriteString("<< /Type /Outlines")
	if len(top) > 0 {
		fmt.Fprintf(&root, " /First %d 0 R /Last %d 0 R", plan.outlineItems[top[0]], plan.outlineItems[top[len(top)-1]])
	}
	fmt.Fprintf(&root, " /Count %d >>", len(o.items))
	set(plan.outlineObj, []byte(root.String()))

	for i, it := range o.items {
		siblings := top
		parentObj := plan.outlineObj
		if it.parent >= 0 {
			siblings = o.items[it.parent].children
			parentObj = plan.outlineItems[it.parent]
		}
		// locate position among siblings
		pos := 0
		for k, s := range siblings {
			if s == i {
				pos = k
				break
			}
		}
		var b strings.Builder
		fmt.Fprintf(&b, "<< /Title %s /Parent %d 0 R", d.pdfString(it.title, plan.outlineItems[i]), parentObj)
		if pos > 0 {
			fmt.Fprintf(&b, " /Prev %d 0 R", plan.outlineItems[siblings[pos-1]])
		}
		if pos < len(siblings)-1 {
			fmt.Fprintf(&b, " /Next %d 0 R", plan.outlineItems[siblings[pos+1]])
		}
		if len(it.children) > 0 {
			fmt.Fprintf(&b, " /First %d 0 R /Last %d 0 R /Count %d",
				plan.outlineItems[it.children[0]], plan.outlineItems[it.children[len(it.children)-1]], len(it.children))
		}
		idx := d.pageIndexOf(it.page)
		if idx >= 0 {
			fmt.Fprintf(&b, " /Dest [%d 0 R /XYZ null %s null]", plan.pageNum[idx], formatFloat(it.y))
		}
		b.WriteString(" >>")
		set(plan.outlineItems[i], []byte(b.String()))
	}
}
