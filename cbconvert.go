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

package main

import (
	"archive/zip"
	"bytes"
	"crypto/md5"
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"io/ioutil"
	"mime"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/MStoykov/go-libarchive"
	"github.com/cheggaaa/go-poppler"
	"github.com/cheggaaa/pb"
	"github.com/gographics/imagick/imagick"
	_ "github.com/hotei/bmp"
	"github.com/nfnt/resize"
	"github.com/skarademir/naturalsort"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	opts    options
	workdir string
	nfiles  int
	current int
	wg      sync.WaitGroup
)

// Command line options
type options struct {
	ToPNG         bool   // encode images to PNG instead of JPEG
	ToBMP         bool   // encode images to 4-Bit BMP instead of JPEG
	Quality       int    // JPEG image quality
	NoRGB         bool   // do not convert images with RGB colorspace
	Width         uint   // image width
	Height        uint   // image height
	Interpolation int    // 0=NearestNeighbor, 1=Bilinear, 2=Bicubic, 3=MitchellNetravali, 4=Lanczos2, 5=Lanczos3
	Suffix        string // add suffix to file basename
	Cover         bool   // extract cover
	Thumbnail     bool   // extract cover thumbnail (freedesktop spec.)
	Outdir        string // output directory
	Recursive     bool   // process subdirectories recursively
	Size          int64  // process only files larger then size (in MB)
	Quiet         bool   // hide console output
}

// Command line arguments
var arguments []string

// Limits go routines to number of CPUs + 1
var throttle = make(chan int, runtime.NumCPU()+1)

// Converts image
func convertImage(img image.Image, index int, pathName string) {
	defer wg.Done()

	var ext string = "jpg"
	if opts.ToPNG {
		ext = "png"
	} else if opts.ToBMP {
		ext = "bmp"
	}

	var filename string
	if pathName != "" {
		filename = filepath.Join(workdir, fmt.Sprintf("%s.%s", getBasename(pathName), ext))
	} else {
		filename = filepath.Join(workdir, fmt.Sprintf("%03d.%s", index, ext))
	}

	var i image.Image
	if opts.Width > 0 || opts.Height > 0 {
		i = resize.Resize(opts.Width, opts.Height, img,
			resize.InterpolationFunction(opts.Interpolation))
	} else {
		i = img
	}

	if opts.ToPNG {
		// convert image to PNG
		f, err := os.Create(filename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error Create: %v\n", err.Error())
		}
		defer f.Close()
		png.Encode(f, i)
	} else if opts.ToBMP {
		// convert image to 4-Bit - 16 colors BMP
		imagick.Initialize()

		mw := imagick.NewMagickWand()
		defer mw.Destroy()

		b := new(bytes.Buffer)
		jpeg.Encode(b, i, &jpeg.Options{jpeg.DefaultQuality})

		err := mw.ReadImageBlob(b.Bytes())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error ReadImageBlob: %v\n", err.Error())
		}

		w := imagick.NewPixelWand()
		w.SetColor("black")
		defer w.Destroy()

		mw.SetImageBackgroundColor(w)
		mw.SetImageAlphaChannel(imagick.ALPHA_CHANNEL_REMOVE)
		mw.SetImageAlphaChannel(imagick.ALPHA_CHANNEL_DEACTIVATE)
		mw.SetImageMatte(false)
		mw.SetImageCompression(imagick.COMPRESSION_NO)
		mw.QuantizeImage(16, imagick.COLORSPACE_SRGB, 8, true, true)
		mw.WriteImage(fmt.Sprintf("BMP3:%s", filename))
	} else {
		// convert image to JPEG (default)
		f, err := os.Create(filename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error Create: %v\n", err.Error())
		}
		defer f.Close()
		jpeg.Encode(f, i, &jpeg.Options{opts.Quality})
	}

	<-throttle
}

