package main

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"image/gif"
	"image/png"
	"net/url"
	"os"
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
	iup.Open()
	defer iup.Close()

	iup.SetGlobal("UTF8MODE", "YES")
	iup.SetGlobal("UTF8MODE_FILE", "YES")

	img, _ := png.Decode(bytes.NewReader(appLogo))
	iup.ImageFromImage(img).SetHandle("logo")

	dlg := iup.Dialog(layout()).SetAttributes(fmt.Sprintf(`TITLE="CBconvert %s", ICON=logo`, appVersion)).SetHandle("dlg")

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

	iup.Map(dlg)
	setActive()

	iup.ShowXY(dlg, iup.CENTER, iup.CENTER)
	iup.MainLoop()
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
	opts.Format = strings.ToLower(iup.GetHandle("Format").GetAttribute("VALUESTRING"))
	opts.Width = iup.GetHandle("Width").GetInt("VALUE")
	opts.Height = iup.GetHandle("Height").GetInt("VALUE")
	opts.Fit = iup.GetHandle("Fit").GetAttribute("VALUE") == "ON"
	opts.Filter = iup.GetHandle("Filter").GetInt("VALUE") - 1
	opts.Quality = iup.GetHandle("Quality").GetInt("VALUE")
	opts.Grayscale = iup.GetHandle("Grayscale").GetAttribute("VALUE") == "ON"
	opts.Brightness = iup.GetHandle("Brightness").GetInt("VALUE")
	opts.Contrast = iup.GetHandle("Contrast").GetInt("VALUE")
	opts.Rotate = iup.GetHandle("Rotate").GetInt("VALUESTRING")

	return opts
}

