package mugenjoy

import (
	"sync"
	"time"
)

// DeviceInfo describes an enumerated joystick.
type DeviceInfo struct {
	Index     int    // MUGEN joystick index (0-3), i.e. the code block
	ID        int    // backend device ID (WinMM joystick ID on Windows)
	Name      string // device name reported by the backend
	VendorID  uint16 // USB vendor ID when known (WinMM wMid), else 0
	ProductID uint16 // USB product ID when known (WinMM wPid), else 0
	Layout    Layout
}

// backend is the platform source of raw joystick data.
type backend interface {
	// enumerate lists devices in the order SDL 1.2 would number them.
	enumerate() []DeviceInfo
	// read polls one device; ok is false if it stopped responding.
	read(info DeviceInfo) (snap Snapshot, ok bool)
}

// Manager polls joysticks and keeps MUGEN's joystick state table.
type Manager struct {
	mu       sync.Mutex
	be       backend
	infos    []DeviceInfo
	devs     []device
	state    State
	lastPoll time.Time
	lastScan time.Time
	// activity.go: which devices are live, by backend device ID
	acts map[int]*activity
	seq  int
	gen  int
	now  func() time.Time
}

// rescanInterval controls how often new or unplugged devices are picked up.
// MUGEN itself only enumerates at startup; rescanning keeps the same
// numbering rule while allowing hot-plugging.
const rescanInterval = 3 * time.Second

// minPollInterval avoids re-reading devices several times within one frame.
const minPollInterval = time.Millisecond

// NewManager enumerates devices using the platform backend.
func NewManager() *Manager {
	m := &Manager{be: newBackend(), now: time.Now}
	m.rescan(m.now())
	return m
}

func (m *Manager) rescan(now time.Time) {
	infos := m.be.enumerate()
	if len(infos) > MaxJoysticks {
		infos = infos[:MaxJoysticks]
	}
	if sameDevices(infos, m.infos) {
		m.lastScan = now
		return
	}
	m.infos = infos
	m.devs = make([]device, len(infos))
	for i, info := range infos {
		m.devs[i].layout = info.Layout
	}
	m.state = State{}
	m.lastScan = now
	m.forgetMissing()
}

func sameDevices(a, b []DeviceInfo) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ID != b[i].ID || a[i].Layout != b[i].Layout {
			return false
		}
	}
	return true
}

// Poll refreshes the state table. It is cheap to call several times per frame.
func (m *Manager) Poll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	if now.Sub(m.lastPoll) < minPollInterval {
		return
	}
	m.lastPoll = now
	if now.Sub(m.lastScan) >= rescanInterval {
		m.rescan(now)
	}
	lost := false
	for i := range m.devs {
		snap, ok := m.be.read(m.infos[i])
		if !ok {
			lost = true
			continue
		}
		base := i * CodesPerJoystick
		var before [CodesPerJoystick]bool
		copy(before[:], m.state[base:base+CodesPerJoystick])
		m.devs[i].apply(i, snap, &m.state)
		a := m.activityFor(m.infos[i].ID)
		if !a.primed {
			a.primed = true // the first poll is the resting position
			continue
		}
		for off := 1; off < CodesPerJoystick; off++ {
			if !before[off] && m.state[base+off] {
				m.freshPress(a, off, now)
			}
		}
	}
	m.checkTwins(now)
	if lost {
		m.rescan(now)
	}
}

// Pressed reports whether a MUGEN joystick code is currently held.
func (m *Manager) Pressed(code int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state.Pressed(code)
}

// Snapshot returns a copy of the whole state table.
func (m *Manager) Snapshot() State {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state
}

// Device returns the joystick with index i, without copying the list (for
// lookups made every frame).
func (m *Manager) Device(i int) (DeviceInfo, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, d := range m.infos {
		if d.Index == i {
			return d, true
		}
	}
	return DeviceInfo{}, false
}

// Devices lists the currently enumerated joysticks.
func (m *Manager) Devices() []DeviceInfo {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]DeviceInfo(nil), m.infos...)
}

// Backend is a source of joysticks for NewManagerWith: it lists devices and
// reports each one's state already in SDL 1.2 / WinMM terms (Snapshot), so
// MUGEN's codes stay the same whatever the source.
type Backend interface {
	Enumerate() []DeviceInfo
	Read(info DeviceInfo) (snap Snapshot, ok bool)
}

type externalBackend struct{ b Backend }

func (e externalBackend) enumerate() []DeviceInfo               { return e.b.Enumerate() }
func (e externalBackend) read(info DeviceInfo) (Snapshot, bool) { return e.b.Read(info) }

// NewManagerWith returns a Manager reading joysticks from b.
func NewManagerWith(b Backend) *Manager {
	m := &Manager{be: externalBackend{b}, now: time.Now}
	m.rescan(m.now())
	return m
}
