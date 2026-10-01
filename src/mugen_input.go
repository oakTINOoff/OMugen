package main

// OMugen: MUGEN 1.1 input compatibility.
//
// When the game folder has a MUGEN data/mugen.cfg, players 1 and 2 use its
// [Input], [P1 Keys], [P2 Keys], [P1 Joystick] and [P2 Joystick] sections
// exactly as MUGEN 1.1 interprets them, instead of Ikemen's Keys_Pn/Joystick_Pn:
//   - key values are SDL 1.2 keysyms, converted here to SDL2 keycodes;
//   - joystick values are MUGEN joystick codes, evaluated by package mugenjoy
//     through the same WinMM API MUGEN's SDL 1.2 uses.

import (
	"strconv"

	"github.com/ikemen-engine/Ikemen-GO/src/mugenjoy"
	"github.com/veandco/go-sdl2/sdl"
)

// mugenActions is MUGEN's key order; it lines up with the first 11 slots of
// Ikemen's [14]bool input (U, D, L, R, a, b, c, x, y, z, s).
var mugenActions = [...]string{"jump", "crouch", "left", "right", "a", "b", "c", "x", "y", "z", "start"}

type mugenPlayerInput struct {
	useKeyboard bool
	useJoystick bool
	keys        [len(mugenActions)]int // SDL 1.2 keysyms
	joy         [len(mugenActions)]int // MUGEN joystick codes
	// menu is an OMugen extension: the joystick code that opens the pause menu
	// (Ikemen's "m" button). -1 = not set in mugen.cfg, pick the pad's own
	// Start/Options button. MUGEN itself ignores this key.
	menu int
}

// ikemenMenuSlot is the index of the "m" (menu) button in Ikemen's [14]bool input.
const ikemenMenuSlot = 13

type mugenInputConfig struct {
	players [2]mugenPlayerInput
}

// mugenJoy is created only when a mugen.cfg joystick binding is in use.
var mugenJoy *mugenjoy.Manager

// mugenInput reads the input sections of a MUGEN mugen.cfg.
// Missing entries keep MUGEN 1.1's shipped defaults.
func mugenInput(mc mugenCfg) *mugenInputConfig {
	cfg := &mugenInputConfig{players: [2]mugenPlayerInput{
		{useKeyboard: true, useJoystick: true, menu: -1,
			keys: [...]int{119, 115, 97, 100, 106, 107, 108, 117, 105, 111, 110},
			joy:  [...]int{9, 10, 11, 12, 13, 14, 20, 15, 16, 18, 24}},
		{useKeyboard: true, useJoystick: true, menu: -1,
			keys: [...]int{273, 274, 276, 275, 260, 261, 262, 263, 264, 265, 266},
			joy:  [...]int{41, 42, 43, 44, 46, 47, 52, 45, 48, 50, 54}},
	}}
	for p := range cfg.players {
		pn := "p" + strconv.Itoa(p+1)
		if v, ok := mc.int(pn+" joystick", "menu"); ok {
			cfg.players[p].menu = v
		}
		if v, ok := mc.int("input", pn+".usekeyboard"); ok {
			cfg.players[p].useKeyboard = v != 0
		}
		if v, ok := mc.int("input", pn+".joystick.type"); ok {
			cfg.players[p].useJoystick = v != 0
		}
		for i, a := range mugenActions {
			if v, ok := mc.int(pn+" keys", a); ok {
				cfg.players[p].keys[i] = v
			}
			if v, ok := mc.int(pn+" joystick", a); ok {
				cfg.players[p].joy[i] = v
			}
		}
	}
	return cfg
}

// applyMugenInputConfig overrides players 1 and 2 with mugen.cfg bindings.
// It does nothing when the game folder has no mugen.cfg.
func applyMugenInputConfig(mc mugenCfg) {
	if mc == nil {
		return
	}
	mugenInputActive = mugenInput(mc)
	for p := range mugenInputActive.players {
		applyMugenPlayer(p)
	}
	applyMugenAutoPlayers()
	LogMessage("OMugen: players 1-2 use controls from %s; players without joystick codes of their own use player 1's on the other controllers", mugenCfgPath)
}

