//go:build !windows

package main

// omugenWindowFrame is the size of a window's frame on each side. Window
// managers draw their own: room for a typical title bar.
func omugenWindowFrame() (left, top, right, bottom int32) {
	return 0, 40, 0, 0
}
