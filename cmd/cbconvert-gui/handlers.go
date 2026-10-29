package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/fvbommel/sortorder"

	"github.com/gen2brain/cbconvert"
	"github.com/gen2brain/cbconvert/cmd/cbconvert-gui/i18n"
	"github.com/gen2brain/iup-go/iup"
)

// selectRow focuses and selects the given 0-based row in the table.
func selectRow(i int) {
	if i < 0 || i >= len(files) {
		return
	}

	index = i
	iup.GetHandle("Table").SetAttribute("FOCUSCELL", fmt.Sprintf("%d:1", i+1))
}

// onSort sorts the files by the clicked column, toggling the direction on a second click, and refills the table.
func onSort(ih iup.Ihandle, col int) int {
	if col == sortCol {
		sortDesc = !sortDesc
	} else {
		sortCol, sortDesc = col, false
	}

	focus, selected := tableState()
	sortFiles()
	fillTable(focus, selected)

	sign := "UP"
	if sortDesc {
		sign = "DOWN"
	}
	ih.SetAttribute(fmt.Sprintf("SORTSIGN%d", col), sign)

	return iup.IGNORE
}

// sortFiles orders the files by sortCol, the natural name order breaking ties.
func sortFiles() {
	slices.SortStableFunc(files, func(a, b cbconvert.File) int {
		var c int
		switch sortCol {
		case 2:
			c = strings.Compare(cbconvert.FileType(a.Path), cbconvert.FileType(b.Path))
		case 3:
			c = cmp.Compare(a.Stat.Size(), b.Stat.Size())
		default:
			c = naturalCompare(a.Name, b.Name)
		}
		if sortDesc {
			c = -c
		}
		if c == 0 {
			c = naturalCompare(a.Name, b.Name)
		}

		return c
	})
}

// naturalCompare compares two names in natural order.
func naturalCompare(a, b string) int {
	switch {
	case sortorder.NaturalLess(a, b):
		return -1
	case sortorder.NaturalLess(b, a):
		return 1
	}

	return 0
}

// tableState returns the path of the focused file and the paths of the selected files.
func tableState() (string, map[string]bool) {
	var focus string
	if index >= 0 && index < len(files) {
		focus = files[index].Path
	}

	selected := make(map[string]bool)
	for _, i := range selectedRows() {
		selected[files[i].Path] = true
	}

	return focus, selected
}

// selectedRows returns the 0-based rows selected in the table, in ascending order.
func selectedRows() []int {
	var rows []int
	for i, c := range iup.GetHandle("Table").GetAttribute("SELECTEDLINES") {
		if c == '+' && i < len(files) {
			rows = append(rows, i)
		}
	}

	return rows
}

// fillTable writes the files into the table and restores the focused and the selected files.
func fillTable(focus string, selected map[string]bool) {
	t := iup.GetHandle("Table")
	t.SetAttribute("NUMLIN", strconv.Itoa(len(files)))

	var lines strings.Builder
	for i, f := range files {
		lin := i + 1
		iup.SetAttributeId2(t, "", lin, 1, f.Name)
		iup.SetAttributeId2(t, "", lin, 2, cbconvert.FileType(f.Path))
		iup.SetAttributeId2(t, "", lin, 3, strconv.FormatFloat(float64(f.Stat.Size())/(1024*1024), 'f', 2, 64))

		if selected[f.Path] {
			lines.WriteByte('+')
		} else {
			lines.WriteByte('-')
		}
	}

	index = -1
	for i, f := range files {
		if f.Path == focus {
			selectRow(i)
			break
		}
	}

	if len(selected) > 0 {
		t.SetAttribute("SELECTEDLINES", lines.String())
	}
}

// addFiles appends files, sorts the list, and refills the table.
func addFiles(fs []cbconvert.File) {
	if len(fs) == 0 {
		return
	}

	wasEmpty := len(files) == 0
	focus, selected := tableState()

	files = append(files, fs...)
	sortFiles()

	if wasEmpty {
		fillTable(files[0].Path, nil)
		setActive()
		previewPost()

		return
	}

	fillTable(focus, selected)
	setActive()
}

