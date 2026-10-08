package main

// OMugen: Lua API for the MUGEN-style controls menu. In MUGEN mode, players
// 1-2 are bound by data/mugen.cfg; these functions let the options menu show,
// capture, change and save those bindings (MUGEN key codes and joystick codes).

import (
	"strconv"
	"strings"
	"time"

	"github.com/oakTINOoff/OMugen/src/mugenjoy"
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

// omugenBinding returns the display text and raw value of a binding. For a
// joystick, the text is the code the player plays with on the controller
// they have now (mugenPlayerCodes) and the value is their own binding in
// mugen.cfg (0 for players on player 1's layout, who have none).
func omugenBinding(player int, kind, action string) (string, int) {
	if mugenInputActive == nil || player < 1 {
		return "", 0
	}
	action = strings.ToLower(action)
	p := player - 1
	if strings.EqualFold(kind, "Keys") {
		if p >= len(mugenInputActive.players) {
			return "", 0
		}
		pc := mugenInputActive.players[p]
		i, ok := omugenActionIndex[action]
		if !ok || pc.keys[i] == 0 {
			return "", 0
		}
		return KeyToString(sdl12KeyToKeycode(pc.keys[i])), pc.keys[i]
	}
	i, ok := omugenActionIndex[action]
	if action == "menu" {
		i, ok = ikemenMenuSlot, true
	}
	if !ok {
		return "", 0
	}
	value := 0
	if !mugenAutoPlayer(p) && p < len(mugenInputActive.players) {
		if i == ikemenMenuSlot {
			if p < len(sys.joystickConfig) && len(sys.joystickConfig[p].mugenCodes) > ikemenMenuSlot {
				value = sys.joystickConfig[p].mugenCodes[ikemenMenuSlot]
			}
		} else {
			value = mugenInputActive.players[p].joy[i]
		}
	}
	shown := value
	if codes := mugenPlayerCodes(p); codes != nil {
		shown = codes[i]
	}
	if shown <= 0 {
		return "", value
	}
	return mugenjoy.Label(shown), value
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
		if code > 0 {
			// Changing one button of a player on player 1's layout, or on the
			// standard layout (mugen.cfg's was made on another controller),
			// keeps the rest of that layout as their own bindings.
			standard := mugenUsesStandardLayout(player - 1)
			if mugenAutoPlayer(player-1) || standard {
				if auto := mugenPlayerCodes(player - 1); auto != nil {
					for i := range pc.joy {
						pc.joy[i] = auto[i]
						omugenAddEdit(mugenCfgEdit{section, mugenCfgKeyNames[i], strconv.Itoa(auto[i])})
					}
					if standard && auto[ikemenMenuSlot] > 0 {
						pc.menu = auto[ikemenMenuSlot]
						omugenAddEdit(mugenCfgEdit{section, "Menu", strconv.Itoa(pc.menu)})
					}
				}
			}
			// A binding is useless while mugen.cfg turns the player's
			// joystick off (P2.Joystick.type = 0 in a fresh mugen.cfg).
			if !pc.useJoystick {
				pc.useJoystick = true
				omugenAddEdit(mugenCfgEdit{"Input", "P" + strconv.Itoa(player) + ".Joystick.type", "1"})
			}
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

// omugenKeyActivity is when each player last pressed one of their keys.
var omugenKeyActivity [mugenMaxPlayers]time.Time

// omugenNoteKey records which players a pressed key belongs to.
func omugenNoteKey(key Key) {
	for p := 0; p < mugenMaxPlayers && p < len(sys.keyConfig); p++ {
		kc := &sys.keyConfig[p]
		for _, k := range [...]int{kc.dU, kc.dD, kc.dL, kc.dR, kc.bA, kc.bB, kc.bC, kc.bX, kc.bY, kc.bZ, kc.bS, kc.bD, kc.bW, kc.bM} {
			if k != 0 && k == int(key) {
				omugenKeyActivity[p] = time.Now()
				break
			}
		}
	}
}

// omugenControllersTable describes players 1-4 for the controller indicator
// (omugen_hud.lua): their controller, whether they want one, whether they
// just pressed something, and which player pressed last.
func omugenControllersTable(l *lua.LState) *lua.LTable {
	t := l.NewTable()
	if mugenInputActive == nil || mugenJoy == nil {
		return t
	}
	now := time.Now()
	t.RawSetString("devices", lua.LNumber(len(mugenJoy.Devices())))
	active, best := 0, time.Time{}
	for p := 0; p < mugenMaxPlayers && p < len(sys.joystickConfig); p++ {
		e := l.NewTable()
		last := omugenKeyActivity[p]
		if pad := mugenPlayerPad(p); pad >= 0 {
			e.RawSetString("pad", lua.LString(string(rune('A'+pad))))
			if lp := mugenJoy.LastPress(pad); lp.After(last) {
				last = lp
			}
		}
		e.RawSetString("wants", lua.LBool(mugenWantsPad(p)))
		e.RawSetString("recent", lua.LBool(now.Sub(last) < 250*time.Millisecond))
		if last.After(best) {
			best, active = last, p+1
		}
		t.RawSetInt(p+1, e)
	}
	t.RawSetString("active", lua.LNumber(active))
	return t
}

func omugenScriptInit(l *lua.LState) {
	omugenScreenpackScriptInit(l)
	luaRegister(l, "omugenControllers", func(l *lua.LState) int {
		/*Players 1-4 and their controllers, for the controller indicator.
		@function omugenControllers
		@treturn table info `devices` (connected controllers), `active` (player who pressed last),
		  and per player `[1..4]`: `pad` (controller letter, "" if none), `wants`, `recent`*/
		l.Push(omugenControllersTable(l))
		return 1
	})
	luaRegister(l, "omugenTakeJoins", func(l *lua.LState) int {
		/*Controllers given to players since the last call.
		@function omugenTakeJoins
		@treturn table joins array of `{player = n, pad = "B"}`*/
		t := l.NewTable()
		for _, j := range mugenPads.joined {
			e := l.NewTable()
			e.RawSetString("player", lua.LNumber(j.player+1))
			e.RawSetString("pad", lua.LString(string(rune('A'+j.pad))))
			t.Append(e)
		}
		mugenPads.joined = nil
		l.Push(t)
		return 1
	})
	luaRegister(l, "omugenTextBounds", func(l *lua.LState) int {
		/*Where a text sprite draws a text, in the engine's 320x240 space (the
		same for every localcoord: draw over it with localcoord 320, 240).
		@function omugenTextBounds
		@tparam TextSprite ts Text sprite userdata (position already set).
		@tparam string text Text to measure.
		@treturn number left Left edge.
		@treturn number baseline Baseline Y.
		@treturn number width Width.
		@treturn number height Font height.*/
		ts, ok := toUserData(l, 1).(*TextSprite)
		if !ok {
			userDataError(l, 1, ts)
		}
		x := ts.x
		w := float32(ts.TextWidth(strArg(l, 2))) * ts.xscl
		h := float32(0)
		if ts.fnt != nil {
			h = float32(ts.fnt.Size[1]) * ts.yscl
		}
		switch {
		case ts.align == 0:
			x -= w / 2
		case ts.align < 0:
			x -= w
		}
		for _, v := range []float32{x, ts.y, w, h} {
			l.Push(lua.LNumber(v))
		}
		return 4
	})
	luaRegister(l, "omugenAnimBounds", func(l *lua.LState) int {
		/*Where an animation draws its current sprite, in the engine's 320x240 space.
		@function omugenAnimBounds
		@tparam Anim anim Animation userdata (position already set).
		@treturn number left Left edge (nothing if there is no sprite).
		@treturn number top Top edge.
		@treturn number width Width.
		@treturn number height Height.*/
		a, ok := toUserData(l, 1).(*Anim)
		if !ok {
			userDataError(l, 1, a)
		}
		if a.anim == nil || a.anim.spr == nil {
			return 0
		}
		spr := a.anim.spr
		x := a.x - float32(spr.Offset[0])*a.xscl
		y := a.y - float32(spr.Offset[1])*a.yscl
		if f := a.anim.curFrame(); f != nil {
			x += float32(f.Xoffset) * a.xscl
			y += float32(f.Yoffset) * a.yscl
		}
		for _, v := range []float32{x, y, float32(spr.Size[0]) * a.xscl, float32(spr.Size[1]) * a.yscl} {
			l.Push(lua.LNumber(v))
		}
		return 4
	})
	luaRegister(l, "omugenResetControllers", func(l *lua.LState) int {
		/*Forget which controller each player has: the next presses hand them out again, in order.
		@function omugenResetControllers*/
		mugenResetPads()
		return 0
	})
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
