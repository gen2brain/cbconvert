package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/gen2brain/cbconvert"
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

// onSort re-syncs the files slice to the table's displayed order after a sort, so rows keep mapping to the right file.
func onSort(ih iup.Ihandle, col int) int {
	n := len(files)
	if n < 2 {
		return iup.DEFAULT
	}

	rowKey := func(name, size string) string {
		return name + "\x00" + size
	}

	buckets := make(map[string][]int, n)
	for i, f := range files {
		size := strconv.FormatFloat(float64(f.Stat.Size())/(1024*1024), 'f', 2, 64)
		k := rowKey(f.Name, size)
		buckets[k] = append(buckets[k], i)
	}

	var selPath string
	if index >= 0 && index < len(files) {
		selPath = files[index].Path
	}

	reordered := make([]cbconvert.File, 0, n)
	for lin := 1; lin <= n; lin++ {
		k := rowKey(iup.GetAttributeId2(ih, "", lin, 1), iup.GetAttributeId2(ih, "", lin, 3))
		idxs := buckets[k]
		if len(idxs) == 0 {
			return iup.DEFAULT
		}
		reordered = append(reordered, files[idxs[0]])
		buckets[k] = idxs[1:]
	}

	files = reordered

	index = -1
	if selPath != "" {
		for i, f := range files {
			if f.Path == selPath {
				selectRow(i)
				break
			}
		}
	}

	return iup.DEFAULT
}

// appendFile adds a file as a new row to the table and the files slice.
func appendFile(file cbconvert.File) {
	files = append(files, file)

	t := iup.GetHandle("Table")
	lin := len(files)
	t.SetAttribute("NUMLIN", strconv.Itoa(lin))
	iup.SetAttributeId2(t, "", lin, 1, file.Name)
	iup.SetAttributeId2(t, "", lin, 2, cbconvert.FileType(file.Path))
	iup.SetAttributeId2(t, "", lin, 3, strconv.FormatFloat(float64(file.Stat.Size())/(1024*1024), 'f', 2, 64))
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

		var s string
		file := files[index]

		img, err := conv.Preview(file.Path, file.Stat, width, height)
		if err != nil {
			s = err.Error()
			fmt.Println(err)
		}

		iup.PostMessage(iup.GetHandle("Preview"), s, 0, img)
	}(opts)
}

func onAddFiles(ih iup.Ihandle) int {
	args, err := fileDlg("Add Files", true, false, inputDirKey)
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

		wasEmpty := len(files) == 0

		for _, file := range fs {
			appendFile(file)
		}

		if wasEmpty && len(files) > 0 {
			selectRow(0)
		}

		setActive()

		if wasEmpty {
			previewPost()
		}
	}

	return iup.DEFAULT
}

func onAddDir(ih iup.Ihandle) int {
	args, err := fileDlg("Add Directory", false, true, inputDirKey)
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

		wasEmpty := len(files) == 0

		for _, file := range fs {
			appendFile(file)
		}

		if wasEmpty && len(files) > 0 {
			selectRow(0)
		}

		setActive()

		if wasEmpty {
			previewPost()
		}
	}

	return iup.DEFAULT
}

func onRemove(ih iup.Ihandle) int {
	if index < 0 || index >= len(files) {
		return iup.IGNORE
	}

	iup.GetHandle("Table").SetAttribute("DELLIN", strconv.Itoa(index+1))
	files = slices.Delete(files, index, index+1)

	if index >= len(files) {
		index = len(files) - 1
	}

	setActive()
	previewPost()

	return iup.DEFAULT
}

func onRemoveAll(ih iup.Ihandle) int {
	index = -1
	files = make([]cbconvert.File, 0)

	iup.GetHandle("Table").SetAttribute("NUMLIN", "0")
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
	args, err := fileDlg("Output Directory", false, true, outputDirKey)
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
	name := saveDlg("Output File", outputDirKey)
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
