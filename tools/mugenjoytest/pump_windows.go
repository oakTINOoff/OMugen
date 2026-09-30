//go:build windows

package main

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32                  = windows.NewLazySystemDLL("user32.dll")
	kernel32                = windows.NewLazySystemDLL("kernel32.dll")
	procPeekMessageW        = user32.NewProc("PeekMessageW")
	procTranslateMsg        = user32.NewProc("TranslateMessage")
	procDispatchMsgW        = user32.NewProc("DispatchMessageW")
	procRegisterClassExW    = user32.NewProc("RegisterClassExW")
	procCreateWindowExW     = user32.NewProc("CreateWindowExW")
	procDefWindowProcW      = user32.NewProc("DefWindowProcW")
	procShowWindow          = user32.NewProc("ShowWindow")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procGetForegroundWindow = user32.NewProc("GetForegroundWindow")
	procGetModuleHandleW    = kernel32.NewProc("GetModuleHandleW")
)

type winMsg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      [2]int32
	private uint32
}

type wndClassEx struct {
	size       uint32
	style      uint32
	wndProc    uintptr
	clsExtra   int32
	wndExtra   int32
	instance   uintptr
	icon       uintptr
	cursor     uintptr
	background uintptr
	menuName   *uint16
	className  *uint16
	iconSm     uintptr
}

var testWindow uintptr

// openTestWindow creates a small window so this process can own the
// foreground, like MUGEN's game window does. Call from a locked OS thread.
func openTestWindow() {
	inst, _, _ := procGetModuleHandleW.Call(0)
	className, _ := windows.UTF16PtrFromString("mugenjoytest")
	title, _ := windows.UTF16PtrFromString("mugenjoytest - click here and keep this window focused")
	wc := wndClassEx{
		wndProc:    windows.NewCallback(func(h, m, w, l uintptr) uintptr { r, _, _ := procDefWindowProcW.Call(h, m, w, l); return r }),
		instance:   inst,
		background: 6, // COLOR_WINDOW+1
		className:  className,
	}
	wc.size = uint32(unsafe.Sizeof(wc))
	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	const wsOverlappedWindow = 0x00CF0000
	testWindow, _, _ = procCreateWindowExW.Call(0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(title)),
		wsOverlappedWindow, 100, 100, 520, 120, 0, 0, inst, 0)
	procShowWindow.Call(testWindow, 5 /* SW_SHOW */)
	procSetForegroundWindow.Call(testWindow)
}

// windowFocused reports whether the test window is the foreground window.
func windowFocused() bool {
	fg, _, _ := procGetForegroundWindow.Call()
	return testWindow != 0 && fg == testWindow
}

// pumpMessages dispatches all pending window messages for this thread.
func pumpMessages() {
	var m winMsg
	for {
		r, _, _ := procPeekMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0, 1 /* PM_REMOVE */)
		if r == 0 {
			return
		}
		procTranslateMsg.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMsgW.Call(uintptr(unsafe.Pointer(&m)))
	}
}
