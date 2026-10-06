package main

import (
	"testing"
	"time"

	"github.com/oakTINOoff/OMugen/src/mugenjoy"
)

// Controllers read through SDL produce the codes MUGEN gives the same
// controller through WinMM (the user's DS4 bindings: Square 17, Cross 18,
// R2 24, Options 26, left stick 1-4, d-pad 13-16, triggers held at rest
// on 9 and 11).
func TestSDLPadsKeepMugenCodes(t *testing.T) {
	codesFor := func(ps bool, s padState) map[int]bool {
		layout := mugenXboxLayout
		if ps {
			layout = mugenDS4Layout
		}
		m := mugenjoy.NewFakeManager([]mugenjoy.DeviceInfo{{Index: 0, ID: 0, Layout: layout}})
		m.SetFakeInput(0, mugenPadSnapshot(ps, padState{})) // resting position
		m.AdvanceFakeClock(10 * time.Millisecond)
		m.Poll()
		m.SetFakeInput(0, mugenPadSnapshot(ps, s))
		m.AdvanceFakeClock(10 * time.Millisecond)
		m.Poll()
		out := map[int]bool{}
		for c := 1; c < mugenjoy.CodesPerJoystick; c++ {
			if m.Pressed(c) {
				out[c] = true
			}
		}
		return out
	}
	for _, tc := range []struct {
		name string
		ps   bool
		s    padState
		want []int
	}{
		{"DS4 at rest", true, padState{}, []int{9, 11}},
		{"Square", true, padState{x: true}, []int{9, 11, 17}},
		{"Cross", true, padState{a: true}, []int{9, 11, 18}},
		{"Options", true, padState{start: true}, []int{9, 11, 26}},
		{"R2", true, padState{rt: 32767}, []int{9, 12, 24}},
		{"left stick up", true, padState{ly: -32768}, []int{1, 9, 11}},
		{"left stick right", true, padState{lx: 32767}, []int{4, 9, 11}},
		{"d-pad up", true, padState{up: true}, []int{9, 11, 13}},
		{"Xbox A", false, padState{a: true}, []int{15}},
		{"Xbox Start", false, padState{start: true}, []int{22}},
	} {
		got := codesFor(tc.ps, tc.s)
		if len(got) != len(tc.want) {
			t.Errorf("%s: codes %v, want %v", tc.name, got, tc.want)
			continue
		}
		for _, c := range tc.want {
			if !got[c] {
				t.Errorf("%s: codes %v, want %v", tc.name, got, tc.want)
				break
			}
		}
	}
}
