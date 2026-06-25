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

	data, err := c.archiveFile(fileName, cover)
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

	img, err := doc.ImageDPI(0, c.renderDPI())
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

// pageArchive extracts the page-th image (natural reading order) from an archive.
func (c *Converter) pageArchive(fileName string, page int) (image.Image, error) {
	contents, err := c.archiveList(fileName)
	if err != nil {
		return nil, fmt.Errorf("pageArchive: %w", err)
	}

	images := imagesFromSlice(contents)
	sort.Sort(sortorder.Natural(images))

	if page < 0 || page >= len(images) {
		return nil, fmt.Errorf("pageArchive: page %d out of range (%d pages)", page+1, len(images))
	}

	data, err := c.archiveFile(fileName, images[page])
	if err != nil {
		return nil, fmt.Errorf("pageArchive: %w", err)
	}

	img, err := c.imageDecode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("pageArchive: %w", err)
	}

	return img, nil
}

// pageDocument extracts the page-th rendered page from a document.
func (c *Converter) pageDocument(fileName string, page int) (image.Image, error) {
	doc, err := fitz.New(fileName)
	if err != nil {
		return nil, fmt.Errorf("pageDocument: %w", err)
	}
	defer doc.Close()

	if page < 0 || page >= doc.NumPage() {
		return nil, fmt.Errorf("pageDocument: page %d out of range (%d pages)", page+1, doc.NumPage())
	}

	img, err := doc.ImageDPI(page, c.renderDPI())
	if err != nil {
		return nil, fmt.Errorf("pageDocument: %w", err)
	}

	return img, nil
}

// pageDirectory extracts the page-th image (natural reading order) from a directory.
func (c *Converter) pageDirectory(dir string, page int) (image.Image, error) {
	contents, err := imagesFromPath(dir)
	if err != nil {
		return nil, fmt.Errorf("pageDirectory: %w", err)
	}

	images := imagesFromSlice(contents)
	sort.Sort(sortorder.Natural(images))

	if page < 0 || page >= len(images) {
		return nil, fmt.Errorf("pageDirectory: page %d out of range (%d pages)", page+1, len(images))
	}

	file, err := os.Open(images[page])
	if err != nil {
		return nil, fmt.Errorf("pageDirectory: %w", err)
	}
	defer file.Close()

	img, err := c.imageDecode(file)
	if err != nil {
		return nil, fmt.Errorf("pageDirectory: %w", err)
	}

	return img, nil
}

// pageImage returns the page-th image of a comic file, document or directory.
func (c *Converter) pageImage(fileName string, fileInfo os.FileInfo, page int) (image.Image, error) {
	var err error
	var img image.Image

	switch {
	case fileInfo.IsDir():
		img, err = c.pageDirectory(fileName, page)
	case isDocument(fileName):
		img, err = c.pageDocument(fileName, page)
	case isArchive(fileName):
		img, err = c.pageArchive(fileName, page)
	}

	if err != nil {
		return nil, fmt.Errorf("pageImage: %w", err)
	}

	return img, nil
}

// PageCount returns the number of pages (images) in a comic file, document or directory.
func (c *Converter) PageCount(fileName string, fileInfo os.FileInfo) (int, error) {
	switch {
	case fileInfo.IsDir():
		contents, err := imagesFromPath(fileName)
		if err != nil {
			return 0, fmt.Errorf("PageCount: %w", err)
		}

		return len(imagesFromSlice(contents)), nil
	case isDocument(fileName):
		doc, err := fitz.New(fileName)
		if err != nil {
			return 0, fmt.Errorf("PageCount: %w", err)
		}
		defer doc.Close()

		return doc.NumPage(), nil
	case isArchive(fileName):
		contents, err := c.archiveList(fileName)
		if err != nil {
			return 0, fmt.Errorf("PageCount: %w", err)
		}

		return len(imagesFromSlice(contents)), nil
	}

	return 0, nil
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
