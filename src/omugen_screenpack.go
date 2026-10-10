package main

// OMugen's built-in screenpack (data/omugen, shipped inside OMugen.exe).
// It replaces the game's screenpack when Config.OMugenScreenpack = 1 (or
// with -omugenui), and takes over when the game's screenpack or lifebars fail
// to load. The game's own select.def and fight.def keep being used.

import (
	"fmt"
	"strings"

	lua "github.com/yuin/gopher-lua"
	"gopkg.in/ini.v1"
)

const (
	omugenMotifDef = "data/omugen/system.def"
	// A folder name no game uses, so game paths looked up relative to the
	// OMugen screenpack (like "LIFEBARS/fight.sff") can't land in it.
	omugenFightDef = "data/omugen/omugen-fallback-lifebars/fight.def"
)

// fightMotifDef is the motif that lifebar files are looked up next to: the
// game's own screenpack, also when the OMugen screenpack replaces it, as
// MUGEN looks next to the game's motif.
func fightMotifDef() string {
	if omugenUI.gameDef != "" && isOMugenMotif(sys.motif.Def) {
		return omugenUI.gameDef
	}
	return sys.motif.Def
}

var omugenUI struct {
	gameDef string // the game's own motif when the OMugen screenpack replaces it
	notice  string // shown once on the title screen after a fallback
}

func omugenScreenpackWanted() bool {
	if _, ok := sys.cmdFlags["-omugenui"]; ok {
		return true
	}
	return sys.cfg.Config.OMugenScreenpack == 1
}

func omugenScreenpackPath() string {
	return FileExist(omugenMotifDef)
}

func isOMugenMotif(def string) bool {
	return strings.HasSuffix(strings.ToLower(mugenSlash(def)), omugenMotifDef)
}

// omugenScreenpackFiles points the OMugen screenpack at the game's own
// select.def, lifebars and sounds, as named by the game's motif.
func omugenScreenpackFiles(m *Motif) {
	if !isOMugenMotif(m.Def) {
		return
	}
	game := omugenUI.gameDef
	sel, fight, snd := "select.def", "fight.def", "system.snd"
	if game != "" {
		if v := readMotifFilesKey(game, "select"); v != "" {
			sel = v
		}
		if v := readMotifFilesKey(game, "fight"); v != "" {
			fight = v
		}
		if v := readMotifFilesKey(game, "snd"); v != "" {
			snd = v
		}
	}
	dirs := []string{game, "", "data/"}
	fightDirs := dirs
	if game == "" {
		// The game's own screenpack is this one (games made with -newgame):
		// its system.def names the game's lifebars (data/omugen/lifebars).
		if v := readMotifFilesKey(m.Def, "fight"); v != "" {
			if p := SearchFile(v, []string{m.Def}); FileExist(p) != "" {
				fight, fightDirs = p, []string{""}
			}
		}
	}
	if p := SearchFile(sel, dirs); FileExist(p) != "" {
		m.Files.Select = p
	}
	if p := SearchFile(fight, fightDirs); FileExist(p) != "" {
		m.Files.Fight = p
	} else if p := FileExist(omugenFightDef); p != "" {
		m.Files.Fight = p
	}
	// MUGEN screenpacks keep the standard menu sounds (100,0 move,
	// 100,1 done, 100,2 cancel), which the OMugen screenpack uses.
	if p := SearchFile(snd, dirs); FileExist(p) != "" {
		m.Files.Snd = p
	}
	LogMessage("OMugen screenpack: game motif %q: select %q, lifebars %q, sounds %q",
		game, m.Files.Select, m.Files.Fight, m.Files.Snd)
}

// omugenScreenpackMusic plays the game's own menu music in the OMugen
// screenpack.
func omugenScreenpackMusic(m *Motif) {
	if !isOMugenMotif(m.Def) || omugenUI.gameDef == "" {
		return
	}
	def := SearchFile(omugenUI.gameDef, []string{"", "data/"})
	text, err := LoadText(def)
	if err != nil {
		return
	}
	f, err := LoadINIText(preprocessINIContent(NormalizeNewlines(text)), ini.LoadOptions{
		InsensitiveSections:     true,
		SkipUnrecognizableLines: true,
		UnparseableSections:     []string{"Infobox Text"},
	})
	if err != nil {
		return
	}
	sec := pickLangSectionMerged(f, "Music")
	if sec == nil || len(sec.Keys()) == 0 {
		return
	}
	// Music paths may be relative to the game's motif folder.
	for _, k := range sec.Keys() {
		if !strings.HasSuffix(strings.ToLower(k.Name()), "bgm") {
			continue
		}
		items := strings.Split(k.Value(), ",")
		for i, it := range items {
			it = strings.TrimSpace(it)
			if it == "" {
				continue
			}
			if p := SearchFile(it, []string{def, "", "data/"}); FileExist(p) != "" {
				items[i] = p
			}
		}
		k.SetValue(strings.Join(items, ","))
	}
	m.Music = parseMusicSection(sec)
}

// readMotifFilesKey reads one [Files] entry of a motif without loading it.
func readMotifFilesKey(def, key string) string {
	return readMotifKey(def, "Files", key)
}

