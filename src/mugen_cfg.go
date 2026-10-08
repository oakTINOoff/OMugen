package main

// OMugen: reading a MUGEN 1.1 data/mugen.cfg so OMugen.exe can be dropped
// into an unmodified MUGEN folder. Settings found there override Ikemen's
// save/config.ini; video settings changed in OMugen's options are written
// back to mugen.cfg (syncMugenCfgVideo), like MUGEN keeps its settings there.

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

const mugenCfgPath = "data/mugen.cfg"

// mugenCfg holds mugen.cfg values by lowercase section and key.
type mugenCfg map[string]map[string]string

// loadMugenCfg parses mugen.cfg: "[Section]" headers, "key = value" lines,
// ';' comments. Section and key names are case-insensitive; the first
// occurrence of a key wins.
func loadMugenCfg(path string) (mugenCfg, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	cfg := mugenCfg{}
	section := ""
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if i := strings.IndexByte(line, ';'); i >= 0 {
			line = line[:i]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if line[0] == '[' {
			if end := strings.IndexByte(line, ']'); end > 0 {
				section = strings.ToLower(strings.TrimSpace(line[1:end]))
			}
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		if cfg[section] == nil {
			cfg[section] = map[string]string{}
		}
		if _, dup := cfg[section][key]; !dup {
			cfg[section][key] = strings.Trim(strings.TrimSpace(val), `"`)
		}
	}
	return cfg, sc.Err()
}

func (c mugenCfg) str(section, key string) (string, bool) {
	v, ok := c[section][key]
	return v, ok && v != ""
}

func (c mugenCfg) int(section, key string) (int, bool) {
	s, ok := c.str(section, key)
	if !ok {
		return 0, false
	}
	n, err := strconv.Atoi(s)
	return n, err == nil
}

// loadGameMugenCfg reads the game folder's mugen.cfg, or returns nil if there is none.
func loadGameMugenCfg() mugenCfg {
	if FileExist(mugenCfgPath) == "" {
		return nil
	}
	mc, err := loadMugenCfg(mugenCfgPath)
	if err != nil {
		LogMessage("OMugen: could not read %s: %v", mugenCfgPath, err)
		return nil
	}
	return mc
}

// applyMugenCfg copies the mugen.cfg settings OMugen understands into the
// in-memory Ikemen config (not into save/config.ini).
func (c *Config) applyMugenCfg(mc mugenCfg) {
	if mc == nil {
		return
	}
	if v, ok := mc.str("options", "motif"); ok {
		c.Config.Motif = v
	}
	if v, ok := mc.int("config", "gamewidth"); ok && v > 0 {
		c.Video.GameWidth = int32(v)
	}
	if v, ok := mc.int("config", "gameheight"); ok && v > 0 {
		c.Video.GameHeight = int32(v)
	}
	if v, ok := mc.int("video", "width"); ok && v > 0 {
		c.Video.WindowWidth = v
	}
	if v, ok := mc.int("video", "height"); ok && v > 0 {
		c.Video.WindowHeight = v
	}
	if v, ok := mc.int("video", "fullscreen"); ok {
		c.Video.Fullscreen = v != 0
	}
	mugenCfgVideo.loaded = true
	mugenCfgVideo.gameW, mugenCfgVideo.gameH = c.Video.GameWidth, c.Video.GameHeight
	mugenCfgVideo.winW, mugenCfgVideo.winH = c.Video.WindowWidth, c.Video.WindowHeight
	mugenCfgVideo.fullscreen = c.Video.Fullscreen
	LogMessage("OMugen: using %s (motif %s, game %dx%d, window %dx%d)", mugenCfgPath,
		c.Config.Motif, c.Video.GameWidth, c.Video.GameHeight, c.Video.WindowWidth, c.Video.WindowHeight)
}

// mugenCfgVideo remembers the video settings taken from mugen.cfg at
// startup, to find the ones changed later in OMugen's options.
var mugenCfgVideo struct {
	loaded       bool
	gameW, gameH int32
	winW, winH   int
	fullscreen   bool
}

// syncMugenCfgVideo writes video settings changed in OMugen's options back
// to mugen.cfg, since mugen.cfg's values override save/config.ini at every
// start (as MUGEN's own settings file). A new resolution also becomes the
// window size, which mugen.cfg sets separately.
func syncMugenCfgVideo(c *Config, path string) {
	v := &mugenCfgVideo
	if !v.loaded {
		return
	}
	var edits []mugenCfgEdit
	if c.Video.GameWidth != v.gameW || c.Video.GameHeight != v.gameH {
		edits = append(edits,
			mugenCfgEdit{"Config", "GameWidth", strconv.Itoa(int(c.Video.GameWidth))},
			mugenCfgEdit{"Config", "GameHeight", strconv.Itoa(int(c.Video.GameHeight))})
		c.Video.WindowWidth, c.Video.WindowHeight = int(c.Video.GameWidth), int(c.Video.GameHeight)
	}
	if c.Video.WindowWidth != v.winW || c.Video.WindowHeight != v.winH {
		edits = append(edits,
			mugenCfgEdit{"Video", "Width", strconv.Itoa(c.Video.WindowWidth)},
			mugenCfgEdit{"Video", "Height", strconv.Itoa(c.Video.WindowHeight)})
	}
	if c.Video.Fullscreen != v.fullscreen && !mugenSessionOverride("Video", "FullScreen") {
		fs := "0"
		if c.Video.Fullscreen {
			fs = "1"
		}
		edits = append(edits, mugenCfgEdit{"Video", "FullScreen", fs})
	}
	if len(edits) == 0 {
		return
	}
	if err := writeMugenCfgEdits(path, edits); err != nil {
		LogMessage("OMugen: could not save video settings to %s: %v", path, err)
		return
	}
	LogMessage("OMugen: saved video settings to %s (game %dx%d, window %dx%d, fullscreen %v)", path,
		c.Video.GameWidth, c.Video.GameHeight, c.Video.WindowWidth, c.Video.WindowHeight, c.Video.Fullscreen)
	v.gameW, v.gameH = c.Video.GameWidth, c.Video.GameHeight
	v.winW, v.winH = c.Video.WindowWidth, c.Video.WindowHeight
	v.fullscreen = c.Video.Fullscreen
}
