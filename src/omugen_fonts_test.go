package main

import (
	"os"
	"path/filepath"
	"testing"
)

// A TrueType font the game names but doesn't ship, and the system doesn't
// have (arial.ttf on a phone), is replaced by OMugen's own.
func TestTtfFallback(t *testing.T) {
	dir := t.TempDir()
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	os.Chdir(dir)
	for _, f := range []string{omugenFallbackTtf, "font/brushie.ttf"} {
		os.MkdirAll(filepath.Dir(f), 0o755)
		os.WriteFile(f, []byte("ttf"), 0o644)
	}
	if p := omugenTtfPath("font/brushie_24.def", "brushie.ttf"); p != "font/brushie.ttf" {
		t.Errorf("the game's own font: got %q", p)
	}
	if p := omugenTtfPath("font/mssansserif-tt36.def", "omugen-no-such-font-7f3a.ttf"); p != omugenFallbackTtf {
		t.Errorf("a missing font: got %q, want %q", p, omugenFallbackTtf)
	}
}
