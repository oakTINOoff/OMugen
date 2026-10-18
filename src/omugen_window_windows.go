//go:build windows

package main

import "syscall"

var getSystemMetrics = syscall.NewLazyDLL("user32.dll").NewProc("GetSystemMetrics")

func systemMetric(index uintptr) int32 {
	r, _, _ := getSystemMetrics.Call(index)
	return int32(r)
}

// omugenWindowFrame is the size of a window's frame on each side: the title
// bar and the resize borders, in the same units as SDL's window sizes.
func omugenWindowFrame() (left, top, right, bottom int32) {
	const smCyCaption, smCxFrame, smCyFrame, smCxPaddedBorder = 4, 32, 33, 92
	side := systemMetric(smCxFrame) + systemMetric(smCxPaddedBorder)
	edge := systemMetric(smCyFrame) + systemMetric(smCxPaddedBorder)
	return side, systemMetric(smCyCaption) + edge, side, edge
}
