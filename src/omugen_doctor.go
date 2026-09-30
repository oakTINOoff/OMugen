package main

// OMugen Doctor: `OMugen.exe -doctor` loads every character and stage listed
// in the game's select.def the way a match does (one at a time, slot 0),
// collects errors, crashes and warnings per item, checks for animation frames
// that point at missing sprites, and writes save/doctor/report.html,
// report.json and report.txt. Nothing in the game folder is modified.
//
// Options: -doctor chars|stages (default both), -doctorlimit N (first N items),
// -doctornodialog (no "finished" message box, for unattended runs).

import (
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"time"
)

type doctorItem struct {
	Kind          string   `json:"kind"`  // "char" or "stage"
	Entry         string   `json:"entry"` // as written in select.def
	Line          int      `json:"line"`  // select.def line
	Def           string   `json:"def"`   // resolved .def ("" if missing)
	Status        string   `json:"status"`
	Error         string   `json:"error,omitempty"`
	Warnings      []string `json:"warnings,omitempty"`
	MissingSprite []string `json:"missingSprites,omitempty"`
	Millis        int64    `json:"ms"`
}

// Status values, worst last.
const (
	doctorOK       = "ok"
	doctorWarning  = "warning"
	doctorMissing  = "missing"
	doctorError    = "error"
	doctorCrash    = "crash"
	doctorTimeout  = "timeout"
	doctorMaxWarns = 200 // per item, to keep reports readable

	doctorInProgress = "(in progress)" // stoppedAt marker for partial reports
)

var doctorState struct {
	sync.Mutex
	active bool
	cur    *doctorItem
	seen   map[string]bool
}

// doctorNote records a log line against the item being loaded (called from
// LogMessage and appendToConsole while the doctor runs).
func doctorNote(msg string) {
	doctorState.Lock()
	defer doctorState.Unlock()
	it := doctorState.cur
	if it == nil {
		return
	}
	for _, line := range strings.Split(strings.TrimSpace(msg), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || doctorState.seen[line] {
			continue
		}
		doctorState.seen[line] = true
		if len(it.Warnings) < doctorMaxWarns {
			it.Warnings = append(it.Warnings, line)
		}
	}
}

func doctorActive() bool {
	doctorState.Lock()
	defer doctorState.Unlock()
	return doctorState.active
}

// doctorEntries parses select.def: characters and stages with their lines.
func doctorEntries(path string) (chars, stages []doctorItem, err error) {
	text, err := LoadText(path)
	if err != nil {
		return nil, nil, err
	}
	section := ""
	seenStage := map[string]bool{}
	addStage := func(entry string, line int) {
		key := strings.ToLower(filepath.ToSlash(entry))
		if entry == "" || seenStage[key] {
			return
		}
		seenStage[key] = true
		stages = append(stages, doctorItem{Kind: "stage", Entry: entry, Line: line})
	}
	for i, raw := range strings.Split(NormalizeNewlines(text), "\n") {
		line := strings.TrimSpace(strings.SplitN(raw, ";", 2)[0])
		if line == "" {
			continue
		}
		if line[0] == '[' {
			section = strings.ToLower(strings.Trim(line, "[] "))
			continue
		}
		parts := strings.Split(line, ",")
		first := strings.TrimSpace(parts[0])
		switch section {
		case "characters":
			switch strings.ToLower(first) {
			case "", "randomselect", "blank", "empty", "dummyslot", "skipslot":
				continue
			}
			if strings.HasPrefix(first, "slot") {
				continue // slot = {...} groups: characters are listed inside
			}
			chars = append(chars, doctorItem{Kind: "char", Entry: first, Line: i + 1})
			for _, p := range parts[1:] {
				if k, v, ok := strings.Cut(strings.TrimSpace(p), "="); ok && strings.EqualFold(strings.TrimSpace(k), "stage") {
					addStage(strings.TrimSpace(v), i+1)
				}
			}
		case "extrastages":
			addStage(first, i+1)
		}
	}
	return chars, stages, nil
}

