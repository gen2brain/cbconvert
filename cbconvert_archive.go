package cbconvert

import (
	"archive/tar"
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/gen2brain/go-unarr"
)

// archiveSave saves workdir to CBZ archive.
func (c *Converter) archiveSave(fileName string) error {
	if c.Opts.Archive == "zip" {
		return c.archiveSaveZip(fileName)
	} else if c.Opts.Archive == "tar" {
		return c.archiveSaveTar(fileName)
	}

	return nil
}

// archiveSaveZip saves workdir to CBZ archive.
func (c *Converter) archiveSaveZip(fileName string) error {
	if c.OnCompress != nil {
		c.OnCompress()
	}

	var zipName string
	if c.Opts.Recursive {
		fDir := strings.Split(filepath.Dir(fileName), string(os.PathSeparator))[1:]
		err := os.MkdirAll(filepath.Join(c.Opts.OutDir, filepath.Join(fDir...)), 0755)
		if err != nil {
			return fmt.Errorf("archiveSaveZip: %w", err)
		}

		zipName = filepath.Join(c.Opts.OutDir, filepath.Join(fDir...), fmt.Sprintf("%s%s.cbz", baseNoExt(fileName), c.Opts.Suffix))
	} else {
		zipName = filepath.Join(c.Opts.OutDir, fmt.Sprintf("%s%s.cbz", baseNoExt(fileName), c.Opts.Suffix))
	}

	zipFile, err := os.Create(zipName)
	if err != nil {
		return fmt.Errorf("archiveSaveZip: %w", err)
	}

	z := zip.NewWriter(zipFile)

	files, err := os.ReadDir(c.Workdir)
	if err != nil {
		return fmt.Errorf("archiveSaveZip: %w", err)
	}

	for _, file := range files {
		r, err := os.ReadFile(filepath.Join(c.Workdir, file.Name()))
		if err != nil {
			return fmt.Errorf("archiveSaveZip: %w", err)
		}

		info, err := file.Info()
		if err != nil {
			return fmt.Errorf("archiveSaveZip: %w", err)
		}

		zipInfo, err := zip.FileInfoHeader(info)
		if err != nil {
			return fmt.Errorf("archiveSaveZip: %w", err)
		}

		zipInfo.Method = zip.Deflate
		w, err := z.CreateHeader(zipInfo)
		if err != nil {
			return fmt.Errorf("archiveSaveZip: %w", err)
		}

		_, err = w.Write(r)
		if err != nil {
			return fmt.Errorf("archiveSaveZip: %w", err)
		}
	}

	if err = z.Close(); err != nil {
		return fmt.Errorf("archiveSaveZip: %w", err)
	}

	if err = zipFile.Close(); err != nil {
		return fmt.Errorf("archiveSaveZip: %w", err)
	}

	err = os.RemoveAll(c.Workdir)
	if err != nil {
		return fmt.Errorf("archiveSaveZip: %w", err)
	}

	return nil
}

