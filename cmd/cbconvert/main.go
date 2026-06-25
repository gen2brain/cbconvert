package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"

	"github.com/gen2brain/cbconvert"
	pb "github.com/schollz/progressbar/v3"
	"golang.org/x/term"
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
			os.Exit(1)
		}
	}

	files, err := conv.Files(args)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	interactive := !opts.Quiet && term.IsTerminal(int(os.Stderr.Fd()))

	var bar *pb.ProgressBar
	newBar := func(max int, description string) {
		bar = pb.NewOptions(max,
			pb.OptionShowCount(),
			pb.OptionClearOnFinish(),
			pb.OptionUseANSICodes(true),
			pb.OptionSetPredictTime(false),
			pb.OptionSetDescription(description),
		)
	}

	clearLine := func() {
		fmt.Fprint(os.Stderr, "\033[2K\r")
	}

	cleanup := func() {
		if interactive {
			if bar != nil {
				_ = bar.Finish()
			}
			clearLine()
		}
	}

	if interactive && (opts.Cover || opts.Thumbnail || opts.Meta) {
		newBar(conv.Nfiles, "")
	}

	conv.OnStart = func() {
		if interactive {
			clearLine()
			newBar(conv.Ncontents, fmt.Sprintf("Converting %d of %d:", conv.CurrFile, conv.Nfiles))
		}
	}

	conv.OnProgress = func() {
		if bar != nil {
			_ = bar.Add(1)
		}
	}

	conv.OnCompress = func() {
		if interactive {
			if bar != nil {
				_ = bar.Finish()
			}
			fmt.Fprintf(os.Stderr, "Compressing %d of %d...", conv.CurrFile, conv.Nfiles)
		}
	}

	if opts.Combine {
		if err := conv.Combine(files); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		cleanup()

		return
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
			if err := conv.Cover(file); err != nil {
				fmt.Println(err)
				os.Exit(1)
			}

			continue
		case opts.Thumbnail:
			if err = conv.Thumbnail(file); err != nil {
				fmt.Println(err)
				os.Exit(1)
			}

			continue
		}

		if err := conv.Convert(file); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}

	cleanup()
}

