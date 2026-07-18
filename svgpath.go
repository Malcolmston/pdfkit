package pdfkit

import (
	"errors"
	"math"
	"strconv"
)

// SVGPath parses the SVG path "d" mini-language in data and appends the
// equivalent PDF path-construction operators (moveto, lineto, curveto, close) to
// the page's content stream in the current user space. After calling SVGPath,
// paint the path with Fill, Stroke or FillStroke.
//
// All SVG commands are supported: M/m, L/l, H/h, V/v, C/c, S/s, Q/q, T/t
// (quadratic curves are elevated to cubic), A/a (elliptical arcs are flattened
// to cubic Béziers) and Z/z. Coordinates are used verbatim; because SVG's y-axis
// points down while PDF's points up, apply a flipping transform (for example
// Page.Scale(1, -1) within a Save/Restore) if you need SVG's orientation.
//
// It returns an error if the path data is malformed.
func (p *Page) SVGPath(data string) error {
	sp := &svgParser{s: data}
	var cx, cy float64   // current point
	var sx, sy float64   // subpath start
	var lcx, lcy float64 // last cubic control point (for S/s)
	var lqx, lqy float64 // last quadratic control point (for T/t)
	var prevCmd byte     // previous command letter
	for {
		cmd, ok := sp.command()
		if !ok {
			break
		}
		abs := cmd >= 'A' && cmd <= 'Z'
		switch upper(cmd) {
		case 'M':
			x, y, err := sp.pair()
			if err != nil {
				return err
			}
			if !abs {
				x, y = cx+x, cy+y
			}
			cx, cy, sx, sy = x, y, x, y
			p.MoveTo(x, y)
			// Subsequent coordinate pairs are implicit lineto.
			for sp.hasNumber() {
				x, y, err := sp.pair()
				if err != nil {
					return err
				}
				if !abs {
					x, y = cx+x, cy+y
				}
				cx, cy = x, y
				p.LineTo(x, y)
			}
		case 'L':
			for sp.hasNumber() {
				x, y, err := sp.pair()
				if err != nil {
					return err
				}
				if !abs {
					x, y = cx+x, cy+y
				}
				cx, cy = x, y
				p.LineTo(x, y)
			}
		case 'H':
			for sp.hasNumber() {
				x, err := sp.number()
				if err != nil {
					return err
				}
				if !abs {
					x = cx + x
				}
				cx = x
				p.LineTo(cx, cy)
			}
		case 'V':
			for sp.hasNumber() {
				y, err := sp.number()
				if err != nil {
					return err
				}
				if !abs {
					y = cy + y
				}
				cy = y
				p.LineTo(cx, cy)
			}
		case 'C':
			for sp.hasNumber() {
				x1, y1, err := sp.pair()
				if err != nil {
					return err
				}
				x2, y2, err := sp.pair()
				if err != nil {
					return err
				}
				x, y, err := sp.pair()
				if err != nil {
					return err
				}
				if !abs {
					x1, y1 = cx+x1, cy+y1
					x2, y2 = cx+x2, cy+y2
					x, y = cx+x, cy+y
				}
				p.CurveTo(x1, y1, x2, y2, x, y)
				lcx, lcy = x2, y2
				cx, cy = x, y
			}
		case 'S':
			for sp.hasNumber() {
				x2, y2, err := sp.pair()
				if err != nil {
					return err
				}
				x, y, err := sp.pair()
				if err != nil {
					return err
				}
				if !abs {
					x2, y2 = cx+x2, cy+y2
					x, y = cx+x, cy+y
				}
				x1, y1 := cx, cy
				if upper(prevCmd) == 'C' || upper(prevCmd) == 'S' {
					x1, y1 = 2*cx-lcx, 2*cy-lcy
				}
				p.CurveTo(x1, y1, x2, y2, x, y)
				lcx, lcy = x2, y2
				cx, cy = x, y
			}
		case 'Q':
			for sp.hasNumber() {
				qx, qy, err := sp.pair()
				if err != nil {
					return err
				}
				x, y, err := sp.pair()
				if err != nil {
					return err
				}
				if !abs {
					qx, qy = cx+qx, cy+qy
					x, y = cx+x, cy+y
				}
				p.quadTo(cx, cy, qx, qy, x, y)
				lqx, lqy = qx, qy
				cx, cy = x, y
			}
		case 'T':
			for sp.hasNumber() {
				x, y, err := sp.pair()
				if err != nil {
					return err
				}
				if !abs {
					x, y = cx+x, cy+y
				}
				qx, qy := cx, cy
				if upper(prevCmd) == 'Q' || upper(prevCmd) == 'T' {
					qx, qy = 2*cx-lqx, 2*cy-lqy
				}
				p.quadTo(cx, cy, qx, qy, x, y)
				lqx, lqy = qx, qy
				cx, cy = x, y
			}
		case 'A':
			for sp.hasNumber() {
				rx, err := sp.number()
				if err != nil {
					return err
				}
				ry, err := sp.number()
				if err != nil {
					return err
				}
				rot, err := sp.number()
				if err != nil {
					return err
				}
				large, err := sp.flag()
				if err != nil {
					return err
				}
				sweep, err := sp.flag()
				if err != nil {
					return err
				}
				x, y, err := sp.pair()
				if err != nil {
					return err
				}
				if !abs {
					x, y = cx+x, cy+y
				}
				p.arcTo(cx, cy, rx, ry, rot, large, sweep, x, y)
				cx, cy = x, y
			}
		case 'Z':
			p.ClosePath()
			cx, cy = sx, sy
		default:
			return errors.New("pdfkit: unknown SVG path command")
		}
		prevCmd = cmd
	}
	return nil
}

