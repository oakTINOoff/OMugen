package main

import (
	"os"
	"path/filepath"
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
