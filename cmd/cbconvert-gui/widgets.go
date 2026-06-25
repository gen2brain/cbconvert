package main

import (
	"bytes"
	"fmt"
	"image/gif"
	"math"
	"net/url"
	"strconv"
	"strings"

	"github.com/gen2brain/cbconvert"
	"github.com/gen2brain/cbconvert/cmd/cbconvert-gui/i18n"
	"github.com/gen2brain/iup-go/iup"
)

func setEffort(format string) {
	val := iup.GetHandle("Effort")

	var name string

	switch format {
	case "webp":
		val.SetAttributes("MIN=0, MAX=6, SHOWTICKS=7, VALUE=4")
		val.SetAttribute("TIP", i18n.Lng(i18n.TipEffortWebp))
		name = i18n.Str(i18n.EffortMethod)
	case "avif":
		val.SetAttributes("MIN=0, MAX=10, SHOWTICKS=11, VALUE=10")
		val.SetAttribute("TIP", i18n.Lng(i18n.TipEffortAvif))
		name = i18n.Str(i18n.EffortSpeed)
	case "jxl":
		val.SetAttributes("MIN=1, MAX=10, SHOWTICKS=10, VALUE=7")
		val.SetAttribute("TIP", i18n.Lng(i18n.TipEffortJxl))
		name = i18n.Str(i18n.EffortEffort)
	default:
		return
	}

	val.SetAttribute("EFFORTNAME", name)
	iup.GetHandle("LabelEffort").SetAttribute("TITLE", fmt.Sprintf("%s: %d", name, val.GetInt("VALUE")))
	iup.Refresh(iup.GetHandle("LabelEffort"))
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
		"TITLE1":         i18n.Lng(i18n.ColTitle),
		"TITLE2":         i18n.Lng(i18n.ColType),
		"TITLE3":         i18n.Lng(i18n.ColSize),
		"WIDTH1":         "300",
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

	t.SetCallback("SORT_CB", iup.TableSortFunc(onSort))

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

		addFiles(fs)

		return iup.DEFAULT
	}))

	return iup.Vbox(t)
}

func preview() iup.Ihandle {
	return iup.Frame(
		iup.Vbox(
			iup.Canvas().SetAttributes("EXPAND=YES, MINSIZE=400x, BORDER=NO").SetHandle("Preview").
				SetCallback("ACTION", iup.ActionFunc(drawPreview)).
				SetCallback("POSTMESSAGE_CB", iup.PostMessageFunc(previewMessage)),
			iup.Label("").SetAttributes("EXPAND=HORIZONTAL, ALIGNMENT=ACENTER").SetHandle("PreviewInfo"),
		),
	)
}

// drawPreview draws the cover scaled to fit, else the logo centered.
func drawPreview(ih iup.Ihandle) int {
	iup.DrawBegin(ih)
	defer iup.DrawEnd(ih)

	cw, ch := iup.DrawGetSize(ih)
	iup.DrawParentBackground(ih)

	name := "logo"
	if hasCover {
		name = "cover"
	}

	iw, ihh, _ := iup.DrawGetImageInfo(name)
	if iw <= 0 || ihh <= 0 {
		return iup.DEFAULT
	}

	dw, dh := iw, ihh
	if hasCover {
		s := math.Min(float64(cw)/float64(iw), float64(ch)/float64(ihh))
		dw = int(float64(iw) * s)
		dh = int(float64(ihh) * s)
	}

	iup.DrawImage(ih, name, (cw-dw)/2, (ch-dh)/2, dw, dh)

	return iup.DEFAULT
}

// previewMessage receives a rendered cover from previewRender and triggers a canvas redraw.
func previewMessage(ih iup.Ihandle, s string, i int, p any) int {
	if i != previewPage {
		return iup.DEFAULT
	}

	img := p.(cbconvert.Image)
	iup.GetHandle("Loading").SetAttributes("VISIBLE=NO, STOP=YES")

	if img.Image != nil && len(s) == 0 {
		iup.Destroy(iup.GetHandle("cover"))
		iup.ImageFromImage(img.Image).SetHandle("cover")
		hasCover = true
		iup.GetHandle("PreviewInfo").SetAttribute("TITLE", fmt.Sprintf("%s (%dx%d)", img.SizeHuman, img.Width, img.Height))
	} else {
		iup.Destroy(iup.GetHandle("cover"))
		hasCover = false
		iup.GetHandle("PreviewInfo").SetAttribute("TITLE", "")

		sp := strings.Split(s, ": ")
		if len(sp) > 1 {
			iup.MessageError(ih, fmt.Sprintf("%s\n\n%s", sp[0], strings.Join(sp[1:], ": ")))
		}
	}

	iup.Update(ih)

	return iup.DEFAULT
}

