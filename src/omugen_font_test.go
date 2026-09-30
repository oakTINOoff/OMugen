package main

import (
	"os"
	"testing"
)

// Regression: FNT v1 fonts with several color banks (Colors < 256) gave every
// bank the last bank's palette (range-variable aliasing), and glyphs were drawn
// with that palette, turning Jump Force's combo counter black.
func TestFntV1BankPalettes(t *testing.T) {
	const path = "D:/Jump Force Mugen/data/LIFEBARS/UMVChitcount.fnt"
	if _, err := os.Stat(path); err != nil {
		t.Skip("Jump Force MUGEN font not present")
	}
	f, err := loadFntV1(path)
	if err != nil {
		t.Fatal(err)
	}
	// Drop queued main-thread work: it uploads GPU textures, and there is no renderer in tests.
	for len(sys.mainThreadTask) > 0 {
		<-sys.mainThreadTask
	}
	if len(f.palettes) < 2 {
		t.Fatalf("expected several banks for Colors=57, got %d", len(f.palettes))
	}
	if f.glyphPals {
		t.Fatal("FNT v1 fonts must use bank palettes, not per-glyph palettes")
	}
	// The glyph pixels use indices 201-253, which are colored in bank 0.
	if f.palettes[0][230]&0xFFFFFF == 0 {
		t.Fatal("bank 0 palette entry 230 is black")
	}
	for c, fci := range f.images[0] {
		for i := range fci.img {
			if len(fci.img[i].Pal) == 0 || &fci.img[i].Pal[0] != &f.palettes[i][0] {
				t.Fatalf("glyph %q bank %d does not use its own bank palette", c, i)
			}
		}
	}
}
