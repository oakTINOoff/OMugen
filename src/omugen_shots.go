package main

// OMugen: -shotticks "300,600,900" saves a screenshot at those match ticks
// to save/shots/tick_NNNNN.png. With -sessionseed it gives frame-exact,
// repeatable captures (graphics comparisons, bug reports).

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	omugenShotOnce  sync.Once
	omugenShotTicks map[int32]bool
)

func omugenShotTick(tick int32) {
	omugenShotOnce.Do(func() {
		v := sys.cmdFlags["-shotticks"]
		if v == "" {
			return
		}
		omugenShotTicks = map[int32]bool{}
		for _, f := range strings.Split(v, ",") {
			if n := Atoi(f); n > 0 {
				omugenShotTicks[n] = true
			}
		}
		_ = os.MkdirAll(filepath.Join("save", "shots"), 0o755)
	})
	if omugenShotTicks[tick] {
		sys.isTakingScreenshot = true
		sys.screenshotName = filepath.Join("save", "shots", fmt.Sprintf("tick_%05d.png", tick))
	}
}
