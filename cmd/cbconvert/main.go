package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"syscall"

	"github.com/gen2brain/cbconvert"
	pb "github.com/schollz/progressbar/v3"
)

var appVersion string

func init() {
	if appVersion != "" {
		return
	}

	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}

	if info.Main.Version != "" {
		appVersion = info.Main.Version
	}

	for _, kv := range info.Settings {
		if kv.Value == "" {
			continue
		}
		if kv.Key == "vcs.revision" {
			appVersion = kv.Value
			if len(appVersion) > 10 {
				appVersion = kv.Value[:10]
			}
		}
	}
}

func main() {
	opts, args := parseFlags()

	if opts.Version {
		fmt.Println(filepath.Base(os.Args[0]), appVersion)
		os.Exit(0)
	}

	conv := cbconvert.New(opts)

	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		for range c {
			if err := os.RemoveAll(conv.Workdir); err != nil {
				fmt.Println(err)
			}
			os.Exit(1)
		}
	}()

	if _, err := os.Stat(opts.OutDir); err != nil {
		if err := os.MkdirAll(opts.OutDir, 0775); err != nil {
			fmt.Println(err)
		}
		os.Exit(1)
	}

	files, err := conv.Files(args)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	var bar *pb.ProgressBar
	if opts.Cover || opts.Thumbnail || opts.Meta {
		if !opts.Quiet {
			bar = pb.NewOptions(conv.Nfiles,
				pb.OptionShowCount(),
				pb.OptionClearOnFinish(),
				pb.OptionUseANSICodes(true),
				pb.OptionSetPredictTime(false),
			)
		}
	}

	conv.OnStart = func() {
		if !opts.Quiet {
			bar = pb.NewOptions(conv.Ncontents,
				pb.OptionShowCount(),
				pb.OptionClearOnFinish(),
				pb.OptionUseANSICodes(true),
				pb.OptionSetDescription(fmt.Sprintf("Converting %d of %d:", conv.CurrFile, conv.Nfiles)),
				pb.OptionSetPredictTime(false),
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
			fmt.Fprintf(os.Stderr, "Compressing %d of %d...\r", conv.CurrFile, conv.Nfiles)
		}
	}

	for _, file := range files {
		switch {
		case opts.Meta:
			ret, err := conv.Meta(file.Path)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}

			if opts.Cover {
				fmt.Println(ret)
			} else if opts.Comment {
				fmt.Println(ret)
			}

			continue
		case opts.Cover:
			if err := conv.Cover(file.Path, file.Stat); err != nil {
				fmt.Println(err)
				os.Exit(1)
			}

			continue
		case opts.Thumbnail:
			if err = conv.Thumbnail(file.Path, file.Stat); err != nil {
				fmt.Println(err)
				os.Exit(1)
			}

			continue
		}

		if err := conv.Convert(file.Path, file.Stat); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}

	fmt.Fprintf(os.Stderr, "\r")
}

