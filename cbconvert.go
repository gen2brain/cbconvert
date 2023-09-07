package cbconvert

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"context"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"

	"image"
	"image/color"
	"image/draw"
	_ "image/gif" // allow gif decoding
	"image/jpeg"
	"image/png"

	"github.com/chai2010/webp"
	"github.com/strukturag/libheif/go/heif"
	"golang.org/x/image/tiff"

	"github.com/disintegration/imaging"
	"github.com/dustin/go-humanize"
	"github.com/fvbommel/sortorder"
	"github.com/gen2brain/go-fitz"
	"github.com/gen2brain/go-unarr"
	"golang.org/x/sync/errgroup"
	"gopkg.in/gographics/imagick.v3/imagick"
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
	// Archive format, valid values are zip, tar
	Archive string
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
	// CBZ metadata
	Meta bool
	// Version
	Version bool
	// ZIP comment
	Comment bool
	// ZIP comment body
	CommentBody string
	// Add file
	FileAdd string
	// Remove file
	FileRemove string
	// Output file
	OutFile string
	// Output directory
	OutDir string
	// Convert images to grayscale (monochromatic)
	Grayscale bool
	// Rotate images, valid values are 0, 90, 180, 270
	Rotate int
	// Adjust the brightness of the images, must be in the range (-100, 100)
	Brightness int
	// Adjust the contrast of the images, must be in the range (-100, 100)
	Contrast int
	// Process subdirectories recursively
	Recursive bool
	// Process only files larger than size (in MB)
	Size int
	// Hide console output
	Quiet bool
	// Shadow input value
	LevelsInMin int
	// Highlight input value
	LevelsInMax int
	// Midpoint/gamma
	LevelsGamma float64
	// Shadow output value
	LevelsOutMin int
	// Highlight output value
	LevelsOutMax int
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

// File type.
type File struct {
	Name      string
	Path      string
	Stat      os.FileInfo
	SizeHuman string
}

// Image type.
type Image struct {
	Image     image.Image
	Width     int
	Height    int
	SizeHuman string
}

// New returns new convertor.
func New(o Options) *Convertor {
	c := &Convertor{}
	c.Opts = o

	return c
}

// convertDocument converts PDF/EPUB document to CBZ.
func (c *Convertor) convertDocument(fileName string) error {
	var err error

	c.Workdir, err = os.MkdirTemp(os.TempDir(), "cbc")
	if err != nil {
		return fmt.Errorf("convertDocument: %w", err)
	}

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
			n := n

			eg.Go(func() error {
				return c.imageConvert(ctx, img, n, "")
			})
		}
	}

	err = eg.Wait()
	if err != nil {
		return fmt.Errorf("convertDocument: %w", err)
	}

	return nil
}

// convertArchive converts archive to CBZ.
func (c *Convertor) convertArchive(fileName string) error {
	var err error

	c.Workdir, err = os.MkdirTemp(os.TempDir(), "cbc")
	if err != nil {
		return fmt.Errorf("convertArchive: %w", err)
	}

	contents, err := c.archiveList(fileName)
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
			if errors.Is(err, io.EOF) {
				break
			}

			return fmt.Errorf("convertArchive: %w", err)
		}

		data, err := archive.ReadAll()
		if err != nil {
			return fmt.Errorf("convertArchive: %w", err)
		}

		pathName := archive.Name()

		if c.isImage(pathName) {
			if c.Opts.NoConvert {
				if err = c.copyFile(bytes.NewReader(data), filepath.Join(c.Workdir, filepath.Base(pathName))); err != nil {
					return fmt.Errorf("convertArchive: %w", err)
				}

				continue
			}

			var img image.Image
			img, err = c.imageDecode(bytes.NewReader(data), pathName)
			if err != nil {
				e := err
				img, err = c.imDecode(bytes.NewReader(data), pathName)
				if err != nil {
					return fmt.Errorf("convertArchive: %w: %w", e, err)
				}
			}

			if cover == pathName && c.Opts.NoCover {
				img = c.imageTransform(img)
				if err = c.imageEncode(img, filepath.Join(c.Workdir, filepath.Base(pathName))); err != nil {
					return fmt.Errorf("convertArchive: %w", err)
				}

				continue
			}

			if c.Opts.NoRGB && !c.isGrayScale(img) {
				img = c.imageTransform(img)
				if err = c.imageEncode(img, filepath.Join(c.Workdir, filepath.Base(pathName))); err != nil {
					return fmt.Errorf("convertArchive: %w", err)
				}

				continue
			}

			if img != nil {
				eg.Go(func() error {
					return c.imageConvert(ctx, img, 0, pathName)
				})
			}
		} else if !c.Opts.NoNonImage {
			if err = c.copyFile(bytes.NewReader(data), filepath.Join(c.Workdir, filepath.Base(pathName))); err != nil {
				return fmt.Errorf("convertArchive: %w", err)
			}
		}
	}

	err = eg.Wait()
	if err != nil {
		return fmt.Errorf("convertArchive: %w", err)
	}

	return nil
}

