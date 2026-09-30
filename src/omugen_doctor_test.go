package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDoctorIssueKeyGroupsSimilarWarnings(t *testing.T) {
	cases := map[string]string{
		"WARNING: Duplicate sprite key in chars/Goku OP/Goku.sff: 5070,10 (index 67 ignored)":                    "WARNING: Duplicate sprite key in <file>: N,N (index N ignored)",
		"WARNING: Son Goku's state 35381 in MG.cns, line 7580: Unknown state controller parameter(s): pausemove": "WARNING: <state>: Unknown state controller parameter(s): pausemove",
		"WARNING: Goku's state -1 in IA.cns, line 12: Unknown state controller parameter(s): supermove":          "WARNING: <state>: Unknown state controller parameter(s): supermove",
		"WARNING: Action 413 in chars/.Chars/dgr/monokuma/monokuma.air has no valid frames":                      "WARNING: Action N in <file> has no valid frames",
	}
	for in, want := range cases {
		if got := doctorIssueKey(in); got != want {
			t.Errorf("doctorIssueKey(%q)\n got %q\nwant %q", in, got, want)
		}
	}
}

func TestDoctorEntries(t *testing.T) {
	dir := t.TempDir()
	sel := filepath.Join(dir, "select.def")
	os.WriteFile(sel, []byte("[Characters]\nkfm, stages/kfm.def\nblank\nrandomselect\n; comment\n  Goku OP , order=1\n\n[ExtraStages]\nstages/kfm.def\nstages/other.def ; dup of nothing\n[Options]\narcade.maxmatches = 6\n"), 0o644)
	chars, stages, err := doctorEntries(sel)
	if err != nil {
		t.Fatal(err)
	}
	if len(chars) != 2 || chars[0].Entry != "kfm" || chars[1].Entry != "Goku OP" || chars[1].Line != 6 {
		t.Fatalf("chars = %+v", chars)
	}
	if len(stages) != 2 || stages[0].Entry != "stages/kfm.def" || stages[1].Entry != "stages/other.def" {
		t.Fatalf("stages = %+v (duplicates must be listed once)", stages)
	}
}
