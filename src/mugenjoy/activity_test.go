package mugenjoy

import (
	"reflect"
	"testing"
	"time"
)

var ds4Layout = Layout{NumAxes: 6, NumHats: 1, NumButtons: 14}

// DS4Windows over Bluetooth: each DS4 is listed twice, and only the virtual
// copies (B and D here) report input.
func fourDS4s() []DeviceInfo {
	var devs []DeviceInfo
	for i := 0; i < 4; i++ {
		devs = append(devs, DeviceInfo{Index: i, ID: i, VendorID: 0x054C, ProductID: 0x05C4, Layout: ds4Layout})
	}
	return devs
}

// step polls the fake manager after moving its clock.
func step(m *Manager, d time.Duration) {
	m.AdvanceFakeClock(d)
	m.Poll()
}

func TestLiveFollowsFirstPresses(t *testing.T) {
	m := NewFakeManager(fourDS4s())
	// Triggers rest at -32768: held from the first poll, never a press.
	rest := Snapshot{Axes: [maxAxes]int32{0, 0, 0, -32768, -32768, 0}}
	for id := 0; id < 4; id++ {
		m.SetFakeInput(id, rest)
	}
	step(m, 10*time.Millisecond) // resting position
	step(m, 10*time.Millisecond)
	if got := m.Live(); len(got) != 0 {
		t.Fatalf("resting triggers made devices live: %v", got)
	}
	// The second listed DS4 (D) is pressed first, then B.
	m.SetFakeInput(3, Snapshot{Axes: rest.Axes, Buttons: 1})
	step(m, 10*time.Millisecond)
	m.SetFakeInput(1, Snapshot{Axes: rest.Axes, Buttons: 2})
	step(m, 500*time.Millisecond)
	if got := m.Live(); !reflect.DeepEqual(got, []int{3, 1}) {
		t.Errorf("live = %v, want [3 1] (press order)", got)
	}
	if m.LastPress(1).IsZero() || !m.LastPress(0).IsZero() {
		t.Errorf("LastPress: B %v, A %v", m.LastPress(1), m.LastPress(0))
	}
}

// When the original pad and its virtual copy both report the same presses,
// only the first one to report is used.
func TestTwinIsLeftOut(t *testing.T) {
	m := NewFakeManager(fourDS4s())
	step(m, 10*time.Millisecond)
	press := func(ids []int, buttons uint32) {
		for _, id := range ids {
			m.SetFakeInput(id, Snapshot{Buttons: buttons})
		}
	}
	press([]int{1}, 1) // the virtual copy first
	step(m, 5*time.Millisecond)
	press([]int{0}, 1) // the original 5 ms later
	step(m, 5*time.Millisecond)
	step(m, 200*time.Millisecond)
	press([]int{0, 1}, 0)
	step(m, 50*time.Millisecond)
	press([]int{0, 1}, 4) // mirrored again
	step(m, 200*time.Millisecond)
	if got := m.Live(); !reflect.DeepEqual(got, []int{1}) {
		t.Fatalf("live = %v, want only [1] (A mirrors B)", got)
	}

	// A device that presses on its own is a separate controller.
	n := NewFakeManager(fourDS4s())
	step(n, 10*time.Millisecond)
	n.SetFakeInput(1, Snapshot{Buttons: 1})
	n.SetFakeInput(3, Snapshot{Buttons: 1}) // same button, same moment: looks like a twin
	step(n, 200*time.Millisecond)
	if got := n.Live(); !reflect.DeepEqual(got, []int{1}) {
		t.Fatalf("live = %v, want [1] at first", got)
	}
	for i, b := range []uint32{2, 0, 8, 0} { // D alone, twice
		n.SetFakeInput(3, Snapshot{Buttons: b})
		step(n, 100*time.Millisecond)
		_ = i
	}
	step(n, 200*time.Millisecond)
	if got := n.Live(); !reflect.DeepEqual(got, []int{1, 3}) {
		t.Errorf("live = %v, want [1 3] once D pressed on its own", got)
	}
}
