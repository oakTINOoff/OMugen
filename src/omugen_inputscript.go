package main

// OMugen: -inputscript <file> plays scripted keyboard input into the engine,
// frame by frame, for automated testing of menus and screenpacks. It never
// touches the real keyboard or other windows. Commands, one per line:
//
//	wait N          wait N frames (in a match: N game ticks, at any frame rate)
//	sleep MS        wait MS milliseconds (loading screens render faster than 60 fps)
//	waitmatch       wait until a match is running (then waits count its ticks)
//	tap KEY [N]     hold KEY for N frames (default 4), then release it
//	padtap P B [N]  hold button B (0-13) of simulated pad P (1-4, OMUGEN_FAKEPADS) for N frames
//	down KEY        press KEY (until "up KEY")
//	up KEY          release KEY
//	shot NAME       save a screenshot to save/shots/NAME.png
//	quit            close the game
//
// KEY uses OMugen key names (RETURN, ESCAPE, UP, DOWN, LEFT, RIGHT, a, z, F1...).
// Lines starting with # are comments.

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/oakTINOoff/OMugen/src/mugenjoy"
)

type omugenInputCmd struct {
	op, arg string
	n       int
	button  int // padtap
}

var omugenInput struct {
	once     sync.Once
	cmds     []omugenInputCmd
	pc       int
	wait     int
	until    time.Time
	releases map[Key]int // key -> frames left before release
	padUp    map[int]int // simulated pad -> frames left before its button is released
	lastTick int
}

func omugenInputScriptLoad() {
	path := sys.cmdFlags["-inputscript"]
	if path == "" {
		return
	}
	text, err := os.ReadFile(path)
	if err != nil {
		LogMessage("OMugen: input script: %v", err)
		return
	}
	for _, line := range strings.Split(strings.ReplaceAll(string(text), "\r\n", "\n"), "\n") {
		f := strings.Fields(strings.SplitN(line, "#", 2)[0])
		if len(f) == 0 {
			continue
		}
		c := omugenInputCmd{op: strings.ToLower(f[0])}
		if len(f) > 1 {
			c.arg = f[1]
		}
		if (c.op == "tap" || c.op == "down" || c.op == "up") && StringToKey(c.arg) == KeyUnknown {
			LogMessage("OMugen: input script: unknown key %q", c.arg)
		}
		c.n = 4
		if c.op == "wait" && len(f) > 1 {
			c.n = int(Atoi(f[1]))
		} else if c.op == "padtap" {
			if len(f) > 2 {
				c.button = int(Atoi(f[2]))
			}
			if len(f) > 3 {
				c.n = int(Atoi(f[3]))
			}
		} else if len(f) > 2 {
			c.n = int(Atoi(f[2]))
		}
		omugenInput.cmds = append(omugenInput.cmds, c)
	}
	omugenInput.releases = map[Key]int{}
	omugenInput.padUp = map[int]int{}
	LogMessage("OMugen: input script %s: %d commands", path, len(omugenInput.cmds))
}

// omugenInputScriptTick runs once per frame, after real window events.
func omugenInputScriptTick() {
	omugenInput.once.Do(omugenInputScriptLoad)
	if omugenInput.releases == nil {
		return
	}
	// In a match, count game ticks, not frames, so a script plays the same at
	// any frame rate.
	if sys.gameRunning && !sys.matchPaused() {
		if sys.tickCount == omugenInput.lastTick {
			return
		}
		omugenInput.lastTick = sys.tickCount
	}
	for id, left := range omugenInput.padUp {
		if left <= 1 {
			mugenJoy.SetFakeInput(id, mugenjoy.Snapshot{})
			delete(omugenInput.padUp, id)
		} else {
			omugenInput.padUp[id] = left - 1
		}
	}
	for k, left := range omugenInput.releases {
		if left <= 1 {
			OnKeyReleased(k, 0)
			delete(omugenInput.releases, k)
		} else {
			omugenInput.releases[k] = left - 1
		}
	}
	if omugenInput.wait > 0 {
		omugenInput.wait--
		return
	}
	if time.Now().Before(omugenInput.until) {
		return
	}
	for omugenInput.pc < len(omugenInput.cmds) {
		c := omugenInput.cmds[omugenInput.pc]
		omugenInput.pc++
		key := StringToKey(c.arg)
		switch c.op {
		case "wait":
			omugenInput.wait = c.n
			return
		case "waitmatch":
			if !sys.gameRunning {
				omugenInput.pc-- // until a match is running
				return
			}
		case "sleep":
			omugenInput.until = time.Now().Add(time.Duration(Atoi(c.arg)) * time.Millisecond)
			return
		case "tap":
			OnKeyPressed(key, 0)
			omugenInput.releases[key] = c.n
			return // one tap per frame, like a person
		case "padtap":
			// press a button of a simulated pad (OMUGEN_FAKEPADS)
			if id := int(Atoi(c.arg)) - 1; mugenJoy != nil && os.Getenv("OMUGEN_FAKEPADS") != "" && id >= 0 {
				mugenJoy.SetFakeInput(id, mugenjoy.Snapshot{Buttons: 1 << uint(c.button)})
				omugenInput.padUp[id] = c.n
			}
			return
		case "down":
			OnKeyPressed(key, 0)
		case "up":
			OnKeyReleased(key, 0)
		case "shot":
			_ = os.MkdirAll(filepath.Join("save", "shots"), 0o755)
			sys.isTakingScreenshot = true
			sys.screenshotName = filepath.Join("save", "shots", c.arg+".png")
			return
		case "quit":
			sys.window.closeflag = true
			return
		default:
			LogMessage("OMugen: input script: unknown command %q", c.op)
		}
	}
}
