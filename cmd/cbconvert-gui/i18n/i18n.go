// Package i18n holds the cbconvert GUI message keys and per-language string packs.
package i18n

import "github.com/gen2brain/iup-go/iup"

const (
	ColTitle = "COL_TITLE"
	ColType  = "COL_TYPE"
	ColSize  = "COL_SIZE"

	TabInput     = "TAB_INPUT"
	TabOutput    = "TAB_OUTPUT"
	TabImage     = "TAB_IMAGE"
	TabTransform = "TAB_TRANSFORM"

	LblPage = "LBL_PAGE"
	TipPage = "TIP_PAGE"

	TglRecursive  = "TGL_RECURSIVE"
	TipRecursive  = "TIP_RECURSIVE"
	TglNoRGB      = "TGL_NORGB"
	TipNoRGB      = "TIP_NORGB"
	TglNoCover    = "TGL_NOCOVER"
	TipNoCover    = "TIP_NOCOVER"
	TglNoNonImage = "TGL_NONONIMAGE"
	TipNoNonImage = "TIP_NONONIMAGE"
	TglNoConvert  = "TGL_NOCONVERT"
	TipNoConvert  = "TIP_NOCONVERT"
	LblMinSize    = "LBL_MINSIZE"
	TipSize       = "TIP_SIZE"
	LblDPI        = "LBL_DPI"
	TipDPI        = "TIP_DPI"

	LblOutDir      = "LBL_OUTDIR"
	TipOutDir      = "TIP_OUTDIR"
	BtnBrowse      = "BTN_BROWSE"
	LblSuffix      = "LBL_SUFFIX"
	TipSuffix      = "TIP_SUFFIX"
	LblArchive     = "LBL_ARCHIVE"
	TipArchive     = "TIP_ARCHIVE"
	LblCompression = "LBL_COMPRESSION"
	TipZipLevel    = "TIP_ZIPLEVEL"
	TglCombine     = "TGL_COMBINE"
	TipCombine     = "TIP_COMBINE"
	LblOutFile     = "LBL_OUTFILE"
	TipOutFile     = "TIP_OUTFILE"

	LblFormat      = "LBL_FORMAT"
	TipFormat      = "TIP_FORMAT"
	LblSize        = "LBL_SIZE"
	CueWidth       = "CUE_WIDTH"
	CueHeight      = "CUE_HEIGHT"
	TipWidthHeight = "TIP_WIDTHHEIGHT"
	TglFit         = "TGL_FIT"
	TipFit         = "TIP_FIT"
	TglNoUpscale   = "TGL_NOUPSCALE"
	TipNoUpscale   = "TIP_NOUPSCALE"
	LblFilter      = "LBL_FILTER"
	LblQuality     = "LBL_QUALITY"
	TipQuality     = "TIP_QUALITY"
	LblEffort      = "LBL_EFFORT"
	TipEffort      = "TIP_EFFORT"
	TglLossless    = "TGL_LOSSLESS"
	TipLossless    = "TIP_LOSSLESS"
	TglGrayscale   = "TGL_GRAYSCALE"
	TipGrayscale   = "TIP_GRAYSCALE"

	LblBrightness = "LBL_BRIGHTNESS"
	TipBrightness = "TIP_BRIGHTNESS"
	LblContrast   = "LBL_CONTRAST"
	TipContrast   = "TIP_CONTRAST"
	LblRotate     = "LBL_ROTATE"
	TipRotate     = "TIP_ROTATE"

	EffortMethod  = "EFFORT_METHOD"
	EffortSpeed   = "EFFORT_SPEED"
	EffortEffort  = "EFFORT_EFFORT"
	TipEffortWebp = "TIP_EFFORT_WEBP"
	TipEffortAvif = "TIP_EFFORT_AVIF"
	TipEffortJxl  = "TIP_EFFORT_JXL"

	BtnAddFiles  = "BTN_ADDFILES"
	BtnAddDir    = "BTN_ADDDIR"
	BtnRemove    = "BTN_REMOVE"
	BtnRemoveAll = "BTN_REMOVEALL"
	BtnThumbnail = "BTN_THUMBNAIL"
	BtnCover     = "BTN_COVER"
	BtnConvert   = "BTN_CONVERT"
	BtnCancel    = "BTN_CANCEL"
	TipCancel    = "TIP_CANCEL"
	BtnReset     = "BTN_RESET"
	TipReset     = "TIP_RESET"
	BtnSave      = "BTN_SAVE"
	TipSave      = "TIP_SAVE"
	BtnCommand   = "BTN_COMMAND"
	TipCommand   = "TIP_COMMAND"
	LblProfile   = "LBL_PROFILE"
	TipProfile   = "TIP_PROFILE"

	TipThumbnail = "TIP_THUMBNAIL"
	TipCover     = "TIP_COVER"
	TipConvert   = "TIP_CONVERT"

	StatusNeedFilesAndDir = "STATUS_NEED_FILES_AND_DIR"
	StatusNeedFiles       = "STATUS_NEED_FILES"
	StatusNeedOutDir      = "STATUS_NEED_OUTDIR"
	StatusFileOf          = "STATUS_FILE_OF"

	FilterNearest  = "FILTER_NEAREST"
	FilterBox      = "FILTER_BOX"
	FilterLinear   = "FILTER_LINEAR"
	FilterMitchell = "FILTER_MITCHELL"
	FilterCatmull  = "FILTER_CATMULL"
	FilterGaussian = "FILTER_GAUSSIAN"
	FilterLanczos  = "FILTER_LANCZOS"

	DlgAddFiles    = "DLG_ADDFILES"
	DlgAddDir      = "DLG_ADDDIR"
	DlgOutputDir   = "DLG_OUTPUTDIR"
	DlgOutputFile  = "DLG_OUTPUTFILE"
	DlgCommandLine = "DLG_COMMANDLINE"
	DlgSaveProfile = "DLG_SAVEPROFILE"

	ParamName           = "PARAM_NAME"
	MsgInvalidNameTitle = "MSG_INVALIDNAME_TITLE"
	MsgInvalidNameBody  = "MSG_INVALIDNAME_BODY"

	NoPreview = "NO_PREVIEW"
)

