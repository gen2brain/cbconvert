package main

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"image/gif"
	"image/png"
	"net/url"
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"

	"github.com/gen2brain/cbconvert"
	"github.com/gen2brain/iup-go/iup"
)

//go:generate rsrc --ico dist/windows/icon.ico --arch amd64 -o main_windows_amd64.syso

//go:embed assets/logo.png
var appLogo []byte

//go:embed assets/loading.gif
var appLoading []byte

var appVersion string

var (
	index = -1
	files []cbconvert.File

	config iup.Ihandle
)

const (
	pathsGroup    = "Paths"
	profilesGroup = "Profiles"

	inputDirKey  = "InputDir"
	outputDirKey = "OutputDir"
)

type settingKind int

const (
	kindBool settingKind = iota
	kindInt
	kindStr
)

type setting struct {
	handle string
	kind   settingKind
	def    string
}

var settings = []setting{
	{"Recursive", kindBool, "OFF"},
	{"NoRGB", kindBool, "OFF"},
	{"NoCover", kindBool, "OFF"},
	{"NoConvert", kindBool, "OFF"},
	{"NoNonImage", kindBool, "OFF"},
	{"Combine", kindBool, "OFF"},
	{"Fit", kindBool, "OFF"},
	{"Lossless", kindBool, "OFF"},
	{"Grayscale", kindBool, "OFF"},
	{"OutDir", kindStr, ""},
	{"Suffix", kindStr, ""},
	{"Width", kindStr, ""},
	{"Height", kindStr, ""},
	{"Size", kindInt, "0"},
	{"Quality", kindInt, "75"},
	{"Effort", kindInt, "0"},
	{"Brightness", kindInt, "0"},
	{"Contrast", kindInt, "0"},
	{"Format", kindInt, "1"},
	{"Archive", kindInt, "1"},
	{"ZipLevel", kindInt, "1"},
	{"Filter", kindInt, "3"},
	{"Rotate", kindInt, "1"},
}

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
			if len(appVersion) > 7 {
				appVersion = kv.Value[:7]
			}
		}
	}
}

func main() {
	parseFlags()

	iup.Open()
	defer iup.Close()

	iup.SetGlobal("APPNAME", "cbconvert")
	iup.SetGlobal("APPID", "io.github.gen2brain.cbconvert")
	iup.SetGlobal("AUTODARKMODE", "YES")

	config = iup.Config()
	iup.ConfigLoad(config)

	img, _ := png.Decode(bytes.NewReader(appLogo))
	iup.ImageFromImage(img).SetHandle("logo")

	dlg := iup.Dialog(layout()).SetAttributes(fmt.Sprintf(`TITLE="CBconvert %s", ICON=logo, SHRINK=YES`, appVersion)).SetHandle("dlg")

	dlg.SetCallback("POSTMESSAGE_CB", iup.PostMessageFunc(func(ih iup.Ihandle, s string, i int, p any) int {
		sp := strings.Split(s, ": ")
		if len(sp) > 1 {
			iup.MessageError(ih, fmt.Sprintf("%s\n\n%s", sp[0], strings.Join(sp[1:], ": ")))
		}

		return iup.DEFAULT
	}))

	dlg.SetCallback("RESIZE_CB", iup.ResizeFunc(func(ih iup.Ihandle, width, height int) int {
		iup.GetHandle("Preview").SetAttribute("IMAGE", "logo")
		iup.Refresh(ih)

		previewPost()

		return iup.DEFAULT
	}))

	dlg.SetCallback("THEMECHANGED_CB", iup.ThemeChangedFunc(func(ih iup.Ihandle, darkMode int) int {
		t := iup.GetHandle("Table")
		tableRowColors(t, darkMode == 1)
		t.SetAttribute("REDRAW", "YES")

		return iup.DEFAULT
	}))

	iup.Map(dlg)
	profilesInit()

	iup.ShowXY(dlg, iup.CENTER, iup.CENTER)
	iup.MainLoop()
}

func parseFlags() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s <command> [<flags>]\n\n", filepath.Base(os.Args[0]))
		fmt.Fprintf(os.Stderr, "\nCommands:\n")
		fmt.Fprintf(os.Stderr, "\n  version\n    \tPrint version\n\n")
	}

	flag.NewFlagSet("version", flag.ExitOnError)
	flag.Parse()

	if flag.NArg() >= 1 {
		if flag.Arg(0) == "version" {
			fmt.Println(filepath.Base(os.Args[0]), appVersion)
			os.Exit(0)
		} else {
			flag.Usage()
			os.Exit(1)
		}
	}
}

func options() cbconvert.Options {
	var opts cbconvert.Options
	opts.Recursive = iup.GetHandle("Recursive").GetAttribute("VALUE") == "ON"
	opts.NoRGB = iup.GetHandle("NoRGB").GetAttribute("VALUE") == "ON"
	opts.NoCover = iup.GetHandle("NoCover").GetAttribute("VALUE") == "ON"
	opts.Size = iup.GetHandle("Size").GetInt("VALUE")
	opts.OutDir = iup.GetHandle("OutDir").GetAttribute("VALUE")
	opts.Suffix = iup.GetHandle("Suffix").GetAttribute("VALUE")
	opts.NoConvert = iup.GetHandle("NoConvert").GetAttribute("VALUE") == "ON"
	opts.NoNonImage = iup.GetHandle("NoNonImage").GetAttribute("VALUE") == "ON"
	opts.Archive = strings.ToLower(iup.GetHandle("Archive").GetAttribute("VALUESTRING"))
	opts.ZipLevel = zipLevel(iup.GetHandle("ZipLevel").GetAttribute("VALUESTRING"))
	opts.Format = strings.ToLower(iup.GetHandle("Format").GetAttribute("VALUESTRING"))
	opts.Width = iup.GetHandle("Width").GetInt("VALUE")
	opts.Height = iup.GetHandle("Height").GetInt("VALUE")
	opts.Fit = iup.GetHandle("Fit").GetAttribute("VALUE") == "ON"
	opts.Filter = iup.GetHandle("Filter").GetInt("VALUE") - 1
	opts.Quality = iup.GetHandle("Quality").GetInt("VALUE")
	switch opts.Format {
	case "webp", "avif", "jxl":
		opts.Effort = iup.GetHandle("Effort").GetInt("VALUE")
	default:
		opts.Effort = -1
	}
	opts.Lossless = iup.GetHandle("Lossless").GetAttribute("VALUE") == "ON"
	opts.Combine = iup.GetHandle("Combine").GetAttribute("VALUE") == "ON"
	if opts.Combine {
		opts.OutFile = iup.GetHandle("OutFile").GetAttribute("VALUE")
	}
	opts.Grayscale = iup.GetHandle("Grayscale").GetAttribute("VALUE") == "ON"
	opts.Brightness = iup.GetHandle("Brightness").GetInt("VALUE")
	opts.Contrast = iup.GetHandle("Contrast").GetInt("VALUE")
	opts.Rotate = iup.GetHandle("Rotate").GetInt("VALUESTRING")

	return opts
}

