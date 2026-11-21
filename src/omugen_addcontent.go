package main

// Adding characters and stages from inside OMugen. The player drops zip, rar
// or 7z archives, folders or .def files onto the window (or onto OMugen.exe),
// or picks them in Options > Add Characters & Stages (on Android, with the
// phone's file picker). OMugen finds the characters and stages in them,
// copies them into chars/ and stages/, lists them in select.def (the old one
// goes to save/backups first) and restarts, as the roster is read once, at
// startup.
//
// What goes where:
//   - a character is the folder of its .def: chars/<folder>/, listed as
//     "<folder>" (or "<folder>/<file>.def" when the names differ). Its files
//     are found relative to the .def, as in MUGEN, so the folder moves as is.
//   - a stage goes to stages/<name>/: its .def and the files it names
//     (sprites, music, model) where the .def will find them. Paths written
//     from the game folder (stages/x.sff, sound/x.mp3) keep their place.
//   - a new character fills the next free box of the select screen: the
//     first of the empty boxes ("blank", "empty") that end the list, else a
//     new box while the screen has room, else an empty box elsewhere. With
//     none left it is still listed, and the player is told why it can't be
//     seen (the OMugen screenpack's grid grows with the roster).
//
// Archives are unpacked inside the game folder (save/omugen-add), so moving
// their files into place is a rename, on USB drives too. Entries that would
// land outside that folder and links are refused, and programs (.exe, .bat,
// .dll...) are left out: MUGEN content never needs them.

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/bodgit/sevenzip"
	"github.com/nwaples/rardecode/v2"
)

// addStaging is where archives are unpacked (and, on Android, the phone's
// files copied) before their contents move into place.
const addStaging = "save/omugen-add"

const (
	addMaxBytes   = int64(32) << 30 // an archive that unpacks to more is refused
	addMaxFiles   = 200000
	addMaxDefSize = 1 << 20
	addScanDepth  = 8
)

// Programs and scripts the system would run: left out of what is installed.
// (Not .cmd: a MUGEN character's commands.)
var addBlockedExt = map[string]bool{
	".exe": true, ".bat": true, ".com": true, ".scr": true, ".pif": true,
	".vbs": true, ".vbe": true, ".js": true, ".jse": true, ".wsf": true, ".wsh": true,
	".ps1": true, ".psm1": true, ".msi": true, ".msp": true, ".lnk": true, ".url": true,
	".dll": true, ".sys": true, ".jar": true, ".hta": true, ".reg": true, ".cpl": true,
	".apk": true, ".sh": true,
}

type addKind int

const (
	addChar addKind = iota
	addStage
)

// addFile is one file to put in place: src on disk, dst relative to the game
// folder, slash-separated.
type addFile struct{ src, dst string }

type addItem struct {
	kind    addKind
	name    string    // shown to the player
	ref     string    // its select.def entry
	dir     string    // a character's folder, to install as a whole
	target  string    // chars/<folder> or stages/<name>, slash-separated
	files   []addFile // a stage's files (the .def first)
	missing []string  // files a stage names that its source lacks
	move    bool      // unpacked by OMugen: moved, not copied
	exists  bool      // target already there: replaced, the old one kept
	inPlace bool      // already at its target, only missing from select.def
	// A stage in a character's folder: it comes with the character.
	withChar *addItem
	listed   bool   // select.def already lists it
	hidden   bool   // listed, but the select screen has no box for it
	source   string // what the player gave, for messages
}

type addPlan struct {
	game      string // the game folder
	selectDef string // its select.def
	capacity  int    // boxes of the select screen (0: grows with the roster)
	items     []*addItem
	notes     []string // what was left out, and why
	staging   []string // OMugen's unpacked folders and copies, deleted afterwards
	blocked   int      // programs left out
}

// addProgress is what the player sees while OMugen works.
type addProgress struct {
	mu          sync.Mutex
	text        string
	done, total int64
}

func (p *addProgress) set(text string, done, total int64) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.text, p.done, p.total = text, done, total
	p.mu.Unlock()
}

func (p *addProgress) add(n int64) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.done += n
	p.mu.Unlock()
}

func (p *addProgress) get() (string, float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.total <= 0 {
		return p.text, -1
	}
	return p.text, float64(p.done) / float64(p.total)
}

// progressWriter counts what is written.
type progressWriter struct {
	w io.Writer
	p *addProgress
}

func (pw progressWriter) Write(b []byte) (int, error) {
	n, err := pw.w.Write(b)
	pw.p.add(int64(n))
	return n, err
}

// ---------------------------------------------------------------- planning

