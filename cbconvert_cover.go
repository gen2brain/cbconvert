package cbconvert

import (
	"bytes"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fvbommel/sortorder"
	"github.com/gen2brain/go-fitz"
	"github.com/gen2brain/go-unarr"
)

// coverArchive extracts cover from archive.
func (c *Converter) coverArchive(fileName string) (image.Image, error) {
	var images []string

	contents, err := c.archiveList(fileName)
	if err != nil {
		return nil, fmt.Errorf("coverArchive: %w", err)
	}

	for _, ct := range contents {
		if isImage(ct) {
			images = append(images, ct)
		}
	}

	cover := c.coverName(images)

	archive, err := unarr.NewArchive(fileName)
	if err != nil {
		return nil, fmt.Errorf("coverArchive: %w", err)
	}
	defer archive.Close()

	if err = archive.EntryFor(cover); err != nil {
		return nil, fmt.Errorf("coverArchive: %w", err)
	}

	data, err := archive.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("coverArchive: %w", err)
	}

	var img image.Image
	img, err = c.imageDecode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("coverArchive: %w", err)
	}

	return img, nil
}

// coverDocument extracts cover from document.
func (c *Converter) coverDocument(fileName string) (image.Image, error) {
	doc, err := fitz.New(fileName)
	if err != nil {
		return nil, fmt.Errorf("coverDocument: %w", err)
	}
	defer doc.Close()

	img, err := doc.Image(0)
	if err != nil {
		return nil, fmt.Errorf("coverDocument: %w", err)
	}

	return img, nil
}

// coverDirectory extracts cover from directory.
func (c *Converter) coverDirectory(dir string) (image.Image, error) {
	contents, err := imagesFromPath(dir)
	if err != nil {
		return nil, fmt.Errorf("coverDirectory: %w", err)
	}

	images := imagesFromSlice(contents)
	cover := c.coverName(images)

	file, err := os.Open(cover)
	if err != nil {
		return nil, fmt.Errorf("coverDirectory: %w", err)
	}
	defer file.Close()

	var img image.Image
	img, err = c.imageDecode(file)
	if err != nil {
		return nil, fmt.Errorf("coverDirectory: %w", err)
	}

	return img, nil
}

// coverName returns the filename that is the most likely to be the cover.
func (c *Converter) coverName(images []string) string {
	if len(images) == 0 {
		return ""
	}

	lower := make([]string, 0)
	for idx, img := range images {
		img = strings.ToLower(img)
		lower = append(lower, img)
		ext := baseNoExt(img)

		if strings.HasPrefix(img, "cover") || strings.HasPrefix(img, "front") ||
			strings.HasSuffix(ext, "cover") || strings.HasSuffix(ext, "front") {
			return filepath.ToSlash(images[idx])
		}
	}

	sort.Sort(sortorder.Natural(lower))
	cover := lower[0]

	for idx, img := range images {
		img = strings.ToLower(img)
		if img == cover {
			return filepath.ToSlash(images[idx])
		}
	}

	return ""
}

// coverImage returns cover as image.Image.
func (c *Converter) coverImage(fileName string, fileInfo os.FileInfo) (image.Image, error) {
	var err error
	var cover image.Image

	switch {
	case fileInfo.IsDir():
		cover, err = c.coverDirectory(fileName)
	case isDocument(fileName):
		cover, err = c.coverDocument(fileName)
	case isArchive(fileName):
		cover, err = c.coverArchive(fileName)
	}

	if c.OnProgress != nil {
		c.OnProgress()
	}

	if err != nil {
		return nil, fmt.Errorf("coverImage: %w", err)
	}

	return cover, nil
}
