package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/oakTINOoff/OMugen/src/mugenjoy"
)

func TestMugenDirectional(t *testing.T) {
	// DualShock 4: sticks 1-8, triggers 9-12, D-pad 13-16, buttons 17-30.
	for off := 1; off <= 30; off++ {
		dir, known := mugenDirectional(mugenDS4Layout, off)
		if want := off <= 8 || off >= 13 && off <= 16; !known || dir != want {
			t.Errorf("DS4 offset %d: directional %v (known %v), want %v", off, dir, known, want)
		}
	}
	// Xbox: left stick 1-4, triggers 5-6, right stick 7-10, D-pad 11-14, buttons 15-24.
	for off := 1; off <= 24; off++ {
		dir, known := mugenDirectional(mugenXboxLayout, off)
		if want := off <= 4 || off >= 7 && off <= 14; !known || dir != want {
			t.Errorf("Xbox offset %d: directional %v (known %v), want %v", off, dir, known, want)
		}
	}
	if _, known := mugenDirectional(mugenjoy.Layout{NumAxes: 4, NumHats: 1, NumButtons: 12}, 9); known {
		t.Errorf("a layout OMugen doesn't make was judged")
	}
}

// padLayoutTest drives player 1 with one fake DualShock 4.
type padLayoutTest struct {
	t    *testing.T
	rest mugenjoy.Snapshot
}

func newPadLayoutTest(t *testing.T) *padLayoutTest {
	savedKeys, savedJoy, savedFlags := sys.keyConfig, sys.joystickConfig, sys.cmdFlags
	savedMJ, savedActive, savedPads, savedRepl, savedYours := mugenJoy, mugenInputActive, mugenPads, mugenReplaced, mugenYourLayouts
	t.Cleanup(func() {
		sys.keyConfig, sys.joystickConfig, sys.cmdFlags = savedKeys, savedJoy, savedFlags
		mugenJoy, mugenInputActive, mugenPads, mugenReplaced, mugenYourLayouts = savedMJ, savedActive, savedPads, savedRepl, savedYours
	})
	sys.keyConfig = make([]KeyConfig, 4)
	sys.joystickConfig = make([]KeyConfig, 4)
	sys.cmdFlags = map[string]string{}
	mugenReplaced = map[int]mugenReplacement{}
	mugenYourLayouts.loaded, mugenYourLayouts.off, mugenYourLayouts.kinds = false, false, nil
	mugenYourLayouts.path = filepath.Join(t.TempDir(), "pad-layouts.ini")
	// A DualShock 4 at rest, as WinMM reports it: both triggers at one end.
	return &padLayoutTest{t: t, rest: mugenjoy.Snapshot{Axes: [6]int32{0, 0, 0, 0, -32767, -32767}}}
}

func (pt *padLayoutTest) with(f func(s *mugenjoy.Snapshot)) mugenjoy.Snapshot {
	s := pt.rest
	f(&s)
	return s
}

func (pt *padLayoutTest) feed(s mugenjoy.Snapshot) [14]bool {
	mugenJoy.SetFakeInput(0, s)
	mugenJoy.AdvanceFakeClock(20 * time.Millisecond)
	return mugenJoystickState(mugenPlayerCodes(0))
}

// start opens a game with these mugen.cfg joystick codes for player 1; the
// pad joins with a press of Cross.
func (pt *padLayoutTest) start(p1 map[string]string) {
	devs := []mugenjoy.DeviceInfo{{Index: 0, ID: 0, Name: "PS4 Controller", VendorID: 0x054C, ProductID: 0x05C4, Layout: mugenDS4Layout}}
	mugenJoy = mugenjoy.NewFakeManager(devs)
	mugenJoy.SetFakeInput(0, pt.rest)
	applyMugenInputConfig(mugenCfg{"input": {"p1.joystick.type": "1", "p2.joystick.type": "0"}, "p1 joystick": p1})
	mugenJoy.AdvanceFakeClock(20 * time.Millisecond)
	mugenJoy.Poll()
	pt.feed(pt.with(func(s *mugenjoy.Snapshot) { s.Buttons = 1 << 1 }))
	pt.feed(pt.rest)
}

// Set up on a 4-axis pad (D-pad at codes 9-12): on a DualShock 4 those are
// the trigger axes, and released triggers would hold Jump and Left for good.
var padLayoutForeign = map[string]string{"jump": "9", "crouch": "10", "left": "11", "right": "12",
	"a": "13", "b": "14", "c": "20", "x": "15", "y": "16", "z": "18", "start": "24"}