// addPrepare unpacks and reads what the player gave and plans where it goes.
func addPrepare(game, selectDef string, capacity int, paths []string, prog *addProgress) *addPlan {
	plan := &addPlan{game: game, selectDef: selectDef, capacity: capacity}
	listedChars, listedStages := addListed(selectDef)
	staging, _ := filepath.Abs(filepath.Join(game, filepath.FromSlash(addStaging)))
	for i, p := range paths {
		// The phone's copies (save/omugen-add/incoming-...) go too.
		if rel, err := filepath.Rel(staging, p); err == nil && !strings.HasPrefix(rel, "..") {
			plan.staging = append(plan.staging, filepath.Join(staging, strings.SplitN(rel, string(filepath.Separator), 2)[0]))
		}
		name := filepath.Base(p)
		fi, err := os.Stat(p)
		if err != nil {
			plan.notes = append(plan.notes, fmt.Sprintf("%s: can't be read (%v)", name, err))
			continue
		}
		ext := strings.ToLower(filepath.Ext(p))
		switch {
		case fi.IsDir():
			if addIsGame(p) {
				plan.notes = append(plan.notes, fmt.Sprintf("%s: a whole MUGEN game, not a character or stage", name))
				continue
			}
			plan.scan(p, filepath.Base(p), "", false, name)
		case ext == ".def":
			plan.scan(filepath.Dir(p), filepath.Base(filepath.Dir(p)), p, false, name)
		case ext == ".zip" || ext == ".rar" || ext == ".7z":
			if addIsRarPart(p) {
				plan.notes = append(plan.notes, fmt.Sprintf("%s: a later part of a split archive (add its first part)", name))
				continue
			}
			dst := filepath.Join(game, filepath.FromSlash(addStaging), fmt.Sprintf("%d-%d", time.Now().UnixNano(), i))
			plan.staging = append(plan.staging, dst)
			prog.set("Unpacking "+name+"...", 0, 0)
			blocked, err := addUnpack(p, dst, prog)
			plan.blocked += blocked
			if err != nil {
				plan.notes = append(plan.notes, fmt.Sprintf("%s: %s", name, addErrorText(err)))
				continue
			}
			if addIsGame(dst) || addIsGame(addSingleDir(dst)) {
				plan.notes = append(plan.notes, fmt.Sprintf("%s: a whole MUGEN game, not a character or stage", name))
				continue
			}
			base := strings.TrimSuffix(name, filepath.Ext(name))
			plan.scan(dst, base, "", true, name)
		default:
			plan.notes = append(plan.notes, fmt.Sprintf("%s: not a zip, rar or 7z archive, a folder or a .def file", name))
		}
	}
	prog.set("Reading...", 0, 0)
	// The same folder twice (two downloads of one character): the first.
	used := map[string]*addItem{}
	var items []*addItem
	for _, it := range plan.items {
		if it.withChar != nil {
			if used[strings.ToLower(it.withChar.target)] == it.withChar {
				items = append(items, it)
			}
			continue
		}
		key := strings.ToLower(it.target)
		if first := used[key]; first != nil {
			plan.notes = append(plan.notes, fmt.Sprintf("%s: %s is also in %s, added once", it.source, it.name, first.source))
			continue
		}
		used[key] = it
		items = append(items, it)
	}
	plan.items = items
	for _, it := range plan.items {
		switch it.kind {
		case addChar:
			it.listed = listedChars[addNormRef(it.ref)]
		case addStage:
			it.listed = listedStages[strings.ToLower(it.ref)]
		}
		if it.withChar != nil {
			continue
		}
		target := filepath.Join(game, filepath.FromSlash(it.target))
		if it.kind == addChar && addSamePath(it.dir, target) {
			it.inPlace = true
			continue
		}
		if _, err := os.Stat(target); err == nil {
			it.exists = true
		}
	}
	return plan
}

// scan finds the characters and stages under root (only the given .def when
// only is set). folder names a character whose .def is root's own.
func (plan *addPlan) scan(root, folder, only string, move bool, source string) {
	type def struct {
		path string
		info addDefInfo
	}
	var chars, stages []def
	var other []string
	err := addWalk(root, func(p string) {
		if only != "" && !addSamePath(p, only) {
			return
		}
		info, err := addReadDef(p)
		if err != nil {
			return
		}
		switch info.kind {
		case "char":
			chars = append(chars, def{p, info})
		case "stage":
			stages = append(stages, def{p, info})
		case "screenpack", "lifebars":
			other = append(other, info.kind)
		}
	})
	if err != nil {
		plan.notes = append(plan.notes, fmt.Sprintf("%s: %v", source, err))
		return
	}
	found := 0
	// One character per folder: the .def named like the folder, else the
	// first by name; folders inside a character's own are its files.
	byDir := map[string][]def{}
	var dirs []string
	for _, d := range chars {
		dir := filepath.Dir(d.path)
		if byDir[dir] == nil {
			dirs = append(dirs, dir)
		}
		byDir[dir] = append(byDir[dir], d)
	}
	sort.Slice(dirs, func(i, j int) bool {
		if a, b := strings.Count(dirs[i], string(filepath.Separator)), strings.Count(dirs[j], string(filepath.Separator)); a != b {
			return a < b
		}
		return dirs[i] < dirs[j]
	})
	var taken []*addItem
	// owner is the character whose folder holds p, with p's path in it.
	owner := func(p string) (*addItem, string) {
		for _, t := range taken {
			if rel, err := filepath.Rel(t.dir, p); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return t, rel
			}
		}
		return nil, ""
	}
	for _, dir := range dirs {
		if t, _ := owner(dir); t != nil {
			continue
		}
		defs := byDir[dir]
		sort.Slice(defs, func(i, j int) bool { return strings.ToLower(defs[i].path) < strings.ToLower(defs[j].path) })
		pick := defs[0]
		for _, d := range defs {
			if strings.EqualFold(addBaseNoExt(d.path), filepath.Base(dir)) {
				pick = d
				break
			}
		}
		name := filepath.Base(dir)
		if addSamePath(dir, root) {
			name = folder
			if move {
				// An archive with the character's files at its top: the folder
				// is named like its .def (kfm.def → chars/kfm).
				name = addBaseNoExt(pick.path)
			}
		}
		name = addSafeFolder(name)
		if name == "" {
			name = "character"
		}
		ref := name
		defFile := filepath.Base(pick.path)
		if addBaseNoExt(defFile) != name {
			ref = name + "/" + defFile
		}
		shown := pick.info.name
		if shown == "" {
			shown = name
		}
		it := &addItem{
			kind: addChar, name: shown, ref: ref, dir: dir, target: "chars/" + name,
			move: move, source: source,
		}
		plan.items = append(plan.items, it)
		taken = append(taken, it)
		found++
	}
	for _, d := range stages {
		if char, rel := owner(d.path); char != nil {
			// A stage that comes in a character's folder stays there.
			name := d.info.name
			if name == "" {
				name = addBaseNoExt(d.path)
			}
			plan.items = append(plan.items, &addItem{
				kind: addStage, name: name, ref: char.target + "/" + filepath.ToSlash(rel),
				target: char.target, withChar: char, source: source,
			})
			continue
		}
		plan.items = append(plan.items, addStageItem(root, d.path, d.info, move, source))
		found++
	}
	if found == 0 {
		switch {
		case len(other) > 0:
			plan.notes = append(plan.notes, fmt.Sprintf("%s: a %s, not a character or stage", source, other[0]))
		case only != "":
			plan.notes = append(plan.notes, fmt.Sprintf("%s: not a character's or stage's .def", source))
		default:
			plan.notes = append(plan.notes, fmt.Sprintf("%s: no character or stage in it", source))
		}
	}
}

