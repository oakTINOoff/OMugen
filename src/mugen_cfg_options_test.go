package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testMugenCfgOptions = "[Options]\r\nDifficulty = 4\r\nLife = 30\r\nTime = -1\r\nGameSpeed = 0\r\n" +
	"Team.1VS2Life = 150\r\nTeam.LoseOnKO = 1\r\nmotif = data/system.def\r\n\r\n" +
	"[Rules]\r\nDefault.Attack.LifeToPowerMul = .5\r\nDefault.GetHit.LifeToPowerMul = .6\r\nSuper.TargetDefenceMul = 1.5\r\n\r\n" +
	"[Config]\r\nGameWidth = 852\r\nGameHeight = 480\r\n\r\n" +
	"[Video]\r\nKeepAspect = 0\r\n ;Stage fit mode.\r\nStageFit = 1\r\n\r\n" +
	"[Sound]\r\nMasterVolume = 150\r\nWavVolume = 70\r\nBGMVolume = 50 ;music\r\n\r\n" +
	"[Arcade]\r\nAI.RandomColor = 1\r\narcade.AIramp.start = 2, 0\r\narcade.AIramp.end   = 4, 2\r\n\r\n" +
	"[Debug]\r\nAllowDebugKeys = 0\r\n"

func TestImportMugenCfgOptions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mugen.cfg")
	os.WriteFile(path, []byte(testMugenCfgOptions), 0o644)
	mc, err := loadMugenCfg(path)
	if err != nil {
		t.Fatal(err)
	}
	var c Config
	c.Video.FightAspectWidth, c.Video.FightAspectHeight = 0, 0
	c.Video.KeepAspect = true
	c.Debug.AllowDebugKeys = true
	c.importMugenCfgOptions(mc)

	o := c.Options
	if o.Difficulty != 4 || o.Life != 30 || o.Time != -1 || o.GameSpeed != 0 ||
		o.Team.SingleVsTeamLife != 150 || !o.Simul.LoseOnKO {
		t.Errorf("options = difficulty %d life %v time %d speed %d 1vs2 %v loseonko %v",
			o.Difficulty, o.Life, o.Time, o.GameSpeed, o.Team.SingleVsTeamLife, o.Simul.LoseOnKO)
	}
	if c.Video.FightAspectWidth != -1 || c.Video.FightAspectHeight != -1 || c.Video.KeepAspect {
		t.Errorf("video = aspect %d:%d keepaspect %v, want stage aspect, no keepaspect",
			c.Video.FightAspectWidth, c.Video.FightAspectHeight, c.Video.KeepAspect)
	}
	// MUGEN's volumes are percents: 150 is the most there is.
	if c.Sound.MasterVolume != 100 || c.Sound.WavVolume != 70 || c.Sound.BGMVolume != 50 {
		t.Errorf("volumes = %d %d %d", c.Sound.MasterVolume, c.Sound.WavVolume, c.Sound.BGMVolume)
	}
	if !c.Arcade.AI.RandomColor || c.Arcade.Arcade.AIramp.Start != [2]int32{2, 0} || c.Arcade.Arcade.AIramp.End != [2]int32{4, 2} {
		t.Errorf("arcade = %v %v %v", c.Arcade.AI.RandomColor, c.Arcade.Arcade.AIramp.Start, c.Arcade.Arcade.AIramp.End)
	}
	if c.Debug.AllowDebugKeys {
		t.Errorf("AllowDebugKeys = 0 not taken")
	}

	loadMugenRules(mc)
	defer func() { mugenRules = nil }()
	if mugenRules["default.attack.lifetopowermul"] != 0.5 || mugenRules["super.targetdefencemul"] != 1.5 {
		t.Errorf("rules = %v", mugenRules)
	}
}

