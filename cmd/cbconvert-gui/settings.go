package main

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/gen2brain/cbconvert/cmd/cbconvert-gui/i18n"
	"github.com/gen2brain/iup-go/iup"
)

const (
	pathsGroup    = "Paths"
	profilesGroup = "Profiles"

	inputDirKey  = "InputDir"
	outputDirKey = "OutputDir"
)

type settingKind int

const (
	kindBool settingKind = iota
	kindInt
	kindStr
)

type setting struct {
	handle string
	kind   settingKind
	def    string
}

var settings = []setting{
	{"Recursive", kindBool, "OFF"},
	{"NoRGB", kindBool, "OFF"},
	{"NoCover", kindBool, "OFF"},
	{"NoConvert", kindBool, "OFF"},
	{"NoNonImage", kindBool, "OFF"},
	{"Combine", kindBool, "OFF"},
	{"Fit", kindBool, "OFF"},
	{"NoUpscale", kindBool, "OFF"},
	{"Lossless", kindBool, "OFF"},
	{"Grayscale", kindBool, "OFF"},
	{"OutDir", kindStr, ""},
	{"Suffix", kindStr, ""},
	{"Width", kindStr, ""},
	{"Height", kindStr, ""},
	{"DPI", kindStr, "Default"},
	{"Size", kindInt, "0"},
	{"Quality", kindInt, "75"},
	{"Effort", kindInt, "0"},
	{"Brightness", kindInt, "0"},
	{"Contrast", kindInt, "0"},
	{"Format", kindInt, "1"},
	{"Archive", kindInt, "1"},
	{"ZipLevel", kindInt, "1"},
	{"Filter", kindInt, "3"},
	{"Rotate", kindInt, "1"},
}

func profileGroup(name string) string {
	return "Profile:" + name
}

func profileNames() []string {
	s := iup.ConfigGetVariableStr(config, profilesGroup, "Names")
	if s == "" {
		return nil
	}

	return strings.Split(s, ";")
}

func currentProfile() string {
	return iup.ConfigGetVariableStrDef(config, profilesGroup, "Current", "Default")
}

func setStartDir(dlg iup.Ihandle, key string) {
	if dir := iup.ConfigGetVariableStr(config, pathsGroup, key); dir != "" {
		dlg.SetAttribute("DIRECTORY", dir)
	}
}

func rememberDir(dlg iup.Ihandle, key string) {
	dir := dlg.GetAttribute("DIRECTORY")
	if dir == "" {
		return
	}

	iup.ConfigSetVariableStr(config, pathsGroup, key, dir)
	iup.ConfigSave(config)
}

func settingsSave(group string) {
	for _, s := range settings {
		h := iup.GetHandle(s.handle)
		switch s.kind {
		case kindBool:
			v := 0
			if h.GetAttribute("VALUE") == "ON" {
				v = 1
			}
			iup.ConfigSetVariableInt(config, group, s.handle, v)
		case kindInt:
			iup.ConfigSetVariableInt(config, group, s.handle, h.GetInt("VALUE"))
		case kindStr:
			iup.ConfigSetVariableStr(config, group, s.handle, h.GetAttribute("VALUE"))
		}
	}

	iup.ConfigSave(config)
}

// settingsApply sets every control from the given profile group, or from defaults when a group is empty.
func settingsApply(group string) {
	for _, s := range settings {
		h := iup.GetHandle(s.handle)
		switch s.kind {
		case kindBool:
			def := 0
			if s.def == "ON" {
				def = 1
			}
			v := def
			if group != "" {
				v = iup.ConfigGetVariableIntDef(config, group, s.handle, def)
			}
			if v != 0 {
				h.SetAttribute("VALUE", "ON")
			} else {
				h.SetAttribute("VALUE", "OFF")
			}
		case kindInt:
			def, _ := strconv.Atoi(s.def)
			v := def
			if group != "" {
				v = iup.ConfigGetVariableIntDef(config, group, s.handle, def)
			}
			h.SetAttribute("VALUE", strconv.Itoa(v))
		case kindStr:
			v := s.def
			if group != "" {
				v = iup.ConfigGetVariableStrDef(config, group, s.handle, s.def)
			}
			h.SetAttribute("VALUE", v)
		}
	}

	syncLabels()
	setActive()
	previewPost()
}

// syncLabels mirrors slider values into their value labels and retunes the effort slider for the current format.
func syncLabels() {
	iup.GetHandle("LabelQuality").SetAttribute("TITLE", iup.GetHandle("Quality").GetInt("VALUE"))
	iup.GetHandle("LabelBrightness").SetAttribute("TITLE", iup.GetHandle("Brightness").GetInt("VALUE"))
	iup.GetHandle("LabelContrast").SetAttribute("TITLE", iup.GetHandle("Contrast").GetInt("VALUE"))

	format := strings.ToLower(iup.GetHandle("Format").GetAttribute("VALUESTRING"))
	eff := iup.GetHandle("Effort").GetInt("VALUE")
	setEffort(format)
	switch format {
	case "webp", "avif", "jxl":
		val := iup.GetHandle("Effort")
		val.SetAttribute("VALUE", strconv.Itoa(eff))
		iup.GetHandle("LabelEffort").SetAttribute("TITLE", fmt.Sprintf("%s: %d", val.GetAttribute("EFFORTNAME"), eff))
	}

	iup.Refresh(iup.GetHandle("Tabs"))
}

func fillProfileList() {
	list := iup.GetHandle("Profile")
	list.SetAttribute("REMOVEITEM", "ALL")

	cur := currentProfile()
	sel := 1
	for i, n := range profileNames() {
		list.SetAttribute(strconv.Itoa(i+1), n)
		if n == cur {
			sel = i + 1
		}
	}

	list.SetAttribute("VALUE", strconv.Itoa(sel))
}

// profilesInit loads the current profile on startup, creating a default one on the first run.
func profilesInit() {
	if len(profileNames()) == 0 {
		iup.ConfigSetVariableStr(config, profilesGroup, "Names", "Default")
		iup.ConfigSetVariableStr(config, profilesGroup, "Current", "Default")
		settingsSave(profileGroup("Default"))
	}

	fillProfileList()
	settingsApply(profileGroup(currentProfile()))
}

func onProfileSelect(ih iup.Ihandle) int {
	name := ih.GetAttribute("VALUESTRING")
	if name == "" {
		return iup.DEFAULT
	}

	iup.ConfigSetVariableStr(config, profilesGroup, "Current", name)
	iup.ConfigSave(config)

	settingsApply(profileGroup(name))

	return iup.DEFAULT
}

func onSave(iup.Ihandle) int {
	name := currentProfile()
	if iup.GetParam(i18n.Str(i18n.DlgSaveProfile), nil, i18n.Str(i18n.ParamName), &name) != 1 {
		return iup.DEFAULT
	}

	name = strings.TrimSpace(name)
	if name == "" || strings.ContainsAny(name, ".;") {
		iup.Message(i18n.Str(i18n.MsgInvalidNameTitle), i18n.Str(i18n.MsgInvalidNameBody))

		return iup.DEFAULT
	}

	settingsSave(profileGroup(name))

	names := profileNames()
	if !slices.Contains(names, name) {
		names = append(names, name)
		iup.ConfigSetVariableStr(config, profilesGroup, "Names", strings.Join(names, ";"))
	}

	iup.ConfigSetVariableStr(config, profilesGroup, "Current", name)
	iup.ConfigSave(config)

	fillProfileList()

	return iup.DEFAULT
}

func onReset(iup.Ihandle) int {
	settingsApply("")

	return iup.DEFAULT
}