// addStageItem plans a stage: its .def and the files it names.
func addStageItem(root, defPath string, info addDefInfo, move bool, source string) *addItem {
	name := addSafeFolder(addBaseNoExt(defPath))
	if name == "" {
		name = "stage"
	}
	dir := "stages/" + name
	defDst := dir + "/" + filepath.Base(defPath)
	it := &addItem{kind: addStage, name: info.name, ref: defDst, target: dir, move: move, source: source}
	if it.name == "" {
		it.name = name
	}
	it.files = append(it.files, addFile{defPath, defDst})
	seen := map[string]bool{strings.ToLower(defDst): true}
	for _, v := range info.files {
		dst := addStageDst(dir, v)
		if dst == "" || seen[strings.ToLower(dst)] {
			continue
		}
		seen[strings.ToLower(dst)] = true
		src := addFindRef(root, filepath.Dir(defPath), v)
		if src == "" {
			it.missing = append(it.missing, v)
			continue
		}
		it.files = append(it.files, addFile{src, dst})
	}
	return it
}

// addStageDst is where a file a stage names goes. Paths from the game folder
// (stages/, sound/, data/) stay; others follow the .def to its folder.
func addStageDst(dir, ref string) string {
	ref = strings.TrimSpace(strings.ReplaceAll(ref, "\\", "/"))
	if ref == "" || strings.HasPrefix(ref, "/") || strings.Contains(ref, ":") {
		return ""
	}
	lower := strings.ToLower(ref)
	var dst string
	if strings.HasPrefix(lower, "stages/") || strings.HasPrefix(lower, "sound/") || strings.HasPrefix(lower, "data/") {
		dst = path.Clean(ref)
	} else {
		dst = path.Clean(dir + "/" + ref)
	}
	if dst == "." || strings.HasPrefix(dst, "../") || dst == ".." {
		return ""
	}
	return dst
}

// addFindRef finds a file a .def names: next to the .def, from the top of
// what was given, or anywhere in it by name.
func addFindRef(root, defDir, ref string) string {
	ref = filepath.FromSlash(strings.ReplaceAll(strings.TrimSpace(ref), "\\", "/"))
	for _, base := range []string{defDir, root} {
		if p := addFoldPath(filepath.Join(base, ref)); p != "" {
			return p
		}
	}
	want := strings.ToLower(filepath.Base(ref))
	found := ""
	filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
		if err != nil || found != "" {
			return nil
		}
		if !fi.IsDir() && strings.ToLower(fi.Name()) == want {
			found = p
		}
		return nil
	})
	return found
}

// addFoldPath is p as it is on disk, its letters' case aside ("" if none):
// archives made on Windows name files in any case.
func addFoldPath(p string) string {
	if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
		return p
	}
	dir, base := filepath.Split(filepath.Clean(p))
	dir = filepath.Clean(dir)
	if dir == p || base == "" {
		return ""
	}
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		if dir = addFoldDir(dir); dir == "" {
			return ""
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() && strings.EqualFold(e.Name(), base) {
			return filepath.Join(dir, e.Name())
		}
	}
	return ""
}

func addFoldDir(p string) string {
	if fi, err := os.Stat(p); err == nil && fi.IsDir() {
		return p
	}
	parent, base := filepath.Split(filepath.Clean(p))
	parent = filepath.Clean(parent)
	if parent == p || base == "" {
		return ""
	}
	if parent = addFoldDir(parent); parent == "" {
		return ""
	}
	entries, err := os.ReadDir(parent)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() && strings.EqualFold(e.Name(), base) {
			return filepath.Join(parent, e.Name())
		}
	}
	return ""
}

