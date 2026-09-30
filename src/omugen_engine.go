package main

// OMugen drop-in support.
//
// Release builds (build tag omugen_engine) embed the engine's own files
// (Lua scripts, shaders, fonts, engine data, runtime DLLs) as a zip, so that
// OMugen.exe alone can be dropped into an unmodified MUGEN folder. On first run
// the pack is extracted once to %LOCALAPPDATA%\OMugen\engine\<content hash>\,
// never into the game folder. File lookups try the game folder first, then the
// extracted engine files, so a game can still override any engine file.

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// engineZipData is the embedded engine pack (nil in development builds, which
// use the files in the repository instead).
var engineZipData []byte

// engineDir is where the engine pack was extracted ("" if none).
var engineDir string

// engineDirError records why the engine pack could not be prepared.
var engineDirError error

// prepareEngineDir extracts the embedded engine pack if needed and sets engineDir.
// It is safe to call more than once.
func prepareEngineDir() {
	if engineDir != "" || engineDirError != nil || len(engineZipData) == 0 {
		return
	}
	sum := sha256.Sum256(engineZipData)
	dir := filepath.Join(engineCacheRoot(), "engine", hex.EncodeToString(sum[:8]))
	marker := filepath.Join(dir, ".complete")
	if _, err := os.Stat(marker); err == nil {
		engineDir = dir
		return
	}
	tmp := fmt.Sprintf("%s.tmp%d", dir, os.Getpid())
	_ = os.RemoveAll(tmp)
	if err := extractEngineZip(engineZipData, tmp); err != nil {
		_ = os.RemoveAll(tmp)
		engineDirError = fmt.Errorf("extracting engine files to %s: %w", tmp, err)
		return
	}
	if err := os.WriteFile(filepath.Join(tmp, ".complete"), nil, 0o644); err != nil {
		_ = os.RemoveAll(tmp)
		engineDirError = err
		return
	}
	if err := os.Rename(tmp, dir); err != nil {
		// Another OMugen instance may have finished first.
		_ = os.RemoveAll(tmp)
		if _, statErr := os.Stat(marker); statErr != nil {
			engineDirError = fmt.Errorf("installing engine files to %s: %w", dir, err)
			return
		}
	}
	engineDir = dir
}

// engineCacheRoot is the per-user folder for OMugen's extracted engine files.
func engineCacheRoot() string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		if d, err := os.UserCacheDir(); err == nil {
			base = d
		} else {
			base = os.TempDir()
		}
	}
	return filepath.Join(base, "OMugen")
}

func extractEngineZip(data []byte, dst string) error {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	for _, f := range zr.File {
		name := filepath.FromSlash(f.Name)
		if strings.Contains(name, "..") || filepath.IsAbs(name) {
			return fmt.Errorf("unsafe path in engine pack: %s", f.Name)
		}
		target := filepath.Join(dst, name)
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := extractZipFile(f, target); err != nil {
			return err
		}
	}
	return nil
}

func extractZipFile(f *zip.File, target string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.Create(target)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, rc); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// engineAsset returns the extracted engine copy of a relative path such as
// "external/script/main.lua" or "data/gofx/gofx.def", or "" if there is none.
func engineAsset(path string) string {
	if engineDir == "" || path == "" || filepath.IsAbs(path) {
		return ""
	}
	p := filepath.Join(engineDir, filepath.FromSlash(path))
	if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
		return filepath.ToSlash(p)
	}
	return ""
}

// enginePath resolves a path the engine itself needs (config defaults, Lua
// entry script, window icons): the game folder first, then the engine files.
func enginePath(path string) string {
	if p := FileExist(path); p != "" {
		return p
	}
	return path
}

// engineLuaPath is a Lua package.path that also searches the engine files.
func engineLuaPath(current string) string {
	if engineDir == "" {
		return current
	}
	return "./?.lua;" + filepath.ToSlash(engineDir) + "/?.lua;" + current
}

// useExeDirAsGameDir makes the executable's folder the working directory when
// OMugen is started from elsewhere (shortcut, command line), as MUGEN does,
// unless the current folder is itself a game folder.
func useExeDirAsGameDir() {
	if _, err := os.Stat("data"); err == nil {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	dir := filepath.Dir(exe)
	if _, err := os.Stat(filepath.Join(dir, "data")); err == nil {
		_ = os.Chdir(dir)
	}
}