// parseFlags parses command line flags.
func parseFlags() (cbconvert.Options, []string) {
	opts := cbconvert.Options{}
	var args []string

	convert := flag.NewFlagSet("convert", flag.ExitOnError)
	convert.IntVar(&opts.Width, "width", 0, "Image width")
	convert.IntVar(&opts.Height, "height", 0, "Image height")
	convert.BoolVar(&opts.Fit, "fit", false, "Best fit for required width and height")
	convert.StringVar(&opts.Format, "format", "jpeg", "Image format, valid values are jpeg, png, tiff, bmp, webp, avif, jxl")
	convert.StringVar(&opts.Archive, "archive", "zip", "Archive format, valid values are zip, tar")
	convert.IntVar(&opts.Quality, "quality", 75, "Image quality")
	convert.IntVar(&opts.Filter, "filter", 2, "0=NearestNeighbor, 1=Box, 2=Linear, 3=MitchellNetravali, 4=CatmullRom, 6=Gaussian, 7=Lanczos")
	convert.BoolVar(&opts.NoCover, "no-cover", false, "Do not convert the cover image")
	convert.BoolVar(&opts.NoRGB, "no-rgb", false, "Do not convert images that have RGB colorspace")
	convert.BoolVar(&opts.NoNonImage, "no-nonimage", false, "Remove non-image files from the archive")
	convert.BoolVar(&opts.NoConvert, "no-convert", false, "Do not transform or convert images")
	convert.BoolVar(&opts.Grayscale, "grayscale", false, "Convert images to grayscale (monochromatic)")
	convert.IntVar(&opts.Rotate, "rotate", 0, "Rotate images, valid values are 0, 90, 180, 270")
	convert.IntVar(&opts.Brightness, "brightness", 0, "Adjust the brightness of the images, must be in the range (-100, 100)")
	convert.IntVar(&opts.Contrast, "contrast", 0, "Adjust the contrast of the images, must be in the range (-100, 100)")
	convert.StringVar(&opts.Suffix, "suffix", "", "Add suffix to file basename")
	convert.StringVar(&opts.OutDir, "outdir", ".", "Output directory")
	convert.IntVar(&opts.Size, "size", 0, "Process only files larger than size (in MB)")
	convert.BoolVar(&opts.Recursive, "recursive", false, "Process subdirectories recursively")
	convert.BoolVar(&opts.Quiet, "quiet", false, "Hide console output")

	cover := flag.NewFlagSet("cover", flag.ExitOnError)
	cover.IntVar(&opts.Width, "width", 0, "Image width")
	cover.IntVar(&opts.Height, "height", 0, "Image height")
	cover.BoolVar(&opts.Fit, "fit", false, "Best fit for required width and height")
	cover.StringVar(&opts.Format, "format", "jpeg", "Image format, valid values are jpeg, png, tiff, bmp, webp, avif")
	cover.IntVar(&opts.Quality, "quality", 75, "Image quality")
	cover.IntVar(&opts.Filter, "filter", 2, "0=NearestNeighbor, 1=Box, 2=Linear, 3=MitchellNetravali, 4=CatmullRom, 6=Gaussian, 7=Lanczos")
	cover.StringVar(&opts.OutDir, "outdir", ".", "Output directory")
	cover.IntVar(&opts.Size, "size", 0, "Process only files larger than size (in MB)")
	cover.BoolVar(&opts.Recursive, "recursive", false, "Process subdirectories recursively")
	cover.BoolVar(&opts.Quiet, "quiet", false, "Hide console output")

	thumbnail := flag.NewFlagSet("thumbnail", flag.ExitOnError)
	thumbnail.IntVar(&opts.Width, "width", 0, "Image width")
	thumbnail.IntVar(&opts.Height, "height", 0, "Image height")
	thumbnail.BoolVar(&opts.Fit, "fit", false, "Best fit for required width and height")
	thumbnail.IntVar(&opts.Filter, "filter", 2, "0=NearestNeighbor, 1=Box, 2=Linear, 3=MitchellNetravali, 4=CatmullRom, 6=Gaussian, 7=Lanczos")
	thumbnail.StringVar(&opts.OutDir, "outdir", ".", "Output directory")
	thumbnail.StringVar(&opts.OutFile, "outfile", "", "Output file")
	thumbnail.IntVar(&opts.Size, "size", 0, "Process only files larger than size (in MB)")
	thumbnail.BoolVar(&opts.Recursive, "recursive", false, "Process subdirectories recursively")
	thumbnail.BoolVar(&opts.Quiet, "quiet", false, "Hide console output")

	meta := flag.NewFlagSet("meta", flag.ExitOnError)
	meta.BoolVar(&opts.Cover, "cover", false, "Print cover name")
	meta.BoolVar(&opts.Comment, "comment", false, "Print zip comment")
	meta.StringVar(&opts.CommentBody, "comment-body", "", "Set zip comment")
	meta.StringVar(&opts.FileAdd, "file-add", "", "Add file to archive")
	meta.StringVar(&opts.FileRemove, "file-remove", "", "Remove file from archive (glob pattern, i.e. *.xml)")

	flag.NewFlagSet("version", flag.ExitOnError)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s <command> [<flags>] [file1 dir1 ... fileOrDirN]\n\n", filepath.Base(os.Args[0]))
		fmt.Fprintf(os.Stderr, "\nCommands:\n")
		fmt.Fprintf(os.Stderr, "\n  convert\n    \tConvert archive or document\n\n")
		order := []string{"width", "height", "fit", "format", "archive", "quality", "filter", "no-cover", "no-rgb",
			"no-nonimage", "no-convert", "grayscale", "rotate", "brightness", "contrast", "suffix", "outdir", "size", "recursive", "quiet"}
		for _, name := range order {
			f := convert.Lookup(name)
			fmt.Fprintf(os.Stderr, "    --%s\n    \t", f.Name)
			fmt.Fprintf(os.Stderr, "%v (default %q)\n", f.Usage, f.DefValue)
		}
		fmt.Fprintf(os.Stderr, "\n  cover\n    \tExtract cover\n\n")
		order = []string{"width", "height", "fit", "format", "quality", "filter", "outdir", "size", "recursive", "quiet"}
		for _, name := range order {
			f := cover.Lookup(name)
			fmt.Fprintf(os.Stderr, "    --%s\n    \t", f.Name)
			fmt.Fprintf(os.Stderr, "%v (default %q)\n", f.Usage, f.DefValue)
		}
		fmt.Fprintf(os.Stderr, "\n  thumbnail\n    \tExtract cover thumbnail (freedesktop spec.)\n\n")
		order = []string{"width", "height", "fit", "filter", "outdir", "outfile", "size", "recursive", "quiet"}
		for _, name := range order {
			f := thumbnail.Lookup(name)
			fmt.Fprintf(os.Stderr, "    --%s\n    \t", f.Name)
			fmt.Fprintf(os.Stderr, "%v (default %q)\n", f.Usage, f.DefValue)
		}
		fmt.Fprintf(os.Stderr, "\n  meta\n    \tCBZ metadata\n\n")
		order = []string{"cover", "comment", "comment-body", "file-add", "file-remove"}
		for _, name := range order {
			f := meta.Lookup(name)
			fmt.Fprintf(os.Stderr, "    --%s\n    \t", f.Name)
			fmt.Fprintf(os.Stderr, "%v (default %q)\n", f.Usage, f.DefValue)
		}
		fmt.Fprintf(os.Stderr, "\n  version\n    \tPrint version\n\n")
	}

	if len(os.Args) < 2 {
		flag.Usage()
		fmt.Fprintf(os.Stderr, "no command\n")
		os.Exit(1)
	}

	pipe := piped()
	if pipe {
		args = lines(os.Stdin)
	}

	switch os.Args[1] {
	case "convert":
		_ = convert.Parse(os.Args[2:])
		if !pipe {
			args = convert.Args()
		}
	case "cover":
		opts.Cover = true
		_ = cover.Parse(os.Args[2:])
		if !pipe {
			args = cover.Args()
		}
	case "thumbnail":
		opts.Thumbnail = true
		_ = thumbnail.Parse(os.Args[2:])
		if !pipe {
			args = thumbnail.Args()
		}
	case "meta":
		opts.Meta = true
		_ = meta.Parse(os.Args[2:])
		if !pipe {
			args = meta.Args()
		}
	case "version":
		opts.Version = true
	}

	if len(args) == 0 && !opts.Version {
		flag.Usage()
		_, _ = fmt.Fprintf(os.Stderr, "no arguments\n")
		os.Exit(1)
	}

	return opts, args
}

// piped checks if we have a piped stdin.
func piped() bool {
	f, err := os.Stdin.Stat()
	if err != nil {
		return false
	}

	if f.Mode()&os.ModeNamedPipe == 0 {
		return false
	}

	return true
}

// lines returns slice of lines from reader.
func lines(r io.Reader) []string {
	data := make([]string, 0)
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		text := scanner.Text()
		data = append(data, text)
	}

	return data
}
