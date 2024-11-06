package cbconvert

import (
	"bytes"
	"context"
	"crypto/md5"
	"fmt"
	"image"
	_ "image/gif"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	pngstructure "github.com/dsoprea/go-png-image-structure"
	"github.com/dustin/go-humanize"
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
		fDir := strings.Split(filepath.Dir(fileName), string(os.PathSeparator))[1:]
		err := os.MkdirAll(filepath.Join(c.Opts.OutDir, filepath.Join(fDir...)), 0755)
		if err != nil {
			return fmt.Errorf("%s: %w", fileName, err)
		}

		fName = filepath.Join(c.Opts.OutDir, filepath.Join(fDir...), fmt.Sprintf("%s.%s", baseNoExt(fileName), ext))
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
			fDir := strings.Split(filepath.Dir(fileName), string(os.PathSeparator))[1:]
			err := os.MkdirAll(filepath.Join(c.Opts.OutDir, filepath.Join(fDir...)), 0755)
			if err != nil {
				return fmt.Errorf("%s: %w", fileName, err)
			}

			fName = filepath.Join(c.Opts.OutDir, filepath.Join(fDir...), fmt.Sprintf("%x.png", md5.Sum([]byte(fURI))))
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