func setActive() {
	opts := options()
	count := iup.GetHandle("Table").GetInt("NUMLIN")

	if count == 0 {
		iup.GetHandle("Remove").SetAttribute("ACTIVE", "NO")
		iup.GetHandle("RemoveAll").SetAttribute("ACTIVE", "NO")

		iup.GetHandle("Preview").SetAttribute("IMAGE", "logo")
		iup.GetHandle("PreviewInfo").SetAttribute("TITLE", "")
	} else {
		if index != -1 {
			iup.GetHandle("Remove").SetAttribute("ACTIVE", "YES")
		}
		iup.GetHandle("RemoveAll").SetAttribute("ACTIVE", "YES")
	}

	active := "YES"
	var tip string
	switch {
	case count == 0 && opts.OutDir == "":
		active, tip = "NO", "Add files and set output directory"
	case count == 0:
		active, tip = "NO", "Add files"
	case opts.OutDir == "":
		active, tip = "NO", "Set output directory"
	}

	enabledTip := map[string]string{
		"Thumbnail": "Extract cover thumbnails",
		"Cover":     "Extract covers",
		"Convert":   "Convert files to the selected format",
	}

	for _, h := range []string{"Thumbnail", "Cover", "Convert"} {
		b := iup.GetHandle(h)
		b.SetAttribute("ACTIVE", active)
		if active == "YES" {
			b.SetAttribute("TIP", enabledTip[h])
		} else {
			b.SetAttribute("TIP", tip)
		}
	}

	if opts.NoConvert {
		iup.GetHandle("VboxImage").SetAttribute("ACTIVE", "NO")
		iup.GetHandle("VboxTransform").SetAttribute("ACTIVE", "NO")
	} else {
		iup.GetHandle("VboxImage").SetAttribute("ACTIVE", "YES")
		iup.GetHandle("VboxTransform").SetAttribute("ACTIVE", "YES")
	}

	canLossless := opts.Format == "webp" || opts.Format == "avif" || opts.Format == "jxl"
	losslessOn := canLossless && opts.Lossless

	if (opts.Format == "jpeg" || canLossless) && !opts.NoConvert && !losslessOn {
		iup.GetHandle("VboxQuality").SetAttribute("ACTIVE", "YES")
	} else {
		iup.GetHandle("VboxQuality").SetAttribute("ACTIVE", "NO")
	}

	if canLossless && !opts.NoConvert {
		iup.GetHandle("VboxEffort").SetAttribute("ACTIVE", "YES")
		iup.GetHandle("Lossless").SetAttribute("ACTIVE", "YES")
	} else {
		iup.GetHandle("VboxEffort").SetAttribute("ACTIVE", "NO")
		iup.GetHandle("Lossless").SetAttribute("ACTIVE", "NO")
	}

	if opts.Width != 0 && opts.Height != 0 && !opts.NoConvert {
		iup.GetHandle("Fit").SetAttribute("ACTIVE", "YES")
	} else {
		iup.GetHandle("Fit").SetAttribute("ACTIVE", "NO")
	}

	if opts.Combine {
		iup.GetHandle("VboxOutFile").SetAttribute("ACTIVE", "YES")
	} else {
		iup.GetHandle("VboxOutFile").SetAttribute("ACTIVE", "NO")
	}

	if opts.Archive == "zip" {
		iup.GetHandle("VboxZipLevel").SetAttribute("ACTIVE", "YES")
	} else {
		iup.GetHandle("VboxZipLevel").SetAttribute("ACTIVE", "NO")
	}
}

// shellArg quotes a command-line argument that contains whitespace.
func shellArg(s string) string {
	if strings.ContainsAny(s, " \t") {
		return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
	}

	return s
}

func commandLine() string {
	parts := append([]string{"cbconvert", "convert"}, options().Args()...)
	for _, file := range files {
		parts = append(parts, file.Path)
	}

	for i, p := range parts {
		parts[i] = shellArg(p)
	}

	return strings.Join(parts, " ")
}

func onCommand(iup.Ihandle) int {
	iup.GetText("Command Line", commandLine(), -1)

	return iup.DEFAULT
}

// zipLevel maps the compression dropdown selection to Options.ZipLevel.
func zipLevel(value string) int {
	switch value {
	case "Default":
		return -1
	case "Store (none)":
		return 0
	default:
		level, err := strconv.Atoi(value)
		if err != nil {
			return -1
		}

		return level
	}
}

func profileGroup(name string) string {
	return "Profile:" + name
}

func profileNames() []string {
	s := iup.ConfigGetVariableStr(config, profilesGroup, "Names")
	if s == "" {
		return nil
	}

	return strings.Split(s, ";")
}

func currentProfile() string {
	return iup.ConfigGetVariableStrDef(config, profilesGroup, "Current", "Default")
}

func setStartDir(dlg iup.Ihandle, key string) {
	if dir := iup.ConfigGetVariableStr(config, pathsGroup, key); dir != "" {
		dlg.SetAttribute("DIRECTORY", dir)
	}
}

func rememberDir(dlg iup.Ihandle, key string) {
	dir := dlg.GetAttribute("DIRECTORY")
	if dir == "" {
		return
	}

	iup.ConfigSetVariableStr(config, pathsGroup, key, dir)
	iup.ConfigSave(config)
}

func settingsSave(group string) {
	for _, s := range settings {
		h := iup.GetHandle(s.handle)
		switch s.kind {
		case kindBool:
			v := 0
			if h.GetAttribute("VALUE") == "ON" {
				v = 1
			}
			iup.ConfigSetVariableInt(config, group, s.handle, v)
		case kindInt:
			iup.ConfigSetVariableInt(config, group, s.handle, h.GetInt("VALUE"))
		case kindStr:
			iup.ConfigSetVariableStr(config, group, s.handle, h.GetAttribute("VALUE"))
		}
	}

	iup.ConfigSave(config)
}

// settingsApply sets every control from the given profile group, or from defaults when group is empty.
func settingsApply(group string) {
	for _, s := range settings {
		h := iup.GetHandle(s.handle)
		switch s.kind {
		case kindBool:
			def := 0
			if s.def == "ON" {
				def = 1
			}
			v := def
			if group != "" {
				v = iup.ConfigGetVariableIntDef(config, group, s.handle, def)
			}
			if v != 0 {
				h.SetAttribute("VALUE", "ON")
			} else {
				h.SetAttribute("VALUE", "OFF")
			}
		case kindInt:
			def, _ := strconv.Atoi(s.def)
			v := def
			if group != "" {
				v = iup.ConfigGetVariableIntDef(config, group, s.handle, def)
			}
			h.SetAttribute("VALUE", strconv.Itoa(v))
		case kindStr:
			v := s.def
			if group != "" {
				v = iup.ConfigGetVariableStrDef(config, group, s.handle, s.def)
			}
			h.SetAttribute("VALUE", v)
		}
	}

	syncLabels()
	setActive()
	previewPost()
}

// syncLabels mirrors slider values into their value labels and retunes the effort slider for the current format.
func syncLabels() {
	iup.GetHandle("LabelQuality").SetAttribute("TITLE", iup.GetHandle("Quality").GetInt("VALUE"))
	iup.GetHandle("LabelBrightness").SetAttribute("TITLE", iup.GetHandle("Brightness").GetInt("VALUE"))
	iup.GetHandle("LabelContrast").SetAttribute("TITLE", iup.GetHandle("Contrast").GetInt("VALUE"))

	format := strings.ToLower(iup.GetHandle("Format").GetAttribute("VALUESTRING"))
	eff := iup.GetHandle("Effort").GetInt("VALUE")
	setEffort(format)
	switch format {
	case "webp", "avif", "jxl":
		val := iup.GetHandle("Effort")
		val.SetAttribute("VALUE", strconv.Itoa(eff))
		iup.GetHandle("LabelEffort").SetAttribute("TITLE", fmt.Sprintf("%s: %d", val.GetAttribute("EFFORTNAME"), eff))
	}

	iup.Refresh(iup.GetHandle("Tabs"))
}

