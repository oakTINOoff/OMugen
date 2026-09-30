//go:build windows

package mugenjoy

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// WinMM joystick API, the same one MUGEN 1.1's SDL.dll (SDL 1.2) uses.
var (
	winmm              = windows.NewLazySystemDLL("winmm.dll")
	procJoyGetNumDevs  = winmm.NewProc("joyGetNumDevs")
	procJoyGetDevCapsW = winmm.NewProc("joyGetDevCapsW")
	procJoyGetPosEx    = winmm.NewProc("joyGetPosEx")
)

const (
	joyReturnX       = 0x001
	joyReturnY       = 0x002
	joyReturnZ       = 0x004
	joyReturnR       = 0x008
	joyReturnU       = 0x010
	joyReturnV       = 0x020
	joyReturnPOV     = 0x040
	joyReturnButtons = 0x080
	joyReturnPOVCTS  = 0x200
	joyReturnAll     = joyReturnX | joyReturnY | joyReturnZ | joyReturnR | joyReturnU | joyReturnV | joyReturnPOV | joyReturnButtons

	joyCapsHasZ   = 0x01
	joyCapsHasR   = 0x02
	joyCapsHasU   = 0x04
	joyCapsHasV   = 0x08
	joyCapsHasPOV = 0x10

	joyPOVCentered = 0xFFFF
	joyPOVForward  = 0
	joyPOVRight    = 9000
	joyPOVBackward = 18000
	joyPOVLeft     = 27000

	sdlMaxJoysticks = 16 // SDL 1.2 MAX_JOYSTICKS for the WinMM backend
	axisMin         = -32768
	axisMax         = 32767
)

type joyCapsW struct {
	Mid, Pid                           uint16
	Pname                              [32]uint16
	Xmin, Xmax, Ymin, Ymax, Zmin, Zmax uint32
	NumButtons                         uint32
	PeriodMin, PeriodMax               uint32
	Rmin, Rmax, Umin, Umax, Vmin, Vmax uint32
	Caps                               uint32
	MaxAxes, NumAxes, MaxButtons       uint32
	RegKey                             [32]uint16
	OEMVxD                             [260]uint16
}

type joyInfoEx struct {
	Size, Flags                        uint32
	Xpos, Ypos, Zpos, Rpos, Upos, Vpos uint32
	Buttons, ButtonNumber, POV         uint32
	Reserved1, Reserved2               uint32
}

// transAxis is SDL 1.2's per-axis scaling from the device range to -32768..32767.
type transAxis struct {
	offset float32
	scale  float32
}

type winmmBackend struct {
	trans map[int][maxAxes]transAxis // keyed by WinMM ID
}

func newBackend() backend {
	return &winmmBackend{trans: map[int][maxAxes]transAxis{}}
}

func joyGetPosEx(id int, info *joyInfoEx) bool {
	return joyGetPosExCode(id, info) == 0
}

func joyGetPosExCode(id int, info *joyInfoEx) uintptr {
	info.Size = uint32(unsafe.Sizeof(*info))
	r, _, _ := procJoyGetPosEx.Call(uintptr(id), uintptr(unsafe.Pointer(info)))
	return r
}

// RawDump reports what WinMM returns for every joystick ID, for diagnostics.
func RawDump() []string {
	if procJoyGetNumDevs.Find() != nil {
		return []string{"winmm.dll not available"}
	}
	n, _, _ := procJoyGetNumDevs.Call()
	var out []string
	for id := 0; id < int(n); id++ {
		var info joyInfoEx
		info.Flags = joyReturnAll | joyReturnPOVCTS
		r := joyGetPosExCode(id, &info)
		if r != 0 {
			if r != 167 { // JOYERR_UNPLUGGED is the normal answer for empty IDs
				out = append(out, fmt.Sprintf("id %2d: error %d", id, r))
			}
			continue
		}
		out = append(out, fmt.Sprintf("id %2d: X=%5d Y=%5d Z=%5d R=%5d U=%5d V=%5d buttons=%08x pov=%d",
			id, info.Xpos, info.Ypos, info.Zpos, info.Rpos, info.Upos, info.Vpos, info.Buttons, info.POV))
	}
	return out
}

