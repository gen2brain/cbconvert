package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/gen2brain/cbconvert"
	"github.com/schollz/progressbar/v3"
	flag "github.com/spf13/pflag"
)

func main() {
	opts, args := parseFlags()
	conv := cbconvert.New(opts)

	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		for range c {
			fmt.Println("\naborting")
			err := os.RemoveAll(conv.Workdir)
			if err != nil {
				fmt.Println(err)
			}
			os.Exit(1)
		}
	}()

	if _, err := os.Stat(opts.Outdir); err != nil {
		err = os.MkdirAll(opts.Outdir, 0775)
		if err != nil {
			fmt.Println(err)
		}
		os.Exit(1)
	}

	files, err := conv.Files(args)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	var bar *progressbar.ProgressBar
	if opts.Cover || opts.Thumbnail {
		if !opts.Quiet {
			bar = progressbar.NewOptions(conv.Nfiles,
				progressbar.OptionShowCount(),
				progressbar.OptionClearOnFinish(),
				progressbar.OptionUseANSICodes(true),
				progressbar.OptionSetPredictTime(false),
			)
		}
	}

	conv.OnStart = func() {
		if !opts.Quiet {
			bar = progressbar.NewOptions(conv.Ncontents,
				progressbar.OptionShowCount(),
				progressbar.OptionClearOnFinish(),
				progressbar.OptionUseANSICodes(true),
				progressbar.OptionSetDescription(fmt.Sprintf("Converting %d of %d:", conv.CurrFile, conv.Nfiles)),
				progressbar.OptionSetPredictTime(false),
			)
		}
	}

	conv.OnProgress = func() {
		if !opts.Quiet {
			_ = bar.Add(1)
		}
	}

	conv.OnCompress = func() {
		if !opts.Quiet {
			_, _ = fmt.Fprintf(os.Stderr, "Compressing %d of %d...\r", conv.CurrFile, conv.Nfiles)
		}
	}

	for _, file := range files {
		stat, err := os.Stat(file)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		if opts.Cover {
			err = conv.ExtractCover(file, stat)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		} else if opts.Thumbnail {
			err = conv.ExtractThumbnail(file, stat)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		}

		err = conv.Convert(file, stat)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}
}