// addWalk calls fn for every .def under root (a few folders deep).
func addWalk(root string, fn func(string)) error {
	files := 0
	return filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			if p == root {
				return err
			}
			return nil
		}
		if fi.IsDir() {
			if rel, err := filepath.Rel(root, p); err == nil && rel != "." &&
				strings.Count(rel, string(filepath.Separator)) >= addScanDepth {
				return filepath.SkipDir
			}
			return nil
		}
		if files++; files > addMaxFiles {
			return errors.New("too many files")
		}
		if fi.Mode().IsRegular() && strings.EqualFold(filepath.Ext(p), ".def") {
			fn(p)
		}
		return nil
	})
}

// addIsGame tells a whole MUGEN game (its data folder with select.def or
// mugen.cfg, next to chars) from a character or stage.
func addIsGame(dir string) bool {
	if dir == "" {
		return false
	}
	data := addFoldDir(filepath.Join(dir, "data"))
	if data == "" || addFoldDir(filepath.Join(dir, "chars")) == "" {
		return false
	}
	return addFoldPath(filepath.Join(data, "select.def")) != "" || addFoldPath(filepath.Join(data, "mugen.cfg")) != ""
}

// addSingleDir is the only folder in dir, if dir holds nothing else.
func addSingleDir(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || !entries[0].IsDir() {
		return ""
	}
	return filepath.Join(dir, entries[0].Name())
}

// addIsRarPart reports a split rar's later parts (name.part2.rar...), which
// the first part reads.
func addIsRarPart(p string) bool {
	lower := strings.ToLower(filepath.Base(p))
	if !strings.HasSuffix(lower, ".rar") {
		return false
	}
	lower = strings.TrimSuffix(lower, ".rar")
	i := strings.LastIndex(lower, ".part")
	if i < 0 {
		return false
	}
	n, err := strconv.Atoi(lower[i+5:])
	return err == nil && n > 1
}

// ---------------------------------------------------------------- .def files

type addDefInfo struct {
	kind  string   // "char", "stage", "screenpack", "lifebars", "storyboard" or ""
	name  string   // [Info] displayname or name
	files []string // a stage's sprites, music and model
}

// addReadDef reads what a .def is, and its name.
func addReadDef(p string) (addDefInfo, error) {
	var info addDefInfo
	fi, err := os.Stat(p)
	if err != nil {
		return info, err
	}
	if fi.Size() > addMaxDefSize {
		return info, errors.New("too big for a .def")
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return info, err
	}
	b = bytes.TrimPrefix(b, []byte("\xef\xbb\xbf"))
	secs := map[string]map[string]string{}
	cur := ""
	for _, line := range strings.Split(strings.ReplaceAll(string(b), "\r", "\n"), "\n") {
		if i := strings.Index(line, ";"); i >= 0 {
			line = line[:i]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") {
			end := strings.Index(line, "]")
			if end < 0 {
				end = len(line)
			}
			cur = strings.ToLower(strings.Join(strings.Fields(line[1:end]), " "))
			if secs[cur] == nil {
				secs[cur] = map[string]string{}
			}
			continue
		}
		eq := strings.Index(line, "=")
		if eq < 0 || cur == "" {
			continue
		}
		k := strings.ToLower(strings.TrimSpace(line[:eq]))
		v := strings.Trim(strings.TrimSpace(line[eq+1:]), "\"")
		if _, ok := secs[cur][k]; !ok {
			secs[cur][k] = strings.TrimSpace(v)
		}
		if k == "spr" || k == "sprite" || k == "model" || strings.HasPrefix(k, "bgmusic") {
			if cur == "bgdef" || cur == "music" || cur == "model" || cur == "stageinfo" {
				if v := strings.TrimSpace(v); v != "" {
					info.files = append(info.files, v)
				}
			}
		}
	}
	files := secs["files"]
	has := func(m map[string]string, keys ...string) bool {
		for _, k := range keys {
			if _, ok := m[k]; ok {
				return true
			}
		}
		return false
	}
	_, hasStageInfo := secs["stageinfo"]
	_, hasBg := secs["bgdef"]
	_, hasCamera := secs["camera"]
	_, hasPlayers := secs["playerinfo"]
	_, hasScene := secs["scenedef"]
	_, hasLifebar := secs["lifebar"]
	switch {
	case files != nil && has(files, "select", "fight"):
		info.kind = "screenpack"
	case hasLifebar || (files != nil && has(files, "fightfx.sff", "fightfx.air")):
		info.kind = "lifebars"
	case files != nil && has(files, "cmd", "cns", "st") && has(files, "sprite", "anim"):
		info.kind = "char"
	case hasScene:
		info.kind = "storyboard"
	case hasStageInfo || (hasBg && (hasCamera || hasPlayers)):
		info.kind = "stage"
	}
	if info.kind != "stage" {
		info.files = nil
	}
	in := secs["info"]
	for _, k := range []string{"displayname", "name"} {
		if v := in[k]; v != "" && utf8.ValidString(v) {
			info.name = v
			break
		}
	}
	return info, nil
}

// ---------------------------------------------------------------- archives

// addUnpack unpacks an archive into dst. It returns how many programs it
// left out.
func addUnpack(archive, dst string, prog *addProgress) (int, error) {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return 0, err
	}
	switch strings.ToLower(filepath.Ext(archive)) {
	case ".zip":
		return addUnzip(archive, dst, prog)
	case ".rar":
		return addUnrar(archive, dst, prog)
	case ".7z":
		return addUn7z(archive, dst, prog)
	}
	return 0, errors.New("not an archive")
}

var (
	errAddPassword = errors.New("password-protected")
	errAddTooBig   = errors.New("too big")
)

