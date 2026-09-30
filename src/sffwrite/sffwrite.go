// Package sffwrite writes Elecbyte SFF v2.01 sprite files with PNG-compressed
// sprites, as read by OMugen and MUGEN 1.1: true-color sprites (format 12,
// PNG32) and 8-bit paletted sprites (format 10, PNG8) that take their colors
// from the file's palette table, so characters can switch palettes.
//
// Layout: 512-byte header, sprite headers (28 bytes each), palette headers
// (16 bytes each), then "ldata" (palettes, 4 bytes RGBA per color) and
// "tdata" (sprite data). Each PNG sprite's data is a uint32 uncompressed size
// followed by the PNG bytes.
package sffwrite

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
)

// Sprite is one image with its group/number and axis (the point drawn at the
// element's position, like MUGEN's sprite offset). An *image.Paletted image
// is stored as an 8-bit sprite that uses palette number Palette (an index
// into the palettes passed to WriteWithPalettes); color index 0 is
// transparent. Any other image is stored as 32-bit RGBA.
type Sprite struct {
	Group, Number uint16
	AxisX, AxisY  int16
	Image         image.Image
	Palette       int
}

// Palette is a palette table entry, e.g. 1,1 to 1,12 for a character's
// selectable colors.
type Palette struct {
	Group, Number uint16
	Colors        []color.RGBA // up to 256; index 0 is transparent
}

// Write encodes true-color sprites into an SFF v2.01 file.
func Write(w io.Writer, sprites []Sprite) error {
	return WriteWithPalettes(w, sprites, nil)
}

// WriteWithPalettes encodes sprites and palettes into an SFF v2.01 file.
func WriteWithPalettes(w io.Writer, sprites []Sprite, palettes []Palette) error {
	if len(sprites) == 0 {
		return errors.New("sffwrite: no sprites")
	}
	if len(palettes) == 0 {
		// One neutral palette: some readers expect at least one.
		palettes = []Palette{{Group: 0, Number: 0, Colors: make([]color.RGBA, 256)}}
	}
	const headerSize = 512
	nSpr, nPal := len(sprites), len(palettes)
	sprHdrOfs := uint32(headerSize)
	palHdrOfs := sprHdrOfs + uint32(28*nSpr)
	ldataOfs := palHdrOfs + uint32(16*nPal)

	var ldata bytes.Buffer
	palOfs := make([]uint32, nPal)
	for i, p := range palettes {
		if len(p.Colors) == 0 || len(p.Colors) > 256 {
			return errors.New("sffwrite: palette must have 1 to 256 colors")
		}
		palOfs[i] = uint32(ldata.Len())
		for j, c := range p.Colors {
			a := c.A
			if j == 0 {
				a = 0
			} else if a == 0 {
				a = 255
			}
			ldata.Write([]byte{c.R, c.G, c.B, a})
		}
	}
	tdataOfs := ldataOfs + uint32(ldata.Len())

	var tdata bytes.Buffer
	type sprInfo struct {
		ofs, size    uint32
		w, h         uint16
		format, bits byte
		pal          uint16
	}
	infos := make([]sprInfo, nSpr)
	for i, s := range sprites {
		b := s.Image.Bounds()
		if b.Dx() <= 0 || b.Dy() <= 0 || b.Dx() > 65535 || b.Dy() > 65535 {
			return errors.New("sffwrite: sprite size out of range")
		}
		info := sprInfo{w: uint16(b.Dx()), h: uint16(b.Dy()), format: 12, bits: 32}
		raw := uint32(b.Dx() * b.Dy() * 4)
		if _, ok := s.Image.(*image.Paletted); ok {
			if s.Palette < 0 || s.Palette >= nPal {
				return errors.New("sffwrite: sprite palette out of range")
			}
			info.format, info.bits, info.pal = 10, 8, uint16(s.Palette)
			raw = uint32(b.Dx() * b.Dy())
		}
		var pngBuf bytes.Buffer
		enc := png.Encoder{CompressionLevel: png.BestCompression}
		if err := enc.Encode(&pngBuf, s.Image); err != nil {
			return err
		}
		info.ofs = uint32(tdata.Len())
		info.size = uint32(4 + pngBuf.Len())
		infos[i] = info
		binary.Write(&tdata, binary.LittleEndian, raw)
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
	le(uint32(ldata.Len()))
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
		le(uint16(0)) // linked sprite index (unused: size > 0)
		out.WriteByte(in.format)
		out.WriteByte(in.bits)
		le(in.ofs)    // offset in tdata
		le(in.size)   // data size
		le(in.pal)    // palette index
		le(uint16(1)) // flags: data is in tdata
	}
	for i, p := range palettes {
		le(p.Group)
		le(p.Number)
		le(uint16(len(p.Colors)))
		le(uint16(0)) // link
		le(palOfs[i]) // offset in ldata
		le(uint32(len(p.Colors) * 4))
	}
	out.Write(ldata.Bytes())
	out.Write(tdata.Bytes())
	_, err := w.Write(out.Bytes())
	return err
}
