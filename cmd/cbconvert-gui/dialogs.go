package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/gen2brain/cbconvert"
	"github.com/gen2brain/cbconvert/cmd/cbconvert-gui/i18n"
	"github.com/gen2brain/iup-go/iup"
)

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
			"SHOWPREVIEW":    "YES",
			"PREVIEWWIDTH":   "240",
			"PREVIEWHEIGHT":  "320",
		})

		dlg.SetCallback("FILE_CB", iup.FileFunc(previewCover()))
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

const dlgPreviewName = "_FILEDLGPREVIEW_"

// previewPad insets the cover from the preview pane edges, in pixels per side.
const previewPad = 8

// previewCover returns a FILE_CB handler that draws the highlighted comic's cover in the dialog preview pane.
// Extracted covers are cached by path so re-highlighting a file doesn't re-extract it.
func previewCover() iup.FileFunc {
	const maxCache = 32

	cache := make(map[string]iup.Ihandle)
	order := make([]string, 0, maxCache)

	cover := func(path string, w, h int) iup.Ihandle {
		if img, ok := cache[path]; ok {
			return img
		}

		img := loadCover(path, w, h)
		cache[path] = img
		order = append(order, path)

		if len(order) > maxCache {
			old := order[0]
			order = order[1:]
			if oi := cache[old]; oi != 0 {
				oi.Destroy()
			}
			delete(cache, old)
		}

		return img
	}

	return func(ih iup.Ihandle, filename, status string) int {
		switch status {
		case "PAINT":
			iup.DrawBegin(ih)
			cw, ch := iup.DrawGetSize(ih)
			iup.DrawParentBackground(ih)

			if image := cover(filename, cw-2*previewPad, ch-2*previewPad); image != 0 {
				iup.SetHandle(dlgPreviewName, image)
				iw, iih, _ := iup.DrawGetImageInfo(dlgPreviewName)
				iup.DrawImage(ih, dlgPreviewName, (cw-iw)/2, (ch-iih)/2, iw, iih)
			} else {
				ih.SetAttribute("DRAWCOLOR", "128 128 128")
				noPreview := i18n.Str(i18n.NoPreview)
				tw, th := iup.DrawGetTextSize(ih, noPreview)
				iup.DrawText(ih, noPreview, (cw-tw)/2, (ch-th)/2, 0, 0)
			}

			iup.DrawEnd(ih)
		case "FINISH":
			for _, img := range cache {
				if img != 0 {
					img.Destroy()
				}
			}
			cache = make(map[string]iup.Ihandle)
			order = order[:0]
		}

		return iup.DEFAULT
	}
}

// loadCover extracts the cover of a comic file and returns it as an IUP image fitted to w by h, or 0.
func loadCover(path string, w, h int) iup.Ihandle {
	if w <= 0 || h <= 0 || !isComic(path) {
		return 0
	}

	fi, err := os.Stat(path)
	if err != nil || fi.IsDir() {
		return 0
	}

	opts := cbconvert.NewOptions()
	opts.DPI = 96

	img, err := cbconvert.New(opts).CoverPreview(path, fi, w, h)
	if err != nil || img.Image == nil {
		return 0
	}

	return iup.ImageFromImage(img.Image)
}

func isComic(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".rar", ".zip", ".7z", ".tar", ".cbr", ".cbz", ".cb7", ".cbt",
		".pdf", ".xps", ".epub", ".mobi", ".docx", ".pptx", ".xlsx":
		return true
	}

	return false
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