// doctorResolveChar finds a character's .def like Select.AddChar does.
func doctorResolveChar(entry string) string {
	p := filepath.ToSlash(entry)
	if strings.HasSuffix(strings.ToLower(p), ".zip") {
		return "" // zipped characters: reported as not checked
	}
	if !strings.HasSuffix(strings.ToLower(p), ".def") {
		if !strings.Contains(p, "/") {
			p = p + "/" + filepath.Base(p) + ".def"
		} else {
			p += ".def"
		}
	}
	return SearchFile(p, []string{"", "data/"}, "chars/")
}

func doctorResolveStage(entry string) string {
	p := filepath.ToSlash(entry)
	if !strings.HasSuffix(strings.ToLower(p), ".def") {
		p += ".def"
	}
	return FileExist(p)
}

// doctorRun loads one item on a worker goroutine while the main thread keeps
// processing GL tasks and window events. It returns false if the user closed
// the window.
func doctorRun(it *doctorItem, load func() error) bool {
	doctorState.Lock()
	doctorState.cur = it
	doctorState.seen = map[string]bool{}
	doctorState.Unlock()

	start := time.Now()
	done := make(chan struct{})
	var loadErr error
	var crash string
	go func() {
		defer close(done)
		defer func() {
			if r := recover(); r != nil {
				crash = fmt.Sprintf("%v\n%s", r, doctorShortStack(debug.Stack()))
			}
		}()
		loadErr = load()
	}()
	const timeout = 3 * time.Minute
	for waiting := true; waiting; {
		select {
		case <-done:
			waiting = false
		default:
			sys.keepAlive()
			if sys.window.shouldClose() {
				return false
			}
			if time.Since(start) > timeout {
				it.Status, it.Error = doctorTimeout, "loading did not finish within 3 minutes"
				return false // a stuck loader would race with the next item: stop here
			}
			time.Sleep(time.Millisecond)
		}
	}
	sys.runMainThreadTask()
	it.Millis = time.Since(start).Milliseconds()

	doctorState.Lock()
	doctorState.cur = nil
	doctorState.Unlock()

	switch {
	case crash != "":
		it.Status, it.Error = doctorCrash, crash
	case loadErr != nil:
		it.Status, it.Error = doctorError, strings.TrimSpace(loadErr.Error())
	case len(it.Warnings) > 0 || len(it.MissingSprite) > 0:
		it.Status = doctorWarning
	default:
		it.Status = doctorOK
	}
	return true
}

func doctorShortStack(stack []byte) string {
	lines := strings.Split(string(stack), "\n")
	var keep []string
	for _, l := range lines {
		if strings.Contains(l, "Ikemen-GO/src.") || strings.Contains(l, ".go:") && strings.Contains(l, "/src/") {
			keep = append(keep, strings.TrimSpace(l))
		}
		if len(keep) >= 12 {
			break
		}
	}
	return strings.Join(keep, "\n")
}

// doctorResetSlot frees what the previous character loaded into slot 0.
func doctorResetSlot() {
	sys.loadMutex.Lock()
	for prefix, ffx := range sys.ffx {
		if ffx != nil && ffx.isCharFX {
			delete(sys.ffx, prefix)
		}
	}
	sys.loadMutex.Unlock()
	sys.cgi[0].sff = nil
	sys.cgi[0].snd = nil
	sys.cgi[0].palettedata = nil
	sys.cgi[0].states = nil
	sys.cgi[0].customShaders = nil
	sys.chars[0] = nil
}

func doctorCheckChar(it *doctorItem) {
	c := sys.chars[0]
	if len(c) == 0 || c[0] == nil {
		return
	}
	gi := c[0].gi()
	if gi.sff == nil {
		return
	}
	var ids []int32
	for id := range gi.animTable.anims {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		a := gi.animTable.anims[id]
		if a == nil {
			continue
		}
		for _, f := range a.frames {
			if f.Group < 0 || f.Number < 0 {
				continue
			}
			if _, ok := gi.sff.sprites[[2]uint16{uint16(f.Group), uint16(f.Number)}]; !ok {
				it.MissingSprite = append(it.MissingSprite, fmt.Sprintf("action %d uses sprite %d,%d", id, f.Group, f.Number))
				break // one line per action is enough
			}
		}
		if len(it.MissingSprite) >= 50 {
			break
		}
	}
}

