package main

// OMugen.exe started outside a MUGEN game (just unzipped in Downloads, say)
// explains where it belongs and offers to make a new game, instead of
// failing on the missing select.def. A game whose select.def is missing gets
// a message that names it, instead of a script error.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// omugenChecksSkipped: Android has its own game folder, the doctor writes its
// own report, and -r names a screenpack anywhere.
func omugenChecksSkipped() bool {
	if runtime.GOOS == "android" {
		return true
	}
	for _, f := range []string{"-doctor", "-r", "-rubric"} {
		if _, ok := sys.cmdFlags[f]; ok {
			return true
		}
	}
	return false
}

// omugenLooksLikeGame reports whether the current folder holds a game, or
// what is left of one (then a missing select.def is that game's problem).
func omugenLooksLikeGame() bool {
	for _, p := range []string{"data", "mugen.exe", "select.def"} {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	return false
}

// omugenNoGame runs before anything is written when the folder has no game.
// It returns true when OMugen should quit.
func omugenNoGame() bool {
	if omugenChecksSkipped() || omugenLooksLikeGame() {
		return false
	}
	dir, _ := os.Getwd()
	if exe, err := os.Executable(); err == nil {
		dir = filepath.Dir(exe)
	}
	msg := fmt.Sprintf("There's no MUGEN game in this folder:\n%s\n\n"+
		"To play a MUGEN 1.1 game, copy OMugen.exe into the game's folder "+
		"(where mugen.exe and the data, chars and stages folders are) and start it there.\n\n"+
		"Or make a game of your own: OMugen creates one, ready to play, with a template "+
		"fighter, a stage, menus and lifebars.", dir)
	if _, quiet := sys.cmdFlags["-nodialog"]; quiet {
		fmt.Fprintln(os.Stderr, msg)
		return true
	}
	if !ShowYesNoDialog(msg+"\n\nCreate a new game here now?", "OMugen") {
		return true
	}
	game, err := createNewGameIn(dir, "")
	if err != nil {
		ShowErrorDialog("Could not create the new game:\n" + err.Error())
		return true
	}
	ShowInfoDialog(fmt.Sprintf("Your new game is ready:\n%s\n\n"+
		"It starts now. Next time, start OMugen.exe in that folder.\n"+
		"GUIDE.txt explains how to make your own characters and stages.", game), "OMugen - New game")
	exe, _ := os.Executable()
	cmd := exec.Command(filepath.Join(game, filepath.Base(exe)))
	cmd.Dir = game
	if err := cmd.Start(); err != nil {
		ShowErrorDialog("Could not start the new game:\n" + err.Error())
	}
	return true
}

// omugenGameSelect finds the game's select.def as the screenpack names it,
// looked up like the engine does (next to the screenpack, then the game
// folder, then data). It returns "" when it is missing, with the name.
func omugenGameSelect() (path, name string) {
	motif := gameMotifPath()
	name = "select.def"
	if v := readMotifFilesKey(motif, "select"); v != "" {
		name = v
	}
	return FileExist(SearchFile(name, []string{motif, "", "data/"})), name
}

// omugenMissingSelect runs once the config is read: a game without its
// select.def can't start. It returns true when OMugen should quit.
func omugenMissingSelect() bool {
	if omugenChecksSkipped() {
		return false
	}
	path, name := omugenGameSelect()
	if path != "" {
		return false
	}
	where := "in the game folder and in data"
	if motif := gameMotifPath(); !isOMugenMotif(motif) {
		where = fmt.Sprintf("next to the screenpack (%s), %s", mugenSlash(motif), where)
	}
	msg := fmt.Sprintf("This game can't start: its list of characters and stages, %s, is missing.\n\n"+
		"OMugen looked for it %s.", mugenSlash(name), where)
	LogMessage("OMugen: %s", msg)
	if _, quiet := sys.cmdFlags["-nodialog"]; !quiet {
		ShowErrorDialog(msg)
	}
	return true
}