func addErrorText(err error) string {
	switch {
	case errors.Is(err, errAddPassword):
		return "it's password-protected: unpack it yourself, then add the folder"
	case errors.Is(err, errAddTooBig):
		return "it unpacks to more than 32 GB"
	case errors.Is(err, rardecode.ErrArchiveEncrypted), errors.Is(err, rardecode.ErrArchivedFileEncrypted):
		return "it's password-protected: unpack it yourself, then add the folder"
	}
	return fmt.Sprintf("can't be unpacked (%v)", err)
}

// addEntryPath is where an archive's entry goes under dst ("" to skip it):
// never outside dst, with names every system accepts.
func addEntryPath(dst, name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	var parts []string
	for _, part := range strings.Split(name, "/") {
		switch part {
		case "", ".":
			continue
		case "..":
			return ""
		}
		if len(parts) == 0 && strings.Contains(part, ":") {
			return "" // a drive letter
		}
		part = addSafeName(part)
		if part == "" {
			return ""
		}
		parts = append(parts, part)
	}
	if len(parts) == 0 {
		return ""
	}
	return filepath.Join(append([]string{dst}, parts...)...)
}

// addSafeName makes a file name valid on Windows, Android and Linux.
func addSafeName(s string) string {
	s = strings.Map(func(r rune) rune {
		switch {
		case r < 32, strings.ContainsRune(`<>:"/\|?*`, r):
			return '_'
		}
		return r
	}, s)
	s = strings.TrimRight(s, ". ")
	if s == "" {
		return ""
	}
	upper := strings.ToUpper(strings.SplitN(s, ".", 2)[0])
	switch upper {
	case "CON", "PRN", "AUX", "NUL",
		"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
		s = "_" + s
	}
	return s
}

// addSafeFolder is a folder name for chars/ or stages/.
func addSafeFolder(s string) string {
	return strings.TrimSpace(addSafeName(strings.TrimSpace(s)))
}

// addBlocked reports the programs left out of what is installed.
func addBlocked(name string) bool {
	return addBlockedExt[strings.ToLower(path.Ext(strings.ReplaceAll(name, "\\", "/")))]
}

// addWriteEntry writes one unpacked file.
func addWriteEntry(target string, r io.Reader, prog *addProgress) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(progressWriter{out, prog}, r); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func addUnzip(archive, dst string, prog *addProgress) (int, error) {
	zr, err := zip.OpenReader(archive)
	if err != nil {
		return 0, err
	}
	defer zr.Close()
	var total int64
	for _, f := range zr.File {
		total += int64(f.UncompressedSize64)
	}
	if total > addMaxBytes || len(zr.File) > addMaxFiles {
		return 0, errAddTooBig
	}
	prog.set("Unpacking "+filepath.Base(archive)+"...", 0, total)
	blocked := 0
	for _, f := range zr.File {
		if f.Flags&0x1 != 0 {
			return blocked, errAddPassword
		}
		if !f.Mode().IsRegular() { // folders, links
			continue
		}
		if addBlocked(f.Name) {
			blocked++
			continue
		}
		target := addEntryPath(dst, f.Name)
		if target == "" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return blocked, err
		}
		err = addWriteEntry(target, io.LimitReader(rc, int64(f.UncompressedSize64)+1), prog)
		rc.Close()
		if err != nil {
			return blocked, err
		}
	}
	return blocked, nil
}

func addUnrar(archive, dst string, prog *addProgress) (int, error) {
	rr, err := rardecode.OpenReader(archive)
	if err != nil {
		return 0, err
	}
	defer rr.Close()
	if fi, err := os.Stat(archive); err == nil {
		// Packed sizes: rar headers come one at a time.
		prog.set("Unpacking "+filepath.Base(archive)+"...", 0, fi.Size())
	}
	var written int64
	files, blocked := 0, 0
	for {
		h, err := rr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return blocked, err
		}
		if h.Encrypted {
			return blocked, errAddPassword
		}
		if h.IsDir || !h.Mode().IsRegular() {
			continue
		}
		if files++; files > addMaxFiles {
			return blocked, errAddTooBig
		}
		if addBlocked(h.Name) {
			blocked++
			continue
		}
		target := addEntryPath(dst, h.Name)
		if target == "" {
			continue
		}
		limit := addMaxBytes - written
		var counted countWriter
		err = addWriteEntry(target, io.TeeReader(io.LimitReader(rr, limit+1), &counted), nil)
		if err != nil {
			return blocked, err
		}
		written += counted.n
		if written > addMaxBytes {
			return blocked, errAddTooBig
		}
		prog.add(h.PackedSize)
	}
	return blocked, nil
}

type countWriter struct{ n int64 }

func (c *countWriter) Write(b []byte) (int, error) {
	c.n += int64(len(b))
	return len(b), nil
}

func addUn7z(archive, dst string, prog *addProgress) (int, error) {
	zr, err := sevenzip.OpenReader(archive)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "password") {
			return 0, errAddPassword
		}
		return 0, err
	}
	defer zr.Close()
	var total int64
	for _, f := range zr.File {
		total += int64(f.UncompressedSize)
	}
	if total > addMaxBytes || len(zr.File) > addMaxFiles {
		return 0, errAddTooBig
	}
	prog.set("Unpacking "+filepath.Base(archive)+"...", 0, total)
	blocked := 0
	// In order: a solid archive is decoded once, front to back.
	for _, f := range zr.File {
		if !f.FileInfo().Mode().IsRegular() {
			continue
		}
		if addBlocked(f.Name) {
			blocked++
			continue
		}
		target := addEntryPath(dst, f.Name)
		if target == "" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "password") {
				return blocked, errAddPassword
			}
			return blocked, err
		}
		err = addWriteEntry(target, io.LimitReader(rc, int64(f.UncompressedSize)+1), prog)
		rc.Close()
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "password") {
				return blocked, errAddPassword
			}
			return blocked, err
		}
	}
	return blocked, nil
}

