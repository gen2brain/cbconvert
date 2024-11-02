//go:build !portal

package main

import (
	"path/filepath"
	"strings"

	"github.com/gen2brain/iup-go/iup"
)

func fileDlg(title string, multiple, directory bool) ([]string, error) {
	ret := make([]string, 0)

	dlg := iup.FileDlg()
	defer dlg.Destroy()

	if !directory {
		mf := "YES"
		if !multiple {
			mf = "NO"
		}

		dlg.SetAttributes(map[string]string{
			"DIALOGTYPE":    "OPEN",
			"MULTIPLEFILES": mf,
			"EXTFILTER":     "Comic Files|*.rar;*.zip;*.7z;*.tar;*.cbr;*.cbz;*.cb7;*.cbt;*.pdf;*.epub;*.mobi;*.docx;*.pptx|",
			"FILTER":        "*.cb*", // for Motif
			"TITLE":         title,
		})
	} else {
		dlg.SetAttributes(map[string]string{
			"DIALOGTYPE": "DIR",
			"TITLE":      title,
		})
	}

	iup.Popup(dlg, iup.CENTERPARENT, iup.CENTERPARENT)

	if dlg.GetInt("STATUS") == 0 {
		if !directory {
			value := dlg.GetAttribute("VALUE")
			sp := strings.Split(value, "|")

			if strings.ToLower(iup.GetGlobal("DRIVER")) == "cocoa" {
				for _, file := range sp {
					ret = append(ret, file)
				}
			} else {
				if len(sp) > 1 {
					for _, file := range sp[1 : len(sp)-1] {
						ret = append(ret, filepath.Join(sp[0], file))
					}
				} else {
					ret = append(ret, value)
				}
			}
		} else {
			value := dlg.GetAttribute("VALUE")
			ret = append(ret, value)
		}
	}

	return ret, nil
}
