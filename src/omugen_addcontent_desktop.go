//go:build !raw && !android

package main

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/sqweek/dialog"
)

// omugenAddPick asks for an archive, a .def file or a folder to add. Several
// at once: drop them on the window.
func omugenAddPick() {
	b := dialog.File().Title("Add characters and stages (or drop them on the OMugen window)").
		Filter("Characters and stages (zip, rar, 7z, def)", "zip", "rar", "7z", "def").
		Filter("All files", "*")
	if home, err := os.UserHomeDir(); err == nil {
		if fi, err := os.Stat(filepath.Join(home, "Downloads")); err == nil && fi.IsDir() {
			b = b.SetStartDir(filepath.Join(home, "Downloads"))
		}
	}
	if p, err := b.Load(); err == nil && p != "" {
		omugenAddQueue(p)
	}
}

// omugenRestart starts OMugen again with the same options, then quits.
func omugenRestart() {
	exe, err := os.Executable()
	if err == nil {
		cmd := exec.Command(exe, omugenLaunchArgs...)
		if wd, werr := os.Getwd(); werr == nil {
			cmd.Dir = wd
		}
		err = cmd.Start()
	}
	if err != nil {
		LogMessage("OMugen: restart failed: %v", err)
		ShowInfoDialog("Start OMugen again to play with them.", "Add characters and stages")
	}
	handleExit()
}
