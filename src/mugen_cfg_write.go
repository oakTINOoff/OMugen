package main

// OMugen: writing MUGEN 1.1 data/mugen.cfg values (used by the controls menu).
// Edits are minimal: only the values of the given keys change; comments,
// ordering, spacing, line endings and every other line are preserved.

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// mugenCfgEdit sets key = value inside [section] (both case-insensitive).
type mugenCfgEdit struct {
	section, key, value string
}

// editMugenCfgText applies edits to mugen.cfg text. Existing keys keep their
// original spelling, spacing and trailing comment; missing keys are added at
// the end of their section; missing sections are appended.
func editMugenCfgText(text string, edits []mugenCfgEdit) string {
	nl := "\n"
	if strings.Contains(text, "\r\n") {
		nl = "\r\n"
	}
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")

	pending := map[string]map[string]string{} // section -> key -> value
	order := map[string][]string{}            // keep caller order for inserts
	var sectionOrder []string
	for _, e := range edits {
		s, k := strings.ToLower(e.section), strings.ToLower(e.key)
		if pending[s] == nil {
			pending[s] = map[string]string{}
			sectionOrder = append(sectionOrder, s)
		}
		if _, dup := pending[s][k]; !dup {
			order[s] = append(order[s], k)
		}
		pending[s][k] = e.value
	}
	display := map[string]string{} // lowercase -> original spelling for new lines
	for _, e := range edits {
		display[strings.ToLower(e.section)] = e.section
		display[strings.ToLower(e.section)+"\x00"+strings.ToLower(e.key)] = e.key
	}

	var out []string
	section := ""
	lastContent := map[string]int{} // section -> index in out of its last key/header line
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "[") {
			if end := strings.IndexByte(trim, ']'); end > 0 {
				section = strings.ToLower(strings.TrimSpace(trim[1:end]))
				out = append(out, line)
				lastContent[section] = len(out) - 1
				continue
			}
		}
		if vals := pending[section]; vals != nil && trim != "" && trim[0] != ';' {
			body, comment := line, ""
			if i := strings.IndexByte(line, ';'); i >= 0 {
				body, comment = line[:i], line[i:]
			}
			if eq := strings.IndexByte(body, '='); eq >= 0 {
				k := strings.ToLower(strings.TrimSpace(body[:eq]))
				if v, ok := vals[k]; ok {
					// Keep the line's own "Key   = " spacing; keep a space before a trailing comment.
					head := body[:eq+1]
					after := body[eq+1:]
					lead := after[:len(after)-len(strings.TrimLeft(after, " \t"))]
					newLine := head + lead + v
					if comment != "" {
						newLine += " " + comment
					}
					out = append(out, newLine)
					delete(vals, k)
					lastContent[section] = len(out) - 1
					continue
				}
			}
		}
		out = append(out, line)
		if trim != "" && trim[0] != ';' {
			lastContent[section] = len(out) - 1
		}
	}

	// Insert keys that were not present, after the section's last key.
	for _, s := range sectionOrder {
		vals := pending[s]
		if len(vals) == 0 {
			continue
		}
		var add []string
		for _, k := range order[s] {
			if v, ok := vals[k]; ok {
				add = append(add, display[s+"\x00"+k]+" = "+v)
			}
		}
		if idx, ok := lastContent[s]; ok && s != "" {
			rest := append([]string{}, out[idx+1:]...)
			out = append(append(out[:idx+1], add...), rest...)
			// Shift recorded positions after the insertion point.
			for other, oi := range lastContent {
				if oi > idx {
					lastContent[other] = oi + len(add)
				}
			}
			lastContent[s] = idx + len(add)
		} else {
			if len(out) > 0 && strings.TrimSpace(out[len(out)-1]) != "" {
				out = append(out, "")
			}
			out = append(out, "["+display[s]+"]")
			out = append(out, add...)
		}
	}
	return strings.Join(out, nl)
}

// writeMugenCfgEdits applies edits to a mugen.cfg file. The previous version
// is first copied to save/backups/ (OMugen's own folder in the game).
func writeMugenCfgEdits(path string, edits []mugenCfgEdit) error {
	orig, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	updated := []byte(editMugenCfgText(string(orig), edits))
	if bytes.Equal(orig, updated) {
		return nil
	}
	backupDir := filepath.Join("save", "backups")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return err
	}
	backup := filepath.Join(backupDir, fmt.Sprintf("mugen.cfg.%s", time.Now().Format("2006-01-02_15-04-05")))
	if err := os.WriteFile(backup, orig, 0o644); err != nil {
		return err
	}
	// Write to a temp file and rename over the original, so a crash never
	// leaves a half-written mugen.cfg.
	tmp := path + ".omugen-tmp"
	if err := os.WriteFile(tmp, updated, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
