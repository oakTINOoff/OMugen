package main

import (
	"testing"

	"github.com/veandco/go-sdl2/sdl"
)

// A 1920x1080 window on a laptop at 125% scaling (a 1536x864 desktop, with
// the taskbar 1536x824 usable) shrinks to fit with its frame, same shape.
func TestFitRect(t *testing.T) {
	area := sdl.Rect{X: 0, Y: 0, W: 1536, H: 824}
	w, h, x, y, ok := fitRect(1920, 1080, area, 8, 31, 8, 8)
	if !ok {
		t.Fatal("a 1920x1080 window fits a 1536x824 screen?")
	}
	if w+16 > area.W || h+39 > area.H {
		t.Errorf("%dx%d plus its frame doesn't fit %dx%d", w, h, area.W, area.H)
	}
	if r := float64(w) / float64(h); r < 1.77 || r > 1.78 {
		t.Errorf("%dx%d isn't 16:9", w, h)
	}
	if x-8 < 0 || y-31 < 0 || x+w+8 > area.W || y+h+8 > area.H {
		t.Errorf("at %d,%d the window leaves the screen", x, y)
	}
	// A window that fits stays as it is.
	if _, _, _, _, ok := fitRect(1280, 720, area, 8, 31, 8, 8); ok {
		t.Error("a 1280x720 window was resized")
	}
	// The usable area can start away from 0,0 (a second screen, a taskbar on top).
	_, _, x, y, _ = fitRect(1920, 1080, sdl.Rect{X: 1920, Y: 40, W: 1536, H: 824}, 8, 31, 8, 8)
	if x < 1920+8 || y < 40+31 {
		t.Errorf("placed at %d,%d, outside the second screen's usable area", x, y)
	}
}
