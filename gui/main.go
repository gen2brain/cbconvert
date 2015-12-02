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

//go:generate genqrc assets

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"mime"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	"github.com/dustin/go-humanize"
	"github.com/gen2brain/cbconvert"
	"github.com/gographics/imagick/imagick"
	"github.com/hotei/bmp"
	"golang.org/x/image/tiff"
	"gopkg.in/qml.v1"
)

// Model
type Comics struct {
	Root qml.Object
	Conv *cbconvert.Convertor
	List []Comic
	Len  int
}

// Comic Element
type Comic struct {
	Name      string
	Path      string
	Type      string
	Size      int64
	SizeHuman string
}

// Sorts by name
type ByName []Comic

func (c ByName) Len() int           { return len(c) }
func (c ByName) Swap(i, j int)      { c[i], c[j] = c[j], c[i] }
func (c ByName) Less(i, j int) bool { return c[i].Name < c[j].Name }

// Sorts by size
type BySize []Comic

func (c BySize) Len() int           { return len(c) }
func (c BySize) Swap(i, j int)      { c[i], c[j] = c[j], c[i] }
func (c BySize) Less(i, j int) bool { return c[i].Size < c[j].Size }

// Sorts by type
type ByType []Comic

func (c ByType) Len() int           { return len(c) }
func (c ByType) Swap(i, j int)      { c[i], c[j] = c[j], c[i] }
func (c ByType) Less(i, j int) bool { return c[i].Type < c[j].Type }

// Adds element to list
func (c *Comics) Add(comic Comic) {
	c.List = append(c.List, comic)
	c.Len = len(c.List)
	qml.Changed(c, &c.Len)
}

// Removes element from list
func (c *Comics) Remove(i int) {
	l := c.List
	l = append(l[:i], l[i+1:]...)
	c.List = l
	c.Len = len(c.List)
	qml.Changed(c, &c.Len)
}

// Removes all elements from list
func (c *Comics) RemoveAll() {
	c.Len = 0
	c.List = make([]Comic, 0)
	qml.Changed(c, &c.Len)
}

// Sorts by name
func (c *Comics) ByName() {
	sort.Sort(ByName(c.List))
	c.Len++
	qml.Changed(c, &c.Len)
	c.Len--
	qml.Changed(c, &c.Len)
}

// Sorts by size
func (c *Comics) BySize() {
	sort.Sort(BySize(c.List))
	c.Len++
	qml.Changed(c, &c.Len)
	c.Len--
	qml.Changed(c, &c.Len)
}

// Sorts by type
func (c *Comics) ByType() {
	sort.Sort(ByType(c.List))
	c.Len++
	qml.Changed(c, &c.Len)
	c.Len--
	qml.Changed(c, &c.Len)
}

// Returns element for given index
func (c *Comics) Get(i int) Comic {
	return c.List[i]
}

// Adds elements from fileUrls to list
func (c *Comics) AddUrls(u string) {
	var args []string
	l := strings.Split(u, "_CBSEP_")
	re := regexp.MustCompile(`^[a-zA-Z]:`)

	for _, f := range l {
		f = strings.Replace(f, "file://", "", -1)
		f = re.ReplaceAllString(f, "")
		f = re.ReplaceAllString(f, "")
		args = append(args, f)
	}

	c.Conv.Opts = c.GetOptions()
	files := c.Conv.GetFiles(args)

	for _, file := range files {
		stat, err := os.Stat(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error Stat AddUrls: %v\n", err.Error())
			continue
		}

		m := mime.TypeByExtension(filepath.Ext(file))
		if m == "" && stat.IsDir() {
			m = "inode/directory"
		}

		c.Add(Comic{
			filepath.Base(file),
			file,
			m,
			stat.Size(),
			humanize.IBytes(uint64(stat.Size())),
		})
	}
}

