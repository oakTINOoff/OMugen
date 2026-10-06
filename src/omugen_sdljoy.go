package main

// Controllers for MUGEN-style input, read through SDL.
//
// MUGEN reads joysticks through the WinMM API, and mugen.cfg codes follow
// WinMM's numbering. WinMM misses controllers that modern drivers handle: a
// DualShock 4 on Bluetooth goes silent there once DS4Windows or Steam has
// switched it to its extended mode, and virtual pads may not appear at all.
// SDL reads them all (the engine already opens every game controller), so
// OMugen takes the controllers from SDL and lays them out the way WinMM
// numbers them: PlayStation pads as a DualShock 4 (6 axes, 1 hat, 14 buttons,
// A-17 is Square), the others as an Xbox pad (5 axes, 1 hat, 10 buttons).
// OMUGEN_JOY=winmm reads WinMM directly, as MUGEN does.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/oakTINOoff/OMugen/src/mugenjoy"
	"github.com/veandco/go-sdl2/sdl"
)

var (
	mugenDS4Layout  = mugenjoy.Layout{NumAxes: 6, NumHats: 1, NumButtons: 14}
	mugenXboxLayout = mugenjoy.Layout{NumAxes: 5, NumHats: 1, NumButtons: 10}
)

// padState is what a game controller reports, in SDL GameController terms
// (sticks -32768..32767, triggers 0..32767).
type padState struct {
	lx, ly, rx, ry, lt, rt                 int16
	a, b, x, y, back, guide, start, ls, rs bool
	lb, rb, up, down, left, right          bool
}

// mugenPadSnapshot lays a controller's state out the way WinMM reports that
// kind of controller (ps: a DualShock 4, else an Xbox pad).
func mugenPadSnapshot(ps bool, s padState) mugenjoy.Snapshot {
	var snap mugenjoy.Snapshot
	bit := func(i int, on bool) {
		if on {
			snap.Buttons |= 1 << uint(i)
		}
	}
	// WinMM reports a trigger as a whole axis: released is one end.
	trigger := func(v int16) int32 { return int32(v)*2 - 32767 }
	if ps {
		// X Y Z R U V = left stick, right stick, L2, R2; buttons in
		// DirectInput order.
		snap.Axes = [6]int32{int32(s.lx), int32(s.ly), int32(s.rx), int32(s.ry), trigger(s.lt), trigger(s.rt)}
		for i, on := range []bool{s.x, s.a, s.b, s.y, s.lb, s.rb, s.lt > 8192, s.rt > 8192,
			s.back, s.start, s.ls, s.rs, s.guide} { // Square Cross Circle Triangle L1 R1 L2 R2 Share Options L3 R3 PS
			bit(i, on)
		}
	} else {
		// X Y = left stick, Z = both triggers, R U = right stick.
		snap.Axes = [6]int32{int32(s.lx), int32(s.ly), int32(s.lt) - int32(s.rt), int32(s.rx), int32(s.ry), 0}
		for i, on := range []bool{s.a, s.b, s.x, s.y, s.lb, s.rb, s.back, s.start, s.ls, s.rs} {
			bit(i, on)
		}
	}
	for _, h := range []struct {
		on  bool
		bit uint8
	}{{s.up, mugenjoy.HatUp}, {s.right, mugenjoy.HatRight}, {s.down, mugenjoy.HatDown}, {s.left, mugenjoy.HatLeft}} {
		if h.on {
			snap.Hat |= h.bit
		}
	}
	return snap
}

// sdlJoyBackend gives mugenjoy the game controllers the engine has open.
type sdlJoyBackend struct{}

func sdlPadIsPS(c *sdl.GameController) bool {
	switch c.Type() {
	case sdl.CONTROLLER_TYPE_PS3, sdl.CONTROLLER_TYPE_PS4, sdl.CONTROLLER_TYPE_PS5:
		return true
	}
	return c.Vendor() == 0x054C
}

func (sdlJoyBackend) Enumerate() []mugenjoy.DeviceInfo {
	var out []mugenjoy.DeviceInfo
	for _, c := range input.controllers {
		if c == nil || !c.Attached() || c.Joystick() == nil {
			continue
		}
		layout := mugenXboxLayout
		if sdlPadIsPS(c) {
			layout = mugenDS4Layout
		}
		out = append(out, mugenjoy.DeviceInfo{Index: len(out), ID: int(c.Joystick().InstanceID()), Name: c.Name(),
			VendorID: uint16(c.Vendor()), ProductID: uint16(c.Product()), Layout: layout})
	}
	return out
}