func runDoctor() {
	mode := strings.ToLower(sys.cmdFlags["-doctor"])
	limit := int(Atoi(sys.cmdFlags["-doctorlimit"]))
	outDir := filepath.Join("save", "doctor")
	_ = os.MkdirAll(outDir, 0o755)
	started := time.Now()
	setTitle := func(s string) {
		if sys.window != nil && sys.window.Window != nil {
			sys.window.SetTitle(s)
		}
	}

	doctorState.Lock()
	doctorState.active = true
	doctorState.Unlock()

	m, err := loadMotif("")
	if err != nil {
		doctorFatal(outDir, "Could not load the screenpack: "+err.Error())
		return
	}
	sys.motif = *m
	fs, err := loadFightScreen("")
	if err != nil {
		doctorFatal(outDir, "Could not load the lifebars (fight.def): "+err.Error())
		return
	}
	sys.fightScreen = *fs
	if err := loadCommonFightFx(sys.fightScreen.def, true); err != nil {
		LogMessage("OMugen Doctor: common FX: %v", err)
	}

	chars, stages, err := doctorEntries(sys.motif.Files.Select)
	if err != nil {
		doctorFatal(outDir, "Could not read select.def: "+err.Error())
		return
	}
	if mode == "stages" {
		chars = nil
	} else if mode == "chars" {
		stages = nil
	}
	if limit > 0 {
		if len(chars) > limit {
			chars = chars[:limit]
		}
		if len(stages) > limit {
			stages = stages[:limit]
		}
	}
	total := len(chars) + len(stages)
	LogMessage("OMugen Doctor: checking %d characters and %d stages from %s", len(chars), len(stages), sys.motif.Files.Select)

	items := make([]*doctorItem, 0, total)
	stopped := ""
	n := 0
	// progress shows an estimate in the title and saves a partial report every
	// 25 items, so an interrupted run still leaves a useful report.
	progress := func(name string) {
		eta := ""
		if n > 5 {
			left := time.Duration(float64(time.Since(started)) / float64(n-1) * float64(total-n+1))
			eta = fmt.Sprintf(" - about %v left", left.Round(time.Minute))
		}
		setTitle(fmt.Sprintf("OMugen Doctor %d/%d%s - %s", n, total, eta, name))
		if n%25 == 0 {
			doctorWriteReports(outDir, items[:len(items)-1], total, doctorInProgress, time.Since(started))
		}
	}
	for i := range chars {
		it := &chars[i]
		items = append(items, it)
		n++
		progress(it.Entry)
		if it.Def = doctorResolveChar(it.Entry); it.Def == "" {
			it.Status, it.Error = doctorMissing, "character .def not found (zip characters are not checked)"
			continue
		}
		doctorResetSlot()
		def := it.Def
		if !doctorRun(it, func() error {
			p := newChar(0, 0)
			sys.chars[0] = []*Char{p}
			if err := p.load(def); err != nil {
				return err
			}
			states, err := newCharCompiler().Compile(0, def, p.gi().constants)
			sys.cgi[0].states = states
			return err
		}) {
			stopped = it.Entry
			break
		}
		if it.Status == doctorOK || it.Status == doctorWarning {
			func() {
				defer func() {
					if r := recover(); r != nil {
						it.Status, it.Error = doctorCrash, fmt.Sprintf("palettes: %v", r)
					}
				}()
				sys.chars[0][0].loadPalettes()
				sys.runMainThreadTask()
			}()
			doctorCheckChar(it)
			if it.Status == doctorOK && len(it.MissingSprite) > 0 {
				it.Status = doctorWarning
			}
		}
		if n%20 == 0 {
			doctorResetSlot()
			runtime.GC()
			sys.runMainThreadTask()
		}
	}
	doctorResetSlot()

	if stopped == "" {
		for i := range stages {
			it := &stages[i]
			items = append(items, it)
			n++
			progress(it.Entry)
			if it.Def = doctorResolveStage(it.Entry); it.Def == "" {
				it.Status, it.Error = doctorMissing, "stage .def not found"
				continue
			}
			def := it.Def
			var st *Stage
			if !doctorRun(it, func() error {
				var err error
				st, err = loadStage(def, true)
				return err
			}) {
				stopped = it.Entry
				break
			}
			if st != nil {
				st.destroy()
			}
			if n%10 == 0 {
				runtime.GC()
				sys.runMainThreadTask()
			}
		}
	}

	doctorState.Lock()
	doctorState.active = false
	doctorState.Unlock()

	path := doctorWriteReports(outDir, items, total, stopped, time.Since(started))
	setTitle("OMugen Doctor - done")
	LogMessage("OMugen Doctor: report written to %s", path)
	if _, quiet := sys.cmdFlags["-doctornodialog"]; !quiet {
		ShowInfoDialog(fmt.Sprintf("OMugen Doctor finished in %v.\n\nReport: %s", time.Since(started).Round(time.Second), path), "OMugen Doctor")
	}
}

