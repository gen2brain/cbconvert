package main

import (
	"strconv"
	"strings"

	"github.com/gen2brain/cbconvert"
	"github.com/gen2brain/cbconvert/cmd/cbconvert-gui/i18n"
	"github.com/gen2brain/iup-go/iup"
)

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
	opts.DPI = dpiValue(iup.GetHandle("DPI").GetAttribute("VALUE"))
	opts.Fit = iup.GetHandle("Fit").GetAttribute("VALUE") == "ON"
	opts.NoUpscale = iup.GetHandle("NoUpscale").GetAttribute("VALUE") == "ON"
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
	iup.GetText(i18n.Str(i18n.DlgCommandLine), commandLine(), -1)

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

func dpiValue(value string) int {
	dpi, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0
	}

	return dpi
}
