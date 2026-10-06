package main

// Roster prefetch for cold starts.
//
// Before the select screen's roster is registered (one character at a time,
// on the main thread, behind the loading screen), the files that step reads
// are read here in parallel: each character's .def and the top of its .cns,
// and each stage's .def. Right after a reboot none of them are in Windows'
// file cache, and reading thousands of small files one after another kept
// the loading screen up for over a minute; read in parallel, the main thread
// finds them in memory. Nothing here changes what is loaded: the data read
// is thrown away, only the cache is warmed.

import (
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"time"
)

const (
	prefetchWorkers = 16
	prefetchCnsHead = 64 << 10 // the [Size] section is near the top
)

var prefetchCnsRe = regexp.MustCompile(`(?im)^[ \t]*cns[ \t]*=[ \t]*([^;\r\n]+)`)

// prefetchRoster starts reading the characters' and stages' files (entries
// as written in select.def) and returns at once.
func prefetchRoster(chars, stages []string) {
	jobs := make(chan func(), 4*prefetchWorkers)
	var done int64
	start := time.Now()
	total := len(chars) + len(stages)
	for i := 0; i < prefetchWorkers; i++ {
		go func() {
			for job := range jobs {
				job()
				if atomic.AddInt64(&done, 1) == int64(total) && loadProf.enabled {
					LogMessage("OMugen: roster prefetch: %d entries in %v", total, time.Since(start).Round(time.Millisecond))
				}
			}
		}()
	}
	go func() {
		for _, c := range chars {
			c := c
			jobs <- func() { prefetchChar(c) }
		}
		for _, s := range stages {
			s := s
			jobs <- func() { prefetchStage(s) }
		}
		close(jobs)
	}()
}

// prefetchChar reads a character's .def and the top of its .cns, finding
// them the way Select.AddChar does (zipped characters are skipped). It
// returns the files it read.
func prefetchChar(entry string) (read []string) {
	guess := mugenSlash(strings.TrimSpace(entry))
	low := strings.ToLower(guess)
	if guess == "" || low == "randomselect" || strings.HasSuffix(low, ".zip") {
		return nil
	}
	if !strings.HasSuffix(low, ".def") {
		if !strings.Contains(guess, "/") {
			guess = filepath.ToSlash(filepath.Join(guess, filepath.Base(guess)+".def"))
		} else {
			guess += ".def"
		}
	}
	def := SearchFile(guess, []string{"", "data/"}, "chars/")
	if def == "" {
		return nil
	}
	data, err := os.ReadFile(def)
	if err != nil {
		return nil
	}
	read = append(read, def)
	if m := prefetchCnsRe.FindSubmatch(data); m != nil {
		cns := strings.Trim(strings.TrimSpace(string(m[1])), `"'`)
		if p := SearchFile(cns, []string{def, "", "data/"}); p != "" {
			prefetchHead(p, prefetchCnsHead)
			read = append(read, p)
		}
	}
	return read
}

// prefetchStage reads a stage's .def, found the way Select.AddStage does.
func prefetchStage(entry string) {
	def := mugenSlash(strings.TrimSpace(entry))
	if def == "" || strings.ContainsRune(def, '=') || strings.HasSuffix(strings.ToLower(def), ".zip") {
		return
	}
	if !strings.HasSuffix(strings.ToLower(def), ".def") {
		def += ".def"
	}
	if p := SearchFile(def, []string{"", "data/"}, "stages/"); p != "" {
		_, _ = os.ReadFile(p)
	}
}

func prefetchHead(path string, n int64) {
	if f, err := os.Open(path); err == nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(f, n))
		f.Close()
	}
}
