package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewGameFromEnginePack(t *testing.T) {
	gameDir := withEngine(t, map[string]string{
		"templates/newgame/GUIDE.txt":                "guide",
		"templates/newgame/data/select.def":          "[Characters]\nproto",
		"templates/newgame/chars/proto/proto.def":    "[Info]",
		"templates/newgame/stages/proto_arena.def":   "[Info]",
		"data/omugen/system.def":                     "[Info]",
		"data/omugen/system.sff":                     "sff",
		"data/omugen/fonts/title.ttf":                "ttf",
		"data/omugen/omugen-fallback-lifebars/f.def": "[Info]",
	})
	saved := startDir
	startDir = gameDir
	t.Cleanup(func() { startDir = saved })

	dir, err := createNewGame("Test Game")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(gameDir, "Test Game"); dir != want {
		t.Fatalf("created %s, want %s", dir, want)
	}
	exe, _ := os.Executable()
	for _, f := range []string{"GUIDE.txt", "data/select.def", "chars/proto/proto.def",
		"stages/proto_arena.def", "data/omugen/system.def", "data/omugen/system.sff",
		"data/omugen/fonts/title.ttf", filepath.Base(exe)} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Errorf("missing %s: %v", f, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "data/omugen/omugen-fallback-lifebars")); err == nil {
		t.Errorf("the engine's fallback lifebars must not be copied")
	}
	cfg, err := os.ReadFile(filepath.Join(dir, "save", "config.ini"))
	if err != nil || !strings.Contains(string(cfg), "WindowTitle = Test Game") {
		t.Errorf("save/config.ini = %q, %v", cfg, err)
	}

	// Never writes into a folder that has files.
	if _, err := createNewGame("Test Game"); err == nil {
		t.Errorf("creating over an existing game must fail")
	}

	// No name: the first free default name.
	a, err := createNewGame("")
	if err != nil {
		t.Fatal(err)
	}
	b, err := createNewGame("")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(a) != newGameDefaultName || filepath.Base(b) != newGameDefaultName+" 2" {
		t.Errorf("default names: %s, %s", filepath.Base(a), filepath.Base(b))
	}
}

// The real OMugen screenpack: the new game's copy describes the game.
func TestNewGameMotifInfobox(t *testing.T) {
	def, err := os.ReadFile("../screenpack/omugen/system.def")
	if err != nil {
		t.Fatal(err)
	}
	out := string(newGameMotif(def, "Dojo 100% Clash"))
	for _, want := range []string{
		"title.text = Dojo 100% Clash\n",
		"[Infobox Text]\nDojo 100%% Clash\nMade with OMugen (%s - %s)\n",
		"[Select Info]",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q", want)
		}
	}
	for _, gone := range []string{"built-in OMugen screenpack", "Switch screenpacks", "title.text = OMUGEN"} {
		if strings.Contains(out, gone) {
			t.Errorf("still has %q", gone)
		}
	}
	if strings.Count(out, "\n") < strings.Count(string(def), "\n")-3 {
		t.Errorf("lost lines: %d -> %d", strings.Count(string(def), "\n"), strings.Count(out, "\n"))
	}
}
