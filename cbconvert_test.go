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

			err = conv.Convert(file.Path, file.Stat)
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
		err = conv.Cover(file.Path, file.Stat)
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
		err = conv.Thumbnail(file.Path, file.Stat)
		if err != nil {
			t.Error(err)
		}
	}

	err = os.RemoveAll(tmpDir)
	if err != nil {
		t.Error(err)
	}
}