func fillProfileList() {
	list := iup.GetHandle("Profile")
	list.SetAttribute("REMOVEITEM", "ALL")

	cur := currentProfile()
	sel := 1
	for i, n := range profileNames() {
		list.SetAttribute(strconv.Itoa(i+1), n)
		if n == cur {
			sel = i + 1
		}
	}

	list.SetAttribute("VALUE", strconv.Itoa(sel))
}

// profilesInit loads the current profile on startup, creating a default one on first run.
func profilesInit() {
	if len(profileNames()) == 0 {
		iup.ConfigSetVariableStr(config, profilesGroup, "Names", "Default")
		iup.ConfigSetVariableStr(config, profilesGroup, "Current", "Default")
		settingsSave(profileGroup("Default"))
	}

	fillProfileList()
	settingsApply(profileGroup(currentProfile()))
}

func onProfileSelect(ih iup.Ihandle) int {
	name := ih.GetAttribute("VALUESTRING")
	if name == "" {
		return iup.DEFAULT
	}

	iup.ConfigSetVariableStr(config, profilesGroup, "Current", name)
	iup.ConfigSave(config)

	settingsApply(profileGroup(name))

	return iup.DEFAULT
}

func onSave(iup.Ihandle) int {
	name := currentProfile()
	if iup.GetParam("Save Profile", nil, "Name: %s\n", &name) != 1 {
		return iup.DEFAULT
	}

	name = strings.TrimSpace(name)
	if name == "" || strings.ContainsAny(name, ".;") {
		iup.Message("Invalid Name", "Profile name must not be empty or contain '.' or ';'.")

		return iup.DEFAULT
	}

	settingsSave(profileGroup(name))

	names := profileNames()
	if !slices.Contains(names, name) {
		names = append(names, name)
		iup.ConfigSetVariableStr(config, profilesGroup, "Names", strings.Join(names, ";"))
	}

	iup.ConfigSetVariableStr(config, profilesGroup, "Current", name)
	iup.ConfigSave(config)

	fillProfileList()

	return iup.DEFAULT
}

func onReset(iup.Ihandle) int {
	settingsApply("")

	return iup.DEFAULT
}

func setEffort(format string) {
	val := iup.GetHandle("Effort")

	var name string

	switch format {
	case "webp":
		val.SetAttributes("MIN=0, MAX=6, SHOWTICKS=7, VALUE=4")
		val.SetAttribute("TIP", "WEBP method, higher is better/slower (0-6, default 4)")
		name = "Method"
	case "avif":
		val.SetAttributes("MIN=0, MAX=10, SHOWTICKS=11, VALUE=10")
		val.SetAttribute("TIP", "AVIF speed, higher is faster/worse (0-10, default 10)")
		name = "Speed"
	case "jxl":
		val.SetAttributes("MIN=1, MAX=10, SHOWTICKS=10, VALUE=7")
		val.SetAttribute("TIP", "JXL effort, higher is better/slower (1-10, default 7)")
		name = "Effort"
	default:
		return
	}

	val.SetAttribute("EFFORTNAME", name)
	iup.GetHandle("LabelEffort").SetAttribute("TITLE", fmt.Sprintf("%s: %d", name, val.GetInt("VALUE")))
	iup.Refresh(iup.GetHandle("LabelEffort"))
}

func layout() iup.Ihandle {
	return iup.Vbox(
		iup.Hbox(
			preview(),
			iup.Hbox(
				iup.Vbox(
					list(),
					tabs(),
				).SetAttributes("NGAP=5"),
			).SetAttributes("NGAP=5"),
			buttons(),
		).SetAttributes("NGAP=5, NMARGIN=5x5"),

		iup.Label("").SetAttributes("SEPARATOR=HORIZONTAL"),
		status(),
	)
}

// tableRowColors sets the alternating row colors for dark or light mode.
func tableRowColors(t iup.Ihandle, dark bool) {
	even, odd := "#F0F0F0", "#FFFFFF"
	if dark {
		even, odd = "#3A3A3A", "#2D2D2D"
	}
	t.SetAttribute("EVENROWCOLOR", even)
	t.SetAttribute("ODDROWCOLOR", odd)
}

func list() iup.Ihandle {
	t := iup.Table().SetHandle("Table")
	t.SetAttributes(map[string]string{
		"EXPAND":         "YES",
		"NUMCOL":         "3",
		"NUMLIN":         "0",
		"TITLE1":         "Title",
		"TITLE2":         "Type",
		"TITLE3":         "Size (MiB)",
		"WIDTH1":         "150",
		"WIDTH2":         "50",
		"WIDTH3":         "100",
		"ALIGNMENT2":     "ACENTER",
		"ALIGNMENT3":     "ARIGHT",
		"SELECTIONMODE":  "SINGLE",
		"USERRESIZE":     "YES",
		"STRETCHLAST":    "NO",
		"FOCUSRECT":      "NO",
		"SORTABLE":       "YES",
		"ALTERNATECOLOR": "YES",
	})

	tableRowColors(t, iup.GetGlobal("DARKMODE") == "YES" && iup.GetGlobal("AUTODARKMODE") == "YES")

	t.SetCallback("ENTERITEM_CB", iup.EnterItemFunc(func(ih iup.Ihandle, lin, col int) int {
		index = lin - 1
		setActive()
		previewPost()

		return iup.DEFAULT
	}))

	t.SetCallback("DROPFILES_CB", iup.DropFilesFunc(func(ih iup.Ihandle, fileName string, num, x, y int) int {
		dec, err := url.QueryUnescape(fileName)
		if err != nil {
			iup.PostMessage(iup.GetHandle("dlg"), err.Error(), 0, 0)
			fmt.Println(err)

			return iup.DEFAULT
		}

		conv := cbconvert.New(options())

		fs, err := conv.Files([]string{dec})
		if err != nil {
			iup.PostMessage(iup.GetHandle("dlg"), err.Error(), 0, 0)
			fmt.Println(err)

			return iup.DEFAULT
		}

		wasEmpty := len(files) == 0

		for _, file := range fs {
			appendFile(file)
		}

		if wasEmpty && len(files) > 0 {
			selectRow(0)
		}

		setActive()

		if wasEmpty {
			previewPost()
		}

		return iup.DEFAULT
	}))

	return iup.Vbox(t)
}

// selectRow focuses and selects the given 0-based row in the table.
func selectRow(i int) {
	if i < 0 || i >= len(files) {
		return
	}

	index = i
	iup.GetHandle("Table").SetAttribute("FOCUSCELL", fmt.Sprintf("%d:1", i+1))
}

