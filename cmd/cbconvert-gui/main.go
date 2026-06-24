package main

import (
	"bytes"
	_ "embed"
	"flag"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"runtime/debug"
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

	activeConv *cbconvert.Converter
	busy       bool
)

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
