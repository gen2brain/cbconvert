// Author: Milan Nikolic <gen2brain@gmail.com>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package cbconvert

import (
	"archive/zip"
	"bytes"
	"crypto/md5"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"io/ioutil"
	"mime"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/cheggaaa/pb"
	"github.com/disintegration/imaging"
	"github.com/gen2brain/go-fitz"
	"github.com/gen2brain/go-unarr"
	"github.com/gographics/imagick/imagick"
	_ "github.com/hotei/bmp"
	"github.com/skarademir/naturalsort"
	"golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

// Resample filters
const (
	NearestNeighbor   int = iota // Fastest resampling filter, no antialiasing
	Box                          // Box filter (averaging pixels)
	Linear                       // Bilinear filter, smooth and reasonably fast
	MitchellNetravali            // А smooth bicubic filter
	CatmullRom                   // A sharp bicubic filter
	Gaussian                     // Blurring filter that uses gaussian function, useful for noise removal
	Lanczos                      // High-quality resampling filter, it's slower than cubic filters
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

var (
	bar *pb.ProgressBar
	wg  sync.WaitGroup
)

// Limits go routines to number of CPUs + 1
var throttle = make(chan int, runtime.NumCPU()+1)

// Options
type Options struct {
	Format       string  // Image format, valid values are jpeg, png, gif, tiff, bmp
	Quality      int     // JPEG image quality
	Width        int     // image width
	Height       int     // image height
	Fit          bool    // Best fit for required width and height
	Filter       int     // 0=NearestNeighbor, 1=Box, 2=Linear, 3=MitchellNetravali, 4=CatmullRom, 6=Gaussian, 7=Lanczos
	ConvertCover bool    // convert cover image
	RGB          bool    // convert images that have RGB colorspace
	NonImage     bool    // Leave non image files in archive
	Suffix       string  // add suffix to file basename
	Cover        bool    // extract cover
	Thumbnail    bool    // extract cover thumbnail (freedesktop spec.)
	Outdir       string  // output directory
	Grayscale    bool    // convert images to grayscale (monochromatic)
	Rotate       int     // Rotate images, valid values are 0, 90, 180, 270
	Flip         string  // Flip images, valid values are none, horizontal, vertical
	Brightness   float64 // Adjust brightness of the images, must be in range (-100, 100)
	Contrast     float64 // Adjust contrast of the images, must be in range (-100, 100)
	Recursive    bool    // process subdirectories recursively
	Size         int64   // process only files larger then size (in MB)
	Quiet        bool    // hide console output
	LevelsInMin  float64 // shadow input value
	LevelsInMax  float64 // highlight input value
	LevelsGamma  float64 // midpoint/gamma
	LevelsOutMin float64 // shadow output value
	LevelsOutMax float64 // highlight output value
}

// Convertor struct
type Convertor struct {
	Opts        Options // Options struct
	Workdir     string  // Current working directory
	Nfiles      int     // Number of files
	CurrFile    int     // Index of current file
	Ncontents   int     // Number of contents in archive/document
	CurrContent int     // Index of current content
}

// NewConvertor returns new convertor
func NewConvertor(o Options) *Convertor {
	c := &Convertor{}
	c.Opts = o
	return c
}

// Converts image
func (c *Convertor) convertImage(img image.Image, index int, pathName string) {
	defer wg.Done()

	var ext string
	switch c.Opts.Format {
	case "jpeg":
		ext = "jpg"
	case "png":
		ext = "png"
	case "gif":
		ext = "gif"
	case "tiff":
		ext = "tiff"
	case "bmp":
		ext = "bmp"

	}

	var filename string
	if pathName != "" {
		filename = filepath.Join(c.Workdir, fmt.Sprintf("%s.%s", c.getBasename(pathName), ext))
	} else {
		filename = filepath.Join(c.Workdir, fmt.Sprintf("%03d.%s", index, ext))
	}

	img = c.TransformImage(img)

	if c.Opts.LevelsInMin != 0 || c.Opts.LevelsInMax != 255 || c.Opts.LevelsGamma != 1.00 ||
		c.Opts.LevelsOutMin != 0 || c.Opts.LevelsOutMax != 255 {
		img = c.LevelImage(img)
	}

	switch c.Opts.Format {
	case "jpeg":
		// convert image to JPEG (default)
		if c.Opts.Grayscale {
			c.encodeImageMagick(img, filename)
		} else {
			c.encodeImage(img, filename)
		}
	case "png":
		// convert image to PNG
		if c.Opts.Grayscale {
			c.encodeImageMagick(img, filename)
		} else {
			c.encodeImage(img, filename)
		}
	case "gif":
		// convert image to GIF
		c.encodeImageMagick(img, filename)
	case "tiff":
		// convert image to TIFF
		if c.Opts.Grayscale {
			c.encodeImageMagick(img, filename)
		} else {
			c.encodeImage(img, filename)
		}
	case "bmp":
		// convert image to 4-Bit BMP (16 colors)
		c.encodeImageMagick(img, filename)
	}

	<-throttle
}

// Transforms image (resize, rotate, flip, brightness, contrast)
func (c *Convertor) TransformImage(img image.Image) image.Image {
	var i image.Image = img

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

// Applies a Photoshop-like levels operation on an image
func (c *Convertor) LevelImage(img image.Image) image.Image {
	imagick.Initialize()

	mw := imagick.NewMagickWand()
	defer mw.Destroy()

	err := mw.ReadImageBlob(c.GetImageBytes(img))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error ReadImageBlob: %v\n", err.Error())
		return img
	}

	_, qrange := imagick.GetQuantumRange()
	quantumRange := float64(qrange)

	inmin := (quantumRange * c.Opts.LevelsInMin) / 255
	inmax := (quantumRange * c.Opts.LevelsInMax) / 255
	outmin := (quantumRange * c.Opts.LevelsOutMin) / 255
	outmax := (quantumRange * c.Opts.LevelsOutMax) / 255

	err = mw.LevelImage(inmin, c.Opts.LevelsGamma, inmax)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error LevelImageChannel Input: %v\n", err.Error())
		return img
	}

	err = mw.LevelImage(-outmin, 1.0, quantumRange+(quantumRange-outmax))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error LevelImageChannel Output: %v\n", err.Error())
		return img
	}

	blob := mw.GetImageBlob()
	i, err := c.decodeImage(bytes.NewReader(blob), "levels")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error decodeImage: %v\n", err.Error())
		return img
	}

	return i
}

