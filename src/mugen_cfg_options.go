package main

// OMugen: the game settings MUGEN keeps in data/mugen.cfg ([Options] life,
// time, difficulty and speed, [Video] StageFit and KeepAspect, [Sound]
// volumes, [Arcade] AI, [Debug] keys). The first start in a game folder
// takes them from mugen.cfg, so the game plays as it does in MUGEN; from
// then on OMugen's options (save/config.ini) rule, and changes made there
// are written back to mugen.cfg (syncMugenCfgOptions). The [Rules]
// multipliers have no option in OMugen: they come from mugen.cfg at every
// start.

import (
	"fmt"
	"strconv"
	"strings"
)

// mugenCfgSetting links a mugen.cfg value to OMugen's config.
type mugenCfgSetting struct {
	section, key string
	// get returns the value as mugen.cfg writes it; false when OMugen's
	// value has no mugen.cfg form (a custom aspect ratio).
	get func(c *Config) (string, bool)
	// conv turns a mugen.cfg value into OMugen config values (query path ->
	// config.ini text); false when it isn't understood.
	conv func(v string) (map[string]string, bool)
}

func mugenIntSetting(section, key, query string, field func(*Config) *int, lo, hi int) mugenCfgSetting {
	return mugenCfgSetting{section, key,
		func(c *Config) (string, bool) { return strconv.Itoa(*field(c)), true },
		func(v string) (map[string]string, bool) {
			n, err := strconv.Atoi(v)
			if err != nil {
				return nil, false
			}
			return map[string]string{query: strconv.Itoa(Clamp(n, lo, hi))}, true
		}}
}

func mugenFloatSetting(section, key, query string, field func(*Config) *float32, lo, hi float32) mugenCfgSetting {
	return mugenCfgSetting{section, key,
		func(c *Config) (string, bool) { return formatMugenFloat(*field(c)), true },
		func(v string) (map[string]string, bool) {
			f, err := strconv.ParseFloat(v, 32)
			if err != nil {
				return nil, false
			}
			return map[string]string{query: formatMugenFloat(Clamp(float32(f), lo, hi))}, true
		}}
}

func formatMugenFloat(f float32) string {
	return strconv.FormatFloat(float64(f), 'g', -1, 32)
}

func mugenBoolSetting(section, key, query string, field func(*Config) *bool) mugenCfgSetting {
	return mugenCfgSetting{section, key,
		func(c *Config) (string, bool) {
			if *field(c) {
				return "1", true
			}
			return "0", true
		},
		func(v string) (map[string]string, bool) {
			n, err := strconv.Atoi(v)
			if err != nil {
				return nil, false
			}
			if n != 0 {
				return map[string]string{query: "1"}, true
			}
			return map[string]string{query: "0"}, true
		}}
}

// mugenRampSetting: "match, difficulty" pairs of the arcade AI ramps (the
// same keys in MUGEN's [Arcade] and OMugen's).
func mugenRampSetting(key string, field func(*Config) *[2]int32) mugenCfgSetting {
	return mugenCfgSetting{"Arcade", key,
		func(c *Config) (string, bool) { p := field(c); return fmt.Sprintf("%d, %d", p[0], p[1]), true },
		func(v string) (map[string]string, bool) {
			a, b, ok := strings.Cut(v, ",")
			if !ok {
				return nil, false
			}
			x, err1 := strconv.Atoi(strings.TrimSpace(a))
			y, err2 := strconv.Atoi(strings.TrimSpace(b))
			if err1 != nil || err2 != nil {
				return nil, false
			}
			return map[string]string{"Arcade." + key: fmt.Sprintf("%d, %d", x, y)}, true
		}}
}

