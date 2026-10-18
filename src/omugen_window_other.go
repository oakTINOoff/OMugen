//go:build !windows

package main

// omugenBeforeVideoInit: display scaling is the window manager's business here.
func omugenBeforeVideoInit() {}

// omugenDisplayScale: windows are sized in the desktop's own units.
func omugenDisplayScale(display int) float64 { return 1 }

// omugenWindowFrame is the size of a window's frame on each side. Window
// managers draw their own: room for a typical title bar.
func omugenWindowFrame(dpi uint32) (left, top, right, bottom int32) {
	return 0, 40, 0, 0
}
