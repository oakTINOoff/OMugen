// packengine builds src/omugen_engine.zip: the engine files OMugen.exe embeds
// (build tag omugen_engine) so it runs from an unmodified MUGEN folder.
//
// Run from the repo root after a normal build has filled lib/:
//
//	go run ./tools/packengine
//
// Entries are sorted and use a fixed timestamp, so identical content always
// produces an identical pack (and the same extraction folder at runtime).
package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// source maps files on disk (glob, relative to the repo root) to their path
// inside the pack. dest "" keeps the same relative path.
type source struct {
	glob     string
	dest     string // directory inside the pack, or "" for the glob's own directory
	required bool
}

var sources = []source{
	// Engine scripts, shaders and window icons.
	{glob: "external/script/*.lua", required: true},
	{glob: "external/shaders/*"},
	{glob: "external/icons/omugen_256.png", required: true},
	{glob: "external/icons/omugen_96.png", required: true},
	{glob: "external/icons/omugen_48.png", required: true},
	// Engine data (common states, commands, constants, system scripts).
	{glob: "data/*.zss", required: true},
	{glob: "data/common.air", required: true},
	{glob: "data/common.cmd", required: true},
	{glob: "data/common.const", required: true},
	// Engine fonts.
	{glob: "font/Open_Sans/*", required: true},
	{glob: "font/debug.def", required: true},
	{glob: "font/default-3x5*", required: true},
	// Assets Ikemen GO releases ship from the Ikemen-GO-Screenpack repo.
	{glob: "build/screenpack/data/gofx/*", dest: "data/gofx", required: true},
	{glob: "build/screenpack/data/glyphs.sff", dest: "data", required: true},
	{glob: "build/screenpack/font/Open_Sans.def", dest: "font", required: true},
	// Runtime DLLs (SDL2, FFmpeg, libxmp, MinGW runtime).
	{glob: "lib/*.dll", required: true},
	// Licenses.
	{glob: "LICENCE.txt", dest: "licenses", required: true},
	{glob: "build/screenpack/LICENCE.txt", dest: "licenses/ikemen-go-screenpack"},
}

const ffmpegNotice = `OMugen dynamically links FFmpeg (avcodec, avformat, avutil, avfilter,
avdevice, swscale, swresample), licensed under the GNU LGPL v2.1.
The DLLs in lib/ may be replaced with compatible builds.
build/build.sh in the OMugen sources builds the exact FFmpeg configuration
from the official FFmpeg sources (https://ffmpeg.org).
`

func main() {
	out := filepath.Join("src", "omugen_engine.zip")
	files := map[string]string{} // pack path -> disk path
	for _, s := range sources {
		matches, err := filepath.Glob(filepath.FromSlash(s.glob))
		check(err)
		var n int
		for _, m := range matches {
			if fi, err := os.Stat(m); err != nil || fi.IsDir() {
				continue
			}
			dir := s.dest
			if dir == "" {
				dir = filepath.ToSlash(filepath.Dir(m))
			}
			files[dir+"/"+filepath.Base(m)] = m
			n++
		}
		if n == 0 && s.required {
			fail("no files match %s (build once with build/build.sh, and clone the Ikemen-GO-Screenpack repo into build/screenpack)", s.glob)
		}
	}

	names := make([]string, 0, len(files))
	for k := range files {
		names = append(names, k)
	}
	sort.Strings(names)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	stamp := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var raw int64
	add := func(name string, data []byte) {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate, Modified: stamp})
		check(err)
		_, err = w.Write(data)
		check(err)
		raw += int64(len(data))
	}
	for _, name := range names {
		data, err := os.ReadFile(files[name])
		check(err)
		add(name, data)
	}
	add("licenses/FFmpeg.txt", []byte(ffmpegNotice))
	check(zw.Close())
	check(os.WriteFile(out, buf.Bytes(), 0o644))
	fmt.Printf("wrote %s: %d files, %.1f MB -> %.1f MB\n", out, len(names)+1,
		float64(raw)/(1<<20), float64(buf.Len())/(1<<20))
}

func check(err error) {
	if err != nil {
		fail("%v", err)
	}
}

func fail(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, "packengine: "+format+"\n", a...)
	os.Exit(1)
}
