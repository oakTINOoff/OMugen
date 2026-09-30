package main

import (
	"bytes"
	"image"
	imgcolor "image/color"
	"testing"

	"github.com/ikemen-engine/Ikemen-GO/src/sffwrite"
)

// Files written by sffwrite must parse with OMugen's own SFF v2 reader.
func TestSffWriteRoundTrip(t *testing.T) {
	a := image.NewRGBA(image.Rect(0, 0, 40, 20))
	for y := 0; y < 20; y++ {
		for x := 0; x < 40; x++ {
			a.Set(x, y, imgcolor.RGBA{uint8(x * 6), uint8(y * 12), 200, 255})
		}
	}
	b := image.NewRGBA(image.Rect(0, 0, 7, 9))
	var buf bytes.Buffer
	if err := sffwrite.Write(&buf, []sffwrite.Sprite{
		{Group: 0, Number: 0, AxisX: 20, AxisY: 10, Image: a},
		{Group: 150, Number: 3, AxisX: -2, AxisY: 4, Image: b},
	}); err != nil {
		t.Fatal(err)
	}
	r := bytes.NewReader(buf.Bytes())

	var h SffHeader
	var lofs, tofs uint32
	if err := h.Read(r, &lofs, &tofs); err != nil {
		t.Fatal(err)
	}
	if h.Version[0] != 2 || h.NumberOfSprites != 2 || h.NumberOfPalettes != 1 {
		t.Fatalf("header %+v", h)
	}
	want := []struct {
		g, n uint16
		size [2]uint16
		ofs  [2]int16
	}{{0, 0, [2]uint16{40, 20}, [2]int16{20, 10}}, {150, 3, [2]uint16{7, 9}, [2]int16{-2, 4}}}
	for i, w := range want {
		r.Seek(int64(h.FirstSpriteHeaderOffset)+int64(28*i), 0)
		s := newSprite()
		var xofs, size uint32
		var link uint16
		if err := s.readHeaderV2(r, &xofs, &size, lofs, tofs, &link); err != nil {
			t.Fatal(err)
		}
		if s.Group != w.g || s.Number != w.n || s.Size != w.size || s.Offset != w.ofs || s.coldepth != 32 || s.rle != -12 {
			t.Fatalf("sprite %d header: %+v", i, s)
		}
		if err := s.readV2(r, int64(xofs), size); err != nil {
			t.Fatalf("sprite %d data: %v", i, err)
		}
	}
	for len(sys.mainThreadTask) > 0 { // texture uploads need a renderer; not under test
		<-sys.mainThreadTask
	}
}

// 8-bit sprites use the file's palettes (1,1 and 1,2 here).
func TestSffWritePaletted(t *testing.T) {
	colors := func(c imgcolor.RGBA) []imgcolor.RGBA {
		p := make([]imgcolor.RGBA, 256)
		p[1] = c
		return p
	}
	img := image.NewPaletted(image.Rect(0, 0, 6, 4), imgcolor.Palette{imgcolor.RGBA{}, imgcolor.RGBA{255, 0, 0, 255}})
	img.Pix[5] = 1
	var buf bytes.Buffer
	err := sffwrite.WriteWithPalettes(&buf, []sffwrite.Sprite{{Group: 9000, Number: 0, Image: img, Palette: 0}},
		[]sffwrite.Palette{{Group: 1, Number: 1, Colors: colors(imgcolor.RGBA{200, 10, 20, 255})},
			{Group: 1, Number: 2, Colors: colors(imgcolor.RGBA{20, 40, 220, 255})}})
	if err != nil {
		t.Fatal(err)
	}
	r := bytes.NewReader(buf.Bytes())
	sff := newSff()
	var lofs, tofs uint32
	if err := sff.header.Read(r, &lofs, &tofs); err != nil {
		t.Fatal(err)
	}
	if sff.header.NumberOfPalettes != 2 {
		t.Fatalf("palettes = %d", sff.header.NumberOfPalettes)
	}
	if err := sff.loadPalettes(r, lofs); err != nil {
		t.Fatal(err)
	}
	for i, want := range []uint32{0xff14_0ac8, 0xffdc_2814} {
		pal := sff.palList.Get(i)
		if pal[0]>>24 != 0 || pal[1] != want {
			t.Errorf("palette %d: index 0 %08x, index 1 %08x, want %08x", i, pal[0], pal[1], want)
		}
	}
	if idx, ok := sff.palList.PalTable[[2]uint16{1, 2}]; !ok || idx != 1 {
		t.Errorf("palette 1,2 -> %v %v", idx, ok)
	}
	r.Seek(int64(sff.header.FirstSpriteHeaderOffset), 0)
	s := newSprite()
	var xofs, size uint32
	var link uint16
	if err := s.readHeaderV2(r, &xofs, &size, lofs, tofs, &link); err != nil {
		t.Fatal(err)
	}
	if s.coldepth != 8 || s.rle != -10 || s.palidx != 0 {
		t.Fatalf("sprite header: depth %d format %d palette %d", s.coldepth, -s.rle, s.palidx)
	}
	if err := s.readV2(r, int64(xofs), size); err != nil {
		t.Fatal(err)
	}
	for len(sys.mainThreadTask) > 0 {
		<-sys.mainThreadTask
	}
}
