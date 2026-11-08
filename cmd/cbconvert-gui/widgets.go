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
		val.SetAttributes("MIN=0, MAX=10, SHOWTICKS=11, VALUE=6")
		val.SetAttribute("TIP", i18n.Lng(i18n.TipEffortAvif))
		name = i18n.Str(i18n.EffortSpeed)
	case "jxl":
		val.SetAttributes("MIN=1, MAX=7, SHOWTICKS=7, VALUE=7")
		val.SetAttribute("TIP", i18n.Lng(i18n.TipEffortJxl))
		name = i18n.Str(i18n.EffortEffort)
	default:
		return
	}

	val.SetAttribute("EFFORTNAME", name)
	iup.GetHandle("LabelEffort").SetAttribute("TITLE", fmt.Sprintf("%s: %d", name, val.GetInt("VALUE")))
	iup.Refresh(iup.GetHandle("LabelEffort"))
}

func list() iup.Ihandle {
	t := iup.Table().SetHandle("Table")
	t.SetAttributes(map[string]string{
		"EXPAND":        "YES",
		"NUMCOL":        "3",
		"NUMLIN":        "0",
		"TITLE1":        i18n.Lng(i18n.ColTitle),
		"TITLE2":        i18n.Lng(i18n.ColType),
		"TITLE3":        i18n.Lng(i18n.ColSize),
		"WIDTH1":        "250",
		"WIDTH2":        "50",
		"WIDTH3":        "100",
		"ALIGNMENT2":    "ACENTER",
		"ALIGNMENT3":    "ARIGHT",
		"SELECTIONMODE": "MULTIPLE",
		"USERRESIZE":    "YES",
		"STRETCHLAST":   "NO",
		"FOCUSFEEDBACK": "NO",
		"SORTABLE":      "YES",
		"ALTERNATEROWS": "YES",
	})

	t.SetCallback("ENTERITEM_CB", iup.EnterItemFunc(func(ih iup.Ihandle, lin, col int) int {
		index = lin - 1
		setActive()
		previewPost()

		return iup.DEFAULT
	}))

	t.SetCallback("SORT_CB", iup.TableSortFunc(onSort))

	t.SetCallback("K_ANY", iup.KAnyFunc(func(ih iup.Ihandle, c int) int {
		if c == iup.K_DEL {
			return onRemove(ih)
		}

		return iup.DEFAULT
	}))

	t.SetCallback("RIGHTCLICK_CB", iup.TableRightClickFunc(func(ih iup.Ihandle, lin, col int) int {
		menu := iup.Menu(
			iup.MenuItem(i18n.Lng(i18n.MenuOpenFolder)).SetCallback("ACTION", iup.ActionFunc(onOpenFolder)),
			iup.MenuSeparator(),
			iup.MenuItem(i18n.Lng(i18n.BtnRemove)).SetCallback("ACTION", iup.ActionFunc(onRemove)),
			iup.MenuItem(i18n.Lng(i18n.BtnRemoveAll)).SetCallback("ACTION", iup.ActionFunc(onRemoveAll)),
		)
		iup.Popup(menu, iup.MOUSEPOS, iup.MOUSEPOS)
		menu.Destroy()

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

		addFiles(fs)

		return iup.DEFAULT
	}))

	return iup.Vbox(t)
}

func preview() iup.Ihandle {
	iup.Timer().SetAttribute("TIME", "300").SetHandle("PreviewTimer").
		SetCallback("ACTION_CB", iup.TimerActionFunc(func(ih iup.Ihandle) int {
			ih.SetAttribute("RUN", "NO")
			previewPost()

			return iup.DEFAULT
		}))

	return iup.Frame(
		iup.Vbox(
			iup.Canvas().SetAttributes("EXPAND=YES, MINSIZE=400x, BORDER=NO").SetHandle("Preview").
				SetCallback("ACTION", iup.ActionFunc(drawPreview)).
				SetCallback("RESIZE_CB", iup.ResizeFunc(onPreviewResize)).
				SetCallback("WHEEL_CB", iup.WheelFunc(onPreviewWheel)).
				SetCallback("BUTTON_CB", iup.ButtonFunc(onPreviewButton)).
				SetCallback("MOTION_CB", iup.MotionFunc(onPreviewMotion)).
				SetCallback("POSTMESSAGE_CB", iup.PostMessageFunc(previewMessage)),
			iup.Label("").SetAttributes("EXPAND=HORIZONTAL, ALIGNMENT=ACENTER").SetHandle("PreviewInfo"),
		),
	)
}

