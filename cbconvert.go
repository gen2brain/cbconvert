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

	"github.com/cheggaaa/pb"
	"github.com/disintegration/imaging"
	"github.com/gen2brain/go-fitz"
	"github.com/gen2brain/go-unarr"
	"github.com/gographics/imagick/imagick"
	_ "github.com/hotei/bmp"
	"github.com/skarademir/naturalsort"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
	"gopkg.in/alecthomas/kingpin.v2"
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

// Globals
var (
	opts    options
	workdir string
	nfiles  int
	current int
	wg      sync.WaitGroup
)

// Command line options
type options struct {
	ToPNG     bool   // encode images to PNG instead of JPEG
	ToBMP     bool   // encode images to 4-Bit BMP (16 colors) instead of JPEG
	ToGIF     bool   // encode images to GIF instead of JPEG
	Quality   int    // JPEG image quality
	Width     int    // image width
	Height    int    // image height
	Filter    int    // 0=NearestNeighbor, 1=Box, 2=Linear, 3=MitchellNetravali, 4=CatmullRom, 6=Gaussian, 7=Lanczos
	RGB       bool   // convert images that have RGB colorspace
	NonImage  bool   // Leave non image files in archive
	Suffix    string // add suffix to file basename
	Cover     bool   // extract cover
	Thumbnail bool   // extract cover thumbnail (freedesktop spec.)
	Outdir    string // output directory
	Grayscale bool   // convert images to grayscale (monochromatic)
	Recursive bool   // process subdirectories recursively
	Size      int64  // process only files larger then size (in MB)
	Quiet     bool   // hide console output
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
	} else if opts.ToGIF {
		ext = "gif"
	}

	var filename string
	if pathName != "" {
		filename = filepath.Join(workdir, fmt.Sprintf("%s.%s", getBasename(pathName), ext))
	} else {
		filename = filepath.Join(workdir, fmt.Sprintf("%03d.%s", index, ext))
	}

	var i image.Image

	if opts.Width > 0 || opts.Height > 0 {
		i = imaging.Resize(img, opts.Width, opts.Height, filters[opts.Filter])
	} else {
		i = img
	}

	if opts.Grayscale {
		i = imaging.Grayscale(img)
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
		// convert image to 4-Bit BMP (16 colors)
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

		var cs imagick.ColorspaceType = imagick.COLORSPACE_SRGB
		if opts.Grayscale {
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
	} else if opts.ToGIF {
		// convert image to GIF
		imagick.Initialize()

		mw := imagick.NewMagickWand()
		defer mw.Destroy()

		b := new(bytes.Buffer)
		jpeg.Encode(b, i, &jpeg.Options{jpeg.DefaultQuality})

		err := mw.ReadImageBlob(b.Bytes())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error ReadImageBlob: %v\n", err.Error())
		}

		var cs imagick.ColorspaceType = imagick.COLORSPACE_SRGB
		if opts.Grayscale {
			cs = imagick.COLORSPACE_GRAY
		}

		mw.SetImageFormat("GIF")
		mw.SetImageAlphaChannel(imagick.ALPHA_CHANNEL_REMOVE)
		mw.SetImageAlphaChannel(imagick.ALPHA_CHANNEL_DEACTIVATE)
		mw.SetImageMatte(false)
		mw.SetImageCompression(imagick.COMPRESSION_LZW)
		mw.QuantizeImage(256, cs, 8, true, true)
		mw.WriteImage(filename)
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

// Converts PDF/EPUB/XPS document to CBZ
func convertDocument(file string) {
	workdir, _ = ioutil.TempDir(os.TempDir(), "cbc")

	doc, err := fitz.NewDocument(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Skipping %s, error: %v", file, err.Error())
		return
	}

	npages := doc.Pages()

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

		img, err := doc.Image(n)

		if err == nil && img != nil {
			throttle <- 1
			wg.Add(1)

			go convertImage(img, n, "")
		}
	}
	wg.Wait()
}