// Set up on a DualShock 4: left stick, buttons (the user's Jump Force layout).
var padLayoutOwn = map[string]string{"jump": "1", "crouch": "2", "left": "3", "right": "4",
	"a": "17", "b": "20", "c": "19", "x": "18", "y": "22", "z": "24", "start": "21"}

func TestMugenStandardLayoutForForeignCodes(t *testing.T) {
	pt := newPadLayoutTest(t) // tests run outside a game folder: no layout of yours
	pt.start(padLayoutForeign)
	codes := mugenPlayerCodes(0)
	if len(codes) != mugenStandardSlots || codes[0] != 1 || codes[4] != 17 || codes[7] != 18 || codes[10] != 21 ||
		codes[ikemenMenuSlot] != 26 || codes[ikemenMenuSlot+1] != 13 {
		t.Fatalf("P1 codes = %v, want the standard layout (stick 1-4, a = Square 17, x = Cross 18, start = L1 21, "+
			"menu = Options 26, D-pad 13-16)", codes)
	}
	if replaced, yours := mugenReplacedLayout(0); !replaced || yours {
		t.Errorf("replaced %v yours %v, want the standard layout recorded", replaced, yours)
	}
	if st := pt.feed(pt.rest); st != ([14]bool{}) {
		t.Errorf("at rest the player holds %v, want nothing", st)
	}
	if st := pt.feed(pt.with(func(s *mugenjoy.Snapshot) { s.Hat = mugenjoy.HatUp })); !st[0] {
		t.Errorf("D-pad up isn't up: %v", st)
	}
	if st := pt.feed(pt.with(func(s *mugenjoy.Snapshot) { s.Axes[0] = -30000 })); !st[2] {
		t.Errorf("left stick left isn't left: %v", st)
	}
	if st := pt.feed(pt.with(func(s *mugenjoy.Snapshot) { s.Buttons = 1 << 0 })); !st[4] {
		t.Errorf("Square isn't a: %v", st)
	}
	if st := pt.feed(pt.with(func(s *mugenjoy.Snapshot) { s.Buttons = 1 << 1 })); !st[7] {
		t.Errorf("Cross isn't x: %v", st)
	}

	// A layout that suits the pad is used as it is.
	pt.start(padLayoutOwn)
	if codes := mugenPlayerCodes(0); len(codes) < 14 || codes[0] != 1 || codes[4] != 17 {
		t.Errorf("P1 codes = %v, want mugen.cfg's own", codes)
	}
	if replaced, _ := mugenReplacedLayout(0); replaced {
		t.Errorf("a suiting layout was replaced")
	}
}

// The layout a player set their pad up with in one game follows them to a
// game whose own layout doesn't fit that pad.
func TestMugenYourLayoutFollows(t *testing.T) {
	dir := t.TempDir()
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	os.Chdir(dir)
	os.MkdirAll("save", 0o755) // a game folder
	pt := newPadLayoutTest(t)

	pt.start(padLayoutOwn) // the first game: its layout suits the pad and is kept
	mugenPlayerCodes(0)
	b, err := os.ReadFile(mugenYourLayouts.path)
	if err != nil {
		t.Fatalf("layout not kept: %v", err)
	}
	if s := string(b); !strings.Contains(s, "[PlayStation]") || !strings.Contains(s, "jump = 1\n") || !strings.Contains(s, "start = 21\n") {
		t.Errorf("pad-layouts.ini =\n%s", s)
	}

	// Next start (read back from the file), another game set up on another pad.
	mugenYourLayouts.loaded, mugenYourLayouts.kinds = false, nil
	pt.start(padLayoutForeign)
	codes := mugenPlayerCodes(0)
	if len(codes) < 14 || codes[0] != 1 || codes[4] != 17 || codes[10] != 21 {
		t.Fatalf("P1 codes = %v, want the kept layout (jump 1, a 17, start 21)", codes)
	}
	if replaced, yours := mugenReplacedLayout(0); !replaced || !yours {
		t.Errorf("replaced %v yours %v, want your layout", replaced, yours)
	}
	if st := pt.feed(pt.rest); st != ([14]bool{}) {
		t.Errorf("at rest the player holds %v, want nothing", st)
	}
	if st := pt.feed(pt.with(func(s *mugenjoy.Snapshot) { s.Buttons = 1 << 0 })); !st[4] {
		t.Errorf("Square isn't a: %v", st)
	}

	// A suiting layout from another game doesn't replace the kept one...
	other := map[string]string{"jump": "13", "crouch": "14", "left": "15", "right": "16",
		"a": "18", "b": "19", "c": "24", "x": "17", "y": "20", "z": "22", "start": "26"}
	pt.start(other)
	mugenPlayerCodes(0)
	if b2, _ := os.ReadFile(mugenYourLayouts.path); string(b2) != string(b) {
		t.Errorf("another game's layout replaced yours:\n%s", b2)
	}
	// ...but one saved in the Controls menu does.
	d, _ := mugenDeviceAt(0)
	mugenRememberLayout(mugenPlayerCodes(0), d, true)
	if b3, _ := os.ReadFile(mugenYourLayouts.path); !strings.Contains(string(b3), "jump = 13\n") {
		t.Errorf("the saved layout wasn't kept:\n%s", b3)
	}
}