func doctorFatal(outDir, msg string) {
	doctorState.Lock()
	doctorState.active = false
	doctorState.Unlock()
	_ = os.WriteFile(filepath.Join(outDir, "report.txt"), []byte("OMugen Doctor could not start:\n"+msg+"\n"), 0o644)
	ShowErrorDialog("OMugen Doctor could not start:\n" + msg)
}

var (
	doctorNumRe  = regexp.MustCompile(`\d+`)
	doctorPathRe = regexp.MustCompile(`(?i)\b(chars|stages|data|font|sound)[/\\][^:,;]*?\.(def|cns|st|cmd|air|sff|snd|act|fnt|zss|txt)\b`)
	doctorWhoRe  = regexp.MustCompile(`^WARNING: .*?'s state -?N in .*?, line N: `)
)

// doctorIssueKey groups similar warnings across characters.
func doctorIssueKey(w string) string {
	k := doctorPathRe.ReplaceAllString(w, "<file>")
	k = doctorNumRe.ReplaceAllString(k, "N")
	k = doctorWhoRe.ReplaceAllString(k, "WARNING: <state>: ")
	if len(k) > 160 {
		k = k[:160] + "..."
	}
	return k
}

type doctorIssue struct {
	Key   string   `json:"issue"`
	Count int      `json:"items"`
	Items []string `json:"examples"`
}