// parseFlags parses command line flags.
func parseFlags() (cbconvert.Options, []string) {
	opts := cbconvert.Options{}
	var args []string

	base := defaultOptions()
	if len(os.Args) >= 2 {
		switch os.Args[1] {
		case "convert", "cover", "thumbnail":
			if name := profileArg(os.Args[2:]); name != "" {
				o, err := loadProfile(name)
				if err != nil {
					fmt.Println(err)
					os.Exit(1)
				}
				base = o
			}
		}
	}

	var profile string
	const profileUsage = "Load a saved GUI profile as defaults; explicit flags still override"

	convert := flag.NewFlagSet("convert", flag.ExitOnError)
	convert.StringVar(&profile, "profile", "", profileUsage)
	convert.IntVar(&opts.Width, "width", base.Width, "Image width")
	convert.IntVar(&opts.Height, "height", base.Height, "Image height")
	convert.BoolVar(&opts.Fit, "fit", base.Fit, "Best fit for required width and height")
	convert.BoolVar(&opts.NoUpscale, "no-upscale", base.NoUpscale, "Do not upscale images already smaller than the requested width/height")
	convert.IntVar(&opts.DPI, "dpi", base.DPI, "Document rendering resolution in DPI (PDF, EPUB, etc.), 0 uses the default (300)")
	convert.StringVar(&opts.Format, "format", base.Format, "Image format, valid values are jpeg, png, tiff, bmp, webp, avif, jxl")
	convert.StringVar(&opts.Archive, "archive", base.Archive, "Archive format, valid values are zip, tar")
	convert.IntVar(&opts.ZipLevel, "zip-level", base.ZipLevel, "ZIP compression level, 0 disables compression, 1-9 sets deflate level (1 fastest, 9 smallest), -1 uses the default")
	convert.IntVar(&opts.Quality, "quality", base.Quality, "Image quality")
	convert.IntVar(&opts.Effort, "effort", base.Effort, "Encoder speed/effort, format-specific (webp method 0-6, avif speed 0-10, jxl effort 1-10), -1 uses the format default")
	convert.BoolVar(&opts.Lossless, "lossless", base.Lossless, "Lossless compression (webp, avif, jxl), ignores quality")
	convert.BoolVar(&opts.Combine, "combine", base.Combine, "Combine all inputs into a single archive")
	convert.StringVar(&opts.OutFile, "outfile", base.OutFile, "Output file name for --combine (default: first input + -combined)")
	convert.IntVar(&opts.Filter, "filter", base.Filter, "0=NearestNeighbor, 1=Box, 2=Linear, 3=MitchellNetravali, 4=CatmullRom, 5=Gaussian, 6=Lanczos")
	convert.BoolVar(&opts.NoCover, "no-cover", base.NoCover, "Do not convert the cover image")
	convert.BoolVar(&opts.NoRGB, "no-rgb", base.NoRGB, "Do not convert images that have RGB colorspace")
	convert.BoolVar(&opts.NoNonImage, "no-nonimage", base.NoNonImage, "Remove non-image files from the archive")
	convert.BoolVar(&opts.NoConvert, "no-convert", base.NoConvert, "Do not transform or convert images")
	convert.BoolVar(&opts.Grayscale, "grayscale", base.Grayscale, "Convert images to grayscale (monochromatic)")
	convert.IntVar(&opts.Rotate, "rotate", base.Rotate, "Rotate images, valid values are 0, 90, 180, 270")
	convert.IntVar(&opts.Brightness, "brightness", base.Brightness, "Adjust the brightness of the images, must be in the range (-100, 100)")
	convert.IntVar(&opts.Contrast, "contrast", base.Contrast, "Adjust the contrast of the images, must be in the range (-100, 100)")
	convert.StringVar(&opts.Suffix, "suffix", base.Suffix, "Add suffix to file basename")
	convert.StringVar(&opts.OutDir, "outdir", base.OutDir, "Output directory")
	convert.IntVar(&opts.Size, "size", base.Size, "Process only files larger than size (in MB)")
	convert.BoolVar(&opts.Recursive, "recursive", base.Recursive, "Process subdirectories recursively")
	convert.BoolVar(&opts.Quiet, "quiet", base.Quiet, "Hide console output")

	cover := flag.NewFlagSet("cover", flag.ExitOnError)
	cover.StringVar(&profile, "profile", "", profileUsage)
	cover.IntVar(&opts.Width, "width", base.Width, "Image width")
	cover.IntVar(&opts.Height, "height", base.Height, "Image height")
	cover.BoolVar(&opts.Fit, "fit", base.Fit, "Best fit for required width and height")
	cover.BoolVar(&opts.NoUpscale, "no-upscale", base.NoUpscale, "Do not upscale images already smaller than the requested width/height")
	cover.IntVar(&opts.DPI, "dpi", base.DPI, "Document rendering resolution in DPI (PDF, EPUB, etc.), 0 uses the default (300)")
	cover.StringVar(&opts.Format, "format", base.Format, "Image format, valid values are jpeg, png, tiff, bmp, webp, avif")
	cover.IntVar(&opts.Quality, "quality", base.Quality, "Image quality")
	cover.IntVar(&opts.Effort, "effort", base.Effort, "Encoder speed/effort, format-specific (webp method 0-6, avif speed 0-10, jxl effort 1-10), -1 uses the format default")
	cover.BoolVar(&opts.Lossless, "lossless", base.Lossless, "Lossless compression (webp, avif, jxl), ignores quality")
	cover.IntVar(&opts.Filter, "filter", base.Filter, "0=NearestNeighbor, 1=Box, 2=Linear, 3=MitchellNetravali, 4=CatmullRom, 5=Gaussian, 6=Lanczos")
	cover.StringVar(&opts.OutDir, "outdir", base.OutDir, "Output directory")
	cover.IntVar(&opts.Size, "size", base.Size, "Process only files larger than size (in MB)")
	cover.BoolVar(&opts.Recursive, "recursive", base.Recursive, "Process subdirectories recursively")
	cover.BoolVar(&opts.Quiet, "quiet", base.Quiet, "Hide console output")

	thumbnail := flag.NewFlagSet("thumbnail", flag.ExitOnError)
	thumbnail.StringVar(&profile, "profile", "", profileUsage)
	thumbnail.IntVar(&opts.Width, "width", base.Width, "Image width")
	thumbnail.IntVar(&opts.Height, "height", base.Height, "Image height")
	thumbnail.BoolVar(&opts.Fit, "fit", base.Fit, "Best fit for required width and height")
	thumbnail.BoolVar(&opts.NoUpscale, "no-upscale", base.NoUpscale, "Do not upscale images already smaller than the requested width/height")
	thumbnail.IntVar(&opts.DPI, "dpi", base.DPI, "Document rendering resolution in DPI (PDF, EPUB, etc.), 0 uses the default (300)")
	thumbnail.IntVar(&opts.Filter, "filter", base.Filter, "0=NearestNeighbor, 1=Box, 2=Linear, 3=MitchellNetravali, 4=CatmullRom, 5=Gaussian, 6=Lanczos")
	thumbnail.StringVar(&opts.OutDir, "outdir", base.OutDir, "Output directory")
	thumbnail.StringVar(&opts.OutFile, "outfile", base.OutFile, "Output file")
	thumbnail.IntVar(&opts.Size, "size", base.Size, "Process only files larger than size (in MB)")
	thumbnail.BoolVar(&opts.Recursive, "recursive", base.Recursive, "Process subdirectories recursively")
	thumbnail.BoolVar(&opts.Quiet, "quiet", base.Quiet, "Hide console output")

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
		order := []string{"profile", "width", "height", "fit", "no-upscale", "dpi", "format", "archive", "zip-level", "quality", "effort", "lossless", "combine", "outfile", "filter", "no-cover", "no-rgb",
			"no-nonimage", "no-convert", "grayscale", "rotate", "brightness", "contrast", "suffix", "outdir", "size", "recursive", "quiet"}
		for _, name := range order {
			f := convert.Lookup(name)
			fmt.Fprintf(os.Stderr, "    --%s\n    \t", f.Name)
			fmt.Fprintf(os.Stderr, "%v (default %q)\n", f.Usage, f.DefValue)
		}
		fmt.Fprintf(os.Stderr, "\n  cover\n    \tExtract cover\n\n")
		order = []string{"profile", "width", "height", "fit", "no-upscale", "dpi", "format", "quality", "effort", "lossless", "filter", "outdir", "size", "recursive", "quiet"}
		for _, name := range order {
			f := cover.Lookup(name)
			fmt.Fprintf(os.Stderr, "    --%s\n    \t", f.Name)
			fmt.Fprintf(os.Stderr, "%v (default %q)\n", f.Usage, f.DefValue)
		}
		fmt.Fprintf(os.Stderr, "\n  thumbnail\n    \tExtract cover thumbnail (freedesktop spec.)\n\n")
		order = []string{"profile", "width", "height", "fit", "no-upscale", "dpi", "filter", "outdir", "outfile", "size", "recursive", "quiet"}
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
		operands := parseArgs(convert, os.Args[2:])
		if !pipe {
			args = operands
		}
	case "cover":
		opts.Cover = true
		operands := parseArgs(cover, os.Args[2:])
		if !pipe {
			args = operands
		}
	case "thumbnail":
		opts.Thumbnail = true
		operands := parseArgs(thumbnail, os.Args[2:])
		if !pipe {
			args = operands
		}
	case "meta":
		opts.Meta = true
		operands := parseArgs(meta, os.Args[2:])
		if !pipe {
			args = operands
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

// parseArgs parses flags interspersed with file/dir operands.
func parseArgs(fs *flag.FlagSet, args []string) []string {
	var operands []string

	_ = fs.Parse(args)
	for fs.NArg() > 0 {
		operands = append(operands, fs.Arg(0))
		_ = fs.Parse(fs.Args()[1:])
	}

	return operands
}

// piped checks if we have piped stdin.
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

// lines returns slice of lines from the reader.
func lines(r io.Reader) []string {
	data := make([]string, 0)
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		text := scanner.Text()
		data = append(data, text)
	}

	return data
}

// configPath returns the IupConfig file the GUI writes, matching IUP's per-platform location for APPNAME "cbconvert".
func configPath() (string, error) {
	if runtime.GOOS == "windows" {
		dir := os.Getenv("LocalAppData")
		if dir == "" {
			return "", errors.New("configPath: LocalAppData is not set")
		}

		return filepath.Join(dir, "cbconvert", "config.cfg"), nil
	}

	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("configPath: %w", err)
	}

	return filepath.Join(dir, "cbconvert", "config"), nil
}