// applyMugenAutoPlayers gives players 3-4 (MUGEN has none) player 1's pad
// layout on the other controllers.
func applyMugenAutoPlayers() {
	if _, noJoy := sys.cmdFlags["-nojoy"]; mugenInputActive == nil || noJoy {
		return
	}
	for p := len(mugenInputActive.players); p < mugenMaxPlayers && p < len(sys.joystickConfig); p++ {
		mugenEnsureJoy()
		sys.joystickConfig[p] = KeyConfig{Joy: -1, mugenAuto: p}
	}
}

func mugenEnsureJoy() {
	if mugenJoy != nil {
		return
	}
	mugenJoy = mugenjoy.NewManager()
	for _, d := range mugenJoy.Devices() {
		LogMessage("OMugen: joystick %c = %q VID %04X PID %04X (%d axes, %d hat, %d buttons)",
			'A'+d.Index, d.Name, d.VendorID, d.ProductID, d.Layout.NumAxes, d.Layout.NumHats, d.Layout.NumButtons)
	}
}

// Shared pad layout. MUGEN gives a player a controller only through their own
// [Pn Joystick] codes, and its config has no players 3 and 4. OMugen gives
// players 2-4 without joystick codes of their own (a fresh mugen.cfg has all
// 0 for P2) player 1's layout on the other connected controllers, so a
// friend's pad works at once with the same buttons. Controllers are matched to
// player 1's by vendor and layout, which leaves out the virtual twin of a pad
// that tools like DS4Windows add, and are handed out in the order Windows
// numbers them, following controllers as they are plugged in. Nothing is
// written to mugen.cfg.
const mugenMaxPlayers = 4

// mugenAutoPlayer reports whether player p (0-based) uses the shared layout.
func mugenAutoPlayer(p int) bool {
	if mugenInputActive == nil || p < 1 || p >= mugenMaxPlayers {
		return false
	}
	if p >= len(mugenInputActive.players) {
		return true
	}
	pc := mugenInputActive.players[p]
	if pc.menu > 0 {
		return false
	}
	for _, c := range pc.joy {
		if c > 0 {
			return false
		}
	}
	return true
}

// mugenHomePad is the controller most of the codes point at (-1: none).
func mugenHomePad(codes []int) int {
	var votes [mugenjoy.MaxJoysticks]int
	for _, c := range codes {
		if c > 0 && c < mugenjoy.NumCodes {
			votes[c/mugenjoy.CodesPerJoystick]++
		}
	}
	pad := -1
	for j := range votes {
		if votes[j] > 0 && (pad < 0 || votes[j] > votes[pad]) {
			pad = j
		}
	}
	return pad
}

// mugenAutoCodes returns the joystick codes of shared-layout player p: player
// 1's codes (pause menu button included) moved to that player's controller,
// or nil when there is no controller for them.
func mugenAutoCodes(p int) []int {
	if !mugenAutoPlayer(p) || mugenJoy == nil || len(sys.joystickConfig) == 0 {
		return nil
	}
	src := sys.joystickConfig[0].mugenCodes
	if len(src) <= ikemenMenuSlot {
		return nil
	}
	home := mugenHomePad(src[:len(mugenActions)])
	if home < 0 {
		return nil
	}
	devs := mugenJoy.Devices()
	homeIdx := -1
	for i, d := range devs {
		if d.Index == home {
			homeIdx = i
		}
	}
	if homeIdx < 0 {
		return nil
	}
	taken := map[int]bool{home: true}
	if !mugenAutoPlayer(1) && len(sys.joystickConfig) > 1 && len(sys.joystickConfig[1].mugenCodes) > ikemenMenuSlot {
		if pad := mugenHomePad(sys.joystickConfig[1].mugenCodes[:len(mugenActions)]); pad >= 0 {
			taken[pad] = true
		}
	}
	var pads []int // devices are listed in index order
	for _, d := range devs {
		if !taken[d.Index] && d.VendorID == devs[homeIdx].VendorID && d.Layout == devs[homeIdx].Layout {
			pads = append(pads, d.Index)
		}
	}
	k := 0 // this player's place among the shared-layout players
	for q := 1; q < p; q++ {
		if mugenAutoPlayer(q) {
			k++
		}
	}
	if k >= len(pads) {
		return nil
	}
	codes := make([]int, len(src))
	for i, c := range src {
		if c > 0 && c/mugenjoy.CodesPerJoystick == home {
			codes[i] = pads[k]*mugenjoy.CodesPerJoystick + c%mugenjoy.CodesPerJoystick
		}
	}
	return codes
}

