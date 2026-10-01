package mugenjoy

import "time"

// NewFakeManager returns a Manager with a fixed list of devices that report
// nothing pressed, for tests of code that hands out controllers.
func NewFakeManager(devs []DeviceInfo) *Manager {
	m := &Manager{be: fakeBackend{devs}}
	m.rescan(time.Now())
	return m
}

type fakeBackend struct{ devs []DeviceInfo }

func (f fakeBackend) enumerate() []DeviceInfo { return append([]DeviceInfo(nil), f.devs...) }

func (fakeBackend) read(DeviceInfo) (Snapshot, bool) { return Snapshot{}, true }
