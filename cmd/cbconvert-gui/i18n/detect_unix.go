//go:build !windows

package i18n

import (
	"os"
	"strings"
)

// systemLangCode returns the two-letter language code from the POSIX locale environment, or "".
func systemLangCode() string {
	for _, env := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		v := os.Getenv(env)
		if v == "" || v == "C" || v == "POSIX" {
			continue
		}

		if i := strings.IndexAny(v, "_.@"); i >= 0 {
			v = v[:i]
		}

		return strings.ToLower(v)
	}

	return ""
}