var mugenCfgSettings = []mugenCfgSetting{
	mugenIntSetting("Options", "Difficulty", "Options.Difficulty", func(c *Config) *int { return &c.Options.Difficulty }, 1, 8),
	mugenFloatSetting("Options", "Life", "Options.Life", func(c *Config) *float32 { return &c.Options.Life }, 1, 1000),
	{"Options", "Time",
		func(c *Config) (string, bool) { return strconv.Itoa(int(c.Options.Time)), true },
		func(v string) (map[string]string, bool) {
			n, err := strconv.Atoi(v)
			if err != nil || n == 0 {
				return nil, false // 0 would end every round at once
			}
			if n < 0 {
				n = -1 // no time limit
			}
			return map[string]string{"Options.Time": strconv.Itoa(Clamp(n, -1, 1000))}, true
		}},
	mugenIntSetting("Options", "GameSpeed", "Options.GameSpeed", func(c *Config) *int { return &c.Options.GameSpeed }, -9, 9),
	mugenFloatSetting("Options", "Team.1VS2Life", "Options.Team.SingleVsTeamLife",
		func(c *Config) *float32 { return &c.Options.Team.SingleVsTeamLife }, 1, 1000),
	// MUGEN's only team mode where KOs count this way is Simul.
	mugenBoolSetting("Options", "Team.LoseOnKO", "Options.Simul.LoseOnKO", func(c *Config) *bool { return &c.Options.Simul.LoseOnKO }),

	// StageFit 0 draws stages to the width of the screen (a 4:3 stage fills a
	// widescreen game, cut at the top and bottom); 1 shrinks them to fit.
	{"Video", "StageFit",
		func(c *Config) (string, bool) {
			switch {
			case c.Video.FightAspectWidth == 0 && c.Video.FightAspectHeight == 0:
				return "0", true
			case c.Video.FightAspectWidth < 0 && c.Video.FightAspectHeight < 0:
				return "1", true
			}
			return "", false
		},
		func(v string) (map[string]string, bool) {
			n, err := strconv.Atoi(v)
			if err != nil {
				return nil, false
			}
			aspect := "0"
			if n != 0 {
				aspect = "-1"
			}
			return map[string]string{"Video.FightAspectWidth": aspect, "Video.FightAspectHeight": aspect}, true
		}},
	mugenBoolSetting("Video", "KeepAspect", "Video.KeepAspect", func(c *Config) *bool { return &c.Video.KeepAspect }),

	mugenIntSetting("Sound", "MasterVolume", "Sound.MasterVolume", func(c *Config) *int { return &c.Sound.MasterVolume }, 0, 100),
	mugenIntSetting("Sound", "WavVolume", "Sound.WavVolume", func(c *Config) *int { return &c.Sound.WavVolume }, 0, 100),
	mugenIntSetting("Sound", "BGMVolume", "Sound.BGMVolume", func(c *Config) *int { return &c.Sound.BGMVolume }, 0, 100),

	mugenBoolSetting("Arcade", "AI.RandomColor", "Arcade.AI.RandomColor", func(c *Config) *bool { return &c.Arcade.AI.RandomColor }),
	mugenRampSetting("arcade.AIramp.start", func(c *Config) *[2]int32 { return &c.Arcade.Arcade.AIramp.Start }),
	mugenRampSetting("arcade.AIramp.end", func(c *Config) *[2]int32 { return &c.Arcade.Arcade.AIramp.End }),
	mugenRampSetting("team.AIramp.start", func(c *Config) *[2]int32 { return &c.Arcade.Team.AIramp.Start }),
	mugenRampSetting("team.AIramp.end", func(c *Config) *[2]int32 { return &c.Arcade.Team.AIramp.End }),
	mugenRampSetting("survival.AIramp.start", func(c *Config) *[2]int32 { return &c.Arcade.Survival.AIramp.Start }),
	mugenRampSetting("survival.AIramp.end", func(c *Config) *[2]int32 { return &c.Arcade.Survival.AIramp.End }),

	mugenBoolSetting("Debug", "AllowDebugMode", "Debug.AllowDebugMode", func(c *Config) *bool { return &c.Debug.AllowDebugMode }),
	mugenBoolSetting("Debug", "AllowDebugKeys", "Debug.AllowDebugKeys", func(c *Config) *bool { return &c.Debug.AllowDebugKeys }),
}

// mugenCfgOptionsSnap holds each setting's value at startup, to find the
// ones changed later in OMugen's options.
var mugenCfgOptionsSnap map[string]string