// quadTo elevates a quadratic Bézier (control q) to a cubic and appends it.
func (p *Page) quadTo(x0, y0, qx, qy, x, y float64) {
	c1x := x0 + 2.0/3.0*(qx-x0)
	c1y := y0 + 2.0/3.0*(qy-y0)
	c2x := x + 2.0/3.0*(qx-x)
	c2y := y + 2.0/3.0*(qy-y)
	p.CurveTo(c1x, c1y, c2x, c2y, x, y)
}

// arcTo flattens an SVG elliptical arc into one or more cubic Béziers.
func (p *Page) arcTo(x0, y0, rx, ry, rotDeg float64, large, sweep bool, x, y float64) {
	if rx == 0 || ry == 0 {
		p.LineTo(x, y)
		return
	}
	rx, ry = math.Abs(rx), math.Abs(ry)
	phi := rotDeg * math.Pi / 180
	cosPhi, sinPhi := math.Cos(phi), math.Sin(phi)

	// Step 1: compute (x1', y1') per SVG implementation notes.
	dx := (x0 - x) / 2
	dy := (y0 - y) / 2
	x1p := cosPhi*dx + sinPhi*dy
	y1p := -sinPhi*dx + cosPhi*dy

	// Correct out-of-range radii.
	lambda := (x1p*x1p)/(rx*rx) + (y1p*y1p)/(ry*ry)
	if lambda > 1 {
		s := math.Sqrt(lambda)
		rx *= s
		ry *= s
	}

	// Step 2: compute center (cx', cy').
	num := rx*rx*ry*ry - rx*rx*y1p*y1p - ry*ry*x1p*x1p
	den := rx*rx*y1p*y1p + ry*ry*x1p*x1p
	co := 0.0
	if den != 0 && num > 0 {
		co = math.Sqrt(num / den)
	}
	if large == sweep {
		co = -co
	}
	cxp := co * rx * y1p / ry
	cyp := -co * ry * x1p / rx

	// Step 3: compute center (cx, cy) in original space.
	cxc := cosPhi*cxp - sinPhi*cyp + (x0+x)/2
	cyc := sinPhi*cxp + cosPhi*cyp + (y0+y)/2

	// Step 4: compute start angle and sweep.
	theta1 := angle(1, 0, (x1p-cxp)/rx, (y1p-cyp)/ry)
	dtheta := angle((x1p-cxp)/rx, (y1p-cyp)/ry, (-x1p-cxp)/rx, (-y1p-cyp)/ry)
	if !sweep && dtheta > 0 {
		dtheta -= 2 * math.Pi
	} else if sweep && dtheta < 0 {
		dtheta += 2 * math.Pi
	}

	// Split into segments of at most 90 degrees.
	segs := int(math.Ceil(math.Abs(dtheta) / (math.Pi / 2)))
	if segs == 0 {
		segs = 1
	}
	delta := dtheta / float64(segs)
	t := 4.0 / 3.0 * math.Tan(delta/4)
	theta := theta1
	for i := 0; i < segs; i++ {
		cosT1, sinT1 := math.Cos(theta), math.Sin(theta)
		theta2 := theta + delta
		cosT2, sinT2 := math.Cos(theta2), math.Sin(theta2)

		// Segment endpoints on the (rotated, translated) ellipse.
		e1x := cxc + rx*cosPhi*cosT1 - ry*sinPhi*sinT1
		e1y := cyc + rx*sinPhi*cosT1 + ry*cosPhi*sinT1
		e2x := cxc + rx*cosPhi*cosT2 - ry*sinPhi*sinT2
		e2y := cyc + rx*sinPhi*cosT2 + ry*cosPhi*sinT2

		// Derivatives of the ellipse parameterization at each endpoint.
		d1x := -rx*cosPhi*sinT1 - ry*sinPhi*cosT1
		d1y := -rx*sinPhi*sinT1 + ry*cosPhi*cosT1
		d2x := -rx*cosPhi*sinT2 - ry*sinPhi*cosT2
		d2y := -rx*sinPhi*sinT2 + ry*cosPhi*cosT2

		p.CurveTo(e1x+t*d1x, e1y+t*d1y, e2x-t*d2x, e2y-t*d2y, e2x, e2y)
		theta = theta2
	}
}

