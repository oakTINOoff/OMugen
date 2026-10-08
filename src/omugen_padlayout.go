package main

// A game's mugen.cfg joystick codes are numbered for the controller they were
// set up on (mugenjoy: axes first, then the hat, then the buttons). On
// another kind of controller the same numbers can mean something else: a
// game set up on a 4-axis pad has its D-pad at codes 9-12, which on a
// DualShock 4 are the L2/R2 trigger axes, and a released trigger reads as
// held, so Jump and Left stay held and the menus scroll by themselves (in
// MUGEN too). When a player's directions don't land on a stick or the D-pad
// of their controller, OMugen gives them their own layout for that kind of
// controller (below), or a standard one.
//
// Your layout follows you: OMugen keeps one layout per kind of controller in
// %LOCALAPPDATA%\OMugen\pad-layouts.ini, never in a game folder. It is the
// first layout that suits a controller of that kind (the game you set your
// pad up in), then any layout saved in the Controls menu.

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/oakTINOoff/OMugen/src/mugenjoy"
)

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

// ---------------------------------------------------------------- your layout

// mugenPadKind names the kinds of controller OMugen lays out ("" otherwise).
func mugenPadKind(l mugenjoy.Layout) string {
	switch l {
	case mugenDS4Layout:
		return "PlayStation"
	case mugenXboxLayout:
		return "Xbox"
	}
	return ""
}

// mugenLayoutKeys are the pad-layouts.ini keys: MUGEN's actions, then the
// pause menu button. Values are offsets on the controller (1-31, 0 unused).
var mugenLayoutKeys = append(append([]string(nil), mugenActions[:]...), "menu")

var mugenYourLayouts struct {
	loaded bool
	off    bool   // not in a game folder (tests): none kept or read
	path   string // "" = the default, %LOCALAPPDATA%\OMugen\pad-layouts.ini
	kinds  map[string][]int
}

func mugenYourLayoutsPath() string {
	if mugenYourLayouts.path != "" {
		return mugenYourLayouts.path
	}
	return filepath.Join(engineCacheRoot(), "pad-layouts.ini")
}

func mugenLoadYourLayouts() {
	if mugenYourLayouts.loaded {
		return
	}
	mugenYourLayouts.loaded = true
	mugenYourLayouts.kinds = map[string][]int{}
	// Only in a game folder (the game has save/; tests run elsewhere).
	if _, err := os.Stat("save"); err != nil {
		mugenYourLayouts.off = true
		return
	}
	f, err := os.Open(mugenYourLayoutsPath())
	if err != nil {
		return
	}
	defer f.Close()
	kind := ""
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(strings.SplitN(sc.Text(), ";", 2)[0])
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			kind = strings.TrimSpace(line[1 : len(line)-1])
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok || kind == "" {
			continue
		}
		for i, name := range mugenLayoutKeys {
			if strings.EqualFold(strings.TrimSpace(k), name) {
				n, err := strconv.Atoi(strings.TrimSpace(v))
				if err != nil || n < 0 || n >= mugenjoy.CodesPerJoystick {
					break
				}
				if mugenYourLayouts.kinds[kind] == nil {
					mugenYourLayouts.kinds[kind] = make([]int, len(mugenLayoutKeys))
				}
				mugenYourLayouts.kinds[kind][i] = n
			}
		}
	}
}