// convertDirectory converts directory to CBZ.
func (c *Convertor) convertDirectory(dirPath string) error {
	var err error

	c.Workdir, err = os.MkdirTemp(os.TempDir(), "cbc")
	if err != nil {
		return fmt.Errorf("convertDirectory: %w", err)
	}

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
			if err = c.copyFile(file, filepath.Join(c.Workdir, filepath.Base(img))); err != nil {
				return fmt.Errorf("convertDirectory: %w", err)
			}

			if err = file.Close(); err != nil {
				return fmt.Errorf("convertDirectory: %w", err)
			}

			continue
		} else if c.isImage(img) {
			if c.Opts.NoConvert {
				if err = c.copyFile(file, filepath.Join(c.Workdir, filepath.Base(img))); err != nil {
					return fmt.Errorf("convertDirectory: %w", err)
				}

				if err = file.Close(); err != nil {
					return fmt.Errorf("convertDirectory: %w", err)
				}

				continue
			}

			var i image.Image
			i, err = c.imageDecode(file, img)
			if err != nil {
				e := err
				_, err = file.Seek(0, io.SeekStart)
				if err != nil {
					return fmt.Errorf("convertDirectory: %w: %w", e, err)
				}

				i, err = c.imDecode(file, img)
				if err != nil {
					return fmt.Errorf("convertDirectory: %w: %w", e, err)
				}
			}

			if c.Opts.NoRGB && !c.isGrayScale(i) {
				i = c.imageTransform(i)
				if err = c.imageEncode(i, filepath.Join(c.Workdir, filepath.Base(img))); err != nil {
					return fmt.Errorf("convertDirectory: %w", err)
				}

				if err = file.Close(); err != nil {
					return fmt.Errorf("convertDirectory: %w", err)
				}

				continue
			}

			if err = file.Close(); err != nil {
				return fmt.Errorf("convertDirectory: %w", err)
			}

			if i != nil {
				index := index
				img := img

				eg.Go(func() error {
					return c.imageConvert(ctx, i, index, img)
				})
			}
		}
	}

	err = eg.Wait()
	if err != nil {
		return fmt.Errorf("convertDirectory: %w", err)
	}

	return nil
}

// imageConvert converts image.Image.
func (c *Convertor) imageConvert(ctx context.Context, img image.Image, index int, pathName string) error {
	err := ctx.Err()
	if err != nil {
		return fmt.Errorf("imageConvert: %w", err)
	}

	atomic.AddInt32(&c.CurrContent, 1)
	if c.OnProgress != nil {
		c.OnProgress()
	}

	var fileName string
	if pathName != "" {
		fileName = filepath.Join(c.Workdir, fmt.Sprintf("%s.%s", c.baseNoExt(pathName), c.Opts.Format))
	} else {
		fileName = filepath.Join(c.Workdir, fmt.Sprintf("%03d.%s", index, c.Opts.Format))
	}

	img = c.imageTransform(img)

	if c.Opts.LevelsInMin != 0 || c.Opts.LevelsInMax != 255 || c.Opts.LevelsGamma != 1.0 ||
		c.Opts.LevelsOutMin != 0 || c.Opts.LevelsOutMax != 255 {
		img, err = c.imageLevel(img)
		if err != nil {
			return err
		}
	}

	switch c.Opts.Format {
	case "jpeg", "png", "tiff", "webp", "avif":
		if err := c.imageEncode(img, fileName); err != nil {
			return fmt.Errorf("imageConvert: %w", err)
		}
	case "bmp":
		// convert image to 4-Bit BMP (16 colors)
		if err := c.imEncode(img, fileName); err != nil {
			return fmt.Errorf("imageConvert: %w", err)
		}
	}

	return nil
}

// imageTransform transforms image (resize, rotate, brightness, contrast).
func (c *Convertor) imageTransform(img image.Image) image.Image {
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

	if c.Opts.Brightness != 0 {
		i = imaging.AdjustBrightness(i, float64(c.Opts.Brightness))
	}

	if c.Opts.Contrast != 0 {
		i = imaging.AdjustContrast(i, float64(c.Opts.Contrast))
	}

	if c.Opts.Grayscale {
		i = imageToGray(i)
	}

	return i
}

// imageLevel applies a Photoshop-like levels operation on an image.
func (c *Convertor) imageLevel(img image.Image) (image.Image, error) {
	mw := imagick.NewMagickWand()
	defer mw.Destroy()

	rgba := imageToRGBA(img)
	err := mw.ConstituteImage(uint(img.Bounds().Dx()), uint(img.Bounds().Dy()), "RGBA", imagick.PIXEL_CHAR, rgba.Pix)
	if err != nil {
		return img, fmt.Errorf("imageLevel: %w", err)
	}

	_, qrange := imagick.GetQuantumRange()
	quantumRange := float64(qrange)

	inMin := (quantumRange * float64(c.Opts.LevelsInMin)) / 255
	inMax := (quantumRange * float64(c.Opts.LevelsInMax)) / 255
	outMin := (quantumRange * float64(c.Opts.LevelsOutMin)) / 255
	outMax := (quantumRange * float64(c.Opts.LevelsOutMax)) / 255

	if err := mw.LevelImage(inMin, c.Opts.LevelsGamma, inMax); err != nil {
		return img, fmt.Errorf("imageLevel: %w", err)
	}

	if err := mw.LevelImage(-outMin, 1.0, quantumRange+(quantumRange-outMax)); err != nil {
		return img, fmt.Errorf("imageLevel: %w", err)
	}

	blob := mw.GetImageBlob()

	var i image.Image
	i, err = c.imageDecode(bytes.NewReader(blob), "levels")
	if err != nil {
		e := err
		i, err = c.imDecode(bytes.NewReader(blob), "levels")
		if err != nil {
			return nil, fmt.Errorf("imageLevel: %w: %w", e, err)
		}
	}

	return i, nil
}

