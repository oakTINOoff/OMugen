package mugenjoy

// Which controllers are really in use.
//
// Windows can list one physical controller twice: DS4Windows adds a virtual
// copy of every pad it takes over, and the original then reports nothing, or
// the same presses as the copy. A device becomes live the first time it
// reports a fresh press (not its resting position, which the first poll
// records). A device that becomes live with the same press as another live
// device, at the same moment, mirrors it (its twin) and is left out, until it
// presses things on its own.

import (
	"sort"
	"time"
)

const (
	twinWindow   = 60 * time.Millisecond
	twinMismatch = 2 // own presses that make a twin an independent controller
	pressMemory  = time.Second
)

type press struct {
	t   time.Time
	off int
}

type activity struct {
	id       int // backend device ID
	primed   bool
	live     bool
	liveSeq  int
	twinOf   int // ID of the live device this one mirrors, -1
	mismatch int
	recent   []press
	checked  int // recent[:checked] were compared with the twin partner
	last     time.Time
}

func (a *activity) pressedNear(off int, t time.Time) bool {
	for _, p := range a.recent {
		if p.off == off && p.t.Sub(t) <= twinWindow && t.Sub(p.t) <= twinWindow {
			return true
		}
	}
	return false
}

// activityFor returns the record of a device ID, creating it.
func (m *Manager) activityFor(id int) *activity {
	if m.acts == nil {
		m.acts = map[int]*activity{}
	}
	a := m.acts[id]
	if a == nil {
		a = &activity{id: id, twinOf: -1}
		m.acts[id] = a
	}
	return a
}

// forgetMissing drops the records of devices no longer listed, and makes the
// others record their resting position again (their state was reset).
func (m *Manager) forgetMissing() {
	present := map[int]bool{}
	for _, info := range m.infos {
		present[info.ID] = true
	}
	for id, a := range m.acts {
		if !present[id] {
			delete(m.acts, id)
			continue
		}
		a.primed = false
		if a.twinOf >= 0 && !present[a.twinOf] {
			a.twinOf = -1
		}
	}
	m.gen++
}

func (m *Manager) freshPress(a *activity, off int, now time.Time) {
	a.last = now
	a.recent = append(a.recent, press{now, off})
	if n := len(a.recent); n > 0 && now.Sub(a.recent[0].t) > pressMemory {
		cut := 0
		for cut < n && now.Sub(a.recent[cut].t) > pressMemory {
			cut++
		}
		a.recent = append(a.recent[:0], a.recent[cut:]...)
		if a.checked -= cut; a.checked < 0 {
			a.checked = 0
		}
	}
	if a.live {
		return
	}
	a.live, a.twinOf, a.mismatch = true, -1, 0
	m.seq++
	a.liveSeq = m.seq
	for _, b := range m.acts {
		if b != a && b.live && b.twinOf < 0 && b.pressedNear(off, now) {
			a.twinOf = b.id
			a.checked = len(a.recent)
			break
		}
	}
	m.gen++
}

// checkTwins frees twins whose own presses the partner doesn't mirror.
func (m *Manager) checkTwins(now time.Time) {
	for _, a := range m.acts {
		if a.twinOf < 0 {
			continue
		}
		partner := m.acts[a.twinOf]
		if partner == nil || !partner.live {
			a.twinOf = -1
			m.gen++
			continue
		}
		for a.checked < len(a.recent) && now.Sub(a.recent[a.checked].t) > twinWindow {
			p := a.recent[a.checked]
			if !partner.pressedNear(p.off, p.t) {
				a.mismatch++
			}
			a.checked++
		}
		if a.mismatch >= twinMismatch {
			a.twinOf = -1
			m.gen++
		}
	}
}

// Live lists the indexes of the devices that have reported a press, in the
// order they first did, leaving out devices that mirror another one.
func (m *Manager) Live() []int {
	m.mu.Lock()
	defer m.mu.Unlock()
	type entry struct{ idx, seq int }
	var live []entry
	for _, info := range m.infos {
		if a := m.acts[info.ID]; a != nil && a.live && a.twinOf < 0 {
			live = append(live, entry{info.Index, a.liveSeq})
		}
	}
	sort.Slice(live, func(i, j int) bool { return live[i].seq < live[j].seq })
	out := make([]int, len(live))
	for i, e := range live {
		out[i] = e.idx
	}
	return out
}

// Generation changes whenever the device list or the live devices change.
func (m *Manager) Generation() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.gen
}

// LastPress reports when device idx last reported a fresh press.
func (m *Manager) LastPress(idx int) time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, info := range m.infos {
		if info.Index == idx {
			if a := m.acts[info.ID]; a != nil {
				return a.last
			}
		}
	}
	return time.Time{}
}

// ResetLive forgets which devices are live, so the next presses decide the
// order again.
func (m *Manager) ResetLive() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, a := range m.acts {
		a.live, a.twinOf, a.mismatch, a.recent, a.checked = false, -1, 0, nil, 0
	}
	m.gen++
}