func previewPost() {
	if index == -1 || len(files) == 0 {
		return
	}

	file := files[index]

	// On a new file, fetch the count first; the Page POSTMESSAGE handler clamps previewPage and renders.
	if file.Path != previewPath {
		previewPath = file.Path
		iup.GetHandle("Loading").SetAttributes("VISIBLE=YES, START=YES")
		go pageCountPost(file)

		return
	}

	previewRender()
}

// previewSize returns the box in screen pixels that the cover is fitted into on the preview canvas.
func previewSize() (int, int) {
	_, cw, ch := iup.GetHandle("Preview").GetIntInt("DRAWSIZE")
	margin := previewMargin(cw, ch)

	scale, err := strconv.ParseFloat(iup.GetGlobal("SCREENSCALE"), 64)
	if err != nil || scale <= 0 {
		scale = 1
	}

	w := int(math.Ceil(float64(cw-2*margin) * scale))
	h := int(math.Ceil(float64(ch-2*margin) * scale))

	return max(w, 1), max(h, 1)
}

// onPreviewResize renders the page again, after the resizing settles, when the cover is too small for the canvas.
func onPreviewResize(ih iup.Ihandle, width, height int) int {
	if !hasCover || coverFull {
		return iup.DEFAULT
	}

	iw, ihh, _ := iup.DrawGetImageInfo("cover")
	w, h := previewSize()
	if iw >= w || ihh >= h {
		return iup.DEFAULT
	}

	t := iup.GetHandle("PreviewTimer")
	t.SetAttribute("RUN", "NO")
	t.SetAttribute("RUN", "YES")

	return iup.DEFAULT
}

// previewRender renders the current file at previewPage off the UI thread.
func previewRender() {
	if index == -1 || len(files) == 0 {
		return
	}

	file := files[index]

	iup.GetHandle("Loading").SetAttributes("VISIBLE=YES, START=YES")

	opts := options()
	page := previewPage
	w, h := previewSize()

	go func(opts cbconvert.Options) {
		conv := cbconvert.New(opts)

		var s string

		img, err := conv.PreviewPage(file.Path, file.Stat, page, w, h)
		if err != nil {
			s = err.Error()
			fmt.Println(err)
		}

		iup.PostMessage(iup.GetHandle("Preview"), s, page, img)
	}(opts)
}

// pageCountPost computes the file's page count off the UI thread and posts it, tagged with the path, to the Page spin.
func pageCountPost(file cbconvert.File) {
	n, err := cbconvert.New(cbconvert.NewOptions()).PageCount(file.Path, file.Stat)
	if err != nil || n < 1 {
		n = 1
	}

	iup.PostMessage(iup.GetHandle("Page"), file.Path, n, nil)
}

// onPageChanged re-renders the preview for the spin's page; dedupes so spin and typing don't both fire.
func onPageChanged() int {
	page := iup.GetHandle("Page").GetInt("VALUE") - 1
	if page < 0 {
		page = 0
	}
	if page == previewPage {
		return iup.DEFAULT
	}

	previewPage = page
	previewRender()

	return iup.DEFAULT
}

func onAddFiles(ih iup.Ihandle) int {
	args, err := fileDlg(i18n.Lng(i18n.DlgAddFiles), true, false, inputDirKey)
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

		addFiles(fs)
	}

	return iup.DEFAULT
}

func onAddDir(ih iup.Ihandle) int {
	args, err := fileDlg(i18n.Lng(i18n.DlgAddDir), false, true, inputDirKey)
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

		addFiles(fs)
	}

	return iup.DEFAULT
}

// onRemove removes the selected files, or the focused file when none is selected.
func onRemove(ih iup.Ihandle) int {
	rows := selectedRows()
	if len(rows) == 0 && index >= 0 && index < len(files) {
		rows = []int{index}
	}
	if len(rows) == 0 {
		return iup.IGNORE
	}

	t := iup.GetHandle("Table")
	for _, i := range slices.Backward(rows) {
		t.SetAttribute("DELLIN", strconv.Itoa(i+1))
		files = slices.Delete(files, i, i+1)
	}

	index = -1
	if len(files) > 0 {
		selectRow(min(rows[0], len(files)-1))
	}

	setActive()
	if len(files) == 0 {
		clearPreview()
	} else {
		previewPost()
	}

	return iup.DEFAULT
}

// onOpenFolder opens the folder of the focused file in the file manager.
func onOpenFolder(ih iup.Ihandle) int {
	if index < 0 || index >= len(files) {
		return iup.DEFAULT
	}

	iup.Help(filepath.Dir(files[index].Path))

	return iup.DEFAULT
}