// Converts archive to CBZ
func convertArchive(file string) {
	workdir, _ = ioutil.TempDir(os.TempDir(), "cbc")

	ncontents := len(listArchive(file))

	archive, err := unarr.NewArchive(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error NewReader: %v\n", err.Error())
	}
	defer archive.Close()

	var bar *pb.ProgressBar
	if !opts.Quiet {
		bar = pb.New(ncontents)
		bar.ShowTimeLeft = false
		bar.Prefix(fmt.Sprintf("Converting %d of %d: ", current, nfiles))
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

		if !opts.Quiet {
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

		if isImage(pathname) {
			img, err := decodeImage(bytes.NewReader(buf), pathname)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error Decode: %v\n", err.Error())
				continue
			}

			if !opts.RGB && !isGrayScale(img) {
				copyFile(bytes.NewReader(buf), filepath.Join(workdir, filepath.Base(pathname)))
				continue
			}

			if img != nil {
				throttle <- 1
				wg.Add(1)
				go convertImage(img, 0, pathname)
			}
		} else {
			if opts.NonImage {
				copyFile(bytes.NewReader(buf), filepath.Join(workdir, filepath.Base(pathname)))
			}
		}
	}
	wg.Wait()
}

// Converts directory to CBZ
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

		if !opts.RGB && !isGrayScale(i) {
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

// Saves workdir to CBZ archive
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

// Lists contents of archive
func listArchive(file string) []string {
	var contents []string
	archive, err := unarr.NewArchive(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error NewReader: %v\n", err.Error())
	}
	defer archive.Close()

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

		pathname := archive.Name()
		contents = append(contents, pathname)
	}

	return contents
}

// Extracts cover from archive
func coverArchive(file string) (image.Image, error) {
	var images []string

	contents := listArchive(file)
	for _, c := range contents {
		if isImage(c) {
			images = append(images, c)
		}
	}

	cover := getCover(images)

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

	img, err := decodeImage(bytes.NewReader(buf), cover)
	if err != nil {
		return nil, err
	}

	return img, nil
}

