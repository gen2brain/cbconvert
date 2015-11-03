CBconvert
=========

Introduction
------------

CBconvert is a [Comic Book](http://en.wikipedia.org/wiki/Comic_Book_Archive_file) convert tool.

Features
--------

 - reads RAR, ZIP, 7Z, GZ, BZ2, CBR, CBZ, CB7, CBT, PDF, EPUB, XPS and plain directory
 - always saves processed comic in CBZ (ZIP) archive format
 - images can be converted to JPEG, PNG, GIF or 4-Bit BMP (16 colors) file format
 - reads JPEG, PNG, BMP, GIF, TIFF and WEBP file formats
 - choose resize algorithm (NearestNeighbor, Bilinear, Bicubic, MitchellNetravali, Lanczos2/3)
 - export covers from comics
 - create thumbnails from covers by [freedesktop](http://www.freedesktop.org/wiki/) specification

Download
--------

 - [Windows binary](https://github.com/gen2brain/cbconvert/releases/download/0.3.0/cbconvert-0.3.0.zip)

 - [Linux 64bit binary](https://github.com/gen2brain/cbconvert/releases/download/0.3.0/cbconvert-0.3.0.tar.gz)
 - [Linux 64bit static binary](https://github.com/gen2brain/cbconvert/releases/download/0.3.0/cbconvert-0.3.0-static.tar.gz)

Using
-----

    usage: cbconvert [<flags>] <args>...

    Comic Book convert tool.

    Flags:
          --help             Show context-sensitive help (also try --help-long and --help-man).
          --version          Show application version.
      -p, --png              encode images to PNG instead of JPEG
      -b, --bmp              encode images to 4-Bit BMP instead of JPEG
      -g, --gif              encode images to GIF instead of JPEG
      -w, --width=0          image width
      -h, --height=0         image height
      -q, --quality=75       JPEG image quality
      -n, --norgb            do not convert images with RGB colorspace
      -r, --resize=1         0=NearestNeighbor, 1=Bilinear, 2=Bicubic, 3=MitchellNetravali, 4=Lanczos2, 5=Lanczos3
      -s, --suffix=SUFFIX    add suffix to file basename
      -c, --cover            extract cover
      -t, --thumbnail        extract cover thumbnail (freedesktop spec.)
      -o, --outdir="."       output directory
      -m, --size=0           process only files larger then size (in MB)
      -R, --recursive        process subdirectories recursively
      -Q, --quiet            hide console output

    Args:
      <args>  filename or directory


Examples
--------

Rescale images to 1200px for all supported files found in directory with size larger then 60MB:

    cbconvert --recursive --width 1200 --size 60 /media/comics/Thorgal/

Convert all images in archive to 4bit BMP image and save result in ~/comics directory:

    cbconvert --bmp --outdir ~/comics /media/comics/Garfield/Garfield_01.cbz

[BMP](http://en.wikipedia.org/wiki/BMP_file_format) format is very good choice for black&white pages. Archive size can be smaller 2-3x and file will be readable by comic readers.

Generate thumbnails by freedesktop specification in ~/.thumbnails/normal directory, Lanczos3 algorithm is used for resizing:

    cbconvert --resize=5 --outdir ~/.thumbnails/normal --thumbnail /media/comics/GrooTheWanderer/

Compile
-------

Install imagemagick dev packages:

    apt-get install libmagickcore-dev libmagickwand-dev

Compile latest MuPDF:

    git clone git://git.ghostscript.com/mupdf.git && cd mupdf
    git submodule update --init --recursive
    curl -L https://gist.githubusercontent.com/gen2brain/7869ac4c6db5933f670f/raw/1619394dc957ae10bcd73c713760993466b4bfea/mupdf-openssl-curl.patch | patch -p1
    sed -e "1iHAVE_X11 = no" -e "1iWANT_OPENSSL = no" -e "1iWANT_CURL = no" -i Makerules
    HAVE_X11=no HAVE_GLFW=no HAVE_GLUT=no WANT_OPENSSL=no WANT_CURL=no HAVE_MUJS=yes HAVE_JSCORE=no HAVE_V8=no make && make install

Compile unarr library:

    git clone https://github.com/zeniko/unarr && cd unarr
    mkdir lzma920 && cd lzma920 && curl -L http://www.7-zip.org/a/lzma920.tar.bz2 | tar -xjvp && cd ..
    curl -L http://zlib.net/zlib-1.2.8.tar.gz | tar -xzvp
    curl -L http://www.bzip.org/1.0.6/bzip2-1.0.6.tar.gz | tar -xzvp
    curl -L https://gist.githubusercontent.com/gen2brain/89fe506863be3fb139e8/raw/8783a7d81e22ad84944d146c5e33beab6dffc641/unarr-makefile.patch | patch -p1
    CFLAGS="-DHAVE_7Z -DHAVE_ZLIB -DHAVE_BZIP2 -I./lzma920/C -I./zlib-1.2.8 -I./bzip2-1.0.6" make
    cp build/debug/libunarr.a /usr/lib64/ && cp unarr.h /usr/include

Install dependencies:

    go get github.com/cheggaaa/pb
    go get github.com/gen2brain/go-fitz
    go get github.com/gen2brain/go-unarr
    go get github.com/gographics/imagick/imagick
    go get github.com/hotei/bmp
    go get github.com/nfnt/resize
    go get github.com/skarademir/naturalsort
    go get golang.org/x/image/tiff
    go get golang.org/x/image/webp    
    go get gopkg.in/alecthomas/kingpin.v2

Install go package:

    go get github.com/gen2brain/cbconvert
    go install github.com/gen2brain/cbconvert