// appendFile adds a file as a new row to the table and the files slice.
func appendFile(file cbconvert.File) {
	files = append(files, file)

	t := iup.GetHandle("Table")
	lin := len(files)
	t.SetAttribute("NUMLIN", strconv.Itoa(lin))
	iup.SetAttributeId2(t, "", lin, 1, file.Name)
	iup.SetAttributeId2(t, "", lin, 2, cbconvert.FileType(file.Path))
	iup.SetAttributeId2(t, "", lin, 3, strconv.FormatFloat(float64(file.Stat.Size())/(1024*1024), 'f', 2, 64))
}

func previewPost() {
	if index == -1 || len(files) == 0 {
		return
	}

	width, height := previewSize()
	iup.GetHandle("Loading").SetAttributes("VISIBLE=YES, START=YES")
	if strings.ToLower(iup.GetGlobal("DRIVER")) == "motif" {
		iup.GetHandle("Preview").SetAttribute("IMAGE", "")
	}

	opts := options()

	go func(opts cbconvert.Options) {
		conv := cbconvert.New(opts)

		var s string
		file := files[index]

		img, err := conv.Preview(file.Path, file.Stat, width, height)
		if err != nil {
			s = err.Error()
			fmt.Println(err)
		}

		iup.PostMessage(iup.GetHandle("Preview"), s, 0, img)
	}(opts)
}

func previewSize() (int, int) {
	var width, height int
	sp := strings.Split(iup.GetHandle("Preview").GetAttribute("RASTERSIZE"), "x")
	if len(sp) == 2 {
		width, _ = strconv.Atoi(sp[0])
		height, _ = strconv.Atoi(sp[1])
	}

	return width, height
}

func preview() iup.Ihandle {
	return iup.Frame(
		iup.Vbox(
			iup.Label("").SetAttributes("EXPAND=YES, ALIGNMENT=ACENTER, MINSIZE=400x, IMAGE=cover").SetHandle("Preview").
				SetCallback("POSTMESSAGE_CB", iup.PostMessageFunc(func(ih iup.Ihandle, s string, i int, p any) int {
					img := p.(cbconvert.Image)

					iup.GetHandle("Loading").SetAttributes("VISIBLE=NO, STOP=YES")

					if img.Image != nil && len(s) == 0 {
						iup.Destroy(iup.GetHandle("cover"))
						iup.ImageFromImage(img.Image).SetHandle("cover")

						ih.SetAttribute("IMAGE", "cover")
						iup.GetHandle("PreviewInfo").SetAttribute("TITLE", fmt.Sprintf("%s (%dx%d)", img.SizeHuman, img.Width, img.Height))
					} else {
						ih.SetAttribute("IMAGE", "logo")
						iup.GetHandle("PreviewInfo").SetAttribute("TITLE", "")

						sp := strings.Split(s, ": ")
						if len(sp) > 1 {
							iup.MessageError(ih, fmt.Sprintf("%s\n\n%s", sp[0], strings.Join(sp[1:], ": ")))
						}
					}

					return iup.DEFAULT
				})),
			iup.Label("").SetAttributes("EXPAND=HORIZONTAL, ALIGNMENT=ACENTER").SetHandle("PreviewInfo"),
		),
	)
}