// Converts PDF/EPUB/XPS document to CBZ
func (c *Convertor) convertDocument(file string) {
	c.Workdir, _ = ioutil.TempDir(os.TempDir(), "cbc")

	doc, err := fitz.NewDocument(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Skipping %s, error: %v", file, err.Error())
		return
	}

	c.Ncontents = doc.Pages()
	c.CurrContent = 0

	if !c.Opts.Quiet {
		bar = pb.New(c.Ncontents)
		bar.ShowTimeLeft = false
		bar.Prefix(fmt.Sprintf("Converting %d of %d: ", c.CurrFile, c.Nfiles))
		bar.Start()
	}

	for n := 0; n < c.Ncontents; n++ {
		c.CurrContent++
		if !c.Opts.Quiet {
			bar.Increment()
		}

		img, err := doc.Image(n)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error Image: %v\n", err.Error())
		}

		if img != nil {
			throttle <- 1
			wg.Add(1)

			go c.convertImage(img, n, "")
		}
	}
	wg.Wait()
}

// Converts archive to CBZ
func (c *Convertor) convertArchive(file string) {
	c.Workdir, _ = ioutil.TempDir(os.TempDir(), "cbc")

	contents := c.listArchive(file)
	c.Ncontents = len(contents)
	c.CurrContent = 0

	cover := c.getCover(c.getImagesFromSlice(contents))

	archive, err := unarr.NewArchive(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error NewReader: %v\n", err.Error())
		return
	}
	defer archive.Close()

	if !c.Opts.Quiet {
		bar = pb.New(c.Ncontents)
		bar.ShowTimeLeft = false
		bar.Prefix(fmt.Sprintf("Converting %d of %d: ", c.CurrFile, c.Nfiles))
		bar.Start()
	}

	for {
		err := archive.Entry()
		if err != nil {
			if err == io.EOF {
				break
			} else {
				fmt.Fprintf(os.Stderr, "Error Entry: %v\n", err.Error())
				continue
			}
		}

		c.CurrContent++
		if !c.Opts.Quiet {
			bar.Increment()
		}

		size := archive.Size()
		pathname := archive.Name()

		buf := make([]byte, size)
		for size > 0 {
			n, err := archive.Read(buf)
			if err != nil && err != io.EOF {
				break
			}
			size -= n
		}

		if size > 0 {
			fmt.Printf("Error Read\n")
			continue
		}

		if c.isImage(pathname) {
			img, err := c.decodeImage(bytes.NewReader(buf), pathname)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error Decode: %v\n", err.Error())
				continue
			}

			if !c.Opts.ConvertCover {
				if cover == pathname {
					img = c.TransformImage(img)
					c.encodeImage(img, filepath.Join(c.Workdir, filepath.Base(pathname)))
					continue
				}
			}

			if !c.Opts.RGB && !c.isGrayScale(img) {
				img = c.TransformImage(img)
				c.encodeImage(img, filepath.Join(c.Workdir, filepath.Base(pathname)))
				continue
			}

			if img != nil {
				throttle <- 1
				wg.Add(1)
				go c.convertImage(img, 0, pathname)
			}
		} else {
			if c.Opts.NonImage {
				c.copyFile(bytes.NewReader(buf), filepath.Join(c.Workdir, filepath.Base(pathname)))
			}
		}
	}
	wg.Wait()
}