// imageDecode decodes image from reader.
func (c *Convertor) imageDecode(reader io.Reader, fileName string) (image.Image, error) {
	img, _, err := image.Decode(reader)
	if err != nil {
		return img, fmt.Errorf("imageDecode: %s: %w", fileName, err)
	}

	return img, nil
}

// imDecode decodes image from reader (ImageMagick).
func (c *Convertor) imDecode(reader io.Reader, fileName string) (image.Image, error) {
	mw := imagick.NewMagickWand()
	defer mw.Destroy()

	var img image.Image
	var err error
	var data []byte
	var out interface{}

	data, err = io.ReadAll(reader)
	if err != nil {
		return img, fmt.Errorf("imDecode: %w", err)
	}

	if err = mw.SetFilename(fileName); err != nil {
		return img, fmt.Errorf("imDecode: %w", err)
	}

	if err = mw.ReadImageBlob(data); err != nil {
		return img, fmt.Errorf("imDecode: %w", err)
	}

	w := mw.GetImageWidth()
	h := mw.GetImageHeight()

	out, err = mw.ExportImagePixels(0, 0, w, h, "RGBA", imagick.PIXEL_CHAR)
	if err != nil {
		return img, fmt.Errorf("imDecode: %w", err)
	}

	data, ok := out.([]byte)

	if ok {
		b := image.Rect(0, 0, int(w), int(h))
		rgba := image.NewRGBA(b)
		rgba.Pix = data
		img = rgba
	}

	return img, nil
}

// imageEncode encodes image to file.
func (c *Convertor) imageEncode(img image.Image, fileName string) error {
	file, err := os.Create(fileName)
	if err != nil {
		return fmt.Errorf("imageEncode: %w", err)
	}
	defer file.Close()

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

		ctx, e := heif.EncodeFromImage(img, heif.CompressionAV1, c.Opts.Quality, lossLess, 0)
		if e != nil {
			return fmt.Errorf("imageEncode: %w", e)
		}
		err = ctx.WriteToFile(fileName)
	}

	if err != nil {
		return fmt.Errorf("imageEncode: %w", err)
	}

	return nil
}

// imEncode encodes image to file (ImageMagick).
func (c *Convertor) imEncode(i image.Image, fileName string) error {
	mw := imagick.NewMagickWand()
	defer mw.Destroy()

	rgba := imageToRGBA(i)
	if err := mw.ConstituteImage(uint(i.Bounds().Dx()), uint(i.Bounds().Dy()),
		"RGBA", imagick.PIXEL_CHAR, rgba.Pix); err != nil {
		return fmt.Errorf("imEncode: %w", err)
	}

	switch filepath.Ext(fileName) {
	case ".png":
		if err := mw.SetImageFormat("PNG"); err != nil {
			return fmt.Errorf("imEncode: %w", err)
		}
		if err := mw.WriteImage(fileName); err != nil {
			return fmt.Errorf("imEncode: %w", err)
		}
	case ".tif", ".tiff":
		if err := mw.SetImageFormat("TIFF"); err != nil {
			return fmt.Errorf("imEncode: %w", err)
		}
		if err := mw.WriteImage(fileName); err != nil {
			return fmt.Errorf("imEncode: %w", err)
		}
	case ".bmp":
		pw := imagick.NewPixelWand()
		pw.SetColor("black")
		defer pw.Destroy()

		if err := mw.SetImageFormat("BMP3"); err != nil {
			return fmt.Errorf("imEncode: %w", err)
		}
		if err := mw.SetImageBackgroundColor(pw); err != nil {
			return fmt.Errorf("imEncode: %w", err)
		}
		if err := mw.SetImageAlphaChannel(imagick.ALPHA_CHANNEL_REMOVE); err != nil {
			return fmt.Errorf("imEncode: %w", err)
		}
		if err := mw.SetImageAlphaChannel(imagick.ALPHA_CHANNEL_DEACTIVATE); err != nil {
			return fmt.Errorf("imEncode: %w", err)
		}
		if err := mw.SetImageMatte(false); err != nil {
			return fmt.Errorf("imEncode: %w", err)
		}
		if err := mw.SetImageCompression(imagick.COMPRESSION_NO); err != nil {
			return fmt.Errorf("imEncode: %w", err)
		}
		if err := mw.QuantizeImage(16, mw.GetImageColorspace(), 1, imagick.DITHER_METHOD_FLOYD_STEINBERG, true); err != nil {
			return fmt.Errorf("imEncode: %w", err)
		}
		if err := mw.WriteImage(fileName); err != nil {
			return fmt.Errorf("imEncode: %w", err)
		}
	case ".jpg", ".jpeg":
		if err := mw.SetImageFormat("JPEG"); err != nil {
			return fmt.Errorf("imEncode: %w", err)
		}
		if err := mw.SetImageCompressionQuality(uint(c.Opts.Quality)); err != nil {
			return fmt.Errorf("imEncode: %w", err)
		}
		if err := mw.WriteImage(fileName); err != nil {
			return fmt.Errorf("imEncode: %w", err)
		}
	case ".avif":
		if err := mw.SetImageFormat("AVIF"); err != nil {
			return fmt.Errorf("imEncode: %w", err)
		}
		if err := mw.SetImageCompressionQuality(uint(c.Opts.Quality)); err != nil {
			return fmt.Errorf("imEncode: %w", err)
		}
		if err := mw.WriteImage(fileName); err != nil {
			return fmt.Errorf("imEncode: %w", err)
		}
	}

	return nil
}