// parseFlags parses command line flags
func parseFlags() (cbconvert.Options, []string) {
	opts := cbconvert.Options{}
	var args []string

	convert := flag.NewFlagSet("convert", flag.ExitOnError)
	convert.SortFlags = false
	convert.IntVar(&opts.Width, "width", 0, "Image width")
	convert.IntVar(&opts.Height, "height", 0, "Image height")
	convert.BoolVar(&opts.Fit, "fit", false, "Best fit for required width and height")
	convert.StringVar(&opts.Format, "format", "jpeg", "Image format, valid values are jpeg, png, tiff, bmp, webp")
	convert.IntVar(&opts.Quality, "quality", 75, "Image quality")
	convert.IntVar(&opts.Filter, "filter", 2, "0=NearestNeighbor, 1=Box, 2=Linear, 3=MitchellNetravali, 4=CatmullRom, 6=Gaussian, 7=Lanczos")
	convert.BoolVar(&opts.NoCover, "no-cover", false, "Do not convert the cover image")
	convert.BoolVar(&opts.NoRGB, "no-rgb", false, "Do not convert images that have RGB colorspace")
	convert.BoolVar(&opts.NoNonImage, "no-nonimage", false, "Remove non-image files from the archive")
	convert.BoolVar(&opts.NoConvert, "no-convert", false, "Do not transform or convert images")
	convert.BoolVar(&opts.Grayscale, "grayscale", false, "Convert images to grayscale (monochromatic)")
	convert.IntVar(&opts.Rotate, "rotate", 0, "Rotate images, valid values are 0, 90, 180, 270")
	convert.StringVar(&opts.Flip, "flip", "none", "Flip images, valid values are none, horizontal, vertical")
	convert.Float64Var(&opts.Brightness, "brightness", 0, "Adjust the brightness of the images, must be in the range (-100, 100)")
	convert.Float64Var(&opts.Contrast, "contrast", 0, "Adjust the contrast of the images, must be in the range (-100, 100)")
	convert.StringVar(&opts.Suffix, "suffix", "", "Add suffix to file basename")
	convert.Float64Var(&opts.LevelsInMin, "levels-inmin", 0, "Shadow input value")
	convert.Float64Var(&opts.LevelsGamma, "levels-gamma", 1.0, "Midpoint/Gamma")
	convert.Float64Var(&opts.LevelsInMax, "levels-inmax", 255, "Highlight input value")
	convert.Float64Var(&opts.LevelsOutMin, "levels-outmin", 0, "Shadow output value")
	convert.Float64Var(&opts.LevelsOutMax, "levels-outmax", 255, "Highlight output value")
	convert.StringVar(&opts.Outdir, "outdir", ".", "Output directory")
	convert.Int64Var(&opts.Size, "size", 0, "Process only files larger than size (in MB)")
	convert.BoolVar(&opts.Recursive, "recursive", false, "Process subdirectories recursively")
	convert.BoolVar(&opts.Quiet, "quiet", false, "Hide console output")

	cover := flag.NewFlagSet("cover", flag.ExitOnError)
	cover.SortFlags = false
	cover.IntVar(&opts.Width, "width", 0, "Image width")
	cover.IntVar(&opts.Height, "height", 0, "Image height")
	cover.BoolVar(&opts.Fit, "fit", false, "Best fit for required width and height")
	cover.IntVar(&opts.Quality, "quality", 75, "Image quality")
	cover.IntVar(&opts.Filter, "filter", 2, "0=NearestNeighbor, 1=Box, 2=Linear, 3=MitchellNetravali, 4=CatmullRom, 6=Gaussian, 7=Lanczos")
	cover.StringVar(&opts.Outdir, "outdir", ".", "Output directory")
	cover.Int64Var(&opts.Size, "size", 0, "Process only files larger than size (in MB)")
	cover.BoolVar(&opts.Recursive, "recursive", false, "Process subdirectories recursively")
	cover.BoolVar(&opts.Quiet, "quiet", false, "Hide console output")

	thumbnail := flag.NewFlagSet("thumbnail", flag.ExitOnError)
	thumbnail.SortFlags = false
	thumbnail.IntVar(&opts.Width, "width", 0, "Image width")
	thumbnail.IntVar(&opts.Height, "height", 0, "Image height")
	thumbnail.BoolVar(&opts.Fit, "fit", false, "Best fit for required width and height")
	thumbnail.IntVar(&opts.Filter, "filter", 2, "0=NearestNeighbor, 1=Box, 2=Linear, 3=MitchellNetravali, 4=CatmullRom, 6=Gaussian, 7=Lanczos")
	thumbnail.StringVar(&opts.Outdir, "outdir", ".", "Output directory")
	thumbnail.StringVar(&opts.Outfile, "outfile", "", "Output file")
	thumbnail.Int64Var(&opts.Size, "size", 0, "Process only files larger than size (in MB)")
	thumbnail.BoolVar(&opts.Recursive, "recursive", false, "Process subdirectories recursively")
	thumbnail.BoolVar(&opts.Quiet, "quiet", false, "Hide console output")

	convert.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr, "Usage: %s <command> [<flags>] [file1 dir1 ... fileOrDirN]\n\n", filepath.Base(os.Args[0]))
		_, _ = fmt.Fprintf(os.Stderr, "\nCommands:\n")
		_, _ = fmt.Fprintf(os.Stderr, "\n  convert*\n    \tConvert archive or document (default command)\n\n")
		convert.VisitAll(func(f *flag.Flag) {
			_, _ = fmt.Fprintf(os.Stderr, "    --%s", f.Name)
			_, _ = fmt.Fprintf(os.Stderr, "\n    \t")
			_, _ = fmt.Fprintf(os.Stderr, "%v (default %q)\n", f.Usage, f.DefValue)
		})
		_, _ = fmt.Fprintf(os.Stderr, "\n  cover\n    \tExtract cover\n\n")
		cover.VisitAll(func(f *flag.Flag) {
			_, _ = fmt.Fprintf(os.Stderr, "    --%s", f.Name)
			_, _ = fmt.Fprintf(os.Stderr, "\n    \t")
			_, _ = fmt.Fprintf(os.Stderr, "%v (default %q)\n", f.Usage, f.DefValue)
		})
		_, _ = fmt.Fprintf(os.Stderr, "\n  thumbnail\n    \tExtract cover thumbnail (freedesktop spec.)\n\n")
		thumbnail.VisitAll(func(f *flag.Flag) {
			_, _ = fmt.Fprintf(os.Stderr, "    --%s", f.Name)
			_, _ = fmt.Fprintf(os.Stderr, "\n    \t")
			_, _ = fmt.Fprintf(os.Stderr, "%v (default %q)\n", f.Usage, f.DefValue)
		})
		_, _ = fmt.Fprintf(os.Stderr, "\n")
	}

	if len(os.Args) < 2 {
		convert.Usage()
		_, _ = fmt.Fprintf(os.Stderr, "no arguments\n")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "convert":
		_ = convert.Parse(os.Args[2:])
		args = convert.Args()
	case "cover":
		opts.Cover = true
		_ = cover.Parse(os.Args[2:])
		args = cover.Args()
	case "thumbnail":
		opts.Thumbnail = true
		_ = thumbnail.Parse(os.Args[2:])
		args = thumbnail.Args()
	default:
		_ = convert.Parse(os.Args[1:])
		args = convert.Args()
	}

	if len(args) == 0 {
		convert.Usage()
		_, _ = fmt.Fprintf(os.Stderr, "no arguments\n")
		os.Exit(1)
	}

	return opts, args
}
