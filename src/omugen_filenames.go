package main

// OMugen: file names in MUGEN content are raw bytes in the author's Windows
// code page. They are decoded as Shift-JIS when they aren't UTF-8, which
// turns names like "Básicos.cns" (Windows-1252) into garbage. When such a
// file isn't found, legacyFileNameVariants gives the other plausible
// spellings to try, like MUGEN reading them with the system code page.

import (
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/japanese"
)

// legacyCodePages are tried in order: Western European first (the most
// common non-Japanese MUGEN content), then the system code page.
func legacyCodePages() []encoding.Encoding {
	cps := []encoding.Encoding{charmap.Windows1252}
	if cp := systemCodePage(); cp != nil && cp != charmap.Windows1252 {
		cps = append(cps, cp)
	}
	return cps
}

func legacyFileNameVariants(name string) []string {
	ascii := true
	for i := 0; i < len(name); i++ {
		if name[i] >= 0x80 {
			ascii = false
			break
		}
	}
	if ascii {
		return nil
	}
	// The original bytes: undo the Shift-JIS decoding, or take the name as
	// is when it wasn't decoded (invalid UTF-8).
	raws := []string{}
	if !utf8.ValidString(name) {
		raws = append(raws, name)
	} else if b, err := japanese.ShiftJIS.NewEncoder().String(name); err == nil && b != name {
		raws = append(raws, b)
	}
	var out []string
	seen := map[string]bool{name: true}
	for _, raw := range raws {
		for _, cp := range legacyCodePages() {
			if s, err := cp.NewDecoder().String(raw); err == nil && !seen[s] {
				seen[s] = true
				out = append(out, s)
			}
		}
	}
	return out
}