// Returns cbconvert options from qml
func (c *Comics) GetOptions() cbconvert.Options {
	var o cbconvert.Options
	o.Quiet = true

	r := c.Root.ObjectByName("checkBoxRecursive")
	o.Recursive = r.Bool("checked")

	r = c.Root.ObjectByName("checkBoxNoRGB")
	o.RGB = !r.Bool("checked")

	r = c.Root.ObjectByName("checkBoxConvertCover")
	o.ConvertCover = !r.Bool("checked")

	r = c.Root.ObjectByName("spinboxSize")
	o.Size = r.Int64("value")

	r = c.Root.ObjectByName("sliderBrightness")
	o.Brightness = r.Float64("value")

	r = c.Root.ObjectByName("sliderContrast")
	o.Contrast = r.Float64("value")

	r = c.Root.ObjectByName("checkBoxGrayscale")
	o.Grayscale = r.Bool("checked")

	r = c.Root.ObjectByName("comboBoxFlip")
	o.Flip = strings.ToLower(r.String("currentText"))

	r = c.Root.ObjectByName("comboBoxRotate")
	o.Rotate, _ = strconv.Atoi(r.String("currentText"))

	r = c.Root.ObjectByName("textFieldOutDir")
	o.Outdir = r.String("text")

	r = c.Root.ObjectByName("textFieldSuffix")
	o.Suffix = r.String("text")

	r = c.Root.ObjectByName("checkBoxNonImage")
	o.NonImage = !r.Bool("checked")

	r = c.Root.ObjectByName("comboBoxFormat")
	o.Format = strings.ToLower(r.String("currentText"))

	r = c.Root.ObjectByName("width")
	o.Width, _ = strconv.Atoi(r.String("text"))

	r = c.Root.ObjectByName("height")
	o.Height, _ = strconv.Atoi(r.String("text"))

	r = c.Root.ObjectByName("checkBoxFit")
	o.Fit = r.Bool("checked")

	r = c.Root.ObjectByName("comboBoxFilter")
	o.Filter = r.Int("currentIndex")

	r = c.Root.ObjectByName("sliderQuality")
	o.Quality = int(r.Float64("value"))

	r = c.Root.ObjectByName("spinboxLevelsInMin")
	o.LevelsInMin = r.Float64("value")

	r = c.Root.ObjectByName("spinboxLevelsInMax")
	o.LevelsInMax = r.Float64("value")

	r = c.Root.ObjectByName("spinboxLevelsGamma")
	o.LevelsGamma = r.Float64("value")

	r = c.Root.ObjectByName("spinboxLevelsOutMin")
	o.LevelsOutMin = r.Float64("value")

	r = c.Root.ObjectByName("spinboxLevelsOutMax")
	o.LevelsOutMax = r.Float64("value")

	return o
}

// Sets "enabled" property
func (c *Comics) SetEnabled(b bool) {
	c.Root.ObjectByName("checkBoxRecursive").Set("enabled", b)
	c.Root.ObjectByName("checkBoxNoRGB").Set("enabled", b)
	c.Root.ObjectByName("checkBoxConvertCover").Set("enabled", b)
	c.Root.ObjectByName("spinboxSize").Set("enabled", b)
	c.Root.ObjectByName("sliderBrightness").Set("enabled", b)
	c.Root.ObjectByName("sliderContrast").Set("enabled", b)
	c.Root.ObjectByName("checkBoxGrayscale").Set("enabled", b)
	c.Root.ObjectByName("comboBoxFlip").Set("enabled", b)
	c.Root.ObjectByName("comboBoxRotate").Set("enabled", b)
	c.Root.ObjectByName("textFieldOutDir").Set("enabled", b)
	c.Root.ObjectByName("textFieldSuffix").Set("enabled", b)
	c.Root.ObjectByName("checkBoxNonImage").Set("enabled", b)
	c.Root.ObjectByName("comboBoxFormat").Set("enabled", b)
	c.Root.ObjectByName("width").Set("enabled", b)
	c.Root.ObjectByName("height").Set("enabled", b)
	c.Root.ObjectByName("checkBoxFit").Set("enabled", b)
	c.Root.ObjectByName("comboBoxFilter").Set("enabled", b)
	c.Root.ObjectByName("sliderQuality").Set("enabled", b)
	c.Root.ObjectByName("buttonAddFile").Set("enabled", b)
	c.Root.ObjectByName("buttonAddDir").Set("enabled", b)
	c.Root.ObjectByName("buttonRemove").Set("enabled", b)
	c.Root.ObjectByName("buttonRemoveAll").Set("enabled", b)
	c.Root.ObjectByName("buttonThumbnail").Set("enabled", b)
	c.Root.ObjectByName("buttonCover").Set("enabled", b)
	c.Root.ObjectByName("buttonConvert").Set("enabled", b)
}

