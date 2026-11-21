//go:build raw

package main

// The raw build has no file dialog: characters are added by dropping them
// on the window.
func omugenAddPick() {}

func omugenRestart() {
	ShowInfoDialog("Start OMugen again to play with them.", "Add characters and stages")
	handleExit()
}