// Extracts cover from document
func coverDocument(file string) (image.Image, error) {
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
			if isArchive(fp) || isDocument(fp) {
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
			if isArchive(path) || isDocument(path) {
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
					if isArchive(f.Name()) || isArchive(f.Name()) {
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
	if len(images) == 0 {
		return ""
	}

	for _, i := range images {
		if strings.HasPrefix(i, "cover") || strings.HasPrefix(i, "front") {
			return i
		}
	}

	sort.Sort(naturalsort.NaturalSort(images))
	return images[0]
}

// Checks if file is archive
func isArchive(f string) bool {
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
func isDocument(f string) bool {
	var types = []string{".pdf", ".epub", ".xps"}
	for _, t := range types {
		if strings.ToLower(filepath.Ext(f)) == t {
			return true
		}
	}
	return false
}

// Checks if file is image
func isImage(f string) bool {
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
	} else if isDocument(file) {
		cover, err = coverDocument(file)
	} else {
		cover, err = coverArchive(file)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error Cover: %v\n", err.Error())
		return
	}

	if opts.Width > 0 || opts.Height > 0 {
		cover = imaging.Resize(cover, opts.Width, opts.Height, filters[opts.Filter])
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
	} else if isDocument(file) {
		cover, err = coverDocument(file)
	} else {
		cover, err = coverArchive(file)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error Thumbnail: %v\n", err.Error())
		return
	}

	if opts.Width > 0 || opts.Height > 0 {
		cover = imaging.Resize(cover, opts.Width, opts.Height, filters[opts.Filter])
	} else {
		cover = imaging.Resize(cover, 256, 0, filters[opts.Filter])
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

	mw.SetImageFormat("PNG")
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
	} else if isDocument(file) {
		convertDocument(file)
		saveArchive(file)
	} else {
		convertArchive(file)
		saveArchive(file)
	}
}

// Parses command line flags
func parseFlags() {
	opts = options{}
	kingpin.Version("CBconvert 0.3.0")
	kingpin.CommandLine.Help = "Comic Book convert tool."
	kingpin.UsageTemplate(kingpin.CompactUsageTemplate)

	kingpin.Flag("outdir", "Output directory").Default(".").Short('o').StringVar(&opts.Outdir)
	kingpin.Flag("size", "Process only files larger then size (in MB)").Short('m').Default(strconv.Itoa(0)).Int64Var(&opts.Size)
	kingpin.Flag("recursive", "Process subdirectories recursively").Short('R').BoolVar(&opts.Recursive)
	kingpin.Flag("quiet", "Hide console output").Short('Q').BoolVar(&opts.Quiet)

	convert := kingpin.Command("convert", "Convert archive or document (default)").Default()
	convert.Arg("args", "filename or directory").Required().ExistingFilesOrDirsVar(&arguments)
	convert.Flag("width", "Image width").Default(strconv.Itoa(0)).Short('w').IntVar(&opts.Width)
	convert.Flag("height", "Image height").Default(strconv.Itoa(0)).Short('h').IntVar(&opts.Height)
	convert.Flag("quality", "JPEG image quality").Short('q').Default(strconv.Itoa(jpeg.DefaultQuality)).IntVar(&opts.Quality)
	convert.Flag("filter", "0=NearestNeighbor, 1=Box, 2=Linear, 3=MitchellNetravali, 4=CatmullRom, 6=Gaussian, 7=Lanczos").Short('f').Default(strconv.Itoa(Linear)).IntVar(&opts.Filter)
	convert.Flag("png", "Encode images to PNG instead of JPEG").Short('p').BoolVar(&opts.ToPNG)
	convert.Flag("bmp", "Encode images to 4-Bit BMP (16 colors) instead of JPEG").Short('b').BoolVar(&opts.ToBMP)
	convert.Flag("gif", "Encode images to GIF instead of JPEG").Short('g').BoolVar(&opts.ToGIF)
	convert.Flag("rgb", "Convert images that have RGB colorspace (use --no-rgb if you only want to process grayscale images)").Short('N').Default("true").BoolVar(&opts.RGB)
	convert.Flag("nonimage", "Leave non image files in archive (use --no-nonimage to remove non image files from archive)").Short('I').Default("true").BoolVar(&opts.NonImage)
	convert.Flag("grayscale", "Convert images to grayscale (monochromatic)").Short('G').BoolVar(&opts.Grayscale)
	convert.Flag("suffix", "Add suffix to file basename").Short('s').StringVar(&opts.Suffix)

	cover := kingpin.Command("cover", "Extract cover")
	cover.Arg("args", "filename or directory").Required().ExistingFilesOrDirsVar(&arguments)
	cover.Flag("width", "Image width").Default(strconv.Itoa(0)).Short('w').IntVar(&opts.Width)
	cover.Flag("height", "Image height").Default(strconv.Itoa(0)).Short('h').IntVar(&opts.Height)
	cover.Flag("quality", "JPEG image quality").Short('q').Default(strconv.Itoa(jpeg.DefaultQuality)).IntVar(&opts.Quality)
	cover.Flag("filter", "0=NearestNeighbor, 1=Box, 2=Linear, 3=MitchellNetravali, 4=CatmullRom, 6=Gaussian, 7=Lanczos").Short('f').Default(strconv.Itoa(Linear)).IntVar(&opts.Filter)

	thumbnail := kingpin.Command("thumbnail", "Extract cover thumbnail (freedesktop spec.)")
	thumbnail.Arg("args", "filename or directory").Required().ExistingFilesOrDirsVar(&arguments)
	thumbnail.Flag("width", "Image width").Default(strconv.Itoa(0)).Short('w').IntVar(&opts.Width)
	thumbnail.Flag("height", "Image height").Default(strconv.Itoa(0)).Short('h').IntVar(&opts.Height)

	switch kingpin.Parse() {
	case "cover":
		opts.Cover = true
	case "thumbnail":
		opts.Thumbnail = true
	}
}

func main() {
	parseFlags()

	c := make(chan os.Signal, 3)
	signal.Notify(c, os.Interrupt, syscall.SIGHUP, syscall.SIGTERM)
	go func() {
		for _ = range c {
			fmt.Fprintf(os.Stderr, "Aborting\n")
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
