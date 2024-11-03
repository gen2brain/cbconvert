package cbconvert

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// imagesFromPath returns list of found image files for given directory.
func imagesFromPath(path string) ([]string, error) {
	var images []string

	walkFiles := func(fp string, f os.FileInfo, err error) error {
		if !f.IsDir() && f.Mode()&os.ModeType == 0 {
			if f.Size() > 0 && (isImage(fp)) {
				images = append(images, fp)
			}
		}

		return nil
	}

	f, err := filepath.Abs(path)
	if err != nil {
		return images, fmt.Errorf("imagesFromPath: %w", err)
	}

	stat, err := os.Stat(f)
	if err != nil {
		return images, fmt.Errorf("imagesFromPath: %w", err)
	}

	if !stat.IsDir() && stat.Mode()&os.ModeType == 0 {
		if isImage(f) {
			images = append(images, f)
		}
	} else {
		err = filepath.Walk(f, walkFiles)
		if err != nil {
			return images, fmt.Errorf("imagesFromPath: %w", err)
		}
	}

	return images, nil
}

// imagesFromSlice returns list of found image files for given slice of files.
func imagesFromSlice(files []string) []string {
	var images []string

	for _, f := range files {
		if isImage(f) {
			images = append(images, f)
		}
	}

	return images
}

// isArchive checks if file is archive.
func isArchive(f string) bool {
	var types = []string{".rar", ".zip", ".7z", ".tar", ".cbr", ".cbz", ".cb7", ".cbt"}
	for _, t := range types {
		if strings.ToLower(filepath.Ext(f)) == t {
			return true
		}
	}

	return false
}

// isDocument checks if file is document.
func isDocument(f string) bool {
	var types = []string{".pdf", ".xps", ".epub", ".mobi", ".docx", ".pptx", ".xlsx"}
	for _, t := range types {
		if strings.ToLower(filepath.Ext(f)) == t {
			return true
		}
	}

	return false
}

// isImage checks if file is image.
func isImage(f string) bool {
	var types = []string{".jpg", ".jpeg", ".png", ".gif", ".bmp", ".tiff", ".tif", ".webp", ".avif", ".jxl"}
	for _, t := range types {
		if strings.ToLower(filepath.Ext(f)) == t {
			return true
		}
	}

	return false
}

// isNonImage checks for allowed files in archive.
func isNonImage(f string) bool {
	var types = []string{".nfo", ".xml", ".txt"}
	for _, t := range types {
		if strings.ToLower(filepath.Ext(f)) == t {
			return true
		}
	}

	return false
}

// isSize checks size of file.
func isSize(a, b int64) bool {
	if a > 0 {
		if b < int64(a)*(1024*1024) {
			return false
		}
	}

	return true
}

// baseNoExt returns base name without extension.
func baseNoExt(filename string) string {
	return strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
}

// copyFile copies reader to file.
func copyFile(reader io.Reader, filename string) error {
	err := os.MkdirAll(filepath.Dir(filename), 0755)
	if err != nil {
		return fmt.Errorf("copyFile: %w", err)
	}

	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("copyFile: %w", err)
	}
	defer file.Close()

	_, err = io.Copy(file, reader)
	if err != nil {
		return fmt.Errorf("copyFile: %w", err)
	}

	return nil
}
