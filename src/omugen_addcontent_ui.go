package main

// The player's side of adding characters and stages (omugen_addcontent.go).
// What is dropped on the window, given on the command line or picked in
// Options waits here; the next menu frame (omugen_hud.lua) shows the
// progress, asks before anything changes, installs, and restarts OMugen.

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	lua "github.com/yuin/gopher-lua"
)

var omugenAdd struct {
	mu       sync.Mutex
	queue    []string
	lastDrop time.Time
	dropping bool // between SDL's DROPBEGIN and DROPCOMPLETE
	waiting  bool // the phone's file picker is open, or copying
	job      *addJob
}

type addJob struct {
	state  string // "reading", "confirm", "installing", "done"
	prog   addProgress
	plan   *addPlan
	result *addResult
}

// omugenAddQueue adds dropped or picked files and folders (any thread).
func omugenAddQueue(paths ...string) {
	omugenAdd.mu.Lock()
	defer omugenAdd.mu.Unlock()
	for _, p := range paths {
		if p = strings.TrimSpace(p); p != "" {
			omugenAdd.queue = append(omugenAdd.queue, p)
		}
	}
	omugenAdd.lastDrop = time.Now()
}

func omugenAddDropping(on bool) {
	omugenAdd.mu.Lock()
	omugenAdd.dropping = on
	omugenAdd.lastDrop = time.Now()
	omugenAdd.mu.Unlock()
}

func omugenAddWaiting(on bool) {
	omugenAdd.mu.Lock()
	omugenAdd.waiting = on
	omugenAdd.mu.Unlock()
}

// omugenIsContentArg reports a command-line argument that is something to
// add (a file dropped on OMugen.exe): an existing archive, .def or folder,
// given with its full path.
func omugenIsContentArg(a string) bool {
	if !filepath.IsAbs(a) {
		return false
	}
	fi, err := os.Stat(a)
	if err != nil {
		return false
	}
	if fi.IsDir() {
		return true
	}
	switch strings.ToLower(filepath.Ext(a)) {
	case ".zip", ".rar", ".7z", ".def":
		return true
	}
	return false
}

// omugenLaunchArgs are the command-line arguments to restart with: the
// files that were added left out.
var omugenLaunchArgs []string

// omugenTakeContentArgs moves the files dropped on OMugen.exe from the
// command line to the queue (a flag's value, as in -r C:\...\system.def,
// stays).
func omugenTakeContentArgs(args []string) []string {
	var rest []string
	for i, a := range args {
		if (i == 0 || !strings.HasPrefix(args[i-1], "-")) && omugenIsContentArg(a) {
			omugenAddQueue(a)
		} else {
			rest = append(rest, a)
		}
	}
	omugenLaunchArgs = rest
	return rest
}

// omugenAddCapacity is the number of boxes of the select screen (0 when its
// grid grows with the roster, as the OMugen screenpack's does).
func omugenAddCapacity() int {
	def := sys.motif.Def
	if def == "" || isOMugenMotif(def) {
		return 0
	}
	if readMotifKey(def, "Select Info", "autorows") == "1" {
		return 0
	}
	rows, _ := strconv.Atoi(readMotifKey(def, "Select Info", "rows"))
	cols, _ := strconv.Atoi(readMotifKey(def, "Select Info", "columns"))
	if rows <= 0 || cols <= 0 {
		return 0
	}
	return rows * cols
}

// omugenAddStep moves the work on and reports it: the state ("idle",
// "waiting", "reading", "confirm", "installing", "done"), a text and the
// progress (0-1, or -1 when unknown).
func omugenAddStep() (string, string, float64) {
	omugenAdd.mu.Lock()
	defer omugenAdd.mu.Unlock()
	job := omugenAdd.job
	if job == nil {
		ready := len(omugenAdd.queue) > 0 && !omugenAdd.dropping &&
			time.Since(omugenAdd.lastDrop) > 300*time.Millisecond
		if !ready {
			if omugenAdd.waiting || len(omugenAdd.queue) > 0 {
				return "waiting", "Getting the files...", -1
			}
			return "idle", "", 0
		}
		paths := omugenAdd.queue
		omugenAdd.queue = nil
		selectDef, _ := omugenGameSelect()
		capacity := omugenAddCapacity()
		job = &addJob{state: "reading"}
		job.prog.set("Reading...", 0, 0)
		omugenAdd.job = job
		go func() {
			defer omugenAddRecover(job)
			plan := addPrepare(".", selectDef, capacity, paths, &job.prog)
			omugenAdd.mu.Lock()
			job.plan, job.state = plan, "confirm"
			omugenAdd.mu.Unlock()
		}()
	}
	switch job.state {
	case "reading", "installing":
		text, frac := job.prog.get()
		return job.state, text, frac
	}
	return job.state, "", 0
}

