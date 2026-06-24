package cbconvert

import (
	"archive/zip"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConvert(t *testing.T) {
	tmpDir, err := os.MkdirTemp(os.TempDir(), "cbc")
	if err != nil {
		t.Error(err)
	}

	opts := NewOptions()
	opts.OutDir = tmpDir

	conv := New(opts)

	files, err := conv.Files([]string{"testdata/test", "testdata"})
	if err != nil {
		t.Error(err)
	}

	for _, format := range []string{"jpeg", "png", "tiff", "bmp", "webp", "avif", "jxl"} {
		conv.Opts.Format = format

		for _, file := range files {
			conv.Opts.Suffix = fmt.Sprintf("_%s%s", format, filepath.Ext(file.Path))

			err = conv.Convert(file)
			if err != nil {
				t.Errorf("format %s: file %s: %v", format, file.Name, err)
			}
		}
	}

	err = os.RemoveAll(tmpDir)
	if err != nil {
		t.Error(err)
	}
}

func TestCover(t *testing.T) {
	tmpDir, err := os.MkdirTemp(os.TempDir(), "cbc")
	if err != nil {
		t.Error(err)
	}

	opts := NewOptions()
	opts.OutDir = tmpDir

	conv := New(opts)

	files, err := conv.Files([]string{"testdata/test.cbt"})
	if err != nil {
		t.Error(err)
	}

	for _, file := range files {
		err = conv.Cover(file)
		if err != nil {
			t.Error(err)
		}
	}

	err = os.RemoveAll(tmpDir)
	if err != nil {
		t.Error(err)
	}
}

func TestThumbnail(t *testing.T) {
	tmpDir, err := os.MkdirTemp(os.TempDir(), "cbc")
	if err != nil {
		t.Error(err)
	}

	opts := NewOptions()
	opts.OutDir = tmpDir

	conv := New(opts)

	files, err := conv.Files([]string{"testdata/test.pdf"})
	if err != nil {
		t.Error(err)
	}

	for _, file := range files {
		err = conv.Thumbnail(file)
		if err != nil {
			t.Error(err)
		}
	}

	err = os.RemoveAll(tmpDir)
	if err != nil {
		t.Error(err)
	}
}

func TestArgs(t *testing.T) {
	opts := NewOptions()
	if got := opts.Args(); len(got) != 0 {
		t.Errorf("defaults should emit no flags, got %v", got)
	}

	opts.Format = "webp"
	opts.Quality = 90
	opts.Effort = 4
	opts.Lossless = true
	opts.Width = 1200
	opts.DPI = 150
	opts.Grayscale = true
	opts.OutDir = "/out"

	got := strings.Join(opts.Args(), " ")
	want := "--width 1200 --dpi 150 --format webp --quality 90 --effort 4 --lossless --grayscale --outdir /out"
	if got != want {
		t.Errorf("Args() = %q, want %q", got, want)
	}
}

func TestConvertDPI(t *testing.T) {
	tmpDir, err := os.MkdirTemp(os.TempDir(), "cbc")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	dims := func(dpi int) int {
		opts := NewOptions()
		opts.OutDir = tmpDir
		opts.DPI = dpi

		conv := New(opts)

		files, err := conv.Files([]string{"testdata/test.pdf"})
		if err != nil {
			t.Fatal(err)
		}

		for _, file := range files {
			if err := conv.Convert(file); err != nil {
				t.Fatal(err)
			}
		}

		return firstPage(t, conv, filepath.Join(tmpDir, "test.cbz")).Bounds().Dx()
	}

	low := dims(150)
	high := dims(600)
	if low >= high {
		t.Errorf("higher DPI should render larger pages: 150dpi=%d, 600dpi=%d", low, high)
	}
}

func TestConvertResize(t *testing.T) {
	tmpDir, err := os.MkdirTemp(os.TempDir(), "cbc")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	opts := NewOptions()
	opts.OutDir = tmpDir
	opts.Width = 100

	conv := New(opts)

	files, err := conv.Files([]string{"testdata/test.cbz"})
	if err != nil {
		t.Fatal(err)
	}

	for _, file := range files {
		if err := conv.Convert(file); err != nil {
			t.Fatal(err)
		}
	}

	img := firstPage(t, conv, filepath.Join(tmpDir, "test.cbz"))
	if got := img.Bounds().Dx(); got != 100 {
		t.Errorf("resized width: got %d, want 100", got)
	}
}

func TestConvertFit(t *testing.T) {
	tmpDir, err := os.MkdirTemp(os.TempDir(), "cbc")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	opts := NewOptions()
	opts.OutDir = tmpDir
	opts.Width = 120
	opts.Height = 120
	opts.Fit = true

	conv := New(opts)

	files, err := conv.Files([]string{"testdata/test.cbz"})
	if err != nil {
		t.Fatal(err)
	}

	for _, file := range files {
		if err := conv.Convert(file); err != nil {
			t.Fatal(err)
		}
	}

	img := firstPage(t, conv, filepath.Join(tmpDir, "test.cbz"))
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	if w > 120 || h > 120 {
		t.Errorf("fit exceeded bounds: got %dx%d, want <= 120x120", w, h)
	}
	if w != 120 && h != 120 {
		t.Errorf("fit did not touch a bound: got %dx%d, want one side == 120", w, h)
	}
}

func TestConvertTar(t *testing.T) {
	tmpDir, err := os.MkdirTemp(os.TempDir(), "cbc")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	opts := NewOptions()
	opts.OutDir = tmpDir
	opts.Archive = "tar"

	conv := New(opts)

	files, err := conv.Files([]string{"testdata/test.cbz"})
	if err != nil {
		t.Fatal(err)
	}

	for _, file := range files {
		if err := conv.Convert(file); err != nil {
			t.Fatal(err)
		}
	}

	out := filepath.Join(tmpDir, "test.cbt")
	list, err := conv.archiveList(out)
	if err != nil {
		t.Fatalf("read tar output: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 pages in tar output, got %d: %v", len(list), list)
	}
}

func TestZipLevel(t *testing.T) {
	convertWith := func(level int) *zip.ReadCloser {
		tmpDir, err := os.MkdirTemp(os.TempDir(), "cbc")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.RemoveAll(tmpDir) })

		opts := NewOptions()
		opts.OutDir = tmpDir
		opts.ZipLevel = level
		opts.NoConvert = true

		conv := New(opts)

		files, err := conv.Files([]string{"testdata/test.cbz"})
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range files {
			if err := conv.Convert(file); err != nil {
				t.Fatal(err)
			}
		}

		zr, err := zip.OpenReader(filepath.Join(tmpDir, "test.cbz"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { zr.Close() })

		return zr
	}

	store := convertWith(0)
	for _, f := range store.File {
		if f.Method != zip.Store {
			t.Errorf("level 0: %s stored with method %d, want Store", f.Name, f.Method)
		}
		if f.CompressedSize64 != f.UncompressedSize64 {
			t.Errorf("level 0: %s is compressed (%d < %d)", f.Name, f.CompressedSize64, f.UncompressedSize64)
		}
	}

	deflate := convertWith(9)
	for _, f := range deflate.File {
		if f.Method != zip.Deflate {
			t.Errorf("level 9: %s method %d, want Deflate", f.Name, f.Method)
		}
	}
}