// archiveSave saves workdir to CBZ archive.
func (c *Convertor) archiveSave(fileName string) error {
	if c.Opts.Archive == "zip" {
		return c.archiveSaveZip(fileName)
	} else if c.Opts.Archive == "tar" {
		return c.archiveSaveTar(fileName)
	}

	return nil
}

// archiveSaveZip saves workdir to CBZ archive.
func (c *Convertor) archiveSaveZip(fileName string) error {
	if c.OnCompress != nil {
		c.OnCompress()
	}

	var zipName string
	if c.Opts.Recursive {
		err := os.MkdirAll(filepath.Join(c.Opts.OutDir, filepath.Dir(fileName)), 0755)
		if err != nil {
			return fmt.Errorf("archiveSaveZip: %w", err)
		}

		zipName = filepath.Join(c.Opts.OutDir, filepath.Dir(fileName), fmt.Sprintf("%s%s.cbz", c.baseNoExt(fileName), c.Opts.Suffix))
	} else {
		zipName = filepath.Join(c.Opts.OutDir, fmt.Sprintf("%s%s.cbz", c.baseNoExt(fileName), c.Opts.Suffix))
	}

	zipFile, err := os.Create(zipName)
	if err != nil {
		return fmt.Errorf("archiveSaveZip: %w", err)
	}

	z := zip.NewWriter(zipFile)

	files, err := os.ReadDir(c.Workdir)
	if err != nil {
		return fmt.Errorf("archiveSaveZip: %w", err)
	}

	for _, file := range files {
		r, err := os.ReadFile(filepath.Join(c.Workdir, file.Name()))
		if err != nil {
			return fmt.Errorf("archiveSaveZip: %w", err)
		}

		info, err := file.Info()
		if err != nil {
			return fmt.Errorf("archiveSaveZip: %w", err)
		}

		zipInfo, err := zip.FileInfoHeader(info)
		if err != nil {
			return fmt.Errorf("archiveSaveZip: %w", err)
		}

		zipInfo.Method = zip.Deflate
		w, err := z.CreateHeader(zipInfo)
		if err != nil {
			return fmt.Errorf("archiveSaveZip: %w", err)
		}

		_, err = w.Write(r)
		if err != nil {
			return fmt.Errorf("archiveSaveZip: %w", err)
		}
	}

	if err = z.Close(); err != nil {
		return fmt.Errorf("archiveSaveZip: %w", err)
	}

	if err = zipFile.Close(); err != nil {
		return fmt.Errorf("archiveSaveZip: %w", err)
	}

	err = os.RemoveAll(c.Workdir)
	if err != nil {
		return fmt.Errorf("archiveSaveZip: %w", err)
	}

	return nil
}

// archiveSaveTar saves workdir to CBT archive.
func (c *Convertor) archiveSaveTar(fileName string) error {
	if c.OnCompress != nil {
		c.OnCompress()
	}

	var tarName string
	if c.Opts.Recursive {
		err := os.MkdirAll(filepath.Join(c.Opts.OutDir, filepath.Dir(fileName)), 0755)
		if err != nil {
			return fmt.Errorf("archiveSaveTar: %w", err)
		}

		tarName = filepath.Join(c.Opts.OutDir, filepath.Dir(fileName), fmt.Sprintf("%s%s.cbt", c.baseNoExt(fileName), c.Opts.Suffix))
	} else {
		tarName = filepath.Join(c.Opts.OutDir, fmt.Sprintf("%s%s.cbt", c.baseNoExt(fileName), c.Opts.Suffix))
	}

	tarFile, err := os.Create(tarName)
	if err != nil {
		return fmt.Errorf("archiveSaveTar: %w", err)
	}

	tw := tar.NewWriter(tarFile)

	files, err := os.ReadDir(c.Workdir)
	if err != nil {
		return fmt.Errorf("archiveSaveTar: %w", err)
	}

	for _, file := range files {
		r, err := os.ReadFile(filepath.Join(c.Workdir, file.Name()))
		if err != nil {
			return fmt.Errorf("archiveSaveTar: %w", err)
		}

		info, err := file.Info()
		if err != nil {
			return fmt.Errorf("archiveSaveTar: %w", err)
		}

		header, err := tar.FileInfoHeader(info, info.Name())
		if err != nil {
			return fmt.Errorf("archiveSaveTar: %w", err)
		}

		err = tw.WriteHeader(header)
		if err != nil {
			return fmt.Errorf("archiveSaveTar: %w", err)
		}

		_, err = tw.Write(r)
		if err != nil {
			return fmt.Errorf("archiveSaveTar: %w", err)
		}
	}

	if err = tw.Close(); err != nil {
		return fmt.Errorf("archiveSaveTar: %w", err)
	}

	if err = tarFile.Close(); err != nil {
		return fmt.Errorf("archiveSaveTar: %w", err)
	}

	err = os.RemoveAll(c.Workdir)
	if err != nil {
		return fmt.Errorf("archiveSaveTar: %w", err)
	}

	return nil
}