func tabs() iup.Ihandle {
	vboxInput := iup.Vbox(
		iup.Toggle(" Recurse SubDirectories").SetHandle("Recursive").
			SetAttributes(`TIP="Process subdirectories recursively"`),
		iup.Toggle(" Only Grayscale Images").SetHandle("NoRGB").
			SetAttributes(`TIP="Do not convert images that have RGB colorspace"`),
		iup.Toggle(" Exclude Cover").SetHandle("NoCover").
			SetAttributes(`TIP="Do not convert the cover image"`),
		iup.Toggle(" Remove Non-Image Files from the Archive").SetHandle("NoNonImage").
			SetAttribute("TIP", "Remove .nfo, .xml, .txt files from the archive"),
		iup.Toggle(" Do not Transform or Convert Images").SetHandle("NoConvert").
			SetAttributes(`TIP="Copy images from archive or directory without modifications"`).
			SetCallback("VALUECHANGED_CB", iup.ValueChangedFunc(func(ih iup.Ihandle) int {
				setActive()

				return iup.DEFAULT
			})),
		iup.Vbox(
			iup.Label("Minimum Size (MiB):"),
			iup.Text().SetAttributes(`SPIN=YES, SPINMAX=2048, VISIBLECOLUMNS=4, MASK="/d*"`).SetHandle("Size").
				SetAttributes(`TIP="Process only files larger than minimum size"`),
		),
		iup.Space().SetAttributes("EXPAND=HORIZONTAL"),
	).SetHandle("VboxInput").SetAttributes("NGAP=10")

	vboxOutput := iup.Hbox(
		iup.Vbox(
			iup.Vbox(
				iup.Label("Output Directory:"),
				iup.Text().SetAttributes("VISIBLECOLUMNS=16, MINSIZE=100x").SetHandle("OutDir").
					SetCallback("VALUECHANGED_CB", iup.ValueChangedFunc(func(ih iup.Ihandle) int {
						setActive()

						return iup.DEFAULT
					})),
				iup.Space().SetAttribute("SIZE", "5x0"),
				iup.Button("Browse...").SetAttributes("PADDING=DEFAULTBUTTONPADDING").
					SetCallback("ACTION", iup.ActionFunc(onOutputDirectory)),
			),
			iup.Vbox(
				iup.Label("Add Suffix to Output File:"),
				iup.Text().SetAttributes("VISIBLECOLUMNS=16, MINSIZE=100x").SetHandle("Suffix").
					SetAttribute("TIP", "Add suffix to filename, i.e. filename_suffix.cbz"),
			),
			iup.Vbox(
				iup.Label("Archive Format:"),
				iup.List().SetAttributes(map[string]string{
					"DROPDOWN": "YES",
					"VALUE":    "1",
					"1":        "ZIP",
					"2":        "TAR",
				}).SetHandle("Archive").
					SetCallback("VALUECHANGED_CB", iup.ValueChangedFunc(func(ih iup.Ihandle) int {
						setActive()

						return iup.DEFAULT
					})),
			),
			iup.Vbox(
				iup.Label("Compression:"),
				iup.List().SetAttributes(map[string]string{
					"DROPDOWN": "YES",
					"VALUE":    "1",
					"1":        "Default",
					"2":        "Store (none)",
					"3":        "1",
					"4":        "2",
					"5":        "3",
					"6":        "4",
					"7":        "5",
					"8":        "6",
					"9":        "7",
					"10":       "8",
					"11":       "9",
				}).SetHandle("ZipLevel").
					SetAttribute("TIP", "ZIP compression: Store disables it, 1 is fastest, 9 is smallest"),
			).SetHandle("VboxZipLevel"),
		).SetAttributes("NGAP=10"),
		iup.Space().SetAttribute("SIZE", "15"),
		iup.Vbox(
			iup.Vbox(
				iup.Toggle(" Combine into single file").SetHandle("Combine").
					SetAttributes(`TIP="Merge all listed files into one archive"`).
					SetCallback("VALUECHANGED_CB", iup.ValueChangedFunc(func(ih iup.Ihandle) int {
						setActive()

						return iup.DEFAULT
					})),
			),
			iup.Vbox(
				iup.Label("Output File:"),
				iup.Text().SetAttributes("VISIBLECOLUMNS=16, MINSIZE=100x").SetHandle("OutFile").
					SetAttribute("TIP", "Combined file name (default: first input + -combined)"),
				iup.Space().SetAttribute("SIZE", "5x0"),
				iup.Button("Browse...").SetAttributes("PADDING=DEFAULTBUTTONPADDING").
					SetCallback("ACTION", iup.ActionFunc(onOutputFile)),
			).SetHandle("VboxOutFile"),
		).SetAttributes("NGAP=10"),
	).SetHandle("VboxOutput")

	vboxImage := iup.Hbox(
		iup.Vbox(
			iup.Vbox(
				iup.Label("Format:"),
				iup.List().SetAttributes(map[string]string{
					"DROPDOWN": "YES",
					"VALUE":    "1",
					"1":        "JPEG",
					"2":        "PNG",
					"3":        "TIFF",
					"4":        "BMP",
					"5":        "WEBP",
					"6":        "AVIF",
					"7":        "JXL",
				}).SetHandle("Format").
					SetCallback("VALUECHANGED_CB", iup.ValueChangedFunc(func(ih iup.Ihandle) int {
						setEffort(strings.ToLower(ih.GetAttribute("VALUESTRING")))
						setActive()
						previewPost()

						return iup.DEFAULT
					})),
			),
			iup.Vbox(
				iup.Label("Size:"),
				iup.Hbox(
					iup.Text().SetAttributes(`CUEBANNER="width", VISIBLECOLUMNS=6, MASK="/d*"`).SetHandle("Width").
						SetAttribute("TIP", "If one of, width or height is not set, the image aspect ratio is preserved").
						SetCallback("VALUECHANGED_CB", iup.ValueChangedFunc(func(ih iup.Ihandle) int {
							setActive()
							ih.SetAttribute("MYVALUE", ih.GetInt("VALUE"))

							return iup.DEFAULT
						})).
						SetCallback("KILLFOCUS_CB", iup.KillFocusFunc(func(ih iup.Ihandle) int {
							if ih.GetAttribute("MYVALUE") != "" {
								previewPost()
							}
							ih.SetAttribute("MYVALUE", "")

							return iup.DEFAULT
						})),
					iup.Space().SetAttribute("SIZE", "2"),
					iup.Label("x"),
					iup.Space().SetAttribute("SIZE", "2"),
					iup.Text().SetAttributes(`CUEBANNER="height", VISIBLECOLUMNS=6, MASK="/d*"`).SetHandle("Height").
						SetAttribute("TIP", "If one of, width or height is not set, the image aspect ratio is preserved").
						SetCallback("VALUECHANGED_CB", iup.ValueChangedFunc(func(ih iup.Ihandle) int {
							setActive()
							ih.SetAttribute("MYVALUE", ih.GetInt("VALUE"))

							return iup.DEFAULT
						})).
						SetCallback("KILLFOCUS_CB", iup.KillFocusFunc(func(ih iup.Ihandle) int {
							if ih.GetAttribute("MYVALUE") != "" {
								previewPost()
							}
							ih.SetAttribute("MYVALUE", "")

							return iup.DEFAULT
						})),
				).SetAttributes("ALIGNMENT=ACENTER, NMARGIN=0"),
			),
			iup.Vbox(
				iup.Toggle(" Best Fit").SetHandle("Fit").
					SetAttributes(`TIP="Best fit for required width and height"`),
			),
			iup.Vbox(
				iup.Label("Resize Filter:"),
				iup.List().SetAttributes(map[string]string{
					"DROPDOWN": "YES",
					"VALUE":    "3",
					"TIP":      "Linear is the bilinear filter, smooth and reasonably fast",
					"1":        "NearestNeighbor",
					"2":        "Box",
					"3":        "Linear",
					"4":        "MitchellNetravali",
					"5":        "CatmullRom",
					"6":        "Gaussian",
					"7":        "Lanczos",
				}).SetHandle("Filter").SetCallback("VALUECHANGED_CB", iup.ValueChangedFunc(onFilterChanged)),
			),
		).SetAttributes("NGAP=10"),
		iup.Space().SetAttribute("SIZE", "15"),
		iup.Vbox(
			iup.Vbox(
				iup.Hbox(
					iup.Label("Quality: "),
					iup.Label("75").SetHandle("LabelQuality"),
				).SetAttributes("NMARGIN=0"),
				iup.Val("").SetAttributes(`MIN=0, MAX=100, VALUE=75, SHOWTICKS=10`).SetHandle("Quality").
					SetAttribute("TIP", "Quality affects JPEG, WEBP, AVIF and JXL").
					SetCallback("VALUECHANGED_CB", iup.ValueChangedFunc(func(ih iup.Ihandle) int {
						iup.GetHandle("LabelQuality").SetAttribute("TITLE", ih.GetInt("VALUE"))
						iup.Refresh(iup.GetHandle("LabelQuality"))
						ih.SetAttribute("MYVALUE", ih.GetInt("VALUE"))

						return iup.DEFAULT
					})).
					SetCallback("KILLFOCUS_CB", iup.KillFocusFunc(func(ih iup.Ihandle) int {
						if ih.GetAttribute("MYVALUE") != "" {
							previewPost()
						}
						ih.SetAttribute("MYVALUE", "")

						return iup.DEFAULT
					})),
			).SetHandle("VboxQuality"),
			iup.Vbox(
				iup.Label("Effort:").SetHandle("LabelEffort"),
				iup.Val("").SetAttributes(`MIN=0, MAX=10, VALUE=0, SHOWTICKS=11`).SetHandle("Effort").
					SetAttribute("TIP", "Encoder speed/effort (WEBP, AVIF, JXL)").
					SetCallback("VALUECHANGED_CB", iup.ValueChangedFunc(func(ih iup.Ihandle) int {
						iup.GetHandle("LabelEffort").SetAttribute("TITLE", fmt.Sprintf("%s: %d", ih.GetAttribute("EFFORTNAME"), ih.GetInt("VALUE")))
						iup.Refresh(iup.GetHandle("LabelEffort"))
						ih.SetAttribute("MYVALUE", ih.GetInt("VALUE"))

						return iup.DEFAULT
					})).
					SetCallback("KILLFOCUS_CB", iup.KillFocusFunc(func(ih iup.Ihandle) int {
						if ih.GetAttribute("MYVALUE") != "" {
							previewPost()
						}
						ih.SetAttribute("MYVALUE", "")

						return iup.DEFAULT
					})),
			).SetHandle("VboxEffort"),
			iup.Vbox(
				iup.Toggle(" Lossless").SetHandle("Lossless").
					SetAttributes(`TIP="Lossless compression (WEBP, AVIF, JXL), ignores quality"`).
					SetCallback("VALUECHANGED_CB", iup.ValueChangedFunc(func(ih iup.Ihandle) int {
						setActive()
						previewPost()

						return iup.DEFAULT
					})),
			),
			iup.Vbox(
				iup.Toggle(" Grayscale").SetHandle("Grayscale").
					SetAttributes(`TIP="Convert images to grayscale (monochromatic)"`).
					SetCallback("VALUECHANGED_CB", iup.ValueChangedFunc(func(ih iup.Ihandle) int {
						previewPost()

						return iup.DEFAULT
					})),
			),
		).SetAttributes("NGAP=10"),
	).SetHandle("VboxImage")

	vboxTransform := iup.Vbox(
		iup.Vbox(
			iup.Hbox(
				iup.Label("Brightness: "),
				iup.Label("0").SetHandle("LabelBrightness"),
			).SetAttributes("ALIGNMENT=ACENTER, NMARGIN=0"),
			iup.Val("").SetAttributes(`MIN=-100, MAX=100, VALUE=0, SHOWTICKS=10`).SetHandle("Brightness").
				SetAttributes(`TIP="Adjust the brightness of the images"`).
				SetCallback("VALUECHANGED_CB", iup.ValueChangedFunc(func(ih iup.Ihandle) int {
					iup.GetHandle("LabelBrightness").SetAttribute("TITLE", iup.GetHandle("Brightness").GetInt("VALUE"))
					iup.Refresh(iup.GetHandle("LabelBrightness"))
					ih.SetAttribute("MYVALUE", ih.GetInt("VALUE"))

					return iup.DEFAULT
				})).
				SetCallback("KILLFOCUS_CB", iup.KillFocusFunc(func(ih iup.Ihandle) int {
					if ih.GetAttribute("MYVALUE") != "" {
						previewPost()
					}
					ih.SetAttribute("MYVALUE", "")

					return iup.DEFAULT
				})),
		),
		iup.Vbox(
			iup.Hbox(
				iup.Label("Contrast: "),
				iup.Label("0").SetHandle("LabelContrast"),
			).SetAttributes("ALIGNMENT=ACENTER, NMARGIN=0"),
			iup.Val("").SetAttributes(`MIN=-100, MAX=100, VALUE=0, SHOWTICKS=10`).SetHandle("Contrast").
				SetAttributes(`TIP="Adjust the contrast of the images"`).
				SetCallback("VALUECHANGED_CB", iup.ValueChangedFunc(func(ih iup.Ihandle) int {
					iup.GetHandle("LabelContrast").SetAttribute("TITLE", iup.GetHandle("Contrast").GetInt("VALUE"))
					iup.Refresh(iup.GetHandle("LabelContrast"))
					ih.SetAttribute("MYVALUE", ih.GetInt("VALUE"))

					return iup.DEFAULT
				})).
				SetCallback("KILLFOCUS_CB", iup.KillFocusFunc(func(ih iup.Ihandle) int {
					if ih.GetAttribute("MYVALUE") != "" {
						previewPost()
					}
					ih.SetAttribute("MYVALUE", "")

					return iup.DEFAULT
				})),
		),
		iup.Vbox(
			iup.Label("Rotate:"),
			iup.List().SetAttributes(map[string]string{
				"DROPDOWN": "YES",
				"VALUE":    "1",
				"1":        "0",
				"2":        "90",
				"3":        "180",
				"4":        "270",
			}).SetHandle("Rotate").
				SetCallback("VALUECHANGED_CB", iup.ValueChangedFunc(func(ih iup.Ihandle) int {
					previewPost()

					return iup.DEFAULT
				})),
		),
	).SetHandle("VboxTransform").SetAttributes("NGAP=10")

	return iup.Tabs(
		vboxInput.SetAttributes("TABTITLE=Input, NMARGIN=10x10"),
		vboxOutput.SetAttributes("TABTITLE=Output, NMARGIN=10x10"),
		vboxImage.SetAttributes("TABTITLE=Image, NMARGIN=10x10"),
		vboxTransform.SetAttributes("TABTITLE=Transform, NMARGIN=10x10"),
	).SetHandle("Tabs")
}