// ---------------------------------------------------------------- select.def

// addNormRef is a select.def character entry as a key: "kfm", "kfm/kfm.def"
// and "chars/kfm/kfm.def" are the same character.
func addNormRef(ref string) string {
	ref = strings.ToLower(strings.TrimSpace(strings.ReplaceAll(ref, "\\", "/")))
	ref = strings.TrimPrefix(ref, "chars/")
	if strings.HasSuffix(ref, ".def") {
		dir, file := path.Split(ref)
		dir = strings.TrimSuffix(dir, "/")
		if dir != "" && strings.TrimSuffix(file, ".def") == path.Base(dir) {
			return dir
		}
		return ref
	}
	return ref
}

// addListed reads the characters and stages select.def lists.
func addListed(selectDef string) (chars, stages map[string]bool) {
	chars, stages = map[string]bool{}, map[string]bool{}
	b, err := os.ReadFile(selectDef)
	if err != nil {
		return
	}
	section := ""
	for _, line := range strings.Split(strings.ReplaceAll(string(b), "\r", "\n"), "\n") {
		s := addLineSection(line)
		if s != "" {
			section = s
			continue
		}
		first := addFirstToken(line)
		if first == "" {
			continue
		}
		switch section {
		case "characters":
			chars[addNormRef(first)] = true
		case "extrastages":
			stages[strings.ToLower(strings.ReplaceAll(first, "\\", "/"))] = true
		}
	}
	return
}

// addLineSection is the lowercase name of the section a line starts ("" if
// it starts none).
func addLineSection(line string) string {
	t := strings.TrimSpace(line)
	if !strings.HasPrefix(t, "[") {
		return ""
	}
	end := strings.Index(t, "]")
	if end < 0 {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(t[1:end]))
}

// addFirstToken is a select.def line's entry, without comments and
// parameters ("" for none).
func addFirstToken(line string) string {
	if i := strings.Index(line, ";"); i >= 0 {
		line = line[:i]
	}
	if i := strings.Index(line, ","); i >= 0 {
		line = line[:i]
	}
	return strings.TrimSpace(line)
}

// addIsPlaceholder reports an empty box of the select screen.
func addIsPlaceholder(entry string) bool {
	switch strings.ToLower(entry) {
	case "blank", "empty", "emptyslot", "dummyslot":
		return true
	}
	return false
}

// addEditSelect lists the new characters and stages in a select.def's text.
// It marks the characters the select screen has no box for.
func addEditSelect(text string, items []*addItem, capacity int) string {
	nl := "\n"
	if strings.Contains(text, "\r\n") {
		nl = "\r\n"
	}
	endsWithNewline := strings.HasSuffix(text, "\n")
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	if endsWithNewline {
		lines = lines[:len(lines)-1]
	}
	var chars, stages []*addItem
	for _, it := range items {
		if it.listed {
			continue
		}
		if it.kind == addChar {
			chars = append(chars, it)
		} else {
			stages = append(stages, it)
		}
	}
	insert := func(at int, s string) {
		lines = append(lines, "")
		copy(lines[at+1:], lines[at:])
		lines[at] = s
	}
	// section finds a section: its header line and the line after its last
	// entry (-1, -1 if missing).
	section := func(name string) (head, end int) {
		head, end = -1, -1
		for i, line := range lines {
			if s := addLineSection(line); s != "" {
				if head >= 0 {
					break
				}
				if s == name {
					head, end = i, i+1
				}
				continue
			}
			if head >= 0 && addFirstToken(line) != "" {
				end = i + 1
			}
		}
		return
	}
	if len(chars) > 0 {
		head, end := section("characters")
		if head < 0 {
			lines = append(lines, "", "[Characters]")
			head, end = len(lines)-1, len(lines)
		}
		// The boxes: entries (a slot = {...} group is one), the empty ones,
		// and the empty and random boxes that end the list.
		var placeholders, trailing []int
		at := -1 // where new boxes go: before the random boxes that end the list
		boxes, inSlot := 0, false
		for i := head + 1; i < end; i++ {
			first := strings.ToLower(addFirstToken(lines[i]))
			switch {
			case first == "":
			case inSlot:
				if first == "}" {
					inSlot = false
				}
			case strings.HasPrefix(first, "slot") && strings.HasSuffix(strings.ReplaceAll(first, " ", ""), "={"):
				inSlot = true
				boxes++
				trailing, at = nil, -1
			case strings.Contains(strings.ToLower(lines[i]), "exclude") && addExcluded(lines[i]):
			case addIsPlaceholder(first):
				boxes++
				placeholders = append(placeholders, i)
				trailing = append(trailing, i)
				at = -1
			case first == "randomselect":
				boxes++
				if at < 0 {
					at = i
				}
			default:
				boxes++
				trailing, at = nil, -1
			}
		}
		if at < 0 {
			at = end
		}
		used := map[int]bool{}
		for _, it := range chars {
			switch {
			case len(trailing) > 0:
				i := trailing[0]
				trailing = trailing[1:]
				lines[i] = it.ref
				used[i] = true
			case capacity <= 0 || boxes < capacity:
				insert(at, it.ref)
				at++
				end++
				boxes++
			default:
				free := -1
				for j := len(placeholders) - 1; j >= 0; j-- {
					if !used[placeholders[j]] {
						free = placeholders[j]
						break
					}
				}
				if free >= 0 {
					lines[free] = it.ref
					used[free] = true
				} else {
					insert(end, it.ref)
					end++
					it.hidden = true
				}
			}
		}
	}
	if len(stages) > 0 {
		head, end := section("extrastages")
		if head < 0 {
			lines = append(lines, "", "[ExtraStages]")
			end = len(lines)
		}
		for _, it := range stages {
			insert(end, it.ref)
			end++
		}
	}
	out := strings.Join(lines, nl)
	if endsWithNewline || len(lines) > 0 {
		out += nl
	}
	return out
}

