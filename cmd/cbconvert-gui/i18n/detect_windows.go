//go:build windows

package i18n

import "syscall"

// primaryLang maps a Windows primary-language id to a two-letter language code.
var primaryLang = map[uint16]string{
	0x09: "en",
	0x16: "pt",
	0x0a: "es",
	0x05: "cs",
	0x19: "ru",
	0x07: "de",
	0x0c: "fr",
	0x04: "zh",
	0x11: "ja",
	0x10: "it",
}

// systemLangCode returns the two-letter code of the user's default UI language, or "".
func systemLangCode() string {
	r, _, _ := syscall.NewLazyDLL("kernel32.dll").NewProc("GetUserDefaultUILanguage").Call()

	return primaryLang[uint16(r)&0x3ff]
}
