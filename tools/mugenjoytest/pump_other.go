//go:build !windows

package main

func openTestWindow()     {}
func windowFocused() bool { return true }
func pumpMessages()       {}
