package main

// OMugen.exe -newgame ["My Game"] starts a game of your own: it creates a
// game folder with a copy of OMugen.exe, the template fighter Proto, the
// Proto Arena stage, an editable copy of the OMugen screenpack and GUIDE.txt,
// which explains how to make characters and stages.
//
// The template comes from the engine pack (templates/newgame, packed by
// tools/packengine) or, in development builds, from the repository.

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// startDir is the folder OMugen was started from, before it switches to the
// exe's folder: a relative -newgame folder is created there.
var startDir, _ = os.Getwd()

const newGameDefaultName = "My OMugen Game"

func runNewGame(arg string) {
	dir, err := createNewGame(arg)
	if err != nil {
		msg := "Could not create the new game:\n" + err.Error()
		fmt.Fprintln(os.Stderr, msg)
		if _, quiet := sys.cmdFlags["-nodialog"]; !quiet {
			ShowErrorDialog(msg)
		}
		return
	}
	msg := fmt.Sprintf("Your new game is ready:\n%s\n\n"+
		"Start it with OMugen.exe in that folder.\n"+
		"GUIDE.txt explains how to make your own characters and stages.", dir)
	if len(engineZipData) == 0 {
		msg += "\n\nThis OMugen.exe was built without its engine files: " +
			"replace the copy in the new folder with a release OMugen.exe."
	}
	fmt.Fprintln(os.Stderr, msg)
	if _, quiet := sys.cmdFlags["-nodialog"]; !quiet {
		ShowInfoDialog(msg, "OMugen - New game")
	}
}

// createNewGame creates the game folder and returns its absolute path. It
// never writes into a folder that already has files in it.
func createNewGame(arg string) (string, error) {
	return createNewGameIn(startDir, arg)
}

// createNewGameIn is createNewGame with a relative arg (or the default name)
// taken from the folder base.
func createNewGameIn(base, arg string) (string, error) {
	dst, err := newGameDir(base, arg)
	if err != nil {
		return "", err
	}
	tmpl := firstDir(newGameSearchDirs("templates/newgame", "templates/newgame")...)
	if tmpl == "" {
		return "", fmt.Errorf("the game template (templates/newgame) is missing")
	}
	screenpack := firstDir(newGameSearchDirs("data/omugen", "screenpack/omugen")...)
	if screenpack == "" {
		return "", fmt.Errorf("the OMugen screenpack is missing")
	}
	lifebars := firstDir(newGameSearchDirs(path.Dir(omugenFightDef), "screenpack/lifebars")...)
	if lifebars == "" {
		return "", fmt.Errorf("the OMugen lifebars are missing")
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(dst, 0o755); err != nil {
		return "", err
	}
	if err := copyTree(tmpl, dst); err != nil {
		return "", err
	}
	// The screenpack (system.def, its sprites and fonts) and a copy of the
	// lifebars, which the game makes its own.
	if err := copyFiles(screenpack, filepath.Join(dst, "data", "omugen")); err != nil {
		return "", err
	}
	if err := copyTree(filepath.Join(screenpack, "fonts"), filepath.Join(dst, "data", "omugen", "fonts")); err != nil {
		return "", err
	}
	if err := copyTree(lifebars, filepath.Join(dst, "data", "omugen", "lifebars")); err != nil {
		return "", err
	}
	motif := filepath.Join(dst, "data", "omugen", "system.def")
	if def, err := os.ReadFile(motif); err == nil {
		if err := os.WriteFile(motif, newGameMotif(def, filepath.Base(dst)), 0o644); err != nil {
			return "", err
		}
	}
	if err := copyFile(exe, filepath.Join(dst, filepath.Base(exe))); err != nil {
		return "", err
	}
	// Outside Windows the runtime libraries ship next to the executable
	// (lib/), which loads them before it starts.
	if lib := filepath.Join(filepath.Dir(exe), "lib"); runtime.GOOS != "windows" && firstDir(lib) != "" {
		if err := copyTree(lib, filepath.Join(dst, "lib")); err != nil {
			return "", err
		}
	}
	// The game's name is its window title.
	cfg := fmt.Sprintf("; Settings of this game: any value missing here uses OMugen's default.\n"+
		"[Config]\nWindowTitle = %s\n", filepath.Base(dst))
	if err := os.MkdirAll(filepath.Join(dst, "save"), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dst, "save", "config.ini"), []byte(cfg), 0o644); err != nil {
		return "", err
	}
	return dst, nil
}