// addExcluded reports a select.def line with exclude = 1 (no box).
func addExcluded(line string) bool {
	if i := strings.Index(line, ";"); i >= 0 {
		line = line[:i]
	}
	for _, p := range strings.Split(line, ",")[1:] {
		kv := strings.SplitN(p, "=", 2)
		if len(kv) == 2 && strings.EqualFold(strings.TrimSpace(kv[0]), "exclude") && strings.TrimSpace(kv[1]) == "1" {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------- installing

// addResult is what was installed.
type addResult struct {
	added, replaced []*addItem
	failed          []string
	changed         bool // select.def or the game's folders changed: restart
}

// apply puts everything in place and lists it in select.def.
func (plan *addPlan) apply(prog *addProgress) *addResult {
	res := &addResult{}
	var total int64
	for _, it := range plan.items {
		if !it.move && !it.inPlace {
			total += addSize(it)
		}
	}
	prog.set("Adding to the game...", 0, total)
	stamp := time.Now().Format("2006-01-02_15-04-05")
	var done []*addItem
	installed := map[*addItem]bool{}
	for _, it := range plan.items {
		if it.inPlace || it.withChar != nil && installed[it.withChar] {
			done = append(done, it)
			installed[it] = true
			continue
		}
		if it.withChar != nil {
			continue // its character wasn't added
		}
		target := filepath.Join(plan.game, filepath.FromSlash(it.target))
		if it.exists {
			backup := filepath.Join(plan.game, "save", "backups", filepath.FromSlash(it.target)+"."+stamp)
			err := os.MkdirAll(filepath.Dir(backup), 0o755)
			if err == nil {
				err = os.Rename(target, backup)
			}
			if err != nil {
				res.failed = append(res.failed, fmt.Sprintf("%s: the old copy can't be moved aside (%v)", it.name, err))
				continue
			}
		}
		var err error
		switch it.kind {
		case addChar:
			err = addInstallDir(it.dir, target, it.move, prog)
		case addStage:
			for _, f := range it.files {
				dst := filepath.Join(plan.game, filepath.FromSlash(f.dst))
				if err = addInstallFile(f.src, dst, it.move, prog); err != nil {
					break
				}
			}
		}
		res.changed = true
		if err != nil {
			res.failed = append(res.failed, fmt.Sprintf("%s: %v", it.name, err))
			continue
		}
		done = append(done, it)
		installed[it] = true
	}
	if len(done) > 0 {
		prog.set("Updating select.def...", 0, 0)
		if err := plan.writeSelect(done, stamp); err != nil {
			res.failed = append(res.failed, fmt.Sprintf("select.def: %v", err))
		} else {
			res.changed = true
		}
		for _, it := range done {
			if it.exists {
				res.replaced = append(res.replaced, it)
			} else {
				res.added = append(res.added, it)
			}
		}
	}
	plan.cleanup()
	return res
}

func (plan *addPlan) writeSelect(items []*addItem, stamp string) error {
	orig, err := os.ReadFile(plan.selectDef)
	if err != nil {
		return err
	}
	updated := []byte(addEditSelect(string(orig), items, plan.capacity))
	if bytes.Equal(orig, updated) {
		return nil
	}
	backupDir := filepath.Join(plan.game, "save", "backups")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return err
	}
	backup := filepath.Join(backupDir, filepath.Base(plan.selectDef)+"."+stamp)
	if err := os.WriteFile(backup, orig, 0o644); err != nil {
		return err
	}
	tmp := plan.selectDef + ".omugen-tmp"
	if err := os.WriteFile(tmp, updated, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, plan.selectDef)
}

// cleanup deletes what OMugen unpacked.
func (plan *addPlan) cleanup() {
	for _, d := range plan.staging {
		os.RemoveAll(d)
	}
	plan.staging = nil
	// The staging folder itself, once empty (the phone's copies too).
	staging := filepath.Join(plan.game, filepath.FromSlash(addStaging))
	if entries, err := os.ReadDir(staging); err == nil && len(entries) == 0 {
		os.Remove(staging)
	}
}

func addSize(it *addItem) int64 {
	var n int64
	if it.kind == addChar {
		filepath.Walk(it.dir, func(p string, fi os.FileInfo, err error) error {
			if err == nil && fi.Mode().IsRegular() && !addBlocked(p) {
				n += fi.Size()
			}
			return nil
		})
		return n
	}
	for _, f := range it.files {
		if fi, err := os.Stat(f.src); err == nil {
			n += fi.Size()
		}
	}
	return n
}

// addInstallDir moves (or copies) a character's folder to target.
func addInstallDir(src, target string, move bool, prog *addProgress) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	if move {
		if err := os.Rename(src, target); err == nil {
			return nil
		}
	}
	return filepath.Walk(src, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		dst := filepath.Join(target, rel)
		switch {
		case fi.IsDir():
			return os.MkdirAll(dst, 0o755)
		case !fi.Mode().IsRegular(), addBlocked(p):
			return nil
		}
		return addCopy(p, dst, prog)
	})
}