// Converts directory to CBZ
func (c *Convertor) convertDirectory(path string) {
	c.Workdir, _ = ioutil.TempDir(os.TempDir(), "cbc")

	images := c.getImagesFromPath(path)
	c.Ncontents = len(images)
	c.CurrContent = 0

	if !c.Opts.Quiet {
		bar = pb.New(c.Ncontents)
		bar.ShowTimeLeft = false
		bar.Prefix(fmt.Sprintf("Converting %d of %d: ", c.CurrFile, c.Nfiles))
		bar.Start()
	}

	for index, img := range images {
		c.CurrContent++
		if !c.Opts.Quiet {
			bar.Increment()
		}

		f, err := os.Open(img)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error Open: %v\n", err.Error())
			continue
		}

		i, err := c.decodeImage(f, img)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error Decode: %v\n", err.Error())
			continue
		}

		if !c.Opts.RGB && !c.isGrayScale(i) {
			i = c.TransformImage(i)
			c.encodeImage(i, filepath.Join(c.Workdir, filepath.Base(img)))
			continue
		}

		f.Close()

		if i != nil {
			throttle <- 1
			wg.Add(1)
			go c.convertImage(i, index, img)
		}
	}
	wg.Wait()
}

// Saves workdir to CBZ archive
func (c *Convertor) saveArchive(file string) {
	defer os.RemoveAll(c.Workdir)

	zipname := filepath.Join(c.Opts.Outdir, fmt.Sprintf("%s%s.cbz", c.getBasename(file), c.Opts.Suffix))
	zipfile, err := os.Create(zipname)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error Create: %v\n", err.Error())
		return
	}
	defer zipfile.Close()

	z := zip.NewWriter(zipfile)
	files, _ := ioutil.ReadDir(c.Workdir)

	ncontents := len(files)

	if !c.Opts.Quiet {
		bar = pb.New(ncontents)
		bar.ShowTimeLeft = false
		bar.Prefix(fmt.Sprintf("Compressing %d of %d: ", c.CurrFile, c.Nfiles))
		bar.Start()
	}

	for _, file := range files {
		if !c.Opts.Quiet {
			bar.Increment()
		}

		r, err := ioutil.ReadFile(filepath.Join(c.Workdir, file.Name()))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error ReadFile: %v\n", err.Error())
			continue
		}

		w, err := z.Create(file.Name())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error Create: %v\n", err.Error())
			continue
		}
		w.Write(r)
	}
	z.Close()
}