// Converts comic
func (c *Comics) Convert() {
	c.Conv.Opts = c.GetOptions()
	c.Conv.Nfiles = c.Len
	c.Conv.CurrFile = 0

	c.SetEnabled(false)

	go func() {
		for _, e := range c.List {
			stat, err := os.Stat(e.Path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error Stat Convert: %v\n", err.Error())
				continue
			}
			c.Conv.ConvertComic(e.Path, stat)
		}
	}()

	go c.showProgress(true, "Converting...")
}

// Extracts cover
func (c *Comics) Cover() {
	c.Conv.Opts = c.GetOptions()
	c.Conv.Nfiles = c.Len
	c.Conv.CurrFile = 0

	c.SetEnabled(false)

	go func() {
		for _, e := range c.List {
			stat, err := os.Stat(e.Path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error Stat Cover: %v\n", err.Error())
				continue
			}
			c.Conv.ExtractCover(e.Path, stat)
		}
	}()

	go c.showProgress(false, "Extracting...")
}

// Extracts thumbnail
func (c *Comics) Thumbnail() {
	c.Conv.Opts = c.GetOptions()
	c.Conv.Nfiles = c.Len
	c.Conv.CurrFile = 0

	c.SetEnabled(false)

	go func() {
		for _, e := range c.List {
			stat, err := os.Stat(e.Path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error Stat Thumbnail: %v\n", err.Error())
				continue
			}
			c.Conv.ExtractThumbnail(e.Path, stat)
		}
	}()

	go c.showProgress(false, "Extracting...")
}

// Shows progress
func (c *Comics) showProgress(cn bool, text string) {
	c.Root.ObjectByName("labelStatus").Set("text", text)
	c.Root.ObjectByName("progressBar").Set("visible", true)

	for {
		if c.Conv.CurrFile == c.Conv.Nfiles {
			if c.Conv.CurrContent == c.Conv.Ncontents {
				c.Root.ObjectByName("progressBar").Set("value", 0)
				c.Root.ObjectByName("labelProgress").Set("text", "")
				c.Root.ObjectByName("labelStatus").Set("text", "Ready")
				c.Root.ObjectByName("labelPercent").Set("text", "")
				c.Root.ObjectByName("progressBar").Set("visible", false)
				c.SetEnabled(true)
				break
			}
		}

		var count, current int
		if cn {
			count = c.Conv.Ncontents
			current = c.Conv.CurrContent
		} else {
			count = c.Conv.Nfiles
			current = c.Conv.CurrFile
		}

		value := float64(current) / float64(count) * 100
		c.Root.ObjectByName("progressBar").Set("value", float64(value))
		c.Root.ObjectByName("labelPercent").Set("text",
			fmt.Sprintf("%d/%d %.0f%%", current, count, float64(value)))
		c.Root.ObjectByName("labelProgress").Set("text",
			fmt.Sprintf("File %d of %d", c.Conv.CurrFile, c.Conv.Nfiles))

		time.Sleep(500 * time.Millisecond)
	}
}

