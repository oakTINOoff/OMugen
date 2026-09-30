package main

// OMugen: Lua API for the MUGEN-style controls menu. In MUGEN mode, players
// 1-2 are bound by data/mugen.cfg; these functions let the options menu show,
// capture, change and save those bindings (MUGEN key codes and joystick codes).

import (
	"strconv"
	"strings"

	"github.com/ikemen-engine/Ikemen-GO/src/mugenjoy"
	lua "github.com/yuin/gopher-lua"
)

// Ikemen key-menu item names -> index into MUGEN's action arrays.
var omugenActionIndex = map[string]int{
	"up": 0, "down": 1, "left": 2, "right": 3,
	"a": 4, "b": 5, "c": 6, "x": 7, "y": 8, "z": 9, "start": 10,
}

// mugenCfgKeyNames are the key names MUGEN writes in [Pn Keys]/[Pn Joystick].
var mugenCfgKeyNames = [...]string{"Jump", "Crouch", "Left", "Right", "A", "B", "C", "X", "Y", "Z", "Start"}

// omugenPendingEdits collects unsaved mugen.cfg changes (last value wins).
var omugenPendingEdits []mugenCfgEdit

// omugenCapturePrev is the joystick state seen by the previous capture poll.
var omugenCapturePrev mugenjoy.State

func omugenSection(player int, kind string) string {
	if strings.EqualFold(kind, "Keys") {
		return "P" + strconv.Itoa(player) + " Keys"
	}
	return "P" + strconv.Itoa(player) + " Joystick"
}

func omugenAddEdit(e mugenCfgEdit) {
	for i := range omugenPendingEdits {
		if strings.EqualFold(omugenPendingEdits[i].section, e.section) && strings.EqualFold(omugenPendingEdits[i].key, e.key) {
			omugenPendingEdits[i].value = e.value
			return
		}
	}
	omugenPendingEdits = append(omugenPendingEdits, e)
}

// omugenBinding returns the display text and raw value of a binding.
func omugenBinding(player int, kind, action string) (string, int) {
	if mugenInputActive == nil || player < 1 || player > len(mugenInputActive.players) {
		return "", 0
	}
	pc := mugenInputActive.players[player-1]
	action = strings.ToLower(action)
	if strings.EqualFold(kind, "Keys") {
		i, ok := omugenActionIndex[action]
		if !ok || pc.keys[i] == 0 {
			return "", 0
		}
		return KeyToString(sdl12KeyToKeycode(pc.keys[i])), pc.keys[i]
	}
	code := 0
	if action == "menu" {
		if p := player - 1; p < len(sys.joystickConfig) && len(sys.joystickConfig[p].mugenCodes) > ikemenMenuSlot {
			code = sys.joystickConfig[p].mugenCodes[ikemenMenuSlot]
		}
	} else if i, ok := omugenActionIndex[action]; ok {
		code = pc.joy[i]
	}
	if code <= 0 {
		return "", 0
	}
	return mugenjoy.Label(code), code
}

// omugenSetBinding changes a binding in memory, applies it at once and queues
// the mugen.cfg edit. value is an Ikemen key name for "Keys" ("" unbinds) or a
// MUGEN joystick code for "Joystick" (0 unbinds).
func omugenSetBinding(player int, kind, action, value string) bool {
	if mugenInputActive == nil || player < 1 || player > len(mugenInputActive.players) {
		return false
	}
	pc := &mugenInputActive.players[player-1]
	action = strings.ToLower(action)
	section := omugenSection(player, kind)
	if strings.EqualFold(kind, "Keys") {
		i, ok := omugenActionIndex[action]
		if !ok {
			return false // MUGEN has no keyboard "menu", "d" or "w" (Esc opens the menu)
		}
		sym := 0
		if value != "" {
			if sym = keycodeToSdl12(StringToKey(value)); sym == 0 {
				return false // a key MUGEN cannot store
			}
		}
		pc.keys[i] = sym
		omugenAddEdit(mugenCfgEdit{section, mugenCfgKeyNames[i], strconv.Itoa(sym)})
	} else {
		code := int(Atoi(value))
		if code < 0 || code >= mugenjoy.NumCodes {
			return false
		}
		if action == "menu" {
			pc.menu = code
			omugenAddEdit(mugenCfgEdit{section, "Menu", strconv.Itoa(code)})
		} else if i, ok := omugenActionIndex[action]; ok {
			pc.joy[i] = code
			omugenAddEdit(mugenCfgEdit{section, mugenCfgKeyNames[i], strconv.Itoa(code)})
		} else {
			return false
		}
	}
	applyMugenPlayer(player - 1)
	return true
}

