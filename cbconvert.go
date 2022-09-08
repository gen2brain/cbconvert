package cbconvert

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/md5"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"mime"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/chai2010/webp"
	_ "github.com/hotei/bmp"
	"github.com/strukturag/libheif/go/heif"
	"golang.org/x/image/tiff"

	"github.com/disintegration/imaging"
	"github.com/fvbommel/sortorder"
	"github.com/gen2brain/go-fitz"
	"github.com/gen2brain/go-unarr"
	"golang.org/x/sync/errgroup"
	"gopkg.in/gographics/imagick.v3/imagick"
)

// Resample filters.
const (
	// NearestNeighbor is the fastest resampling filter, no antialiasing
	NearestNeighbor int = iota
	// Box filter (averaging pixels)
	Box
	// Linear is the bilinear filter, smooth and reasonably fast
	Linear
	// MitchellNetravali is a smooth bicubic filter
	MitchellNetravali
	// CatmullRom is a sharp bicubic filter
	CatmullRom
	// Gaussian is a blurring filter that uses gaussian function, useful for noise removal
	Gaussian
	// Lanczos is a high-quality resampling filter, it's slower than cubic filters
	Lanczos
)

var filters = map[int]imaging.ResampleFilter{
	NearestNeighbor:   imaging.NearestNeighbor,
	Box:               imaging.Box,
	Linear:            imaging.Linear,
	MitchellNetravali: imaging.MitchellNetravali,
	CatmullRom:        imaging.CatmullRom,
	Gaussian:          imaging.Gaussian,
	Lanczos:           imaging.Lanczos,
}

// Options type.
type Options struct {
	// Image format, valid values are jpeg, png, tiff, bmp, webp, avif
	Format string
	// JPEG image quality
	Quality int
	// Lossless compression (avif)
	Lossless bool
	// Image width
	Width int
	// Image height
	Height int
	// Best fit for required width and height
	Fit bool
	// 0=NearestNeighbor, 1=Box, 2=Linear, 3=MitchellNetravali, 4=CatmullRom, 6=Gaussian, 7=Lanczos
	Filter int
	// Do not convert the cover image
	NoCover bool
	// Do not convert images that have RGB colorspace
	NoRGB bool
	// Remove non-image files from the archive
	NoNonImage bool
	// Do not transform or convert images
	NoConvert bool
	// Add suffix to file baseNoExt
	Suffix string
	// Extract cover
	Cover bool
	// Extract cover thumbnail (freedesktop spec.)
	Thumbnail bool
	// Output file
	Outfile string
	// Output directory
	Outdir string
	// Convert images to grayscale (monochromatic)
	Grayscale bool
	// Rotate images, valid values are 0, 90, 180, 270
	Rotate int
	// Flip images, valid values are none, horizontal, vertical
	Flip string
	// Adjust the brightness of the images, must be in the range (-100, 100)
	Brightness float64
	// Adjust the contrast of the images, must be in the range (-100, 100)
	Contrast float64
	// Process subdirectories recursively
	Recursive bool
	// Process only files larger than size (in MB)
	Size int64
	// Hide console output
	Quiet bool
	// Shadow input value
	LevelsInMin float64
	// Highlight input value
	LevelsInMax float64
	// Midpoint/gamma
	LevelsGamma float64
	// Shadow output value
	LevelsOutMin float64
	// Highlight output value
	LevelsOutMax float64
}

// Convertor type.
type Convertor struct {
	// Options struct
	Opts Options
	// Current working directory
	Workdir string
	// Number of files
	Nfiles int
	// Index of current file
	CurrFile int
	// Number of contents in archive/document
	Ncontents int
	// Index of current content
	CurrContent int32
	// Start function
	OnStart func()
	// Progress function
	OnProgress func()
	// Compress function
	OnCompress func()
}

// New returns new convertor.
func New(o Options) *Convertor {
	c := &Convertor{}
	c.Opts = o
	return c
}