// packs maps an IUP language name to its message pack, filled by each language file's init.
var packs = map[string]map[string]string{}

// register adds a language pack; called from each i18n_<lang>.go init.
func register(lang string, pack map[string]string) {
	packs[lang] = pack
}

// langByCode maps a two-letter language code to the IUP language name.
var langByCode = map[string]string{
	"en": "ENGLISH",
	"pt": "PORTUGUESE",
	"es": "SPANISH",
	"cs": "CZECH",
	"ru": "RUSSIAN",
	"de": "GERMAN",
	"fr": "FRENCH",
	"zh": "CHINESE",
	"ja": "JAPANESE",
	"it": "ITALIAN",
}

// Lng wraps a message key for IUP's automatic language-string lookup.
func Lng(key string) string {
	return "_@" + key
}

// Str returns the translated string for a key, for use where IUP's "_@" prefix does not apply.
func Str(key string) string {
	return iup.GetLanguageString(key)
}

// Init detects the system language, switches IUP's predefined strings to it, and registers the message packs.
func Init() {
	lang := detect()
	iup.SetLanguage(lang)

	registerPack(packs["ENGLISH"])
	if lang != "ENGLISH" {
		if pack, ok := packs[lang]; ok {
			registerPack(pack)
		}
	}
}

func registerPack(pack map[string]string) {
	for name, value := range pack {
		iup.SetLanguageString(name, value)
	}
}

// detect returns the IUP language name for the system locale, or ENGLISH.
func detect() string {
	if name, ok := langByCode[systemLangCode()]; ok {
		return name
	}

	return "ENGLISH"
}
