//go:build !android

package main

import "testing"

// The frame takes the game resolution's shape, as big as the window allows.
func TestFitAspect(t *testing.T) {
	for _, c := range []struct {
		availW, availH, aw, ah, w, h int32
	}{
		{1729, 973, 1280, 720, 1729, 973},   // 16:9 in a 16:9 window: all of it
		{1920, 1080, 852, 480, 1917, 1080},  // a slightly narrower game: bars of 1-2 pixels
		{2560, 1080, 1280, 720, 1920, 1080}, // ultrawide: pillarbox
		{1280, 1024, 1280, 720, 1280, 720},  // 5:4: letterbox
		{1600, 1200, 640, 480, 1600, 1200},  // 4:3 in 4:3
	} {
		w, h := fitAspect(c.availW, c.availH, c.aw, c.ah)
		if w != c.w || h != c.h {
			t.Errorf("fit %dx%d in %dx%d: %dx%d, want %dx%d", c.aw, c.ah, c.availW, c.availH, w, h, c.w, c.h)
		}
		if w > c.availW || h > c.availH {
			t.Errorf("%dx%d doesn't fit %dx%d", w, h, c.availW, c.availH)
		}
	}
}

// A frame the picture's size is copied; otherwise sharp bilinear (Nearest)
// or bicubic (Linear, and shrinking); whole factors stay nearest pixels.
func TestFinalScaleMode(t *testing.T) {
	for _, c := range []struct {
		srcW, srcH, dstW, dstH int32
		linear                 bool
		want                   int32
	}{
		{1729, 973, 1729, 973, false, 0},
		{1729, 973, 1729, 973, true, 0},
		{640, 360, 1280, 720, false, 0}, // 2x: nearest is exact
		{852, 480, 1727, 973, false, 1}, // uneven upscale, Nearest
		{852, 480, 1727, 973, true, 2},  // uneven upscale, Linear
		{1920, 1080, 1729, 973, false, 2},
		{1920, 1080, 960, 540, false, 2}, // shrinking: bicubic either way
	} {
		if got := finalScaleMode(c.srcW, c.srcH, c.dstW, c.dstH, c.linear); got != c.want {
			t.Errorf("%dx%d to %dx%d (linear %v): mode %d, want %d", c.srcW, c.srcH, c.dstW, c.dstH, c.linear, got, c.want)
		}
	}
}