// clearPreview resets the preview state and repaints the canvas to its empty state.
func clearPreview() {
	iup.Destroy(iup.GetHandle("cover"))
	hasCover = false
	previewPath = ""
	previewPage = 0

	iup.GetHandle("PreviewInfo").SetAttribute("TITLE", "")
	iup.Update(iup.GetHandle("Preview"))
}

func onRemoveAll(ih iup.Ihandle) int {
	index = -1
	files = make([]cbconvert.File, 0)

	iup.GetHandle("Table").SetAttribute("NUMLIN", "0")
	clearPreview()
	setActive()

	return iup.DEFAULT
}

func onThumbnail(ih iup.Ihandle) int {
	conv := cbconvert.New(options())
	conv.Nfiles = len(files)
	activeConv = conv
	setBusy(true)

	conv.OnProgress = func() {
		iup.PostMessage(iup.GetHandle("ProgressBar"), "progress2", 0, conv)
	}

	var canceled atomic.Bool
	conv.OnCancel = func() {
		canceled.Store(true)
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
			if canceled.Load() {
				break
			}

			if err := c.Thumbnail(file); err != nil {
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
	activeConv = conv
	setBusy(true)

	conv.OnProgress = func() {
		iup.PostMessage(iup.GetHandle("ProgressBar"), "progress2", 0, conv)
	}

	var canceled atomic.Bool
	conv.OnCancel = func() {
		canceled.Store(true)
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
			if canceled.Load() {
				break
			}

			if err := c.Cover(file); err != nil {
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
	if busy {
		if activeConv != nil {
			activeConv.Cancel()
		}

		return iup.DEFAULT
	}

	conv := cbconvert.New(options())
	conv.Nfiles = len(files)
	activeConv = conv
	setBusy(true)

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

	convertErr := func(err error) {
		if errors.Is(err, context.Canceled) {
			if err := os.RemoveAll(conv.Workdir); err != nil {
				fmt.Println(err)
			}

			return
		}

		iup.PostMessage(iup.GetHandle("dlg"), err.Error(), 0, 0)
		fmt.Println(err)

		if err := os.RemoveAll(conv.Workdir); err != nil {
			fmt.Println(err)
		}
	}

	go func(c *cbconvert.Converter) {
		if c.Opts.Combine {
			if err := c.Combine(files); err != nil {
				convertErr(err)
			}
		} else {
			for _, file := range files {
				if err := c.Convert(file); err != nil {
					convertErr(err)
					if errors.Is(err, context.Canceled) {
						break
					}

					continue
				}
			}
		}

		iup.PostMessage(iup.GetHandle("ProgressBar"), "finish", 0, 0)
	}(conv)

	return iup.DEFAULT
}

func onOutputDirectory(ih iup.Ihandle) int {
	args, err := fileDlg(i18n.Lng(i18n.DlgOutputDir), false, true, outputDirKey)
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

func onOutputFile(ih iup.Ihandle) int {
	name := saveDlg(i18n.Lng(i18n.DlgOutputFile), outputDirKey)
	if name != "" {
		iup.GetHandle("OutFile").SetAttribute("VALUE", filepath.Base(name))
		iup.GetHandle("OutDir").SetAttribute("VALUE", filepath.Dir(name))
		setActive()
	}

	return iup.DEFAULT
}

func onFilterChanged(ih iup.Ihandle) int {
	switch ih.GetInt("VALUE") {
	case 1:
		ih.SetAttribute("TIP", i18n.Lng(i18n.FilterNearest))
	case 2:
		ih.SetAttribute("TIP", i18n.Lng(i18n.FilterBox))
	case 3:
		ih.SetAttribute("TIP", i18n.Lng(i18n.FilterLinear))
	case 4:
		ih.SetAttribute("TIP", i18n.Lng(i18n.FilterMitchell))
	case 5:
		ih.SetAttribute("TIP", i18n.Lng(i18n.FilterCatmull))
	case 6:
		ih.SetAttribute("TIP", i18n.Lng(i18n.FilterGaussian))
	case 7:
		ih.SetAttribute("TIP", i18n.Lng(i18n.FilterLanczos))
	}

	previewPost()

	return iup.DEFAULT
}
