package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The files tools/lifebarsgen generates (build/omugen_build.sh runs it): the
// engine reads every sound and animation the characters and the engine's
// states call by number.
func TestGeneratedFightFilesLoad(t *testing.T) {
	root := filepath.Join("..", "screenpack")
	if _, err := os.Stat(filepath.Join(root, "engine", "gofx", "gofx.def")); err != nil {
		t.Skip("generated files missing (go run ./tools/lifebarsgen)")
	}
	sounds := map[string][][2]int32{
		"lifebars/fight.snd":   {{0, 1}, {0, 9}, {0, 10}, {1, 0}, {2, 0}, {2, 1}, {2, 2}, {3, 0}, {3, 1}},
		"lifebars/common.snd":  {{5, 0}, {5, 1}, {5, 2}, {5, 3}, {5, 4}, {6, 0}, {7, 0}, {7, 1}, {7, 2}, {20, 0}},
		"engine/gofx/gofx.snd": {{5300, 0}, {5400, 0}},
	}
	for file, want := range sounds {
		s, err := LoadSnd(filepath.Join(root, file))
		if err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		for _, gn := range want {
			if s.Get(gn) == nil {
				t.Errorf("%s: no sound %v", file, gn)
			}
		}
	}
	glyphsHaveEveryIcon(t, filepath.Join(root, "engine", "glyphs.sff"))
	anims := map[string][]int32{
		"lifebars/fightfx.air": {0, 1, 2, 3, 10, 11, 12, 40, 60, 61, 62, 70, 71, 72, 100, 101, 102, 120, 130},
		"engine/gofx/gofx.air": {5300, 5308, 5400, 5401, 5402, 5410, 5600, 5601, 5608},
		"lifebars/fight.def":   {501, 509, 510, 520, 530, 531, 532, 533, 591, 595},
	}
	for file, want := range anims {
		text, err := LoadText(filepath.Join(root, file))
		if err != nil {
			t.Fatal(err)
		}
		lines, i := SplitAndTrim(text, "\n"), 0
		sff := newSff()
		at := ReadAnimationTable(file, sff, &sff.palList, lines, &i, false)
		for _, no := range want {
			if at.get(no) == nil {
				t.Errorf("%s: no action %d", file, no)
			}
		}
	}
}

// glyphsHaveEveryIcon checks that the move list icons (glyphs.sff) have each
// sprite the default motif's [Glyphs] names, once.
func glyphsHaveEveryIcon(t *testing.T, path string) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sff := newSff()
	var lofs, tofs uint32
	if err := sff.header.Read(f, &lofs, &tofs); err != nil {
		t.Fatal(err)
	}
	if sff.header.Version[0] != 2 {
		t.Fatalf("%s: SFF v%d, want v2", path, sff.header.Version[0])
	}
	count := map[[2]uint16]int{}
	for i := 0; i < int(sff.header.NumberOfSprites); i++ {
		f.Seek(int64(sff.header.FirstSpriteHeaderOffset)+int64(i)*28, 0)
		s := newSprite()
		var xofs, size uint32
		var prev uint16
		if err := s.readHeaderV2(f, &xofs, &size, lofs, tofs, &prev); err != nil {
			t.Fatal(err)
		}
		count[[2]uint16{s.Group, s.Number}]++
	}
	for gn, n := range count {
		if n > 1 {
			t.Errorf("%s: sprite %d,%d is there %d times", path, gn[0], gn[1], n)
		}
	}

	text, err := LoadText(filepath.Join("resources", "defaultMotif.ini"))
	if err != nil {
		t.Fatal(err)
	}
	lines, i := SplitAndTrim(NormalizeNewlines(text), "\n"), 0
	for i < len(lines) {
		is, name, _ := ReadIniSection(lines, &i)
		if name == "" {
			break
		}
		if !strings.EqualFold(name, "Glyphs") {
			continue
		}
		if len(is) < 50 {
			t.Fatalf("[Glyphs] has only %d entries", len(is))
		}
		for key, v := range is {
			v = strings.TrimSpace(strings.SplitN(v, ";", 2)[0])
			// "_=" = 128, 0: the key has an = of its own.
			if j := strings.LastIndex(v, "="); j >= 0 {
				v = v[j+1:]
			}
			var g, n uint16
			if _, err := fmt.Sscanf(strings.ReplaceAll(v, " ", ""), "%d,%d", &g, &n); err != nil {
				t.Errorf("[Glyphs] %s = %q: %v", key, v, err)
				continue
			}
			if count[[2]uint16{g, n}] == 0 {
				t.Errorf("%s: no sprite %d,%d for glyph %s", path, g, n, key)
			}
		}
		return
	}
	t.Fatal("no [Glyphs] in defaultMotif.ini")
}