// previewTheme holds the preview canvas colors, taken from the IUP palette of the current appearance.
type previewTheme struct {
	bg, card          string
	cardBorder, frame string
}

func previewThemeOf(ih iup.Ihandle) previewTheme {
	fg := iup.GetGlobal("DLGFGCOLOR")

	return previewTheme{
		bg:         ih.GetAttribute("BGCOLOR"),
		card:       iup.GetGlobal("TXTBGCOLOR"),
		cardBorder: fg + " 64",
		frame:      fg + " 96",
	}
}

// Light and shadow are drawn as translucent white and black over the theme colors.
const (
	previewLight  = "255 255 255 20"
	previewShade  = "0 0 0 32"
	previewShadow = "0 0 0 72"
)

// drawPreview paints a gradient background, then the cover framed with a drop shadow, or an empty page card with the logo.
func drawPreview(ih iup.Ihandle) int {
	iup.DrawBegin(ih)
	defer iup.DrawEnd(ih)

	cw, ch := iup.DrawGetSize(ih)
	th := previewThemeOf(ih)

	ih.SetAttribute("DRAWSTYLE", "FILL")
	ih.SetAttribute("DRAWCOLOR", th.bg)
	iup.DrawRectangle(ih, 0, 0, cw, ch)
	iup.DrawLinearGradient(ih, 0, 0, cw, ch, 270, previewLight, previewShade)

	margin := previewMargin(cw, ch)

	if hasCover {
		drawCover(ih, th, cw, ch, margin)
	} else {
		drawPlaceholder(ih, th, cw, ch, margin)
	}

	return iup.DEFAULT
}

// previewMargin is the space kept around the cover or the placeholder card.
func previewMargin(cw, ch int) int {
	margin := cw / 24
	if m := ch / 24; m < margin {
		margin = m
	}
	if margin < 12 {
		margin = 12
	}

	return margin
}

// coverRect is where the cover is drawn: fitted inside the margins, then zoomed and panned.
func coverRect(cw, ch, margin, iw, ihh int) (x, y, dw, dh int) {
	s := math.Min(float64(cw-2*margin)/float64(iw), float64(ch-2*margin)/float64(ihh)) * previewZoom
	dw, dh = int(float64(iw)*s), int(float64(ihh)*s)

	return (cw-dw)/2 + previewPanX, (ch-dh)/2 + previewPanY, dw, dh
}

// clampPan keeps a zoomed cover covering the canvas inside the margins, and a fitted cover centered.
func clampPan(cw, ch, margin, dw, dh int) {
	limit := func(pan, size, view int) int {
		m := max((size-view)/2, 0)
		return min(max(pan, -m), m)
	}

	previewPanX = limit(previewPanX, dw, cw-2*margin)
	previewPanY = limit(previewPanY, dh, ch-2*margin)
}

// setZoom zooms the preview around the canvas point x, y, keeping the cover point under it in place.
func setZoom(ih iup.Ihandle, zoom float64, x, y int) {
	zoom = min(max(zoom, 1), 8)
	if zoom == previewZoom {
		return
	}

	iw, ihh, _ := iup.DrawGetImageInfo("cover")
	if !hasCover || iw <= 0 || ihh <= 0 {
		return
	}

	_, cw, ch := ih.GetInt2("DRAWSIZE")
	margin := previewMargin(cw, ch)

	ox, oy, dw, dh := coverRect(cw, ch, margin, iw, ihh)
	fx, fy := float64(x-ox)/float64(dw), float64(y-oy)/float64(dh)

	previewZoom = zoom
	previewPanX, previewPanY = 0, 0
	nx, ny, ndw, ndh := coverRect(cw, ch, margin, iw, ihh)
	previewPanX = x - int(fx*float64(ndw)) - nx
	previewPanY = y - int(fy*float64(ndh)) - ny
	clampPan(cw, ch, margin, ndw, ndh)

	if previewZoom > 1 {
		ih.SetAttribute("CURSOR", "MOVE")
	} else {
		ih.SetAttribute("CURSOR", "ARROW")
	}

	iup.Update(ih)
	onPreviewResize(ih, cw, ch)
}

