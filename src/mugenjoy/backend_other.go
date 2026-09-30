//go:build !windows

package mugenjoy

// MUGEN 1.1 only exists on Windows, so its joystick codes are defined by the
// WinMM backend. Other platforms report no devices for MUGEN-style bindings.
type nullBackend struct{}

func newBackend() backend { return nullBackend{} }

func (nullBackend) enumerate() []DeviceInfo { return nil }

func (nullBackend) read(DeviceInfo) (Snapshot, bool) { return Snapshot{}, false }
