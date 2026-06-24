package main

import "github.com/gen2brain/iup-go/iup"

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
