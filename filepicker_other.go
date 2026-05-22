//go:build !windows

package main

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
)

// openOutputFolder opens the given path in the system file manager.
func openOutputFolder(path string) {
	switch runtime.GOOS {
	case "darwin":
		exec.Command("open", path).Start() //nolint:errcheck
	default:
		exec.Command("xdg-open", path).Start() //nolint:errcheck
	}
}

// pickFile shows the Fyne file-open dialog (fallback on non-Windows platforms).
func pickFile(title string, w fyne.Window, callback func(string)) {
	dialog.ShowFileOpen(func(f fyne.URIReadCloser, err error) {
		if err != nil || f == nil {
			return
		}
		defer f.Close()
		p := filepath.FromSlash(strings.TrimPrefix(f.URI().Path(), "/"))
		callback(p)
	}, w)
}

// pickFolder shows the Fyne folder-open dialog (fallback on non-Windows platforms).
func pickFolder(title string, w fyne.Window, callback func(string)) {
	dialog.ShowFolderOpen(func(lu fyne.ListableURI, err error) {
		if err != nil || lu == nil {
			return
		}
		p := filepath.FromSlash(strings.TrimPrefix(lu.Path(), "/"))
		callback(p)
	}, w)
}
