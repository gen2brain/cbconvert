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

//go:generate goversioninfo

import (
	"fmt"
	"image/jpeg"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/cheggaaa/pb"
	"github.com/gen2brain/cbconvert"
	"gopkg.in/alecthomas/kingpin.v2"
)

// Parses command line flags
func parseFlags() (cbconvert.Options, []string) {
	opts := cbconvert.Options{}
	var args []string

	kingpin.Version("CBconvert 0.5.0")
	kingpin.CommandLine.Help = "Comic Book convert tool."
	kingpin.UsageTemplate(kingpin.CompactUsageTemplate)

	kingpin.Flag("outdir", "Output directory").Default(".").StringVar(&opts.Outdir)
	kingpin.Flag("size", "Process only files larger then size (in MB)").Default(strconv.Itoa(0)).Int64Var(&opts.Size)
	kingpin.Flag("recursive", "Process subdirectories recursively").BoolVar(&opts.Recursive)
	kingpin.Flag("quiet", "Hide console output").BoolVar(&opts.Quiet)

	convert := kingpin.Command("convert", "Convert archive or document (default command)").Default()
	convert.Arg("args", "filename or directory").Required().ExistingFilesOrDirsVar(&args)
	convert.Flag("width", "Image width").Default(strconv.Itoa(0)).IntVar(&opts.Width)
	convert.Flag("height", "Image height").Default(strconv.Itoa(0)).IntVar(&opts.Height)
	convert.Flag("fit", "Best fit for required width and height").BoolVar(&opts.Fit)
	convert.Flag("format", "Image format, valid values are jpeg, png, gif, tiff, bmp").Default("jpeg").StringVar(&opts.Format)
	convert.Flag("quality", "JPEG image quality").Default(strconv.Itoa(jpeg.DefaultQuality)).IntVar(&opts.Quality)
	convert.Flag("filter", "0=NearestNeighbor, 1=Box, 2=Linear, 3=MitchellNetravali, 4=CatmullRom, 6=Gaussian, 7=Lanczos").Default(strconv.Itoa(cbconvert.Linear)).IntVar(&opts.Filter)
	convert.Flag("cover", "Convert cover image (use --no-cover if you want to exclude cover)").Default("true").BoolVar(&opts.ConvertCover)
	convert.Flag("rgb", "Convert images that have RGB colorspace (use --no-rgb if you only want to convert grayscaled images)").Default("true").BoolVar(&opts.RGB)
	convert.Flag("nonimage", "Leave non image files in archive (use --no-nonimage to remove non image files from archive)").Default("true").BoolVar(&opts.NonImage)
	convert.Flag("grayscale", "Convert images to grayscale (monochromatic)").BoolVar(&opts.Grayscale)
	convert.Flag("rotate", "Rotate images, valid values are 0, 90, 180, 270").Default(strconv.Itoa(0)).IntVar(&opts.Rotate)
	convert.Flag("flip", "Flip images, valid values are none, horizontal, vertical").Default("none").StringVar(&opts.Flip)
	convert.Flag("brightness", "Adjust brightness of the images, must be in range (-100, 100)").Default(strconv.Itoa(0)).Float64Var(&opts.Brightness)
	convert.Flag("contrast", "Adjust contrast of the images, must be in range (-100, 100)").Default(strconv.Itoa(0)).Float64Var(&opts.Contrast)
	convert.Flag("suffix", "Add suffix to file basename").StringVar(&opts.Suffix)
	convert.Flag("levels-inmin", "Shadow input value").Default(strconv.Itoa(0)).Float64Var(&opts.LevelsInMin)
	convert.Flag("levels-gamma", "Midpoint/Gamma").Default(strconv.Itoa(1.00)).Float64Var(&opts.LevelsGamma)
	convert.Flag("levels-inmax", "Highlight input value").Default(strconv.Itoa(255)).Float64Var(&opts.LevelsInMax)
	convert.Flag("levels-outmin", "Shadow output value").Default(strconv.Itoa(0)).Float64Var(&opts.LevelsOutMin)
	convert.Flag("levels-outmax", "Highlight output value").Default(strconv.Itoa(255)).Float64Var(&opts.LevelsOutMax)

	cover := kingpin.Command("cover", "Extract cover")
	cover.Arg("args", "filename or directory").Required().ExistingFilesOrDirsVar(&args)
	cover.Flag("width", "Image width").Default(strconv.Itoa(0)).IntVar(&opts.Width)
	cover.Flag("height", "Image height").Default(strconv.Itoa(0)).IntVar(&opts.Height)
	cover.Flag("fit", "Best fit for required width and height").BoolVar(&opts.Fit)
	cover.Flag("quality", "JPEG image quality").Default(strconv.Itoa(jpeg.DefaultQuality)).IntVar(&opts.Quality)
	cover.Flag("filter", "0=NearestNeighbor, 1=Box, 2=Linear, 3=MitchellNetravali, 4=CatmullRom, 6=Gaussian, 7=Lanczos").Default(strconv.Itoa(cbconvert.Linear)).IntVar(&opts.Filter)

	thumbnail := kingpin.Command("thumbnail", "Extract cover thumbnail (freedesktop spec.)")
	thumbnail.Arg("args", "filename or directory").Required().ExistingFilesOrDirsVar(&args)
	thumbnail.Flag("width", "Image width").Default(strconv.Itoa(0)).IntVar(&opts.Width)
	thumbnail.Flag("height", "Image height").Default(strconv.Itoa(0)).IntVar(&opts.Height)
	thumbnail.Flag("fit", "Best fit for required width and height").BoolVar(&opts.Fit)
	thumbnail.Flag("filter", "0=NearestNeighbor, 1=Box, 2=Linear, 3=MitchellNetravali, 4=CatmullRom, 6=Gaussian, 7=Lanczos").Default(strconv.Itoa(cbconvert.Linear)).IntVar(&opts.Filter)

	switch kingpin.Parse() {
	case "cover":
		opts.Cover = true
	case "thumbnail":
		opts.Thumbnail = true
	}

	return opts, args
}

func main() {
	opts, args := parseFlags()
	conv := cbconvert.NewConvertor(opts)

	var bar *pb.ProgressBar

	c := make(chan os.Signal, 3)
	signal.Notify(c, os.Interrupt, syscall.SIGHUP, syscall.SIGTERM)
	go func() {
		for _ = range c {
			fmt.Fprintf(os.Stderr, "Aborting\n")
			os.RemoveAll(conv.Workdir)
			os.Exit(1)
		}
	}()

	if _, err := os.Stat(opts.Outdir); err != nil {
		os.MkdirAll(opts.Outdir, 0777)
	}

	files := conv.GetFiles(args)

	if opts.Cover || opts.Thumbnail {
		if !opts.Quiet {
			bar = pb.New(conv.Nfiles)
			bar.ShowTimeLeft = false
			bar.Start()
		}
	}

	for _, file := range files {
		stat, err := os.Stat(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error Stat: %v\n", err.Error())
			continue
		}

		if opts.Cover {
			conv.ExtractCover(file, stat)
			if !opts.Quiet {
				bar.Increment()
			}
			continue
		} else if opts.Thumbnail {
			conv.ExtractThumbnail(file, stat)
			if !opts.Quiet {
				bar.Increment()
			}
			continue
		}

		conv.ConvertComic(file, stat)
	}
}