// Provides image://cover/
func (c *Comics) CoverProvider(file string, width int, height int) image.Image {
	c.Conv.Opts = c.GetOptions()

	stat, err := os.Stat(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error Stat CoverProvider: %v\n", err.Error())
		return image.NewRGBA(image.Rect(0, 0, width, height))
	}

	cover, err := c.Conv.GetCoverImage(file, stat)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error GetCoverImage: %v\n", err.Error())
		return image.NewRGBA(image.Rect(0, 0, width, height))
	}

	cover = c.Conv.TransformImage(cover)

	if c.Conv.Opts.LevelsInMin != 0 || c.Conv.Opts.LevelsInMax != 255 || c.Conv.Opts.LevelsGamma != 1.00 ||
		c.Conv.Opts.LevelsOutMin != 0 || c.Conv.Opts.LevelsOutMax != 255 {
		cover = c.Conv.LevelImage(cover)
	}

	// imaging is used for preview only
	if c.Conv.Opts.Grayscale {
		cover = imaging.Grayscale(cover)
	}

	// size preview
	s := 0
	b := new(bytes.Buffer)

	w := 0
	h := 0

	switch c.Conv.Opts.Format {
	case "jpeg":
		jpeg.Encode(b, cover, &jpeg.Options{c.Conv.Opts.Quality})
		s = len(b.Bytes())
		cover, _ = jpeg.Decode(bytes.NewReader(b.Bytes()))
		config, _, _ := image.DecodeConfig(bytes.NewReader(b.Bytes()))
		w = config.Width
		h = config.Height
	case "png":
		png.Encode(b, cover)
		s = len(b.Bytes())
		cover, _ = png.Decode(bytes.NewReader(b.Bytes()))
		config, _, _ := image.DecodeConfig(bytes.NewReader(b.Bytes()))
		w = config.Width
		h = config.Height
	case "gif":
		mw := imagick.NewMagickWand()
		defer mw.Destroy()

		mw.ReadImageBlob(c.Conv.GetImageBytes(cover))
		mw.SetImageFormat("GIF")
		blob := mw.GetImageBlob()

		s = len(blob)
		cover, _ = gif.Decode(bytes.NewReader(blob))
		config, _, _ := image.DecodeConfig(bytes.NewReader(blob))
		w = config.Width
		h = config.Height
	case "tiff":
		tiff.Encode(b, cover, &tiff.Options{tiff.Uncompressed, false})

		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		gz.Write(b.Bytes())
		gz.Close()

		s = buf.Len()
		cover, _ = tiff.Decode(bytes.NewReader(b.Bytes()))
		config, _, _ := image.DecodeConfig(bytes.NewReader(b.Bytes()))
		w = config.Width
		h = config.Height
	case "bmp":
		mw := imagick.NewMagickWand()
		defer mw.Destroy()

		bb := c.Conv.GetImageBytes(cover)
		mw.ReadImageBlob(bb)

		wand := imagick.NewPixelWand()
		wand.SetColor("black")
		defer wand.Destroy()

		mw.SetImageFormat("BMP3")
		mw.SetImageBackgroundColor(wand)
		mw.SetImageAlphaChannel(imagick.ALPHA_CHANNEL_REMOVE)
		mw.SetImageAlphaChannel(imagick.ALPHA_CHANNEL_DEACTIVATE)
		mw.SetImageMatte(false)
		mw.SetImageCompression(imagick.COMPRESSION_NO)
		mw.QuantizeImage(16, mw.GetImageColorspace(), 8, true, true)

		var buf bytes.Buffer
		blob := mw.GetImageBlob()
		gz := gzip.NewWriter(&buf)
		gz.Write(blob)
		gz.Close()

		s = buf.Len()
		cover, _ = bmp.Decode(bytes.NewReader(blob))
		config, _, _ := image.DecodeConfig(bytes.NewReader(bb))
		w = config.Width
		h = config.Height
	}

	if cover == nil {
		return image.NewRGBA(image.Rect(0, 0, width, height))
	}

	human := humanize.IBytes(uint64(s))
	c.Root.ObjectByName("sizePreview").Set("text", fmt.Sprintf("%s (%dx%d)", human, w, h))

	return cover
}

func run() error {
	qml.SetWindowIcon(":///assets/icon.png")

	engine := qml.NewEngine()

	c := &Comics{}
	engine.Context().SetVar("c", c)

	engine.AddImportPath("qrc:/assets")
	engine.AddImageProvider("cover", c.CoverProvider)

	q, err := engine.LoadFile("qrc:///assets/main.qml")
	if err != nil {
		return err
	}

	window := q.CreateWindow(nil)
	c.Root = window.Root()
	c.Conv = cbconvert.NewConvertor(c.GetOptions())

	c.Root.On("closing", func(o qml.Object) {
		os.RemoveAll(c.Conv.Workdir)
	})

	c.Root.ObjectByName("buttonConvert").On("clicked", c.Convert)
	c.Root.ObjectByName("buttonCover").On("clicked", c.Cover)
	c.Root.ObjectByName("buttonThumbnail").On("clicked", c.Thumbnail)

	// center window
	x := c.Root.Int("screenWidth")/2 - c.Root.Int("width")/2
	y := c.Root.Int("screenHeight")/2 - c.Root.Int("height")/2
	window.Set("x", x)
	window.Set("y", y)

	window.Show()
	window.Wait()
	return nil
}

func main() {
	if err := qml.Run(run); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