// addInstallFile moves (or copies) one file to dst, replacing what is there.
func addInstallFile(src, dst string, move bool, prog *addProgress) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if move {
		os.Remove(dst)
		if err := os.Rename(src, dst); err == nil {
			return nil
		}
	}
	return addCopy(src, dst, prog)
}

func addCopy(src, dst string, prog *addProgress) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	return addWriteEntry(dst, in, prog)
}

// ---------------------------------------------------------------- messages

// summary is the question before anything is installed.
func (plan *addPlan) summary() string {
	var b strings.Builder
	var add, replace, list []string
	for _, it := range plan.items {
		s := it.name + addKindText(it)
		switch {
		case it.inPlace:
			if !it.listed {
				list = append(list, s)
			}
		case it.withChar != nil:
			if !it.listed {
				add = append(add, s)
			}
		case it.exists:
			replace = append(replace, s)
		default:
			add = append(add, s)
		}
	}
	section := func(title string, lines []string) {
		if len(lines) == 0 {
			return
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(title + "\n")
		for i, s := range lines {
			if i == 12 && len(lines) > 13 {
				fmt.Fprintf(&b, "  and %d more\n", len(lines)-12)
				break
			}
			b.WriteString("  • " + s + "\n")
		}
	}
	section("Add to the game:", add)
	section("Replace (the old copy is kept in save/backups):", replace)
	section("Already in the game's folders, add to the select screen:", list)
	section("Left out:", plan.notes)
	if plan.blocked > 0 {
		fmt.Fprintf(&b, "\n%d program file(s) (.exe, .bat...) are left out: MUGEN doesn't use them.\n", plan.blocked)
	}
	b.WriteString("\nOMugen restarts afterwards to load them.")
	return b.String()
}

// wanted reports whether the plan changes anything.
func (plan *addPlan) wanted() bool {
	for _, it := range plan.items {
		if it.inPlace || it.withChar != nil {
			if !it.listed {
				return true
			}
			continue
		}
		return true // files to put in place
	}
	return false
}

// nothingText says why nothing can be added.
func (plan *addPlan) nothingText() string {
	var b strings.Builder
	b.WriteString("Nothing to add.\n")
	for _, it := range plan.items {
		if it.listed {
			fmt.Fprintf(&b, "\n  • %s%s is already in the game", it.name, addKindText(it))
		}
	}
	for _, n := range plan.notes {
		b.WriteString("\n  • " + n)
	}
	b.WriteString("\n\nOMugen adds characters and stages from zip, rar and 7z archives, folders and .def files.")
	return b.String()
}

func addKindText(it *addItem) string {
	if it.kind == addStage {
		return " (stage)"
	}
	return ""
}

// text is the report once everything is installed.
func (res *addResult) text() string {
	var b strings.Builder
	names := func(items []*addItem) string {
		var s []string
		for i, it := range items {
			if i == 15 {
				s = append(s, fmt.Sprintf("and %d more", len(items)-15))
				break
			}
			s = append(s, it.name+addKindText(it))
		}
		return strings.Join(s, ", ")
	}
	if len(res.added) > 0 {
		b.WriteString("Added: " + names(res.added) + ".\n")
	}
	if len(res.replaced) > 0 {
		b.WriteString("Replaced: " + names(res.replaced) + " (the old copies are in save/backups).\n")
	}
	var hidden []*addItem
	var missing []string
	for _, it := range append(append([]*addItem{}, res.added...), res.replaced...) {
		if it.hidden {
			hidden = append(hidden, it)
		}
		for _, m := range it.missing {
			missing = append(missing, it.name+": "+m)
		}
	}
	if len(hidden) > 0 {
		b.WriteString("\nThe select screen has no free box for " + names(hidden) + ". They're in select.def, " +
			"and the OMugen screenpack (Options > Video Settings) shows everyone; or turn an entry of " +
			"select.def into theirs.\n")
	}
	if len(missing) > 0 {
		b.WriteString("\nFiles the stage names but its download lacks: " + strings.Join(missing, ", ") + ".\n")
	}
	if len(res.failed) > 0 {
		b.WriteString("\nNot added:\n")
		for _, f := range res.failed {
			b.WriteString("  • " + f + "\n")
		}
	}
	if res.changed {
		b.WriteString("\nOMugen restarts now to load them.")
	}
	return strings.TrimSpace(b.String())
}

// ---------------------------------------------------------------- paths

func addBaseNoExt(p string) string {
	b := filepath.Base(p)
	return strings.TrimSuffix(b, filepath.Ext(b))
}

func addSamePath(a, b string) bool {
	aa, err1 := filepath.Abs(a)
	bb, err2 := filepath.Abs(b)
	if err1 != nil || err2 != nil {
		return false
	}
	return strings.EqualFold(filepath.Clean(aa), filepath.Clean(bb))
}
