package main

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/text/encoding/charmap"
)

func TestLegacyFileNameVariants(t *testing.T) {
	// "Básicos.cns" saved in Windows-1252, then read as Shift-JIS.
	raw, _ := charmap.Windows1252.NewEncoder().String("Básicos.cns")
	garbled := decodeShiftJIS(raw)
	if garbled == "Básicos.cns" {
		t.Fatalf("test input was not garbled")
	}
	found := false
	for _, v := range legacyFileNameVariants(garbled) {
		if v == "Básicos.cns" {
			found = true
		}
	}
	if !found {
		t.Errorf("variants of %q = %q, want Básicos.cns", garbled, legacyFileNameVariants(garbled))
	}
	if v := legacyFileNameVariants("plain.cns"); v != nil {
		t.Errorf("ASCII names need no variants, got %q", v)
	}
}

func TestSearchFileLegacyName(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Técnicas.cns"), []byte("; test"), 0o644)
	raw, _ := charmap.Windows1252.NewEncoder().String("Técnicas.cns")
	got := SearchFile(decodeShiftJIS(raw), []string{filepath.ToSlash(dir) + "/char.def"})
	if want := filepath.ToSlash(filepath.Join(dir, "Técnicas.cns")); got != want {
		t.Errorf("SearchFile = %q, want %q", got, want)
	}
}
