package pdfkit

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// buildBasicDoc creates a small multi-feature document used across tests.
func buildBasicDoc(t *testing.T) *Document {
	t.Helper()
	doc := New()
	doc.SetTitle("Test Doc")
	doc.SetAuthor("Tester")
	doc.SetSubject("Testing")
	doc.SetKeywords("pdf,go")
	doc.SetCreator("pdfkit-test")

	p := doc.AddPage(A4)
	p.SetFont(Helvetica, 18)
	p.SetFillColor(Black)
	p.DrawText(72, 720, "Hello, PDF!")

	p.SetStrokeColor(Red)
	p.SetLineWidth(2)
	p.Rect(72, 600, 200, 80)
	p.Stroke()

	p.SetFillColor(Blue)
	p.Circle(300, 400, 50)
	p.Fill()

	return doc
}

func TestBytesStructure(t *testing.T) {
	doc := buildBasicDoc(t)
	data, err := doc.Bytes()
	if err != nil {
		t.Fatalf("Bytes: %v", err)
	}
	if !bytes.HasPrefix(data, []byte("%PDF-1.7")) {
		t.Errorf("output does not start with %%PDF-1.7")
	}
	for _, tok := range []string{"xref", "trailer", "startxref", "%%EOF", "/Type /Catalog", "/Type /Pages", "/Type /Page"} {
		if !bytes.Contains(data, []byte(tok)) {
			t.Errorf("output missing %q", tok)
		}
	}
	if !bytes.HasSuffix(data, []byte("%%EOF\n")) {
		t.Errorf("output does not end with %%EOF")
	}
}

func TestExpectedObjectCount(t *testing.T) {
	// 1 catalog + 1 pages + 1 info + 1 font + 1 page + 1 content = 6 objects.
	doc := buildBasicDoc(t)
	data, err := doc.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	r, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got, want := r.ObjectCount(), 6; got != want {
		t.Errorf("object count = %d, want %d", got, want)
	}
	if r.Size != 7 {
		t.Errorf("trailer /Size = %d, want 7", r.Size)
	}
	if r.Root != 1 {
		t.Errorf("Root = %d, want 1", r.Root)
	}
	if r.Info != 3 {
		t.Errorf("Info = %d, want 3", r.Info)
	}
}

func TestReaderResolvesObjects(t *testing.T) {
	doc := buildBasicDoc(t)
	data, err := doc.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	r, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	cat := r.Object(r.Root)
	if !bytes.Contains(cat, []byte("/Type /Catalog")) {
		t.Errorf("Root object is not a catalog: %q", cat)
	}
	info := r.Object(r.Info)
	if !bytes.Contains(info, []byte("/Title (Test Doc)")) {
		t.Errorf("Info object missing title: %q", info)
	}
	// Every offset must point at "N 0 obj".
	for n := 1; n <= r.ObjectCount(); n++ {
		obj := r.Object(n)
		if obj == nil {
			t.Fatalf("object %d not resolvable", n)
		}
		if !bytes.HasPrefix(obj, []byte(itoa(n)+" 0 obj")) {
			t.Errorf("object %d header wrong: %q", n, first(obj, 16))
		}
	}
}

func TestMultiPageAndMetadata(t *testing.T) {
	doc := New()
	doc.AddPage(A4)
	doc.AddPage(Letter.Landscape())
	doc.AddPage(Custom(200, 300))
	if doc.PageCount() != 3 {
		t.Fatalf("PageCount = %d, want 3", doc.PageCount())
	}
	data, err := doc.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if got := bytes.Count(data, []byte("/Type /Page ")); got != 3 {
		t.Errorf("expected 3 page objects, found %d", got)
	}
	// Landscape Letter -> width 792 height 612
	if !bytes.Contains(data, []byte("/MediaBox [0 0 792 612]")) {
		t.Errorf("landscape Letter MediaBox not found")
	}
	if !bytes.Contains(data, []byte("/MediaBox [0 0 200 300]")) {
		t.Errorf("custom MediaBox not found")
	}
}

func TestTextWidthTables(t *testing.T) {
	cases := []struct {
		font *Font
		text string
		size float64
		want float64
	}{
		// Helvetica: 'A'=667, 'V'=667 => 1334 * 10/1000 = 13.34
		{Helvetica, "AV", 10, 13.34},
		// Courier is monospaced 600: "abc" => 1800 * 12/1000 = 21.6
		{Courier, "abc", 12, 21.6},
		// Times-Roman space=250, 'i'=278 -> "i " = 528 *10/1000 = 5.28
		{TimesRoman, "i ", 10, 5.28},
	}
	for _, c := range cases {
		got := c.font.Width(c.text, c.size)
		if diff := got - c.want; diff > 1e-9 || diff < -1e-9 {
			t.Errorf("%s.Width(%q,%v) = %v, want %v", c.font.BaseFont, c.text, c.size, got, c.want)
		}
	}
}

