package main

// The lifebar art: bars, portrait frames, the timer badge, win icons and the
// round announcements. Everything faces player 1 (left side); player 2's
// elements use facing = -1 in fight.def.

import (
	"image"
	"math"

	"github.com/oakTINOoff/OMugen/tools/internal/art"
	"github.com/oakTINOoff/OMugen/tools/internal/logo"
)

var (
	ink      = art.Hex(0x07070A, 1)
	panel    = art.Hex(0x101016, 1)
	red      = art.Hex(0xFF2A3A, 1)
	redHot   = art.Hex(0xFF7A66, 1)
	redDeep  = art.Hex(0x9E000B, 1)
	blue     = art.Hex(0x2F8CFF, 1)
	blueHot  = art.Hex(0x8CC4FF, 1)
	blueDeep = art.Hex(0x0A3A8A, 1)
	white    = art.Hex(0xF5F5F7, 1)
	gray     = art.Hex(0xA0A0AC, 1)
	gold     = art.Hex(0xFFC93C, 1)
	goldHot  = art.Hex(0xFFF0B0, 1)
)

const (
	fontXB  = "font/Open_Sans/OpenSans-ExtraBold.ttf"
	fontXBI = "font/Open_Sans/OpenSans-ExtraBoldItalic.ttf"
)

// side holds a player's accent colors.
type side struct{ col, hot, deep art.RGBA }

var sides = [2]side{{red, redHot, redDeep}, {blue, blueHot, blueDeep}}

// barGeom is a slanted bar: length l (along the bottom edge), height h, and
// its top edge shifted right by slant. Sprites are drawn with the bar's
// bottom-left corner at (pad, pad+h) and their axis at its top-left
// corner (pad, pad), so every layer of a bar lines up on the same position.
type barGeom struct{ l, h, slant, pad float64 }

func (g barGeom) size() (int, int) {
	return int(math.Ceil(g.l + g.slant + 2*g.pad)), int(math.Ceil(g.h + 2*g.pad))
}

// shape is the bar grown by d on every side.
func (g barGeom) shape(d float64) []logo.Pt {
	k := g.slant / g.h // horizontal shift per pixel of height
	x0, y0, x1, y1 := g.pad-d-k*d, g.pad-d, g.pad+g.l+d-k*d, g.pad+g.h+d
	return art.Para(x0, y0, x1, y1, g.slant+2*k*d)
}

func (g barGeom) mask(d float64) []float64 {
	w, h := g.size()
	return art.ShapeMask(w, h, g.shape(d))
}

func (g barGeom) ring(outer, inner float64) []float64 {
	w, h := g.size()
	return art.ShapeMask(w, h, g.shape(outer), art.Reversed(g.shape(inner)))
}

func (g barGeom) axis() (int, int) { return int(g.pad), int(g.pad) }

func img(c *art.Canvas) image.Image { return c.Image(false) }

// barBack is the frame and the dark well behind a bar's fill, with the
// player's accent: a glow under the bar and a stripe at its outer end.
func barBack(g barGeom, s side) image.Image {
	w, h := g.size()
	c := art.NewCanvas(w, h)
	// Drop shadow.
	c.PaintMask(art.Blur(g.mask(5), w, h, 4), art.Solid(ink.Alpha(0.7)))
	// Accent glow along the bottom edge.
	glow := art.ShapeMask(w, h, art.Para(g.pad, g.pad+g.h-2, g.pad+g.l, g.pad+g.h+5, 0))
	c.LightMask(art.Blur(glow, w, h, 5), s.col, 0.55)
	// Border and well.
	c.PaintMask(g.mask(4), art.Linear(0, g.pad-4, 0, g.pad+g.h+4, art.Hex(0x3A3A46, 1), art.Hex(0x15151C, 1)))
	c.PaintMask(g.mask(0), art.Linear(0, g.pad, 0, g.pad+g.h, art.Hex(0x1A1A22, 1), art.Hex(0x0B0B10, 1)))
	// Fine diagonal hatching in the well.
	well := g.mask(0)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if v := well[y*w+x]; v > 0 && (x+y)%6 == 0 {
				c.Over(x, y, white, v*0.05)
			}
		}
	}
	// Accent stripe at the outer end.
	k := g.slant / g.h
	stripe := art.ShapeMask(w, h, art.Para(g.pad-4-k*4, g.pad-4, g.pad+10-k*4, g.pad+g.h+4, g.slant+8*k))
	c.PaintMask(stripe, art.Linear(0, g.pad-4, 0, g.pad+g.h+4, s.hot, s.col))
	return img(c)
}

