package main

import "github.com/gen2brain/iup-go/iup"

func setActive() {
	if busy {
		return
	}

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
		convert.SetAttribute("TITLE", "Cancel")
		convert.SetAttribute("TIP", "Cancel the running operation (or press Esc)")
	} else {
		activeConv = nil
		convert.SetAttribute("TITLE", "&Convert")
		convert.SetAttribute("TIP", "Convert files to the selected format")
		setActive() // restores the conditional buttons and option boxes
	}
}
