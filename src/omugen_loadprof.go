package main

// OMugen: optional load-time profiler for character loading.
// Enable with the environment variable OMUGEN_LOADPROF=1; totals per phase
// are printed to stderr every 100 characters.

import (
	"fmt"
	"os"
	"runtime/pprof"
	"sort"
	"strings"
	"sync"
	"time"
)

var loadProf = struct {
	sync.Mutex
	enabled bool
	start   time.Time
	chars   int
	phases  map[string]time.Duration
}{
	enabled: os.Getenv("OMUGEN_LOADPROF") != "",
	phases:  map[string]time.Duration{},
}

// loadProfAdd adds the time since t to a phase.
func loadProfAdd(phase string, t time.Time) {
	if !loadProf.enabled {
		return
	}
	d := time.Since(t)
	loadProf.Lock()
	loadProf.phases[phase] += d
	loadProf.Unlock()
}

// loadProfChar counts one processed select.def entry.
func loadProfChar() {
	if !loadProf.enabled {
		return
	}
	loadProf.Lock()
	defer loadProf.Unlock()
	if loadProf.chars == 0 {
		loadProf.start = time.Now()
	}
	loadProf.chars++
	if loadProf.chars%100 == 0 {
		loadProfPrintLocked()
	}
}

// loadProfDump prints the totals (e.g. when background loading finishes).
func loadProfDump() {
	loadProfStopCPU()
	if !loadProf.enabled {
		return
	}
	loadProf.Lock()
	defer loadProf.Unlock()
	loadProfPrintLocked()
}

func loadProfPrintLocked() {
	names := make([]string, 0, len(loadProf.phases))
	for k := range loadProf.phases {
		names = append(names, k)
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, k := range names {
		parts = append(parts, fmt.Sprintf("%s=%v", k, loadProf.phases[k].Round(time.Millisecond)))
	}
	fmt.Fprintf(os.Stderr, "OMugen loadprof: %d entries, wall %v | %s\n",
		loadProf.chars, time.Since(loadProf.start).Round(time.Millisecond), strings.Join(parts, " "))
}

// OMUGEN_CPUPROFILE=file writes a CPU profile (go tool pprof) of the start
// of the game, up to the end of background loading.
var loadProfCPU *os.File

func init() {
	if p := os.Getenv("OMUGEN_CPUPROFILE"); p != "" {
		if f, err := os.Create(p); err == nil && pprof.StartCPUProfile(f) == nil {
			loadProfCPU = f
		}
	}
}

func loadProfStopCPU() {
	if loadProfCPU != nil {
		pprof.StopCPUProfile()
		loadProfCPU.Close()
		loadProfCPU = nil
	}
}