// enumerate mirrors SDL_SYS_JoystickInit: every WinMM ID that answers
// joyGetPosEx and joyGetDevCaps becomes the next SDL joystick index.
func (b *winmmBackend) enumerate() []DeviceInfo {
	if procJoyGetNumDevs.Find() != nil {
		return nil
	}
	maxDevs, _, _ := procJoyGetNumDevs.Call()
	var out []DeviceInfo
	for id := 0; id < int(maxDevs) && len(out) < sdlMaxJoysticks; id++ {
		var info joyInfoEx
		info.Flags = joyReturnAll
		if !joyGetPosEx(id, &info) {
			continue
		}
		var caps joyCapsW
		r, _, _ := procJoyGetDevCapsW.Call(uintptr(id), uintptr(unsafe.Pointer(&caps)), unsafe.Sizeof(caps))
		if r != 0 {
			continue
		}
		layout := Layout{
			NumAxes:    int(caps.NumAxes),
			NumButtons: int(caps.NumButtons),
		}
		if layout.NumAxes > maxAxes {
			layout.NumAxes = maxAxes
		}
		if caps.Caps&joyCapsHasPOV != 0 {
			layout.NumHats = 1
		}
		b.trans[id] = axisTransforms(&caps)
		out = append(out, DeviceInfo{
			Index:  len(out),
			ID:     id,
			Name:   windows.UTF16ToString(caps.Pname[:]),
			Layout: layout,
		})
	}
	return out
}

// axisTransforms mirrors SDL_SYS_JoystickOpen's transaxis setup.
func axisTransforms(caps *joyCapsW) [maxAxes]transAxis {
	mins := [maxAxes]uint32{caps.Xmin, caps.Ymin, caps.Zmin, caps.Rmin, caps.Umin, caps.Vmin}
	maxs := [maxAxes]uint32{caps.Xmax, caps.Ymax, caps.Zmax, caps.Rmax, caps.Umax, caps.Vmax}
	flags := [maxAxes - 2]uint32{joyCapsHasZ, joyCapsHasR, joyCapsHasU, joyCapsHasV}
	var t [maxAxes]transAxis
	for i := 0; i < maxAxes; i++ {
		if (i < 2 || caps.Caps&flags[i-2] != 0) && maxs[i] != mins[i] {
			t[i].offset = float32(axisMin - int64(mins[i]))
			t[i].scale = float32(axisMax-axisMin) / float32(int64(maxs[i])-int64(mins[i]))
		} else {
			t[i].offset = 0
			t[i].scale = 1
		}
	}
	return t
}

// read mirrors SDL_SYS_JoystickUpdate.
func (b *winmmBackend) read(d DeviceInfo) (Snapshot, bool) {
	var info joyInfoEx
	info.Flags = joyReturnAll | joyReturnPOVCTS
	if d.Layout.NumHats == 0 {
		info.Flags &^= joyReturnPOV | joyReturnPOVCTS
	}
	if !joyGetPosEx(d.ID, &info) {
		return Snapshot{}, false
	}
	var snap Snapshot
	t := b.trans[d.ID]
	pos := [maxAxes]uint32{info.Xpos, info.Ypos, info.Zpos, info.Rpos, info.Upos, info.Vpos}
	for i := 0; i < d.Layout.NumAxes; i++ {
		snap.Axes[i] = int32((float32(pos[i]) + t[i].offset) * t[i].scale)
	}
	snap.Buttons = info.Buttons
	if d.Layout.NumHats > 0 {
		snap.Hat = translatePOV(info.POV)
	}
	return snap, true
}

// translatePOV mirrors SDL 1.2's TranslatePOV (diagonals set two bits).
func translatePOV(v uint32) uint8 {
	var pos uint8
	if v == joyPOVCentered {
		return 0
	}
	if v > joyPOVLeft || v < joyPOVRight {
		pos |= HatUp
	}
	if v > joyPOVForward && v < joyPOVBackward {
		pos |= HatRight
	}
	if v > joyPOVRight && v < joyPOVLeft {
		pos |= HatDown
	}
	if v > joyPOVBackward {
		pos |= HatLeft
	}
	return pos
}
