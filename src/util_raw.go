//go:build raw

package main

import (
	"C"
	"io"
	"os"
)

// Main entry point for C programs
//
//export GoMain
func GoMain() {
	main()
}

// Log writer implementation
func NewLogWriter() io.Writer {
	return os.Stderr
}

// Message box implementation using stderr
func ShowInfoDialog(message, title string) {
	print(title + "\n\n" + message)
}

func ShowErrorDialog(message string) {
	print("OMugen Error\n\n" + message)
}

// ShowYesNoDialog can't ask here: the answer is No.
func ShowYesNoDialog(message, title string) bool {
	print(title + "\n\n" + message)
	return false
}

// TTF font loading stub
func LoadFntTtf(f *Fnt, fontfile string, filename string, height int32) {
	panic(Error("TrueType fonts are not supported on this platform"))
}
