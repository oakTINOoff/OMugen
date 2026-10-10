package main

import (
	findfont "github.com/flopp/go-findfont"
)

// omugenFallbackTtf is OMugen's own TrueType font (in the engine files).
const omugenFallbackTtf = "font/Open_Sans/OpenSans-Regular.ttf"

// omugenTtfPath finds the TrueType font file a MUGEN font definition names:
// next to the definition or the screenpack, in the game folder, data or font,
// then among the system's fonts. MUGEN games often name a Windows font they
// don't ship (arial.ttf in mssansserif-tt36.def): where the system has none
// (Android, Linux), OMugen's Open Sans stands in, so the screenpack still
// loads instead of falling back to OMugen's.
func omugenTtfPath(fontfile, filename string) string {
	path := SearchFile(filename, []string{fontfile, sys.motif.Def, "", "data/"}, "font/")
	if p := FileExist(path); p != "" {
		return p
	}
	if p, err := findfont.Find(path); err == nil {
		return p
	}
	if p := FileExist(omugenFallbackTtf); p != "" {
		LogMessage("OMugen: font %s (%s) isn't here or on this system: using Open Sans", filename, fontfile)
		return p
	}
	return path
}
