package pdfkit

import (
	"strconv"
	"strings"
)

// formatFloat renders a float for inclusion in a PDF content stream or object.
// It uses fixed notation (never scientific), trims trailing zeros, and collapses
// negative zero to "0".
func formatFloat(v float64) string {
	s := strconv.FormatFloat(v, 'f', 4, 64)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
	}
	if s == "-0" || s == "" {
		s = "0"
	}
	return s
}

// escapeString escapes a Go string for use as a PDF literal string, i.e. the
// content that appears between "(" and ")". Backslashes, parentheses and
// control characters are escaped per the PDF specification.
func escapeString(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 8)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '\\':
			b.WriteString(`\\`)
		case '(':
			b.WriteString(`\(`)
		case ')':
			b.WriteString(`\)`)
		case '\r':
			b.WriteString(`\r`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}
