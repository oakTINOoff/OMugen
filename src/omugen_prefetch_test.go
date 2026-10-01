package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The prefetch finds the same .def and .cns files as the roster loader.
func TestPrefetchCharFindsDefAndCns(t *testing.T) {
	dir := t.TempDir()
	wd, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(wd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	write := func(p, s string) {
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("chars/kfm/kfm.def", "[Info]\nname = \"KFM\"\n[Files]\ncmd = kfm.cmd\nCNS   = \"kfm.cns\" ; constants\n")
	write("chars/kfm/kfm.cns", "[Size]\nxscale = 1\n")
	write("chars/other/x.def", "[Files]\ncns = sub/x.cns\n")
	write("chars/other/sub/x.cns", "[Size]\n")

	for entry, want := range map[string][]string{
		"kfm":               {"chars/kfm/kfm.def", "chars/kfm/kfm.cns"},
		" kfm/kfm.def ":     {"chars/kfm/kfm.def", "chars/kfm/kfm.cns"},
		"other/x":           {"chars/other/x.def", "chars/other/sub/x.cns"},
		"randomselect":      nil,
		"missing":           nil,
		"packed/char.zip":   nil,
		"chars/kfm/kfm.def": {"chars/kfm/kfm.def", "chars/kfm/kfm.cns"},
	} {
		got := prefetchChar(entry)
		if len(got) != len(want) {
			t.Errorf("%q: read %v, want %v", entry, got, want)
			continue
		}
		for i := range want {
			if !strings.EqualFold(filepath.ToSlash(got[i]), want[i]) {
				t.Errorf("%q: read %v, want %v", entry, got, want)
			}
		}
	}
}
