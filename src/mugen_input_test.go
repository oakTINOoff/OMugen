package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/veandco/go-sdl2/sdl"
)

func TestLoadMugenInputConfig(t *testing.T) {
	cfgText := `; comment
[Input]
P1.UseKeyboard = 1
P2.UseKeyboard = 0   ; trailing comment
P1.Joystick.type = 1
P2.Joystick.type = 0

[P1 Keys]
Jump   = 119
Crouch = 115
Left   = 97
Right  = 100
A      = 113
Start  = 96

[P2 Keys]
Jump   = 273

[P1 Joystick]
jump = 1
A    = 17
Start = 21
`
	path := filepath.Join(t.TempDir(), "mugen.cfg")
	if err := os.WriteFile(path, []byte(cfgText), 0o644); err != nil {
		t.Fatal(err)
	}
	mc, err := loadMugenCfg(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg := mugenInput(mc)
	p1, p2 := cfg.players[0], cfg.players[1]
	if !p1.useKeyboard || !p1.useJoystick || p2.useKeyboard || p2.useJoystick {
		t.Errorf("flags wrong: p1 kb=%v joy=%v, p2 kb=%v joy=%v", p1.useKeyboard, p1.useJoystick, p2.useKeyboard, p2.useJoystick)
	}
	if p1.keys[0] != 119 || p1.keys[4] != 113 || p1.keys[10] != 96 {
		t.Errorf("p1 keys wrong: %v", p1.keys)
	}
	// B was not set, so it keeps MUGEN's default (107 = 'k').
	if p1.keys[5] != 107 {
		t.Errorf("p1 B should keep default 107, got %d", p1.keys[5])
	}
	if p1.joy[0] != 1 || p1.joy[4] != 17 || p1.joy[10] != 21 {
		t.Errorf("p1 joystick wrong: %v", p1.joy)
	}
	if p2.keys[0] != 273 {
		t.Errorf("p2 jump wrong: %d", p2.keys[0])
	}
}

func TestSdl12KeyToKeycode(t *testing.T) {
	cases := map[int]sdl.Keycode{
		0:   sdl.K_UNKNOWN,
		13:  sdl.K_RETURN,
		19:  sdl.K_PAUSE,
		27:  sdl.K_ESCAPE,
		59:  sdl.K_SEMICOLON,
		96:  sdl.K_BACKQUOTE,
		119: sdl.K_w,
		256: sdl.K_KP_0,
		260: sdl.K_KP_4,
		271: sdl.K_KP_ENTER,
		273: sdl.K_UP,
		274: sdl.K_DOWN,
		275: sdl.K_RIGHT,
		276: sdl.K_LEFT,
		282: sdl.K_F1,
		293: sdl.K_F12,
		304: sdl.K_LSHIFT,
		306: sdl.K_LCTRL,
	}
	for sym, want := range cases {
		if got := sdl12KeyToKeycode(sym); got != want {
			t.Errorf("sdl12KeyToKeycode(%d) = %#x, want %#x", sym, got, want)
		}
	}
}

// Parses the real Jump Force MUGEN config when it is present on this machine.
func TestLoadRealJumpForceConfig(t *testing.T) {
	const path = "D:/Jump Force Mugen/data/mugen.cfg"
	if _, err := os.Stat(path); err != nil {
		t.Skip("Jump Force MUGEN not present")
	}
	mc, err := loadMugenCfg(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg := mugenInput(mc)
	p1 := cfg.players[0]
	wantJoy := [...]int{1, 2, 3, 4, 17, 20, 19, 18, 22, 24, 21}
	if p1.joy != wantJoy {
		t.Errorf("P1 joystick = %v, want %v", p1.joy, wantJoy)
	}
	wantKeys := [...]int{119, 115, 97, 100, 113, 101, 99, 120, 102, 122, 96}
	if p1.keys != wantKeys {
		t.Errorf("P1 keys = %v, want %v", p1.keys, wantKeys)
	}
	if !cfg.players[1].useKeyboard || cfg.players[1].useJoystick {
		t.Errorf("P2 should be keyboard only")
	}

	var c Config
	c.applyMugenCfg(mc)
	if c.Config.Motif != "data/mugen1/system.def" {
		t.Errorf("motif = %q", c.Config.Motif)
	}
	if c.Video.GameWidth != 1920 || c.Video.GameHeight != 1030 || c.Video.WindowWidth != 1920 || c.Video.WindowHeight != 1030 {
		t.Errorf("video = game %dx%d window %dx%d", c.Video.GameWidth, c.Video.GameHeight, c.Video.WindowWidth, c.Video.WindowHeight)
	}
	if c.Video.Fullscreen {
		t.Errorf("fullscreen should be off")
	}
}

// Regression: both players used to share P2's joystick codes (loop variable aliasing).
func TestApplyMugenInputKeepsPerPlayerCodes(t *testing.T) {
	mc := mugenCfg{
		"input":       {"p1.joystick.type": "1", "p2.joystick.type": "1"},
		"p1 joystick": {"jump": "1", "a": "17"},
		"p2 joystick": {"jump": "41", "a": "0"},
		"p1 keys":     {"jump": "119"},
		"p2 keys":     {"jump": "273"},
	}
	savedKeys, savedJoy, savedFlags := sys.keyConfig, sys.joystickConfig, sys.cmdFlags
	defer func() { sys.keyConfig, sys.joystickConfig, sys.cmdFlags = savedKeys, savedJoy, savedFlags }()
	sys.keyConfig = make([]KeyConfig, 2)
	sys.joystickConfig = make([]KeyConfig, 2)
	sys.cmdFlags = map[string]string{}

	applyMugenInputConfig(mc)

	p1, p2 := sys.joystickConfig[0].mugenCodes, sys.joystickConfig[1].mugenCodes
	if len(p1) == 0 || len(p2) == 0 {
		t.Fatalf("mugenCodes not set: p1=%v p2=%v", p1, p2)
	}
	if p1[0] != 1 || p1[4] != 17 {
		t.Errorf("P1 codes = %v, want jump=1 a=17", p1)
	}
	if p2[0] != 41 || p2[4] != 0 {
		t.Errorf("P2 codes = %v, want jump=41 a=0", p2)
	}
	if sys.keyConfig[0].dU != int(sdl.K_w) || sys.keyConfig[1].dU != int(sdl.K_UP) {
		t.Errorf("keyboard jump: p1=%#x p2=%#x", sys.keyConfig[0].dU, sys.keyConfig[1].dU)
	}
}

func TestMugenCfgParsing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mugen.cfg")
	text := "[Options]\n  motif = \"data/foo/system.def\" ; quoted\n[OPTIONS]\nmotif = ignored\n[Video]\nWidth=640\nHeight = abc\n"
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	mc, err := loadMugenCfg(path)
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := mc.str("options", "motif"); v != "data/foo/system.def" {
		t.Errorf("motif = %q (quotes stripped, first occurrence wins)", v)
	}
	if v, ok := mc.int("video", "width"); !ok || v != 640 {
		t.Errorf("width = %d, %v", v, ok)
	}
	if _, ok := mc.int("video", "height"); ok {
		t.Errorf("non-numeric height must not parse")
	}
}