// barTop is drawn over the fill: the border's bright edges, so the fill
// tucks under the frame.
func barTop(g barGeom) image.Image {
	w, h := g.size()
	c := art.NewCanvas(w, h)
	c.PaintMask(g.ring(4, 1), art.Linear(0, g.pad-4, 0, g.pad+g.h+4, art.Hex(0x6A6A78, 1), art.Hex(0x22222C, 1)))
	c.PaintMask(g.ring(4, 3), art.Linear(0, g.pad-4, 0, g.pad+g.h, white.Alpha(0.75), white.Alpha(0)))
	return img(c)
}

// barFill is a gradient fill from c0 (top) to c1 (bottom), with a soft
// highlight band and a bright leading edge.
func barFill(g barGeom, c0, c1 art.RGBA) image.Image {
	w, h := g.size()
	c := art.NewCanvas(w, h)
	m := g.mask(0)
	c.PaintMask(m, art.Linear(0, g.pad, 0, g.pad+g.h, c0, c1))
	// Highlight band in the upper third.
	band := art.ShapeMask(w, h, art.Para(g.pad+g.slant*0.65, g.pad+g.h*0.12, g.pad+g.l+g.slant*0.65, g.pad+g.h*0.38, g.slant*0.26))
	for i, v := range band {
		band[i] = math.Min(v, m[i])
	}
	c.LightMask(art.Blur(band, w, h, 1), white, 0.28)
	// Darker lower edge.
	low := art.ShapeMask(w, h, art.Para(g.pad, g.pad+g.h*0.82, g.pad+g.l, g.pad+g.h, g.slant*0.18))
	for i, v := range low {
		low[i] = math.Min(v, m[i])
	}
	c.PaintMask(low, art.Solid(ink.Alpha(0.22)))
	return img(c)
}

// ---------------------------------------------------------------- portrait

// faceGeom is the portrait frame: a square of side s with its bottom-right
// corner cut, drawn with its top-left corner at (pad, pad) (the axis).
type faceGeom struct{ s, cut, pad float64 }

func (f faceGeom) size() int { return int(math.Ceil(f.s + 2*f.pad)) }

func (f faceGeom) shape(d float64) []logo.Pt {
	p := f.pad
	return []logo.Pt{{X: p - d, Y: p - d}, {X: p + f.s + d, Y: p - d}, {X: p + f.s + d, Y: p + f.s - f.cut + d*0.4},
		{X: p + f.s - f.cut + d*0.4, Y: p + f.s + d}, {X: p - d, Y: p + f.s + d}}
}

func (f faceGeom) mask(d float64) []float64 {
	n := f.size()
	return art.ShapeMask(n, n, f.shape(d))
}

// faceBack is the dark panel the portrait is drawn on.
func faceBack(f faceGeom, s side) image.Image {
	n := f.size()
	c := art.NewCanvas(n, n)
	c.PaintMask(art.Blur(f.mask(6), n, n, 5), art.Solid(ink.Alpha(0.7)))
	c.PaintMask(f.mask(4), art.Linear(0, f.pad, 0, f.pad+f.s, art.Hex(0x3A3A46, 1), art.Hex(0x15151C, 1)))
	c.PaintMask(f.mask(0), art.Linear(0, f.pad, f.s, f.pad+f.s, art.Mix(panel, s.deep, 0.55), panel))
	return img(c)
}

// faceTop is the frame over the portrait: the border, a player-colored
// corner and an inner shade at the bottom, where portraits get cut off.
func faceTop(f faceGeom, s side) image.Image {
	n := f.size()
	c := art.NewCanvas(n, n)
	ring := art.ShapeMask(n, n, f.shape(4), art.Reversed(f.shape(0)))
	c.PaintMask(ring, art.Linear(0, f.pad-4, 0, f.pad+f.s+4, art.Hex(0x6A6A78, 1), art.Hex(0x22222C, 1)))
	// Accent along the top and left edges.
	p := f.pad
	acc := art.ShapeMask(n, n, []logo.Pt{{X: p - 4, Y: p - 4}, {X: p + f.s*0.55, Y: p - 4}, {X: p + f.s*0.55 - 6, Y: p + 2},
		{X: p + 2, Y: p + 2}, {X: p + 2, Y: p + f.s*0.55 - 6}, {X: p - 4, Y: p + f.s*0.55}})
	c.PaintMask(acc, art.Solid(s.col))
	c.LightMask(art.Blur(acc, n, n, 4), s.col, 0.5)
	// Shade at the bottom of the window.
	in := f.mask(0)
	for y := 0; y < n; y++ {
		t := art.Smooth((float64(y) - (p + f.s*0.72)) / (f.s * 0.28))
		for x := 0; x < n; x++ {
			if v := in[y*n+x]; v > 0 && t > 0 {
				c.Over(x, y, ink, v*t*0.55)
			}
		}
	}
	return img(c)
}

// ---------------------------------------------------------------- timer