// convertImage converts image.Image.
func (c *Convertor) convertImage(ctx context.Context, img image.Image, index int, pathName string) error {
	err := ctx.Err()
	if err != nil {
		return err
	}

	atomic.AddInt32(&c.CurrContent, 1)
	if c.OnProgress != nil {
		c.OnProgress()
	}

	var ext = c.Opts.Format
	var fileName string
	if pathName != "" {
		fileName = filepath.Join(c.Workdir, fmt.Sprintf("%s.%s", c.baseNoExt(pathName), ext))
	} else {
		fileName = filepath.Join(c.Workdir, fmt.Sprintf("%03d.%s", index, ext))
	}

	img = c.transformImage(img)

	if c.Opts.LevelsInMin != 0 || c.Opts.LevelsInMax != 255 || c.Opts.LevelsGamma != 1.0 ||
		c.Opts.LevelsOutMin != 0 || c.Opts.LevelsOutMax != 255 {
		img, err = c.levelImage(img)
		if err != nil {
			return err
		}
	}

	switch c.Opts.Format {
	case "jpeg":
		err = c.encodeImage(img, fileName)
		if err != nil {
			return err
		}
	case "png":
		err = c.encodeImage(img, fileName)
		if err != nil {
			return err
		}
	case "tiff":
		err = c.encodeImage(img, fileName)
		if err != nil {
			return err
		}
	case "bmp":
		// convert image to 4-Bit BMP (16 colors)
		err = c.encodeIM(img, fileName)
		if err != nil {
			return err
		}
	case "webp":
		err = c.encodeImage(img, fileName)
		if err != nil {
			return err
		}
	case "avif":
		err = c.encodeImage(img, fileName)
		if err != nil {
			return err
		}
	}

	return nil
}

// transformImage transforms image (resize, rotate, flip, brightness, contrast).
func (c *Convertor) transformImage(img image.Image) image.Image {
	var i = img

	if c.Opts.Width > 0 || c.Opts.Height > 0 {
		if c.Opts.Fit {
			i = imaging.Fit(i, c.Opts.Width, c.Opts.Height, filters[c.Opts.Filter])
		} else {
			i = imaging.Resize(i, c.Opts.Width, c.Opts.Height, filters[c.Opts.Filter])
		}
	}

	if c.Opts.Rotate > 0 {
		switch c.Opts.Rotate {
		case 90:
			i = imaging.Rotate90(i)
		case 180:
			i = imaging.Rotate180(i)
		case 270:
			i = imaging.Rotate270(i)
		}
	}

	if c.Opts.Flip != "none" {
		switch c.Opts.Flip {
		case "horizontal":
			i = imaging.FlipH(i)
		case "vertical":
			i = imaging.FlipV(i)
		}
	}

	if c.Opts.Brightness != 0 {
		i = imaging.AdjustBrightness(i, c.Opts.Brightness)
	}

	if c.Opts.Contrast != 0 {
		i = imaging.AdjustContrast(i, c.Opts.Contrast)
	}

	return i
}

// levelImage applies a Photoshop-like levels operation on an image.
func (c *Convertor) levelImage(img image.Image) (image.Image, error) {
	imagick.Initialize()

	mw := imagick.NewMagickWand()
	defer mw.Destroy()

	rgba := imageToRGBA(img)
	err := mw.ConstituteImage(uint(img.Bounds().Dx()), uint(img.Bounds().Dy()), "RGBA", imagick.PIXEL_CHAR, rgba.Pix)
	if err != nil {
		return img, fmt.Errorf("levelImage: %w", err)
	}

	_, qrange := imagick.GetQuantumRange()
	quantumRange := float64(qrange)

	inmin := (quantumRange * c.Opts.LevelsInMin) / 255
	inmax := (quantumRange * c.Opts.LevelsInMax) / 255
	outmin := (quantumRange * c.Opts.LevelsOutMin) / 255
	outmax := (quantumRange * c.Opts.LevelsOutMax) / 255

	err = mw.LevelImage(inmin, c.Opts.LevelsGamma, inmax)
	if err != nil {
		return img, fmt.Errorf("levelImage: %w", err)
	}

	err = mw.LevelImage(-outmin, 1.0, quantumRange+(quantumRange-outmax))
	if err != nil {
		return img, fmt.Errorf("levelImage: %w", err)
	}

	blob := mw.GetImageBlob()
	i, err := c.decodeImage(bytes.NewReader(blob), "levels")
	if err != nil {
		return img, fmt.Errorf("levelImage: %w", err)
	}

	return i, nil
}

