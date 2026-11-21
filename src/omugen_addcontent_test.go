package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAddEditSelect(t *testing.T) {
	char := func(ref string) *addItem { return &addItem{kind: addChar, ref: ref, name: ref} }
	stage := func(ref string) *addItem { return &addItem{kind: addStage, ref: ref, name: ref} }
	cases := []struct {
		name     string
		in       string
		items    []*addItem
		capacity int
		want     string
		hidden   []bool
	}{
		{
			name:  "fills the empty boxes that end the list",
			in:    "[Characters]\nkfm\nblank\nblank\n\n[ExtraStages]\nstages/a.def\n",
			items: []*addItem{char("goku"), stage("stages/namek/namek.def")},
			want:  "[Characters]\nkfm\ngoku\nblank\n\n[ExtraStages]\nstages/a.def\nstages/namek/namek.def\n",
		},
		{
			name:     "a new box while the screen has room",
			in:       "; roster\n[Characters]\nkfm, stages/kfm.def\nsuave\n; the end\n\n[ExtraStages]\n",
			items:    []*addItem{char("goku")},
			capacity: 3,
			want:     "; roster\n[Characters]\nkfm, stages/kfm.def\nsuave\ngoku\n; the end\n\n[ExtraStages]\n",
		},
		{
			name:  "before the random boxes that end the list",
			in:    "[Characters]\nproto\nrandomselect\n",
			items: []*addItem{char("goku"), char("vegeta")},
			want:  "[Characters]\nproto\ngoku\nvegeta\nrandomselect\n",
		},
		{
			name:  "an empty box before a random one, then before the random one",
			in:    "[Characters]\nproto\nblank\nrandomselect\n",
			items: []*addItem{char("goku"), char("vegeta")},
			want:  "[Characters]\nproto\ngoku\nvegeta\nrandomselect\n",
		},
		{
			name:     "an empty box elsewhere when the screen is full",
			in:       "[Characters]\nkfm\nblank\nsuave\n",
			items:    []*addItem{char("goku")},
			capacity: 3,
			want:     "[Characters]\nkfm\ngoku\nsuave\n",
		},
		{
			name:     "listed anyway when no box is left",
			in:       "[Characters]\nkfm\nsuave\n",
			items:    []*addItem{char("goku")},
			capacity: 2,
			want:     "[Characters]\nkfm\nsuave\ngoku\n",
			hidden:   []bool{true},
		},
		{
			name:     "slot groups are one box, excluded characters none",
			in:       "[Characters]\nslot = {\na\nb\n}\nboss, exclude = 1\n",
			items:    []*addItem{char("goku")},
			capacity: 2,
			want:     "[Characters]\nslot = {\na\nb\n}\nboss, exclude = 1\ngoku\n",
		},
		{
			name:  "Windows line ends stay",
			in:    "[Characters]\r\nkfm\r\n\r\n[ExtraStages]\r\n",
			items: []*addItem{char("goku"), stage("stages/x/x.def")},
			want:  "[Characters]\r\nkfm\r\ngoku\r\n\r\n[ExtraStages]\r\nstages/x/x.def\r\n",
		},
		{
			name:  "sections that are missing are added",
			in:    "[Characters]\nkfm\n",
			items: []*addItem{stage("stages/x/x.def")},
			want:  "[Characters]\nkfm\n\n[ExtraStages]\nstages/x/x.def\n",
		},
	}
	for _, c := range cases {
		got := addEditSelect(c.in, c.items, c.capacity)
		if got != c.want {
			t.Errorf("%s:\ngot  %q\nwant %q", c.name, got, c.want)
		}
		for i, h := range c.hidden {
			if c.items[i].hidden != h {
				t.Errorf("%s: item %d hidden = %v, want %v", c.name, i, c.items[i].hidden, h)
			}
		}
	}
}

func TestAddEntryPath(t *testing.T) {
	dst := filepath.Join("x", "staging")
	for name, ok := range map[string]bool{
		"kfm/kfm.def":       true,
		`kfm\sprites\a.sff`: true,
		"../evil.txt":       false,
		"a/../../evil.txt":  false,
		"C:/Windows/x.dll":  false,
		"/etc/passwd":       true, // the leading slash is dropped: stays inside
		"kfm/con.txt":       true, // renamed _con.txt
		"kfm/what?.def":     true,
		"./":                false,
	} {
		got := addEntryPath(dst, name)
		if ok != (got != "") {
			t.Errorf("addEntryPath(%q) = %q", name, got)
			continue
		}
		if got != "" {
			if rel, err := filepath.Rel(dst, got); err != nil || strings.HasPrefix(rel, "..") {
				t.Errorf("addEntryPath(%q) = %q, outside %s", name, got, dst)
			}
		}
	}
	if got := filepath.Base(addEntryPath(dst, "kfm/con.txt")); got != "_con.txt" {
		t.Errorf("reserved name: %q", got)
	}
}

