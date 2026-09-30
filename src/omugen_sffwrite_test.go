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