// Converts pdf file to cbz
func convertPDF(file string) {
	workdir, _ = ioutil.TempDir(os.TempDir(), "cbc")

	doc, err := poppler.Open(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Skipping %s, error: %v", file, err.Error())
		return
	}

	npages := doc.GetNPages()

	var bar *pb.ProgressBar
	if !opts.Quiet {
		bar = pb.New(npages)
		bar.ShowTimeLeft = false
		bar.Prefix(fmt.Sprintf("Converting %d of %d: ", current, nfiles))
		bar.Start()
	}

	for n := 0; n < npages; n++ {
		if !opts.Quiet {
			bar.Increment()
		}

		page := doc.GetPage(n)
		images := page.Images()

		if len(images) == 1 {
			throttle <- 1
			wg.Add(1)

			surface := images[0].GetSurface()
			go convertImage(surface.GetImage(), page.Index(), "")
		} else {
			// FIXME merge images?
		}
	}
	wg.Wait()
}

// Converts archive to cbz
func convertArchive(file string) {
	workdir, _ = ioutil.TempDir(os.TempDir(), "cbc")

	f, err := os.Open(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error Open: %v\n", err.Error())
		return
	}
	defer f.Close()

	reader, err := archive.NewReader(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error NewReader: %v\n", err.Error())
	}
	defer reader.Free()
	defer reader.Close()

	var bar *pb.ProgressBar
	if !opts.Quiet {
		s, _ := f.Stat()
		bar = pb.New(int(s.Size()))
		bar.SetUnits(pb.U_BYTES)
		bar.ShowTimeLeft = false
		bar.Prefix(fmt.Sprintf("Converting %d of %d: ", current, nfiles))
		bar.Start()
	}

	for {
		entry, err := reader.Next()
		if err != nil {
			if err == archive.ErrArchiveEOF {
				break
			} else {
				fmt.Fprintf(os.Stderr, "Error Next: %v\n", err.Error())
				continue
			}
		}

		stat := entry.Stat()
		if stat.Mode()&os.ModeType != 0 || stat.IsDir() {
			continue
		}

		if !opts.Quiet {
			size := reader.Size()
			bar.Set(size)
		}

		pathname := entry.PathName()

		if isImage(pathname) {
			buf := new(bytes.Buffer)
			_, err := buf.ReadFrom(reader)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error ReadFrom: %v\n", err.Error())
				continue
			}

			img, err := decodeImage(bytes.NewReader(buf.Bytes()), pathname)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error Decode: %v\n", err.Error())
				continue
			}

			if opts.NoRGB && !isGrayScale(img) {
				copyFile(bytes.NewReader(buf.Bytes()), filepath.Join(workdir, filepath.Base(pathname)))
				continue
			}

			if img != nil {
				throttle <- 1
				wg.Add(1)
				go convertImage(img, 0, pathname)
			}
		} else {
			copyFile(reader, filepath.Join(workdir, filepath.Base(pathname)))
		}
	}
	wg.Wait()
}

// Converts directory to cbz
func convertDirectory(path string) {
	workdir, _ = ioutil.TempDir(os.TempDir(), "cbc")

	images := getImages(path)

	var bar *pb.ProgressBar
	if !opts.Quiet {
		bar = pb.New(nfiles)
		bar.ShowTimeLeft = false
		bar.Prefix(fmt.Sprintf("Converting %d of %d: ", current, nfiles))
		bar.Start()
	}

	for index, img := range images {
		if opts.Quiet {
			bar.Increment()
		}

		f, err := os.Open(img)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error Open: %v\n", err.Error())
			continue
		}

		i, err := decodeImage(f, img)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error Decode: %v\n", err.Error())
			continue
		}

		if opts.NoRGB && !isGrayScale(i) {
			copyFile(f, filepath.Join(workdir, filepath.Base(img)))
			continue
		}

		f.Close()

		if i != nil {
			throttle <- 1
			wg.Add(1)
			go convertImage(i, index, img)
		}
	}
	wg.Wait()
}