var (
	infoboxTitleRe = regexp.MustCompile(`(?m)^(title\.text[ \t]*=[ \t]*)OMUGEN[ \t]*$`)
	infoboxTextRe  = regexp.MustCompile(`(?s)(\[Infobox Text\]\r?\n).*?(\r?\n;-)`)
	fightKeyRe     = regexp.MustCompile(`(?m)^(fight[ \t]*=[ \t]*)fight\.def[^\r\n]*`)
)

// newGameMotif turns the info box of the screenpack copy (shown on the first
// run) from a note about OMugen's built-in screenpack into one about the game.
func newGameMotif(def []byte, name string) []byte {
	// The game's lifebars are its own copy of OMugen's (data/omugen/lifebars).
	def = fightKeyRe.ReplaceAll(def, []byte("${1}lifebars/fight.def   ;this game's lifebars (data/omugen/lifebars)"))
	def = infoboxTitleRe.ReplaceAllFunc(def, func(m []byte) []byte {
		return append(append([]byte{}, infoboxTitleRe.FindSubmatch(m)[1]...), name...)
	})
	// The info box text is a format string (version, build date).
	text := strings.ReplaceAll(name, "%", "%%") + "\nMade with OMugen (%s - %s)\n\n" +
		"* This box appears the first time the game starts.\n" +
		"* Game makers: write your own text in data/omugen/system.def,\n" +
		"  section [Infobox Text].\n"
	return infoboxTextRe.ReplaceAllFunc(def, func(m []byte) []byte {
		sub := infoboxTextRe.FindSubmatch(m)
		return append(append(append([]byte{}, sub[1]...), text...), sub[2]...)
	})
}

// newGameDir picks the new game's folder: arg (relative to base), or the
// first free "My OMugen Game", "My OMugen Game 2"... when arg is empty.
func newGameDir(base, arg string) (string, error) {
	arg = strings.TrimSpace(arg)
	if base == "" {
		base = "."
	}
	if arg == "" {
		for i := 1; ; i++ {
			name := newGameDefaultName
			if i > 1 {
				name = fmt.Sprintf("%s %d", newGameDefaultName, i)
			}
			dir := filepath.Join(base, name)
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				return filepath.Abs(dir)
			}
		}
	}
	dir := arg
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(base, dir)
	}
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	if entries, err := os.ReadDir(dir); err == nil && len(entries) > 0 {
		return "", fmt.Errorf("%s already exists and is not empty", dir)
	} else if err == nil || os.IsNotExist(err) {
		return dir, nil
	} else {
		return "", err
	}
}

// newGameSearchDirs lists where a template folder can be: the extracted
// engine files (packPath), then the exe's folder and the start folder in a
// development checkout (repoPath).
func newGameSearchDirs(packPath, repoPath string) []string {
	var dirs []string
	if engineDir != "" {
		dirs = append(dirs, filepath.Join(engineDir, filepath.FromSlash(packPath)))
	}
	if exe, err := os.Executable(); err == nil {
		dirs = append(dirs, filepath.Join(filepath.Dir(exe), filepath.FromSlash(repoPath)))
	}
	if startDir != "" {
		dirs = append(dirs, filepath.Join(startDir, filepath.FromSlash(repoPath)))
	}
	return dirs
}

func firstDir(dirs ...string) string {
	for _, d := range dirs {
		if fi, err := os.Stat(d); err == nil && fi.IsDir() {
			return d
		}
	}
	return ""
}

// copyFiles copies the files (not the subfolders) of src into dst.
func copyFiles(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if err := copyFile(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

// copyTree copies the folder src into dst, subfolders included.
func copyTree(src, dst string) error {
	return filepath.Walk(src, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if fi.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
