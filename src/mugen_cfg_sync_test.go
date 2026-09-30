package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyncMugenCfgVideo(t *testing.T) {
	dir := t.TempDir()
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	os.Chdir(dir)
	saved := mugenCfgVideo
	defer func() { mugenCfgVideo = saved }()

	os.MkdirAll("data", 0o755)
	cfg := filepath.Join("data", "mugen.cfg")
	orig := "[Config]\r\nGameWidth = 1920 ;game\r\nGameHeight = 1030\r\n\r\n[Video]\r\nWidth  = 1920\r\nHeight = 1030\r\nFullScreen = 0\r\n"
	os.WriteFile(cfg, []byte(orig), 0o644)
	mc, err := loadMugenCfg(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var c Config
	c.applyMugenCfg(mc)

	// Nothing changed: mugen.cfg is left alone (no backup either).
	syncMugenCfgVideo(&c, cfg)
	if b, _ := os.ReadFile(cfg); string(b) != orig {
		t.Fatalf("unchanged settings rewrote mugen.cfg:\n%s", b)
	}
	if _, err := os.Stat(filepath.Join("save", "backups")); err == nil {
		t.Errorf("backup made without changes")
	}

	// 1080p + fullscreen chosen in the options.
	c.Video.GameHeight = 1080
	c.Video.Fullscreen = true
	syncMugenCfgVideo(&c, cfg)
	b, _ := os.ReadFile(cfg)
	want := "[Config]\r\nGameWidth = 1920 ;game\r\nGameHeight = 1080\r\n\r\n[Video]\r\nWidth  = 1920\r\nHeight = 1080\r\nFullScreen = 1\r\n"
	if string(b) != want {
		t.Errorf("mugen.cfg =\n%q\nwant\n%q", b, want)
	}
	if c.Video.WindowWidth != 1920 || c.Video.WindowHeight != 1080 {
		t.Errorf("window = %dx%d, want 1920x1080", c.Video.WindowWidth, c.Video.WindowHeight)
	}
	backups, _ := os.ReadDir(filepath.Join("save", "backups"))
	if len(backups) != 1 || !strings.HasPrefix(backups[0].Name(), "mugen.cfg.") {
		t.Errorf("backups = %v", backups)
	}
	// It reads back as the new settings.
	mc, _ = loadMugenCfg(cfg)
	var c2 Config
	c2.applyMugenCfg(mc)
	if c2.Video.GameHeight != 1080 || c2.Video.WindowHeight != 1080 || !c2.Video.Fullscreen {
		t.Errorf("reloaded: game %dx%d window %dx%d fullscreen %v", c2.Video.GameWidth, c2.Video.GameHeight,
			c2.Video.WindowWidth, c2.Video.WindowHeight, c2.Video.Fullscreen)
	}
}
