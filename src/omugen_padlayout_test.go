package main

import (
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

// A game set up on a 4-axis pad (D-pad at codes 9-12), played on a
// DualShock 4: there 9-12 are the trigger axes, and released triggers would
// hold Jump and Left for good. The player gets the standard layout instead.
func TestMugenStandardLayoutForForeignCodes(t *testing.T) {
	devs := []mugenjoy.DeviceInfo{{Index: 0, ID: 0, Name: "PS4 Controller", VendorID: 0x054C, ProductID: 0x05C4, Layout: mugenDS4Layout}}
	foreign := mugenCfg{
		"input":       {"p1.joystick.type": "1", "p2.joystick.type": "0"},
		"p1 joystick": {"jump": "9", "crouch": "10", "left": "11", "right": "12", "a": "13", "b": "14", "c": "20", "x": "15", "y": "16", "z": "18", "start": "24"},
	}
	savedKeys, savedJoy, savedFlags := sys.keyConfig, sys.joystickConfig, sys.cmdFlags
	savedMJ, savedActive, savedPads, savedStd := mugenJoy, mugenInputActive, mugenPads, mugenStandardFor
	defer func() {
		sys.keyConfig, sys.joystickConfig, sys.cmdFlags = savedKeys, savedJoy, savedFlags
		mugenJoy, mugenInputActive, mugenPads, mugenStandardFor = savedMJ, savedActive, savedPads, savedStd
	}()
	sys.keyConfig = make([]KeyConfig, 4)
	sys.joystickConfig = make([]KeyConfig, 4)
	sys.cmdFlags = map[string]string{}
	mugenStandardFor = map[int]int{}

	// A DualShock 4 at rest, as WinMM reports it: both triggers at one end.
	rest := mugenjoy.Snapshot{Axes: [6]int32{0, 0, 0, 0, -32767, -32767}}
	with := func(f func(s *mugenjoy.Snapshot)) mugenjoy.Snapshot { s := rest; f(&s); return s }
	feed := func(s mugenjoy.Snapshot) [14]bool {
		mugenJoy.SetFakeInput(0, s)
		mugenJoy.AdvanceFakeClock(20 * time.Millisecond)
		return mugenJoystickState(mugenPlayerCodes(0))
	}
	start := func(mc mugenCfg) {
		mugenJoy = mugenjoy.NewFakeManager(devs)
		mugenJoy.SetFakeInput(0, rest)
		applyMugenInputConfig(mc)
		mugenJoy.AdvanceFakeClock(20 * time.Millisecond)
		mugenJoy.Poll()
		feed(with(func(s *mugenjoy.Snapshot) { s.Buttons = 1 << 1 })) // Cross: the pad joins as P1
		feed(rest)
	}

	start(foreign)
	codes := mugenPlayerCodes(0)
	if len(codes) != mugenStandardSlots || codes[0] != 13 || codes[4] != 18 || codes[10] != 26 || codes[ikemenMenuSlot] != 25 {
		t.Fatalf("P1 codes = %v, want the standard layout (D-pad 13-16, a = Cross 18, start = Options 26, menu = Share 25)", codes)
	}
	if !mugenUsesStandardLayout(0) {
		t.Errorf("the standard layout isn't recorded")
	}
	if st := feed(rest); st != ([14]bool{}) {
		t.Errorf("at rest the player holds %v, want nothing", st)
	}
	if st := feed(with(func(s *mugenjoy.Snapshot) { s.Hat = mugenjoy.HatUp })); !st[0] {
		t.Errorf("D-pad up isn't up: %v", st)
	}
	if st := feed(with(func(s *mugenjoy.Snapshot) { s.Axes[0] = -30000 })); !st[2] {
		t.Errorf("left stick left isn't left: %v", st)
	}
	if st := feed(with(func(s *mugenjoy.Snapshot) { s.Buttons = 1 << 1 })); !st[4] {
		t.Errorf("Cross isn't a: %v", st)
	}

	// A layout that suits the pad (left stick, buttons) is used as it is.
	start(mugenCfg{
		"input":       {"p1.joystick.type": "1", "p2.joystick.type": "0"},
		"p1 joystick": {"jump": "1", "crouch": "2", "left": "3", "right": "4", "a": "17", "b": "20", "c": "19", "x": "18", "y": "22", "z": "24", "start": "21"},
	})
	if codes := mugenPlayerCodes(0); len(codes) < 14 || codes[0] != 1 || codes[4] != 17 || mugenUsesStandardLayout(0) {
		t.Errorf("P1 codes = %v (standard %v), want mugen.cfg's own", codes, mugenUsesStandardLayout(0))
	}
}