func (sdlJoyBackend) Read(d mugenjoy.DeviceInfo) (mugenjoy.Snapshot, bool) {
	for _, c := range input.controllers {
		if c == nil || !c.Attached() || c.Joystick() == nil || int(c.Joystick().InstanceID()) != d.ID {
			continue
		}
		btn := func(b sdl.GameControllerButton) bool { return c.Button(b) != 0 }
		s := padState{
			lx: c.Axis(sdl.CONTROLLER_AXIS_LEFTX), ly: c.Axis(sdl.CONTROLLER_AXIS_LEFTY),
			rx: c.Axis(sdl.CONTROLLER_AXIS_RIGHTX), ry: c.Axis(sdl.CONTROLLER_AXIS_RIGHTY),
			lt: c.Axis(sdl.CONTROLLER_AXIS_TRIGGERLEFT), rt: c.Axis(sdl.CONTROLLER_AXIS_TRIGGERRIGHT),
			a: btn(sdl.CONTROLLER_BUTTON_A), b: btn(sdl.CONTROLLER_BUTTON_B),
			x: btn(sdl.CONTROLLER_BUTTON_X), y: btn(sdl.CONTROLLER_BUTTON_Y),
			back: btn(sdl.CONTROLLER_BUTTON_BACK), guide: btn(sdl.CONTROLLER_BUTTON_GUIDE),
			start: btn(sdl.CONTROLLER_BUTTON_START),
			ls:    btn(sdl.CONTROLLER_BUTTON_LEFTSTICK), rs: btn(sdl.CONTROLLER_BUTTON_RIGHTSTICK),
			lb: btn(sdl.CONTROLLER_BUTTON_LEFTSHOULDER), rb: btn(sdl.CONTROLLER_BUTTON_RIGHTSHOULDER),
			up: btn(sdl.CONTROLLER_BUTTON_DPAD_UP), down: btn(sdl.CONTROLLER_BUTTON_DPAD_DOWN),
			left: btn(sdl.CONTROLLER_BUTTON_DPAD_LEFT), right: btn(sdl.CONTROLLER_BUTTON_DPAD_RIGHT),
		}
		return mugenPadSnapshot(d.Layout == mugenDS4Layout, s), true
	}
	return mugenjoy.Snapshot{}, false
}

// mugenJoyBackendName says where controllers come from, for the report.
var mugenJoyBackendName = "SDL"

// mugenWriteControllerReport writes save/logs/controllers.txt: the
// controllers OMugen sees, which ones send input and who plays with which,
// for troubleshooting. It is rewritten whenever that changes.
func mugenWriteControllerReport() {
	if mugenJoy == nil {
		return
	}
	var b strings.Builder
	fmt.Fprintf(&b, "OMugen controllers (%s) - %s\n\n", mugenJoyBackendName, time.Now().Format("2006-01-02 15:04:05"))
	live := map[int]bool{}
	for _, i := range mugenJoy.Live() {
		live[i] = true
	}
	devs := mugenJoy.Devices()
	if len(devs) == 0 {
		b.WriteString("No controllers.\n")
	}
	for _, d := range devs {
		state := "no input yet"
		if live[d.Index] {
			state = "sends input"
		}
		fmt.Fprintf(&b, "%c  %-32s VID %04X PID %04X  %d axes %d hat %d buttons  %s\n", 'A'+d.Index, d.Name,
			d.VendorID, d.ProductID, d.Layout.NumAxes, d.Layout.NumHats, d.Layout.NumButtons, state)
	}
	b.WriteString("\n")
	for p := 0; p < mugenMaxPlayers && p < len(sys.joystickConfig); p++ {
		switch {
		case !mugenWantsPad(p):
			fmt.Fprintf(&b, "P%d  no joystick layout\n", p+1)
		case mugenPads.idx[p] >= 0:
			fmt.Fprintf(&b, "P%d  controller %c\n", p+1, 'A'+mugenPads.idx[p])
		default:
			fmt.Fprintf(&b, "P%d  waiting for a controller to send input\n", p+1)
		}
	}
	// Only in a game folder (the game creates save/; tests run elsewhere).
	dir := filepath.Join("save", "logs")
	if _, err := os.Stat("save"); err == nil && os.MkdirAll(dir, 0o755) == nil {
		_ = os.WriteFile(filepath.Join(dir, "controllers.txt"), []byte(b.String()), 0o644)
	}
}

// mugenPadColors are the players' colors (those of the controller indicator,
// omugen_hud.lua), shown on controllers with a light bar (DualShock 4,
// DualSense).
var mugenPadColors = [mugenMaxPlayers][3]uint8{{232, 52, 63}, {47, 123, 234}, {45, 190, 108}, {242, 182, 48}}

var mugenPadLightsAt time.Time

// mugenSetPadLights colors each controller's light bar with its player's
// color, dim white while it has no player. SDL sets them by connection order
// when a controller opens, so they are set again now and then.
func mugenSetPadLights() {
	if mugenJoyBackendName != "SDL" {
		return
	}
	mugenPadLightsAt = time.Now()
	owner := map[int]int{} // device ID -> player
	for p, id := range mugenPads.id {
		if id >= 0 {
			owner[id] = p
		}
	}
	for _, c := range input.controllers {
		if c == nil || !c.Attached() || c.Joystick() == nil || !c.HasLED() {
			continue
		}
		col := [3]uint8{40, 40, 40}
		if p, ok := owner[int(c.Joystick().InstanceID())]; ok {
			col = mugenPadColors[p]
		}
		// go-sdl2's SetLED hands its last two arguments to SDL swapped.
		_ = c.SetLED(col[0], col[2], col[1])
	}
}