func mugenSaveYourLayouts() error {
	var b strings.Builder
	b.WriteString("; Your controller layouts, used in games whose own (data/mugen.cfg) doesn't fit\n" +
		"; your controller. Values are MUGEN joystick codes on the controller (1-31).\n")
	for _, kind := range SortedKeys(mugenYourLayouts.kinds) {
		fmt.Fprintf(&b, "\n[%s]\n", kind)
		for i, name := range mugenLayoutKeys {
			fmt.Fprintf(&b, "%s = %d\n", name, mugenYourLayouts.kinds[kind][i])
		}
	}
	path := mugenYourLayoutsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// mugenRememberLayout keeps the codes a player plays with on controller d as
// the layout for its kind of controller: only the first time, unless saved
// (changed in the Controls menu).
func mugenRememberLayout(codes []int, d mugenjoy.DeviceInfo, saved bool) {
	kind := mugenPadKind(d.Layout)
	if kind == "" || len(codes) <= ikemenMenuSlot {
		return
	}
	mugenLoadYourLayouts()
	if _, kept := mugenYourLayouts.kinds[kind]; mugenYourLayouts.off || kept && !saved || !mugenLayoutFits(codes, d) {
		return
	}
	offs := make([]int, len(mugenLayoutKeys))
	for i := range mugenActions {
		offs[i] = mugenCodeOn(codes[i], d)
	}
	offs[len(mugenActions)] = mugenCodeOn(codes[ikemenMenuSlot], d)
	if old, kept := mugenYourLayouts.kinds[kind]; kept && fmt.Sprint(old) == fmt.Sprint(offs) {
		return
	}
	mugenYourLayouts.kinds[kind] = offs
	if err := mugenSaveYourLayouts(); err != nil {
		LogMessage("OMugen: could not keep your %s controller layout: %v", kind, err)
		return
	}
	LogMessage("OMugen: kept your %s controller layout (%s) for games whose own doesn't fit", kind, mugenYourLayoutsPath())
}

// mugenCodeOn is code's offset when it is on controller d, else 0.
func mugenCodeOn(code int, d mugenjoy.DeviceInfo) int {
	if code > 0 && code/mugenjoy.CodesPerJoystick == d.Index {
		return code % mugenjoy.CodesPerJoystick
	}
	return 0
}

// mugenYourLayout returns your layout for controller d's kind, as codes on
// d, or nil when none is kept.
func mugenYourLayout(d mugenjoy.DeviceInfo) []int {
	kind := mugenPadKind(d.Layout)
	if kind == "" {
		return nil
	}
	mugenLoadYourLayouts()
	offs, ok := mugenYourLayouts.kinds[kind]
	if !ok {
		return nil
	}
	codes := make([]int, ikemenMenuSlot+1)
	for i, o := range offs {
		slot := i
		if i == len(mugenActions) {
			slot = ikemenMenuSlot
		}
		if o > 0 {
			codes[slot] = d.Index*mugenjoy.CodesPerJoystick + o
		}
	}
	if !mugenLayoutFits(codes, d) {
		return nil
	}
	return codes
}

// ---------------------------------------------------------------- per player

// mugenReplacement is the layout a player got instead of mugen.cfg's.
type mugenReplacement struct {
	device int  // device ID
	yours  bool // your layout, else the standard one
}

// mugenReplaced records, per player, the replacement layout in use, so it is
// logged once and shown in controllers.txt.
var mugenReplaced = map[int]mugenReplacement{}

// mugenReplacementCodes returns the layout player p plays with on controller
// d when their codes don't suit it (yours, or the standard one), else nil;
// codes that suit d become your layout for its kind if you have none yet.
func mugenReplacementCodes(p int, codes []int, d mugenjoy.DeviceInfo) []int {
	if mugenLayoutFits(codes, d) {
		delete(mugenReplaced, p)
		if !mugenAutoPlayer(p) {
			mugenRememberLayout(codes, d, false)
		}
		return nil
	}
	r := mugenReplacement{device: d.ID}
	repl := mugenYourLayout(d)
	if repl != nil {
		r.yours = true
	} else if repl = mugenStandardLayout(d); repl == nil {
		return nil
	}
	if old, ok := mugenReplaced[p]; !ok || old != r {
		mugenReplaced[p] = r
		which := "the standard layout"
		if r.yours {
			which = "your " + mugenPadKind(d.Layout) + " controller layout"
		}
		LogMessage("OMugen: P%d's joystick layout in %s was made on another kind of controller "+
			"(its directions are on %s's triggers or buttons): P%d plays with %s",
			p+1, mugenCfgPath, d.Name, p+1, which)
		mugenWriteControllerReport()
	}
	return repl
}

// mugenReplacedLayout reports whether player p plays with a replacement for
// mugen.cfg's layout, and whether it is yours.
func mugenReplacedLayout(p int) (replaced, yours bool) {
	r, ok := mugenReplaced[p]
	return ok, r.yours
}