func setActive() {
	opts := options()
	count := iup.GetHandle("List").GetInt("COUNT")

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

	if opts.OutDir == "" {
		iup.GetHandle("Thumbnail").SetAttributes(`ACTIVE=NO`)
		iup.GetHandle("Cover").SetAttributes(`ACTIVE=NO`)
		iup.GetHandle("Convert").SetAttributes(`ACTIVE=NO`)
		if count > 0 {
			iup.GetHandle("Thumbnail").SetAttributes(`ACTIVE=NO, TIP="Set Output Directory"`)
			iup.GetHandle("Cover").SetAttributes(`ACTIVE=NO, TIP="Set Output Directory"`)
			iup.GetHandle("Convert").SetAttributes(`ACTIVE=NO, TIP="Set Output Directory"`)
		}
	} else {
		if count > 0 {
			iup.GetHandle("Thumbnail").SetAttributes(`ACTIVE=YES, TIP=""`)
			iup.GetHandle("Cover").SetAttributes(`ACTIVE=YES, TIP=""`)
			iup.GetHandle("Convert").SetAttributes(`ACTIVE=YES, TIP=""`)
		} else {
			iup.GetHandle("Thumbnail").SetAttributes(`ACTIVE=NO`)
			iup.GetHandle("Cover").SetAttributes(`ACTIVE=NO`)
			iup.GetHandle("Convert").SetAttributes(`ACTIVE=NO`)
		}
	}

	if opts.NoConvert {
		iup.GetHandle("VboxImage").SetAttribute("ACTIVE", "NO")
		iup.GetHandle("VboxTransform").SetAttribute("ACTIVE", "NO")
	} else {
		iup.GetHandle("VboxImage").SetAttribute("ACTIVE", "YES")
		iup.GetHandle("VboxTransform").SetAttribute("ACTIVE", "YES")
	}

	if (opts.Format == "jpeg" || opts.Format == "webp" || opts.Format == "avif" || opts.Format == "jxl") && !opts.NoConvert {
		iup.GetHandle("VboxQuality").SetAttribute("ACTIVE", "YES")
	} else {
		iup.GetHandle("VboxQuality").SetAttribute("ACTIVE", "NO")
	}

	if opts.Width != 0 && opts.Height != 0 && !opts.NoConvert {
		iup.GetHandle("Fit").SetAttribute("ACTIVE", "YES")
	} else {
		iup.GetHandle("Fit").SetAttribute("ACTIVE", "NO")
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

func list() iup.Ihandle {
	return iup.Vbox(
		iup.List().SetAttributes("EXPAND=YES, VISIBLECOLUMNS=16, VISIBLELINES=5").SetHandle("List").
			SetCallback("ACTION", iup.ListActionFunc(func(ih iup.Ihandle, text string, item int, state int) int {
				if state == 1 {
					index = item - 1
					setActive()
					previewPost()
				}

				return iup.DEFAULT
			})).
			SetCallback("DROPFILES_CB", iup.DropFilesFunc(func(ih iup.Ihandle, fileName string, num, x, y int) int {
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

				for _, file := range fs {
					iup.SetAttribute(iup.GetHandle("List"), "APPENDITEM", fmt.Sprintf("%s (%s)", file.Name, file.SizeHuman))
					files = append(files, file)
				}

				setActive()

				return iup.DEFAULT
			})),
	)
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

		file := files[index]
		img, err := conv.Preview(file.Path, file.Stat, width, height)
		if err != nil {
			fmt.Println(err)
		}

		iup.PostMessage(iup.GetHandle("Preview"), "", 0, img)
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

					if img.Image != nil {
						iup.Destroy(iup.GetHandle("cover"))
						iup.ImageFromImage(img.Image).SetHandle("cover")

						ih.SetAttribute("IMAGE", "cover")
						iup.GetHandle("PreviewInfo").SetAttribute("TITLE", fmt.Sprintf("%s (%dx%d)", img.SizeHuman, img.Width, img.Height))
					} else {
						ih.SetAttribute("IMAGE", "logo")
						iup.GetHandle("PreviewInfo").SetAttribute("TITLE", "")
					}

					iup.GetHandle("Loading").SetAttributes("VISIBLE=NO, STOP=YES")

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
	).SetHandle("VboxInput").SetAttributes("MARGIN=5x5, GAP=5")

	vboxOutput := iup.Vbox(
		iup.Vbox(
			iup.Label("Output Directory:"),
			iup.Text().SetAttributes("VISIBLECOLUMNS=16, MINSIZE=100x").SetHandle("OutDir").
				SetCallback("VALUECHANGED_CB", iup.ValueChangedFunc(func(ih iup.Ihandle) int {
					setActive()

					return iup.DEFAULT
				})),
			iup.Button("Browse...").SetAttributes("PADDING=DEFAULTBUTTONPADDING").
				SetCallback("ACTION", iup.ActionFunc(onOutputDirectory)),
		),
		iup.Vbox(
			iup.Label("Add Suffix to Output File:"),
			iup.Text().SetAttributes("VISIBLECOLUMNS=16, MINSIZE=100x").SetHandle("Suffix").
				SetAttribute("TIP", "Add suffix to filename, i.e. filename_suffix.cbz"),
		),
		iup.Vbox(
			iup.Toggle(" Remove Non-Image Files from the Archive").SetHandle("NoNonImage").
				SetAttribute("TIP", "Remove .nfo, .xml, .txt files from the archive"),
		),
		iup.Vbox(
			iup.Label("Archive Format:"),
			iup.List().SetAttributes(map[string]string{
				"DROPDOWN": "YES",
				"VALUE":    "1",
				"1":        "ZIP",
				"2":        "TAR",
			}).SetHandle("Archive"),
		),
	).SetHandle("VboxOutput").SetAttributes("MARGIN=5x5, GAP=5")

	vboxImage := iup.Vbox(
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
					setActive()
					previewPost()

					return iup.DEFAULT
				})),
		),
		iup.Vbox(
			iup.Label("Size:"),
			iup.Hbox(
				iup.Text().SetAttributes(`CUEBANNER=" width", VISIBLECOLUMNS=4, MASK="/d*"`).SetHandle("Width").
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
				iup.Label("x"),
				iup.Text().SetAttributes(`CUEBANNER=" height", VISIBLECOLUMNS=4, MASK="/d*"`).SetHandle("Height").
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
			).SetAttributes("ALIGNMENT=ACENTER, MARGIN=0"),
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
		iup.Vbox(
			iup.Hbox(
				iup.Label("Quality: "),
				iup.Label("75").SetHandle("LabelQuality"),
			).SetAttributes("MARGIN=0"),
			iup.Val("").SetAttributes(`MIN=0, MAX=100, VALUE=75, SHOWTICKS=10`).SetHandle("Quality").
				SetAttribute("TIP", "Quality affects JPEG, WEBP, AVIF and JXL").
				SetCallback("VALUECHANGED_CB", iup.ValueChangedFunc(func(ih iup.Ihandle) int {
					iup.GetHandle("LabelQuality").SetAttribute("TITLE", ih.GetInt("VALUE"))
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
			iup.Toggle(" Grayscale").SetHandle("Grayscale").
				SetAttributes(`TIP="Convert images to grayscale (monochromatic)"`).
				SetCallback("VALUECHANGED_CB", iup.ValueChangedFunc(func(ih iup.Ihandle) int {
					previewPost()

					return iup.DEFAULT
				})),
		),
	).SetHandle("VboxImage").SetAttributes("MARGIN=5x5, GAP=5")

	vboxTransform := iup.Vbox(
		iup.Vbox(
			iup.Hbox(
				iup.Label("Brightness: "),
				iup.Label("0").SetHandle("LabelBrightness"),
			).SetAttributes("ALIGNMENT=ACENTER, MARGIN=0"),
			iup.Val("").SetAttributes(`MIN=-100, MAX=100, VALUE=0, SHOWTICKS=10`).SetHandle("Brightness").
				SetAttributes(`TIP="Adjust the brightness of the images"`).
				SetCallback("VALUECHANGED_CB", iup.ValueChangedFunc(func(ih iup.Ihandle) int {
					iup.GetHandle("LabelBrightness").SetAttribute("TITLE", iup.GetHandle("Brightness").GetInt("VALUE"))
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
			).SetAttributes("ALIGNMENT=ACENTER, MARGIN=0"),
			iup.Val("").SetAttributes(`MIN=-100, MAX=100, VALUE=0, SHOWTICKS=10`).SetHandle("Contrast").
				SetAttributes(`TIP="Adjust the contrast of the images"`).
				SetCallback("VALUECHANGED_CB", iup.ValueChangedFunc(func(ih iup.Ihandle) int {
					iup.GetHandle("LabelContrast").SetAttribute("TITLE", iup.GetHandle("Contrast").GetInt("VALUE"))
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
	).SetHandle("VboxTransform").SetAttributes("MARGIN=5x5, GAP=5")

	return iup.Tabs(
		vboxInput.SetAttributes("TABTITLE=Input"),
		vboxOutput.SetAttributes("TABTITLE=Output"),
		vboxImage.SetAttributes("TABTITLE=Image"),
		vboxTransform.SetAttributes("TABTITLE=Transform"),
	).SetHandle("Tabs").SetAttributes("MINSIZE=320x400, EXPAND=HORIZONTAL")
}

func buttons() iup.Ihandle {
	return iup.Vbox(
		iup.Frame(
			iup.Vbox(
				iup.Button("Add &Files...").SetHandle("AddFiles").SetAttributes("EXPAND=HORIZONTAL, PADDING=DEFAULTBUTTONPADDING").
					SetCallback("ACTION", iup.ActionFunc(onAddFiles)),
				iup.Button("Add &Dir...").SetHandle("AddDir").SetAttributes("EXPAND=HORIZONTAL, PADDING=DEFAULTBUTTONPADDING").
					SetCallback("ACTION", iup.ActionFunc(onAddDir)),
				iup.Button("Remove").SetHandle("Remove").SetAttributes("EXPAND=HORIZONTAL, PADDING=DEFAULTBUTTONPADDING").
					SetCallback("ACTION", iup.ActionFunc(onRemove)),
				iup.Button("Remove All").SetHandle("RemoveAll").SetAttributes("EXPAND=HORIZONTAL, PADDING=DEFAULTBUTTONPADDING").
					SetCallback("ACTION", iup.ActionFunc(onRemoveAll)),
			).SetAttributes("NGAP=5"),
		),
		iup.Frame(
			iup.Vbox(
				iup.Button("Thumbnail").SetHandle("Thumbnail").SetAttributes("EXPAND=HORIZONTAL, PADDING=DEFAULTBUTTONPADDING").
					SetCallback("ACTION", iup.ActionFunc(onThumbnail)),
				iup.Button("Cover").SetHandle("Cover").SetAttributes("EXPAND=HORIZONTAL, PADDING=DEFAULTBUTTONPADDING").
					SetCallback("ACTION", iup.ActionFunc(onCover)),
			).SetAttributes("NGAP=5"),
		),
		iup.Frame(
			iup.Vbox(
				iup.Button("&Convert").SetHandle("Convert").SetAttributes("EXPAND=HORIZONTAL, PADDING=DEFAULTBUTTONPADDING").
					SetCallback("ACTION", iup.ActionFunc(onConvert)),
			),
		),
	).SetHandle("Buttons").SetAttributes("ALIGNMENT=ACENTER, NGAP=10")
}

func status() iup.Ihandle {
	return iup.Hbox(
		loading(),
		iup.Fill(),
		iup.Label("File 1 of 1").SetHandle("LabelStatus1").SetAttributes("VISIBLE=NO"),
		iup.Space().SetAttribute("SIZE", "5x0"),
		iup.Label("(000/000)").SetHandle("LabelStatus2").SetAttributes("VISIBLE=NO"),
		iup.Space().SetAttribute("SIZE", "5x0"),
		iup.ProgressBar().SetAttributes("RASTERSIZE=200x15, VISIBLE=NO").SetHandle("ProgressBar").
			SetCallback("POSTMESSAGE_CB", iup.PostMessageFunc(func(ih iup.Ihandle, s string, i int, p any) int {
				switch s {
				case "convert":
					conv := p.(*cbconvert.Converter)
					ih.SetAttributes("VALUE=0, VISIBLE=YES")
					ih.SetAttribute("MAX", conv.Ncontents)

					iup.GetHandle("List").SetAttributes("ACTIVE=NO")
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

					iup.GetHandle("List").SetAttributes("ACTIVE=NO")
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
					iup.GetHandle("List").SetAttributes("ACTIVE=YES")
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
	args, err := fileDlg("Add Files", true, false)
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

		for _, file := range fs {
			iup.SetAttribute(iup.GetHandle("List"), "APPENDITEM", fmt.Sprintf("%s (%s)", file.Name, file.SizeHuman))
			files = append(files, file)
		}

		setActive()
	}

	return iup.DEFAULT
}

func onAddDir(ih iup.Ihandle) int {
	args, err := fileDlg("Add Directory", false, true)
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

		for _, file := range fs {
			iup.SetAttribute(iup.GetHandle("List"), "APPENDITEM", fmt.Sprintf("%s (%s)", file.Name, file.SizeHuman))
			files = append(files, file)
		}

		setActive()
	}

	return iup.DEFAULT
}

func onRemove(ih iup.Ihandle) int {
	if index == -1 || len(files) == 0 {
		return iup.IGNORE
	}

	if len(files) == 1 {
		files = make([]cbconvert.File, 0)
	} else {
		files = slices.Delete(files, index, index)
	}

	iup.GetHandle("List").SetAttribute("REMOVEITEM", iup.GetHandle("List").GetAttribute("VALUE"))
	setActive()

	return iup.DEFAULT
}

func onRemoveAll(ih iup.Ihandle) int {
	index = -1
	files = make([]cbconvert.File, 0)

	iup.GetHandle("List").SetAttribute("REMOVEITEM", "ALL")
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

			if err := c.Thumbnail(file.Path, file.Stat); err != nil {
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

			if err := c.Cover(file.Path, file.Stat); err != nil {
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

	go func(c *cbconvert.Converter) {
		for _, file := range files {
			if err := c.Convert(file.Path, file.Stat); err != nil {
				if errors.Is(err, context.Canceled) {
					if err := os.RemoveAll(c.Workdir); err != nil {
						fmt.Println(err)
					}

					break
				}

				iup.PostMessage(iup.GetHandle("dlg"), err.Error(), 0, 0)
				fmt.Println(err)

				if err := os.RemoveAll(c.Workdir); err != nil {
					fmt.Println(err)
				}

				continue
			}
		}

		iup.PostMessage(iup.GetHandle("ProgressBar"), "finish", 0, 0)
	}(conv)

	return iup.DEFAULT
}

func onOutputDirectory(ih iup.Ihandle) int {
	args, err := fileDlg("Output Directory", false, true)
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
