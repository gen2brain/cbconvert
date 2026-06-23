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
	nearestNeighbor int = iota
	// Box filter (averaging pixels).
	box
	// Linear is the bilinear filter, smooth and reasonably fast.
	linear
	// MitchellNetravali is a smooth bicubic filter.
	mitchellNetravali
	// CatmullRom is a sharp bicubic filter.
	catmullRom
	// Gaussian is a blurring filter, which uses gaussian function, useful for noise removal.
	gaussian
	// Lanczos is a high-quality resampling filter, it's slower than cubic filters.
	lanczos
)

var filters = map[int]transform.ResampleFilter{
	nearestNeighbor:   transform.NearestNeighbor,
	box:               transform.Box,
	linear:            transform.Linear,
	mitchellNetravali: transform.MitchellNetravali,
	catmullRom:        transform.CatmullRom,
	gaussian:          transform.Gaussian,
	lanczos:           transform.Lanczos,
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

// isGrayScale checks if the image is grayscale.
func isGrayScale(img image.Image) bool {
	model := img.ColorModel()
	if model == color.GrayModel || model == color.Gray16Model {
		return true
	}

	return false
}

var colors16 = []color.Color{
	color.RGBA{A: 255},
	color.RGBA{R: 17, G: 17, B: 17, A: 255},
	color.RGBA{R: 34, G: 34, B: 34, A: 255},
	color.RGBA{R: 51, G: 51, B: 51, A: 255},
	color.RGBA{R: 68, G: 68, B: 68, A: 255},
	color.RGBA{R: 85, G: 85, B: 85, A: 255},
	color.RGBA{R: 102, G: 102, B: 102, A: 255},
	color.RGBA{R: 119, G: 119, B: 119, A: 255},
	color.RGBA{R: 136, G: 136, B: 136, A: 255},
	color.RGBA{R: 153, G: 153, B: 153, A: 255},
	color.RGBA{R: 170, G: 170, B: 170, A: 255},
	color.RGBA{R: 187, G: 187, B: 187, A: 255},
	color.RGBA{R: 204, G: 204, B: 204, A: 255},
	color.RGBA{R: 221, G: 221, B: 221, A: 255},
	color.RGBA{R: 238, G: 238, B: 238, A: 255},
	color.RGBA{R: 255, G: 255, B: 255, A: 255},
}

// imageToPaletted converts an image.Image to *image.Paletted using 16-color palette.
func imageToPaletted(src image.Image) *image.Paletted {
	b := src.Bounds()
	dst := image.NewPaletted(b, colors16)
	draw.FloydSteinberg.Draw(dst, b, imageToGray(src), b.Min)

	return dst
}
