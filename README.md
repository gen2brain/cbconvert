CBconvert
=========

Introduction
------------

CBconvert is a [Comic Book](http://en.wikipedia.org/wiki/Comic_Book_Archive_file) convert tool.


Features
--------

 - reads rar, zip, 7z, gz, bz2, cbr, cbz, cb7, cbt, pdf and plain directory
 - always saves processed comic in cbz (zip) format
 - images can be converted to JPEG, PNG or 4-Bit BMP (16 colors) format
 - choose resize algorithm (NearestNeighbor, Bilinear, Bicubic, MitchellNetravali, Lanczos2/3)
 - export covers from comics
 - create thumbnails from covers by [freedesktop](http://www.freedesktop.org/wiki/) specification

Download
--------

 - [Windows static build](https://github.com/gen2brain/cbconvert/releases/download/0.1.0/cbconvert-0.1.0.zip)
 - [Linux 64bit build](https://github.com/gen2brain/cbconvert/releases/download/0.1.0/cbconvert-0.1.0.tar.gz)

Compile
-------

Install poppler, poppler-glib, cairo, libarchive and imagemagick dev packages:

    apt-get install libpoppler-glib-dev libcairo2-dev libarchive-dev libmagickcore-dev libmagickwand-dev

Install go package:

    go get github.com/gen2brain/cbconvert
    go install github.com/gen2brain/cbconvert && cbconvert

Dependencies
------------

	go get github.com/MStoykov/go-libarchive
	go get github.com/cheggaaa/go-poppler
	go get github.com/cheggaaa/pb
	go get github.com/gographics/imagick/imagick
	go get github.com/hotei/bmp
	go get github.com/nfnt/resize
	go get github.com/skarademir/naturalsort
	go get github.com/ungerik/go-cairo
    go get gopkg.in/alecthomas/kingpin.v2

Using
-----

    usage: cbconvert [<flags>] <args>...

    Comic Book convert tool.

    Flags:
          --help             Show context-sensitive help (also try --help-long and --help-man).
          --version          Show application version.
      -p, --png              encode images to PNG instead of JPEG
      -b, --bmp              encode images to 4-Bit BMP instead of JPEG
      -w, --width=0          image width
      -h, --height=0         image height
      -q, --quality=75       JPEG image quality
      -n, --norgb            do not convert images with RGB colorspace
      -i, --interpolation=1  0=NearestNeighbor, 1=Bilinear, 2=Bicubic, 3=MitchellNetravali, 4=Lanczos2, 5=Lanczos3
      -s, --suffix=SUFFIX    add suffix to file basename
      -c, --cover            extract cover
      -t, --thumbnail        extract cover thumbnail (freedesktop spec.)
      -o, --outdir="."       output directory
      -m, --size=0           process only files larger then size (in MB)
      -r, --recursive        process subdirectories recursively
      -Q, --quiet            hide console output

    Args:
      <args>  filename or directory


Examples
--------

    cbconvert --recursive --width 1200 --size 60 /media/comics/Thorgal/

Rescale images to 1200px for all supported files found in directory with size larger then 60MB.

    cbconvert --bmp --outdir ~/comics /media/comics/Garfield/Garfield_01.cbz

Convert all images in archive to 4bit BMP image and save result in ~/comics directory. [BMP](http://en.wikipedia.org/wiki/BMP_file_format) format is uncompressed, for black&white pages very good choice. Archive size can be smaller 2-3x and file will be readable by comic readers.

    cbconvert --interpolation=5 --outdir ~/.thumbnails/normal --thumbnail /media/comics/GrooTheWanderer/

Generate thumbnails by freedesktop specification in ~/.thumbnails/normal directory, Lanczos3 algorithm is used for resizing.