// convertDocument converts PDF/EPUB document to CBZ.
func (c *Convertor) convertDocument(fileName string) error {
	c.Workdir, _ = os.MkdirTemp(os.TempDir(), "cbc")

	doc, err := fitz.New(fileName)
	if err != nil {
		return fmt.Errorf("convertDocument: %w", err)
	}

	defer doc.Close()

	c.Ncontents = doc.NumPage()
	c.CurrContent = 0

	if c.OnStart != nil {
		c.OnStart()
	}

	eg, ctx := errgroup.WithContext(context.Background())
	eg.SetLimit(runtime.NumCPU() + 1)

	for n := 0; n < c.Ncontents; n++ {
		img, err := doc.Image(n)
		if err != nil {
			return fmt.Errorf("convertDocument: %w", err)
		}

		if img != nil {
			eg.Go(func() error {
				return c.convertImage(ctx, img, n, "")
			})
		}
	}

	return eg.Wait()
}

// convertArchive converts archive to CBZ.
func (c *Convertor) convertArchive(fileName string) error {
	c.Workdir, _ = os.MkdirTemp(os.TempDir(), "cbc")

	contents, err := c.listArchive(fileName)
	if err != nil {
		return fmt.Errorf("convertArchive: %w", err)
	}

	images := c.imagesFromSlice(contents)

	c.Ncontents = len(images)
	c.CurrContent = 0

	if c.OnStart != nil {
		c.OnStart()
	}

	cover := c.coverName(images)

	archive, err := unarr.NewArchive(fileName)
	if err != nil {
		return fmt.Errorf("convertArchive: %w", err)
	}
	defer archive.Close()

	eg, ctx := errgroup.WithContext(context.Background())
	eg.SetLimit(runtime.NumCPU() + 1)

	for {
		err := archive.Entry()
		if err != nil {
			if err == io.EOF {
				break
			} else {
				return fmt.Errorf("convertArchive: %w", err)
			}
		}

		data, err := archive.ReadAll()
		if err != nil {
			return fmt.Errorf("convertArchive: %w", err)
		}

		pathName := archive.Name()

		if c.isImage(pathName) {
			if c.Opts.NoConvert {
				err = c.copyFile(bytes.NewReader(data), filepath.Join(c.Workdir, filepath.Base(pathName)))
				if err != nil {
					return fmt.Errorf("convertArchive: %w", err)
				}

				continue
			}

			img, err := c.decodeImage(bytes.NewReader(data), pathName)
			if err != nil {
				return fmt.Errorf("convertArchive: %w", err)
			}

			if cover == pathName && c.Opts.NoCover {
				img = c.transformImage(img)
				err = c.encodeImage(img, filepath.Join(c.Workdir, filepath.Base(pathName)))
				if err != nil {
					return fmt.Errorf("convertArchive: %w", err)
				}

				continue
			}

			if c.Opts.NoRGB && !c.isGrayScale(img) {
				img = c.transformImage(img)
				err = c.encodeImage(img, filepath.Join(c.Workdir, filepath.Base(pathName)))
				if err != nil {
					return fmt.Errorf("convertArchive: %w", err)
				}

				continue
			}

			if img != nil {
				eg.Go(func() error {
					return c.convertImage(ctx, img, 0, pathName)
				})
			}
		} else {
			if !c.Opts.NoNonImage {
				err = c.copyFile(bytes.NewReader(data), filepath.Join(c.Workdir, filepath.Base(pathName)))
				if err != nil {
					return fmt.Errorf("convertArchive: %w", err)
				}
			}
		}
	}

	return eg.Wait()
}

