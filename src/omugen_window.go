package main

import (
	"math"

	"github.com/veandco/go-sdl2/sdl"
)

// omugenFitWindow shrinks a window whose client area (w x h) doesn't fit on
// its screen with its frame to the largest size of the same shape that does,
// centered in the screen's usable area (the desktop without the taskbar). A
// 1920x1080 game on a laptop whose Windows scaling (125%) makes the desktop
// 1536x864 would otherwise hang off the screen's edges. It returns the new
// size and position; ok is false when the window already fits.
func omugenFitWindow(win *sdl.Window, w, h int32, bordered bool) (nw, nh, nx, ny int32, ok bool) {
	if win == nil || w <= 0 || h <= 0 {
		return
	}
	d, err := win.GetDisplayIndex()
	if err != nil {
		d = 0
	}
	ub, err := sdl.GetDisplayUsableBounds(d)
	if err != nil || ub.W <= 0 || ub.H <= 0 {
		return
	}
	var left, top, right, bottom int32
	if bordered {
		left, top, right, bottom = omugenWindowFrame(uint32(math.Round(96 * omugenDisplayScale(d))))
	}
	nw, nh, nx, ny, ok = fitRect(w, h, ub, left, top, right, bottom)
	if !ok {
		return
	}
	win.SetSize(nw, nh)
	win.SetPosition(nx, ny)
	LogMessage("OMugen: the %dx%d window doesn't fit on the screen (%dx%d usable): %dx%d instead",
		w, h, ub.W, ub.H, nw, nh)
	return
}

// fitRect is the client size and position that fit w x h, with a frame of
// the given sizes, into the area: the same shape, centered. ok is false
// when it fits as it is.
func fitRect(w, h int32, area sdl.Rect, left, top, right, bottom int32) (nw, nh, nx, ny int32, ok bool) {
	availW, availH := area.W-left-right, area.H-top-bottom
	if availW <= 0 || availH <= 0 || (w <= availW && h <= availH) {
		return 0, 0, 0, 0, false
	}
	s := math.Min(float64(availW)/float64(w), float64(availH)/float64(h))
	nw, nh = int32(float64(w)*s), int32(float64(h)*s)
	nx = area.X + left + (availW-nw)/2
	ny = area.Y + top + (availH-nh)/2
	return nw, nh, nx, ny, true
}

// omugenScaledWindow is a window size in a screen's pixels: the configured
// size times the screen's display scaling, so a window looks as big as on an
// unscaled screen, drawn at the screen's own resolution.
func omugenScaledWindow(w, h int32, display int) (int32, int32) {
	s := omugenDisplayScale(display)
	if s < 1.01 {
		return w, h
	}
	sw, sh := int32(math.Round(float64(w)*s)), int32(math.Round(float64(h)*s))
	LogMessage("OMugen: display scaling %.0f%%: the %dx%d window is %dx%d pixels", s*100, w, h, sw, sh)
	return sw, sh
}
