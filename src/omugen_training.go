package main

// OMugen training tools, set from the training pause menu (menu.lua) and
// drawn by omugen_training.lua:
//   - hitboxes, without the debug mode;
//   - the input display: what each player holds, frame by frame;
//   - frame data: a move's startup, active and recovery frames, and who
//     can act first once it hit or was blocked (frame advantage);
//   - the dummy's recording: P1's controller plays the dummy for up to 10
//     seconds, then the dummy plays it back in a loop.

import (
	lua "github.com/yuin/gopher-lua"
)

const (
	trainHistoryLen = 18
	trainRecordMax  = 600 // ticks: 10 seconds
	trainPlayGap    = 30  // ticks between two playbacks
	trainKnockdown  = 180 // ticks the defender may take to act again before it counts as a knockdown
)

// trainInput is one line of the input display.
type trainInput struct {
	dir     int8   // numpad notation, forward to the right: 5 neutral, 6 forward, 2 down...
	buttons uint16 // a b c x y z s d w (bits 0-8)
	frames  int32
}

// trainFrames is the frame data of a player's last move.
type trainFrames struct {
	valid                    bool
	startup, active, recover int32
	contact                  int8 // 0 whiffed, 1 hit, 2 blocked
	adv                      int32
	knockdown                bool
	age                      int32 // ticks since it was measured
}

// trainMove measures a move as it runs.
type trainMove struct {
	measuring, wasAttacking   bool
	t, startup, active, last  int32
	contact                   int8
	attackerFree, defenderHit bool
	attackerFreeTick          int32
	defenderFreeTick          int32
	waited                    int32
}

var omugenTrain struct {
	hitboxes  bool
	inputs    int // 0 off, 1 P1, 2 P1 and P2
	frameData bool
	playback  bool

	history [2][]trainInput
	moves   [2]trainMove
	frames  [2]trainFrames
	tick    int32

	recording  bool
	recordLeft int32
	rec        []uint16 // per tick: U D B F a b c x y z s d w m (bits 0-13), back and forward as the dummy faced
	playPos    int
	playGap    int32
}

// omugenTrainActive reports a training match played here (not online, not a
// replay).
func omugenTrainActive() bool {
	return sys.gameMode == "training" && sys.netConnection == nil && sys.replayFile == nil &&
		sys.rollback.session == nil
}

func trainRoot(pn int) *Char {
	if pn < 0 || pn >= len(sys.chars) || len(sys.chars[pn]) == 0 {
		return nil
	}
	return sys.chars[pn][0]
}

// trainPack stores held buttons with back and forward instead of left and
// right, so the recording plays the same from either side.
func trainPack(b [14]bool, flip bool) uint16 {
	var v uint16
	for i, on := range b {
		if on {
			v |= 1 << uint(i)
		}
	}
	if flip {
		v = trainSwapLR(v)
	}
	return v
}

func trainUnpack(v uint16, flip bool) (b [14]bool) {
	if flip {
		v = trainSwapLR(v)
	}
	for i := range b {
		b[i] = v&(1<<uint(i)) != 0
	}
	return
}

func trainSwapLR(v uint16) uint16 {
	l, r := v&4, v&8
	return v&^12 | l<<1 | r>>1
}

// omugenTrainButtons gives the inputs of a root character that the training
// tools drive (false when its own controller does): while recording, the
// dummy is played with P1's controller and P1 stands still; on playback, the
// dummy plays the recording.
func omugenTrainButtons(cl *CommandList, c *Char, out *[14]bool) bool {
	t := &omugenTrain
	if c == nil || c.helperIndex != 0 || !(t.recording || t.playback) || !omugenTrainActive() {
		return false
	}
	switch {
	case t.recording:
		switch c.playerNo {
		case 1:
			p1 := trainRoot(0)
			if p1 == nil || p1.controller < 0 || p1.controller >= len(sys.inputRemap) {
				return false
			}
			*out = cl.Buffer.InputReader.LocalInput(sys.inputRemap[p1.controller])
			t.rec = append(t.rec, trainPack(*out, c.fbFlip))
			if t.recordLeft--; t.recordLeft <= 0 {
				t.recording = false
				t.playPos, t.playGap = 0, 0
			}
			return true
		case 0:
			*out = [14]bool{}
			return true
		}
	case t.playback && len(t.rec) > 0 && c.playerNo == 1:
		if t.playGap > 0 {
			t.playGap--
			*out = [14]bool{}
			return true
		}
		if t.playPos >= len(t.rec) {
			t.playPos = 0
		}
		*out = trainUnpack(t.rec[t.playPos], c.fbFlip)
		if t.playPos++; t.playPos >= len(t.rec) {
			t.playPos, t.playGap = 0, trainPlayGap
		}
		return true
	}
	return false
}

