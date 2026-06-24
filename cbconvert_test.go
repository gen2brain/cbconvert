package cbconvert

import (
	"fmt"
	"os"
	"path/filepath"
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
