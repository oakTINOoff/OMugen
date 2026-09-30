package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadMotifKey(t *testing.T) {
	def := filepath.Join(t.TempDir(), "system.def")
	os.WriteFile(def, []byte("[Info]\nname = \"Test\"\nlocalcoord = 1280,720 ;HD\n\n[Files]\nspr = system.sff\nselect = ../select.def   ; roster\nfight = LIFEBARS/fight.def\n\n[Title Info]\nmenu.pos = 1,2\n"), 0o644)
	for _, c := range []struct{ section, key, want string }{
		{"Info", "localcoord", "1280,720"},
		{"Files", "select", "../select.def"},
		{"Files", "fight", "LIFEBARS/fight.def"},
		{"Files", "snd", ""},
		{"Music", "title.bgm", ""},
	} {
		if got := readMotifKey(def, c.section, c.key); got != c.want {
			t.Errorf("[%s] %s = %q, want %q", c.section, c.key, got, c.want)
		}
	}
}

func TestIsOMugenMotif(t *testing.T) {
	for def, want := range map[string]bool{
		"data/omugen/system.def": true,
		"C:/Users/x/AppData/Local/OMugen/engine/abc/data/omugen/system.def": true,
		`C:\engine\data\OMugen\System.def`:                                  true,
		"data/mugen1/system.def":                                            false,
		"data/system.def":                                                   false,
	} {
		if got := isOMugenMotif(def); got != want {
			t.Errorf("isOMugenMotif(%q) = %v, want %v", def, got, want)
		}
	}
}