// omugenTrainInputs notes what a root character holds this tick (the input
// display).
func omugenTrainInputs(c *Char, U, D, B, F, a, b, cc, x, y, z, s, d, w bool) {
	t := &omugenTrain
	if t.inputs == 0 || c == nil || c.helperIndex != 0 || c.playerNo > 1 || !omugenTrainActive() {
		return
	}
	dir := int8(5)
	switch {
	case U:
		dir = 8
	case D:
		dir = 2
	}
	switch {
	case B:
		dir--
	case F:
		dir++
	}
	var btn uint16
	for i, on := range []bool{a, b, cc, x, y, z, s, d, w} {
		if on {
			btn |= 1 << uint(i)
		}
	}
	h := t.history[c.playerNo]
	if n := len(h); n > 0 && h[n-1].dir == dir && h[n-1].buttons == btn {
		if h[n-1].frames < 999 {
			h[n-1].frames++
		}
		return
	}
	h = append(h, trainInput{dir: dir, buttons: btn, frames: 1})
	if len(h) > trainHistoryLen {
		h = append(h[:0], h[len(h)-trainHistoryLen:]...)
	}
	t.history[c.playerNo] = h
}

// omugenTrainTick measures moves, once per game tick after the characters
// acted.
func omugenTrainTick() {
	t := &omugenTrain
	if !omugenTrainActive() {
		return
	}
	t.tick++
	for pn := 0; pn < 2; pn++ {
		if t.frames[pn].valid {
			t.frames[pn].age++
		}
		if t.frameData {
			t.moves[pn].step(trainRoot(pn), trainRoot(1-pn), &t.frames[pn], t.tick)
		}
	}
}

// trainFrozen reports a character held still this tick: its own hit pause,
// or a pause it doesn't move during.
func trainFrozen(c *Char) bool {
	return c.hitPause() || sys.supertime > 0 && c.superMovetime == 0 ||
		sys.pausetime > 0 && c.pauseMovetime == 0
}

// trainHitbox reports a hitbox that can hit this tick.
func trainHitbox(c *Char) bool {
	return c.atktmp != 0 && c.hitdef.attr > 0 && len(c.getClsn(1)) > 0
}

func trainHelperHitbox(pn int) bool {
	if pn < 0 || pn >= len(sys.chars) {
		return false
	}
	for _, h := range sys.chars[pn] {
		if h != nil && h.helperIndex != 0 && trainHitbox(h) {
			return true
		}
	}
	return false
}

// trainFree reports a character that can act.
func trainFree(c *Char) bool {
	return c.ctrl() && c.ss.moveType != MT_H
}

// step follows a move: startup up to its first active hitbox, the active
// frames, the recovery up to control again; after a hit or a block, the
// tick each side can act again.
func (m *trainMove) step(c, d *Char, out *trainFrames, tick int32) {
	if c == nil || d == nil {
		return
	}
	attacking := c.ss.moveType == MT_A
	if !m.measuring {
		if attacking && !m.wasAttacking {
			*m = trainMove{measuring: true}
		}
		m.wasAttacking = attacking
		if !m.measuring {
			return
		}
	}
	m.wasAttacking = attacking
	if c.ss.moveType == MT_H && !m.attackerFree {
		m.measuring = false // hit out of the move
		return
	}
	if !trainFrozen(c) && !m.attackerFree {
		m.t++
		// Active: a hitbox that can hit, the character's or a helper's
		// (projectile-like moves); once it hit, the character's boxes still
		// count.
		hitbox := trainHitbox(c) || trainHelperHitbox(c.playerNo)
		if m.startup == 0 && hitbox {
			m.startup = m.t
		}
		if m.startup > 0 && (hitbox || len(c.getClsn(1)) > 0) {
			m.active++
			m.last = m.t
		}
	}
	if m.contact == 0 {
		if c.moveHit() > 0 {
			m.contact = 1
		} else if c.moveGuarded() > 0 {
			m.contact = 2
		}
	}
	if m.contact != 0 && d.ss.moveType == MT_H {
		m.defenderHit = true
	}
	if !m.attackerFree && m.t > 0 && trainFree(c) {
		m.attackerFree, m.attackerFreeTick = true, tick
	}
	if m.defenderHit && m.defenderFreeTick == 0 && trainFree(d) {
		m.defenderFreeTick = tick
	}
	if !m.attackerFree {
		if m.t > 600 {
			m.measuring = false // a stance or a held move: no data
		}
		return
	}
	res := trainFrames{valid: true, startup: m.startup, active: m.active, contact: m.contact}
	if m.startup > 0 {
		res.recover = m.t - m.last - 1
		if res.recover < 0 {
			res.recover = 0
		}
	} else {
		res.recover = m.t - 1
	}
	if m.contact != 0 {
		if m.defenderFreeTick == 0 {
			if m.waited++; m.waited < trainKnockdown {
				return // the defender still can't act
			}
			res.knockdown = true
		} else {
			res.adv = m.defenderFreeTick - m.attackerFreeTick
		}
	}
	*out = res
	m.measuring = false
}