func buttons() iup.Ihandle {
	addFiles := iup.Button("Add &Files...").SetHandle("AddFiles").SetAttributes("PADDING=DEFAULTBUTTONPADDING").
		SetCallback("ACTION", iup.ActionFunc(onAddFiles))
	addDir := iup.Button("Add &Dir...").SetHandle("AddDir").SetAttributes("PADDING=DEFAULTBUTTONPADDING").
		SetCallback("ACTION", iup.ActionFunc(onAddDir))
	remove := iup.Button("Remove").SetHandle("Remove").SetAttributes("PADDING=DEFAULTBUTTONPADDING").
		SetCallback("ACTION", iup.ActionFunc(onRemove))
	removeAll := iup.Button("Remove All").SetHandle("RemoveAll").SetAttributes("PADDING=DEFAULTBUTTONPADDING").
		SetCallback("ACTION", iup.ActionFunc(onRemoveAll))
	thumbnail := iup.Button("Thumbnail").SetHandle("Thumbnail").SetAttributes("PADDING=DEFAULTBUTTONPADDING").
		SetCallback("ACTION", iup.ActionFunc(onThumbnail))
	cover := iup.Button("Cover").SetHandle("Cover").SetAttributes("PADDING=DEFAULTBUTTONPADDING").
		SetCallback("ACTION", iup.ActionFunc(onCover))
	convert := iup.Button("&Convert").SetHandle("Convert").SetAttributes("PADDING=DEFAULTBUTTONPADDING").
		SetCallback("ACTION", iup.ActionFunc(onConvert))
	reset := iup.Button("Reset").SetHandle("Reset").SetAttributes("PADDING=DEFAULTBUTTONPADDING").
		SetAttribute("TIP", "Restore all settings to their defaults").
		SetCallback("ACTION", iup.ActionFunc(onReset))
	save := iup.Button("Save").SetHandle("Save").SetAttributes("PADDING=DEFAULTBUTTONPADDING").
		SetAttribute("TIP", "Save current settings to a profile").
		SetCallback("ACTION", iup.ActionFunc(onSave))

	command := iup.Button("Command").SetAttributes("PADDING=DEFAULTBUTTONPADDING").
		SetAttribute("TIP", "Show the equivalent command line").
		SetCallback("ACTION", iup.ActionFunc(onCommand))

	profile := iup.List().SetAttributes("DROPDOWN=YES").SetHandle("Profile").
		SetAttribute("TIP", "Select a settings profile").
		SetCallback("VALUECHANGED_CB", iup.ValueChangedFunc(onProfileSelect))

	iup.Normalizer(addFiles, addDir, remove, removeAll, thumbnail, cover, convert, reset, save, command).SetAttribute("NORMALIZE", "BOTH")
	iup.Normalizer(addFiles, addDir, remove, removeAll, thumbnail, cover, convert, reset, save, command, profile).SetAttribute("NORMALIZE", "HORIZONTAL")

	return iup.Vbox(
		iup.Vbox(
			addFiles,
			addDir,
			remove,
			removeAll,
		).SetAttribute("NGAP", "2"),
		iup.Space().SetAttribute("SIZE", "x8"),
		iup.Vbox(
			thumbnail,
			cover,
		).SetAttribute("NGAP", "2"),
		iup.Space().SetAttribute("SIZE", "x8"),
		iup.Vbox(
			convert,
		),
		iup.Fill(),
		iup.Vbox(
			iup.Label("Profile:"),
			profile,
			reset,
			save,
			command,
		).SetAttribute("NGAP", "2"),
	).SetHandle("Buttons").SetAttributes("ALIGNMENT=ACENTER")
}