// mugenInputActive holds the mugen.cfg bindings in use (nil: not in MUGEN mode).
var mugenInputActive *mugenInputConfig

// applyMugenPlayer installs player p's mugen.cfg bindings into Ikemen's key configs.
func applyMugenPlayer(p int) {
	if mugenInputActive == nil || p < 0 || p >= len(mugenInputActive.players) || p >= len(sys.keyConfig) {
		return
	}
	pc := mugenInputActive.players[p]
	_, noJoy := sys.cmdFlags["-nojoy"]
	kc := KeyConfig{Joy: -1}
	if pc.useKeyboard {
		v := [14]int{}
		for i, sym := range pc.keys {
			v[i] = int(sdl12KeyToKeycode(sym))
		}
		kc.set(v)
	}
	sys.keyConfig[p] = kc

	if p >= len(sys.joystickConfig) {
		return
	}
	jc := KeyConfig{Joy: -1}
	if noJoy {
		// no joysticks at all
	} else if mugenAutoPlayer(p) {
		mugenEnsureJoy()
		jc.mugenAuto = p
	} else if pc.useJoystick {
		mugenEnsureJoy()
		// A fresh slice per player (never alias the config arrays).
		codes := make([]int, ikemenMenuSlot+1)
		copy(codes, pc.joy[:])
		codes[ikemenMenuSlot] = mugenMenuCode(pc, mugenJoy.Devices())
		if codes[ikemenMenuSlot] > 0 {
			LogMessage("OMugen: P%d pause menu button = joystick code %d [%s]",
				p+1, codes[ikemenMenuSlot], mugenjoy.Label(codes[ikemenMenuSlot]))
		}
		jc.mugenCodes = codes
	}
	sys.joystickConfig[p] = jc
}

// mugenMenuCode picks the joystick code that opens the pause menu: an explicit
// "Menu = code" in [Pn Joystick], else the Start/Options button of the pad the
// player's bindings point at, unless that button is already bound to an action.
func mugenMenuCode(pc mugenPlayerInput, devs []mugenjoy.DeviceInfo) int {
	if pc.menu >= 0 {
		return pc.menu
	}
	// The pad this player uses: the code block most of their bindings fall in.
	var votes [mugenjoy.MaxJoysticks]int
	for _, c := range pc.joy {
		if c > 0 && c < mugenjoy.NumCodes {
			votes[c/mugenjoy.CodesPerJoystick]++
		}
	}
	joy := 0
	for j := range votes {
		if votes[j] > votes[joy] {
			joy = j
		}
	}
	if votes[joy] == 0 {
		return 0
	}
	for _, d := range devs {
		if d.Index != joy {
			continue
		}
		b, ok := mugenjoy.MenuButton(d)
		if !ok {
			return 0
		}
		code := mugenjoy.ButtonCode(d, b)
		for _, c := range pc.joy {
			if c == code {
				return 0 // already used by a MUGEN action
			}
		}
		return code
	}
	return 0
}

// mugenJoystickState evaluates MUGEN joystick codes for one player.
func mugenJoystickState(codes []int) [14]bool {
	var out [14]bool
	if mugenJoy == nil {
		return out
	}
	mugenJoy.Poll()
	for i, code := range codes {
		if i < len(out) {
			out[i] = mugenJoy.Pressed(code)
		}
	}
	return out
}

