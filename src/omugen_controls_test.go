package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/veandco/go-sdl2/sdl"
)

// Changing bindings updates the live key configs and saves only those values.
func TestOmugenControlsSetAndSave(t *testing.T) {
	dir := t.TempDir()
	wd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(wd)
	os.MkdirAll("data", 0o755)
	cfgText := "[Input]\nP1.Joystick.type = 1\nP2.Joystick.type = 0\n[P1 Keys]\nJump = 119 ; w\nA = 113\n[P1 Joystick]\nJump = 1\nA = 17\n"
	os.WriteFile(mugenCfgPath, []byte(cfgText), 0o644)

	savedKeys, savedJoy, savedFlags, savedActive, savedEdits := sys.keyConfig, sys.joystickConfig, sys.cmdFlags, mugenInputActive, omugenPendingEdits
	defer func() {
		sys.keyConfig, sys.joystickConfig, sys.cmdFlags, mugenInputActive, omugenPendingEdits = savedKeys, savedJoy, savedFlags, savedActive, savedEdits
	}()
	sys.keyConfig, sys.joystickConfig = make([]KeyConfig, 2), make([]KeyConfig, 2)
	sys.cmdFlags = map[string]string{}
	if KeyToStringLUT == nil {
		initLUTs() // key-name tables are filled at game start
	}
	omugenPendingEdits = nil
	mc, _ := loadMugenCfg(mugenCfgPath)
	applyMugenInputConfig(mc)

	if text, v := omugenBinding(1, "Keys", "up"); text != "w" || v != 119 {
		t.Fatalf("P1 up = %q/%d", text, v)
	}
	if text, _ := omugenBinding(1, "Joystick", "a"); text != "A-17" {
		t.Fatalf("P1 joystick a = %q", text)
	}
	if !omugenSetBinding(1, "Keys", "a", "j") || sys.keyConfig[0].bA != int(sdl.K_j) {
		t.Fatal("keyboard A not applied")
	}
	if !omugenSetBinding(1, "Joystick", "a", "18") || sys.joystickConfig[0].mugenCodes[4] != 18 {
		t.Fatal("joystick A not applied")
	}
	if !omugenSetBinding(1, "Joystick", "menu", "26") || sys.joystickConfig[0].mugenCodes[ikemenMenuSlot] != 26 {
		t.Fatal("menu button not applied")
	}
	if omugenSetBinding(1, "Keys", "menu", "ESCAPE") {
		t.Fatal("MUGEN has no keyboard menu binding")
	}
	omugenSetBinding(1, "Keys", "a", "k") // last value wins
	edits := omugenPendingEdits
	if err := writeMugenCfgEdits(mugenCfgPath, edits); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(mugenCfgPath)
	want := "[Input]\nP1.Joystick.type = 1\nP2.Joystick.type = 0\n[P1 Keys]\nJump = 119 ; w\nA = 107\n[P1 Joystick]\nJump = 1\nA = 18\nMenu = 26\n"
	if string(got) != want {
		t.Fatalf("saved:\n%s\nwant:\n%s", got, want)
	}
	if b, _ := filepath.Glob("save/backups/mugen.cfg.*"); len(b) != 1 {
		t.Fatalf("expected one backup, got %v", b)
	}
}