// pageBox is the page-navigation spin shown in the status bar; hidden until a comic is selected.
func pageBox() iup.Ihandle {
	return iup.Hbox(
		iup.Space().SetAttribute("SIZE", "5"),
		iup.Label(i18n.Lng(i18n.LblPage)),
		iup.Space().SetAttribute("SIZE", "3"),
		iup.Text().SetAttributes(`SPIN=YES, SPINMIN=1, SPINMAX=1, VALUE=1, VISIBLECOLUMNS=3, MASK="/d*"`).SetHandle("Page").
			SetAttribute("TIP", i18n.Lng(i18n.TipPage)).
			SetCallback("SPIN_CB", iup.SpinFunc(func(ih iup.Ihandle, pos int) int {
				return onPageChanged()
			})).
			SetCallback("VALUECHANGED_CB", iup.ValueChangedFunc(func(ih iup.Ihandle) int {
				return onPageChanged()
			})).
			SetCallback("POSTMESSAGE_CB", iup.PostMessageFunc(func(ih iup.Ihandle, s string, i int, p any) int {
				if s != previewPath {
					return iup.DEFAULT
				}

				ih.SetAttribute("SPINMAX", strconv.Itoa(i))
				iup.GetHandle("PageCount").SetAttribute("TITLE", fmt.Sprintf("/ %d", i))

				if previewPage > i-1 {
					previewPage = i - 1
				}
				if previewPage < 0 {
					previewPage = 0
				}
				ih.SetAttribute("VALUE", strconv.Itoa(previewPage+1))

				iup.Refresh(iup.GetHandle("PageBox"))

				previewRender()

				return iup.DEFAULT
			})),
		iup.Space().SetAttribute("SIZE", "3"),
		iup.Label("").SetHandle("PageCount"),
	).SetAttributes("ALIGNMENT=ACENTER, VISIBLE=NO").SetHandle("PageBox")
}

func tabInput() iup.Ihandle {
	return iup.Hbox(
		iup.Vbox(
			iup.Toggle(i18n.Lng(i18n.TglRecursive)).SetHandle("Recursive").
				SetAttribute("TIP", i18n.Lng(i18n.TipRecursive)),
			iup.Toggle(i18n.Lng(i18n.TglNoRGB)).SetHandle("NoRGB").
				SetAttribute("TIP", i18n.Lng(i18n.TipNoRGB)),
			iup.Toggle(i18n.Lng(i18n.TglNoCover)).SetHandle("NoCover").
				SetAttribute("TIP", i18n.Lng(i18n.TipNoCover)),
			iup.Toggle(i18n.Lng(i18n.TglNoNonImage)).SetHandle("NoNonImage").
				SetAttribute("TIP", i18n.Lng(i18n.TipNoNonImage)),
			iup.Toggle(i18n.Lng(i18n.TglNoConvert)).SetHandle("NoConvert").
				SetAttribute("TIP", i18n.Lng(i18n.TipNoConvert)).
				SetCallback("VALUECHANGED_CB", iup.ValueChangedFunc(func(ih iup.Ihandle) int {
					setActive()

					return iup.DEFAULT
				})),
		).SetAttributes("NGAP=10"),
		iup.Space().SetAttribute("SIZE", "15"),
		iup.Vbox(
			iup.Vbox(
				iup.Label(i18n.Lng(i18n.LblMinSize)),
				iup.Text().SetAttributes(`SPIN=YES, SPINMAX=2048, VISIBLECOLUMNS=4, MASK="/d*"`).SetHandle("Size").
					SetAttribute("TIP", i18n.Lng(i18n.TipSize)),
			),
			iup.Vbox(
				iup.Label(i18n.Lng(i18n.LblDPI)),
				iup.List().SetAttributes(map[string]string{
					"DROPDOWN":       "YES",
					"EDITBOX":        "YES",
					"VISIBLECOLUMNS": "6",
					"VALUE":          "Default",
					"1":              "Default",
					"2":              "150",
					"3":              "300",
					"4":              "600",
					"5":              "1200",
				}).SetHandle("DPI").
					SetAttribute("TIP", i18n.Lng(i18n.TipDPI)),
			),
		).SetAttributes("NGAP=10"),
	).SetHandle("VboxInput")
}