// sdl12KeyToKeycode converts an SDL 1.2 keysym (as stored in mugen.cfg) to
// the SDL2 keycode Ikemen tracks. Printable ASCII keysyms are identical.
func sdl12KeyToKeycode(sym int) sdl.Keycode {
	if k, ok := sdl12Special[sym]; ok {
		return k
	}
	switch {
	case sym <= 0:
		return sdl.K_UNKNOWN
	case sym < 128:
		return sdl.Keycode(sym)
	case sym >= 160 && sym <= 255:
		// SDLK_WORLD_0..95 were defined as the Latin-1 characters 0xA0-0xFF,
		// which is what SDL2 uses as the keycode for those characters.
		return sdl.Keycode(sym)
	case sym >= 256 && sym <= 265:
		return [...]sdl.Keycode{sdl.K_KP_0, sdl.K_KP_1, sdl.K_KP_2, sdl.K_KP_3, sdl.K_KP_4,
			sdl.K_KP_5, sdl.K_KP_6, sdl.K_KP_7, sdl.K_KP_8, sdl.K_KP_9}[sym-256]
	case sym >= 282 && sym <= 296:
		return [...]sdl.Keycode{sdl.K_F1, sdl.K_F2, sdl.K_F3, sdl.K_F4, sdl.K_F5, sdl.K_F6, sdl.K_F7, sdl.K_F8,
			sdl.K_F9, sdl.K_F10, sdl.K_F11, sdl.K_F12, sdl.K_F13, sdl.K_F14, sdl.K_F15}[sym-282]
	}
	return sdl.K_UNKNOWN
}

// keycodeToSdl12 converts an SDL2 keycode back to the SDL 1.2 keysym MUGEN
// stores in mugen.cfg (0 if MUGEN has no equivalent). It inverts
// sdl12KeyToKeycode, preferring the lowest keysym when several map to one key.
func keycodeToSdl12(k sdl.Keycode) int {
	if k == sdl.K_UNKNOWN {
		return 0
	}
	for sym := 1; sym <= 322; sym++ {
		if sdl12KeyToKeycode(sym) == k {
			return sym
		}
	}
	return 0
}

// sdl12Special lists SDL 1.2 keysyms whose SDL2 keycode differs.
var sdl12Special = map[int]sdl.Keycode{
	12:  sdl.K_CLEAR,
	19:  sdl.K_PAUSE,
	266: sdl.K_KP_PERIOD,
	267: sdl.K_KP_DIVIDE,
	268: sdl.K_KP_MULTIPLY,
	269: sdl.K_KP_MINUS,
	270: sdl.K_KP_PLUS,
	271: sdl.K_KP_ENTER,
	272: sdl.K_KP_EQUALS,
	273: sdl.K_UP,
	274: sdl.K_DOWN,
	275: sdl.K_RIGHT,
	276: sdl.K_LEFT,
	277: sdl.K_INSERT,
	278: sdl.K_HOME,
	279: sdl.K_END,
	280: sdl.K_PAGEUP,
	281: sdl.K_PAGEDOWN,
	300: sdl.K_NUMLOCKCLEAR,
	301: sdl.K_CAPSLOCK,
	302: sdl.K_SCROLLLOCK,
	303: sdl.K_RSHIFT,
	304: sdl.K_LSHIFT,
	305: sdl.K_RCTRL,
	306: sdl.K_LCTRL,
	307: sdl.K_RALT,
	308: sdl.K_LALT,
	309: sdl.K_RGUI, // RMETA
	310: sdl.K_LGUI, // LMETA
	311: sdl.K_LGUI, // LSUPER
	312: sdl.K_RGUI, // RSUPER
	313: sdl.K_MODE,
	315: sdl.K_HELP,
	316: sdl.K_PRINTSCREEN,
	317: sdl.K_SYSREQ,
	318: sdl.K_PAUSE,       // BREAK
	319: sdl.K_APPLICATION, // SDL 1.2 MENU is the Windows "Apps" key
	320: sdl.K_POWER,
	321: 0x20AC, // EURO
	322: sdl.K_UNDO,
}
