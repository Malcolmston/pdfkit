# Changelog

## 0.3.0

Added a batch of new exported functionality moving pdfkit closer to feature
parity with the PDFKit library it ports. All additions are pure standard library,
deterministic, and covered by known-answer table tests.

### Color

- `CMYKColor` type plus `CMYK` constructor and `CMYKColor.RGB` / `Color.CMYK`
  conversions for the DeviceCMYK print color space.
- `Page.SetFillCMYK` and `Page.SetStrokeCMYK` emit the native `k` / `K`
  operators.
- `HSL` constructs an RGB color from hue/saturation/lightness.
- `Hex` parses `#rgb` / `#rrggbb` CSS hex strings and `Color.Hex` formats a
  color back to `#rrggbb`.

### Vector graphics

- `LineCap` / `LineJoin` types with `Page.SetLineCap`, `Page.SetLineJoin` and
  `Page.SetMiterLimit` for stroke styling.
- `Page.RoundedRect` adds a rounded-corner rectangle subpath.
- `Page.Polyline`, `Page.Polygon` and `Page.RegularPolygon` add multi-point
  paths.

### Font metrics

- `Font.Ascent`, `Font.Descent`, `Font.CapHeight`, `Font.XHeight` and
  `Font.LineHeight` expose canonical AFM vertical metrics for the standard 14
  fonts (and head/hhea/OS2 metrics for embedded fonts).

### Text measurement and layout

- `WrapText` exposes the greedy word-wrapper.
- `Font.HeightOfString` measures the wrapped height of a string.
- `TruncateToWidth` shortens a string with an ellipsis to fit a width.
- `TextAlign` and `TextOptions` types.
- `Page.DrawTextAligned`, `Page.DrawTextCentered`, `Page.DrawTextRight`,
  `Page.DrawTextBox` (with left/center/right/justify alignment and optional
  underline/strikethrough), `Page.DrawUnderline` and `Page.DrawStrikethrough`.

## 0.2.0

- Initial tracked release.
