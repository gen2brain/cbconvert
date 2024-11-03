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
	// Gaussian is a blurring filter that uses gaussian function, useful for noise removal.
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
	color.RGBA{17, 17, 17, 255},
	color.RGBA{34, 34, 34, 255},
	color.RGBA{51, 51, 51, 255},
	color.RGBA{68, 68, 68, 255},
	color.RGBA{85, 85, 85, 255},
	color.RGBA{102, 102, 102, 255},
	color.RGBA{119, 119, 119, 255},
	color.RGBA{136, 136, 136, 255},
	color.RGBA{153, 153, 153, 255},
	color.RGBA{170, 170, 170, 255},
	color.RGBA{187, 187, 187, 255},
	color.RGBA{204, 204, 204, 255},
	color.RGBA{221, 221, 221, 255},
	color.RGBA{238, 238, 238, 255},
	color.RGBA{255, 255, 255, 255},
}

// imageToPaletted converts an image.Image to *image.Paletted using 16-color palette.
func imageToPaletted(src image.Image) *image.Paletted {
	b := src.Bounds()
	dst := image.NewPaletted(b, colors16)
	draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Src)

	return dst
}
