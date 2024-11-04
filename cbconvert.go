package cbconvert

import (
	"bytes"
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/gen2brain/avif"
	"github.com/gen2brain/jpegli"
	"github.com/gen2brain/jpegxl"
	"github.com/gen2brain/webp"
	"github.com/jsummers/gobmp"
	"golang.org/x/image/tiff"

	pngstructure "github.com/dsoprea/go-png-image-structure"
	"github.com/dustin/go-humanize"
	"github.com/fvbommel/sortorder"
	"github.com/gen2brain/go-fitz"
	"github.com/gen2brain/go-unarr"
	"golang.org/x/sync/errgroup"
)

// Options type.
type Options struct {
	// Image format, valid values are jpeg, png, tiff, bmp, webp, avif, jxl
	Format string
	// Archive format, valid values are zip, tar
	Archive string
	// JPEG image quality
	Quality int
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
}

// Converter type.
type Converter struct {
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
	// Cancel function
	OnCancel func()
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

// NewOptions returns default options.
func NewOptions() Options {
	o := Options{}
	o.Format = "jpeg"
	o.Archive = "zip"
	o.Quality = 75
	o.Filter = 2

	return o
}

// New returns new converter.
func New(o Options) *Converter {
	c := &Converter{}
	c.Opts = o

	return c
}

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
		} else if !c.Opts.NoNonImage {
			if err = copyFile(bytes.NewReader(data), filepath.Join(c.Workdir, filepath.Base(pathName))); err != nil {
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

// coverArchive extracts cover from archive.
func (c *Converter) coverArchive(fileName string) (image.Image, error) {
	var images []string

	contents, err := c.archiveList(fileName)
	if err != nil {
		return nil, fmt.Errorf("coverArchive: %w", err)
	}

	for _, ct := range contents {
		if isImage(ct) {
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
	img, err = c.imageDecode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("coverArchive: %w", err)
	}

	return img, nil
}

// coverDocument extracts cover from document.
func (c *Converter) coverDocument(fileName string) (image.Image, error) {
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
func (c *Converter) coverDirectory(dir string) (image.Image, error) {
	contents, err := imagesFromPath(dir)
	if err != nil {
		return nil, fmt.Errorf("coverDirectory: %w", err)
	}

	images := imagesFromSlice(contents)
	cover := c.coverName(images)

	file, err := os.Open(cover)
	if err != nil {
		return nil, fmt.Errorf("coverDirectory: %w", err)
	}
	defer file.Close()

	var img image.Image
	img, err = c.imageDecode(file)
	if err != nil {
		return nil, fmt.Errorf("coverDirectory: %w", err)
	}

	return img, nil
}

// coverName returns the filename that is the most likely to be the cover.
func (c *Converter) coverName(images []string) string {
	if len(images) == 0 {
		return ""
	}

	lower := make([]string, 0)
	for idx, img := range images {
		img = strings.ToLower(img)
		lower = append(lower, img)
		ext := baseNoExt(img)

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
func (c *Converter) coverImage(fileName string, fileInfo os.FileInfo) (image.Image, error) {
	var err error
	var cover image.Image

	switch {
	case fileInfo.IsDir():
		cover, err = c.coverDirectory(fileName)
	case isDocument(fileName):
		cover, err = c.coverDocument(fileName)
	case isArchive(fileName):
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

// Cancel cancels the operation.
func (c *Converter) Cancel() {
	if c.OnCancel != nil {
		c.OnCancel()
	}
}

// Files returns list of found comic files.
func (c *Converter) Files(args []string) ([]File, error) {
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
		if isArchive(fp) || isDocument(fp) {
			if isSize(int64(c.Opts.Size), f.Size()) {
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
				if !fn.IsDir() && isImage(fn.Name()) {
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
			if isArchive(path) || isDocument(path) {
				if isSize(int64(c.Opts.Size), stat.Size()) {
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
					if isArchive(f.Name()) || isDocument(f.Name()) {
						info, err := f.Info()
						if err != nil {
							return files, fmt.Errorf("%s: %w", arg, err)
						}
						if isSize(int64(c.Opts.Size), info.Size()) {
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
func (c *Converter) Cover(fileName string, fileInfo os.FileInfo) error {
	c.CurrFile++

	cover, err := c.coverImage(fileName, fileInfo)
	if err != nil {
		return fmt.Errorf("%s: %w", fileName, err)
	}

	if c.Opts.Width > 0 || c.Opts.Height > 0 {
		if c.Opts.Fit {
			cover = fit(cover, c.Opts.Width, c.Opts.Height, filters[c.Opts.Filter])
		} else {
			cover = resize(cover, c.Opts.Width, c.Opts.Height, filters[c.Opts.Filter])
		}
	}

	ext := c.Opts.Format
	if ext == "jpeg" {
		ext = "jpg"
	}

	var fName string
	if c.Opts.Recursive {
		err := os.MkdirAll(filepath.Join(c.Opts.OutDir, filepath.Dir(fileName)), 0755)
		if err != nil {
			return fmt.Errorf("%s: %w", fileName, err)
		}

		fName = filepath.Join(c.Opts.OutDir, filepath.Dir(fileName), fmt.Sprintf("%s.%s", baseNoExt(fileName), ext))
	} else {
		fName = filepath.Join(c.Opts.OutDir, fmt.Sprintf("%s.%s", baseNoExt(fileName), ext))
	}

	w, err := os.Create(fName)
	if err != nil {
		return fmt.Errorf("imageConvert: %w", err)
	}
	defer w.Close()

	if err := c.imageEncode(cover, w); err != nil {
		return fmt.Errorf("%s: %w", fileName, err)
	}

	return nil
}

// Thumbnail extracts thumbnail.
func (c *Converter) Thumbnail(fileName string, fileInfo os.FileInfo) error {
	c.CurrFile++

	cover, err := c.coverImage(fileName, fileInfo)
	if err != nil {
		return fmt.Errorf("%s: %w", fileName, err)
	}

	if c.Opts.Width > 0 || c.Opts.Height > 0 {
		if c.Opts.Fit {
			cover = fit(cover, c.Opts.Width, c.Opts.Height, filters[c.Opts.Filter])
		} else {
			cover = resize(cover, c.Opts.Width, c.Opts.Height, filters[c.Opts.Filter])
		}
	} else {
		cover = resize(cover, 256, 0, filters[c.Opts.Filter])
	}

	var buf bytes.Buffer
	err = png.Encode(&buf, cover)
	if err != nil {
		return fmt.Errorf("%s: %w", fileName, err)
	}

	pmp := pngstructure.NewPngMediaParser()
	csTmp, err := pmp.ParseBytes(buf.Bytes())
	if err != nil {
		return fmt.Errorf("%s: %w", fileName, err)
	}

	cs, ok := csTmp.(*pngstructure.ChunkSlice)
	if !ok {
		return fmt.Errorf("%s: type is not ChunkSlice", fileName)
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

	chunks := cs.Chunks()
	textChunks := []*pngstructure.Chunk{
		{Type: `tEXt`, Data: []uint8("Software\x00" + "CBconvert")},
		{Type: `tEXt`, Data: []uint8("Description\x00" + "Thumbnail of " + fileName)},
		{Type: `tEXt`, Data: []uint8("Thumb::URI\x00" + fURI)},
		{Type: `tEXt`, Data: []uint8("Thumb::MTime\x00" + strconv.FormatInt(fileInfo.ModTime().Unix(), 10))},
		{Type: `tEXt`, Data: []uint8("Thumb::Size\x00" + strconv.FormatInt(fileInfo.Size(), 10))},
	}

	for _, textChunk := range textChunks {
		textChunk.Length = uint32(len(textChunk.Data))
		textChunk.UpdateCrc32()
	}

	chunks = append(
		chunks[:1],
		append(
			textChunks,
			chunks[1:]...,
		)...,
	)

	cs = pngstructure.NewChunkSlice(chunks)
	err = cs.WriteTo(&buf)
	if err != nil {
		return fmt.Errorf("%s: %w", fileName, err)
	}

	f, err := os.Create(fName)
	if err != nil {
		return fmt.Errorf("%s: %w", fileName, err)
	}

	defer f.Close()

	_, err = buf.WriteTo(f)
	if err != nil {
		return fmt.Errorf("%s: %w", fileName, err)
	}

	return nil
}

// Meta manipulates with CBZ metadata.
func (c *Converter) Meta(fileName string) (any, error) {
	c.CurrFile++

	switch {
	case c.Opts.Cover:
		var images []string

		contents, err := c.archiveList(fileName)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", fileName, err)
		}

		for _, ct := range contents {
			if isImage(ct) {
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
func (c *Converter) Preview(fileName string, fileInfo os.FileInfo, width, height int) (Image, error) {
	var img Image

	i, err := c.coverImage(fileName, fileInfo)
	if err != nil {
		return img, fmt.Errorf("%s: %w", fileName, err)
	}

	i = c.imageTransform(i)

	var w bytes.Buffer

	if err := c.imageEncode(i, &w); err != nil {
		return img, fmt.Errorf("%s: %w", fileName, err)
	}

	img.Width = i.Bounds().Dx()
	img.Height = i.Bounds().Dy()
	img.SizeHuman = humanize.IBytes(uint64(len(w.Bytes())))

	r := bytes.NewReader(w.Bytes())

	dec, err := c.imageDecode(r)
	if err != nil {
		return img, fmt.Errorf("%s: %w", fileName, err)
	}

	if width != 0 && height != 0 {
		dec = fit(dec, width, height, filters[c.Opts.Filter])
	}

	img.Image = dec

	return img, nil
}

// Convert converts comic book.
func (c *Converter) Convert(fileName string, fileInfo os.FileInfo) error {
	c.CurrFile++

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c.OnCancel = cancel

	switch {
	case fileInfo.IsDir():
		if err := c.convertDirectory(ctx, fileName); err != nil {
			return fmt.Errorf("%s: %w", fileName, err)
		}
	case isDocument(fileName):
		if err := c.convertDocument(ctx, fileName); err != nil {
			return fmt.Errorf("%s: %w", fileName, err)
		}
	case isArchive(fileName):
		if err := c.convertArchive(ctx, fileName); err != nil {
			return fmt.Errorf("%s: %w", fileName, err)
		}
	}

	if err := c.archiveSave(fileName); err != nil {
		return fmt.Errorf("%s: %w", fileName, err)
	}

	c.OnCancel = nil

	return nil
}