func status() iup.Ihandle {
	return iup.Hbox(
		loading(),
		iup.Fill(),
		iup.Label("File 1 of 1").SetHandle("LabelStatus1").SetAttributes("VISIBLE=NO"),
		iup.Space().SetAttribute("SIZE", "5"),
		iup.Label("(000/000)").SetHandle("LabelStatus2").SetAttributes("VISIBLE=NO"),
		iup.Space().SetAttribute("SIZE", "5"),
		iup.ProgressBar().SetAttributes("RASTERSIZE=200x, VISIBLE=NO").SetHandle("ProgressBar").
			SetCallback("POSTMESSAGE_CB", iup.PostMessageFunc(func(ih iup.Ihandle, s string, i int, p any) int {
				switch s {
				case "convert":
					conv := p.(*cbconvert.Converter)
					ih.SetAttributes("VALUE=0, VISIBLE=YES")
					ih.SetAttribute("MAX", conv.Ncontents)

					iup.GetHandle("Table").SetAttributes("ACTIVE=NO")
					iup.GetHandle("Tabs").SetAttributes("ACTIVE=NO")
					iup.GetHandle("Buttons").SetAttributes("ACTIVE=NO")

					iup.GetHandle("LabelStatus1").SetAttribute("TITLE", fmt.Sprintf("File %d of %d", conv.CurrFile, conv.Nfiles))
					iup.GetHandle("LabelStatus1").SetAttributes("VISIBLE=YES")
					iup.GetHandle("LabelStatus2").SetAttributes("VISIBLE=YES")

					iup.Refresh(iup.GetHandle("StatusBar"))
				case "start":
					conv := p.(*cbconvert.Converter)
					ih.SetAttributes("VALUE=0, VISIBLE=YES")
					ih.SetAttribute("MAX", conv.Nfiles)

					iup.GetHandle("Table").SetAttributes("ACTIVE=NO")
					iup.GetHandle("Tabs").SetAttributes("ACTIVE=NO")
					iup.GetHandle("Buttons").SetAttributes("ACTIVE=NO")

					iup.GetHandle("LabelStatus2").SetAttributes("VISIBLE=YES")
				case "progress":
					conv := p.(*cbconvert.Converter)
					ih.SetAttribute("VALUE", conv.CurrContent)
					iup.GetHandle("LabelStatus2").SetAttribute("TITLE", fmt.Sprintf("(%03d/%03d)", conv.CurrContent, conv.Ncontents))

					iup.Refresh(iup.GetHandle("StatusBar"))
				case "progress2":
					conv := p.(*cbconvert.Converter)
					ih.SetAttribute("VALUE", conv.CurrFile)
					iup.GetHandle("LabelStatus2").SetAttribute("TITLE", fmt.Sprintf("(%03d/%03d)", conv.CurrFile, conv.Nfiles))

					iup.Refresh(iup.GetHandle("StatusBar"))
				case "finish":
					iup.GetHandle("Table").SetAttributes("ACTIVE=YES")
					iup.GetHandle("Tabs").SetAttributes("ACTIVE=YES")
					iup.GetHandle("Buttons").SetAttributes("ACTIVE=YES")

					iup.GetHandle("LabelStatus1").SetAttributes(`TITLE="", VISIBLE=NO`)
					iup.GetHandle("LabelStatus2").SetAttributes(`TITLE="", VISIBLE=NO`)
					ih.SetAttributes("VALUE=0, VISIBLE=NO")

					iup.Refresh(iup.GetHandle("StatusBar"))

					iup.GetHandle("dlg").SetCallback("K_ANY", nil)
					iup.GetHandle("dlg").SetCallback("CLOSE_CB", nil)
				}

				return iup.DEFAULT
			})),
		iup.Space().SetAttribute("SIZE", "5x0"),
	).SetAttributes("ALIGNMENT=ACENTER, NMARGIN=5x5").SetHandle("StatusBar")
}

func loading() iup.Ihandle {
	img, _ := gif.DecodeAll(bytes.NewReader(appLoading))
	animation := iup.User()

	for idx, i := range img.Image {
		name := fmt.Sprintf("Loading%d", idx)
		iup.ImageFromImage(i).SetHandle(name)
		iup.Append(animation, iup.GetHandle(name))
	}

	return iup.AnimatedLabel(animation).SetAttributes("VISIBLE=NO").SetHandle("Loading")
}

func onAddFiles(ih iup.Ihandle) int {
	args, err := fileDlg("Add Files", true, false, inputDirKey)
	if err != nil {
		iup.PostMessage(iup.GetHandle("dlg"), err.Error(), 0, 0)
		fmt.Println(err)

		return iup.DEFAULT
	}

	if len(args) > 0 {
		conv := cbconvert.New(options())

		fs, err := conv.Files(args)
		if err != nil {
			iup.PostMessage(iup.GetHandle("dlg"), err.Error(), 0, 0)
			fmt.Println(err)

			return iup.DEFAULT
		}

		wasEmpty := len(files) == 0

		for _, file := range fs {
			appendFile(file)
		}

		if wasEmpty && len(files) > 0 {
			selectRow(0)
		}

		setActive()

		if wasEmpty {
			previewPost()
		}
	}

	return iup.DEFAULT
}

func onAddDir(ih iup.Ihandle) int {
	args, err := fileDlg("Add Directory", false, true, inputDirKey)
	if err != nil {
		iup.PostMessage(iup.GetHandle("dlg"), err.Error(), 0, 0)
		fmt.Println(err)

		return iup.DEFAULT
	}

	if len(args) > 0 {
		conv := cbconvert.New(options())

		fs, err := conv.Files(args)
		if err != nil {
			iup.PostMessage(iup.GetHandle("dlg"), err.Error(), 0, 0)
			fmt.Println(err)

			return iup.DEFAULT
		}

		wasEmpty := len(files) == 0

		for _, file := range fs {
			appendFile(file)
		}

		if wasEmpty && len(files) > 0 {
			selectRow(0)
		}

		setActive()

		if wasEmpty {
			previewPost()
		}
	}

	return iup.DEFAULT
}

func onRemove(ih iup.Ihandle) int {
	if index < 0 || index >= len(files) {
		return iup.IGNORE
	}

	iup.GetHandle("Table").SetAttribute("DELLIN", strconv.Itoa(index+1))
	files = slices.Delete(files, index, index+1)

	if index >= len(files) {
		index = len(files) - 1
	}

	setActive()
	previewPost()

	return iup.DEFAULT
}

func onRemoveAll(ih iup.Ihandle) int {
	index = -1
	files = make([]cbconvert.File, 0)

	iup.GetHandle("Table").SetAttribute("NUMLIN", "0")
	setActive()

	return iup.DEFAULT
}

func onThumbnail(ih iup.Ihandle) int {
	conv := cbconvert.New(options())
	conv.Nfiles = len(files)

	conv.OnProgress = func() {
		iup.PostMessage(iup.GetHandle("ProgressBar"), "progress2", 0, conv)
	}

	var canceled = false
	conv.OnCancel = func() {
		canceled = true
	}

	iup.GetHandle("dlg").SetCallback("K_ANY", iup.KAnyFunc(func(ih iup.Ihandle, c int) int {
		if c == iup.K_ESC {
			conv.Cancel()
		}

		return iup.DEFAULT
	}))

	iup.PostMessage(iup.GetHandle("ProgressBar"), "start", 0, conv)

	go func(c *cbconvert.Converter) {
		for _, file := range files {
			if canceled {
				break
			}

			if err := c.Thumbnail(file); err != nil {
				iup.PostMessage(iup.GetHandle("dlg"), err.Error(), 0, 0)
				fmt.Println(err)

				continue
			}
		}

		iup.PostMessage(iup.GetHandle("ProgressBar"), "finish", 0, 0)
	}(conv)

	return iup.DEFAULT
}