func TestImageTransforms(t *testing.T) {
	conv := New(NewOptions())

	f, err := os.Open("testdata/test/00.jpg")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	src, err := conv.imageDecode(f)
	if err != nil {
		t.Fatal(err)
	}
	srcW, srcH := src.Bounds().Dx(), src.Bounds().Dy()

	conv.Opts.Rotate = 90
	rotated := conv.imageTransform(src)
	if rotated.Bounds().Dx() != srcH || rotated.Bounds().Dy() != srcW {
		t.Errorf("rotate 90: got %dx%d, want %dx%d", rotated.Bounds().Dx(), rotated.Bounds().Dy(), srcH, srcW)
	}

	conv.Opts = NewOptions()
	conv.Opts.Grayscale = true
	gray := conv.imageTransform(src)
	if !isGrayScale(gray) {
		t.Errorf("grayscale: result is not grayscale")
	}

	conv.Opts = NewOptions()
	conv.Opts.Brightness = 20
	conv.Opts.Contrast = 20
	adjusted := conv.imageTransform(src)
	if adjusted.Bounds().Dx() != srcW || adjusted.Bounds().Dy() != srcH {
		t.Errorf("brightness/contrast changed dimensions: got %dx%d, want %dx%d",
			adjusted.Bounds().Dx(), adjusted.Bounds().Dy(), srcW, srcH)
	}
}

