//go:build portal

package main

import (
	"net/url"

	"github.com/godbus/dbus/v5"
)

func fileDlg(title string, multiple, directory bool) ([]string, error) {
	ret := make([]string, 0)

	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return ret, err
	}
	defer conn.Close()

	dest := "org.freedesktop.portal.Desktop"
	path := "/org/freedesktop/portal/desktop"
	resp := "org.freedesktop.portal.Request.Response"

	if err = conn.AddMatchSignal(
		dbus.WithMatchInterface(dest),
		dbus.WithMatchObjectPath(dbus.ObjectPath(path)),
		dbus.WithMatchSender(conn.Names()[0]),
	); err != nil {
		return ret, err
	}

	c := make(chan *dbus.Signal, 10)
	conn.Signal(c)

	type Item struct {
		Index  uint32
		Filter string
	}

	type Filter struct {
		Title   string
		Filters []Item
	}

	filters := []Filter{
		{
			"Comic Files",
			[]Item{
				Item{0, "*.rar"},
				Item{0, "*.zip"},
				Item{0, "*.7z"},
				Item{0, "*.tar"},
				Item{0, "*.cbr"},
				Item{0, "*.cbz"},
				Item{0, "*.cb7"},
				Item{0, "*.cbt"},
				Item{0, "*.pdf"},
				Item{0, "*.epub"},
				Item{0, "*.mobi"},
				Item{0, "*.docx"},
				Item{0, "*.pptx"},
			},
		},
	}

	opts := map[string]any{
		"multiple":  multiple,
		"directory": directory,
	}

	if !directory {
		opts["filters"] = filters
	}

	obj := conn.Object(dest, dbus.ObjectPath(path))
	call := obj.Call("org.freedesktop.portal.FileChooser.OpenFile", 0, "", title, opts)
	if call.Err != nil {
		return ret, call.Err
	}

	for v := range c {
		if v.Name != resp {
			continue
		}

		status := v.Body[0].(uint32)

		if status == 0 {
			m := v.Body[1].(map[string]dbus.Variant)
			uris := m["uris"].Value().([]string)

			for _, uri := range uris {
				u, err := url.ParseRequestURI(uri)
				if err != nil {
					return ret, err
				}

				ret = append(ret, u.Path)
			}
		}

		break
	}

	return ret, nil
}
