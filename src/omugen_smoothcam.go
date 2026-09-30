package main

// Smooth camera for frame rates above the game's 60 ticks per second.
//
// The camera's game logic runs once per game tick, as in MUGEN: its position
// sets the screen edges that characters and projectiles are bound to, so it
// must not depend on the frame rate. With more frames than ticks, the frames
// between two ticks are drawn with a camera blended from the previous tick's
// camera toward the camera of the tick being shown, which is worked out ahead
// on a scratch copy from the characters' new game positions. Characters are
// blended between their old and new positions the same way (interPos), so
// they and the camera stay in step.

import (
	"fmt"
	"math"
	"os"
	"time"

	"github.com/veandco/go-sdl2/sdl"
)

var camSmooth struct {
	valid    bool    // from/to are set for the tick being drawn
	from, to Camera  // camera of the previous tick, and of this tick (ahead of time)
	show     bool    // draw this frame with display
	frac     float32 // blend factor of this frame (1: the game tick itself)
	display  Camera
}

// trackLogicPositions sets the camera's tracking bounds from the characters'
// game positions.
func (cl *CharList) trackLogicPositions() {
	for _, c := range cl.runOrder {
		if !c.scf(SCF_disabled) {
			c.trackCamera(true)
		}
	}
}

// camSmoothPrepare runs on the first frame of a tick that spans several
// frames, after the characters have moved: it works out this tick's camera
// on a copy, leaving the game's camera as it is until the tick's last frame.
func (s *System) camSmoothPrepare(x, y, scl float32, pause bool) {
	camSmooth.valid = false
	if !s.cfg.Config.TickInterpolation {
		return
	}
	saved := s.cam
	s.cam.ResetTracking()
	s.charList.trackLogicPositions()
	nx, ny, nscl := s.cam.action(x, y, scl, pause)
	if !s.cam.zoomEnabled() {
		nx = float32(math.Ceil(float64(nx)*4-0.5) / 4)
	}
	s.cam.Update(nscl, nx, ny)
	camSmooth.to = s.cam
	s.cam = saved
	camSmooth.from = saved
	camSmooth.valid = true
}

// camSmoothFrame picks the camera to draw this frame with. It uses the same
// blend factor as the characters, taken at the same time.
func (s *System) camSmoothFrame() {
	camSmooth.show, camSmooth.frac = false, 1
	if !camSmooth.valid {
		return
	}
	f := s.tickInterpolation()
	if f >= 1 {
		return
	}
	from, to := &camSmooth.from, &camSmooth.to
	d := *to
	scl := from.Scale/from.BaseScale() + (to.Scale/to.BaseScale()-from.Scale/from.BaseScale())*f
	x := from.Pos[0] + (to.Pos[0]-from.Pos[0])*f
	y := from.Pos[1] + (to.Pos[1]-from.Pos[1])*f
	d.Update(scl, x, y)
	camSmooth.display = d
	camSmooth.show, camSmooth.frac = true, f
}

// camSmoothBegin swaps in the camera to draw this frame with; the returned
// function puts the game's camera back.
func (s *System) camSmoothBegin() func() {
	if !camSmooth.show {
		return func() {}
	}
	saved := s.cam
	s.cam = camSmooth.display
	return func() { s.cam = saved }
}

// OMUGEN_TICKLOG=file logs the game state after every game tick of a match
// (positions, life, camera), to check that it doesn't depend on the frame
// rate: the same input script must give the same log at 60 and 240 fps.
var tickLog struct {
	f    *os.File
	tick int
}

func omugenTickLog() {
	if tickLog.f == nil {
		p := os.Getenv("OMUGEN_TICKLOG")
		if p == "" {
			return
		}
		f, err := os.Create(p)
		if err != nil {
			return
		}
		tickLog.f = f
	}
	tickLog.tick++
	if tickLog.tick%60 == 0 {
		fmt.Fprintf(tickLog.f, "# %.0f fps\n", sys.gameFPS)
	}
	line := fmt.Sprintf("%d r%d t%d cam %.3f %.3f %.4f", tickLog.tick, sys.roundNo, sys.curRoundTime,
		sys.cam.Pos[0], sys.cam.Pos[1], sys.cam.Scale)
	for i := 0; i < 2 && i < len(sys.chars); i++ {
		if len(sys.chars[i]) > 0 && sys.chars[i][0] != nil {
			c := sys.chars[i][0]
			line += fmt.Sprintf(" | p%d s%d %.3f %.3f l%d", i+1, c.ss.no, c.pos[0], c.pos[1], c.life)
		}
	}
	fmt.Fprintln(tickLog.f, line)
}

// smoothOffset is a stage element's own scroll offset (velocity, sine
// motion, BGCtrl) blended between game ticks like the camera. Big jumps,
// such as a BGCtrl moving the element back, are not blended.
func (bg *backGround) smoothOffset() [2]float32 {
	o := bg.bga.offset
	f := camSmooth.frac
	if f >= 1 {
		return o
	}
	for i := range o {
		if d := o[i] - bg.prevOffset[i]; d > -64 && d < 64 {
			o[i] = bg.prevOffset[i] + d*f
		}
	}
	return o
}

// monitorRefreshRate is the refresh rate of the display the game window is
// on (60 if unknown), used by Framerate = 0.
var monitorRate struct {
	hz      int
	checked time.Time
}

func monitorRefreshRate() int {
	// The window can move to another display: check again now and then.
	if monitorRate.hz > 0 && time.Since(monitorRate.checked) < 2*time.Second {
		return monitorRate.hz
	}
	hz := 60
	if sys.window != nil && sys.window.Window != nil {
		if i, err := sys.window.Window.GetDisplayIndex(); err == nil {
			if m, err := sdl.GetCurrentDisplayMode(i); err == nil && m.RefreshRate >= 30 {
				hz = int(m.RefreshRate)
			}
		}
	}
	monitorRate.hz, monitorRate.checked = hz, time.Now()
	return hz
}