func mugenCfgSettingID(section, key string) string {
	return strings.ToLower(section) + "\x00" + strings.ToLower(key)
}

// importMugenCfgOptions takes the game's settings from mugen.cfg (on the
// first start in a game folder), into the config and its save/config.ini.
func (c *Config) importMugenCfgOptions(mc mugenCfg) {
	if mc == nil {
		return
	}
	var took []string
	for _, s := range mugenCfgSettings {
		v, ok := mc.str(strings.ToLower(s.section), strings.ToLower(s.key))
		if !ok {
			continue
		}
		vals, ok := s.conv(v)
		if !ok {
			continue
		}
		for _, q := range SortedKeys(vals) {
			var err error
			if c.IniFile != nil {
				err = c.SetValueUpdate(q, vals[q])
			} else {
				err = SetValue(c, q, vals[q])
			}
			if err != nil {
				LogMessage("OMugen: %s %s = %s: %v", mugenCfgPath, s.key, v, err)
			}
		}
		took = append(took, s.key+" = "+v)
	}
	if len(took) > 0 {
		LogMessage("OMugen: first start: game settings from %s: %s", mugenCfgPath, strings.Join(took, ", "))
	}
}

// snapshotMugenCfgOptions remembers the settings as they are now.
func snapshotMugenCfgOptions(c *Config) {
	mugenCfgOptionsSnap = map[string]string{}
	for _, s := range mugenCfgSettings {
		if v, ok := s.get(c); ok {
			mugenCfgOptionsSnap[mugenCfgSettingID(s.section, s.key)] = v
		}
	}
}

// syncMugenCfgOptions writes the settings changed in OMugen's options to
// mugen.cfg, so MUGEN's settings file stays the game's settings.
func syncMugenCfgOptions(c *Config, path string) {
	if mugenCfgOptionsSnap == nil || FileExist(path) == "" {
		return
	}
	var edits []mugenCfgEdit
	for _, s := range mugenCfgSettings {
		v, ok := s.get(c)
		if !ok || v == mugenCfgOptionsSnap[mugenCfgSettingID(s.section, s.key)] || mugenSessionOverride(s.section, s.key) {
			continue
		}
		edits = append(edits, mugenCfgEdit{s.section, s.key, v})
	}
	if len(edits) == 0 {
		return
	}
	if err := writeMugenCfgEdits(path, edits); err != nil {
		LogMessage("OMugen: could not save settings to %s: %v", path, err)
		return
	}
	names := make([]string, len(edits))
	for i, e := range edits {
		names[i] = e.key + " = " + e.value
		mugenCfgOptionsSnap[mugenCfgSettingID(e.section, e.key)] = e.value
	}
	LogMessage("OMugen: saved settings to %s: %s", path, strings.Join(names, ", "))
}

// mugenSessionOverride reports whether a mugen.cfg setting was set for this
// session only by a command-line flag (-nosound, -setvolume, -ailevel,
// -windowed): such values are never written back to mugen.cfg.
func mugenSessionOverride(section, key string) bool {
	flag := func(f string) bool { _, ok := sys.cmdFlags[f]; return ok }
	switch strings.ToLower(section + "." + key) {
	case "sound.mastervolume":
		return flag("-nosound") || flag("-setvolume")
	case "options.difficulty":
		return flag("-ailevel")
	case "video.fullscreen":
		return flag("-windowed")
	}
	return false
}

// mugenRules are mugen.cfg's [Rules] multipliers, applied to every
// character after the engine's common constants (a character's own
// [Constants] still win).
var mugenRules map[string]float32

var mugenRuleKeys = []string{"default.attack.lifetopowermul", "default.gethit.lifetopowermul", "super.targetdefencemul"}

func loadMugenRules(mc mugenCfg) {
	mugenRules = nil
	for _, k := range mugenRuleKeys {
		if v, ok := mc.str("rules", k); ok {
			if f, err := strconv.ParseFloat(v, 32); err == nil {
				if mugenRules == nil {
					mugenRules = map[string]float32{}
				}
				mugenRules[k] = float32(f)
			}
		}
	}
}