// Saves workdir to cbz archive
func saveArchive(file string) {
	defer os.RemoveAll(workdir)

	zipname := filepath.Join(opts.Outdir, fmt.Sprintf("%s%s.cbz", getBasename(file), opts.Suffix))
	zipfile, err := os.Create(zipname)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error Create: %v\n", err.Error())
		return
	}
	defer zipfile.Close()

	z := zip.NewWriter(zipfile)
	files, _ := ioutil.ReadDir(workdir)

	var bar *pb.ProgressBar
	if !opts.Quiet {
		bar = pb.New(len(files))
		bar.ShowTimeLeft = false
		bar.Prefix(fmt.Sprintf("Compressing %d of %d: ", current, nfiles))
		bar.Start()
	}

	for _, file := range files {
		if !opts.Quiet {
			bar.Increment()
		}

		r, err := ioutil.ReadFile(filepath.Join(workdir, file.Name()))
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

// Unpacks archive to directory
func unpackArchive(file string, dir string) {
	f, err := os.Open(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error Open: %v\n", err.Error())
		return
	}
	defer f.Close()

	reader, err := archive.NewReader(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error NewReader: %v\n", err.Error())
		return
	}
	defer reader.Free()
	defer reader.Close()

	for {
		entry, err := reader.Next()
		if err != nil {
			if err == archive.ErrArchiveEOF {
				break
			} else {
				continue
			}
		}

		if entry.Stat().Mode()&os.ModeType == 0 {
			err = copyFile(reader, filepath.Join(dir, entry.PathName()))
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err.Error())
		}
	}
}

// Extracts cover from archive
func coverArchive(file string) (image.Image, error) {
	tmpdir, _ := ioutil.TempDir(os.TempDir(), "cbc")
	defer os.RemoveAll(tmpdir)

	unpackArchive(file, tmpdir)

	images := getImages(tmpdir)
	if len(images) == 0 {
		return nil, errors.New("No images")
	}

	cover := getCover(images)

	p, err := os.Open(cover)
	if err != nil {
		return nil, err
	}
	defer p.Close()

	img, err := decodeImage(p, cover)
	if err != nil {
		return nil, err
	}

	return img, nil
}

// Extracts cover from pdf
func coverPDF(file string) (image.Image, error) {
	doc, err := poppler.Open(file)
	if err != nil {
		return nil, err
	}

	page := doc.GetPage(0)
	images := page.Images()

	if len(images) == 1 {
		surface := images[0].GetSurface()
		img := surface.GetImage()

		if img == nil {
			return nil, errors.New("Image is nil")
		}

		return img, nil
	}

	return nil, nil
}

