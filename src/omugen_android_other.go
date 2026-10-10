//go:build !android

package main

// The Android app's hooks (util_android.go) do nothing elsewhere.

func androidArgs() []string { return nil }

func androidEngineReady() {}

func androidEngineExit() {}
