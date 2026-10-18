//go:build windows

package main

import (
	"os"
	"syscall"

	"github.com/veandco/go-sdl2/sdl"
)

var (
	user32                 = syscall.NewLazyDLL("user32.dll")
	getSystemMetrics       = user32.NewProc("GetSystemMetrics")
	getSystemMetricsForDpi = user32.NewProc("GetSystemMetricsForDpi") // Windows 10 1607+
)

// omugenBeforeVideoInit makes OMugen aware of Windows' display scaling, so a
// screen at 125% or 150% shows it at its own resolution (sharp) instead of
// stretching a smaller picture. OMUGEN_DPI_UNAWARE=1 lets Windows stretch it.
func omugenBeforeVideoInit() {
	if os.Getenv("OMUGEN_DPI_UNAWARE") == "" {
		sdl.SetHint("SDL_WINDOWS_DPI_AWARENESS", "permonitorv2")
	}
}

// omugenDisplayScale is a screen's display scaling (1.25 at 125%), 1 when
// Windows stretches OMugen itself.
func omugenDisplayScale(display int) float64 {
	_, h, _, err := sdl.GetDisplayDPI(display)
	if err != nil || h < 48 || h > 960 {
		return 1
	}
	return float64(h) / 96
}

func systemMetric(index uintptr, dpi uint32) int32 {
	if dpi > 0 && getSystemMetricsForDpi.Find() == nil {
		r, _, _ := getSystemMetricsForDpi.Call(index, uintptr(dpi))
		return int32(r)
	}
	r, _, _ := getSystemMetrics.Call(index)
	return int32(r)
}

// omugenWindowFrame is the size of a window's frame on each side (the title
// bar and the resize borders) on a screen of that DPI (0: unknown), in the
// same units as SDL's window sizes.
func omugenWindowFrame(dpi uint32) (left, top, right, bottom int32) {
	const smCyCaption, smCxFrame, smCyFrame, smCxPaddedBorder = 4, 32, 33, 92
	side := systemMetric(smCxFrame, dpi) + systemMetric(smCxPaddedBorder, dpi)
	edge := systemMetric(smCyFrame, dpi) + systemMetric(smCxPaddedBorder, dpi)
	return side, systemMetric(smCyCaption, dpi) + edge, side, edge
}
