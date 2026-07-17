package pdfkit

// Color is an RGB color with components in the range [0, 1].
type Color struct {
	R, G, B float64
}

// RGB constructs a Color from 8-bit components (0..255).
func RGB(r, g, b uint8) Color {
	return Color{
		R: float64(r) / 255.0,
		G: float64(g) / 255.0,
		B: float64(b) / 255.0,
	}
}

// Gray constructs a grayscale Color from an 8-bit level (0=black, 255=white).
func Gray(level uint8) Color {
	v := float64(level) / 255.0
	return Color{R: v, G: v, B: v}
}

// Common colors.
var (
	Black = Color{0, 0, 0}
	White = Color{1, 1, 1}
	Red   = Color{1, 0, 0}
	Green = Color{0, 1, 0}
	Blue  = Color{0, 0, 1}
)

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
