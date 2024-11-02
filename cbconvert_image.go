package cbconvert

import (
	"image"
	"image/color"
	"image/draw"
	"math"

	"github.com/anthonynsimon/bild/adjust"
	"github.com/anthonynsimon/bild/transform"
)

// Resample filters.
const (
	// NearestNeighbor is the fastest resampling filter, no antialiasing.
	NearestNeighbor int = iota
	// Box filter (averaging pixels).
	Box
	// Linear is the bilinear filter, smooth and reasonably fast.
	Linear
	// MitchellNetravali is a smooth bicubic filter.
	MitchellNetravali
	// CatmullRom is a sharp bicubic filter.
	CatmullRom
	// Gaussian is a blurring filter that uses gaussian function, useful for noise removal.
	Gaussian
	// Lanczos is a high-quality resampling filter, it's slower than cubic filters.
	Lanczos
)

var filters = map[int]transform.ResampleFilter{
	NearestNeighbor:   transform.NearestNeighbor,
	Box:               transform.Box,
	Linear:            transform.Linear,
	MitchellNetravali: transform.MitchellNetravali,
	CatmullRom:        transform.CatmullRom,
	Gaussian:          transform.Gaussian,
	Lanczos:           transform.Lanczos,
}

func resize(img image.Image, width, height int, filter transform.ResampleFilter) *image.RGBA {
	dstW, dstH := width, height

	srcW := img.Bounds().Dx()
	srcH := img.Bounds().Dy()

	if dstW == 0 {
		tmpW := float64(dstH) * float64(srcW) / float64(srcH)
		dstW = int(math.Max(1.0, math.Floor(tmpW+0.5)))
	}
	if dstH == 0 {
		tmpH := float64(dstW) * float64(srcH) / float64(srcW)
		dstH = int(math.Max(1.0, math.Floor(tmpH+0.5)))
	}

	if srcW == dstW && srcH == dstH {
		return imageToRGBA(img)
	}

	return transform.Resize(img, dstW, dstH, filter)
}

func fit(img image.Image, width, height int, filter transform.ResampleFilter) *image.RGBA {
	maxW, maxH := width, height

	b := img.Bounds()
	srcW := b.Dx()
	srcH := b.Dy()

	if srcW <= maxW && srcH <= maxH {
		return imageToRGBA(img)
	}

	srcAspectRatio := float64(srcW) / float64(srcH)
	maxAspectRatio := float64(maxW) / float64(maxH)

	var dstW, dstH int
	if srcAspectRatio > maxAspectRatio {
		dstW = maxW
		dstH = int(float64(dstW) / srcAspectRatio)
	} else {
		dstH = maxH
		dstW = int(float64(dstH) * srcAspectRatio)
	}

	return resize(img, dstW, dstH, filter)
}

func rotate(img image.Image, angle float64) *image.RGBA {
	return transform.Rotate(img, angle, &transform.RotationOptions{ResizeBounds: true, Pivot: &image.Point{}})
}

func brightness(img image.Image, change float64) *image.RGBA {
	return adjust.Brightness(img, change/100)
}

func contrast(img image.Image, change float64) *image.RGBA {
	return adjust.Contrast(img, change/100)
}

// imageToRGBA converts an image.Image to *image.RGBA.
func imageToRGBA(src image.Image) *image.RGBA {
	if dst, ok := src.(*image.RGBA); ok {
		return dst
	}

	b := src.Bounds()
	dst := image.NewRGBA(b)
	draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Src)

	return dst
}

// imageToGray converts an image.Image to *image.Gray.
func imageToGray(src image.Image) *image.Gray {
	if dst, ok := src.(*image.Gray); ok {
		return dst
	}

	b := src.Bounds()
	dst := image.NewGray(b)
	draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Src)

	return dst
}

// isGrayScale checks if image is grayscale.
func isGrayScale(img image.Image) bool {
	model := img.ColorModel()
	if model == color.GrayModel || model == color.Gray16Model {
		return true
	}

	return false
}

var colors16 = []color.Color{
	color.RGBA{0, 0, 0, 255},
	color.RGBA{24, 24, 24, 255},
	color.RGBA{40, 40, 40, 255},
	color.RGBA{56, 56, 56, 255},
	color.RGBA{71, 71, 71, 255},
	color.RGBA{86, 86, 86, 255},
	color.RGBA{100, 100, 100, 255},
	color.RGBA{113, 113, 113, 255},
	color.RGBA{126, 126, 126, 255},
	color.RGBA{140, 140, 140, 255},
	color.RGBA{155, 155, 155, 255},
	color.RGBA{171, 171, 171, 255},
	color.RGBA{189, 189, 189, 255},
	color.RGBA{209, 209, 209, 255},
	color.RGBA{231, 231, 231, 255},
	color.RGBA{255, 255, 255, 255},
}

// imageToPaletted converts an image.Image to *image.Paletted using 16-color palette.
func imageToPaletted(src image.Image) *image.Paletted {
	b := src.Bounds()
	dst := image.NewPaletted(b, colors16)
	draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Src)

	return dst
}