// omugenAddRecover ends a job that failed badly with what went wrong.
func omugenAddRecover(job *addJob) {
	if r := recover(); r != nil {
		LogMessage("OMugen: adding characters failed: %v", r)
		omugenAdd.mu.Lock()
		if job.plan != nil {
			job.plan.cleanup()
		}
		job.result = &addResult{failed: []string{"something went wrong: " + toString(r)}}
		job.state = "done"
		omugenAdd.mu.Unlock()
	}
}

func toString(v interface{}) string {
	if err, ok := v.(error); ok {
		return err.Error()
	}
	if s, ok := v.(string); ok {
		return s
	}
	return "unknown error"
}

// omugenAddConfirm asks the player (main thread), then installs.
func omugenAddConfirm() {
	omugenAdd.mu.Lock()
	job := omugenAdd.job
	omugenAdd.mu.Unlock()
	if job == nil || job.state != "confirm" {
		return
	}
	plan := job.plan
	if !plan.wanted() {
		addTell(plan.nothingText())
		plan.cleanup()
		omugenAddEnd()
		return
	}
	if _, quiet := sys.cmdFlags["-nodialog"]; quiet {
		LogMessage("OMugen: adding: %s", plan.summary())
	} else if !ShowYesNoDialog(plan.summary(), "Add characters and stages") {
		plan.cleanup()
		omugenAddEnd()
		return
	}
	omugenAdd.mu.Lock()
	job.state = "installing"
	omugenAdd.mu.Unlock()
	go func() {
		defer omugenAddRecover(job)
		res := plan.apply(&job.prog)
		for _, f := range res.failed {
			LogMessage("OMugen: not added: %s", f)
		}
		omugenAdd.mu.Lock()
		job.result, job.state = res, "done"
		omugenAdd.mu.Unlock()
	}()
}

func omugenAddEnd() {
	omugenAdd.mu.Lock()
	omugenAdd.job = nil
	omugenAdd.mu.Unlock()
}

// omugenAddFinish tells the player what was added and reports whether
// OMugen must restart.
func omugenAddFinish() bool {
	omugenAdd.mu.Lock()
	job := omugenAdd.job
	omugenAdd.mu.Unlock()
	if job == nil || job.state != "done" || job.result == nil {
		return false
	}
	omugenAddEnd()
	if text := job.result.text(); text != "" {
		addTell(text)
	}
	return job.result.changed
}

// addTell shows a message (logs it with -nodialog, for scripted tests).
func addTell(text string) {
	if _, quiet := sys.cmdFlags["-nodialog"]; quiet {
		LogMessage("OMugen: %s", text)
		return
	}
	ShowInfoDialog(text, "Add characters and stages")
}

func omugenAddScriptInit(l *lua.LState) {
	luaRegister(l, "omugenAddBusy", func(l *lua.LState) int {
		/*Whether characters or stages are waiting to be added (dropped on the
		window, picked in Options...), or are being added.
		@function omugenAddBusy
		@treturn boolean busy*/
		omugenAdd.mu.Lock()
		busy := omugenAdd.job != nil || omugenAdd.waiting || len(omugenAdd.queue) > 0
		omugenAdd.mu.Unlock()
		l.Push(lua.LBool(busy))
		return 1
	})
	luaRegister(l, "omugenAddStep", func(l *lua.LState) int {
		/*Moves the adding of characters and stages on.
		@function omugenAddStep
		@treturn string state "idle", "waiting", "reading", "confirm" (call omugenAddConfirm),
		  "installing" or "done" (call omugenAddFinish)
		@treturn string text What is going on.
		@treturn number progress 0-1, or -1 when unknown.*/
		state, text, frac := omugenAddStep()
		l.Push(lua.LString(state))
		l.Push(lua.LString(text))
		l.Push(lua.LNumber(frac))
		return 3
	})
	luaRegister(l, "omugenAddConfirm", func(l *lua.LState) int {
		/*Asks the player whether to add what was found, then adds it.
		@function omugenAddConfirm*/
		omugenAddConfirm()
		return 0
	})
	luaRegister(l, "omugenAddFinish", func(l *lua.LState) int {
		/*Tells the player what was added.
		@function omugenAddFinish
		@treturn boolean restart Whether OMugen must restart to load it.*/
		l.Push(lua.LBool(omugenAddFinish()))
		return 1
	})
	luaRegister(l, "omugenAddPick", func(l *lua.LState) int {
		/*Lets the player pick characters and stages to add (a file dialog; on
		Android, the phone's file picker).
		@function omugenAddPick*/
		omugenAddPick()
		return 0
	})
	luaRegister(l, "omugenRestart", func(l *lua.LState) int {
		/*Restarts OMugen on the same game.
		@function omugenRestart*/
		omugenRestart()
		return 0
	})
}