func tabOutput() iup.Ihandle {
	return iup.Hbox(
		iup.Vbox(
			iup.Vbox(
				iup.Label(i18n.Lng(i18n.LblOutDir)),
				iup.Text().SetAttributes("VISIBLECOLUMNS=16, MINSIZE=100x").SetHandle("OutDir").
					SetAttribute("TIP", i18n.Lng(i18n.TipOutDir)).
					SetCallback("VALUECHANGED_CB", iup.ValueChangedFunc(func(ih iup.Ihandle) int {
						setActive()

						return iup.DEFAULT
					})),
				iup.Space().SetAttribute("SIZE", "5x0"),
				iup.Button(i18n.Lng(i18n.BtnBrowse)).SetAttributes("PADDING=DEFAULTBUTTONPADDING").
					SetCallback("ACTION", iup.ActionFunc(onOutputDirectory)),
			),
			iup.Vbox(
				iup.Label(i18n.Lng(i18n.LblSuffix)),
				iup.Text().SetAttributes("VISIBLECOLUMNS=16, MINSIZE=100x").SetHandle("Suffix").
					SetAttribute("TIP", i18n.Lng(i18n.TipSuffix)),
			),
			iup.Vbox(
				iup.Label(i18n.Lng(i18n.LblArchive)),
				iup.List().SetAttributes(map[string]string{
					"DROPDOWN": "YES",
					"VALUE":    "1",
					"1":        "ZIP",
					"2":        "TAR",
				}).SetHandle("Archive").
					SetAttribute("TIP", i18n.Lng(i18n.TipArchive)).
					SetCallback("VALUECHANGED_CB", iup.ValueChangedFunc(func(ih iup.Ihandle) int {
						setActive()

						return iup.DEFAULT
					})),
			),
			iup.Vbox(
				iup.Label(i18n.Lng(i18n.LblCompression)),
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
					SetAttribute("TIP", i18n.Lng(i18n.TipZipLevel)),
			).SetHandle("VboxZipLevel"),
		).SetAttributes("NGAP=10"),
		iup.Space().SetAttribute("SIZE", "15"),
		iup.Vbox(
			iup.Vbox(
				iup.Toggle(i18n.Lng(i18n.TglCombine)).SetHandle("Combine").
					SetAttribute("TIP", i18n.Lng(i18n.TipCombine)).
					SetCallback("VALUECHANGED_CB", iup.ValueChangedFunc(func(ih iup.Ihandle) int {
						setActive()

						return iup.DEFAULT
					})),
			),
			iup.Vbox(
				iup.Label(i18n.Lng(i18n.LblOutFile)),
				iup.Text().SetAttributes("VISIBLECOLUMNS=16, MINSIZE=100x").SetHandle("OutFile").
					SetAttribute("TIP", i18n.Lng(i18n.TipOutFile)),
				iup.Space().SetAttribute("SIZE", "5x0"),
				iup.Button(i18n.Lng(i18n.BtnBrowse)).SetAttributes("PADDING=DEFAULTBUTTONPADDING").
					SetCallback("ACTION", iup.ActionFunc(onOutputFile)),
			).SetHandle("VboxOutFile"),
		).SetAttributes("NGAP=10"),
	).SetHandle("VboxOutput")
}

