package pdfkit

// PageSize describes the dimensions of a page in PDF points (1/72 inch).
type PageSize struct {
	Width  float64
	Height float64
}

// Standard page sizes in points (1 point = 1/72 inch). Sizes are given in
// portrait orientation; use Landscape to rotate.
var (
	A3      = PageSize{841.89, 1190.55}
	A4      = PageSize{595.28, 841.89}
	A5      = PageSize{419.53, 595.28}
	Letter  = PageSize{612, 792}
	Legal   = PageSize{612, 1008}
	Tabloid = PageSize{792, 1224}
)

// Landscape returns the page size rotated to landscape orientation (width and
// height swapped so that width >= height).
func (s PageSize) Landscape() PageSize {
	if s.Width < s.Height {
		return PageSize{s.Height, s.Width}
	}
	return s
}

// Portrait returns the page size in portrait orientation (width <= height).
func (s PageSize) Portrait() PageSize {
	if s.Width > s.Height {
		return PageSize{s.Height, s.Width}
	}
	return s
}

// Custom builds a PageSize from an explicit width and height in points.
func Custom(width, height float64) PageSize {
	return PageSize{Width: width, Height: height}
}