func TestAddNormRef(t *testing.T) {
	for in, want := range map[string]string{
		"kfm":               "kfm",
		"KFM/kfm.def":       "kfm",
		"chars/kfm/kfm.def": "kfm",
		"kfm/kfm720.def":    "kfm/kfm720.def",
		`Goku\goku.def`:     "goku",
	} {
		if got := addNormRef(in); got != want {
			t.Errorf("addNormRef(%q) = %q, want %q", in, got, want)
		}
	}
}

const testCharDef = "[Info]\nname = \"Goku\"\ndisplayname = \"Son Goku\"\n[Files]\ncmd = goku.cmd\ncns = goku.cns\nsprite = goku.sff\nanim = goku.air\n"
const testStageDef = "[Info]\nname = \"Namek\"\n[Camera]\nstartx = 0\n[BGdef]\nspr = namek.sff\n[Music]\nbgmusic = sound/namek.mp3\n"

// testGame makes a game folder with a select.def.
func testGame(t *testing.T, sel string) string {
	game := t.TempDir()
	os.MkdirAll(filepath.Join(game, "data"), 0o755)
	os.MkdirAll(filepath.Join(game, "chars"), 0o755)
	if err := os.WriteFile(filepath.Join(game, "data", "select.def"), []byte(sel), 0o644); err != nil {
		t.Fatal(err)
	}
	return game
}

func testZip(t *testing.T, dir, name string, files map[string]string) string {
	p := filepath.Join(dir, name)
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for n, body := range files {
		w, err := zw.Create(n)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(body))
	}
	zw.Close()
	f.Close()
	return p
}

func TestAddFromZip(t *testing.T) {
	game := testGame(t, "[Characters]\nkfm\nblank\n\n[ExtraStages]\n")
	dl := t.TempDir()
	z := testZip(t, dl, "Goku pack.zip", map[string]string{
		"Goku/goku.def":   testCharDef,
		"Goku/goku.sff":   "sprites",
		"Goku/goku.cmd":   "cmd",
		"Goku/setup.exe":  "MZ",
		"../escape.txt":   "out",
		"Namek/namek.def": testStageDef,
		"Namek/namek.sff": "stage sprites",
		"sound/namek.mp3": "music",
		"readme.txt":      "hi",
	})
	plan := addPrepare(game, filepath.Join(game, "data", "select.def"), 0, []string{z}, nil)
	if len(plan.items) != 2 {
		t.Fatalf("items = %d, notes %v", len(plan.items), plan.notes)
	}
	if plan.blocked != 1 {
		t.Errorf("programs left out = %d, want 1", plan.blocked)
	}
	if !plan.wanted() || !strings.Contains(plan.summary(), "Son Goku") {
		t.Errorf("summary: %s", plan.summary())
	}
	res := plan.apply(nil)
	if len(res.failed) > 0 || len(res.added) != 2 || !res.changed {
		t.Fatalf("result: %s", res.text())
	}
	for _, f := range []string{
		"chars/Goku/goku.def", "chars/Goku/goku.sff", "chars/Goku/goku.cmd", "stages/namek/namek.def",
		"stages/namek/namek.sff", "sound/namek.mp3",
	} {
		if _, err := os.Stat(filepath.Join(game, f)); err != nil {
			t.Errorf("missing %s", f)
		}
	}
	for _, f := range []string{"chars/Goku/setup.exe", "../escape.txt", "escape.txt"} {
		if _, err := os.Stat(filepath.Join(game, f)); err == nil {
			t.Errorf("%s should not be there", f)
		}
	}
	sel, _ := os.ReadFile(filepath.Join(game, "data", "select.def"))
	want := "[Characters]\nkfm\nGoku/goku.def\n\n[ExtraStages]\nstages/namek/namek.def\n"
	if string(sel) != want {
		t.Errorf("select.def:\n%q\nwant\n%q", sel, want)
	}
	if b, _ := filepath.Glob(filepath.Join(game, "save", "backups", "select.def.*")); len(b) != 1 {
		t.Errorf("select.def backups: %v", b)
	}
	if _, err := os.Stat(filepath.Join(game, "save", "omugen-add")); err == nil {
		t.Errorf("the unpacked files are still there")
	}
	// The same download again: replaced, the old copy kept.
	plan = addPrepare(game, filepath.Join(game, "data", "select.def"), 0, []string{z}, nil)
	for _, it := range plan.items {
		if !it.exists || !it.listed {
			t.Errorf("%s: exists %v listed %v", it.name, it.exists, it.listed)
		}
	}
	res = plan.apply(nil)
	if len(res.replaced) != 2 {
		t.Errorf("replaced: %s", res.text())
	}
	if b, _ := filepath.Glob(filepath.Join(game, "save", "backups", "chars", "Goku.*")); len(b) != 1 {
		t.Errorf("char backups: %v", b)
	}
	sel2, _ := os.ReadFile(filepath.Join(game, "data", "select.def"))
	if string(sel2) != want {
		t.Errorf("select.def changed again: %q", sel2)
	}
}