func TestFontWidthConsistency(t *testing.T) {
	// Oblique variants must share widths with their upright counterparts.
	if Helvetica.Width("Hello World", 12) != HelveticaOblique.Width("Hello World", 12) {
		t.Errorf("Helvetica and Helvetica-Oblique widths differ")
	}
	if HelveticaBold.Width("Hello", 12) != HelveticaBoldOblique.Width("Hello", 12) {
		t.Errorf("Helvetica-Bold and BoldOblique widths differ")
	}
	// out-of-range rune falls back to default width, does not panic
	if Helvetica.Width("☃", 10) <= 0 {
		t.Errorf("default width fallback failed")
	}
}

func TestStandardFontLookup(t *testing.T) {
	if StandardFont("Times-Bold") != TimesBold {
		t.Errorf("StandardFont(Times-Bold) mismatch")
	}
	if StandardFont("does-not-exist") != nil {
		t.Errorf("unknown font should be nil")
	}
	names := []string{
		"Helvetica", "Helvetica-Bold", "Helvetica-Oblique", "Helvetica-BoldOblique",
		"Times-Roman", "Times-Bold", "Times-Italic", "Times-BoldItalic",
		"Courier", "Courier-Bold", "Courier-Oblique", "Courier-BoldOblique",
		"Symbol", "ZapfDingbats",
	}
	if len(names) != 14 {
		t.Fatal("expected 14 standard fonts")
	}
	for _, n := range names {
		if StandardFont(n) == nil {
			t.Errorf("standard font %q missing", n)
		}
	}
}

func TestContentStreamGeneration(t *testing.T) {
	doc := New()
	p := doc.AddPage(A4)
	p.SetFont(Helvetica, 12)
	p.DrawText(10, 20, "Hi (world)")
	p.MoveTo(0, 0)
	p.LineTo(100, 100)
	p.CurveTo(1, 2, 3, 4, 5, 6)
	p.Stroke()
	content := string(p.content())

	for _, want := range []string{
		"BT\n", "/F1 12 Tf\n", "10 20 Td\n", `(Hi \(world\)) Tj`, "ET\n",
		"0 0 m\n", "100 100 l\n", "1 2 3 4 5 6 c\n", "S\n",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("content stream missing %q\n---\n%s", want, content)
		}
	}
}

func TestTextWidthOnPage(t *testing.T) {
	doc := New()
	p := doc.AddPage(A4)
	if p.TextWidth("anything") != 0 {
		t.Errorf("TextWidth without font should be 0")
	}
	p.SetFont(Courier, 10)
	// 5 chars * 600 * 10/1000 = 30
	if got := p.TextWidth("hello"); got != 30 {
		t.Errorf("TextWidth = %v, want 30", got)
	}
}

func TestDrawParagraphWraps(t *testing.T) {
	doc := New()
	p := doc.AddPage(A4)
	p.SetFont(Helvetica, 12)
	text := "The quick brown fox jumps over the lazy dog again and again"
	endY := p.DrawParagraph(72, 700, 120, 14, text)
	if endY >= 700 {
		t.Errorf("paragraph did not advance y: %v", endY)
	}
	lines := wrapText(Helvetica, 12, 120, text)
	if len(lines) < 2 {
		t.Errorf("expected wrapping into multiple lines, got %d", len(lines))
	}
	for _, ln := range lines {
		// multi-word lines must fit within the max width
		if strings.Contains(ln, " ") && Helvetica.Width(ln, 12) > 120.0001 {
			t.Errorf("wrapped line too wide: %q width=%v", ln, Helvetica.Width(ln, 12))
		}
	}
}

func TestMiscHelpers(t *testing.T) {
	if A4.Portrait() != A4 {
		t.Errorf("A4.Portrait should be unchanged")
	}
	if A4.Landscape().Portrait() != A4 {
		t.Errorf("round-trip orientation failed")
	}
	doc := New()
	doc.SetProducer("myapp")
	p := doc.AddPage(Letter)
	if p.Size() != Letter {
		t.Errorf("Page.Size mismatch")
	}
	data, _ := doc.Bytes()
	if !bytes.Contains(data, []byte("/Producer (myapp)")) {
		t.Errorf("producer not written")
	}
	if escapeString("a\tb\rc\nd") != `a\tb\rc\nd` {
		t.Errorf("escapeString control chars wrong: %q", escapeString("a\tb\rc\nd"))
	}
}

func TestGrayscaleJPEG(t *testing.T) {
	gray := image.NewGray(image.Rect(0, 0, 8, 8))
	for i := range gray.Pix {
		gray.Pix[i] = uint8(i)
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, gray, nil); err != nil {
		t.Fatal(err)
	}
	ji, err := LoadJPEG(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if ji.colorSpace != "DeviceGray" {
		t.Errorf("grayscale jpeg colorspace = %q, want DeviceGray", ji.colorSpace)
	}
}

func TestDrawParagraphNewlines(t *testing.T) {
	lines := wrapText(Helvetica, 12, 1000, "line one\nline two\n\nlast")
	want := []string{"line one", "line two", "", "last"}
	if len(lines) != len(want) {
		t.Fatalf("got %d lines, want %d: %v", len(lines), len(want), lines)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, lines[i], want[i])
		}
	}
}

