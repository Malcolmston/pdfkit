// Library content for the pdfkit documentation site. Mirrors the shape used by
// the malcolmston/go landing site's data.ts so the sibling sites stay in sync.
export interface Lib {
  id: string; name: string; icon: string; accent: string; pkg: string; node: string;
  repo: string; docs: string; tagline: string; blurb: string; tags: string[];
  features: string[]; node_code: string; go_code: string; integrate: string;
}

export const NODE_ACCENT = '#8cc84b';

export const PDFKIT: Lib = {
  id:"pdfkit", name:"PDFKit", icon:'<i class="fa-solid fa-file-pdf"></i>', accent:"#e5484d",
  pkg:"github.com/malcolmston/pdfkit", node:"foliojs/pdfkit",
  repo:"https://github.com/malcolmston/pdfkit", docs:"https://malcolmston.github.io/pdfkit/",
  tagline:"PDFKit-style PDF generation in pure Go.",
  blurb:"A from-scratch, standard-library-only Go take on Node's PDFKit: build a document, add pages, draw text, "+
    "vector graphics and images, and serialize valid %PDF-1.7 bytes. Everything sits on a real PDF object model "+
    "— indirect objects, a /Catalog, a /Pages tree, per-page content streams, an xref table, a trailer and %%EOF "+
    "— so the output is a conforming PDF that opens anywhere. It ships built-in Adobe Font Metrics for all 14 "+
    "standard Type1 fonts, so text is measured and word-wrapped accurately without embedding font files, plus "+
    "PNG and JPEG image embedding, vector paths with stroke/fill, standard and custom page sizes, and /Info "+
    "metadata. No cgo, no third-party modules — just the Go standard library.",
  tags:["PDF 1.7","object model","14 standard fonts","AFM widths","vector paths","stroke/fill","PNG + JPEG","word wrap","page sizes","metadata","xref/trailer","zero deps"],
  features:[
    "Document / Page model — <code>New</code>, <code>AddPage</code>, then <code>Save</code>, <code>Bytes</code> or <code>Write</code> emit a conforming PDF-1.7 catalog, page tree, xref and trailer",
    "The 14 standard Type1 fonts as package vars (<code>Helvetica</code>, <code>TimesRoman</code>, <code>Courier</code>, <code>Symbol</code>, <code>ZapfDingbats</code> …) or by name via <code>StandardFont</code>, with built-in AFM widths",
    "Text drawing &amp; layout — <code>SetFont</code>, <code>DrawText</code>, <code>DrawLines</code> and greedy word-wrapping <code>DrawParagraph</code>, with <code>Font.Width</code> / <code>TextWidth</code> for accurate measurement",
    "Vector paths — <code>MoveTo</code>, <code>LineTo</code>, <code>CurveTo</code> Bézier, <code>Rect</code>, <code>Circle</code>, <code>Ellipse</code> and <code>DrawLine</code>, painted with <code>Stroke</code>, <code>Fill</code> or <code>FillStroke</code>",
    "Colors &amp; state — RGB <code>Color</code> from <code>RGB</code> / <code>Gray</code>, applied via <code>SetFillColor</code>, <code>SetStrokeColor</code> and <code>SetLineWidth</code>, with <code>Save</code> / <code>Restore</code> graphics state",
    "Image embedding — <code>LoadImage</code> (auto-detect), <code>LoadPNG</code> (FlateDecode + soft mask) and <code>LoadJPEG</code> (DCTDecode) placed with <code>DrawImage</code> as image XObjects",
    "Page geometry — standard sizes <code>A4</code>, <code>Letter</code>, <code>Legal</code>, <code>Tabloid</code> …, plus <code>Custom</code> and <code>PageSize.Landscape</code> / <code>Portrait</code>",
    "Metadata &amp; read-back — <code>SetTitle</code>/<code>SetAuthor</code>/<code>SetSubject</code>/<code>SetKeywords</code> via <code>/Info</code>, plus a minimal xref <code>Reader</code>; zero dependencies, pure Go stdlib"
  ],
  node_code:
`const PDFDocument = require("pdfkit");
const fs = require("fs");

const doc = new PDFDocument({ size: "A4" });
doc.pipe(fs.createWriteStream("hello.pdf"));

doc.font("Helvetica").fontSize(24).text("Hello, PDF!", 72, 72);
doc.rect(72, 120, 200, 80).stroke();
doc.end();`,
  go_code:
`import "github.com/malcolmston/pdfkit"

doc := pdfkit.New()
page := doc.AddPage(pdfkit.A4)

page.SetFont(pdfkit.Helvetica, 24)
page.DrawText(72, 720, "Hello, PDF!")

page.Rect(72, 600, 200, 80)
page.Stroke()

doc.Save("hello.pdf")`,
  integrate:
`<span class="tok-c">// Start a document, set metadata, and add an A4 page.</span>
doc := pdfkit.New()
doc.SetTitle("Report")
doc.SetAuthor("malcolmston")
page := doc.AddPage(pdfkit.A4)

<span class="tok-c">// Draw a heading in blue, then a word-wrapped paragraph below it.</span>
page.SetFont(pdfkit.HelveticaBold, 24)
page.SetFillColor(pdfkit.Blue)
page.DrawText(72, 760, "Quarterly Report")

page.SetFont(pdfkit.TimesRoman, 12)
page.SetFillColor(pdfkit.Black)
nextY := page.DrawParagraph(72, 720, 400, 15,
	"A long run of text that is greedily wrapped to the given width in points.")

<span class="tok-c">// Vector graphics: a stroked rectangle and a filled circle.</span>
page.SetStrokeColor(pdfkit.RGB(200, 0, 0))
page.SetLineWidth(2)
page.Rect(72, nextY-120, 200, 80)
page.Stroke()

page.SetFillColor(pdfkit.RGB(0, 120, 200))
page.Circle(360, nextY-80, 40)
page.Fill()

<span class="tok-c">// Embed a PNG or JPEG (auto-detected) as an image XObject.</span>
if f, err := os.Open("logo.png"); err == nil {
	if img, err := pdfkit.LoadImage(f); err == nil {
		page.DrawImage(img, 72, 72, 120, 120)
	}
}

<span class="tok-c">// Serialize valid %PDF-1.7 bytes to a file (or Bytes / Write).</span>
if err := doc.Save("report.pdf"); err != nil {
	log.Fatal(err)
}`
};
