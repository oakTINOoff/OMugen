package mugenjoy

import "testing"

func TestLabelMatchesMugenConfigScreen(t *testing.T) {
	cases := map[int]string{0: "Not used", 1: "A-01", 17: "A-17", 41: "B-09", 54: "B-22", 100: "D-04"}
	for code, want := range cases {
		if got := Label(code); got != want {
			t.Errorf("Label(%d) = %q, want %q", code, got, want)
		}
	}
}

// MUGEN 1.1's shipped mugen.cfg defaults (P1 Jump=9, A=13; P2 Jump=41) fit a
// device with 4 axes and 1 hat: hat up at offset 9, button 0 at offset 13.
func TestLayoutMatchesShippedDefaults(t *testing.T) {
	l := Layout{NumAxes: 4, NumHats: 1, NumButtons: 12}
	if got := l.HatBase() + 1; got != 9 {
		t.Errorf("hat up offset = %d, want 9", got)
	}
	if got := l.ButtonBase() + 0 + 1; got != 13 {
		t.Errorf("button 0 offset = %d, want 13", got)
	}
	if got := 1*CodesPerJoystick + l.HatBase() + 1; got != 41 {
		t.Errorf("P2 hat up code = %d, want 41", got)
	}
}

func TestAxisCodes(t *testing.T) {
	d := device{layout: Layout{NumAxes: 2}}
	var st State
	d.apply(0, Snapshot{Axes: [maxAxes]int32{0, -32768}}, &st) // stick up
	if !st[1] || st[2] || st[3] || st[4] {
		t.Fatalf("stick up: want only code 1, got %v", st[:5])
	}
	d.apply(0, Snapshot{Axes: [maxAxes]int32{32767, 0}}, &st) // stick right
	if st[1] || st[2] || st[3] || !st[4] {
		t.Fatalf("stick right: want only code 4, got %v", st[:5])
	}
	d.apply(0, Snapshot{Axes: [maxAxes]int32{-20000, 20000}}, &st) // down-left
	if st[1] || !st[2] || !st[3] || st[4] {
		t.Fatalf("down-left: want codes 2 and 3, got %v", st[:5])
	}
}

func TestAxisDeadZoneIsExclusive(t *testing.T) {
	d := device{layout: Layout{NumAxes: 2}}
	var st State
	d.apply(0, Snapshot{Axes: [maxAxes]int32{16000, 0}}, &st)
	if st[4] {
		t.Fatal("16000 must be inside the dead zone")
	}
	d.apply(0, Snapshot{Axes: [maxAxes]int32{16500, 0}}, &st)
	if !st[4] {
		t.Fatal("16500 must press right")
	}
}

func TestSmallAxisChangesAreIgnoredLikeSDL(t *testing.T) {
	d := device{layout: Layout{NumAxes: 2}}
	var st State
	d.apply(0, Snapshot{Axes: [maxAxes]int32{16100, 0}}, &st)
	if !st[4] {
		t.Fatal("16100 should press right")
	}
	// A 200-unit wobble is below SDL 1.2's 255 threshold: no new event, stays pressed.
	d.apply(0, Snapshot{Axes: [maxAxes]int32{15900, 0}}, &st)
	if !st[4] {
		t.Fatal("change below SDL threshold must not release")
	}
}

func TestHatAndButtons(t *testing.T) {
	// 5 axes, 1 hat: hatBase 10, buttonBase 14.
	d := device{layout: Layout{NumAxes: 5, NumHats: 1, NumButtons: 10}}
	var st State
	d.apply(1, Snapshot{Hat: HatUp | HatRight, Buttons: 1<<0 | 1<<2}, &st)
	base := 32
	if !st[base+11] || !st[base+14] || st[base+12] || st[base+13] {
		t.Fatalf("hat up-right wrong: %v", st[base+11:base+15])
	}
	if !st[base+15] || st[base+16] || !st[base+17] {
		t.Fatalf("buttons 0 and 2 should be codes 47 and 49")
	}
	d.apply(1, Snapshot{}, &st)
	for c := base; c < base+CodesPerJoystick; c++ {
		if st[c] {
			t.Fatalf("code %d still held after release", c)
		}
	}
}

func TestButtonsPastSlot31AreIgnored(t *testing.T) {
	// 6 axes, 1 hat: buttonBase 16, so buttons 15+ would reach offset 32.
	d := device{layout: Layout{NumAxes: 6, NumHats: 1, NumButtons: 20}}
	var st State
	d.apply(0, Snapshot{Buttons: 1 << 15}, &st)
	for c := range st {
		if st[c] {
			t.Fatalf("button 15 must be ignored, but code %d is held", c)
		}
	}
	d.apply(0, Snapshot{Buttons: 1 << 14}, &st)
	if !st[31] {
		t.Fatal("button 14 should map to code 31")
	}
}

func TestMenuButtonCodes(t *testing.T) {
	ds4 := DeviceInfo{Index: 0, VendorID: 0x054C, Layout: Layout{NumAxes: 6, NumHats: 1, NumButtons: 14}}
	b, ok := MenuButton(ds4)
	if !ok || ButtonCode(ds4, b) != 26 {
		t.Errorf("DS4 Options: button %d ok=%v code %d, want code 26", b, ok, ButtonCode(ds4, b))
	}
	xbox := DeviceInfo{Index: 1, VendorID: 0x045E, Layout: Layout{NumAxes: 5, NumHats: 1, NumButtons: 10}}
	b, ok = MenuButton(xbox)
	if !ok || ButtonCode(xbox, b) != 32+14+7+1 {
		t.Errorf("Xbox Start: button %d ok=%v code %d", b, ok, ButtonCode(xbox, b))
	}
	if _, ok := MenuButton(DeviceInfo{VendorID: 0x1234}); ok {
		t.Error("unknown vendor should have no menu button")
	}
	big := DeviceInfo{Layout: Layout{NumAxes: 6, NumHats: 1, NumButtons: 20}}
	if ButtonCode(big, 15) != 0 || ButtonCode(big, 14) != 31 {
		t.Error("buttons past offset 31 must have no code")
	}
}

func TestDescribe(t *testing.T) {
	l := Layout{NumAxes: 6, NumHats: 1, NumButtons: 14}
	cases := map[int]string{
		1:  "joystick 1 axis 1-",
		4:  "joystick 1 axis 0+",
		13: "joystick 1 hat up",
		17: "joystick 1 button 0",
		24: "joystick 1 button 7",
	}
	for code, want := range cases {
		if got := Describe(code, l); got != want {
			t.Errorf("Describe(%d) = %q, want %q", code, got, want)
		}
	}
}