// angle returns the angle between vectors (ux,uy) and (vx,vy).
func angle(ux, uy, vx, vy float64) float64 {
	dot := ux*vx + uy*vy
	l := math.Hypot(ux, uy) * math.Hypot(vx, vy)
	if l == 0 {
		return 0
	}
	c := dot / l
	if c < -1 {
		c = -1
	} else if c > 1 {
		c = 1
	}
	a := math.Acos(c)
	if ux*vy-uy*vx < 0 {
		a = -a
	}
	return a
}

// svgParser tokenizes SVG path data.
type svgParser struct {
	s string
	i int
}

func upper(b byte) byte {
	if b >= 'a' && b <= 'z' {
		return b - 32
	}
	return b
}

func (p *svgParser) skip() {
	for p.i < len(p.s) {
		c := p.s[p.i]
		if c == ' ' || c == ',' || c == '\t' || c == '\n' || c == '\r' {
			p.i++
			continue
		}
		break
	}
}

// command returns the next command letter, or ok=false at end of input.
func (p *svgParser) command() (byte, bool) {
	p.skip()
	if p.i >= len(p.s) {
		return 0, false
	}
	c := p.s[p.i]
	if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
		p.i++
		return c, true
	}
	return 0, false
}

// hasNumber reports whether a number begins at the current position.
func (p *svgParser) hasNumber() bool {
	p.skip()
	if p.i >= len(p.s) {
		return false
	}
	c := p.s[p.i]
	return (c >= '0' && c <= '9') || c == '+' || c == '-' || c == '.'
}

// number scans a single floating-point number.
func (p *svgParser) number() (float64, error) {
	p.skip()
	start := p.i
	if p.i < len(p.s) && (p.s[p.i] == '+' || p.s[p.i] == '-') {
		p.i++
	}
	seenDot := false
	seenDigit := false
	for p.i < len(p.s) {
		c := p.s[p.i]
		switch {
		case c >= '0' && c <= '9':
			seenDigit = true
			p.i++
		case c == '.' && !seenDot:
			seenDot = true
			p.i++
		case (c == 'e' || c == 'E') && seenDigit:
			p.i++
			if p.i < len(p.s) && (p.s[p.i] == '+' || p.s[p.i] == '-') {
				p.i++
			}
		default:
			goto done
		}
	}
done:
	if !seenDigit {
		return 0, errors.New("pdfkit: expected number in SVG path")
	}
	return strconv.ParseFloat(p.s[start:p.i], 64)
}

// flag scans a single-character 0/1 arc flag.
func (p *svgParser) flag() (bool, error) {
	p.skip()
	if p.i >= len(p.s) {
		return false, errors.New("pdfkit: expected arc flag")
	}
	c := p.s[p.i]
	if c == '0' {
		p.i++
		return false, nil
	}
	if c == '1' {
		p.i++
		return true, nil
	}
	return false, errors.New("pdfkit: invalid arc flag")
}

// pair scans two numbers.
func (p *svgParser) pair() (float64, float64, error) {
	x, err := p.number()
	if err != nil {
		return 0, 0, err
	}
	y, err := p.number()
	if err != nil {
		return 0, 0, err
	}
	return x, y, nil
}