// resetZoom shows the whole page again.
func resetZoom() {
	previewZoom = 1
	previewPanX, previewPanY = 0, 0
	iup.GetHandle("Preview").SetAttribute("CURSOR", "ARROW")
}

// onPreviewWheel zooms the preview around the mouse pointer.
func onPreviewWheel(ih iup.Ihandle, delta float64, x, y int, status string) int {
	setZoom(ih, previewZoom*math.Pow(1.25, delta), x, y)

	return iup.DEFAULT
}

// onPreviewButton starts and ends a drag of the zoomed preview; a double-click shows the whole page.
func onPreviewButton(ih iup.Ihandle, button, pressed, x, y int, status string) int {
	if button != iup.BUTTON1 {
		return iup.DEFAULT
	}

	if pressed == 1 && iup.IsDouble(status) {
		resetZoom()
		iup.Update(ih)

		return iup.DEFAULT
	}

	previewDrag = pressed == 1 && previewZoom > 1
	previewDragX, previewDragY = x, y

	return iup.DEFAULT
}

// onPreviewMotion pans the zoomed preview while the left button is held.
func onPreviewMotion(ih iup.Ihandle, x, y int, status string) int {
	if !previewDrag || !iup.IsButton1(status) {
		return iup.DEFAULT
	}

	iw, ihh, _ := iup.DrawGetImageInfo("cover")
	if !hasCover || iw <= 0 || ihh <= 0 {
		return iup.DEFAULT
	}

	_, cw, ch := ih.GetInt2("DRAWSIZE")
	margin := previewMargin(cw, ch)

	previewPanX += x - previewDragX
	previewPanY += y - previewDragY
	previewDragX, previewDragY = x, y

	_, _, dw, dh := coverRect(cw, ch, margin, iw, ihh)
	clampPan(cw, ch, margin, dw, dh)
	iup.Update(ih)

	return iup.DEFAULT
}

// drawCover draws the loaded cover scaled to fit, with a drop shadow and a thin frame.
func drawCover(ih iup.Ihandle, th previewTheme, cw, ch, margin int) {
	iw, ihh, _ := iup.DrawGetImageInfo("cover")
	if iw <= 0 || ihh <= 0 {
		return
	}

	x, y, dw, dh := coverRect(cw, ch, margin, iw, ihh)

	const off = 6
	ih.SetAttribute("DRAWSTYLE", "FILL")
	ih.SetAttribute("DRAWCOLOR", previewShadow)
	iup.DrawRectangle(ih, x+off, y+off, x+dw+off, y+dh+off)

	iup.DrawImage(ih, "cover", x, y, dw, dh)

	ih.SetAttribute("DRAWSTYLE", "STROKE")
	ih.SetAttribute("DRAWLINEWIDTH", "1")
	ih.SetAttribute("DRAWCOLOR", th.frame)
	iup.DrawRectangle(ih, x, y, x+dw, y+dh)
}