// convertDirectory converts directory to CBZ.
func (c *Convertor) convertDirectory(dirPath string) error {
	c.Workdir, _ = os.MkdirTemp(os.TempDir(), "cbc")

	contents, err := c.imagesFromPath(dirPath)
	if err != nil {
		return fmt.Errorf("convertDirectory: %w", err)
	}

	images := c.imagesFromSlice(contents)
	c.Ncontents = len(images)
	c.CurrContent = 0

	if c.OnStart != nil {
		c.OnStart()
	}

	eg, ctx := errgroup.WithContext(context.Background())
	eg.SetLimit(runtime.NumCPU() + 1)

	for index, img := range contents {
		file, err := os.Open(img)
		if err != nil {
			return fmt.Errorf("convertDirectory: %w", err)
		}

		if c.isNonImage(img) && !c.Opts.NoNonImage {
			err = c.copyFile(file, filepath.Join(c.Workdir, filepath.Base(img)))
			if err != nil {
				return fmt.Errorf("convertArchive: %w", err)
			}

			err = file.Close()
			if err != nil {
				return fmt.Errorf("convertDirectory: %w", err)
			}

			continue
		} else if c.isImage(img) {

			i, err := c.decodeImage(file, img)
			if err != nil {
				return fmt.Errorf("convertDirectory: %w", err)
			}

			if c.Opts.NoRGB && !c.isGrayScale(i) {
				i = c.transformImage(i)
				err = c.encodeImage(i, filepath.Join(c.Workdir, filepath.Base(img)))
				if err != nil {
					return fmt.Errorf("convertDirectory: %w", err)
				}

				err = file.Close()
				if err != nil {
					return fmt.Errorf("convertDirectory: %w", err)
				}

				continue
			}

			err = file.Close()
			if err != nil {
				return fmt.Errorf("convertDirectory: %w", err)
			}

			if i != nil {
				eg.Go(func() error {
					return c.convertImage(ctx, i, index, img)
				})
			}
		}
	}

	return eg.Wait()
}

// saveArchive saves workdir to CBZ archive.
func (c *Convertor) saveArchive(fileName string) error {
	if c.OnCompress != nil {
		c.OnCompress()
	}

	zipname := filepath.Join(c.Opts.Outdir, fmt.Sprintf("%s%s.cbz", c.baseNoExt(fileName), c.Opts.Suffix))

	zipfile, err := os.Create(zipname)
	if err != nil {
		return fmt.Errorf("saveArchive: %w", err)
	}

	z := zip.NewWriter(zipfile)

	files, err := os.ReadDir(c.Workdir)
	if err != nil {
		return fmt.Errorf("saveArchive: %w", err)
	}

	for _, file := range files {
		r, err := os.ReadFile(filepath.Join(c.Workdir, file.Name()))
		if err != nil {
			return fmt.Errorf("saveArchive: %w", err)
		}

		info, err := file.Info()
		if err != nil {
			return fmt.Errorf("saveArchive: %w", err)
		}

		zipinfo, err := zip.FileInfoHeader(info)
		if err != nil {
			return fmt.Errorf("saveArchive: %w", err)
		}

		zipinfo.Method = zip.Deflate
		w, err := z.CreateHeader(zipinfo)
		if err != nil {
			return fmt.Errorf("saveArchive: %w", err)
		}

		_, err = w.Write(r)
		if err != nil {
			return fmt.Errorf("saveArchive: %w", err)
		}
	}

	err = z.Close()
	if err != nil {
		return fmt.Errorf("saveArchive: %w", err)
	}

	err = zipfile.Close()
	if err != nil {
		return fmt.Errorf("saveArchive: %w", err)
	}

	return os.RemoveAll(c.Workdir)
}

// decodeImage decodes image from reader.
func (c *Convertor) decodeImage(reader io.Reader, fileName string) (img image.Image, err error) {
	img, _, err = image.Decode(reader)
	if err != nil {
		err = fmt.Errorf("decodeImage: %s: %w", fileName, err)
	}

	return
}

// decodeIM decodes image from reader (ImageMagick).
func (c *Convertor) decodeIM(reader io.Reader, fileName string) (img image.Image, err error) {
	imagick.Initialize()

	mw := imagick.NewMagickWand()
	defer mw.Destroy()

	var data []byte
	var out interface{}

	data, err = io.ReadAll(reader)
	if err != nil {
		return img, fmt.Errorf("decodeIM: %w", err)
	}

	err = mw.SetFilename(fileName)
	if err != nil {
		return img, fmt.Errorf("decodeIM: %w", err)
	}

	err = mw.ReadImageBlob(data)
	if err != nil {
		return img, fmt.Errorf("decodeIM: %w", err)
	}

	w := mw.GetImageWidth()
	h := mw.GetImageHeight()

	out, err = mw.ExportImagePixels(0, 0, w, h, "RGBA", imagick.PIXEL_CHAR)
	if err != nil {
		return img, fmt.Errorf("decodeIM: %w", err)
	}

	b := image.Rect(0, 0, int(w), int(h))
	rgba := image.NewRGBA(b)
	rgba.Pix = out.([]byte)
	img = rgba

	return
}