// A game started before your layout was kept (by another game running at
// the same time) picks it up when controllers are handed out again.
func TestMugenYourLayoutKeptLater(t *testing.T) {
	dir := t.TempDir()
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	os.Chdir(dir)
	os.MkdirAll("save", 0o755)
	pt := newPadLayoutTest(t)

	pt.start(padLayoutForeign)
	if codes := mugenPlayerCodes(0); len(codes) != mugenStandardSlots {
		t.Fatalf("P1 codes = %v, want the standard layout (nothing kept yet)", codes)
	}
	os.WriteFile(mugenYourLayouts.path, []byte("[PlayStation]\njump = 13\ncrouch = 14\nleft = 15\nright = 16\n"+
		"a = 18\nb = 19\nc = 24\nx = 17\ny = 20\nz = 22\nstart = 21\nmenu = 26\n"), 0o644)

	mugenResetPads() // Options > Input > Reassign Controllers
	pt.feed(pt.with(func(s *mugenjoy.Snapshot) { s.Buttons = 1 << 1 }))
	pt.feed(pt.rest)
	if codes := mugenPlayerCodes(0); len(codes) < 14 || codes[0] != 13 || codes[4] != 18 {
		t.Errorf("P1 codes = %v, want the layout kept meanwhile", codes)
	}
}

// With SDL no controller is listed yet when the bindings are read, so the
// pause menu button (the pad's Options) is only added when the pad joins.
// The kept layout has it; one kept without it (older OMugen) gets it too.
func TestMugenKeptLayoutPauseButton(t *testing.T) {
	dir := t.TempDir()
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	os.Chdir(dir)
	os.MkdirAll("save", 0o755)
	pt := newPadLayoutTest(t)
	devs := []mugenjoy.DeviceInfo{{Index: 0, ID: 0, Name: "PS4 Controller", VendorID: 0x054C, ProductID: 0x05C4, Layout: mugenDS4Layout}}

	mugenJoy = mugenjoy.NewFakeManager(nil)
	applyMugenInputConfig(mugenCfg{"input": {"p1.joystick.type": "1", "p2.joystick.type": "0"}, "p1 joystick": padLayoutOwn})
	mugenJoy = mugenjoy.NewFakeManager(devs)
	mugenJoy.SetFakeInput(0, pt.rest)
	mugenJoy.AdvanceFakeClock(20 * time.Millisecond)
	mugenJoy.Poll()
	pt.feed(pt.with(func(s *mugenjoy.Snapshot) { s.Buttons = 1 << 1 }))
	if codes := mugenPlayerCodes(0); len(codes) < 14 || codes[ikemenMenuSlot] != 26 {
		t.Fatalf("P1 codes = %v, want Options (26) as the pause button", codes)
	}
	if b, _ := os.ReadFile(mugenYourLayouts.path); !strings.Contains(string(b), "menu = 26\n") {
		t.Errorf("kept layout without the pause button:\n%s", b)
	}

	os.WriteFile(mugenYourLayouts.path, []byte("[PlayStation]\njump = 1\ncrouch = 2\nleft = 3\nright = 4\n"+
		"a = 17\nb = 20\nc = 19\nx = 18\ny = 22\nz = 24\nstart = 21\nmenu = 0\n"), 0o644)
	mugenYourLayouts.loaded = false
	pt.start(padLayoutForeign)
	if codes := mugenPlayerCodes(0); len(codes) < 14 || codes[0] != 1 || codes[ikemenMenuSlot] != 26 {
		t.Errorf("P1 codes = %v, want your layout with Options (26) as the pause button", codes)
	}
}