// timerBack is the badge behind the round timer, centered on its axis: a
// slanted ink plate split red/blue along the bottom.
func timerBack(w, h, slant float64) (image.Image, int, int) {
	pad := 16.0
	cw, ch := int(w+slant+2*pad), int(h+2*pad)
	c := art.NewCanvas(cw, ch)
	plate := func(d float64) []logo.Pt {
		k := slant / h
		return art.Para(pad-d-k*d, pad-d, pad+w+d-k*d, pad+h+d, slant+2*k*d)
	}
	c.PaintMask(art.Blur(art.ShapeMask(cw, ch, plate(6)), cw, ch, 5), art.Solid(ink.Alpha(0.75)))
	c.PaintMask(art.ShapeMask(cw, ch, plate(4)), art.Linear(0, pad, 0, pad+h, art.Hex(0x6A6A78, 1), art.Hex(0x22222C, 1)))
	c.PaintMask(art.ShapeMask(cw, ch, plate(0)), art.Linear(0, pad, 0, pad+h, art.Hex(0x1C1C24, 1), art.Hex(0x09090D, 1)))
	// Red and blue halves of a stripe along the bottom.
	mid := pad + slant*0.1 + w/2
	for i, s := range sides {
		x0, x1 := pad, mid
		if i == 1 {
			x0, x1 = mid, pad+w
		}
		m := art.ShapeMask(cw, ch, art.Para(x0, pad+h-6, x1, pad+h, 6*slant/h))
		c.PaintMask(m, art.Solid(s.col))
		c.LightMask(art.Blur(m, cw, ch, 4), s.col, 0.6)
	}
	return img(c), cw / 2, ch / 2
}

// ---------------------------------------------------------------- win icons

// winIcon is a small slanted pip; perfect wins are gold.
func winIcon(col, hot art.RGBA) (image.Image, int, int) {
	const w, h, slant, pad = 18.0, 14.0, 6.0, 6.0
	cw, ch := int(w+slant+2*pad), int(h+2*pad)
	c := art.NewCanvas(cw, ch)
	m := art.ShapeMask(cw, ch, art.Para(pad, pad, pad+w, pad+h, slant))
	c.LightMask(art.Blur(m, cw, ch, 3), col, 0.7)
	c.PaintMask(m, art.Linear(0, pad, 0, pad+h, hot, col))
	c.PaintMask(art.ShapeMask(cw, ch, art.Para(pad, pad, pad+w, pad+h, slant), art.Reversed(art.Para(pad+2, pad+2, pad+w-2, pad+h-2, slant))), art.Solid(white.Alpha(0.6)))
	return img(c), cw / 2, ch / 2
}

// ---------------------------------------------------------------- announcements

// announce draws a round announcement: heavy italic text with a gradient
// fill, an ink outline and a colored glow, centered on its axis.
func announce(text string, size float64, top, bottom, glow art.RGBA) (image.Image, int, int) {
	f := art.Face(fontXBI, size)
	tw := art.TextWidth(f, text, size*0.02)
	pad := size * 0.6
	w, h := int(tw+2*pad), int(size*1.25+2*pad)
	base := pad + size*0.95
	m := art.TextMask(w, h, f, text, float64(w)/2, base, 0, size*0.02)
	c := art.NewCanvas(w, h)
	c.LightMask(art.Blur(m, w, h, int(size*0.18)), glow, 0.9)
	outline := art.ScaleMask(art.Blur(m, w, h, int(math.Max(2, size*0.05))), 6)
	c.PaintMask(outline, art.Solid(ink))
	c.PaintMask(m, art.Linear(0, base-size*0.75, 0, base, top, bottom))
	// A bright sheen across the upper half of the letters.
	for y := 0; y < h; y++ {
		t := 1 - art.Smooth(math.Abs(float64(y)-(base-size*0.55))/(size*0.12))
		if t <= 0 {
			continue
		}
		for x := 0; x < w; x++ {
			if v := m[y*w+x]; v > 0 {
				c.Light(x, y, white, v*t*0.35)
			}
		}
	}
	return img(c), w / 2, int(base - size*0.36)
}

// slash is the light streak behind announcements, centered on its axis.
func slash(w, h float64, col art.RGBA) (image.Image, int, int) {
	cw, ch := int(w), int(h)
	c := art.NewCanvas(cw, ch)
	cy := h / 2
	for y := 0; y < ch; y++ {
		for x := 0; x < cw; x++ {
			fx, fy := float64(x)+0.5, float64(y)+0.5
			// A thin bright core with a wide soft halo, fading at both ends.
			d := math.Abs(fy - cy + (fx-w/2)*0.04)
			along := 1 - math.Pow(math.Abs(fx-w/2)/(w/2), 2)
			if along <= 0 {
				continue
			}
			core := math.Exp(-d * d / (2 * 2.5 * 2.5))
			halo := math.Exp(-d*d/(2*h*h*0.03)) * 0.35
			c.Light(x, y, col, along*(core+halo))
		}
	}
	return c.Additive(), cw / 2, ch / 2
}
