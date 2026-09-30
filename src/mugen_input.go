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
	"bufio"
	"os"
	"strconv"
	"strings"

	"github.com/ikemen-engine/Ikemen-GO/src/mugenjoy"
	"github.com/veandco/go-sdl2/sdl"
)

const mugenCfgPath = "data/mugen.cfg"

// mugenActions is MUGEN's key order; it lines up with the first 11 slots of
// Ikemen's [14]bool input (U, D, L, R, a, b, c, x, y, z, s).
var mugenActions = [...]string{"jump", "crouch", "left", "right", "a", "b", "c", "x", "y", "z", "start"}

type mugenPlayerInput struct {
	useKeyboard bool
	useJoystick bool
	keys        [len(mugenActions)]int // SDL 1.2 keysyms
	joy         [len(mugenActions)]int // MUGEN joystick codes
}

type mugenInputConfig struct {
	players [2]mugenPlayerInput
}

// mugenJoy is created only when a mugen.cfg joystick binding is in use.
var mugenJoy *mugenjoy.Manager

// loadMugenInputConfig parses the input sections of a MUGEN mugen.cfg.
// Missing entries keep MUGEN 1.1's shipped defaults.
func loadMugenInputConfig(path string) (*mugenInputConfig, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	cfg := &mugenInputConfig{players: [2]mugenPlayerInput{
		{useKeyboard: true, useJoystick: true,
			keys: [...]int{119, 115, 97, 100, 106, 107, 108, 117, 105, 111, 110},
			joy:  [...]int{9, 10, 11, 12, 13, 14, 20, 15, 16, 18, 24}},
		{useKeyboard: true, useJoystick: true,
			keys: [...]int{273, 274, 276, 275, 260, 261, 262, 263, 264, 265, 266},
			joy:  [...]int{41, 42, 43, 44, 46, 47, 52, 45, 48, 50, 54}},
	}}

	section := ""
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if i := strings.IndexByte(line, ';'); i >= 0 {
			line = line[:i]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if line[0] == '[' {
			if end := strings.IndexByte(line, ']'); end > 0 {
				section = strings.ToLower(strings.TrimSpace(line[1:end]))
			}
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		n, err := strconv.Atoi(strings.TrimSpace(val))
		if err != nil {
			continue
		}
		switch section {
		case "input":
			for p := range cfg.players {
				prefix := "p" + strconv.Itoa(p+1) + "."
				switch key {
				case prefix + "usekeyboard":
					cfg.players[p].useKeyboard = n != 0
				case prefix + "joystick.type":
					cfg.players[p].useJoystick = n != 0
				}
			}
		case "p1 keys", "p2 keys", "p1 joystick", "p2 joystick":
			p := int(section[1] - '1')
			for i, a := range mugenActions {
				if key != a {
					continue
				}
				if strings.HasSuffix(section, "keys") {
					cfg.players[p].keys[i] = n
				} else {
					cfg.players[p].joy[i] = n
				}
			}
		}
	}
	return cfg, sc.Err()
}

// applyMugenInputConfig overrides players 1 and 2 with mugen.cfg bindings.
// It does nothing when the game folder has no mugen.cfg.
func applyMugenInputConfig() {
	if FileExist(mugenCfgPath) == "" {
		return
	}
	cfg, err := loadMugenInputConfig(mugenCfgPath)
	if err != nil {
		LogMessage("OMugen: could not read %s: %v", mugenCfgPath, err)
		return
	}
	_, noJoy := sys.cmdFlags["-nojoy"]
	for p, pc := range cfg.players {
		if p >= len(sys.keyConfig) {
			break
		}
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
			continue
		}
		jc := KeyConfig{Joy: -1}
		if pc.useJoystick && !noJoy {
			jc.mugenCodes = pc.joy[:]
			if mugenJoy == nil {
				mugenJoy = mugenjoy.NewManager()
				for _, d := range mugenJoy.Devices() {
					LogMessage("OMugen: joystick %c = %q (%d axes, %d hat, %d buttons)",
						'A'+d.Index, d.Name, d.Layout.NumAxes, d.Layout.NumHats, d.Layout.NumButtons)
				}
			}
		}
		sys.joystickConfig[p] = jc
	}
	LogMessage("OMugen: players 1-2 use controls from %s", mugenCfgPath)
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