// encodeImage encodes image to file.
func (c *Convertor) encodeImage(img image.Image, fileName string) error {
	file, err := os.Create(fileName)
	if err != nil {
		return fmt.Errorf("encodeImage: %w", err)
	}
	defer file.Close()

	if c.Opts.Grayscale {
		img = imageToGray(img)
	}

	switch filepath.Ext(fileName) {
	case ".png":
		err = png.Encode(file, img)
	case ".tif", ".tiff":
		err = tiff.Encode(file, img, &tiff.Options{Compression: tiff.Uncompressed})
	case ".jpg", ".jpeg":
		err = jpeg.Encode(file, img, &jpeg.Options{Quality: c.Opts.Quality})
	case ".webp":
		err = webp.Encode(file, img, &webp.Options{Quality: float32(c.Opts.Quality)})
	case ".avif":
		img = imageToRGBA(img)
		lossLess := heif.LosslessModeDisabled
		if c.Opts.Lossless {
			lossLess = heif.LosslessModeEnabled
		}

		ctx, err := heif.EncodeFromImage(img, heif.CompressionAV1, c.Opts.Quality, lossLess, 0)
		if err != nil {
			return fmt.Errorf("encodeImage: %w", err)
		}
		err = ctx.WriteToFile(fileName)
	}
	if err != nil {
		return fmt.Errorf("encodeImage: %w", err)
	}

	return nil
}

// encodeIM encodes image to file (ImageMagick).
func (c *Convertor) encodeIM(i image.Image, fileName string) error {
	imagick.Initialize()

	mw := imagick.NewMagickWand()
	defer mw.Destroy()

	rgba := imageToRGBA(i)
	err := mw.ConstituteImage(uint(i.Bounds().Dx()), uint(i.Bounds().Dy()), "RGBA", imagick.PIXEL_CHAR, rgba.Pix)
	if err != nil {
		return fmt.Errorf("encodeIM: %w", err)
	}

	if c.Opts.Grayscale {
		_ = mw.TransformImageColorspace(imagick.COLORSPACE_GRAY)
	}

	switch filepath.Ext(fileName) {
	case ".png":
		_ = mw.SetImageFormat("PNG")
		_ = mw.WriteImage(fileName)
	case ".tif", ".tiff":
		_ = mw.SetImageFormat("TIFF")
		_ = mw.WriteImage(fileName)
	case ".bmp":
		pw := imagick.NewPixelWand()
		pw.SetColor("black")
		defer pw.Destroy()

		_ = mw.SetImageFormat("BMP3")
		_ = mw.SetImageBackgroundColor(pw)
		_ = mw.SetImageAlphaChannel(imagick.ALPHA_CHANNEL_REMOVE)
		_ = mw.SetImageAlphaChannel(imagick.ALPHA_CHANNEL_DEACTIVATE)
		_ = mw.SetImageMatte(false)
		_ = mw.SetImageCompression(imagick.COMPRESSION_NO)
		_ = mw.QuantizeImage(16, mw.GetImageColorspace(), 1, imagick.DITHER_METHOD_FLOYD_STEINBERG, true)
		_ = mw.WriteImage(fileName)
	case ".jpg", ".jpeg":
		_ = mw.SetImageFormat("JPEG")
		_ = mw.SetImageCompressionQuality(uint(c.Opts.Quality))
		_ = mw.WriteImage(fileName)
	case ".avif":
		_ = mw.SetImageFormat("AVIF")
		_ = mw.SetImageCompressionQuality(uint(c.Opts.Quality))
		_ = mw.WriteImage(fileName)
	}

	return nil
}