// archiveList lists contents of archive.
func (c *Convertor) archiveList(fileName string) ([]string, error) {
	var contents []string

	archive, err := unarr.NewArchive(fileName)
	if err != nil {
		return contents, fmt.Errorf("archiveList: %w", err)
	}
	defer archive.Close()

	contents, err = archive.List()
	if err != nil {
		return contents, fmt.Errorf("archiveList: %w", err)
	}

	return contents, nil
}

// archiveComment returns ZIP comment.
func (c *Convertor) archiveComment(fileName string) (string, error) {
	zr, err := zip.OpenReader(fileName)
	if err != nil {
		return "", fmt.Errorf("archiveComment: %w", err)
	}
	defer zr.Close()

	return zr.Comment, nil
}

// archiveSetComment sets ZIP comment.
func (c *Convertor) archiveSetComment(fileName, commentBody string) error {
	zr, err := zip.OpenReader(fileName)
	if err != nil {
		return fmt.Errorf("archiveSetComment: %w", err)
	}
	defer zr.Close()

	zf, err := os.CreateTemp(os.TempDir(), "cbc")
	if err != nil {
		return fmt.Errorf("archiveSetComment: %w", err)
	}

	tmpName := zf.Name()
	defer os.Remove(tmpName)

	zw := zip.NewWriter(zf)
	err = zw.SetComment(commentBody)
	if err != nil {
		return fmt.Errorf("archiveSetComment: %w", err)
	}

	for _, item := range zr.File {
		ir, err := item.OpenRaw()
		if err != nil {
			return fmt.Errorf("archiveSetComment: %w", err)
		}

		item := item

		it, err := zw.CreateRaw(&item.FileHeader)
		if err != nil {
			return fmt.Errorf("archiveSetComment: %w", err)
		}

		_, err = io.Copy(it, ir)
		if err != nil {
			return fmt.Errorf("archiveSetComment: %w", err)
		}
	}

	err = zw.Close()
	if err != nil {
		return fmt.Errorf("archiveSetComment: %w", err)
	}

	err = zf.Close()
	if err != nil {
		return fmt.Errorf("archiveSetComment: %w", err)
	}

	data, err := os.ReadFile(tmpName)
	if err != nil {
		return fmt.Errorf("archiveSetComment: %w", err)
	}

	err = os.WriteFile(fileName, data, 0644)
	if err != nil {
		return fmt.Errorf("archiveSetComment: %w", err)
	}

	return nil
}

// archiveFileAdd adds file to archive.
func (c *Convertor) archiveFileAdd(fileName, newFileName string) error {
	zr, err := zip.OpenReader(fileName)
	if err != nil {
		return fmt.Errorf("archiveFileAdd: %w", err)
	}
	defer zr.Close()

	zf, err := os.CreateTemp(os.TempDir(), "cbc")
	if err != nil {
		return fmt.Errorf("archiveFileAdd: %w", err)
	}

	tmpName := zf.Name()
	defer os.Remove(tmpName)

	zw := zip.NewWriter(zf)

	for _, item := range zr.File {
		if item.Name == newFileName {
			continue
		}

		ir, err := item.OpenRaw()
		if err != nil {
			return fmt.Errorf("archiveFileAdd: %w", err)
		}

		item := item

		it, err := zw.CreateRaw(&item.FileHeader)
		if err != nil {
			return fmt.Errorf("archiveFileAdd: %w", err)
		}

		_, err = io.Copy(it, ir)
		if err != nil {
			return fmt.Errorf("archiveFileAdd: %w", err)
		}
	}

	info, err := os.Stat(newFileName)
	if err != nil {
		return fmt.Errorf("archiveFileAdd: %w", err)
	}

	newData, err := os.ReadFile(newFileName)
	if err != nil {
		return fmt.Errorf("archiveFileAdd: %w", err)
	}

	zipInfo, err := zip.FileInfoHeader(info)
	if err != nil {
		return fmt.Errorf("archiveFileAdd: %w", err)
	}

	zipInfo.Method = zip.Deflate
	w, err := zw.CreateHeader(zipInfo)
	if err != nil {
		return fmt.Errorf("archiveFileAdd: %w", err)
	}

	_, err = w.Write(newData)
	if err != nil {
		return fmt.Errorf("archiveFileAdd: %w", err)
	}

	err = zw.Close()
	if err != nil {
		return fmt.Errorf("archiveFileAdd: %w", err)
	}

	err = zf.Close()
	if err != nil {
		return fmt.Errorf("archiveFileAdd: %w", err)
	}

	data, err := os.ReadFile(tmpName)
	if err != nil {
		return fmt.Errorf("archiveFileAdd: %w", err)
	}

	err = os.WriteFile(fileName, data, 0644)
	if err != nil {
		return fmt.Errorf("archiveFileAdd: %w", err)
	}

	return nil
}

