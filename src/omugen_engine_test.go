package main

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testEnginePack(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(body))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// withEngine installs a test pack in a temporary LOCALAPPDATA and game folder.
func withEngine(t *testing.T, files map[string]string) (gameDir string) {
	t.Helper()
	savedZip, savedDir, savedErr := engineZipData, engineDir, engineDirError
	wd, _ := os.Getwd()
	t.Setenv("LOCALAPPDATA", t.TempDir())
	gameDir = t.TempDir()
	// Registered after TempDir so it runs first (cleanups are LIFO): leave the
	// temp folder before the testing package deletes it.
	t.Cleanup(func() {
		engineZipData, engineDir, engineDirError = savedZip, savedDir, savedErr
		os.Chdir(wd)
	})
	if err := os.Chdir(gameDir); err != nil {
		t.Fatal(err)
	}
	engineZipData, engineDir, engineDirError = testEnginePack(t, files), "", nil
	prepareEngineDir()
	if engineDirError != nil || engineDir == "" {
		t.Fatalf("prepareEngineDir: dir=%q err=%v", engineDir, engineDirError)
	}
	return gameDir
}

func TestEngineFallbackAndOverride(t *testing.T) {
	withEngine(t, map[string]string{
		"external/script/main.lua": "-- engine main",
		"data/gofx/gofx.def":       "[Info]",
	})
	if !strings.HasPrefix(filepath.ToSlash(engineDir), filepath.ToSlash(os.Getenv("LOCALAPPDATA"))) {
		t.Fatalf("engine files must go to LOCALAPPDATA, got %s", engineDir)
	}
	// Missing from the game folder: served from the engine files.
	p := FileExist("data/gofx/gofx.def")
	if p == "" || !strings.HasPrefix(p, filepath.ToSlash(engineDir)) {
		t.Fatalf("FileExist fallback = %q", p)
	}
	text, err := LoadText("external/script/main.lua")
	if err != nil || text != "-- engine main" {
		t.Fatalf("LoadText fallback = %q, %v", text, err)
	}
	// A game file with the same path wins.
	os.MkdirAll("external/script", 0o755)
	os.WriteFile("external/script/main.lua", []byte("-- game main"), 0o644)
	if text, _ := LoadText("external/script/main.lua"); text != "-- game main" {
		t.Fatalf("game override not used: %q", text)
	}
	if FileExist("data/missing.def") != "" {
		t.Fatal("unknown files must stay missing")
	}
	// Nothing may be written into the game folder except what the test wrote.
	entries, _ := os.ReadDir(".")
	for _, e := range entries {
		if e.Name() != "external" {
			t.Errorf("unexpected file in game folder: %s", e.Name())
		}
	}
	if !strings.Contains(engineLuaPath("x"), filepath.ToSlash(engineDir)+"/?.lua") {
		t.Error("Lua package.path must include the engine files")
	}
}

func TestEngineExtractsOnlyOnce(t *testing.T) {
	withEngine(t, map[string]string{"data/a.zss": "one"})
	first := engineDir
	// Simulate a second launch: the marker makes it reuse the folder untouched.
	os.WriteFile(filepath.Join(first, "data", "a.zss"), []byte("kept"), 0o644)
	engineDir = ""
	prepareEngineDir()
	if engineDir != first {
		t.Fatalf("second launch used %s, want %s", engineDir, first)
	}
	if b, _ := os.ReadFile(filepath.Join(first, "data", "a.zss")); string(b) != "kept" {
		t.Fatal("second launch re-extracted the pack")
	}
}

func TestEnginePackRejectsUnsafePaths(t *testing.T) {
	data := testEnginePack(t, map[string]string{"../evil.txt": "x"})
	if err := extractEngineZip(data, t.TempDir()); err == nil {
		t.Fatal("expected an error for a path escaping the engine folder")
	}
}