// listArchive lists contents of archive.
func (c *Convertor) listArchive(fileName string) ([]string, error) {
	var contents []string

	archive, err := unarr.NewArchive(fileName)
	if err != nil {
		return contents, err
	}
	defer archive.Close()

	return archive.List()
}

// coverArchive extracts cover from archive.
func (c *Convertor) coverArchive(fileName string) (image.Image, error) {
	var images []string

	contents, err := c.listArchive(fileName)
	if err != nil {
		return nil, fmt.Errorf("coverArchive: %w", err)
	}

	for _, ct := range contents {
		if c.isImage(ct) {
			images = append(images, ct)
		}
	}

	cover := c.coverName(images)

	archive, err := unarr.NewArchive(fileName)
	if err != nil {
		return nil, err
	}
	defer archive.Close()

	err = archive.EntryFor(cover)
	if err != nil {
		return nil, err
	}

	data, err := archive.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("coverArchive: %w", err)
	}

	img, err := c.decodeImage(bytes.NewReader(data), cover)
	if err != nil {
		return nil, fmt.Errorf("coverArchive: %w", err)
	}

	return img, nil
}

// coverDocument extracts cover from document.
func (c *Convertor) coverDocument(fileName string) (image.Image, error) {
	doc, err := fitz.New(fileName)
	if err != nil {
		return nil, fmt.Errorf("coverDocument: %w", err)
	}

	defer doc.Close()

	img, err := doc.Image(0)
	if err != nil {
		return nil, fmt.Errorf("coverDocument: %w", err)
	}

	return img, nil
}

// coverDirectory extracts cover from directory.
func (c *Convertor) coverDirectory(dir string) (image.Image, error) {
	contents, err := c.imagesFromPath(dir)
	if err != nil {
		return nil, fmt.Errorf("coverDirectory: %w", err)
	}

	images := c.imagesFromSlice(contents)
	cover := c.coverName(images)

	file, err := os.Open(cover)
	if err != nil {
		return nil, fmt.Errorf("coverDirectory: %w", err)
	}
	defer file.Close()

	img, err := c.decodeImage(file, cover)
	if err != nil {
		return nil, fmt.Errorf("coverDirectory: %w", err)
	}

	return img, nil
}

// imagesFromPath returns list of found image files for given directory.
func (c *Convertor) imagesFromPath(path string) ([]string, error) {
	var images []string

	walkFiles := func(fp string, f os.FileInfo, err error) error {
		if !f.IsDir() && f.Mode()&os.ModeType == 0 {
			if f.Size() > 0 && (c.isImage(fp) || c.isNonImage(fp)) {
				images = append(images, fp)
			}
		}
		return nil
	}

	f, err := filepath.Abs(path)
	if err != nil {
		return images, fmt.Errorf("imagesFromPath: %w", err)
	}

	stat, err := os.Stat(f)
	if err != nil {
		return images, fmt.Errorf("imagesFromPath: %w", err)
	}

	if !stat.IsDir() && stat.Mode()&os.ModeType == 0 {
		if c.isImage(f) {
			images = append(images, f)
		}
	} else {
		err = filepath.Walk(f, walkFiles)
		if err != nil {
			return images, fmt.Errorf("imagesFromPath: %w", err)
		}
	}

	return images, nil
}