// archiveFileRemove removes files from archive.
func (c *Convertor) archiveFileRemove(fileName, pattern string) error {
	zr, err := zip.OpenReader(fileName)
	if err != nil {
		return fmt.Errorf("archiveFileRemove: %w", err)
	}
	defer zr.Close()

	zf, err := os.CreateTemp(os.TempDir(), "cbc")
	if err != nil {
		return fmt.Errorf("archiveFileRemove: %w", err)
	}

	tmpName := zf.Name()
	defer os.Remove(tmpName)

	zw := zip.NewWriter(zf)

	for _, item := range zr.File {
		matched, err := filepath.Match(pattern, item.Name)
		if err != nil {
			return fmt.Errorf("archiveFileRemove: %w", err)
		}

		if matched {
			continue
		}

		ir, err := item.OpenRaw()
		if err != nil {
			return fmt.Errorf("archiveFileRemove: %w", err)
		}

		item := item

		it, err := zw.CreateRaw(&item.FileHeader)
		if err != nil {
			return fmt.Errorf("archiveFileRemove: %w", err)
		}

		_, err = io.Copy(it, ir)
		if err != nil {
			return fmt.Errorf("archiveFileRemove: %w", err)
		}
	}

	err = zw.Close()
	if err != nil {
		return fmt.Errorf("archiveFileRemove: %w", err)
	}

	err = zf.Close()
	if err != nil {
		return fmt.Errorf("archiveFileRemove: %w", err)
	}

	data, err := os.ReadFile(tmpName)
	if err != nil {
		return fmt.Errorf("archiveFileRemove: %w", err)
	}

	err = os.WriteFile(fileName, data, 0644)
	if err != nil {
		return fmt.Errorf("archiveFileRemove: %w", err)
	}

	return nil
}

// coverArchive extracts cover from archive.
func (c *Convertor) coverArchive(fileName string) (image.Image, error) {
	var images []string

	contents, err := c.archiveList(fileName)
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
		return nil, fmt.Errorf("coverArchive: %w", err)
	}
	defer archive.Close()

	if err = archive.EntryFor(cover); err != nil {
		return nil, fmt.Errorf("coverArchive: %w", err)
	}

	data, err := archive.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("coverArchive: %w", err)
	}

	var img image.Image
	img, err = c.imageDecode(bytes.NewReader(data), cover)
	if err != nil {
		e := err
		img, err = c.imDecode(bytes.NewReader(data), cover)
		if err != nil {
			return nil, fmt.Errorf("coverArchive: %w: %w", e, err)
		}
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

	var img image.Image
	img, err = c.imageDecode(file, cover)
	if err != nil {
		e := err
		_, err = file.Seek(0, io.SeekStart)
		if err != nil {
			return nil, fmt.Errorf("coverDirectory: %w: %w", e, err)
		}

		img, err = c.imDecode(file, cover)
		if err != nil {
			return nil, fmt.Errorf("coverDirectory: %w: %w", e, err)
		}
	}

	return img, nil
}

// coverName returns the filename that is the most likely to be the cover.
func (c *Convertor) coverName(images []string) string {
	if len(images) == 0 {
		return ""
	}

	lower := make([]string, 0)
	for idx, img := range images {
		img = strings.ToLower(img)
		lower = append(lower, img)
		ext := c.baseNoExt(img)

		if strings.HasPrefix(img, "cover") || strings.HasPrefix(img, "front") ||
			strings.HasSuffix(ext, "cover") || strings.HasSuffix(ext, "front") {
			return filepath.ToSlash(images[idx])
		}
	}

	sort.Sort(sortorder.Natural(lower))
	cover := lower[0]

	for idx, img := range images {
		img = strings.ToLower(img)
		if img == cover {
			return filepath.ToSlash(images[idx])
		}
	}

	return ""
}

