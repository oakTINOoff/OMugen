package mugenjoy

import "time"

// NewFakeManager returns a Manager with a fixed list of devices whose input
// and clock tests set (SetFakeInput, AdvanceFakeClock). Devices report their
// resting position until told otherwise.
func NewFakeManager(devs []DeviceInfo) *Manager {
	fb := &fakeBackend{devs: devs, input: map[int]Snapshot{}, clock: time.Unix(1000, 0)}
	m := &Manager{be: fb, now: func() time.Time { return fb.clock }}
	m.rescan(m.now())
	return m
}

type fakeBackend struct {
	devs  []DeviceInfo
	input map[int]Snapshot // by device ID
	clock time.Time
}

func (f *fakeBackend) enumerate() []DeviceInfo { return append([]DeviceInfo(nil), f.devs...) }

func (f *fakeBackend) read(d DeviceInfo) (Snapshot, bool) { return f.input[d.ID], true }

// SetFakeInput sets what fake device id reports (fake managers only).
func (m *Manager) SetFakeInput(id int, s Snapshot) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.be.(*fakeBackend).input[id] = s
}

// AdvanceFakeClock moves a fake manager's clock (fake managers only).
func (m *Manager) AdvanceFakeClock(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.be.(*fakeBackend).clock = m.be.(*fakeBackend).clock.Add(d)
}

// NewFakeManagerRealtime is NewFakeManager on the real clock, for scripted
// screen tests without controllers (OMUGEN_FAKEPADS).
func NewFakeManagerRealtime(devs []DeviceInfo) *Manager {
	m := NewFakeManager(devs)
	m.now = time.Now
	m.rescan(m.now())
	return m
}