// parseINI reads a simple INI file into section -> key -> value.
func parseINI(path string) (map[string]map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sections := make(map[string]map[string]string)
	var cur map[string]string

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			cur = make(map[string]string)
			sections[line[1:len(line)-1]] = cur

			continue
		}

		if cur == nil {
			continue
		}

		if k, v, ok := strings.Cut(line, "="); ok {
			cur[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}

	if err := sc.Err(); err != nil {
		return nil, err
	}

	return sections, nil
}

// defaultOptions returns the convert defaults used when no profile is loaded.
func defaultOptions() cbconvert.Options {
	o := cbconvert.NewOptions()
	o.OutDir = "."

	return o
}

// loadProfile reads the named GUI profile and translates its control values into Options.
func loadProfile(name string) (cbconvert.Options, error) {
	o := defaultOptions()

	path, err := configPath()
	if err != nil {
		return o, fmt.Errorf("loadProfile: %w", err)
	}

	ini, err := parseINI(path)
	if err != nil {
		return o, fmt.Errorf("loadProfile: %w", err)
	}

	sec, ok := ini["Profile:"+name]
	if !ok {
		return o, fmt.Errorf("loadProfile: profile %q not found in %s%s", name, path, knownProfiles(ini))
	}

	str := func(key string, set func(string)) {
		if v, ok := sec[key]; ok {
			set(v)
		}
	}
	boolean := func(key string, set func(bool)) {
		if v, ok := sec[key]; ok {
			set(v == "1")
		}
	}
	integer := func(key string, set func(int)) {
		if v, ok := sec[key]; ok {
			if n, err := strconv.Atoi(v); err == nil {
				set(n)
			}
		}
	}

	integer("Width", func(n int) { o.Width = n })
	integer("Height", func(n int) { o.Height = n })
	boolean("Fit", func(b bool) { o.Fit = b })
	boolean("NoUpscale", func(b bool) { o.NoUpscale = b })
	str("DPI", func(v string) { o.DPI = dpiFromString(v) })
	str("Format", func(v string) { o.Format = formatFromIndex(v) })
	str("Archive", func(v string) { o.Archive = archiveFromIndex(v) })
	str("ZipLevel", func(v string) { o.ZipLevel = zipLevelFromIndex(v) })
	integer("Quality", func(n int) { o.Quality = n })
	integer("Effort", func(n int) { o.Effort = n })
	boolean("Lossless", func(b bool) { o.Lossless = b })
	boolean("Combine", func(b bool) { o.Combine = b })
	integer("Filter", func(n int) { o.Filter = n - 1 })
	boolean("NoCover", func(b bool) { o.NoCover = b })
	boolean("NoRGB", func(b bool) { o.NoRGB = b })
	boolean("NoNonImage", func(b bool) { o.NoNonImage = b })
	boolean("NoConvert", func(b bool) { o.NoConvert = b })
	boolean("Grayscale", func(b bool) { o.Grayscale = b })
	str("Rotate", func(v string) { o.Rotate = rotateFromIndex(v) })
	integer("Brightness", func(n int) { o.Brightness = n })
	integer("Contrast", func(n int) { o.Contrast = n })
	str("Suffix", func(v string) { o.Suffix = v })
	str("OutDir", func(v string) { o.OutDir = v })
	integer("Size", func(n int) { o.Size = n })
	boolean("Recursive", func(b bool) { o.Recursive = b })

	// Effort is format-specific in the GUI: only webp/avif/jxl use the slider, others fall back to the format default.
	switch o.Format {
	case "webp", "avif", "jxl":
	default:
		o.Effort = -1
	}

	return o, nil
}

