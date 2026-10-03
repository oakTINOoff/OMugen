// Package mugenjoy reproduces how MUGEN 1.1 turns joystick input into the
// numeric codes stored in the [P1 Joystick] / [P2 Joystick] sections of
// data/mugen.cfg.
//
// MUGEN 1.1 reads joysticks through SDL 1.2, whose Windows backend is the
// WinMM joystick API (joyGetPosEx). Its SDL event handler (Mugen.exe 1.1b1,
// 0x474790) maps each device's axes, first hat and buttons into a block of 32
// codes per joystick:
//
//	code       = joy*32 + offset          (joy 0..3, 0 = unassigned)
//	axis a     : a' = a with 0 and 1 swapped; negative -> 2a'+1, positive -> 2a'+2
//	             (so axis Y up/down = 1/2, axis X left/right = 3/4)
//	hatBase    = 2 * numAxes
//	hat 0      : up/down/left/right = hatBase+1 .. hatBase+4
//	buttonBase = hatBase + 4*numHats
//	button b   : buttonBase + b + 1, ignored when the offset reaches 32
//
// An axis only counts as pressed beyond +/-16000 of SDL's -32768..32767 range.
package mugenjoy

import "fmt"

const (
	// MaxJoysticks is how many devices MUGEN opens (it clamps SDL_NumJoysticks to 4).
	MaxJoysticks = 4
	// CodesPerJoystick is the size of each device's code block.
	CodesPerJoystick = 32
	// NumCodes is the size of MUGEN's joystick state table (4*32+1).
	NumCodes = MaxJoysticks*CodesPerJoystick + 1

	axisDeadZone = 16000
	// SDL 1.2 only reports an axis change larger than (AXIS_MAX-AXIS_MIN)/256.
	sdlAxisThreshold = (32767 - (-32768)) / 256

	maxAxes    = 6
	maxButtons = 32
)

// SDL 1.2 hat bits.
const (
	HatUp    = 0x01
	HatRight = 0x02
	HatDown  = 0x04
	HatLeft  = 0x08
)

// Layout is SDL 1.2's view of a device, which fixes where its codes land.
type Layout struct {
	NumAxes    int
	NumHats    int
	NumButtons int
}

// HatBase is the offset just before the first hat code.
func (l Layout) HatBase() int { return 2 * l.NumAxes }

// ButtonBase is the offset just before the first button code.
func (l Layout) ButtonBase() int { return l.HatBase() + 4*l.NumHats }

// Snapshot is one poll of a device, already in SDL 1.2 units.
type Snapshot struct {
	Axes    [maxAxes]int32 // SDL axis values (-32768..32767)
	Buttons uint32         // bit b set = button b down
	Hat     uint8          // SDL hat bits for hat 0
}

// device tracks the SDL 1.2 state MUGEN would have seen from its events.
type device struct {
	layout  Layout
	axes    [maxAxes]int32 // last axis value SDL reported
	buttons uint32
	hat     uint8
}

// State is MUGEN's joystick table: State[code] is true while that input is held.
type State [NumCodes]bool

// Pressed reports whether a MUGEN joystick code is held. Out-of-range codes and 0 are never held.
func (s *State) Pressed(code int) bool {
	if code <= 0 || code >= NumCodes {
		return false
	}
	return s[code]
}

// apply feeds a new snapshot for joystick index joy into the state table,
// following SDL 1.2's change detection and MUGEN's event translation.
func (d *device) apply(joy int, snap Snapshot, st *State) {
	base := joy * CodesPerJoystick

	for a := 0; a < d.layout.NumAxes && a < maxAxes; a++ {
		v := snap.Axes[a]
		if change := v - d.axes[a]; change >= -sdlAxisThreshold && change <= sdlAxisThreshold {
			continue // SDL 1.2 would not emit an event for this change
		}
		// SDL stores the reported value as Sint16.
		v16 := int16(v)
		d.axes[a] = int32(v16)
		neg, pos := axisCodes(a)
		st[base+neg] = v16 < -axisDeadZone
		st[base+pos] = v16 > axisDeadZone
	}

	if d.layout.NumHats > 0 && snap.Hat != d.hat {
		d.hat = snap.Hat
		h := base + d.layout.HatBase()
		st[h+1] = snap.Hat&HatUp != 0
		st[h+2] = snap.Hat&HatDown != 0
		st[h+3] = snap.Hat&HatLeft != 0
		st[h+4] = snap.Hat&HatRight != 0
	}

	nb := d.layout.NumButtons
	if nb > maxButtons {
		nb = maxButtons
	}
	for b := 0; b < nb; b++ {
		down := snap.Buttons&(1<<uint(b)) != 0
		if down == (d.buttons&(1<<uint(b)) != 0) {
			continue
		}
		if off := d.layout.ButtonBase() + b + 1; off < CodesPerJoystick {
			st[base+off] = down
		}
	}
	d.buttons = snap.Buttons
}

// axisCodes returns the negative and positive offsets MUGEN uses for SDL axis a.
func axisCodes(a int) (neg, pos int) {
	switch a {
	case 0:
		a = 1
	case 1:
		a = 0
	}
	return 2*a + 1, 2*a + 2
}

// ButtonCode returns the MUGEN code for button b (0-based) of a device,
// or 0 when MUGEN cannot address that button (offset 32 or more).
func ButtonCode(d DeviceInfo, b int) int {
	off := d.Layout.ButtonBase() + b + 1
	if b < 0 || b >= d.Layout.NumButtons || off >= CodesPerJoystick {
		return 0
	}
	return d.Index*CodesPerJoystick + off
}

// MenuButton returns the 0-based button index of a controller's own
// Start/Options-style button for known controller families, as numbered
// by the WinMM joystick API.
func MenuButton(d DeviceInfo) (int, bool) {
	switch d.VendorID {
	case 0x054C: // Sony DualShock 4 / DualSense: Square Cross Circle Triangle L1 R1 L2 R2 Share Options...
		return 9, d.Layout.NumButtons > 9
	case 0x045E: // Microsoft Xbox pads: A B X Y LB RB Back Start...
		return 7, d.Layout.NumButtons > 7
	}
	// Other pads read through SDL are laid out as an Xbox pad (OMugen).
	if d.Layout == (Layout{NumAxes: 5, NumHats: 1, NumButtons: 10}) {
		return 7, true
	}
	return 0, false
}

// Label formats a code the way MUGEN's joystick config screen does ("A-17").
func Label(code int) string {
	if code <= 0 {
		return "Not used"
	}
	return fmt.Sprintf("%c-%02d", 'A'+(code>>5)&3, code&^0x60)
}

// Describe explains what physical input a code refers to on a device with the given layout.
func Describe(code int, l Layout) string {
	if code <= 0 || code >= NumCodes {
		return "not used"
	}
	joy, off := code/CodesPerJoystick, code%CodesPerJoystick
	prefix := fmt.Sprintf("joystick %d ", joy+1)
	switch {
	case off == 0:
		return prefix + "(unused slot)"
	case off <= 2*l.NumAxes:
		a := (off - 1) / 2
		switch a {
		case 0:
			a = 1
		case 1:
			a = 0
		}
		dir := "-"
		if off%2 == 0 {
			dir = "+"
		}
		return fmt.Sprintf("%saxis %d%s", prefix, a, dir)
	case off <= l.ButtonBase():
		return prefix + [...]string{"hat up", "hat down", "hat left", "hat right"}[(off-l.HatBase()-1)%4]
	default:
		return fmt.Sprintf("%sbutton %d", prefix, off-l.ButtonBase()-1)
	}
}