// Decodes image from reader
func (c *Convertor) decodeImage(reader io.Reader, filename string) (i image.Image, err error) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "Recovered in decodeImage %s: %v\n", filename, r)
		}
	}()

	i, _, err = image.Decode(reader)
	return i, err
}

// Encode image to file
func (c *Convertor) encodeImage(i image.Image, filename string) (err error) {
	f, err := os.Create(filename)
	if err != nil {
		return
	}

	switch filepath.Ext(filename) {
	case ".png":
		err = png.Encode(f, i)
	case ".tif":
	case ".tiff":
		err = tiff.Encode(f, i, &tiff.Options{tiff.Uncompressed, false})
	case ".gif":
		err = gif.Encode(f, i, nil)
	default:
		err = jpeg.Encode(f, i, &jpeg.Options{c.Opts.Quality})
	}

	f.Close()
	return
}

// Encode image to file (ImageMagick)
func (c *Convertor) encodeImageMagick(i image.Image, filename string) (err error) {
	imagick.Initialize()

	mw := imagick.NewMagickWand()
	defer mw.Destroy()

	err = mw.ReadImageBlob(c.GetImageBytes(i))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error ReadImageBlob: %v\n", err.Error())
		return
	}

	if c.Opts.Grayscale {
		c := mw.GetImageColors()
		mw.QuantizeImage(c, imagick.COLORSPACE_GRAY, 8, true, true)
	}

	switch filepath.Ext(filename) {
	case ".png":
		mw.SetImageFormat("PNG")
		mw.WriteImage(filename)
	case ".tif":
	case ".tiff":
		mw.SetImageFormat("TIFF")
		mw.WriteImage(filename)
	case ".gif":
		mw.SetImageFormat("GIF")
		mw.WriteImage(filename)
	case ".bmp":
		w := imagick.NewPixelWand()
		w.SetColor("black")
		defer w.Destroy()

		cs := mw.GetImageColorspace()
		if c.Opts.Grayscale {
			cs = imagick.COLORSPACE_GRAY
		}

		mw.SetImageFormat("BMP3")
		mw.SetImageBackgroundColor(w)
		mw.SetImageAlphaChannel(imagick.ALPHA_CHANNEL_REMOVE)
		mw.SetImageAlphaChannel(imagick.ALPHA_CHANNEL_DEACTIVATE)
		mw.SetImageMatte(false)
		mw.SetImageCompression(imagick.COMPRESSION_NO)
		mw.QuantizeImage(16, cs, 8, true, true)
		mw.WriteImage(filename)
	default:
		mw.SetImageFormat("JPEG")
		mw.WriteImage(filename)
	}

	return
}

// Lists contents of archive
func (c *Convertor) listArchive(file string) []string {
	var contents []string
	archive, err := unarr.NewArchive(file)
	if err != nil {
		return contents
	}
	defer archive.Close()

	for {
		err := archive.Entry()
		if err != nil {
			if err == io.EOF {
				break
			} else {
				continue
			}
		}

		pathname := archive.Name()
		contents = append(contents, pathname)
	}

	return contents
}

// Extracts cover from archive
func (c *Convertor) coverArchive(file string) (image.Image, error) {
	var images []string

	contents := c.listArchive(file)
	for _, ct := range contents {
		if c.isImage(ct) {
			images = append(images, ct)
		}
	}

	cover := c.getCover(images)

	archive, err := unarr.NewArchive(file)
	if err != nil {
		return nil, err
	}
	defer archive.Close()

	err = archive.EntryFor(cover)
	if err != nil {
		return nil, err
	}

	size := archive.Size()
	buf := make([]byte, size)
	for size > 0 {
		n, err := archive.Read(buf)
		if err != nil && err != io.EOF {
			break
		}
		size -= n
	}

	if size > 0 {
		return nil, errors.New("Error Read")
	}

	img, err := c.decodeImage(bytes.NewReader(buf), cover)
	if err != nil {
		return nil, err
	}

	return img, nil
}

// Extracts cover from document
func (c *Convertor) coverDocument(file string) (image.Image, error) {
	doc, err := fitz.NewDocument(file)
	if err != nil {
		return nil, err
	}

	img, err := doc.Image(0)
	if err != nil {
		return nil, err
	}

	if img == nil {
		return nil, errors.New("Image is nil")
	}

	return img, nil
}