func TestSyncMugenCfgOptions(t *testing.T) {
	dir := t.TempDir()
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	os.Chdir(dir)
	defer func() { mugenCfgOptionsSnap = nil }()

	os.MkdirAll("data", 0o755)
	path := filepath.Join("data", "mugen.cfg")
	os.WriteFile(path, []byte(testMugenCfgOptions), 0o644)
	mc, _ := loadMugenCfg(path)
	var c Config
	c.importMugenCfgOptions(mc)
	snapshotMugenCfgOptions(&c)

	// Nothing changed: mugen.cfg is left alone.
	syncMugenCfgOptions(&c, path)
	if b, _ := os.ReadFile(path); string(b) != testMugenCfgOptions {
		t.Fatalf("unchanged settings rewrote mugen.cfg:\n%s", b)
	}

	// Changed in OMugen's options: life, music volume, stage fit, a new key.
	c.Options.Life = 100
	c.Sound.BGMVolume = 80
	c.Video.FightAspectWidth, c.Video.FightAspectHeight = 0, 0
	c.Debug.AllowDebugMode = true
	syncMugenCfgOptions(&c, path)
	b, _ := os.ReadFile(path)
	got := string(b)
	for _, want := range []string{"Life = 100\r\n", "BGMVolume = 80 ;music\r\n", " ;Stage fit mode.\r\nStageFit = 0\r\n",
		"AllowDebugMode = 1", "Difficulty = 4\r\n", "MasterVolume = 150\r\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("mugen.cfg lacks %q:\n%s", want, got)
		}
	}
	// A custom aspect ratio has no StageFit form: StageFit stays.
	c.Video.FightAspectWidth, c.Video.FightAspectHeight = 4, 3
	syncMugenCfgOptions(&c, path)
	if b2, _ := os.ReadFile(path); string(b2) != got {
		t.Errorf("custom aspect rewrote mugen.cfg:\n%s", b2)
	}
}

// The first start in a game folder takes the game's settings from
// mugen.cfg; a later start keeps the player's own (save/config.ini).
func TestLoadConfigFirstStartTakesMugenCfg(t *testing.T) {
	dir := t.TempDir()
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	os.Chdir(dir)
	kc, jc, gw, gh, msaa := sys.keyConfig, sys.joystickConfig, sys.gameWidth, sys.gameHeight, sys.msaa
	inp, vid := mugenInputActive, mugenCfgVideo
	defer func() {
		sys.keyConfig, sys.joystickConfig, sys.gameWidth, sys.gameHeight, sys.msaa = kc, jc, gw, gh, msaa
		mugenInputActive, mugenCfgVideo = inp, vid
		mugenCfgOptionsSnap, mugenRules = nil, nil
	}()

	os.MkdirAll("data", 0o755)
	os.MkdirAll("save", 0o755)
	os.WriteFile(filepath.Join("data", "mugen.cfg"), []byte(testMugenCfgOptions), 0o644)
	def := filepath.Join("save", "config.ini")

	c, err := loadConfig(def)
	if err != nil {
		t.Fatal(err)
	}
	if c.Options.Life != 30 || c.Options.Time != -1 || c.Options.Difficulty != 4 || c.Sound.BGMVolume != 50 {
		t.Fatalf("first start: life %v time %d difficulty %d bgm %d, want mugen.cfg's 30 -1 4 50",
			c.Options.Life, c.Options.Time, c.Options.Difficulty, c.Sound.BGMVolume)
	}
	if b, _ := os.ReadFile(def); !strings.Contains(string(b), "Life") {
		t.Fatalf("save/config.ini not written")
	}

	// The player changes the music volume in OMugen (config.ini only, as if
	// mugen.cfg had been edited elsewhere): the next start keeps it.
	if err := c.SetValueUpdate("Sound.BGMVolume", 90); err != nil {
		t.Fatal(err)
	}
	if err := c.Save(def); err != nil {
		t.Fatal(err)
	}
	sys.keyConfig, sys.joystickConfig = kc, jc
	c2, err := loadConfig(def)
	if err != nil {
		t.Fatal(err)
	}
	if c2.Sound.BGMVolume != 90 || c2.Options.Life != 30 {
		t.Errorf("second start: bgm %d life %v, want 90 30", c2.Sound.BGMVolume, c2.Options.Life)
	}
}
