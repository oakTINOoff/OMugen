package main

// OMugen: reading a MUGEN 1.1 data/mugen.cfg so OMugen.exe can be dropped
// into an unmodified MUGEN folder. Settings found there override Ikemen's
// save/config.ini for this run only; they are never written back to it.

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
	LogMessage("OMugen: using %s (motif %s, game %dx%d, window %dx%d)", mugenCfgPath,
		c.Config.Motif, c.Video.GameWidth, c.Video.GameHeight, c.Video.WindowWidth, c.Video.WindowHeight)
}
