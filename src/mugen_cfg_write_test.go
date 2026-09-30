package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/veandco/go-sdl2/sdl"
)

func TestEditMugenCfgTextPreservesEverythingElse(t *testing.T) {
	in := "; header comment\r\n[Input]\r\nP1.UseKeyboard = 1\r\n\r\n[P1 Keys]\r\nJump   = 119 ; up\r\nCrouch = 115\r\n  A=113\r\n\r\n[P2 Keys]\r\nJump   = 273\r\n"
	out := editMugenCfgText(in, []mugenCfgEdit{
		{"P1 Keys", "Jump", "105"},
		{"p1 keys", "a", "106"},
		{"P1 Keys", "Start", "13"},   // missing key: added to the section
		{"P1 Joystick", "Jump", "1"}, // missing section: appended
	})
	want := "; header comment\r\n[Input]\r\nP1.UseKeyboard = 1\r\n\r\n[P1 Keys]\r\nJump   = 105 ; up\r\nCrouch = 115\r\n  A=106\r\nStart = 13\r\n\r\n[P2 Keys]\r\nJump   = 273\r\n\r\n[P1 Joystick]\r\nJump = 1"
	if out != want {
		t.Fatalf("got:\n%q\nwant:\n%q", out, want)
	}
	// No edits: identical text.
	if editMugenCfgText(in, nil) != in {
		t.Fatal("text changed without edits")
	}
}

func TestEditMugenCfgRealFileRoundTrip(t *testing.T) {
	const path = "D:/Jump Force Mugen/data/mugen.cfg"
	orig, err := os.ReadFile(path)
	if err != nil {
		t.Skip("Jump Force MUGEN not present")
	}
	// Setting every P1 key to its current value must not change a single byte.
	mc, err := loadMugenCfg(path)
	if err != nil {
		t.Fatal(err)
	}
	var edits []mugenCfgEdit
	for _, a := range mugenActions {
		if v, ok := mc.str("p1 keys", a); ok {
			edits = append(edits, mugenCfgEdit{"P1 Keys", a, v})
		}
	}
	if got := editMugenCfgText(string(orig), edits); got != string(orig) {
		t.Fatal("no-op edit changed the real mugen.cfg text")
	}
	// A real change touches exactly one line.
	changed := editMugenCfgText(string(orig), []mugenCfgEdit{{"P1 Keys", "Jump", "105"}})
	a, b := strings.Split(string(orig), "\n"), strings.Split(changed, "\n")
	if len(a) != len(b) {
		t.Fatalf("line count %d -> %d", len(a), len(b))
	}
	diff := 0
	for i := range a {
		if a[i] != b[i] {
			diff++
		}
	}
	if diff != 1 {
		t.Fatalf("%d lines changed, want 1", diff)
	}
}

func TestWriteMugenCfgEditsBacksUp(t *testing.T) {
	dir := t.TempDir()
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	os.Chdir(dir)
	os.MkdirAll("data", 0o755)
	cfg := filepath.Join("data", "mugen.cfg")
	os.WriteFile(cfg, []byte("[P1 Keys]\nJump = 119\n"), 0o644)
	if err := writeMugenCfgEdits(cfg, []mugenCfgEdit{{"P1 Keys", "Jump", "105"}}); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(cfg); string(b) != "[P1 Keys]\nJump = 105\n" {
		t.Fatalf("written: %q", b)
	}
	backups, _ := filepath.Glob(filepath.Join("save", "backups", "mugen.cfg.*"))
	if len(backups) != 1 {
		t.Fatalf("backups: %v", backups)
	}
	if b, _ := os.ReadFile(backups[0]); string(b) != "[P1 Keys]\nJump = 119\n" {
		t.Fatalf("backup content: %q", b)
	}
}

func TestKeycodeToSdl12RoundTrip(t *testing.T) {
	for _, sym := range []int{8, 9, 13, 19, 27, 32, 59, 96, 97, 119, 122, 256, 265, 271, 273, 276, 282, 293, 303, 304, 306} {
		k := sdl12KeyToKeycode(sym)
		if got := keycodeToSdl12(k); got != sym {
			t.Errorf("sym %d -> %#x -> %d", sym, k, got)
		}
	}
	if keycodeToSdl12(sdl.K_UNKNOWN) != 0 {
		t.Error("unknown key must map to 0")
	}
}
