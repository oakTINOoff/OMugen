package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// Other versions' engine files go once unused for a month (half-unpacked
// ones after a day); recent ones may belong to an OMugen running elsewhere.
func TestPruneEngineDirs(t *testing.T) {
	if runtime.GOOS == "android" {
		t.Skip("Android removes every other copy")
	}
	root := t.TempDir()
	mk := func(name string, complete bool, age time.Duration) string {
		p := filepath.Join(root, name)
		os.MkdirAll(p, 0o755)
		when := time.Now().Add(-age)
		if complete {
			m := filepath.Join(p, ".complete")
			os.WriteFile(m, nil, 0o644)
			os.Chtimes(m, when, when)
		}
		os.Chtimes(p, when, when)
		return p
	}
	keep := mk("current", true, 90*24*time.Hour)
	old := mk("old", true, 40*24*time.Hour)
	recent := mk("recent", true, 2*24*time.Hour)
	halfOld := mk("half.tmp123", false, 2*24*time.Hour)
	halfNew := mk("half.tmp456", false, time.Hour)

	pruneEngineDirs(keep)

	for p, want := range map[string]bool{keep: true, old: false, recent: true, halfOld: false, halfNew: true} {
		if _, err := os.Stat(p); (err == nil) != want {
			t.Errorf("%s: kept = %v, want %v", filepath.Base(p), err == nil, want)
		}
	}
	if info, err := os.Stat(filepath.Join(keep, ".complete")); err != nil || time.Since(info.ModTime()) > time.Hour {
		t.Errorf("the engine files in use weren't marked as used")
	}
}