func tabImage() iup.Ihandle {
	return iup.Hbox(
		iup.Vbox(
			iup.Vbox(
				iup.Label(i18n.Lng(i18n.LblFormat)),
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
					SetAttribute("TIP", i18n.Lng(i18n.TipFormat)).
					SetCallback("VALUECHANGED_CB", iup.ValueChangedFunc(func(ih iup.Ihandle) int {
						setEffort(strings.ToLower(ih.GetAttribute("VALUESTRING")))
						setActive()
						previewPost()

						return iup.DEFAULT
					})),
			),
			iup.Vbox(
				iup.Label(i18n.Lng(i18n.LblSize)),
				iup.Hbox(
					iup.Text().SetAttributes(`VISIBLECOLUMNS=6, MASK="/d*"`).SetHandle("Width").
						SetAttribute("CUEBANNER", i18n.Lng(i18n.CueWidth)).
						SetAttribute("TIP", i18n.Lng(i18n.TipWidthHeight)).
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
					iup.Text().SetAttributes(`VISIBLECOLUMNS=6, MASK="/d*"`).SetHandle("Height").
						SetAttribute("CUEBANNER", i18n.Lng(i18n.CueHeight)).
						SetAttribute("TIP", i18n.Lng(i18n.TipWidthHeight)).
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
				iup.Toggle(i18n.Lng(i18n.TglFit)).SetHandle("Fit").
					SetAttribute("TIP", i18n.Lng(i18n.TipFit)),
				iup.Toggle(i18n.Lng(i18n.TglNoUpscale)).SetHandle("NoUpscale").
					SetAttribute("TIP", i18n.Lng(i18n.TipNoUpscale)),
			),
			iup.Vbox(
				iup.Label(i18n.Lng(i18n.LblFilter)),
				iup.List().SetAttributes(map[string]string{
					"DROPDOWN": "YES",
					"VALUE":    "3",
					"TIP":      i18n.Lng(i18n.FilterLinear),
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
					iup.Label(i18n.Lng(i18n.LblQuality)),
					iup.Label("75").SetHandle("LabelQuality"),
				).SetAttributes("NMARGIN=0"),
				iup.Val("").SetAttributes(`MIN=0, MAX=100, VALUE=75, SHOWTICKS=10`).SetHandle("Quality").
					SetAttribute("TIP", i18n.Lng(i18n.TipQuality)).
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
				iup.Label(i18n.Lng(i18n.LblEffort)).SetHandle("LabelEffort"),
				iup.Val("").SetAttributes(`MIN=0, MAX=10, VALUE=0, SHOWTICKS=11`).SetHandle("Effort").
					SetAttribute("TIP", i18n.Lng(i18n.TipEffort)).
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
				iup.Toggle(i18n.Lng(i18n.TglLossless)).SetHandle("Lossless").
					SetAttribute("TIP", i18n.Lng(i18n.TipLossless)).
					SetCallback("VALUECHANGED_CB", iup.ValueChangedFunc(func(ih iup.Ihandle) int {
						setActive()
						previewPost()

						return iup.DEFAULT
					})),
			),
			iup.Vbox(
				iup.Toggle(i18n.Lng(i18n.TglGrayscale)).SetHandle("Grayscale").
					SetAttribute("TIP", i18n.Lng(i18n.TipGrayscale)).
					SetCallback("VALUECHANGED_CB", iup.ValueChangedFunc(func(ih iup.Ihandle) int {
						previewPost()

						return iup.DEFAULT
					})),
			),
		).SetAttributes("NGAP=10"),
	).SetHandle("VboxImage")
}

func tabTransform() iup.Ihandle {
	return iup.Vbox(
		iup.Vbox(
			iup.Hbox(
				iup.Label(i18n.Lng(i18n.LblBrightness)),
				iup.Label("0").SetHandle("LabelBrightness"),
			).SetAttributes("ALIGNMENT=ACENTER, NMARGIN=0"),
			iup.Val("").SetAttributes(`MIN=-100, MAX=100, VALUE=0, SHOWTICKS=10`).SetHandle("Brightness").
				SetAttribute("TIP", i18n.Lng(i18n.TipBrightness)).
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
				iup.Label(i18n.Lng(i18n.LblContrast)),
				iup.Label("0").SetHandle("LabelContrast"),
			).SetAttributes("ALIGNMENT=ACENTER, NMARGIN=0"),
			iup.Val("").SetAttributes(`MIN=-100, MAX=100, VALUE=0, SHOWTICKS=10`).SetHandle("Contrast").
				SetAttribute("TIP", i18n.Lng(i18n.TipContrast)).
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
			iup.Label(i18n.Lng(i18n.LblRotate)),
			iup.List().SetAttributes(map[string]string{
				"DROPDOWN": "YES",
				"VALUE":    "1",
				"1":        "0",
				"2":        "90",
				"3":        "180",
				"4":        "270",
			}).SetHandle("Rotate").
				SetAttribute("TIP", i18n.Lng(i18n.TipRotate)).
				SetCallback("VALUECHANGED_CB", iup.ValueChangedFunc(func(ih iup.Ihandle) int {
					previewPost()

					return iup.DEFAULT
				})),
		),
	).SetHandle("VboxTransform").SetAttributes("NGAP=10")
}

func tabs() iup.Ihandle {
	return iup.Tabs(
		tabInput().SetAttributes("TABTITLE="+i18n.Lng(i18n.TabInput)+", NMARGIN=10x10"),
		tabOutput().SetAttributes("TABTITLE="+i18n.Lng(i18n.TabOutput)+", NMARGIN=10x10"),
		tabImage().SetAttributes("TABTITLE="+i18n.Lng(i18n.TabImage)+", NMARGIN=10x10"),
		tabTransform().SetAttributes("TABTITLE="+i18n.Lng(i18n.TabTransform)+", NMARGIN=10x10"),
	).SetHandle("Tabs")
}

func buttons() iup.Ihandle {
	addFiles := iup.Button(i18n.Lng(i18n.BtnAddFiles)).SetHandle("AddFiles").SetAttributes("PADDING=DEFAULTBUTTONPADDING").
		SetCallback("ACTION", iup.ActionFunc(onAddFiles))
	addDir := iup.Button(i18n.Lng(i18n.BtnAddDir)).SetHandle("AddDir").SetAttributes("PADDING=DEFAULTBUTTONPADDING").
		SetCallback("ACTION", iup.ActionFunc(onAddDir))
	remove := iup.Button(i18n.Lng(i18n.BtnRemove)).SetHandle("Remove").SetAttributes("PADDING=DEFAULTBUTTONPADDING").
		SetCallback("ACTION", iup.ActionFunc(onRemove))
	removeAll := iup.Button(i18n.Lng(i18n.BtnRemoveAll)).SetHandle("RemoveAll").SetAttributes("PADDING=DEFAULTBUTTONPADDING").
		SetCallback("ACTION", iup.ActionFunc(onRemoveAll))
	thumbnail := iup.Button(i18n.Lng(i18n.BtnThumbnail)).SetHandle("Thumbnail").SetAttributes("PADDING=DEFAULTBUTTONPADDING").
		SetCallback("ACTION", iup.ActionFunc(onThumbnail))
	cover := iup.Button(i18n.Lng(i18n.BtnCover)).SetHandle("Cover").SetAttributes("PADDING=DEFAULTBUTTONPADDING").
		SetCallback("ACTION", iup.ActionFunc(onCover))
	convert := iup.Button(i18n.Lng(i18n.BtnConvert)).SetHandle("Convert").SetAttributes("PADDING=DEFAULTBUTTONPADDING").
		SetCallback("ACTION", iup.ActionFunc(onConvert))
	reset := iup.Button(i18n.Lng(i18n.BtnReset)).SetHandle("Reset").SetAttributes("PADDING=DEFAULTBUTTONPADDING").
		SetAttribute("TIP", i18n.Lng(i18n.TipReset)).
		SetCallback("ACTION", iup.ActionFunc(onReset))
	save := iup.Button(i18n.Lng(i18n.BtnSave)).SetHandle("Save").SetAttributes("PADDING=DEFAULTBUTTONPADDING").
		SetAttribute("TIP", i18n.Lng(i18n.TipSave)).
		SetCallback("ACTION", iup.ActionFunc(onSave))

	command := iup.Button(i18n.Lng(i18n.BtnCommand)).SetHandle("Command").SetAttributes("PADDING=DEFAULTBUTTONPADDING").
		SetAttribute("TIP", i18n.Lng(i18n.TipCommand)).
		SetCallback("ACTION", iup.ActionFunc(onCommand))

	profile := iup.List().SetAttributes("DROPDOWN=YES").SetHandle("Profile").
		SetAttribute("TIP", i18n.Lng(i18n.TipProfile)).
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
		iup.Space().SetAttribute("SIZE", "x5"),
		iup.Vbox(
			thumbnail,
			cover,
		).SetAttribute("NGAP", "2"),
		iup.Space().SetAttribute("SIZE", "x5"),
		iup.Vbox(
			convert,
		),
		iup.Fill(),
		iup.Vbox(
			iup.Label(i18n.Lng(i18n.LblProfile)),
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
		pageBox(),
		iup.Fill(),
		iup.Label("File 1 of 1").SetHandle("LabelStatus1").SetAttributes("VISIBLE=NO"),
		iup.Space().SetAttribute("SIZE", "5"),
		iup.Label("(000/000)").SetHandle("LabelStatus2").SetAttributes("VISIBLE=NO"),
		iup.Space().SetAttribute("SIZE", "5"),
		iup.ProgressBar().SetAttributes("VISIBLE=NO").SetHandle("ProgressBar").
			SetCallback("POSTMESSAGE_CB", iup.PostMessageFunc(func(ih iup.Ihandle, s string, i int, p any) int {
				switch s {
				case "convert":
					conv := p.(*cbconvert.Converter)
					ih.SetAttributes("VALUE=0, VISIBLE=YES")
					ih.SetAttribute("MAX", conv.Ncontents)

					iup.GetHandle("LabelStatus1").SetAttribute("TITLE", fmt.Sprintf(i18n.Str(i18n.StatusFileOf), conv.CurrFile, conv.Nfiles))
					iup.GetHandle("LabelStatus1").SetAttributes("VISIBLE=YES")
					iup.GetHandle("LabelStatus2").SetAttributes("VISIBLE=YES")

					iup.Refresh(iup.GetHandle("StatusBar"))
				case "start":
					conv := p.(*cbconvert.Converter)
					ih.SetAttributes("VALUE=0, VISIBLE=YES")
					ih.SetAttribute("MAX", conv.Nfiles)

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
					setBusy(false)

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