// drawPlaceholder draws an empty comic-page card with the logo centered, shown until a cover is loaded.
func drawPlaceholder(ih iup.Ihandle, th previewTheme, cw, ch, margin int) {
	cardH := ch - 2*margin
	cardW := cardH * 2 / 3
	if cardW > cw-2*margin {
		cardW = cw - 2*margin
		cardH = cardW * 3 / 2
	}
	x, y := (cw-cardW)/2, (ch-cardH)/2

	r := cardW / 14
	if r < 8 {
		r = 8
	}

	const off = 7
	ih.SetAttribute("DRAWSTYLE", "FILL")
	ih.SetAttribute("DRAWCOLOR", previewShadow)
	iup.DrawRoundedRectangle(ih, x+off, y+off, x+cardW+off, y+cardH+off, r)

	ih.SetAttribute("DRAWCOLOR", th.card)
	iup.DrawRoundedRectangle(ih, x, y, x+cardW, y+cardH, r)

	ih.SetAttribute("DRAWSTYLE", "STROKE")
	ih.SetAttribute("DRAWLINEWIDTH", "1")
	ih.SetAttribute("DRAWCOLOR", th.cardBorder)
	iup.DrawRoundedRectangle(ih, x, y, x+cardW, y+cardH, r)

	lw, lh, _ := iup.DrawGetImageInfo("logo")
	if lw <= 0 || lh <= 0 {
		return
	}

	s := float64(cardW*2/5) / float64(lw)
	dw, dh := int(float64(lw)*s), int(float64(lh)*s)
	iup.DrawImage(ih, "logo", x+(cardW-dw)/2, y+(cardH-dh)/2, dw, dh)
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
		coverFull = img.Image.Bounds().Dx() >= img.Width
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
							previewLater()

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
							previewLater()

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
				iup.Val("").SetAttributes(`MIN=0, MAX=100, VALUE=75, SHOWTICKS=10, EXPAND=HORIZONTAL`).SetHandle("Quality").
					SetAttribute("TIP", i18n.Lng(i18n.TipQuality)).
					SetCallback("VALUECHANGED_CB", iup.ValueChangedFunc(func(ih iup.Ihandle) int {
						iup.GetHandle("LabelQuality").SetAttribute("TITLE", ih.GetInt("VALUE"))
						iup.Refresh(iup.GetHandle("LabelQuality"))
						previewLater()

						return iup.DEFAULT
					})),
			).SetHandle("VboxQuality"),
			iup.Vbox(
				iup.Label(i18n.Lng(i18n.LblEffort)).SetHandle("LabelEffort"),
				iup.Val("").SetAttributes(`MIN=0, MAX=10, VALUE=0, SHOWTICKS=11, EXPAND=HORIZONTAL`).SetHandle("Effort").
					SetAttribute("TIP", i18n.Lng(i18n.TipEffort)).
					SetCallback("VALUECHANGED_CB", iup.ValueChangedFunc(func(ih iup.Ihandle) int {
						iup.GetHandle("LabelEffort").SetAttribute("TITLE", fmt.Sprintf("%s: %d", ih.GetAttribute("EFFORTNAME"), ih.GetInt("VALUE")))
						iup.Refresh(iup.GetHandle("LabelEffort"))
						previewLater()

						return iup.DEFAULT
					})),
			).SetHandle("VboxEffort"),
			iup.Vbox(
				iup.Toggle(i18n.Lng(i18n.TglLossless)).SetHandle("Lossless").
					SetAttribute("TIP", i18n.Lng(i18n.TipLossless)).
					SetCallback("VALUECHANGED_CB", iup.ValueChangedFunc(func(ih iup.Ihandle) int {
						userLossless = ih.GetAttribute("VALUE") == "ON"
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
			iup.Val("").SetAttributes(`MIN=-100, MAX=100, VALUE=0, SHOWTICKS=10, EXPAND=HORIZONTAL`).SetHandle("Brightness").
				SetAttribute("TIP", i18n.Lng(i18n.TipBrightness)).
				SetCallback("VALUECHANGED_CB", iup.ValueChangedFunc(func(ih iup.Ihandle) int {
					iup.GetHandle("LabelBrightness").SetAttribute("TITLE", iup.GetHandle("Brightness").GetInt("VALUE"))
					iup.Refresh(iup.GetHandle("LabelBrightness"))
					previewLater()

					return iup.DEFAULT
				})),
		),
		iup.Vbox(
			iup.Hbox(
				iup.Label(i18n.Lng(i18n.LblContrast)),
				iup.Label("0").SetHandle("LabelContrast"),
			).SetAttributes("ALIGNMENT=ACENTER, NMARGIN=0"),
			iup.Val("").SetAttributes(`MIN=-100, MAX=100, VALUE=0, SHOWTICKS=10, EXPAND=HORIZONTAL`).SetHandle("Contrast").
				SetAttribute("TIP", i18n.Lng(i18n.TipContrast)).
				SetCallback("VALUECHANGED_CB", iup.ValueChangedFunc(func(ih iup.Ihandle) int {
					iup.GetHandle("LabelContrast").SetAttribute("TITLE", iup.GetHandle("Contrast").GetInt("VALUE"))
					iup.Refresh(iup.GetHandle("LabelContrast"))
					previewLater()

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