func TestCoverName(t *testing.T) {
	conv := New(NewOptions())

	tests := []struct {
		name   string
		images []string
		want   string
	}{
		{"empty", nil, ""},
		{"natural sort", []string{"10.jpg", "2.jpg", "1.jpg"}, "1.jpg"},
		{"cover prefix wins", []string{"01.jpg", "cover.jpg", "02.jpg"}, "cover.jpg"},
		{"front prefix wins", []string{"01.jpg", "front.png", "00.jpg"}, "front.png"},
		{"cover suffix wins", []string{"01.jpg", "page_cover.jpg"}, "page_cover.jpg"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := conv.coverName(tt.images); got != tt.want {
				t.Errorf("coverName(%v) = %q, want %q", tt.images, got, tt.want)
			}
		})
	}
}

func TestCoverDirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp(os.TempDir(), "cbc")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	opts := NewOptions()
	opts.OutDir = tmpDir

	conv := New(opts)

	files, err := conv.Files([]string{"testdata/test"})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 directory file, got %d", len(files))
	}

	for _, file := range files {
		if err := conv.Cover(file); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := os.Stat(filepath.Join(tmpDir, "test.jpg")); err != nil {
		t.Errorf("directory cover not written: %v", err)
	}
}

func TestFileType(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"testdata/test.cbz", "ZIP"},
		{"testdata/test.cbr", "RAR"},
		{"testdata/test.cb7", "7Z"},
		{"testdata/test.cbt", "TAR"},
		{"testdata/test.pdf", "PDF"},
		{"testdata/test", "DIR"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := FileType(tt.path); got != tt.want {
				t.Errorf("FileType(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestCombine(t *testing.T) {
	tmpDir, err := os.MkdirTemp(os.TempDir(), "cbc")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	opts := NewOptions()
	opts.OutDir = tmpDir
	opts.OutFile = "merged"

	conv := New(opts)

	files, err := conv.Files([]string{"testdata/test.cbz", "testdata/test.cbt"})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 input files, got %d", len(files))
	}

	if err := conv.Combine(files); err != nil {
		t.Fatal(err)
	}

	zr, err := zip.OpenReader(filepath.Join(tmpDir, "merged.cbz"))
	if err != nil {
		t.Fatalf("open combined archive: %v", err)
	}
	defer zr.Close()

	var names []string
	for _, f := range zr.File {
		names = append(names, f.Name)
	}

	if len(names) != 4 {
		t.Fatalf("expected 4 pages in combined archive, got %d: %v", len(names), names)
	}

	// each input is prefixed so identically named pages do not collide
	var first, second int
	for _, n := range names {
		switch {
		case strings.HasPrefix(n, "0001_"):
			first++
		case strings.HasPrefix(n, "0002_"):
			second++
		}
	}
	if first != 2 || second != 2 {
		t.Errorf("expected 2 pages from each input, got 0001_=%d 0002_=%d: %v", first, second, names)
	}
}

func TestSubfolders(t *testing.T) {
	page0, err := os.ReadFile("testdata/test/00.jpg")
	if err != nil {
		t.Fatal(err)
	}
	page1, err := os.ReadFile("testdata/test/01.jpg")
	if err != nil {
		t.Fatal(err)
	}

	inDir, err := os.MkdirTemp(os.TempDir(), "cbc-in")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(inDir)

	src := filepath.Join(inDir, "chapters.cbz")
	buildZip(t, src, []zipEntry{
		{"chapter1/00.jpg", page0},
		{"chapter1/01.jpg", page1},
		{"chapter2/00.jpg", page0},
		{"chapter2/01.jpg", page1},
	})

	tmpDir, err := os.MkdirTemp(os.TempDir(), "cbc-out")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	opts := NewOptions()
	opts.OutDir = tmpDir

	conv := New(opts)

	files, err := conv.Files([]string{src})
	if err != nil {
		t.Fatal(err)
	}

	for _, file := range files {
		if err := conv.Convert(file); err != nil {
			t.Fatal(err)
		}
	}

	zr, err := zip.OpenReader(filepath.Join(tmpDir, "chapters.cbz"))
	if err != nil {
		t.Fatalf("open output archive: %v", err)
	}
	defer zr.Close()

	// without subfolder preservation chapter2/00 overwrites chapter1/00 and only 2 pages survive
	if len(zr.File) != 4 {
		var names []string
		for _, f := range zr.File {
			names = append(names, f.Name)
		}
		t.Fatalf("expected 4 pages from numbered subfolders, got %d: %v", len(zr.File), names)
	}
}

func TestMeta(t *testing.T) {
	tmpDir, err := os.MkdirTemp(os.TempDir(), "cbc")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// operate on a copy so the fixture stays intact
	data, err := os.ReadFile("testdata/test.cbz")
	if err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(tmpDir, "meta.cbz")
	if err := os.WriteFile(archive, data, 0644); err != nil {
		t.Fatal(err)
	}

	conv := New(NewOptions())

	conv.Opts = NewOptions()
	conv.Opts.CommentBody = "hello world"
	if _, err := conv.Meta(archive); err != nil {
		t.Fatalf("set comment: %v", err)
	}

	conv.Opts = NewOptions()
	conv.Opts.Comment = true
	got, err := conv.Meta(archive)
	if err != nil {
		t.Fatalf("get comment: %v", err)
	}
	if got != "hello world" {
		t.Errorf("comment roundtrip: got %q, want %q", got, "hello world")
	}

	extra := filepath.Join(tmpDir, "ComicInfo.xml")
	if err := os.WriteFile(extra, []byte("<ComicInfo/>"), 0644); err != nil {
		t.Fatal(err)
	}

	conv.Opts = NewOptions()
	conv.Opts.FileAdd = extra
	if _, err := conv.Meta(archive); err != nil {
		t.Fatalf("add file: %v", err)
	}
	if !archiveHas(t, conv, archive, "ComicInfo.xml") {
		t.Errorf("added file not found in archive")
	}

	conv.Opts = NewOptions()
	conv.Opts.FileRemove = "ComicInfo.xml"
	if _, err := conv.Meta(archive); err != nil {
		t.Fatalf("remove file: %v", err)
	}
	if archiveHas(t, conv, archive, "ComicInfo.xml") {
		t.Errorf("removed file still present in archive")
	}
}

func TestRecursive(t *testing.T) {
	inDir, err := os.MkdirTemp(os.TempDir(), "cbc-in")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(inDir)

	sub := filepath.Join(inDir, "chapter1")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}

	src, err := os.ReadFile("testdata/test.cbz")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "test.cbz"), src, 0644); err != nil {
		t.Fatal(err)
	}

	outDir, err := os.MkdirTemp(os.TempDir(), "cbc-out")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(outDir)

	opts := NewOptions()
	opts.OutDir = outDir
	opts.Recursive = true

	conv := New(opts)

	files, err := conv.Files([]string{inDir})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	for _, file := range files {
		if err := conv.Convert(file); err != nil {
			t.Error(err)
		}
	}

	// output must mirror the input subtree relative to the input root, not the absolute path
	want := filepath.Join(outDir, "chapter1", "test.cbz")
	if _, err := os.Stat(want); err != nil {
		t.Errorf("expected output relative to input root at %s: %v", want, err)
	}
}

type zipEntry struct {
	name string
	data []byte
}

func buildZip(t *testing.T, path string, entries []zipEntry) {
	t.Helper()

	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	for _, e := range entries {
		w, err := zw.Create(e.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(e.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
}

func firstPage(t *testing.T, conv *Converter, archive string) image.Image {
	t.Helper()

	zr, err := zip.OpenReader(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()

	if len(zr.File) == 0 {
		t.Fatalf("archive %s has no entries", archive)
	}

	rc, err := zr.File[0].Open()
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()

	img, err := conv.imageDecode(rc)
	if err != nil {
		t.Fatal(err)
	}

	return img
}

func archiveHas(t *testing.T, conv *Converter, archive, name string) bool {
	t.Helper()

	list, err := conv.archiveList(archive)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range list {
		if n == name {
			return true
		}
	}

	return false
}