// imagesFromSlice returns list of found image files for given slice of files.
func (c *Convertor) imagesFromSlice(files []string) []string {
	var images []string

	for _, f := range files {
		if c.isImage(f) {
			images = append(images, f)
		}
	}

	return images
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

// isArchive checks if file is archive.
func (c *Convertor) isArchive(f string) bool {
	var types = []string{".rar", ".zip", ".7z", ".tar", ".cbr", ".cbz", ".cb7", ".cbt"}
	for _, t := range types {
		if strings.ToLower(filepath.Ext(f)) == t {
			return true
		}
	}
	return false
}

// isDocument checks if file is document.
func (c *Convertor) isDocument(f string) bool {
	var types = []string{".pdf", ".epub"}
	for _, t := range types {
		if strings.ToLower(filepath.Ext(f)) == t {
			return true
		}
	}
	return false
}

// isImage checks if file is image.
func (c *Convertor) isImage(f string) bool {
	var types = []string{".jpg", ".jpeg", ".png", ".gif", ".bmp", ".tiff", ".tif", ".webp", ".avif"}
	for _, t := range types {
		if strings.ToLower(filepath.Ext(f)) == t {
			return true
		}
	}
	return false
}

// isNonImage checks for allowed files in archive.
func (c *Convertor) isNonImage(f string) bool {
	var types = []string{".nfo", ".xml"}
	for _, t := range types {
		if strings.ToLower(filepath.Ext(f)) == t {
			return true
		}
	}
	return false
}

// isSize checks size of file.
func (c *Convertor) isSize(size int64) bool {
	if c.Opts.Size > 0 {
		if size < c.Opts.Size*(1024*1024) {
			return false
		}
	}
	return true
}

// isGrayScale checks if image is grayscale.
func (c *Convertor) isGrayScale(img image.Image) bool {
	model := img.ColorModel()
	if model == color.GrayModel || model == color.Gray16Model {
		return true
	}
	return false
}

// baseNoExt returns base name without extension.
func (c *Convertor) baseNoExt(filename string) string {
	return strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
}

// copyFile copies reader to file.
func (c *Convertor) copyFile(reader io.Reader, filename string) error {
	err := os.MkdirAll(filepath.Dir(filename), 0755)
	if err != nil {
		return fmt.Errorf("copyFile: %w", err)
	}

	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("copyFile: %w", err)
	}
	defer file.Close()

	_, err = io.Copy(file, reader)
	if err != nil {
		return fmt.Errorf("copyFile: %w", err)
	}

	return nil
}

// coverName returns the filename that is the most likely to be the cover.
func (c *Convertor) coverName(images []string) string {
	if len(images) == 0 {
		return ""
	}

	for _, i := range images {
		e := c.baseNoExt(i)
		if strings.HasPrefix(i, "cover") || strings.HasPrefix(i, "front") ||
			strings.HasSuffix(e, "cover") || strings.HasSuffix(e, "front") {
			return i
		}
	}

	sort.Sort(sortorder.Natural(images))
	return images[0]
}

// coverImage returns cover as image.Image.
func (c *Convertor) coverImage(fileName string, fileInfo os.FileInfo) (image.Image, error) {
	var err error
	var cover image.Image

	if fileInfo.IsDir() {
		cover, err = c.coverDirectory(fileName)
	} else if c.isDocument(fileName) {
		cover, err = c.coverDocument(fileName)
	} else {
		cover, err = c.coverArchive(fileName)
	}

	if c.OnProgress != nil {
		c.OnProgress()
	}

	if err != nil {
		return nil, fmt.Errorf("coverImage: %w", err)
	}

	return cover, nil
}

// Files returns list of found comic files.
func (c *Convertor) Files(args []string) ([]string, error) {
	var files []string

	walkFiles := func(fp string, f os.FileInfo, err error) error {
		if f.IsDir() {
			return nil
		}
		if c.isArchive(fp) || c.isDocument(fp) {
			if c.isSize(f.Size()) {
				files = append(files, fp)
			}
		}
		return nil
	}

	for _, arg := range args {
		path, err := filepath.Abs(arg)
		if err != nil {
			return files, fmt.Errorf("files: %w", err)
		}

		stat, err := os.Stat(path)
		if err != nil {
			return files, fmt.Errorf("files: %w", err)
		}

		if !stat.IsDir() {
			if c.isArchive(path) || c.isDocument(path) {
				if c.isSize(stat.Size()) {
					files = append(files, path)
				}
			}
		} else {
			if c.Opts.Recursive {
				err = filepath.Walk(path, walkFiles)
				if err != nil {
					return files, fmt.Errorf("files: %w", err)
				}
			} else {
				fs, err := os.ReadDir(path)
				if err != nil {
					return files, fmt.Errorf("files: %w", err)
				}

				for _, f := range fs {
					if c.isArchive(f.Name()) || c.isDocument(f.Name()) {
						info, err := f.Info()
						if err != nil {
							return files, fmt.Errorf("files: %w", err)
						}
						if c.isSize(info.Size()) {
							files = append(files, filepath.Join(path, f.Name()))
						}
					}
				}
			}

			if len(files) == 0 {
				// append plain directory with images
				files = append(files, path)
			}
		}
	}

	c.Nfiles = len(files)
	return files, nil
}