func readMotifKey(def, section, key string) string {
	p := SearchFile(def, []string{"", "data/"})
	text, err := LoadText(p)
	if err != nil {
		return ""
	}
	lines := SplitAndTrim(NormalizeNewlines(text), "\n")
	for i := 0; i < len(lines); {
		is, name, _ := ReadIniSection(lines, &i)
		if len(name) == 0 {
			break
		}
		if strings.EqualFold(name, section) {
			v := strings.TrimSpace(is[key])
			if sc := strings.Index(v, ";"); sc >= 0 {
				v = strings.TrimSpace(v[:sc])
			}
			return v
		}
	}
	return ""
}

// omugenFightLocalcoord is the localcoord for lifebars that don't set one.
// They follow the screenpack's; with the OMugen screenpack that is the
// game's own screenpack (320x240 when it has none, as in MUGEN).
func omugenFightLocalcoord() [2]int32 {
	if !isOMugenMotif(sys.motif.Def) {
		// Quick VS (-p1/-p2) loads the lifebars before the screenpack (its
		// fight path is still unset): read the localcoord from its file, as
		// MUGEN does in every mode.
		if sys.motif.Files.Fight == "" {
			return motifDefLocalcoord(sys.motif.Def)
		}
		return [2]int32{sys.motif.Info.Localcoord[0], sys.motif.Info.Localcoord[1]}
	}
	lc := [2]int32{320, 240}
	if omugenUI.gameDef != "" {
		lc = motifDefLocalcoord(omugenUI.gameDef)
	}
	LogMessage("OMugen screenpack: lifebars without a localcoord use %dx%d", lc[0], lc[1])
	return lc
}

// motifDefLocalcoord reads [Info] localcoord from a screenpack's file
// (320x240 when it sets none).
func motifDefLocalcoord(def string) [2]int32 {
	if def != "" {
		if v := readMotifKey(def, "Info", "localcoord"); v != "" {
			if p := strings.Split(v, ","); len(p) == 2 {
				if w, h := Atoi(strings.TrimSpace(p[0])), Atoi(strings.TrimSpace(p[1])); w > 0 && h > 0 {
					return [2]int32{w, h}
				}
			}
		}
	}
	return [2]int32{320, 240}
}

// loadMotifSafe is loadMotif with panics turned into errors, so a broken
// screenpack can fall back to the OMugen one instead of closing the game.
func loadMotifSafe(def string) (m *Motif, err error) {
	defer func() {
		if r := recover(); r != nil {
			m, err = nil, fmt.Errorf("%v", r)
		}
	}()
	return loadMotif(def)
}

// omugenMotifFallback loads the OMugen screenpack after the game's failed.
func omugenMotifFallback(failed error) (*Motif, error) {
	p := omugenScreenpackPath()
	if p == "" || isOMugenMotif(sys.motif.Def) {
		return nil, failed
	}
	LogMessage("OMugen: the screenpack %s could not be loaded: %v", sys.motif.Def, failed)
	LogMessage("OMugen: using the OMugen screenpack instead")
	omugenUI.gameDef = sys.motif.Def
	sys.motif.Def = p
	m, err := loadMotifSafe(p)
	if err != nil {
		return nil, fmt.Errorf("%v (the OMugen screenpack failed too: %v)", failed, err)
	}
	// The engine's old default (not shipped) is replaced quietly.
	if !strings.EqualFold(mugenSlash(omugenUI.gameDef), legacyDefaultMotif) {
		// The reason on screen too (a phone's log is out of reach).
		why := strings.TrimSpace(strings.SplitN(failed.Error(), "\n", 2)[0])
		if len(why) > 90 {
			why = why[:87] + "..."
		}
		omugenUI.notice = fmt.Sprintf("The screenpack %s could not be loaded\n(%s),\nso OMugen's built-in screenpack is used.",
			mugenSlash(omugenUI.gameDef), why)
	}
	return m, nil
}

// legacyDefaultMotif is the default screenpack of config files written
// before OMugen had its own.
const legacyDefaultMotif = "data/ikemen1/system.def"

// omugenFightFallback loads OMugen's own lifebars after the game's failed.
func omugenFightFallback(def string, failed error) (*FightScreen, error) {
	p := FileExist(omugenFightDef)
	if p == "" || strings.EqualFold(mugenSlash(def), p) {
		return nil, failed
	}
	if def == "" {
		def = sys.fightScreen.def
	}
	LogMessage("OMugen: the lifebars %s could not be loaded: %v", def, failed)
	LogMessage("OMugen: using the OMugen lifebars instead")
	fs, err := loadFightScreen(p)
	if err != nil {
		return nil, fmt.Errorf("%v (the OMugen lifebars failed too: %v)", failed, err)
	}
	return fs, nil
}

func loadFightScreenSafe(def string) (fs *FightScreen, err error) {
	defer func() {
		if r := recover(); r != nil {
			fs, err = nil, fmt.Errorf("%v", r)
		}
	}()
	return loadFightScreen(def)
}

func omugenScreenpackScriptInit(l *lua.LState) {
	luaRegister(l, "omugenScreenpackNotice", func(l *lua.LState) int {
		l.Push(lua.LString(omugenUI.notice))
		omugenUI.notice = ""
		return 1
	})
	luaRegister(l, "omugenScreenpackActive", func(l *lua.LState) int {
		l.Push(lua.LBool(isOMugenMotif(sys.motif.Def)))
		return 1
	})
}
