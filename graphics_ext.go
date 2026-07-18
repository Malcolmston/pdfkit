package pdfkit

// LineCap selects the shape drawn at the open ends of a stroked path (the PDF
// line cap style, set with the "J" operator).
type LineCap int

// The three PDF line cap styles.
const (
	// CapButt squares the stroke off flush with the endpoint (default).
	CapButt LineCap = 0
	// CapRound draws a semicircular arc centered on the endpoint.
	CapRound LineCap = 1
	// CapSquare extends the stroke by half its width past the endpoint.
	CapSquare LineCap = 2
)

// LineJoin selects the shape drawn where two path segments meet (the PDF line
// join style, set with the "j" operator).
type LineJoin int

// The three PDF line join styles.
const (
	// JoinMiter extends the outer edges to a sharp point (default).
	JoinMiter LineJoin = 0
	// JoinRound fills the corner with a circular arc.
	JoinRound LineJoin = 1
	// JoinBevel fills the corner with a straight edge.
	JoinBevel LineJoin = 2
)

// SetLineCap sets the line cap style used at the ends of subsequently stroked
// open subpaths.
func (p *Page) SetLineCap(c LineCap) {
	p.op(gxitoa(int(c)), "J")
}

// SetLineJoin sets the line join style used at the corners of subsequently
// stroked paths.
func (p *Page) SetLineJoin(j LineJoin) {
	p.op(gxitoa(int(j)), "j")
}

// SetMiterLimit sets the miter limit: the maximum ratio of miter length to line
// width before a JoinMiter corner is converted to a bevel. Values below 1 are
// clamped to 1.
func (p *Page) SetMiterLimit(m float64) {
	if m < 1 {
		m = 1
	}
	p.op(f2s(m), "M")
}

// RoundedRect adds a rectangle subpath with rounded corners of radius r to the
// current path. The rectangle's lower-left corner is at (x, y). The radius is
// clamped to at most half of the shorter side. Paint it afterwards with Stroke,
// Fill or FillStroke.
func (p *Page) RoundedRect(x, y, w, h, r float64) {
	if r < 0 {
		r = 0
	}
	half := w
	if h < half {
		half = h
	}
	if r > half/2 {
		r = half / 2
	}
	if r == 0 {
		p.Rect(x, y, w, h)
		return
	}
	c := r * kappa
	// Start at the bottom edge just right of the lower-left corner.
	p.MoveTo(x+r, y)
	p.LineTo(x+w-r, y) // bottom edge
	p.CurveTo(x+w-r+c, y, x+w, y+r-c, x+w, y+r)
	p.LineTo(x+w, y+h-r) // right edge
	p.CurveTo(x+w, y+h-r+c, x+w-r+c, y+h, x+w-r, y+h)
	p.LineTo(x+r, y+h) // top edge
	p.CurveTo(x+r-c, y+h, x, y+h-r+c, x, y+h-r)
	p.LineTo(x, y+r) // left edge
	p.CurveTo(x, y+r-c, x+r-c, y, x+r, y)
	p.ClosePath()
}

// Polyline adds a connected open path through the given points to the current
// path. pts is a flat list of alternating x, y coordinates, so len(pts) must be
// even and describe at least two points; otherwise Polyline is a no-op. Paint it
// with Stroke.
func (p *Page) Polyline(pts ...float64) {
	if len(pts) < 4 || len(pts)%2 != 0 {
		return
	}
	p.MoveTo(pts[0], pts[1])
	for i := 2; i+1 < len(pts); i += 2 {
		p.LineTo(pts[i], pts[i+1])
	}
}

// Polygon adds a closed path through the given points to the current path. pts
// is a flat list of alternating x, y coordinates, so len(pts) must be even and
// describe at least three points; otherwise Polygon is a no-op. Paint it with
// Fill, Stroke or FillStroke.
func (p *Page) Polygon(pts ...float64) {
	if len(pts) < 6 || len(pts)%2 != 0 {
		return
	}
	p.Polyline(pts...)
	p.ClosePath()
}

// RegularPolygon adds a closed regular polygon with the given number of sides,
// centered at (cx, cy) with circumradius r, to the current path. The first
// vertex is placed at the top (straight up from the center). It is a no-op for
// fewer than 3 sides. Paint it with Fill, Stroke or FillStroke.
func (p *Page) RegularPolygon(cx, cy, r float64, sides int) {
	if sides < 3 {
		return
	}
	pts := make([]float64, 0, sides*2)
	step := 2 * pi / float64(sides)
	for i := 0; i < sides; i++ {
		// Start pointing up (angle = pi/2) and go counterclockwise.
		a := pi/2 + float64(i)*step
		pts = append(pts, cx+r*cosApprox(a), cy+r*sinApprox(a))
	}
	p.Polygon(pts...)
}

// gxitoa is a tiny integer formatter used to avoid importing strconv here.
func gxitoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// pi is a local constant to avoid importing math for the trig approximations.
const pi = 3.14159265358979323846

// sinApprox computes sin(x) via a range-reduced Taylor series accurate to well
// within 1e-9 over the reduced range, keeping the package free of math imports
// while remaining fully deterministic.
func sinApprox(x float64) float64 {
	// Reduce to [-pi, pi].
	twoPi := 2 * pi
	x -= twoPi * floorDiv(x+pi, twoPi)
	x2 := x * x
	term := x
	sum := x
	// Taylor series: x - x^3/3! + x^5/5! - ...
	for n := 1; n <= 8; n++ {
		term *= -x2 / float64((2*n)*(2*n+1))
		sum += term
	}
	return sum
}

// cosApprox computes cos(x) as sin(x + pi/2).
func cosApprox(x float64) float64 {
	return sinApprox(x + pi/2)
}