// ExtractCover extracts cover.
func (c *Convertor) ExtractCover(fileName string, fileInfo os.FileInfo) error {
	c.CurrFile++

	cover, err := c.coverImage(fileName, fileInfo)
	if err != nil {
		return fmt.Errorf("extractCover: %w", err)
	}

	if c.Opts.Width > 0 || c.Opts.Height > 0 {
		if c.Opts.Fit {
			cover = imaging.Fit(cover, c.Opts.Width, c.Opts.Height, filters[c.Opts.Filter])
		} else {
			cover = imaging.Resize(cover, c.Opts.Width, c.Opts.Height, filters[c.Opts.Filter])
		}
	}

	fname := filepath.Join(c.Opts.Outdir, fmt.Sprintf("%s.jpg", c.baseNoExt(fileName)))
	file, err := os.Create(fname)
	if err != nil {
		return fmt.Errorf("extractCover: %w", err)
	}

	err = jpeg.Encode(file, cover, &jpeg.Options{Quality: c.Opts.Quality})
	if err != nil {
		return fmt.Errorf("extractCover: %w", err)
	}

	err = file.Close()
	if err != nil {
		return fmt.Errorf("extractCover: %w", err)
	}

	return nil
}

// ExtractThumbnail extracts thumbnail.
func (c *Convertor) ExtractThumbnail(filename string, info os.FileInfo) error {
	c.CurrFile++

	cover, err := c.coverImage(filename, info)
	if err != nil {
		return err
	}

	if c.Opts.Width > 0 || c.Opts.Height > 0 {
		if c.Opts.Fit {
			cover = imaging.Fit(cover, c.Opts.Width, c.Opts.Height, filters[c.Opts.Filter])
		} else {
			cover = imaging.Resize(cover, c.Opts.Width, c.Opts.Height, filters[c.Opts.Filter])
		}
	} else {
		cover = imaging.Resize(cover, 256, 0, filters[c.Opts.Filter])
	}

	imagick.Initialize()

	mw := imagick.NewMagickWand()
	defer mw.Destroy()

	rgba := imageToRGBA(cover)
	err = mw.ConstituteImage(uint(cover.Bounds().Dx()), uint(cover.Bounds().Dy()), "RGBA", imagick.PIXEL_CHAR, rgba.Pix)
	if err != nil {
		return fmt.Errorf("extractThumbnail: %w", err)
	}

	var fname string
	var furi string

	if c.Opts.Outfile == "" {
		furi = "file://" + filename
		fname = filepath.Join(c.Opts.Outdir, fmt.Sprintf("%x.png", md5.Sum([]byte(furi))))
	} else {
		abs, _ := filepath.Abs(c.Opts.Outfile)
		furi = "file://" + abs
		fname = abs
	}

	_ = mw.SetImageFormat("PNG")
	_ = mw.SetImageProperty("Software", "CBconvert")
	_ = mw.SetImageProperty("Description", "Thumbnail of "+furi)
	_ = mw.SetImageProperty("Thumb::URI", furi)
	_ = mw.SetImageProperty("Thumb::MTime", strconv.FormatInt(info.ModTime().Unix(), 10))
	_ = mw.SetImageProperty("Thumb::Size", strconv.FormatInt(info.Size(), 10))
	_ = mw.SetImageProperty("Thumb::Mimetype", mime.TypeByExtension(filepath.Ext(filename)))

	_ = mw.WriteImage(fname)
	return nil
}

// Convert converts comic book.
func (c *Convertor) Convert(filename string, info os.FileInfo) error {
	c.CurrFile++

	if info.IsDir() {
		err := c.convertDirectory(filename)
		if err != nil {
			return err
		}
		err = c.saveArchive(filename)
		if err != nil {
			return err
		}
	} else if c.isDocument(filename) {
		err := c.convertDocument(filename)
		if err != nil {
			return err
		}
		err = c.saveArchive(filename)
		if err != nil {
			return err
		}
	} else {
		err := c.convertArchive(filename)
		if err != nil {
			return err
		}
		err = c.saveArchive(filename)
		if err != nil {
			return err
		}
	}

	return nil
}
