package pdfkit

import (
	"bytes"
	"compress/zlib"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
)

// imageFilter identifies how the embedded image data is encoded in the PDF.
type imageFilter int

const (
	filterFlate imageFilter = iota // raw samples, zlib-compressed (PNG path)
	filterDCT                      // native JPEG bytes (DCTDecode)
)

// Image is a decoded raster image ready to be embedded in a PDF document as an
// XObject. Load one with LoadImage, LoadPNG or LoadJPEG, then draw it onto a
// page with Page.DrawImage.
type Image struct {
	Width  int
	Height int

	filter     imageFilter
	colorSpace string // "DeviceRGB" or "DeviceGray"
	bpc        int    // bits per component
	data       []byte // image sample data (already filtered/compressed)
	smask      []byte // optional soft-mask (alpha) data, zlib-compressed; nil if none
}

// LoadImage reads an image from r, auto-detecting PNG or JPEG encoding.
func LoadImage(r io.Reader) (*Image, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	switch {
	case len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}):
		return LoadPNG(bytes.NewReader(data))
	case len(data) >= 2 && data[0] == 0xff && data[1] == 0xd8:
		return LoadJPEG(bytes.NewReader(data))
	default:
		return nil, errors.New("pdfkit: unrecognized image format (want PNG or JPEG)")
	}
}

// LoadJPEG loads a JPEG image. The original JPEG bytes are embedded directly
// using the PDF DCTDecode filter, so no re-encoding occurs.
func LoadJPEG(r io.Reader) (*Image, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("pdfkit: decode jpeg: %w", err)
	}
	cs := "DeviceRGB"
	switch cfg.ColorModel {
	case jpegGray:
		cs = "DeviceGray"
	}
	return &Image{
		Width:      cfg.Width,
		Height:     cfg.Height,
		filter:     filterDCT,
		colorSpace: cs,
		bpc:        8,
		data:       data,
	}, nil
}

// LoadPNG loads a PNG image. The pixels are decoded and re-encoded as raw,
// zlib-compressed RGB samples (PDF FlateDecode). If the source has an alpha
// channel, a soft mask (SMask) is generated so transparency is preserved.
func LoadPNG(r io.Reader) (*Image, error) {
	img, err := png.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("pdfkit: decode png: %w", err)
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()

	rgb := make([]byte, 0, w*h*3)
	alpha := make([]byte, 0, w*h)
	hasAlpha := false
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			cr, cg, cb, ca := img.At(x, y).RGBA()
			rgb = append(rgb, byte(cr>>8), byte(cg>>8), byte(cb>>8))
			a := byte(ca >> 8)
			alpha = append(alpha, a)
			if a != 0xff {
				hasAlpha = true
			}
		}
	}

	comp, err := zlibCompress(rgb)
	if err != nil {
		return nil, err
	}
	im := &Image{
		Width:      w,
		Height:     h,
		filter:     filterFlate,
		colorSpace: "DeviceRGB",
		bpc:        8,
		data:       comp,
	}
	if hasAlpha {
		sm, err := zlibCompress(alpha)
		if err != nil {
			return nil, err
		}
		im.smask = sm
	}
	return im, nil
}

// zlibCompress zlib-compresses b, matching the PDF FlateDecode filter.
func zlibCompress(b []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := zlib.NewWriter(&buf)
	if _, err := zw.Write(b); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// jpegGray is the color model reported by image/jpeg for grayscale images.
var jpegGray = image.NewGray(image.Rect(0, 0, 1, 1)).ColorModel()