// coverImage returns cover as image.Image.
func (c *Convertor) coverImage(fileName string, fileInfo os.FileInfo) (image.Image, error) {
	var err error
	var cover image.Image

	switch {
	case fileInfo.IsDir():
		cover, err = c.coverDirectory(fileName)
	case c.isDocument(fileName):
		cover, err = c.coverDocument(fileName)
	case c.isArchive(fileName):
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
	var types = []string{".pdf", ".epub", ".mobi"}
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
	var types = []string{".nfo", ".xml", ".txt"}
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
		if size < int64(c.Opts.Size)*(1024*1024) {
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

// tempName generates a temporary name.
func (c *Convertor) tempName(prefix, suffix string) string {
	randBytes := make([]byte, 16)
	_, _ = rand.Read(randBytes)
	return filepath.Join(os.TempDir(), prefix+hex.EncodeToString(randBytes)+suffix)
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

// Initialize inits ImageMagick.
func (c *Convertor) Initialize() {
	imagick.Initialize()
}

// Terminate terminates ImageMagick.
func (c *Convertor) Terminate() {
	imagick.Terminate()
}

// Files returns list of found comic files.
func (c *Convertor) Files(args []string) ([]File, error) {
	var files []File

	toFile := func(fp string, f os.FileInfo) File {
		var file File
		file.Name = filepath.Base(fp)
		file.Path = fp
		file.Stat = f
		file.SizeHuman = humanize.IBytes(uint64(f.Size()))
		return file
	}

	walkFiles := func(fp string, f os.FileInfo, err error) error {
		if f.IsDir() {
			return nil
		}
		if c.isArchive(fp) || c.isDocument(fp) {
			if c.isSize(f.Size()) {
				files = append(files, toFile(fp, f))
			}
		}

		return nil
	}

	walkDirs := func(fp string, f os.FileInfo, err error) error {
		if f.IsDir() {
			fs, err := os.ReadDir(filepath.Join(filepath.Dir(fp), f.Name()))
			if err != nil {
				return err
			}

			count := 0
			for _, fn := range fs {
				if !fn.IsDir() {
					count++
				}
			}

			if count > 1 {
				files = append(files, toFile(fp, f))
			}
		}

		return nil
	}

	for _, arg := range args {
		path, err := filepath.Abs(arg)
		if err != nil {
			return files, fmt.Errorf("%s: %w", arg, err)
		}

		stat, err := os.Stat(path)
		if err != nil {
			return files, fmt.Errorf("%s: %w", arg, err)
		}

		if !stat.IsDir() {
			if c.isArchive(path) || c.isDocument(path) {
				if c.isSize(stat.Size()) {
					files = append(files, toFile(path, stat))
				}
			}
		} else {
			if c.Opts.Recursive {
				if err := filepath.Walk(path, walkFiles); err != nil {
					return files, fmt.Errorf("%s: %w", arg, err)
				}
			} else {
				fs, err := os.ReadDir(path)
				if err != nil {
					return files, fmt.Errorf("%s: %w", arg, err)
				}

				for _, f := range fs {
					if c.isArchive(f.Name()) || c.isDocument(f.Name()) {
						info, err := f.Info()
						if err != nil {
							return files, fmt.Errorf("%s: %w", arg, err)
						}
						if c.isSize(info.Size()) {
							files = append(files, toFile(filepath.Join(path, f.Name()), info))
						}
					}
				}
			}

			if len(files) == 0 {
				// append plain directory with images
				if c.Opts.Recursive {
					if err := filepath.Walk(path, walkDirs); err != nil {
						return files, fmt.Errorf("%s: %w", arg, err)
					}
				} else {
					files = append(files, toFile(path, stat))
				}
			}
		}
	}

	c.Nfiles = len(files)

	return files, nil
}

// Cover extracts cover.
func (c *Convertor) Cover(fileName string, fileInfo os.FileInfo) error {
	c.CurrFile++

	cover, err := c.coverImage(fileName, fileInfo)
	if err != nil {
		return fmt.Errorf("%s: %w", fileName, err)
	}

	if c.Opts.Width > 0 || c.Opts.Height > 0 {
		if c.Opts.Fit {
			cover = imaging.Fit(cover, c.Opts.Width, c.Opts.Height, filters[c.Opts.Filter])
		} else {
			cover = imaging.Resize(cover, c.Opts.Width, c.Opts.Height, filters[c.Opts.Filter])
		}
	}

	var fName string
	if c.Opts.Recursive {
		err := os.MkdirAll(filepath.Join(c.Opts.OutDir, filepath.Dir(fileName)), 0755)
		if err != nil {
			return fmt.Errorf("%s: %w", fileName, err)
		}

		fName = filepath.Join(c.Opts.OutDir, filepath.Dir(fileName), fmt.Sprintf("%s.%s", c.baseNoExt(fileName), c.Opts.Format))
	} else {
		fName = filepath.Join(c.Opts.OutDir, fmt.Sprintf("%s.%s", c.baseNoExt(fileName), c.Opts.Format))
	}

	switch c.Opts.Format {
	case "jpeg", "png", "tiff", "webp", "avif":
		if err := c.imageEncode(cover, fName); err != nil {
			return fmt.Errorf("%s: %w", fileName, err)
		}
	case "bmp":
		if err := c.imEncode(cover, fName); err != nil {
			return fmt.Errorf("%s: %w", fileName, err)
		}
	}

	return nil
}

// Thumbnail extracts thumbnail.
func (c *Convertor) Thumbnail(fileName string, fileInfo os.FileInfo) error {
	c.CurrFile++

	cover, err := c.coverImage(fileName, fileInfo)
	if err != nil {
		return fmt.Errorf("%s: %w", fileName, err)
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

	mw := imagick.NewMagickWand()
	defer mw.Destroy()

	rgba := imageToRGBA(cover)
	if err := mw.ConstituteImage(uint(cover.Bounds().Dx()), uint(cover.Bounds().Dy()),
		"RGBA", imagick.PIXEL_CHAR, rgba.Pix); err != nil {
		return fmt.Errorf("%s: %w", fileName, err)
	}

	var fName string
	var fURI string

	if c.Opts.OutFile == "" {
		fURI = "file://" + fileName

		if c.Opts.Recursive {
			err := os.MkdirAll(filepath.Join(c.Opts.OutDir, filepath.Dir(fileName)), 0755)
			if err != nil {
				return fmt.Errorf("%s: %w", fileName, err)
			}

			fName = filepath.Join(c.Opts.OutDir, filepath.Dir(fileName), fmt.Sprintf("%x.png", md5.Sum([]byte(fURI))))
		} else {
			fName = filepath.Join(c.Opts.OutDir, fmt.Sprintf("%x.png", md5.Sum([]byte(fURI))))
		}
	} else {
		abs, _ := filepath.Abs(c.Opts.OutFile)
		fURI = "file://" + abs
		fName = abs
	}

	if err := mw.SetImageFormat("PNG"); err != nil {
		return fmt.Errorf("%s: %w", fileName, err)
	}
	if err := mw.SetImageProperty("Software", "CBconvert"); err != nil {
		return fmt.Errorf("%s: %w", fileName, err)
	}
	if err := mw.SetImageProperty("Description", "Thumbnail of "+fileName); err != nil {
		return fmt.Errorf("%s: %w", fileName, err)
	}
	if err := mw.SetImageProperty("Thumb::URI", fURI); err != nil {
		return fmt.Errorf("%s: %w", fileName, err)
	}
	if err := mw.SetImageProperty("Thumb::MTime", strconv.FormatInt(fileInfo.ModTime().Unix(), 10)); err != nil {
		return fmt.Errorf("%s: %w", fileName, err)
	}
	if err := mw.SetImageProperty("Thumb::Size", strconv.FormatInt(fileInfo.Size(), 10)); err != nil {
		return fmt.Errorf("%s: %w", fileName, err)
	}

	err = mw.WriteImage(fName)
	if err != nil {
		return fmt.Errorf("%s: %w", fileName, err)
	}

	return nil
}

// Meta manipulates with CBZ metadata.
func (c *Convertor) Meta(fileName string) (any, error) {
	c.CurrFile++

	switch {
	case c.Opts.Cover:
		var images []string

		contents, err := c.archiveList(fileName)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", fileName, err)
		}

		for _, ct := range contents {
			if c.isImage(ct) {
				images = append(images, ct)
			}
		}

		return c.coverName(images), nil
	case c.Opts.Comment:
		comment, err := c.archiveComment(fileName)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", fileName, err)
		}

		return comment, nil
	case c.Opts.CommentBody != "":
		err := c.archiveSetComment(fileName, c.Opts.CommentBody)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", fileName, err)
		}
	case c.Opts.FileAdd != "":
		err := c.archiveFileAdd(fileName, c.Opts.FileAdd)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", fileName, err)
		}
	case c.Opts.FileRemove != "":
		err := c.archiveFileRemove(fileName, c.Opts.FileRemove)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", fileName, err)
		}
	}

	return "", nil
}