func onCover(ih iup.Ihandle) int {
	conv := cbconvert.New(options())
	conv.Nfiles = len(files)

	conv.OnProgress = func() {
		iup.PostMessage(iup.GetHandle("ProgressBar"), "progress2", 0, conv)
	}

	var canceled = false
	conv.OnCancel = func() {
		canceled = true
	}

	iup.GetHandle("dlg").SetCallback("K_ANY", iup.KAnyFunc(func(ih iup.Ihandle, c int) int {
		if c == iup.K_ESC {
			conv.Cancel()
		}

		return iup.DEFAULT
	}))

	iup.PostMessage(iup.GetHandle("ProgressBar"), "start", 0, conv)

	go func(c *cbconvert.Converter) {
		for _, file := range files {
			if canceled {
				break
			}

			if err := c.Cover(file); err != nil {
				iup.PostMessage(iup.GetHandle("dlg"), err.Error(), 0, 0)
				fmt.Println(err)

				continue
			}
		}

		iup.PostMessage(iup.GetHandle("ProgressBar"), "finish", 0, 0)
	}(conv)

	return iup.DEFAULT
}

func onConvert(ih iup.Ihandle) int {
	conv := cbconvert.New(options())
	conv.Nfiles = len(files)

	conv.OnStart = func() {
		iup.PostMessage(iup.GetHandle("ProgressBar"), "convert", 0, conv)
	}

	conv.OnProgress = func() {
		iup.PostMessage(iup.GetHandle("ProgressBar"), "progress", 0, conv)
	}

	iup.GetHandle("dlg").SetCallback("K_ANY", iup.KAnyFunc(func(ih iup.Ihandle, c int) int {
		if c == iup.K_ESC {
			conv.Cancel()
		}

		return iup.DEFAULT
	})).SetCallback("CLOSE_CB", iup.CloseFunc(func(ih iup.Ihandle) int {
		if err := os.RemoveAll(conv.Workdir); err != nil {
			fmt.Println(err)
		}

		return iup.DEFAULT
	}))

	convertErr := func(err error) {
		if errors.Is(err, context.Canceled) {
			if err := os.RemoveAll(conv.Workdir); err != nil {
				fmt.Println(err)
			}

			return
		}

		iup.PostMessage(iup.GetHandle("dlg"), err.Error(), 0, 0)
		fmt.Println(err)

		if err := os.RemoveAll(conv.Workdir); err != nil {
			fmt.Println(err)
		}
	}

	go func(c *cbconvert.Converter) {
		if c.Opts.Combine {
			if err := c.Combine(files); err != nil {
				convertErr(err)
			}
		} else {
			for _, file := range files {
				if err := c.Convert(file); err != nil {
					convertErr(err)
					if errors.Is(err, context.Canceled) {
						break
					}

					continue
				}
			}
		}

		iup.PostMessage(iup.GetHandle("ProgressBar"), "finish", 0, 0)
	}(conv)

	return iup.DEFAULT
}

func onOutputDirectory(ih iup.Ihandle) int {
	args, err := fileDlg("Output Directory", false, true, outputDirKey)
	if err != nil {
		iup.PostMessage(iup.GetHandle("dlg"), err.Error(), 0, 0)
		fmt.Println(err)

		return iup.DEFAULT
	}

	if len(args) == 1 {
		iup.GetHandle("OutDir").SetAttribute("VALUE", args[0])
	}

	setActive()

	return iup.DEFAULT
}

func onOutputFile(ih iup.Ihandle) int {
	name := saveDlg("Output File", outputDirKey)
	if name != "" {
		iup.GetHandle("OutFile").SetAttribute("VALUE", filepath.Base(name))
		iup.GetHandle("OutDir").SetAttribute("VALUE", filepath.Dir(name))
		setActive()
	}

	return iup.DEFAULT
}

func onFilterChanged(ih iup.Ihandle) int {
	switch ih.GetInt("VALUE") {
	case 1:
		ih.SetAttribute("TIP", "NearestNeighbor is the fastest resampling filter, no antialiasing")
	case 2:
		ih.SetAttribute("TIP", "Box filter (averaging pixels)")
	case 3:
		ih.SetAttribute("TIP", "Linear is the bilinear filter, smooth and reasonably fast")
	case 4:
		ih.SetAttribute("TIP", "MitchellNetravali is a smooth bicubic filter")
	case 5:
		ih.SetAttribute("TIP", "CatmullRom is a sharp bicubic filter")
	case 6:
		ih.SetAttribute("TIP", "Gaussian is a blurring filter that uses gaussian function, useful for noise removal")
	case 7:
		ih.SetAttribute("TIP", "Lanczos is a high-quality resampling filter, it's slower than cubic filters")
	}

	previewPost()

	return iup.DEFAULT
}

func fileDlg(title string, multiple, directory bool, dirKey string) ([]string, error) {
	ret := make([]string, 0)

	dlg := iup.FileDlg()
	defer dlg.Destroy()

	if !directory {
		mf := "YES"
		if !multiple {
			mf = "NO"
		}

		dlg.SetAttributes(map[string]string{
			"DIALOGTYPE":     "OPEN",
			"MULTIPLEFILES":  mf,
			"MULTIVALUEPATH": "YES",
			"EXTFILTER":      "Comic Files|*.rar;*.zip;*.7z;*.tar;*.cbr;*.cbz;*.cb7;*.cbt;*.pdf;*.epub;*.mobi;*.docx;*.pptx|",
			"FILTER":         "*.cb*", // for Motif
			"TITLE":          title,
		})
	} else {
		dlg.SetAttributes(map[string]string{
			"DIALOGTYPE": "DIR",
			"TITLE":      title,
		})
	}

	setStartDir(dlg, dirKey)

	iup.Popup(dlg, iup.CENTERPARENT, iup.CENTERPARENT)

	if dlg.GetInt("STATUS") == 0 {
		switch {
		case multiple:
			// MULTIVALUEPATH makes each MULTIVALUE a full path (id 0 is the path), so a folder-spanning selection works.
			count := dlg.GetInt("MULTIVALUECOUNT")
			if count > 1 {
				for i := 1; i < count; i++ {
					ret = append(ret, iup.GetAttributeId(dlg, "MULTIVALUE", i))
				}
			} else if value := dlg.GetAttribute("VALUE"); value != "" {
				ret = append(ret, value)
			}
		default:
			ret = append(ret, dlg.GetAttribute("VALUE"))
		}

		rememberDir(dlg, dirKey)
	}

	return ret, nil
}

func saveDlg(title, dirKey string) string {
	dlg := iup.FileDlg()
	defer dlg.Destroy()

	dlg.SetAttributes(map[string]string{
		"DIALOGTYPE": "SAVE",
		"EXTFILTER":  "Comic Files|*.cbz;*.cbt|",
		"FILTER":     "*.cb*", // for Motif
		"TITLE":      title,
	})

	setStartDir(dlg, dirKey)

	iup.Popup(dlg, iup.CENTERPARENT, iup.CENTERPARENT)

	if dlg.GetInt("STATUS") == -1 {
		return ""
	}

	rememberDir(dlg, dirKey)

	return dlg.GetAttribute("VALUE")
}
