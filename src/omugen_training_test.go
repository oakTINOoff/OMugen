package main

import "testing"

func TestTrainPack(t *testing.T) {
	// Left (back when facing right) and a held: played from the other side,
	// the same back is right.
	var b [14]bool
	b[2], b[4] = true, true // L, a
	v := trainPack(b, false)
	if got := trainUnpack(v, false); got != b {
		t.Errorf("same side: %v", got)
	}
	got := trainUnpack(v, true)
	if !got[3] || got[2] || !got[4] {
		t.Errorf("other side: back should be right, got %v", got)
	}
	// Recorded facing left: right was back.
	var r [14]bool
	r[3] = true
	if got := trainUnpack(trainPack(r, true), false); !got[2] || got[3] {
		t.Errorf("recorded facing left, played facing right: %v", got)
	}
}

func TestTrainInputs(t *testing.T) {
	old, oldInputs := sys.gameMode, omugenTrain.inputs
	defer func() {
		sys.gameMode, omugenTrain.inputs = old, oldInputs
		omugenTrain.history = [2][]trainInput{}
	}()
	sys.gameMode = "training"
	omugenTrain.inputs = 1
	omugenTrain.history = [2][]trainInput{}
	c := &Char{}
	for i := 0; i < 3; i++ {
		omugenTrainInputs(c, false, true, false, true, false, false, false, false, false, false, false, false, false) // down-forward
	}
	omugenTrainInputs(c, false, true, false, true, true, false, false, false, false, false, false, false, false)  // + a
	omugenTrainInputs(c, true, false, true, false, false, false, false, false, false, false, false, false, false) // up-back
	h := omugenTrain.history[0]
	want := []trainInput{{dir: 3, frames: 3}, {dir: 3, buttons: 1, frames: 1}, {dir: 7, frames: 1}}
	if len(h) != len(want) {
		t.Fatalf("history %+v", h)
	}
	for i := range want {
		if h[i] != want[i] {
			t.Errorf("entry %d = %+v, want %+v", i, h[i], want[i])
		}
	}
	for i := 0; i < 40; i++ {
		omugenTrainInputs(c, i%2 == 0, false, false, false, false, false, false, false, false, false, false, false, false)
	}
	if len(omugenTrain.history[0]) != trainHistoryLen {
		t.Errorf("history keeps %d entries, want %d", len(omugenTrain.history[0]), trainHistoryLen)
	}
	sys.gameMode = "arcade"
	omugenTrain.history = [2][]trainInput{}
	omugenTrainInputs(c, true, false, false, false, false, false, false, false, false, false, false, false, false)
	if len(omugenTrain.history[0]) != 0 {
		t.Errorf("inputs noted outside training")
	}
}