// Preview returns image preview.
func (c *Convertor) Preview(fileName string, fileInfo os.FileInfo, width, height int) (Image, error) {
	var img Image

	i, err := c.coverImage(fileName, fileInfo)
	if err != nil {
		return img, fmt.Errorf("%s: %w", fileName, err)
	}

	i = c.imageTransform(i)

	if c.Opts.LevelsInMin != 0 || c.Opts.LevelsInMax != 255 || c.Opts.LevelsGamma != 1.0 ||
		c.Opts.LevelsOutMin != 0 || c.Opts.LevelsOutMax != 255 {
		i, err = c.imageLevel(i)
		if err != nil {
			return img, fmt.Errorf("%s: %w", fileName, err)
		}
	}

	tmpName := c.tempName("cbc", "."+c.Opts.Format)

	switch c.Opts.Format {
	case "jpeg", "png", "tiff", "webp", "avif":
		if err := c.imageEncode(i, tmpName); err != nil {
			return img, fmt.Errorf("%s: %w", fileName, err)
		}
	case "bmp":
		if err := c.imEncode(i, tmpName); err != nil {
			return img, fmt.Errorf("%s: %w", fileName, err)
		}
	}

	stat, err := os.Stat(tmpName)
	if err != nil {
		return img, fmt.Errorf("%s: %w", fileName, err)
	}

	img.Width = i.Bounds().Dx()
	img.Height = i.Bounds().Dy()
	img.SizeHuman = humanize.IBytes(uint64(stat.Size()))

	f, err := os.Open(tmpName)
	if err != nil {
		return img, fmt.Errorf("%s: %w", fileName, err)
	}

	defer os.Remove(tmpName)

	dec, err := c.imageDecode(f, tmpName)
	if err != nil {
		e := err
		_, err = f.Seek(0, io.SeekStart)
		if err != nil {
			return img, fmt.Errorf("%s: %w: %w", tmpName, e, err)
		}

		dec, err = c.imDecode(f, tmpName)
		if err != nil {
			return img, fmt.Errorf("%s: %w: %w", tmpName, e, err)
		}
	}

	err = f.Close()
	if err != nil {
		return img, fmt.Errorf("%s: %w", fileName, err)
	}

	if width != 0 && height != 0 {
		dec = imaging.Fit(dec, width, height, filters[c.Opts.Filter])
	}

	img.Image = dec

	return img, nil
}

// Convert converts comic book.
func (c *Convertor) Convert(fileName string, fileInfo os.FileInfo) error {
	c.CurrFile++

	switch {
	case fileInfo.IsDir():
		if err := c.convertDirectory(fileName); err != nil {
			return fmt.Errorf("%s: %w", fileName, err)
		}
	case c.isDocument(fileName):
		if err := c.convertDocument(fileName); err != nil {
			return fmt.Errorf("%s: %w", fileName, err)
		}
	case c.isArchive(fileName):
		if err := c.convertArchive(fileName); err != nil {
			return fmt.Errorf("%s: %w", fileName, err)
		}
	}

	if err := c.archiveSave(fileName); err != nil {
		return fmt.Errorf("%s: %w", fileName, err)
	}

	return nil
}
