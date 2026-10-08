package main

// A game's mugen.cfg joystick codes are numbered for the controller they were
// set up on (mugenjoy: axes first, then the hat, then the buttons). On
// another kind of controller the same numbers can mean something else: a
// game set up on a 4-axis pad has its D-pad at codes 9-12, which on a
// DualShock 4 are the L2/R2 trigger axes, and a released trigger reads as
// held, so Jump and Left stay held and the menus scroll by themselves (in
// MUGEN too). When a player's directions don't land on a stick or the D-pad
// of their controller, OMugen gives them a standard layout for it instead.

import "github.com/oakTINOoff/OMugen/src/mugenjoy"

// mugenDirectional reports whether offset off (1-31) of a controller laid
// out as l is a direction: a stick or the D-pad, not a trigger or a button.
// known is false for layouts OMugen doesn't lay out itself.
func mugenDirectional(l mugenjoy.Layout, off int) (dir, known bool) {
	var triggers []int // the trigger axes, in SDL axis numbers
	switch l {
	case mugenDS4Layout:
		triggers = []int{4, 5} // L2, R2
	case mugenXboxLayout:
		triggers = []int{2} // both triggers on one axis
	default:
		return false, false
	}
	switch {
	case off >= 1 && off <= l.HatBase():
		a := (off - 1) / 2
		if a <= 1 {
			a = 1 - a // MUGEN numbers axes 0 and 1 the other way round
		}
		for _, t := range triggers {
			if a == t {
				return false, true
			}
		}
		return true, true
	case off > l.HatBase() && off <= l.ButtonBase():
		return true, true
	}
	return false, true
}

// mugenLayoutFits reports whether the directions of a player's codes (on
// controller d) land on its sticks or D-pad.
func mugenLayoutFits(codes []int, d mugenjoy.DeviceInfo) bool {
	for _, c := range codes[:4] {
		if c <= 0 || c/mugenjoy.CodesPerJoystick != d.Index {
			continue
		}
		if dir, known := mugenDirectional(d.Layout, c%mugenjoy.CodesPerJoystick); known && !dir {
			return false
		}
	}
	return true
}

// mugenStandardSlots is the length of a standard layout: Ikemen's 14 input
// slots, then second codes for the four directions (the left stick).
const mugenStandardSlots = ikemenMenuSlot + 1 + 4

// mugenStandardLayout is OMugen's layout for controller d, the engine's own
// default ([Joystick_P1]): the D-pad and the left stick move; X Y Z are
// Square Triangle R1 and A B C are Cross Circle R2 (Xbox: X Y RB, A B RT);
// Start is Options (Start) and the pause menu Share (Back). nil for layouts
// OMugen doesn't know.
func mugenStandardLayout(d mugenjoy.DeviceInfo) []int {
	// Slots: up down left right a b c x y z start d w menu, stick up down left right.
	var offs [mugenStandardSlots]int
	switch d.Layout {
	case mugenDS4Layout: // hat 13-16, buttons 17-30: Square Cross Circle Triangle L1 R1 L2 R2 Share Options
		offs = [...]int{13, 14, 15, 16, 18, 19, 24, 17, 20, 22, 26, 0, 0, 25, 1, 2, 3, 4}
	case mugenXboxLayout: // triggers axis 5/6 (RT = 5), hat 11-14, buttons 15-24: A B X Y LB RB Back Start
		offs = [...]int{11, 12, 13, 14, 15, 16, 5, 17, 18, 20, 22, 0, 0, 21, 1, 2, 3, 4}
	default:
		return nil
	}
	codes := make([]int, mugenStandardSlots)
	for i, o := range offs {
		if o > 0 {
			codes[i] = d.Index*mugenjoy.CodesPerJoystick + o
		}
	}
	return codes
}

// mugenDeviceAt returns the controller with device index i.
func mugenDeviceAt(i int) (mugenjoy.DeviceInfo, bool) {
	if mugenJoy == nil {
		return mugenjoy.DeviceInfo{}, false
	}
	for _, d := range mugenJoy.Devices() {
		if d.Index == i {
			return d, true
		}
	}
	return mugenjoy.DeviceInfo{}, false
}

// mugenStandardFor records, per player, the controller (device ID) given the
// standard layout, so it is logged once and shown in controllers.txt.
var mugenStandardFor = map[int]int{}

// mugenStandardCodes returns the standard layout for player p on controller
// d when their codes don't suit it, else nil.
func mugenStandardCodes(p int, codes []int, d mugenjoy.DeviceInfo) []int {
	if mugenLayoutFits(codes, d) {
		if _, ok := mugenStandardFor[p]; ok {
			delete(mugenStandardFor, p)
		}
		return nil
	}
	std := mugenStandardLayout(d)
	if std == nil {
		return nil
	}
	if id, ok := mugenStandardFor[p]; !ok || id != d.ID {
		mugenStandardFor[p] = d.ID
		LogMessage("OMugen: P%d's joystick layout in %s was made on another kind of controller "+
			"(its directions are on %s's triggers or buttons): P%d plays with the standard layout",
			p+1, mugenCfgPath, d.Name, p+1)
		mugenWriteControllerReport()
	}
	return std
}

// mugenUsesStandardLayout reports whether player p currently plays with the
// standard layout.
func mugenUsesStandardLayout(p int) bool {
	_, ok := mugenStandardFor[p]
	return ok
}