func omugenTrainingScriptInit(l *lua.LState) {
	luaRegister(l, "omugenTrainingSet", func(l *lua.LState) int {
		/*Sets a training tool: "hitboxes" (boolean), "inputs" (0 off, 1 P1,
		2 P1 and P2), "framedata" (boolean), "playback" (boolean).
		@function omugenTrainingSet
		@tparam string name
		@param value*/
		t := &omugenTrain
		switch strArg(l, 1) {
		case "hitboxes":
			t.hitboxes = boolArg(l, 2)
			sys.clsnDisplay = t.hitboxes
		case "inputs":
			t.inputs = int(numArg(l, 2))
			if t.inputs == 0 {
				t.history = [2][]trainInput{}
			}
		case "framedata":
			t.frameData = boolArg(l, 2)
			t.moves, t.frames = [2]trainMove{}, [2]trainFrames{}
		case "playback":
			t.playback = boolArg(l, 2)
			t.playPos, t.playGap = 0, 0
		}
		return 0
	})
	luaRegister(l, "omugenTrainingApply", func(l *lua.LState) int {
		/*Applies the training tools at a round's start (hitboxes, fresh input display).
		@function omugenTrainingApply*/
		t := &omugenTrain
		if t.hitboxes {
			sys.clsnDisplay = true
		}
		t.history = [2][]trainInput{}
		t.moves, t.frames = [2]trainMove{}, [2]trainFrames{}
		return 0
	})
	luaRegister(l, "omugenTrainingReset", func(l *lua.LState) int {
		/*Forgets the dummy's recording (a new training match).
		@function omugenTrainingReset*/
		t := &omugenTrain
		t.recording, t.playback, t.rec = false, false, nil
		t.history = [2][]trainInput{}
		t.moves, t.frames = [2]trainMove{}, [2]trainFrames{}
		return 0
	})
	luaRegister(l, "omugenTrainingRecord", func(l *lua.LState) int {
		/*Starts recording the dummy, played with P1's controller (up to 10 seconds).
		@function omugenTrainingRecord*/
		t := &omugenTrain
		t.recording, t.recordLeft, t.rec = true, trainRecordMax, nil
		t.playPos, t.playGap = 0, 0
		return 0
	})
	luaRegister(l, "omugenTrainingStop", func(l *lua.LState) int {
		/*Stops recording the dummy.
		@function omugenTrainingStop*/
		omugenTrain.recording = false
		return 0
	})
	luaRegister(l, "omugenTrainingInfo", func(l *lua.LState) int {
		/*What the training tools show.
		@function omugenTrainingInfo
		@treturn table info `recording`, `recordLeft` (seconds), `recorded` (seconds),
		  `playback`, `inputs` (0-2), `framedata`, and per player `[1]`, `[2]`:
		  `history` (newest first: `dir` 1-9, `buttons` ("abc..."), `frames`) and
		  `frames` (`startup`, `active`, `recovery`, `contact` ("hit", "block", ""),
		  `adv`, `knockdown`, `age` (ticks)) when a move was measured.*/
		t := &omugenTrain
		tbl := l.NewTable()
		tbl.RawSetString("recording", lua.LBool(t.recording))
		tbl.RawSetString("recordLeft", lua.LNumber(float64(t.recordLeft)/60))
		tbl.RawSetString("recorded", lua.LNumber(float64(len(t.rec))/60))
		tbl.RawSetString("playback", lua.LBool(t.playback && len(t.rec) > 0 && !t.recording))
		tbl.RawSetString("inputs", lua.LNumber(t.inputs))
		tbl.RawSetString("framedata", lua.LBool(t.frameData))
		for pn := 0; pn < 2; pn++ {
			p := l.NewTable()
			hist := l.NewTable()
			h := t.history[pn]
			for i := len(h) - 1; i >= 0; i-- {
				e := l.NewTable()
				e.RawSetString("dir", lua.LNumber(h[i].dir))
				var s []byte
				for j, name := range "abcxyzsdw" {
					if h[i].buttons&(1<<uint(j)) != 0 {
						s = append(s, byte(name))
					}
				}
				e.RawSetString("buttons", lua.LString(s))
				e.RawSetString("frames", lua.LNumber(h[i].frames))
				hist.Append(e)
			}
			p.RawSetString("history", hist)
			if f := t.frames[pn]; f.valid {
				fr := l.NewTable()
				fr.RawSetString("startup", lua.LNumber(f.startup))
				fr.RawSetString("active", lua.LNumber(f.active))
				fr.RawSetString("recovery", lua.LNumber(f.recover))
				fr.RawSetString("contact", lua.LString([]string{"", "hit", "block"}[f.contact]))
				fr.RawSetString("adv", lua.LNumber(f.adv))
				fr.RawSetString("knockdown", lua.LBool(f.knockdown))
				fr.RawSetString("age", lua.LNumber(f.age))
				p.RawSetString("frames", fr)
			}
			tbl.RawSetInt(pn+1, p)
		}
		l.Push(tbl)
		return 1
	})
}
