package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ikemen-engine/Ikemen-GO/src/mugenjoy"

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

func TestMugenMenuCode(t *testing.T) {
	ds4 := []mugenjoy.DeviceInfo{{Index: 0, VendorID: 0x054C,
		Layout: mugenjoy.Layout{NumAxes: 6, NumHats: 1, NumButtons: 14}}}
	// The user's Jump Force P1 bindings: stick 1-4, buttons 17-24. Options (code 26) is free.
	p := mugenPlayerInput{menu: -1, joy: [...]int{1, 2, 3, 4, 17, 20, 19, 18, 22, 24, 21}}
	if got := mugenMenuCode(p, ds4); got != 26 {
		t.Errorf("DS4 default menu code = %d, want 26 (Options)", got)
	}
	p.menu = 27
	if got := mugenMenuCode(p, ds4); got != 27 {
		t.Errorf("explicit Menu = %d, want 27", got)
	}
	p.menu = -1
	p.joy[10] = 26 // Start already on Options: don't double-bind it
	if got := mugenMenuCode(p, ds4); got != 0 {
		t.Errorf("menu must stay unbound when Options is already used, got %d", got)
	}
	if got := mugenMenuCode(mugenPlayerInput{menu: -1}, ds4); got != 0 {
		t.Errorf("player without joystick bindings got menu code %d", got)
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

// Two DS4s over Bluetooth with DS4Windows: Windows lists four pads, the
// originals (A, C) send nothing, the virtual copies (B, D) send everything.
// Players get the pads that send input, in the order they are pressed, and
// play with player 1's layout moved to their pad.
func TestControllersByPressOrder(t *testing.T) {
	ds4 := mugenjoy.Layout{NumAxes: 6, NumHats: 1, NumButtons: 14}
	devs := []mugenjoy.DeviceInfo{
		{Index: 0, ID: 0, VendorID: 0x054C, ProductID: 0x05C4, Layout: ds4},
		{Index: 1, ID: 1, VendorID: 0x054C, ProductID: 0x05C4, Layout: ds4},
		{Index: 2, ID: 2, VendorID: 0x054C, ProductID: 0x09CC, Layout: ds4},
		{Index: 3, ID: 3, VendorID: 0x054C, ProductID: 0x05C4, Layout: ds4},
	}
	mc := mugenCfg{
		"input":       {"p1.joystick.type": "1", "p2.joystick.type": "0"},
		"p1 joystick": {"jump": "1", "crouch": "2", "left": "3", "right": "4", "a": "17", "b": "20", "c": "19", "x": "18", "y": "22", "z": "24", "start": "21"},
		"p2 joystick": {"jump": "0", "crouch": "0", "left": "0", "right": "0", "a": "0", "b": "0", "c": "0", "x": "0", "y": "0", "z": "0", "start": "0"},
	}
	savedKeys, savedJoy, savedFlags := sys.keyConfig, sys.joystickConfig, sys.cmdFlags
	savedMJ, savedActive, savedEdits, savedPads := mugenJoy, mugenInputActive, omugenPendingEdits, mugenPads
	defer func() {
		sys.keyConfig, sys.joystickConfig, sys.cmdFlags = savedKeys, savedJoy, savedFlags
		mugenJoy, mugenInputActive, omugenPendingEdits, mugenPads = savedMJ, savedActive, savedEdits, savedPads
	}()
	sys.keyConfig = make([]KeyConfig, 4)
	sys.joystickConfig = make([]KeyConfig, 4)
	sys.cmdFlags = map[string]string{}
	omugenPendingEdits = nil
	mugenJoy = mugenjoy.NewFakeManager(devs)
	press := func(id int) {
		mugenJoy.SetFakeInput(id, mugenjoy.Snapshot{Buttons: 1})
		mugenJoy.AdvanceFakeClock(20 * time.Millisecond)
		mugenJoy.Poll()
		mugenJoy.SetFakeInput(id, mugenjoy.Snapshot{})
		mugenJoy.AdvanceFakeClock(200 * time.Millisecond)
		mugenJoy.Poll()
	}
	applyMugenInputConfig(mc)
	mugenJoy.AdvanceFakeClock(20 * time.Millisecond)
	mugenJoy.Poll() // resting position
	mugenPads.joined = nil

	if codes := mugenPlayerCodes(0); codes != nil {
		t.Fatalf("no controller has sent input yet, P1 codes = %v", codes)
	}
	press(1) // the first DS4 (its virtual copy B)
	press(3) // the second DS4 (D)

	p1, p2 := mugenPlayerCodes(0), mugenPlayerCodes(1)
	if len(p1) < 14 || p1[0] != 32+1 || p1[4] != 32+17 || p1[ikemenMenuSlot] != 32+26 {
		t.Errorf("P1 codes = %v, want player 1's layout on B (jump 33, a 49, menu 58)", p1)
	}
	if len(p2) < 14 || p2[0] != 96+1 || p2[10] != 96+21 {
		t.Errorf("P2 codes = %v, want player 1's layout on D", p2)
	}
	if mugenPlayerCodes(2) != nil || mugenPlayerCodes(3) != nil {
		t.Errorf("P3/P4 have no controller left")
	}
	if text, v := omugenBinding(1, "Joystick", "a"); text != "B-17" || v != 17 {
		t.Errorf("P1 'a' = %q/%d, want B-17 shown, 17 kept as the binding", text, v)
	}
	if text, v := omugenBinding(2, "Joystick", "a"); text != "D-17" || v != 0 {
		t.Errorf("P2 'a' = %q/%d, want D-17 with no binding of its own", text, v)
	}
	if len(mugenPads.joined) != 2 || mugenPads.joined[0] != (mugenJoin{0, 1}) || mugenPads.joined[1] != (mugenJoin{1, 3}) {
		t.Errorf("joins = %+v, want P1<-B then P2<-D", mugenPads.joined)
	}

	// One P2 button changed: the rest of the layout becomes P2's own (on D)
	// and P2's joystick is turned on in mugen.cfg.
	if !omugenSetBinding(2, "Joystick", "b", strconv.Itoa(96+22)) {
		t.Fatal("omugenSetBinding failed")
	}
	if got := mugenPlayerCodes(1); len(got) < 14 || got[0] != 97 || got[5] != 118 {
		t.Errorf("P2 codes after one change = %v, want D's layout with b = 118", got)
	}
	typeOn := false
	for _, e := range omugenPendingEdits {
		typeOn = typeOn || (strings.EqualFold(e.key, "P2.Joystick.type") && e.value == "1")
	}
	if !typeOn {
		t.Errorf("P2.Joystick.type = 1 not queued: %+v", omugenPendingEdits)
	}

	// Reassign: P2 keeps the pad its own codes name (D) even when D is
	// pressed first; P1 takes the next one.
	mugenResetPads()
	press(3)
	press(1)
	if pad := mugenPlayerPad(1); pad != 3 {
		t.Errorf("P2 pad after reassign = %d, want D (its own codes)", pad)
	}
	if pad := mugenPlayerPad(0); pad != 1 {
		t.Errorf("P1 pad after reassign = %d, want B", pad)
	}
}
