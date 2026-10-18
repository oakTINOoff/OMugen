package main

// Native resolution: OMugen draws each frame at the size of the window's
// picture, in the screen's own pixels, so it reaches the screen 1:1 (sharp).
// MUGEN and the upstream engine draw at a fixed resolution (Video.GameWidth x
// GameHeight) and stretch the result to the window, which blurs or, with
// nearest pixels at uneven factors, makes it jagged.
//
// The game's resolution still sets the picture's shape (16:9, 4:3) and the
// window's size; all drawing is in relative units, so only the sharpness
// changes. The frame follows the window: resizing it, Alt+Enter, another
// monitor. Online play, rollback and replays draw at the game's resolution
// (the camera rounds to its pixels, and both sides must match). The OpenGL
// 3.3 renderer does it; the others keep the fixed resolution, scaled.

import (
	"math"
	"runtime"
)

// nativeRenderAllowed reports whether frames follow the window's size now.
func (s *System) nativeRenderAllowed() bool {
	return s.cfg.Video.NativeResolution && runtime.GOOS != "android" && gfx != nil &&
		gfx.GetName() == "OpenGL 3.3" && s.window != nil &&
		s.netConnection == nil && s.rollback.session == nil && s.replayFile == nil
}

// renderSize is the size to draw frames at, and whether it is the window's
// own (else the game's resolution).
func (s *System) renderSize() (w, h int32, native bool) {
	gw, gh := s.cfg.Video.GameWidth, s.cfg.Video.GameHeight
	if gw <= 0 || gh <= 0 {
		gw, gh = s.scrrect[2], s.scrrect[3]
	}
	if !s.nativeRenderAllowed() {
		return gw, gh, false
	}
	dw, dh := s.window.drawableSize()
	if dw < 64 || dh < 64 { // minimized
		return s.scrrect[2], s.scrrect[3], s.nativeRender
	}
	w, h = fitAspect(dw, dh, gw, gh)
	// Keep Aspect Ratio off stretches the frame to the window. When that is a
	// few pixels (852x480 in a 16:9 window), draw at the window's size: a
	// 1:1 copy instead of resampling for an invisible stretch.
	if !s.cfg.Video.KeepAspect && dw-w <= 4 && dh-h <= 4 {
		w, h = dw, dh
	}
	return w, h, true
}

// fitAspect is the largest size with the shape aw:ah that fits availW x availH.
func fitAspect(availW, availH, aw, ah int32) (w, h int32) {
	if aw <= 0 || ah <= 0 {
		return availW, availH
	}
	if int64(availW)*int64(ah) > int64(availH)*int64(aw) {
		h = availH
		w = int32(math.Round(float64(availH) * float64(aw) / float64(ah)))
	} else {
		w = availW
		h = int32(math.Round(float64(availW) * float64(ah) / float64(aw)))
	}
	return Min(w, availW), Min(h, availH)
}

// updateRenderSize makes the frame follow the window, before a frame starts.
// The aspect state in use (menus, or a fight's own aspect) keeps its shape;
// its scale follows the new size.
func (s *System) updateRenderSize() {
	w, h, native := s.renderSize()
	s.nativeRender = native
	if w == s.scrrect[2] && h == s.scrrect[3] {
		return
	}
	s.scrrect[2], s.scrrect[3] = w, h
	if s.gameWidth > 0 && s.gameHeight > 0 {
		s.widthScale = float32(w) / s.gameWidth
		s.heightScale = float32(h) / s.gameHeight
	}
	gfx.Resize()
	LogMessage("OMugen: drawing at %dx%d", w, h)
}

// beginFrame starts a frame at the size the window calls for.
func (s *System) beginFrame(clearColor bool) {
	s.updateRenderSize()
	gfx.BeginFrame(clearColor)
}