func TestSymbolicFontNoEncoding(t *testing.T) {
	doc := New()
	p := doc.AddPage(A4)
	p.SetFont(Symbol, 12)
	p.DrawText(10, 10, "abc")
	data, _ := doc.Bytes()
	if !bytes.Contains(data, []byte("/BaseFont /Symbol >>")) {
		t.Errorf("Symbol font should have no /Encoding")
	}
}

func TestPNGImageEmbedding(t *testing.T) {
	// build an RGBA image with transparency
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			a := uint8(255)
			if x == 0 {
				a = 128
			}
			img.Set(x, y, color.RGBA{R: uint8(x * 60), G: uint8(y * 60), B: 100, A: a})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	pi, err := LoadImage(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("LoadImage png: %v", err)
	}
	if pi.Width != 4 || pi.Height != 4 {
		t.Errorf("png dims = %dx%d", pi.Width, pi.Height)
	}
	if pi.smask == nil {
		t.Errorf("expected soft mask for image with alpha")
	}

	doc := New()
	p := doc.AddPage(A4)
	p.DrawImage(pi, 72, 72, 100, 100)
	data, err := doc.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	for _, tok := range []string{"/Subtype /Image", "/Filter /FlateDecode", "/SMask", "/Im1 Do", "cm"} {
		if !bytes.Contains(data, []byte(tok)) {
			t.Errorf("png doc missing %q", tok)
		}
	}
	if _, err := Parse(data); err != nil {
		t.Errorf("png doc did not parse: %v", err)
	}
}

func TestJPEGImageEmbedding(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 30), G: uint8(y * 30), B: 60, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatal(err)
	}
	ji, err := LoadImage(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("LoadImage jpeg: %v", err)
	}
	if ji.filter != filterDCT {
		t.Errorf("jpeg should use DCTDecode")
	}
	doc := New()
	p := doc.AddPage(A4)
	p.DrawImage(ji, 0, 0, 50, 50)
	data, _ := doc.Bytes()
	if !bytes.Contains(data, []byte("/Filter /DCTDecode")) {
		t.Errorf("missing DCTDecode filter")
	}
}

func TestImageReuseSharesXObject(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	pi, _ := LoadPNG(bytes.NewReader(buf.Bytes()))

	doc := New()
	p := doc.AddPage(A4)
	p.DrawImage(pi, 0, 0, 10, 10)
	p.DrawImage(pi, 20, 20, 10, 10)
	if len(doc.images) != 1 {
		t.Errorf("expected image reuse, got %d images", len(doc.images))
	}
}

func TestUnknownImageFormat(t *testing.T) {
	_, err := LoadImage(bytes.NewReader([]byte("not an image")))
	if err == nil {
		t.Errorf("expected error for unknown format")
	}
}

func TestSaveToFile(t *testing.T) {
	doc := buildBasicDoc(t)
	dir := t.TempDir()
	fp := filepath.Join(dir, "out.pdf")
	if err := doc.Save(fp); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(fp)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(data, []byte("%PDF-1.7")) {
		t.Errorf("saved file not a PDF")
	}
	if _, err := Parse(data); err != nil {
		t.Errorf("saved file did not parse: %v", err)
	}
}

func TestColorHelpers(t *testing.T) {
	c := RGB(255, 0, 0)
	if c.R != 1 || c.G != 0 || c.B != 0 {
		t.Errorf("RGB(255,0,0) = %+v", c)
	}
	g := Gray(128)
	if g.R != g.G || g.G != g.B {
		t.Errorf("Gray not equal components: %+v", g)
	}
	if clamp01(-1) != 0 || clamp01(2) != 1 || clamp01(0.5) != 0.5 {
		t.Errorf("clamp01 wrong")
	}
}

func TestFormatFloat(t *testing.T) {
	cases := map[float64]string{
		1.0:                  "1",
		1.5:                  "1.5",
		1.2500:               "1.25",
		math.Copysign(0, -1): "0",
		100.125:              "100.125",
		0.0001:               "0.0001",
	}
	for in, want := range cases {
		if got := formatFloat(in); got != want {
			t.Errorf("formatFloat(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestDrawGraphicsHelpers(t *testing.T) {
	doc := New()
	p := doc.AddPage(A4)
	p.Save()
	p.SetLineWidth(1.5)
	p.SetStrokeColor(Green)
	p.DrawLine(0, 0, 10, 10)
	p.Ellipse(50, 50, 20, 10)
	p.FillStroke()
	p.ClosePath()
	p.Restore()
	c := string(p.content())
	for _, tok := range []string{"q\n", "1.5 w\n", "0 1 0 RG\n", "B\n", "h\n", "Q\n"} {
		if !strings.Contains(c, tok) {
			t.Errorf("graphics content missing %q", tok)
		}
	}
}

func TestParseErrors(t *testing.T) {
	if _, err := Parse([]byte("not a pdf")); err == nil {
		t.Errorf("expected header error")
	}
	if _, err := Parse([]byte("%PDF-1.7 no startxref here")); err == nil {
		t.Errorf("expected startxref error")
	}
}

// small local integer-to-string helper to avoid importing strconv in tests.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

func first(b []byte, n int) []byte {
	if len(b) < n {
		return b
	}
	return b[:n]
}