func doctorWriteReports(outDir string, items []*doctorItem, total int, stopped string, took time.Duration) string {
	counts := map[string]int{}
	for _, it := range items {
		counts[it.Status]++
	}
	issues := map[string]*doctorIssue{}
	for _, it := range items {
		keys := map[string]bool{}
		add := func(k string) {
			if keys[k] {
				return
			}
			keys[k] = true
			is := issues[k]
			if is == nil {
				is = &doctorIssue{Key: k}
				issues[k] = is
			}
			is.Count++
			if len(is.Items) < 8 {
				is.Items = append(is.Items, it.Entry)
			}
		}
		for _, w := range it.Warnings {
			add(doctorIssueKey(w))
		}
		if len(it.MissingSprite) > 0 {
			add("Animation frames use sprites that are not in the character's .sff")
		}
		if it.Error != "" {
			first := strings.SplitN(it.Error, "\n", 2)[0]
			add(strings.ToUpper(it.Status) + ": " + doctorIssueKey(first))
		}
	}
	var sorted []*doctorIssue
	for _, is := range issues {
		sorted = append(sorted, is)
	}
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Count != sorted[j].Count {
			return sorted[i].Count > sorted[j].Count
		}
		return sorted[i].Key < sorted[j].Key
	})

	// JSON
	type report struct {
		Game      string         `json:"game"`
		Generated string         `json:"generated"`
		Seconds   int            `json:"seconds"`
		Checked   int            `json:"checked"`
		Total     int            `json:"total"`
		StoppedAt string         `json:"stoppedAt,omitempty"`
		Counts    map[string]int `json:"counts"`
		Issues    []*doctorIssue `json:"issues"`
		Items     []*doctorItem  `json:"items"`
	}
	wd, _ := os.Getwd()
	rep := report{Game: wd, Generated: time.Now().Format(time.RFC3339), Seconds: int(took.Seconds()),
		Checked: len(items), Total: total, StoppedAt: stopped, Counts: counts, Issues: sorted, Items: items}
	if b, err := json.MarshalIndent(rep, "", "  "); err == nil {
		_ = os.WriteFile(filepath.Join(outDir, "report.json"), b, 0o644)
	}

	// Text summary
	var t strings.Builder
	fmt.Fprintf(&t, "OMugen Doctor report - %s\nGame: %s\nChecked %d of %d items in %v\n", rep.Generated, wd, len(items), total, took.Round(time.Second))
	if stopped != "" {
		if stopped == doctorInProgress {
			t.WriteString("IN PROGRESS: partial report, the doctor is still running\n")
		} else {
			fmt.Fprintf(&t, "STOPPED at %s (window closed or a load did not finish)\n", stopped)
		}
	}
	for _, s := range []string{doctorOK, doctorWarning, doctorMissing, doctorError, doctorCrash, doctorTimeout} {
		fmt.Fprintf(&t, "  %-8s %d\n", s, counts[s])
	}
	t.WriteString("\nMost common issues:\n")
	for i, is := range sorted {
		if i >= 40 {
			break
		}
		fmt.Fprintf(&t, "%5d  %s\n", is.Count, is.Key)
	}
	t.WriteString("\nErrors and crashes:\n")
	for _, it := range items {
		if it.Status == doctorError || it.Status == doctorCrash || it.Status == doctorMissing || it.Status == doctorTimeout {
			fmt.Fprintf(&t, "[%s] %s (select.def line %d): %s\n", it.Status, it.Entry, it.Line, strings.SplitN(it.Error, "\n", 2)[0])
		}
	}
	_ = os.WriteFile(filepath.Join(outDir, "report.txt"), []byte(t.String()), 0o644)

	// HTML
	var h strings.Builder
	esc := html.EscapeString
	h.WriteString(`<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>OMugen Doctor</title><style>
:root{--bg:#111216;--fg:#ececf1;--mut:#9a9aa8;--card:#1b1c22;--line:#2c2d36;--red:#ff3a3a;--ok:#35c26b;--warn:#f0b429;--err:#ff5c5c}
body{margin:0;background:var(--bg);color:var(--fg);font:15px/1.5 system-ui,Segoe UI,sans-serif}
main{max-width:1100px;margin:0 auto;padding:24px 16px}h1{margin:0 0 4px;font-size:28px}h1 b{color:var(--red)}
.mut{color:var(--mut)}.cards{display:flex;flex-wrap:wrap;gap:10px;margin:18px 0}.card{background:var(--card);border:1px solid var(--line);border-radius:10px;padding:10px 16px;min-width:110px}
.card div{font-size:26px;font-weight:700}.ok{color:var(--ok)}.warning{color:var(--warn)}.error,.crash,.missing,.timeout{color:var(--err)}
table{width:100%;border-collapse:collapse;margin:8px 0 24px}td,th{border-bottom:1px solid var(--line);padding:6px 8px;text-align:left;vertical-align:top}
th{color:var(--mut);font-weight:600}td.n{text-align:right;width:70px;font-variant-numeric:tabular-nums}
details{background:var(--card);border:1px solid var(--line);border-radius:8px;margin:6px 0;padding:6px 10px}summary{cursor:pointer}
pre{white-space:pre-wrap;word-break:break-word;color:var(--mut);margin:6px 0}input{background:var(--card);color:var(--fg);border:1px solid var(--line);border-radius:6px;padding:6px 10px;width:100%;max-width:360px}
</style></head><body><main>`)
	fmt.Fprintf(&h, `<h1><b>O</b>Mugen Doctor</h1><div class="mut">%s &middot; %s &middot; checked %d of %d in %v</div>`,
		esc(wd), esc(rep.Generated), len(items), total, took.Round(time.Second))
	if stopped != "" {
		if stopped == doctorInProgress {
			h.WriteString(`<p class="warning">In progress: partial report, the doctor is still running.</p>`)
		} else {
			fmt.Fprintf(&h, `<p class="error">Stopped at %s (window closed or a load did not finish).</p>`, esc(stopped))
		}
	}
	h.WriteString(`<p class="mut"><span class="error">error / crash / missing</span>: the item cannot be used (or crashes) and needs fixing.
<span class="warning">warning</span>: content notes such as unknown or duplicate parameters, or animation frames pointing at sprites the .sff does not have.
MUGEN tolerates these silently too, so they are usually harmless.</p>`)
	h.WriteString(`<div class="cards">`)
	for _, s := range []string{doctorOK, doctorWarning, doctorMissing, doctorError, doctorCrash, doctorTimeout} {
		if counts[s] == 0 && (s == doctorTimeout || s == doctorMissing) {
			continue
		}
		fmt.Fprintf(&h, `<div class="card"><span class="mut">%s</span><div class="%s">%d</div></div>`, s, s, counts[s])
	}
	h.WriteString(`</div><h2>Most common issues</h2><table><tr><th class="n">Items</th><th>Issue</th><th>Examples</th></tr>`)
	for i, is := range sorted {
		if i >= 100 {
			break
		}
		fmt.Fprintf(&h, `<tr><td class="n">%d</td><td>%s</td><td class="mut">%s</td></tr>`, is.Count, esc(is.Key), esc(strings.Join(is.Items, ", ")))
	}
	h.WriteString(`</table><h2>All items</h2><input id="q" placeholder="Filter by name or status..." oninput="f()"><div id="list">`)
	order := map[string]int{doctorCrash: 0, doctorTimeout: 1, doctorError: 2, doctorMissing: 3, doctorWarning: 4, doctorOK: 5}
	sortedItems := append([]*doctorItem(nil), items...)
	sort.SliceStable(sortedItems, func(i, j int) bool { return order[sortedItems[i].Status] < order[sortedItems[j].Status] })
	for _, it := range sortedItems {
		fmt.Fprintf(&h, `<details data-k="%s"><summary><span class="%s">%s</span> &middot; %s <span class="mut">(%s, select.def line %d, %d ms, %d warnings)</span></summary>`,
			esc(strings.ToLower(it.Entry+" "+it.Status+" "+it.Kind)), it.Status, it.Status, esc(it.Entry), it.Kind, it.Line, it.Millis, len(it.Warnings))
		if it.Def != "" {
			fmt.Fprintf(&h, `<div class="mut">%s</div>`, esc(it.Def))
		}
		if it.Error != "" {
			fmt.Fprintf(&h, `<pre class="%s">%s</pre>`, it.Status, esc(it.Error))
		}
		if len(it.MissingSprite) > 0 {
			fmt.Fprintf(&h, `<pre>Missing sprites:\n%s</pre>`, esc(strings.Join(it.MissingSprite, "\n")))
		}
		if len(it.Warnings) > 0 {
			fmt.Fprintf(&h, `<pre>%s</pre>`, esc(strings.Join(it.Warnings, "\n")))
		}
		h.WriteString(`</details>`)
	}
	h.WriteString(`</div><script>function f(){var q=document.getElementById('q').value.toLowerCase();document.querySelectorAll('#list details').forEach(function(d){d.style.display=d.dataset.k.indexOf(q)>=0?'':'none'})}</script></main></body></html>`)
	path := filepath.Join(outDir, "report.html")
	_ = os.WriteFile(path, []byte(h.String()), 0o644)
	return path
}
