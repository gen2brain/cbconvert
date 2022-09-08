## CBconvert

### Introduction

CBconvert is a [Comic Book](http://en.wikipedia.org/wiki/Comic_Book_Archive_file) converter.

It can convert comics to different formats to fit your various devices.

### Features

* reads RAR, ZIP, 7Z, CBR, CBZ, CB7, CBT, PDF, EPUB, and plain directory
* always saves processed comics in CBZ (ZIP) archive format
* images can be converted to JPEG, PNG, TIFF, WEBP, AVIF, or 4-Bit BMP (16 colors) file format
* rotate, flip, adjust brightness/contrast, adjust levels (Photoshop-like) or grayscale images
* resize algorithms (NearestNeighbor, Box, Linear, MitchellNetravali, CatmullRom, Gaussian, Lanczos)
* export covers from comics
* create thumbnails from covers by [FreeDesktop](http://www.freedesktop.org/wiki/) specification

### Download

* [Windows x86_64](https://github.com/gen2brain/cbconvert/releases/download/0.8.0/cbconvert-0.8.0-windows-x86_64.zip)
* [Linux x86_64](https://github.com/gen2brain/cbconvert/releases/download/0.8.0/cbconvert-0.8.0-linux-x86_64.tar.gz)
* [macOS x86_64](https://github.com/gen2brain/cbconvert/releases/download/0.8.0/cbconvert-0.8.0-darwin-x86_64.tar.gz)
* [macOS aarch64](https://github.com/gen2brain/cbconvert/releases/download/0.8.0/cbconvert-0.8.0-darwin-aarch64.tar.gz)

### Using cbconvert in file managers to generate FreeDesktop thumbnails

Copy cbconvert cli binary to your PATH and create file ~/.local/share/thumbnailers/cbconvert.thumbnailer:

```
[Thumbnailer Entry]
TryExec=cbconvert
Exec=cbconvert thumbnail --quiet --width %s --outfile %o %i
MimeType=application/pdf;application/x-pdf;image/pdf;application/x-cbz;application/x-cbr;application/x-cb7;application/x-cbt;application/epub+zip;
```

This is what it looks like in the PCManFM file manager:

![thumbnails](https://bit.ly/3BaTvTV)


### Using command line app

```
    Usage: cbconvert <command> [<flags>] [file1 dir1 ... fileOrDirN]


    Commands:

      convert*
            Convert archive or document (default command)

        --width
            Image width (default "0")
        --height
            Image height (default "0")
        --fit
            Best fit for required width and height (default "false")
        --format
            Image format, valid values are jpeg, png, tiff, bmp, webp, avif (default "jpeg")
        --quality
            Image quality (default "75")
        --lossless
            Lossless compression (avif) (default "false")
        --filter
            0=NearestNeighbor, 1=Box, 2=Linear, 3=MitchellNetravali, 4=CatmullRom, 6=Gaussian, 7=Lanczos (default "2")
        --no-cover
            Do not convert the cover image (default "false")
        --no-rgb
            Do not convert images that have RGB colorspace (default "false")
        --no-nonimage
            Remove non-image files from the archive (default "false")
        --no-convert
    	    Do not transform or convert images (default "false")
        --grayscale
            Convert images to grayscale (monochromatic) (default "false")
        --rotate
            Rotate images, valid values are 0, 90, 180, 270 (default "0")
        --flip
            Flip images, valid values are none, horizontal, vertical (default "none")
        --brightness
            Adjust the brightness of the images, must be in the range (-100, 100) (default "0")
        --contrast
            Adjust the contrast of the images, must be in the range (-100, 100) (default "0")
        --suffix
            Add suffix to file basename (default "")
        --levels-inmin
            Shadow input value (default "0")
        --levels-gamma
            Midpoint/Gamma (default "1")
        --levels-inmax
            Highlight input value (default "255")
        --levels-outmin
            Shadow output value (default "0")
        --levels-outmax
            Highlight output value (default "255")
        --outdir
            Output directory (default ".")
        --size
            Process only files larger than size (in MB) (default "0")
        --recursive
            Process subdirectories recursively (default "false")
        --quiet
            Hide console output (default "false")

      cover
            Extract cover

        --width
            Image width (default "0")
        --height
            Image height (default "0")
        --fit
            Best fit for required width and height (default "false")
        --quality
            JPEG image quality (default "75")
        --filter
            0=NearestNeighbor, 1=Box, 2=Linear, 3=MitchellNetravali, 4=CatmullRom, 6=Gaussian, 7=Lanczos (default "2")
        --outdir
            Output directory (default ".")
        --size
            Process only files larger than size (in MB) (default "0")
        --recursive
            Process subdirectories recursively (default "false")
        --quiet
            Hide console output (default "false")

      thumbnail
            Extract cover thumbnail (freedesktop spec.)

        --width
            Image width (default "0")
        --height
            Image height (default "0")
        --fit
            Best fit for required width and height (default "false")
        --filter
            0=NearestNeighbor, 1=Box, 2=Linear, 3=MitchellNetravali, 4=CatmullRom, 6=Gaussian, 7=Lanczos (default "2")
        --outdir
            Output directory (default ".")
        --outfile
            Output file (default "")
        --size
            Process only files larger than size (in MB) (default "0")
        --recursive
            Process subdirectories recursively (default "false")
        --quiet
            Hide console output (default "false")
```

### Examples

Rescale images to 1200px for all supported files found in a directory with a size larger than 60MB:

`cbconvert --recursive --width 1200 --size 60 /media/comics/Thorgal/`

Convert all images in pdf to 4bit BMP images and save the result in ~/comics directory:

`cbconvert --bmp --outdir ~/comics /media/comics/Garfield/Garfield_01.pdf`

[BMP](http://en.wikipedia.org/wiki/BMP_file_format) format is a very good choice for black&white pages. Archive size can be smaller 2-3x and the file will be readable by comic readers.

Generate thumbnails by [freedesktop specification](http://specifications.freedesktop.org/thumbnail-spec/thumbnail-spec-latest.html) in ~/.cache/thumbnails/normal directory with width 512:

`cbconvert thumbnail --width 512 --outdir ~/.cache/thumbnails/normal /media/comics/GrooTheWanderer/`

Extract covers to ~/covers dir for all supported files found in the directory, Lanczos algorithm is used for resizing:

`cbconvert cover --outdir ~/covers --filter=7 /media/comics/GrooTheWanderer/`

### Compile

Install ImageMagick 7 libraries and headers and then install to GOBIN:

`go install github.com/gen2brain/cbconvert/cmd/cbconvert@latest`