// omugenCaptureJoystick returns the MUGEN code of an input that became
// pressed since the previous call, or 0. Inputs already held (such as analog
// triggers at rest) never register.
func omugenCaptureJoystick(reset bool) int {
	if mugenJoy == nil {
		mugenJoy = mugenjoy.NewManager()
	}
	mugenJoy.Poll()
	cur := mugenJoy.Snapshot()
	defer func() { omugenCapturePrev = cur }()
	if reset {
		return 0
	}
	for code := 1; code < mugenjoy.NumCodes; code++ {
		if cur[code] && !omugenCapturePrev[code] {
			return code
		}
	}
	return 0
}

func omugenScriptInit(l *lua.LState) {
	omugenScreenpackScriptInit(l)
	luaRegister(l, "omugenControlsActive", func(l *lua.LState) int {
		/*Whether players 1-2 use MUGEN controls from data/mugen.cfg.
		@function omugenControlsActive
		@treturn boolean active*/
		l.Push(lua.LBool(mugenInputActive != nil))
		return 1
	})
	luaRegister(l, "omugenGetBinding", func(l *lua.LState) int {
		/*Get a MUGEN-mode binding.
		@function omugenGetBinding
		@tparam int player 1 or 2
		@tparam string kind "Keys" or "Joystick"
		@tparam string action up/down/left/right/a/b/c/x/y/z/start/menu
		@treturn string display Key name or MUGEN joystick label ("A-17"), "" if unbound
		@treturn int value SDL 1.2 key code or MUGEN joystick code*/
		text, v := omugenBinding(int(numArg(l, 1)), strArg(l, 2), strArg(l, 3))
		l.Push(lua.LString(text))
		l.Push(lua.LNumber(v))
		return 2
	})
	luaRegister(l, "omugenSetBinding", func(l *lua.LState) int {
		/*Change a MUGEN-mode binding (applied at once, saved by omugenSaveControls).
		@function omugenSetBinding
		@tparam int player 1 or 2
		@tparam string kind "Keys" or "Joystick"
		@tparam string action up/down/left/right/a/b/c/x/y/z/start/menu
		@tparam string value Key name for Keys ("" unbinds), joystick code for Joystick ("0" unbinds)
		@treturn boolean ok*/
		l.Push(lua.LBool(omugenSetBinding(int(numArg(l, 1)), strArg(l, 2), strArg(l, 3), strArg(l, 4))))
		return 1
	})
	luaRegister(l, "omugenCaptureJoystick", func(l *lua.LState) int {
		/*Return the MUGEN joystick code newly pressed since the last call (0 if none).
		@function omugenCaptureJoystick
		@tparam[opt] boolean reset Only record the current state (start of a capture)
		@treturn int code
		@treturn string label MUGEN config-screen label, e.g. "A-17"*/
		code := omugenCaptureJoystick(!nilArg(l, 1) && lua.LVAsBool(l.Get(1)))
		l.Push(lua.LNumber(code))
		l.Push(lua.LString(mugenjoy.Label(code)))
		return 2
	})
	luaRegister(l, "omugenKeySupported", func(l *lua.LState) int {
		/*Whether MUGEN's mugen.cfg can store a keyboard key.
		@function omugenKeySupported
		@tparam string key OMugen key name (as returned by getKey)
		@treturn boolean ok*/
		l.Push(lua.LBool(keycodeToSdl12(StringToKey(strArg(l, 1))) != 0))
		return 1
	})
	luaRegister(l, "omugenRevertControls", func(l *lua.LState) int {
		/*Drop unsaved MUGEN-mode binding changes and reload data/mugen.cfg.
		@function omugenRevertControls*/
		if len(omugenPendingEdits) > 0 {
			omugenPendingEdits = nil
			applyMugenInputConfig(loadGameMugenCfg())
		}
		return 0
	})
	luaRegister(l, "omugenSaveControls", func(l *lua.LState) int {
		/*Write changed MUGEN-mode bindings to data/mugen.cfg (old file backed up to save/backups).
		@function omugenSaveControls
		@treturn boolean ok
		@treturn string error*/
		if len(omugenPendingEdits) == 0 {
			l.Push(lua.LTrue)
			return 1
		}
		if err := writeMugenCfgEdits(mugenCfgPath, omugenPendingEdits); err != nil {
			l.Push(lua.LFalse)
			l.Push(lua.LString(err.Error()))
			return 2
		}
		omugenPendingEdits = nil
		l.Push(lua.LTrue)
		return 1
	})
}
