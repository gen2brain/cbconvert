package cbconvert

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"

	"github.com/gen2brain/avif"
	"github.com/gen2brain/go-fitz"
	"github.com/gen2brain/go-unarr"
	"github.com/gen2brain/jpegli"
	"github.com/gen2brain/jpegxl"
	"github.com/gen2brain/webp"
	"github.com/jsummers/gobmp"
	"golang.org/x/image/tiff"
	"golang.org/x/sync/errgroup"
)

// convertDocument converts PDF/EPUB document to CBZ.
func (c *Converter) convertDocument(ctx context.Context, fileName string) error {
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

	eg, ctx := errgroup.WithContext(ctx)
	eg.SetLimit(runtime.NumCPU() + 1)

	for n := 0; n < c.Ncontents; n++ {
		if ctx.Err() != nil {
			return fmt.Errorf("convertDocument: %w", ctx.Err())
		}

		img, err := doc.Image(n)
		if err != nil {
			return fmt.Errorf("convertDocument: %w", err)
		}

		if img != nil {
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
func (c *Converter) convertArchive(ctx context.Context, fileName string) error {
	var err error

	c.Workdir, err = os.MkdirTemp(os.TempDir(), "cbc")
	if err != nil {
		return fmt.Errorf("convertArchive: %w", err)
	}

	contents, err := c.archiveList(fileName)
	if err != nil {
		return fmt.Errorf("convertArchive: %w", err)
	}

	images := imagesFromSlice(contents)

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

	eg, ctx := errgroup.WithContext(ctx)
	eg.SetLimit(runtime.NumCPU() + 1)

	for {
		if ctx.Err() != nil {
			return fmt.Errorf("convertArchive: %w", ctx.Err())
		}

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

		if isImage(pathName) {
			if c.Opts.NoConvert {
				if err = copyFile(bytes.NewReader(data), filepath.Join(c.Workdir, filepath.Base(pathName))); err != nil {
					return fmt.Errorf("convertArchive: %w", err)
				}

				continue
			}

			if cover == pathName && c.Opts.NoCover {
				if err = copyFile(bytes.NewReader(data), filepath.Join(c.Workdir, filepath.Base(pathName))); err != nil {
					return fmt.Errorf("convertArchive: %w", err)
				}

				continue
			}

			var img image.Image
			img, err = c.imageDecode(bytes.NewReader(data))
			if err != nil {
				return fmt.Errorf("convertArchive: %w", err)
			}

			if c.Opts.NoRGB && !isGrayScale(img) {
				if err = copyFile(bytes.NewReader(data), filepath.Join(c.Workdir, filepath.Base(pathName))); err != nil {
					return fmt.Errorf("convertArchive: %w", err)
				}

				continue
			}

			if img != nil {
				eg.Go(func() error {
					return c.imageConvert(ctx, img, 0, pathName)
				})
			}
		} else {
			if filepath.Ext(pathName) == ".DS_Store" || strings.Contains(pathName, "__MACOSX") {
				continue
			}

			if !c.Opts.NoNonImage {
				if err = copyFile(bytes.NewReader(data), filepath.Join(c.Workdir, filepath.Base(pathName))); err != nil {
					return fmt.Errorf("convertArchive: %w", err)
				}
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
func (c *Converter) convertDirectory(ctx context.Context, dirPath string) error {
	var err error

	c.Workdir, err = os.MkdirTemp(os.TempDir(), "cbc")
	if err != nil {
		return fmt.Errorf("convertDirectory: %w", err)
	}

	contents, err := imagesFromPath(dirPath)
	if err != nil {
		return fmt.Errorf("convertDirectory: %w", err)
	}

	images := imagesFromSlice(contents)
	c.Ncontents = len(images)
	c.CurrContent = 0

	if c.OnStart != nil {
		c.OnStart()
	}

	eg, ctx := errgroup.WithContext(ctx)
	eg.SetLimit(runtime.NumCPU() + 1)

	for index, img := range contents {
		if ctx.Err() != nil {
			return fmt.Errorf("convertDirectory: %w", ctx.Err())
		}

		file, err := os.Open(img)
		if err != nil {
			return fmt.Errorf("convertDirectory: %w", err)
		}

		if isNonImage(img) && !c.Opts.NoNonImage {
			if err = copyFile(file, filepath.Join(c.Workdir, filepath.Base(img))); err != nil {
				return fmt.Errorf("convertDirectory: %w", err)
			}

			if err = file.Close(); err != nil {
				return fmt.Errorf("convertDirectory: %w", err)
			}

			continue
		} else if isImage(img) {
			if c.Opts.NoConvert {
				if err = copyFile(file, filepath.Join(c.Workdir, filepath.Base(img))); err != nil {
					return fmt.Errorf("convertDirectory: %w", err)
				}

				if err = file.Close(); err != nil {
					return fmt.Errorf("convertDirectory: %w", err)
				}

				continue
			}

			var i image.Image
			i, err = c.imageDecode(file)
			if err != nil {
				return fmt.Errorf("convertDirectory: %w", err)
			}

			if c.Opts.NoRGB && !isGrayScale(i) {
				if err = copyFile(file, filepath.Join(c.Workdir, filepath.Base(img))); err != nil {
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
func (c *Converter) imageConvert(ctx context.Context, img image.Image, index int, pathName string) error {
	err := ctx.Err()
	if err != nil {
		return fmt.Errorf("imageConvert: %w", err)
	}

	atomic.AddInt32(&c.CurrContent, 1)
	if c.OnProgress != nil {
		c.OnProgress()
	}

	ext := c.Opts.Format
	if ext == "jpeg" {
		ext = "jpg"
	}

	var fileName string
	if pathName != "" {
		fileName = filepath.Join(c.Workdir, fmt.Sprintf("%s.%s", baseNoExt(pathName), ext))
	} else {
		fileName = filepath.Join(c.Workdir, fmt.Sprintf("%03d.%s", index, ext))
	}

	img = c.imageTransform(img)

	w, err := os.Create(fileName)
	if err != nil {
		return fmt.Errorf("imageConvert: %w", err)
	}
	defer w.Close()

	if err := c.imageEncode(img, w); err != nil {
		return fmt.Errorf("imageConvert: %w", err)
	}

	return nil
}

// imageTransform transforms image (resize, rotate, brightness, contrast).
func (c *Converter) imageTransform(img image.Image) image.Image {
	var i = img

	if c.Opts.Width > 0 || c.Opts.Height > 0 {
		if c.Opts.Fit {
			i = fit(i, c.Opts.Width, c.Opts.Height, filters[c.Opts.Filter])
		} else {
			i = resize(i, c.Opts.Width, c.Opts.Height, filters[c.Opts.Filter])
		}
	}

	if c.Opts.Rotate > 0 {
		switch c.Opts.Rotate {
		case 90:
			i = rotate(i, 90)
		case 180:
			i = rotate(i, 180)
		case 270:
			i = rotate(i, 270)
		}
	}

	if c.Opts.Brightness != 0 {
		i = brightness(i, float64(c.Opts.Brightness))
	}

	if c.Opts.Contrast != 0 {
		i = contrast(i, float64(c.Opts.Contrast))
	}

	if c.Opts.Grayscale {
		i = imageToGray(i)
	}

	return i
}

// imageDecode decodes image from reader.
func (c *Converter) imageDecode(reader io.Reader) (image.Image, error) {
	img, _, err := image.Decode(reader)
	if err != nil {
		return img, fmt.Errorf("imageDecode: %w", err)
	}

	return img, nil
}

// imageEncode encodes image to file.
func (c *Converter) imageEncode(img image.Image, w io.Writer) error {
	var err error

	switch c.Opts.Format {
	case "png":
		err = png.Encode(w, img)
	case "tiff":
		err = tiff.Encode(w, img, &tiff.Options{Compression: tiff.Uncompressed})
	case "jpeg":
		opts := &jpegli.EncodingOptions{}
		opts.Quality = c.Opts.Quality
		opts.ChromaSubsampling = image.YCbCrSubsampleRatio420
		opts.ProgressiveLevel = 2
		opts.AdaptiveQuantization = true
		opts.DCTMethod = jpegli.DefaultDCTMethod
		err = jpegli.Encode(w, img, opts)
	case "webp":
		err = webp.Encode(w, img, webp.Options{Quality: c.Opts.Quality, Method: webp.DefaultMethod})
	case "avif":
		err = avif.Encode(w, img, avif.Options{Quality: c.Opts.Quality, Speed: avif.DefaultSpeed})
	case "jxl":
		err = jpegxl.Encode(w, img, jpegxl.Options{Quality: c.Opts.Quality, Effort: jpegxl.DefaultEffort})
	case "bmp":
		opts := &gobmp.EncoderOptions{}
		opts.SupportTransparency(false)
		err = gobmp.EncodeWithOptions(w, imageToPaletted(img), opts)
	}

	if err != nil {
		return fmt.Errorf("imageEncode: %w", err)
	}

	return nil
}