// Extracts cover from directory
func (c *Convertor) coverDirectory(dir string) (image.Image, error) {
	images := c.getImagesFromPath(dir)
	cover := c.getCover(images)

	p, err := os.Open(cover)
	if err != nil {
		return nil, err
	}
	defer p.Close()

	img, err := c.decodeImage(p, cover)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error Decode: %v\n", err.Error())
		return nil, err
	}

	if img == nil {
		return nil, errors.New("Image is nil")
	}

	return img, nil
}

// Returns list of found comic files
func (c *Convertor) GetFiles(args []string) []string {
	var files []string

	walkFiles := func(fp string, f os.FileInfo, err error) error {
		if !f.IsDir() {
			if c.isArchive(fp) || c.isDocument(fp) {
				if c.isSize(f.Size()) {
					files = append(files, fp)
				}
			}
		}
		return nil
	}

	for _, arg := range args {
		path, _ := filepath.Abs(arg)
		stat, err := os.Stat(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error Stat GetFiles: %v\n", err.Error())
			continue
		}

		if !stat.IsDir() {
			if c.isArchive(path) || c.isDocument(path) {
				if c.isSize(stat.Size()) {
					files = append(files, path)
				}
			}
		} else {
			if c.Opts.Recursive {
				filepath.Walk(path, walkFiles)
			} else {
				fs, _ := ioutil.ReadDir(path)
				for _, f := range fs {
					if c.isArchive(f.Name()) || c.isDocument(f.Name()) {
						if c.isSize(f.Size()) {
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
	return files
}

// Returns list of found image files for given directory
func (c *Convertor) getImagesFromPath(path string) []string {
	var images []string

	walkFiles := func(fp string, f os.FileInfo, err error) error {
		if !f.IsDir() && f.Mode()&os.ModeType == 0 {
			if f.Size() > 0 && c.isImage(fp) {
				images = append(images, fp)
			}
		}
		return nil
	}

	f, _ := filepath.Abs(path)
	stat, err := os.Stat(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error Stat getImagesFromPath: %v\n", err.Error())
		return images
	}

	if !stat.IsDir() && stat.Mode()&os.ModeType == 0 {
		if c.isImage(f) {
			images = append(images, f)
		}
	} else {
		filepath.Walk(f, walkFiles)
	}

	return images
}

// Returns list of found image files for given slice of files
func (c *Convertor) getImagesFromSlice(files []string) []string {
	var images []string

	for _, f := range files {
		if c.isImage(f) {
			images = append(images, f)
		}
	}

	return images
}

// Returns image bytes/blob to be used with ImageMagick
func (c *Convertor) GetImageBytes(i image.Image) []byte {
	b := new(bytes.Buffer)
	jpeg.Encode(b, i, &jpeg.Options{c.Opts.Quality})
	return b.Bytes()
}

// Returns the filename that is the most likely to be the cover
func (c *Convertor) getCover(images []string) string {
	if len(images) == 0 {
		return ""
	}

	for _, i := range images {
		e := c.getBasename(i)
		if strings.HasPrefix(i, "cover") || strings.HasPrefix(i, "front") ||
			strings.HasSuffix(e, "cover") || strings.HasSuffix(e, "front") {
			return i
		}
	}

	sort.Sort(naturalsort.NaturalSort(images))
	return images[0]
}

// Checks if file is archive
func (c *Convertor) isArchive(f string) bool {
	var types = []string{".rar", ".zip", ".7z", ".gz",
		".bz2", ".cbr", ".cbz", ".cb7", ".cbt"}
	for _, t := range types {
		if strings.ToLower(filepath.Ext(f)) == t {
			return true
		}
	}
	return false
}

// Checks if file is document
func (c *Convertor) isDocument(f string) bool {
	var types = []string{".pdf", ".epub", ".xps"}
	for _, t := range types {
		if strings.ToLower(filepath.Ext(f)) == t {
			return true
		}
	}
	return false
}

// Checks if file is image
func (c *Convertor) isImage(f string) bool {
	var types = []string{".jpg", ".jpeg", ".jpe", ".png",
		".gif", ".bmp", ".tiff", ".tif", ".webp"}
	for _, t := range types {
		if strings.ToLower(filepath.Ext(f)) == t {
			return true
		}
	}
	return false
}

// Checks size of file
func (c *Convertor) isSize(size int64) bool {
	if c.Opts.Size > 0 {
		if size < c.Opts.Size*(1024*1024) {
			return false
		}
	}
	return true
}

// Checks if image is grayscale
func (c *Convertor) isGrayScale(img image.Image) bool {
	model := img.ColorModel()
	if model == color.GrayModel || model == color.Gray16Model {
		return true
	}
	return false
}

// Copies reader to file
func (c *Convertor) copyFile(reader io.Reader, filename string) error {
	os.MkdirAll(filepath.Dir(filename), 0755)

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, reader)
	if err != nil {
		return err
	}

	return nil
}

// Returns basename without extension
func (c *Convertor) getBasename(file string) string {
	basename := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
	basename = strings.TrimSuffix(basename, ".tar")
	return basename
}

// Returns cover image.Image
func (c *Convertor) GetCoverImage(file string, info os.FileInfo) (image.Image, error) {
	var err error
	var cover image.Image

	if info.IsDir() {
		cover, err = c.coverDirectory(file)
	} else if c.isDocument(file) {
		cover, err = c.coverDocument(file)
	} else {
		cover, err = c.coverArchive(file)
	}

	if err != nil {
		return nil, err
	}

	return cover, nil
}

// Extracts cover
func (c *Convertor) ExtractCover(file string, info os.FileInfo) {
	c.CurrFile += 1

	cover, err := c.GetCoverImage(file, info)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error GetCoverImage: %v\n", err.Error())
		return
	}

	if c.Opts.Width > 0 || c.Opts.Height > 0 {
		if c.Opts.Fit {
			cover = imaging.Fit(cover, c.Opts.Width, c.Opts.Height, filters[c.Opts.Filter])
		} else {
			cover = imaging.Resize(cover, c.Opts.Width, c.Opts.Height, filters[c.Opts.Filter])
		}
	}

	filename := filepath.Join(c.Opts.Outdir, fmt.Sprintf("%s.jpg", c.getBasename(file)))
	f, err := os.Create(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error Create: %v\n", err.Error())
		return
	}
	defer f.Close()

	jpeg.Encode(f, cover, &jpeg.Options{c.Opts.Quality})
}

// Extracts thumbnail
func (c *Convertor) ExtractThumbnail(file string, info os.FileInfo) {
	c.CurrFile += 1

	cover, err := c.GetCoverImage(file, info)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error GetCoverImage: %v\n", err.Error())
		return
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error Thumbnail: %v\n", err.Error())
		return
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

	b := new(bytes.Buffer)
	png.Encode(b, cover)

	err = mw.ReadImageBlob(b.Bytes())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error ReadImageBlob: %v\n", err.Error())
	}

	fileuri := "file://" + file
	filename := filepath.Join(c.Opts.Outdir, fmt.Sprintf("%x.png", md5.Sum([]byte(fileuri))))

	mw.SetImageFormat("PNG")
	mw.SetImageProperty("Software", "CBconvert")
	mw.SetImageProperty("Description", "Thumbnail of "+fileuri)
	mw.SetImageProperty("Thumb::URI", fileuri)
	mw.SetImageProperty("Thumb::MTime", strconv.FormatInt(info.ModTime().Unix(), 10))
	mw.SetImageProperty("Thumb::Size", strconv.FormatInt(info.Size(), 10))
	mw.SetImageProperty("Thumb::Mimetype", mime.TypeByExtension(filepath.Ext(file)))

	mw.WriteImage(filename)
}

// Converts comic book
func (c *Convertor) ConvertComic(file string, info os.FileInfo) {
	c.CurrFile += 1
	if info.IsDir() {
		c.convertDirectory(file)
		c.saveArchive(file)
	} else if c.isDocument(file) {
		c.convertDocument(file)
		c.saveArchive(file)
	} else {
		c.convertArchive(file)
		c.saveArchive(file)
	}
}
