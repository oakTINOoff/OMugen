// Package sffwrite writes Elecbyte SFF v2.01 sprite files with PNG-compressed
// true-color sprites (format 12), as read by OMugen/Ikemen and MUGEN 1.1.
//
// Layout: 512-byte header, sprite headers (28 bytes each), palette headers
// (16 bytes each), then "ldata" (palettes) and "tdata" (sprite data). Each
// PNG sprite's data is a uint32 uncompressed size followed by the PNG bytes.
package sffwrite

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/png"
	"io"
)

// Sprite is one image with its group/number and axis (the point drawn at the
// element's position, like MUGEN's sprite offset).
type Sprite struct {
	Group, Number uint16
	AxisX, AxisY  int16
	Image         image.Image
}

// Write encodes sprites into an SFF v2.01 file.
func Write(w io.Writer, sprites []Sprite) error {
	if len(sprites) == 0 {
		return errors.New("sffwrite: no sprites")
	}
	const headerSize = 512
	nSpr, nPal := len(sprites), 1
	sprHdrOfs := uint32(headerSize)
	palHdrOfs := sprHdrOfs + uint32(28*nSpr)
	ldataOfs := palHdrOfs + uint32(16*nPal)

	// One neutral 256-color palette: some readers expect at least one.
	ldata := make([]byte, 256*4)
	tdataOfs := ldataOfs + uint32(len(ldata))

	var tdata bytes.Buffer
	type sprInfo struct {
		ofs, size uint32
		w, h      uint16
	}
	infos := make([]sprInfo, nSpr)
	for i, s := range sprites {
		b := s.Image.Bounds()
		if b.Dx() <= 0 || b.Dy() <= 0 || b.Dx() > 65535 || b.Dy() > 65535 {
			return errors.New("sffwrite: sprite size out of range")
		}
		var pngBuf bytes.Buffer
		enc := png.Encoder{CompressionLevel: png.BestCompression}
		if err := enc.Encode(&pngBuf, s.Image); err != nil {
			return err
		}
		infos[i] = sprInfo{ofs: uint32(tdata.Len()), size: uint32(4 + pngBuf.Len()), w: uint16(b.Dx()), h: uint16(b.Dy())}
		binary.Write(&tdata, binary.LittleEndian, uint32(b.Dx()*b.Dy()*4))
		tdata.Write(pngBuf.Bytes())
	}

	var out bytes.Buffer
	le := func(v interface{}) { binary.Write(&out, binary.LittleEndian, v) }
	out.WriteString("ElecbyteSpr\x00")
	out.Write([]byte{0, 1, 0, 2}) // version 2.01 (lo3, lo2, lo1, hi)
	le(uint32(0))
	le(uint32(0))
	out.Write([]byte{0, 0, 0, 2}) // compatible version 2.00
	le(uint32(0))
	le(uint32(0))
	le(sprHdrOfs)
	le(uint32(nSpr))
	le(palHdrOfs)
	le(uint32(nPal))
	le(ldataOfs)
	le(uint32(len(ldata)))
	le(tdataOfs)
	le(uint32(tdata.Len()))
	out.Write(make([]byte, headerSize-out.Len()))

	for i, s := range sprites {
		in := infos[i]
		le(s.Group)
		le(s.Number)
		le(in.w)
		le(in.h)
		le(s.AxisX)
		le(s.AxisY)
		le(uint16(0))     // linked sprite index (unused: size > 0)
		out.WriteByte(12) // format: PNG, 32-bit
		out.WriteByte(32) // color depth
		le(in.ofs)        // offset in tdata
		le(in.size)       // data size
		le(uint16(0))     // palette index
		le(uint16(1))     // flags: data is in tdata
	}
	le(uint16(0)) // palette group
	le(uint16(0)) // palette number
	le(uint16(256))
	le(uint16(0)) // link
	le(uint32(0)) // offset in ldata
	le(uint32(len(ldata)))
	out.Write(ldata)
	out.Write(tdata.Bytes())
	_, err := w.Write(out.Bytes())
	return err
}
