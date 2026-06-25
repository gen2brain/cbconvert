package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestProfileArg(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"--profile", "webp", "a.cbz"}, "webp"},
		{[]string{"--profile=webp", "a.cbz"}, "webp"},
		{[]string{"-profile", "webp"}, "webp"},
		{[]string{"-profile=webp"}, "webp"},
		{[]string{"--width", "800", "--profile", "x", "a.cbz"}, "x"},
		{[]string{"--width", "800", "a.cbz"}, ""},
		{[]string{"--profile"}, ""},
	}

	for _, c := range cases {
		if got := profileArg(c.args); got != c.want {
			t.Errorf("profileArg(%v) = %q, want %q", c.args, got, c.want)
		}
	}
}

func TestIndexTranslations(t *testing.T) {
	if got := formatFromIndex("5"); got != "webp" {
		t.Errorf("formatFromIndex(5) = %q, want webp", got)
	}
	if got := formatFromIndex("1"); got != "jpeg" {
		t.Errorf("formatFromIndex(1) = %q, want jpeg", got)
	}
	if got := formatFromIndex("99"); got != "jpeg" {
		t.Errorf("formatFromIndex(99) = %q, want jpeg fallback", got)
	}
	if got := archiveFromIndex("2"); got != "tar" {
		t.Errorf("archiveFromIndex(2) = %q, want tar", got)
	}
	if got := archiveFromIndex("1"); got != "zip" {
		t.Errorf("archiveFromIndex(1) = %q, want zip", got)
	}

	zip := map[string]int{"1": -1, "2": 0, "3": 1, "11": 9}
	for in, want := range zip {
		if got := zipLevelFromIndex(in); got != want {
			t.Errorf("zipLevelFromIndex(%s) = %d, want %d", in, got, want)
		}
	}

	rot := map[string]int{"1": 0, "2": 90, "3": 180, "4": 270}
	for in, want := range rot {
		if got := rotateFromIndex(in); got != want {
			t.Errorf("rotateFromIndex(%s) = %d, want %d", in, got, want)
		}
	}

	if got := dpiFromString("Default"); got != 0 {
		t.Errorf("dpiFromString(Default) = %d, want 0", got)
	}
	if got := dpiFromString("150"); got != 150 {
		t.Errorf("dpiFromString(150) = %d, want 150", got)
	}
}

func TestParseINI(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	data := "[Profiles]\nNames=Default;webp\n\n[Profile:webp]\nFormat=5\nQuality=90\n"
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}

	ini, err := parseINI(path)
	if err != nil {
		t.Fatal(err)
	}
	if ini["Profile:webp"]["Format"] != "5" {
		t.Errorf("Format = %q, want 5", ini["Profile:webp"]["Format"])
	}
	if ini["Profiles"]["Names"] != "Default;webp" {
		t.Errorf("Names = %q", ini["Profiles"]["Names"])
	}
}

func TestLoadProfile(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("config path override via XDG_CONFIG_HOME is Linux-specific")
	}

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "cbconvert"), 0755); err != nil {
		t.Fatal(err)
	}
	data := "[Profile:webp]\nFormat=5\nQuality=90\nEffort=4\nArchive=2\nWidth=800\nFit=1\nFilter=7\nRotate=2\n"
	if err := os.WriteFile(filepath.Join(dir, "cbconvert", "config"), []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", dir)

	o, err := loadProfile("webp")
	if err != nil {
		t.Fatal(err)
	}

	if o.Format != "webp" {
		t.Errorf("Format = %q, want webp", o.Format)
	}
	if o.Quality != 90 {
		t.Errorf("Quality = %d, want 90", o.Quality)
	}
	if o.Effort != 4 {
		t.Errorf("Effort = %d, want 4 (webp keeps the slider value)", o.Effort)
	}
	if o.Archive != "tar" {
		t.Errorf("Archive = %q, want tar", o.Archive)
	}
	if o.Width != 800 || !o.Fit {
		t.Errorf("Width/Fit = %d/%v, want 800/true", o.Width, o.Fit)
	}
	if o.Filter != 6 {
		t.Errorf("Filter = %d, want 6 (GUI index 7 - 1)", o.Filter)
	}
	if o.Rotate != 90 {
		t.Errorf("Rotate = %d, want 90", o.Rotate)
	}

	if _, err := loadProfile("missing"); err == nil {
		t.Error("loadProfile(missing) should error")
	}
}

func TestLoadProfileEffortGate(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("config path override via XDG_CONFIG_HOME is Linux-specific")
	}

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "cbconvert"), 0755); err != nil {
		t.Fatal(err)
	}
	// Format=1 (jpeg) with a stored Effort must collapse to -1, mirroring the GUI.
	data := "[Profile:jpeg]\nFormat=1\nEffort=4\n"
	if err := os.WriteFile(filepath.Join(dir, "cbconvert", "config"), []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", dir)

	o, err := loadProfile("jpeg")
	if err != nil {
		t.Fatal(err)
	}
	if o.Effort != -1 {
		t.Errorf("Effort = %d, want -1 for non-effort format", o.Effort)
	}
}