// archiveSaveTar saves workdir to CBT archive.
func (c *Converter) archiveSaveTar(fileName string) error {
	if c.OnCompress != nil {
		c.OnCompress()
	}

	var tarName string
	if c.Opts.Recursive {
		fDir := strings.Split(filepath.Dir(fileName), string(os.PathSeparator))[1:]
		err := os.MkdirAll(filepath.Join(c.Opts.OutDir, filepath.Join(fDir...)), 0755)
		if err != nil {
			return fmt.Errorf("archiveSaveTar: %w", err)
		}

		tarName = filepath.Join(c.Opts.OutDir, filepath.Join(fDir...), fmt.Sprintf("%s%s.cbt", baseNoExt(fileName), c.Opts.Suffix))
	} else {
		tarName = filepath.Join(c.Opts.OutDir, fmt.Sprintf("%s%s.cbt", baseNoExt(fileName), c.Opts.Suffix))
	}

	tarFile, err := os.Create(tarName)
	if err != nil {
		return fmt.Errorf("archiveSaveTar: %w", err)
	}

	tw := tar.NewWriter(tarFile)

	files, err := os.ReadDir(c.Workdir)
	if err != nil {
		return fmt.Errorf("archiveSaveTar: %w", err)
	}

	for _, file := range files {
		r, err := os.ReadFile(filepath.Join(c.Workdir, file.Name()))
		if err != nil {
			return fmt.Errorf("archiveSaveTar: %w", err)
		}

		info, err := file.Info()
		if err != nil {
			return fmt.Errorf("archiveSaveTar: %w", err)
		}

		header, err := tar.FileInfoHeader(info, info.Name())
		if err != nil {
			return fmt.Errorf("archiveSaveTar: %w", err)
		}

		err = tw.WriteHeader(header)
		if err != nil {
			return fmt.Errorf("archiveSaveTar: %w", err)
		}

		_, err = tw.Write(r)
		if err != nil {
			return fmt.Errorf("archiveSaveTar: %w", err)
		}
	}

	if err = tw.Close(); err != nil {
		return fmt.Errorf("archiveSaveTar: %w", err)
	}

	if err = tarFile.Close(); err != nil {
		return fmt.Errorf("archiveSaveTar: %w", err)
	}

	err = os.RemoveAll(c.Workdir)
	if err != nil {
		return fmt.Errorf("archiveSaveTar: %w", err)
	}

	return nil
}

// archiveList lists contents of archive.
func (c *Converter) archiveList(fileName string) ([]string, error) {
	var contents []string

	archive, err := unarr.NewArchive(fileName)
	if err != nil {
		return contents, fmt.Errorf("archiveList: %w", err)
	}
	defer archive.Close()

	contents, err = archive.List()
	if err != nil {
		return contents, fmt.Errorf("archiveList: %w", err)
	}

	return contents, nil
}

// archiveComment returns ZIP comment.
func (c *Converter) archiveComment(fileName string) (string, error) {
	zr, err := zip.OpenReader(fileName)
	if err != nil {
		return "", fmt.Errorf("archiveComment: %w", err)
	}
	defer zr.Close()

	return zr.Comment, nil
}

// archiveSetComment sets ZIP comment.
func (c *Converter) archiveSetComment(fileName, commentBody string) error {
	zr, err := zip.OpenReader(fileName)
	if err != nil {
		return fmt.Errorf("archiveSetComment: %w", err)
	}
	defer zr.Close()

	zf, err := os.CreateTemp(os.TempDir(), "cbc")
	if err != nil {
		return fmt.Errorf("archiveSetComment: %w", err)
	}

	tmpName := zf.Name()
	defer os.Remove(tmpName)

	zw := zip.NewWriter(zf)
	err = zw.SetComment(commentBody)
	if err != nil {
		return fmt.Errorf("archiveSetComment: %w", err)
	}

	for _, item := range zr.File {
		ir, err := item.OpenRaw()
		if err != nil {
			return fmt.Errorf("archiveSetComment: %w", err)
		}

		item := item

		it, err := zw.CreateRaw(&item.FileHeader)
		if err != nil {
			return fmt.Errorf("archiveSetComment: %w", err)
		}

		_, err = io.Copy(it, ir)
		if err != nil {
			return fmt.Errorf("archiveSetComment: %w", err)
		}
	}

	err = zw.Close()
	if err != nil {
		return fmt.Errorf("archiveSetComment: %w", err)
	}

	err = zf.Close()
	if err != nil {
		return fmt.Errorf("archiveSetComment: %w", err)
	}

	data, err := os.ReadFile(tmpName)
	if err != nil {
		return fmt.Errorf("archiveSetComment: %w", err)
	}

	err = os.WriteFile(fileName, data, 0644)
	if err != nil {
		return fmt.Errorf("archiveSetComment: %w", err)
	}

	return nil
}

