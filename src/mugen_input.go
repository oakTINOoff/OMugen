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
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/oakTINOoff/OMugen/src/mugenjoy"
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
		sys.joystickConfig[p] = KeyConfig{Joy: -1, mugenPlayer: p + 1}
	}
	mugenPads.gen = -1
}

func mugenEnsureJoy() {
	if mugenJoy != nil {
		return
	}
	// Controllers come from SDL, laid out as WinMM numbers them
	// (omugen_sdljoy.go); OMUGEN_JOY=winmm reads WinMM like MUGEN does.
	// OMUGEN_FAKEPADS=n: n simulated DS4s instead of the real controllers,
	// pressed by input scripts (padtap), for screen tests without hardware.
	if n := int(Atoi(os.Getenv("OMUGEN_FAKEPADS"))); n <= 0 {
		if strings.EqualFold(os.Getenv("OMUGEN_JOY"), "winmm") {
			mugenJoy, mugenJoyBackendName = mugenjoy.NewManager(), "WinMM"
		} else {
			mugenJoy = mugenjoy.NewManagerWith(sdlJoyBackend{})
		}
	} else {
		mugenJoyBackendName = "simulated"
		var devs []mugenjoy.DeviceInfo
		for i := 0; i < n && i < mugenjoy.MaxJoysticks; i++ {
			devs = append(devs, mugenjoy.DeviceInfo{Index: i, ID: i, Name: "Fake DS4", VendorID: 0x054C, ProductID: 0x05C4,
				Layout: mugenjoy.Layout{NumAxes: 6, NumHats: 1, NumButtons: 14}})
		}
		mugenJoy = mugenjoy.NewFakeManagerRealtime(devs)
	}
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

// mugenBaseCodes is the joystick layout player p plays with, as written for
// the controller it was made on: their own codes, or player 1's for players
// without codes of their own. nil if they have none.
func mugenBaseCodes(p int) []int {
	if mugenAutoPlayer(p) {
		p = 0
	}
	if p < 0 || p >= len(sys.joystickConfig) || len(sys.joystickConfig[p].mugenCodes) <= ikemenMenuSlot {
		return nil
	}
	return sys.joystickConfig[p].mugenCodes
}

// Controller assignment. Windows can list controllers that send nothing
// (DS4Windows' originals of the pads it copies), so players are given the
// controllers that do send input (mugenjoy's live devices): the one their own
// codes name if it is live, otherwise the next live one in the order they
// were first pressed. A player keeps their controller until it goes away.
// The layout's codes then move to that controller.
var mugenPads = struct {
	gen    int                  // mugenJoy generation the slots match (-1: work them out again)
	id     [mugenMaxPlayers]int // backend device ID per player, -1: none
	idx    [mugenMaxPlayers]int // its device index
	joined []mugenJoin          // new assignments, for the on-screen notice
}{gen: -1, id: [mugenMaxPlayers]int{-1, -1, -1, -1}, idx: [mugenMaxPlayers]int{-1, -1, -1, -1}}

type mugenJoin struct {
	player int // 0-based
	pad    int // device index
}

// mugenWantsPad reports whether player p plays with a joystick layout.
func mugenWantsPad(p int) bool {
	return p >= 0 && p < mugenMaxPlayers && p < len(sys.joystickConfig) &&
		sys.joystickConfig[p].mugenPlayer == p+1 && mugenBaseCodes(p) != nil
}

func mugenAssignPads() {
	if mugenJoy == nil {
		return
	}
	gen := mugenJoy.Generation()
	if gen == mugenPads.gen {
		return
	}
	mugenPads.gen = gen
	index := map[int]int{} // device ID -> index
	idOf := map[int]int{}  // index -> device ID
	for _, d := range mugenJoy.Devices() {
		index[d.ID], idOf[d.Index] = d.Index, d.ID
	}
	live := mugenJoy.Live()
	isLive := map[int]bool{}
	for _, i := range live {
		isLive[i] = true
	}
	claimed := map[int]bool{}
	for p := 0; p < mugenMaxPlayers; p++ {
		if i, ok := index[mugenPads.id[p]]; ok && mugenPads.id[p] >= 0 && isLive[i] && mugenWantsPad(p) {
			mugenPads.idx[p] = i
			claimed[i] = true
		} else {
			mugenPads.id[p], mugenPads.idx[p] = -1, -1
		}
	}
	claim := func(p, i int) {
		mugenPads.id[p], mugenPads.idx[p] = idOf[i], i
		claimed[i] = true
		mugenPads.joined = append(mugenPads.joined, mugenJoin{p, i})
		LogMessage("OMugen: P%d plays with joystick %c", p+1, 'A'+i)
	}
	// A player's own controller first (the one their own codes name)...
	for p := 0; p < mugenMaxPlayers; p++ {
		if mugenPads.id[p] >= 0 || !mugenWantsPad(p) || mugenAutoPlayer(p) {
			continue
		}
		if h := mugenHomePad(mugenBaseCodes(p)[:len(mugenActions)]); h >= 0 && isLive[h] && !claimed[h] {
			claim(p, h)
		}
	}
	// ...then the other live controllers, in the order they were pressed.
	for p := 0; p < mugenMaxPlayers; p++ {
		if mugenPads.id[p] >= 0 || !mugenWantsPad(p) {
			continue
		}
		for _, i := range live {
			if !claimed[i] {
				claim(p, i)
				break
			}
		}
	}
	mugenWriteControllerReport()
	mugenSetPadLights()
}

// mugenPlayerPad is the device index player p plays with (-1: none yet).
func mugenPlayerPad(p int) int {
	if mugenJoy == nil || p < 0 || p >= mugenMaxPlayers {
		return -1
	}
	mugenJoy.Poll() // so a first press assigns a controller at once
	mugenAssignPads()
	if time.Since(mugenPadLightsAt) > 3*time.Second {
		mugenSetPadLights()
	}
	return mugenPads.idx[p]
}

// mugenPlayerCodes returns the joystick codes player p plays with: their
// layout moved to their controller, or nil when they have none yet.
func mugenPlayerCodes(p int) []int {
	base := mugenBaseCodes(p)
	if base == nil {
		return nil
	}
	pad := mugenPlayerPad(p)
	home := mugenHomePad(base[:len(mugenActions)])
	if pad < 0 || home < 0 {
		return nil
	}
	codes := base
	if pad != home {
		codes = make([]int, len(base))
		for i, c := range base {
			if c > 0 && c/mugenjoy.CodesPerJoystick == home {
				codes[i] = pad*mugenjoy.CodesPerJoystick + c%mugenjoy.CodesPerJoystick
			}
		}
	}
	d, ok := mugenDeviceAt(pad)
	if !ok {
		return codes
	}
	// A layout made on another kind of controller: yours or the standard one.
	if repl := mugenReplacementCodes(p, codes, d); repl != nil {
		return repl
	}
	// No pause menu button yet (no controller was connected when the bindings
	// were read): the controller's own Start/Options button, if it is free.
	src := p
	if mugenAutoPlayer(p) {
		src = 0
	}
	if codes[ikemenMenuSlot] == 0 && mugenInputActive != nil && src < len(mugenInputActive.players) && mugenInputActive.players[src].menu < 0 {
		if b, ok := mugenjoy.MenuButton(d); ok {
			c, free := mugenjoy.ButtonCode(d, b), true
			for _, x := range codes[:len(mugenActions)] {
				free = free && x != c
			}
			if c > 0 && free {
				codes = append([]int(nil), codes...)
				codes[ikemenMenuSlot] = c
			}
		}
	}
	return codes
}

// mugenResetPads forgets which controller each player has: the next presses
// hand them out again, in order (Options > Input > Reassign controllers).
func mugenResetPads() {
	if mugenJoy == nil {
		return
	}
	mugenJoy.ResetLive()
	mugenPads.gen = -1
	mugenReplaced = map[int]mugenReplacement{} // work out their layouts again too
	for p := range mugenPads.id {
		mugenPads.id[p], mugenPads.idx[p] = -1, -1
	}
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
	mugenPads.gen = -1 // bindings changed: hand out controllers again
	if noJoy {
		// no joysticks at all
	} else if mugenAutoPlayer(p) {
		mugenEnsureJoy()
		jc.mugenPlayer = p + 1
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
		jc.mugenPlayer = p + 1
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
		} else if j := i - len(out); j < 4 {
			// a standard layout's second codes for the directions (left stick)
			out[j] = out[j] || mugenJoy.Pressed(code)
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