// Extracts cover from directory
func coverDirectory(dir string) (image.Image, error) {
	images := getImages(dir)
	cover := getCover(images)

	p, err := os.Open(cover)
	if err != nil {
		return nil, err
	}
	defer p.Close()

	img, err := decodeImage(p, cover)
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
func getFiles() []string {
	var files []string

	walkFiles := func(fp string, f os.FileInfo, err error) error {
		if !f.IsDir() {
			if isComic(fp) {
				if isSize(f.Size()) {
					files = append(files, fp)
				}
			}
		}
		return nil
	}

	for _, arg := range arguments {
		path, _ := filepath.Abs(arg)
		stat, err := os.Stat(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error Stat: %v\n", err.Error())
			continue
		}

		if !stat.IsDir() {
			if isComic(path) {
				if isSize(stat.Size()) {
					files = append(files, path)
				}
			}
		} else {
			if opts.Recursive {
				filepath.Walk(path, walkFiles)
			} else {
				fs, _ := ioutil.ReadDir(path)
				for _, f := range fs {
					if isComic(f.Name()) {
						if isSize(f.Size()) {
							files = append(files, f.Name())
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

	return files
}

// Returns list of found image files for given directory
func getImages(path string) []string {
	var images []string

	walkFiles := func(fp string, f os.FileInfo, err error) error {
		if !f.IsDir() && f.Mode()&os.ModeType == 0 {
			if f.Size() > 0 && isImage(fp) {
				images = append(images, fp)
			}
		}
		return nil
	}

	f, _ := filepath.Abs(path)
	stat, err := os.Stat(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error Stat: %v\n", err.Error())
		return images
	}

	if !stat.IsDir() && stat.Mode()&os.ModeType == 0 {
		if isImage(f) {
			images = append(images, f)
		}
	} else {
		filepath.Walk(f, walkFiles)
	}

	return images
}

// Returns the filename that is the most likely to be the cover
func getCover(images []string) string {
	for _, i := range images {
		if strings.HasPrefix(i, "cover") || strings.HasPrefix(i, "front") {
			return i
		}
	}

	sort.Sort(naturalsort.NaturalSort(images))
	return images[0]
}

// Checks if file is comic
func isComic(f string) bool {
	var types = []string{".rar", ".zip", ".7z", ".gz", ".bz2",
		".cbr", ".cbz", ".cb7", ".cbt", ".pdf"}
	for _, t := range types {
		if strings.ToLower(filepath.Ext(f)) == t {
			return true
		}
	}
	return false
}

// Checks if file is image
func isImage(f string) bool {
	var types = []string{".jpg", ".jpeg", ".jpe",
		".png", ".gif", ".bmp"}
	for _, t := range types {
		if strings.ToLower(filepath.Ext(f)) == t {
			return true
		}
	}
	return false
}

// Checks size of file
func isSize(size int64) bool {
	if opts.Size > 0 {
		if size < opts.Size*(1024*1024) {
			return false
		}
	}
	return true
}

// Checks if image is grayscale
func isGrayScale(img image.Image) bool {
	model := img.ColorModel()
	if model == color.GrayModel || model == color.Gray16Model {
		return true
	}
	return false
}

// Decodes image from reader
func decodeImage(reader io.Reader, filename string) (i image.Image, err error) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "Recovered in decodeImage %s: %v\n", filename, r)
		}
	}()

	i, _, err = image.Decode(reader)
	return i, err
}

// Copies reader to file
func copyFile(reader io.Reader, filename string) error {
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
func getBasename(file string) string {
	basename := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
	basename = strings.TrimSuffix(basename, ".tar")
	return basename
}

// Extracts cover
func extractCover(file string, info os.FileInfo) {
	var err error
	var cover image.Image
	if info.IsDir() {
		cover, err = coverDirectory(file)
	} else if strings.ToLower(filepath.Ext(file)) == ".pdf" {
		cover, err = coverPDF(file)
	} else {
		cover, err = coverArchive(file)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error Cover: %v\n", err.Error())
		return
	}

	if opts.Width > 0 || opts.Height > 0 {
		cover = resize.Resize(opts.Width, opts.Height, cover,
			resize.InterpolationFunction(opts.Interpolation))
	}

	filename := filepath.Join(opts.Outdir, fmt.Sprintf("%s.jpg", getBasename(file)))
	f, err := os.Create(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error Create: %v\n", err.Error())
		return
	}
	defer f.Close()

	jpeg.Encode(f, cover, &jpeg.Options{opts.Quality})
}

// Extracts thumbnail
func extractThumbnail(file string, info os.FileInfo) {
	var err error
	var cover image.Image
	if info.IsDir() {
		cover, err = coverDirectory(file)
	} else if strings.ToLower(filepath.Ext(file)) == ".pdf" {
		cover, err = coverPDF(file)
	} else {
		cover, err = coverArchive(file)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error Thumbnail: %v\n", err.Error())
		return
	}

	if opts.Width > 0 || opts.Height > 0 {
		cover = resize.Resize(opts.Width, opts.Height, cover,
			resize.InterpolationFunction(opts.Interpolation))
	} else {
		cover = resize.Resize(256, 0, cover,
			resize.InterpolationFunction(opts.Interpolation))
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
	filename := filepath.Join(opts.Outdir, fmt.Sprintf("%x.png", md5.Sum([]byte(fileuri))))

	mw.SetImageFormat("png")
	mw.SetImageProperty("Software", "cbconvert")
	mw.SetImageProperty("Description", "Thumbnail of "+fileuri)
	mw.SetImageProperty("Thumb::URI", fileuri)
	mw.SetImageProperty("Thumb::MTime", strconv.FormatInt(info.ModTime().Unix(), 10))
	mw.SetImageProperty("Thumb::Size", strconv.FormatInt(info.Size(), 10))
	mw.SetImageProperty("Thumb::Mimetype", mime.TypeByExtension(filepath.Ext(file)))

	mw.WriteImage(filename)
}

// Converts comic book
func convertComic(file string, info os.FileInfo) {
	if info.IsDir() {
		convertDirectory(file)
		saveArchive(file)
	} else if strings.ToLower(filepath.Ext(file)) == ".pdf" {
		convertPDF(file)
		saveArchive(file)
	} else {
		convertArchive(file)
		saveArchive(file)
	}
}

// Parses command line flags
func parseFlags() {
	opts = options{}
	kingpin.Version("CBconvert 0.1.0")
	kingpin.CommandLine.Help = "Comic Book convert tool."
	kingpin.Flag("png", "encode images to PNG instead of JPEG").Short('p').BoolVar(&opts.ToPNG)
	kingpin.Flag("bmp", "encode images to 4-Bit BMP instead of JPEG").Short('b').BoolVar(&opts.ToBMP)
	kingpin.Flag("width", "image width").Default(strconv.Itoa(0)).Short('w').UintVar(&opts.Width)
	kingpin.Flag("height", "image height").Default(strconv.Itoa(0)).Short('h').UintVar(&opts.Height)
	kingpin.Flag("quality", "JPEG image quality").Short('q').Default(strconv.Itoa(jpeg.DefaultQuality)).IntVar(&opts.Quality)
	kingpin.Flag("norgb", "do not convert images with RGB colorspace").Short('n').BoolVar(&opts.NoRGB)
	kingpin.Flag("interpolation", "0=NearestNeighbor, 1=Bilinear, 2=Bicubic, 3=MitchellNetravali, 4=Lanczos2, 5=Lanczos3").Short('i').
		Default(strconv.Itoa(int(resize.Bilinear))).IntVar(&opts.Interpolation)
	kingpin.Flag("suffix", "add suffix to file basename").Short('s').StringVar(&opts.Suffix)
	kingpin.Flag("cover", "extract cover").Short('c').BoolVar(&opts.Cover)
	kingpin.Flag("thumbnail", "extract cover thumbnail (freedesktop spec.)").Short('t').BoolVar(&opts.Thumbnail)
	kingpin.Flag("outdir", "output directory").Default(".").Short('o').StringVar(&opts.Outdir)
	kingpin.Flag("size", "process only files larger then size (in MB)").Short('m').Default(strconv.Itoa(0)).Int64Var(&opts.Size)
	kingpin.Flag("recursive", "process subdirectories recursively").Short('r').BoolVar(&opts.Recursive)
	kingpin.Flag("quiet", "hide console output").Short('Q').BoolVar(&opts.Quiet)
	kingpin.Arg("args", "filename or directory").Required().ExistingFilesOrDirsVar(&arguments)
	kingpin.Parse()
}

func main() {
	parseFlags()

	c := make(chan os.Signal, 3)
	signal.Notify(c, os.Interrupt, syscall.SIGHUP, syscall.SIGTERM)
	go func() {
		for _ = range c {
			os.RemoveAll(workdir)
			os.Exit(1)
		}
	}()

	if _, err := os.Stat(opts.Outdir); err != nil {
		os.MkdirAll(opts.Outdir, 0777)
	}

	files := getFiles()
	nfiles = len(files)
	for n, file := range files {
		current = n + 1

		stat, err := os.Stat(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error Stat: %v\n", err.Error())
			continue
		}

		if opts.Cover {
			extractCover(file, stat)
			continue
		} else if opts.Thumbnail {
			extractThumbnail(file, stat)
			continue
		}

		convertComic(file, stat)
	}
}