// archiveFileAdd adds file to archive.
func (c *Converter) archiveFileAdd(fileName, newFileName string) error {
	zr, err := zip.OpenReader(fileName)
	if err != nil {
		return fmt.Errorf("archiveFileAdd: %w", err)
	}
	defer zr.Close()

	zf, err := os.CreateTemp(os.TempDir(), "cbc")
	if err != nil {
		return fmt.Errorf("archiveFileAdd: %w", err)
	}

	tmpName := zf.Name()
	defer os.Remove(tmpName)

	zw := zip.NewWriter(zf)

	for _, item := range zr.File {
		if item.Name == newFileName {
			continue
		}

		ir, err := item.OpenRaw()
		if err != nil {
			return fmt.Errorf("archiveFileAdd: %w", err)
		}

		item := item

		it, err := zw.CreateRaw(&item.FileHeader)
		if err != nil {
			return fmt.Errorf("archiveFileAdd: %w", err)
		}

		_, err = io.Copy(it, ir)
		if err != nil {
			return fmt.Errorf("archiveFileAdd: %w", err)
		}
	}

	info, err := os.Stat(newFileName)
	if err != nil {
		return fmt.Errorf("archiveFileAdd: %w", err)
	}

	newData, err := os.ReadFile(newFileName)
	if err != nil {
		return fmt.Errorf("archiveFileAdd: %w", err)
	}

	zipInfo, err := zip.FileInfoHeader(info)
	if err != nil {
		return fmt.Errorf("archiveFileAdd: %w", err)
	}

	zipInfo.Method = zip.Deflate
	w, err := zw.CreateHeader(zipInfo)
	if err != nil {
		return fmt.Errorf("archiveFileAdd: %w", err)
	}

	_, err = w.Write(newData)
	if err != nil {
		return fmt.Errorf("archiveFileAdd: %w", err)
	}

	err = zw.Close()
	if err != nil {
		return fmt.Errorf("archiveFileAdd: %w", err)
	}

	err = zf.Close()
	if err != nil {
		return fmt.Errorf("archiveFileAdd: %w", err)
	}

	data, err := os.ReadFile(tmpName)
	if err != nil {
		return fmt.Errorf("archiveFileAdd: %w", err)
	}

	err = os.WriteFile(fileName, data, 0644)
	if err != nil {
		return fmt.Errorf("archiveFileAdd: %w", err)
	}

	return nil
}

// archiveFileRemove removes files from archive.
func (c *Converter) archiveFileRemove(fileName, pattern string) error {
	zr, err := zip.OpenReader(fileName)
	if err != nil {
		return fmt.Errorf("archiveFileRemove: %w", err)
	}
	defer zr.Close()

	zf, err := os.CreateTemp(os.TempDir(), "cbc")
	if err != nil {
		return fmt.Errorf("archiveFileRemove: %w", err)
	}

	tmpName := zf.Name()
	defer os.Remove(tmpName)

	zw := zip.NewWriter(zf)

	for _, item := range zr.File {
		matched, err := filepath.Match(pattern, item.Name)
		if err != nil {
			return fmt.Errorf("archiveFileRemove: %w", err)
		}

		if matched {
			continue
		}

		ir, err := item.OpenRaw()
		if err != nil {
			return fmt.Errorf("archiveFileRemove: %w", err)
		}

		item := item

		it, err := zw.CreateRaw(&item.FileHeader)
		if err != nil {
			return fmt.Errorf("archiveFileRemove: %w", err)
		}

		_, err = io.Copy(it, ir)
		if err != nil {
			return fmt.Errorf("archiveFileRemove: %w", err)
		}
	}

	err = zw.Close()
	if err != nil {
		return fmt.Errorf("archiveFileRemove: %w", err)
	}

	err = zf.Close()
	if err != nil {
		return fmt.Errorf("archiveFileRemove: %w", err)
	}

	data, err := os.ReadFile(tmpName)
	if err != nil {
		return fmt.Errorf("archiveFileRemove: %w", err)
	}

	err = os.WriteFile(fileName, data, 0644)
	if err != nil {
		return fmt.Errorf("archiveFileRemove: %w", err)
	}

	return nil
}
