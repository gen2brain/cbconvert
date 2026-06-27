package main

import (
	"runtime/debug"
	"strings"

	"github.com/gen2brain/cbconvert/cmd/cbconvert-gui/i18n"
	"github.com/gen2brain/iup-go/iup"
)

// jxlLosslessBuild reports whether wasm2go and nodynamic leave zune-jpegxl (lossless-only) as the only jxl encoder.
var jxlLosslessBuild = func() bool {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return false
	}
	var wasm2go, nodynamic bool
	for _, kv := range info.Settings {
		if kv.Key != "-tags" {
			continue
		}
		for _, t := range strings.Split(kv.Value, ",") {
			switch t {
			case "wasm2go":
				wasm2go = true
			case "nodynamic":
				nodynamic = true
			}
		}
	}
	return wasm2go && nodynamic
}()

// userLossless is the user's Lossless preference, tracked separately because a jxl
// wasm2go build force-sets the widget on.
var userLossless bool

func setActive() {
	if busy {
		return
	}

	opts := options()
	count := iup.GetHandle("Table").GetInt("NUMLIN")

	if count > 0 && index != -1 {
		iup.GetHandle("PageBox").SetAttribute("VISIBLE", "YES")
	} else {
		iup.GetHandle("PageBox").SetAttribute("VISIBLE", "NO")
	}

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
		active, tip = "NO", i18n.Lng(i18n.StatusNeedFilesAndDir)
	case count == 0:
		active, tip = "NO", i18n.Lng(i18n.StatusNeedFiles)
	case opts.OutDir == "":
		active, tip = "NO", i18n.Lng(i18n.StatusNeedOutDir)
	}

	enabledTip := map[string]string{
		"Thumbnail": i18n.Lng(i18n.TipThumbnail),
		"Cover":     i18n.Lng(i18n.TipCover),
		"Convert":   i18n.Lng(i18n.TipConvert),
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
	jxlLossless := jxlLosslessBuild && opts.Format == "jxl"

	// jxl wasm2go forces lossless on; otherwise show the user's preference so it doesn't stay stuck on.
	losslessVal := "OFF"
	if jxlLossless || userLossless {
		losslessVal = "ON"
	}
	iup.GetHandle("Lossless").SetAttribute("VALUE", losslessVal)
	losslessOn := jxlLossless || (canLossless && userLossless)

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

	if jxlLossless {
		iup.GetHandle("Lossless").SetAttribute("ACTIVE", "NO")
		iup.GetHandle("VboxEffort").SetAttribute("ACTIVE", "NO")
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

// setBusy locks the UI while an operation runs and turns Convert into a Cancel button.
func setBusy(on bool) {
	busy = on

	// Controls not governed by setActive; setActive owns the rest.
	always := "YES"
	if on {
		always = "NO"
	}
	for _, h := range []string{"AddFiles", "AddDir", "Profile", "Reset", "Save", "Command", "Tabs", "Table"} {
		iup.GetHandle(h).SetAttribute("ACTIVE", always)
	}

	convert := iup.GetHandle("Convert")
	if on {
		for _, h := range []string{"Remove", "RemoveAll", "Thumbnail", "Cover"} {
			iup.GetHandle(h).SetAttribute("ACTIVE", "NO")
		}
		convert.SetAttribute("ACTIVE", "YES")
		convert.SetAttribute("TITLE", i18n.Lng(i18n.BtnCancel))
		convert.SetAttribute("TIP", i18n.Lng(i18n.TipCancel))
	} else {
		activeConv = nil
		convert.SetAttribute("TITLE", i18n.Lng(i18n.BtnConvert))
		convert.SetAttribute("TIP", i18n.Lng(i18n.TipConvert))
		setActive() // restores the conditional buttons and option boxes
	}
}