func TestAddCharAtArchiveTop(t *testing.T) {
	game := testGame(t, "[Characters]\nkfm\n")
	z := testZip(t, t.TempDir(), "Goku_SSJ_by_someone.zip", map[string]string{
		"goku.def": testCharDef,
		"goku.sff": "sprites",
	})
	plan := addPrepare(game, filepath.Join(game, "data", "select.def"), 0, []string{z}, nil)
	if len(plan.items) != 1 || plan.items[0].target != "chars/goku" || plan.items[0].ref != "goku" {
		t.Fatalf("items: %+v notes %v", plan.items, plan.notes)
	}
	plan.apply(nil)
	if _, err := os.Stat(filepath.Join(game, "chars", "goku", "goku.def")); err != nil {
		t.Error(err)
	}
}

func TestAddLeavesOut(t *testing.T) {
	game := testGame(t, "[Characters]\nkfm\n")
	dl := t.TempDir()
	wholeGame := testZip(t, dl, "game.zip", map[string]string{
		"MyGame/data/select.def":   "[Characters]\n",
		"MyGame/chars/kfm/kfm.def": testCharDef,
	})
	screenpack := testZip(t, dl, "screenpack.zip", map[string]string{
		"sp/system.def": "[Files]\nspr = system.sff\nselect = select.def\nfight = fight.def\n",
	})
	notArchive := filepath.Join(dl, "notes.txt")
	os.WriteFile(notArchive, []byte("x"), 0o644)
	plan := addPrepare(game, filepath.Join(game, "data", "select.def"), 0, []string{wholeGame, screenpack, notArchive}, nil)
	if len(plan.items) != 0 || len(plan.notes) != 3 || plan.wanted() {
		t.Fatalf("items %d, notes %v", len(plan.items), plan.notes)
	}
	for i, want := range []string{"whole MUGEN game", "screenpack", "not a zip"} {
		if !strings.Contains(plan.notes[i], want) {
			t.Errorf("note %d = %q, want %q", i, plan.notes[i], want)
		}
	}
	plan.cleanup()
}

func TestAddFolderAndStageInCharFolder(t *testing.T) {
	game := testGame(t, "[Characters]\nkfm\n\n[ExtraStages]\n")
	src := filepath.Join(t.TempDir(), "Goku")
	os.MkdirAll(filepath.Join(src, "stage"), 0o755)
	os.WriteFile(filepath.Join(src, "goku.def"), []byte(testCharDef), 0o644)
	os.WriteFile(filepath.Join(src, "stage", "arena.def"), []byte(testStageDef), 0o644)
	plan := addPrepare(game, filepath.Join(game, "data", "select.def"), 0, []string{src}, nil)
	if len(plan.items) != 2 {
		t.Fatalf("items %d notes %v", len(plan.items), plan.notes)
	}
	if st := plan.items[1]; st.withChar == nil || st.ref != "chars/Goku/stage/arena.def" {
		t.Errorf("stage: %+v", st)
	}
	plan.apply(nil)
	if _, err := os.Stat(src); err != nil {
		t.Errorf("a folder is copied, not moved: %v", err)
	}
	sel, _ := os.ReadFile(filepath.Join(game, "data", "select.def"))
	if want := "[Characters]\nkfm\nGoku/goku.def\n\n[ExtraStages]\nchars/Goku/stage/arena.def\n"; string(sel) != want {
		t.Errorf("select.def %q, want %q", sel, want)
	}
	// The game's own character, missing from select.def: only listed.
	os.MkdirAll(filepath.Join(game, "chars", "suave"), 0o755)
	os.WriteFile(filepath.Join(game, "chars", "suave", "suave.def"), []byte(testCharDef), 0o644)
	plan = addPrepare(game, filepath.Join(game, "data", "select.def"), 0, []string{filepath.Join(game, "chars", "suave")}, nil)
	if len(plan.items) != 1 || !plan.items[0].inPlace || !plan.wanted() {
		t.Fatalf("in place: %+v", plan.items)
	}
}