// knownProfiles lists the profile names from the config, for a helpful "not found" message.
func knownProfiles(ini map[string]map[string]string) string {
	if p, ok := ini["Profiles"]; ok {
		if names := p["Names"]; names != "" {
			return "\navailable profiles: " + strings.ReplaceAll(names, ";", ", ")
		}
	}

	return ""
}

// profileArg extracts the --profile value from args, since it must be known before flag defaults are built.
func profileArg(args []string) string {
	for i := 0; i < len(args); i++ {
		if args[i] == "--profile" || args[i] == "-profile" {
			if i+1 < len(args) {
				return args[i+1]
			}

			return ""
		}

		for _, pfx := range []string{"--profile=", "-profile="} {
			if v, ok := strings.CutPrefix(args[i], pfx); ok {
				return v
			}
		}
	}

	return ""
}

// The index translations below mirror the GUI dropdown encodings stored in the profile.

func dpiFromString(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}

	return n
}

var profileFormats = []string{"jpeg", "png", "tiff", "bmp", "webp", "avif", "jxl"}

func formatFromIndex(s string) string {
	if i, _ := strconv.Atoi(s); i >= 1 && i <= len(profileFormats) {
		return profileFormats[i-1]
	}

	return "jpeg"
}

func archiveFromIndex(s string) string {
	if s == "2" {
		return "tar"
	}

	return "zip"
}

func zipLevelFromIndex(s string) int {
	switch i, _ := strconv.Atoi(s); i {
	case 1:
		return -1
	case 2:
		return 0
	default:
		return i - 2
	}
}

func rotateFromIndex(s string) int {
	switch s {
	case "2":
		return 90
	case "3":
		return 180
	case "4":
		return 270
	default:
		return 0
	}
}
