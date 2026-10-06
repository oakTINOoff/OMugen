// screenpackgen draws the sprites of the OMugen screenpack and writes
// screenpack/omugen/system.sff. Everything is generated from code (shapes,
// gradients, glows and Open Sans text), so the art has no third-party
// license.
//
// Run from the repo root: go run ./tools/screenpackgen
//
// Sprite groups (see screenpack/omugen/system.def):
//
//	0-30     title screen: background, light layers, logo, menu bar, footer
//	100-150  select screen: background, header, panels, cells, cursors, team menu
//	200      versus screen
package main

import (
	"bytes"
	"fmt"
	"image"
	"math"
	"math/rand"
	"os"
	"path/filepath"

	"github.com/oakTINOoff/OMugen/src/sffwrite"
	"github.com/oakTINOoff/OMugen/tools/internal/art"
	"github.com/oakTINOoff/OMugen/tools/internal/logo"
)

const W, H = 1920, 1080

var (
	ink      = art.Hex(0x07070A, 1)
	red      = art.Hex(0xFF2A3A, 1)
	redHot   = art.Hex(0xFF7A66, 1)
	redDeep  = art.Hex(0x9E000B, 1)
	blue     = art.Hex(0x2F8CFF, 1)
	blueHot  = art.Hex(0x8CC4FF, 1)
	blueDeep = art.Hex(0x0A3A8A, 1)
	white    = art.Hex(0xF5F5F7, 1)
	gray     = art.Hex(0xA0A0AC, 1)
)

const (
	fontXB  = "font/Open_Sans/OpenSans-ExtraBold.ttf"
	fontXBI = "font/Open_Sans/OpenSans-ExtraBoldItalic.ttf"
	fontSB  = "font/Open_Sans/OpenSans-SemiBold.ttf"
)

type sprite struct {
	group, number int
	axisX, axisY  int
	img           image.Image
}

var sprites []sprite

func add(group, number, ax, ay int, img image.Image) {
	sprites = append(sprites, sprite{group, number, ax, ay, img})
}

// ---------------------------------------------------------------- backgrounds

type glow struct {
	x, y, r float64
	col     art.RGBA
}

// background is a dark gradient with soft glows, fine diagonal hatching and
// a vignette.
func background(glows ...glow) *art.Canvas {
	c := art.NewCanvas(W, H)
	c.Fill(art.Linear(0, 0, 0, H, art.Hex(0x0F0609, 1), art.Hex(0x040406, 1)))
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			fx, fy := float64(x)+0.5, float64(y)+0.5
			for _, g := range glows {
				d := math.Hypot(fx-g.x, fy-g.y) / g.r
				if d < 1 {
					c.Light(x, y, g.col, math.Pow(1-d, 2.2))
				}
			}
		}
	}
	hatch(c, 18, white.Alpha(0.022))
	vignette(c, 0.6)
	return c
}

// hatch draws 1px diagonal lines every spacing pixels.
func hatch(c *art.Canvas, spacing float64, col art.RGBA) {
	sin, cos := math.Sincos(-math.Pi / 3)
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			f := float64(x)*cos + float64(y)*sin
			u := math.Mod(f, spacing)
			if u < 0 {
				u += spacing
			}
			d := math.Min(u, spacing-u)
			if d < 1 {
				c.Over(x, y, col, 1-d)
			}
		}
	}
}

func vignette(c *art.Canvas, amt float64) {
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			nx := (float64(x)/float64(c.W) - 0.5) * 2
			ny := (float64(y)/float64(c.H) - 0.5) * 2
			c.Over(x, y, ink, amt*art.Smooth((nx*nx*0.55+ny*ny*0.9-0.25)/1.2))
		}
	}
}

// streak draws a glowing line segment that fades at both ends, repeated
// every wrap pixels horizontally so the layer tiles.
func streak(c *art.Canvas, x0, y0, x1, y1, th float64, col art.RGBA, amt, wrap float64) {
	dx, dy := x1-x0, y1-y0
	l2 := dx*dx + dy*dy
	for _, off := range []float64{-wrap, 0, wrap} {
		ax, bx := x0+off, x1+off
		minX := int(math.Floor(math.Min(ax, bx) - th*4))
		maxX := int(math.Ceil(math.Max(ax, bx) + th*4))
		minY := int(math.Floor(math.Min(y0, y1) - th*4))
		maxY := int(math.Ceil(math.Max(y0, y1) + th*4))
		for y := minY; y <= maxY; y++ {
			for x := minX; x <= maxX; x++ {
				px, py := float64(x)+0.5-ax, float64(y)+0.5-y0
				t := (px*dx + py*dy) / l2
				if t < 0 || t > 1 {
					continue
				}
				d := math.Abs(px*dy-py*dx) / math.Sqrt(l2)
				fade := math.Sin(t * math.Pi)
				core := art.Clamp01(th/2 + 0.5 - d)
				halo := math.Exp(-d*d/(2*th*th*4)) * 0.35
				c.Light(x, y, col, amt*fade*math.Max(core, halo))
			}
		}
	}
}

func titleLayers() {
	add(0, 0, 0, 0, background(
		glow{1400, 380, 1150, art.Hex(0xB0101F, 0.55)},
		glow{1650, 250, 420, art.Hex(0xFF4A3A, 0.18)},
		glow{250, 1080, 700, art.Hex(0x5A0A14, 0.35)},
	).Image(true))

	// Light streaks (trans = add, scrolling).
	rng := rand.New(rand.NewSource(7))
	c := art.NewCanvas(W, H)
	for i := 0; i < 22; i++ {
		x := rng.Float64() * W
		y := rng.Float64() * H
		l := 250 + rng.Float64()*800
		a := -0.21
		col := art.Mix(red, white, rng.Float64()*0.3)
		streak(c, x, y, x+l*math.Cos(a), y+l*math.Sin(a), 0.7+rng.Float64()*1.6, col, 0.12+rng.Float64()*0.35, W)
	}
	add(0, 1, 0, 0, c.Additive())

	// Embers (trans = add, rising, tiles both ways).
	const T = 512
	c = art.NewCanvas(T, T)
	for i := 0; i < 34; i++ {
		cx, cy := rng.Float64()*T, rng.Float64()*T
		r := 0.7 + rng.Float64()*2.0
		col := art.Mix(red, redHot, rng.Float64())
		if rng.Intn(6) == 0 {
			col = white
		}
		amt := 0.2 + rng.Float64()*0.5
		for y := int(cy - r*5); y <= int(cy+r*5); y++ {
			for x := int(cx - r*5); x <= int(cx+r*5); x++ {
				d := math.Hypot(float64(x)+0.5-cx, float64(y)+0.5-cy)
				v := math.Exp(-d*d/(2*r*r)) + 0.25*math.Exp(-d*d/(2*r*r*9))
				c.Light((x%T+T)%T, (y%T+T)%T, col, amt*v)
			}
		}
	}
	add(0, 2, 0, 0, c.Additive())

	// Light sweep: a tilted soft beam (trans = add).
	c = art.NewCanvas(700, H)
	for y := 0; y < H; y++ {
		for x := 0; x < 700; x++ {
			cx := 350 + (float64(y)-H/2)*0.3
			d := (float64(x) + 0.5 - cx) / 90
			c.Light(x, y, art.Mix(red, white, 0.3), 0.16*math.Exp(-d*d/2))
		}
	}
	add(0, 3, 0, 0, c.Additive())
}

// ---------------------------------------------------------------- logo

func logoSprites() {
	shapes, lw, lh := logo.WordmarkShapes()
	const target = 1000.0
	s := target / float64(lw)
	const pad = 70
	w := int(float64(lw)*s) + 2*pad
	h := int(float64(lh)*s) + 2*pad + 40
	c := art.NewCanvas(w, h)
	var all [][]logo.Pt
	for _, sh := range shapes {
		all = append(all, art.Translate(sh.Contours, pad, pad, s)...)
	}
	mask := art.ShapeMask(w, h, all...)
	// Red halo and a drop shadow.
	c.PaintMask(art.ScaleMask(art.Blur(mask, w, h, 18), 1.4), art.Solid(red.Alpha(0.55)))
	var shadow [][]logo.Pt
	for _, sh := range shapes {
		shadow = append(shadow, art.Translate(sh.Contours, pad, pad+12, s)...)
	}
	c.PaintMask(art.Blur(art.ShapeMask(w, h, shadow...), w, h, 6), art.Solid(ink.Alpha(0.8)))
	for _, sh := range shapes {
		m := art.ShapeMask(w, h, art.Translate(sh.Contours, pad, pad, s)...)
		_, _, minY, maxY := logo.Bounds(art.Translate(sh.Contours, pad, pad, s))
		p := art.Linear(0, minY, 0, maxY, white, art.Hex(0xCFCFD8, 1))
		if sh.Fill == "red" {
			p = art.Linear(0, minY, 0, maxY, art.RGBA{R: 1, G: 0.29, B: 0.25, A: 1}, redDeep)
		}
		c.PaintMask(m, p)
	}
	// Tagline, right-aligned under the wordmark.
	f := art.Face(fontSB, 26)
	_, maxX, _, maxY := logo.Bounds(all)
	c.PaintMask(art.TextMask(w, h, f, "64-BIT  MUGEN-COMPATIBLE  ENGINE", maxX-30, maxY+44, 1, 5), art.Solid(gray.Alpha(0.9)))
	add(10, 0, pad, pad, c.Image(false))

	// App icon, 200 px.
	icon := logo.IconShapes()
	const is = 200
	c = art.NewCanvas(is, is)
	for _, sh := range icon {
		m := art.ShapeMask(is, is, art.Translate(sh.Contours, 0, 0, is/256.0)...)
		switch sh.Fill {
		case "red":
			c.PaintMask(m, art.Linear(0, 0, 0, is, art.RGBA{R: 1, G: 0.23, B: 0.23, A: 1}, redDeep))
		case "white":
			c.PaintMask(m, art.Solid(white))
		default:
			c.PaintMask(m, art.Solid(art.Hex(0x121216, 1)))
		}
	}
	add(10, 1, 0, 0, c.Image(false))
}

// ---------------------------------------------------------------- title UI

func titleUI() {
	// Active menu item: a slanted bar that fades out to the right.
	const bw, bh = 760, 70
	c := art.NewCanvas(bw, bh)
	bar := art.ShapeMask(bw, bh, art.Para(0, 8, 700, 64, 14))
	c.PaintMask(art.ScaleMask(art.Blur(bar, bw, bh, 6), 0.6), art.Linear(0, 0, 700, 0, red.Alpha(0.5), red.Alpha(0)))
	c.PaintMask(bar, func(x, y float64) art.RGBA {
		t := x / 700
		return art.Mix(red.Alpha(0.95), redDeep.Alpha(0), art.Smooth(t*1.1))
	})
	c.PaintMask(art.ShapeMask(bw, bh, art.Para(0, 8, 9, 64, 14)), art.Solid(redHot))
	c.PaintMask(art.ShapeMask(bw, bh, art.Para(8, 8, 520, 10, 0.4)), art.Linear(0, 0, 520, 0, white.Alpha(0.35), white.Alpha(0)))
	add(20, 0, 0, 36, c.Image(false))

	// Footer strip.
	c = art.NewCanvas(W, 56)
	c.Fill(art.Solid(ink.Alpha(0.78)))
	for x := 0; x < W; x++ {
		t := math.Abs(float64(x)/W-0.5) * 2
		for y := 0; y < 2; y++ {
			c.Over(x, y, red, 1-art.Smooth(t))
		}
	}
	add(30, 0, 0, 0, c.Image(false))
}

// ---------------------------------------------------------------- select screen

// panel is a translucent rounded card with a colored top accent.
func panel(w, h int, accent art.RGBA, watermark string) *art.Canvas {
	c := art.NewCanvas(w, h)
	fw, fh := float64(w), float64(h)
	m := art.ShapeMask(w, h, logo.RoundRect(0, 0, fw, fh, 20))
	c.PaintMask(m, art.Linear(0, 0, 0, fh, art.Hex(0x181820, 0.82), art.Hex(0x0A0A0E, 0.9)))
	c.PaintMask(m, func(x, y float64) art.RGBA {
		if y < 7 {
			return accent
		}
		return accent.Alpha(0.22 * math.Exp(-(y-7)/60))
	})
	if watermark != "" {
		f := art.Face(fontXB, 250)
		c.PaintMask(art.TextMask(w, h, f, watermark, fw-26, fh-40, 1, -6), art.Solid(white.Alpha(0.045)))
	}
	c.PaintMask(art.ShapeMask(w, h, art.Ring(0, 0, fw, fh, 20, 1.5)...), art.Solid(white.Alpha(0.09)))
	return c
}

// plate is a slanted name bar; dir 1 fades to the right, -1 to the left.
func plate(w, h int, col, hot art.RGBA, dir float64) *art.Canvas {
	c := art.NewCanvas(w, h)
	fw, fh := float64(w), float64(h)
	x0, x1 := 0.0, fw-24
	m := art.ShapeMask(w, h, art.Para(x0, 8, x1, fh-2, 16))
	c.PaintMask(m, func(x, y float64) art.RGBA {
		t := x / fw
		if dir < 0 {
			t = 1 - t
		}
		return art.Mix(col.Alpha(0.9), col.Alpha(0), art.Smooth(t*1.15))
	})
	if dir > 0 {
		c.PaintMask(art.ShapeMask(w, h, art.Para(x0, 8, x0+8, fh-2, 16)), art.Solid(hot))
	} else {
		c.PaintMask(art.ShapeMask(w, h, art.Para(x1-8, 8, x1, fh-2, 16)), art.Solid(hot))
	}
	return c
}

// cell is the empty character box, drawn for 100 px cells.
func cell() *art.Canvas {
	c := art.NewCanvas(100, 100)
	m := art.ShapeMask(100, 100, logo.RoundRect(0, 0, 100, 100, 12))
	c.PaintMask(m, art.Linear(0, 0, 0, 100, art.Hex(0x24242E, 1), art.Hex(0x121218, 1)))
	c.PaintMask(art.ShapeMask(100, 100, art.Ring(0, 0, 100, 100, 12, 2)...), art.Solid(white.Alpha(0.07)))
	c.PaintMask(art.ShapeMask(100, 100, logo.RoundRect(6, 3, 94, 5, 1)), art.Solid(white.Alpha(0.06)))
	return c
}

// cursor frames are 124x150 with the 100 px cell at (12, 38): the label pill
// sits above the cell.
func cursor(col, hot art.RGBA, label string, right bool, glowAmt float64, done bool) *art.Canvas {
	const w, h = 124, 150
	const cx0, cy0, cx1, cy1 = 12.0, 38.0, 112.0, 138.0
	c := art.NewCanvas(w, h)
	if done {
		c.PaintMask(art.ShapeMask(w, h, logo.RoundRect(cx0, cy0, cx1, cy1, 12)), art.Solid(col.Alpha(0.3)))
	}
	frame := art.ShapeMask(w, h, art.Ring(cx0-3, cy0-3, cx1+3, cy1+3, 15, 8)...)
	c.PaintMask(art.ScaleMask(art.Blur(frame, w, h, 6), 1.8), art.Solid(col.Alpha(0.9*glowAmt)))
	c.PaintMask(frame, art.Solid(col))
	// Bright corners.
	for _, k := range [][4]float64{{cx0 - 3, cy0 - 3, 1, 1}, {cx1 + 3, cy0 - 3, -1, 1}, {cx0 - 3, cy1 + 3, 1, -1}, {cx1 + 3, cy1 + 3, -1, -1}} {
		x, y, sx, sy := k[0], k[1], k[2], k[3]
		arm := []logo.Pt{{X: x, Y: y}, {X: x + sx*28, Y: y}, {X: x + sx*28, Y: y + sy*8}, {X: x + sx*8, Y: y + sy*8}, {X: x + sx*8, Y: y + sy*28}, {X: x, Y: y + sy*28}}
		if sx*sy < 0 {
			arm = art.Reversed(arm)
		}
		c.PaintMask(art.ShapeMask(w, h, arm), art.Solid(hot))
	}
	// Player label.
	px0 := cx0 - 2
	if right {
		px0 = cx1 + 2 - 50
	}
	c.PaintMask(art.ShapeMask(w, h, logo.RoundRect(px0, 4, px0+50, 32, 14)), art.Solid(col))
	f := art.Face(fontXB, 19)
	c.PaintMask(art.TextMask(w, h, f, label, px0+25, 25, 0, 0.5), art.Solid(white))
	return c
}

func diamond(size float64, col art.RGBA, outline bool) *art.Canvas {
	n := int(size)
	c := art.NewCanvas(n, n)
	s := size / 2
	shape := []logo.Pt{{X: s, Y: 1}, {X: size - 1, Y: s}, {X: s, Y: size - 1}, {X: 1, Y: s}}
	if outline {
		inner := []logo.Pt{{X: s, Y: 5}, {X: 5, Y: s}, {X: s, Y: size - 5}, {X: size - 5, Y: s}}
		c.PaintMask(art.ShapeMask(n, n, shape, inner), art.Solid(col))
	} else {
		c.PaintMask(art.ScaleMask(art.Blur(art.ShapeMask(n, n, shape), n, n, 2), 1.3), art.Solid(col.Alpha(0.6)))
		c.PaintMask(art.ShapeMask(n, n, shape), art.Solid(col))
	}
	return c
}

func questionMark(w, h int, size float64, baseline float64) *art.Canvas {
	c := art.NewCanvas(w, h)
	f := art.Face(fontXB, size)
	m := art.TextMask(w, h, f, "?", float64(w)/2, baseline, 0, 0)
	c.PaintMask(art.ScaleMask(art.Blur(m, w, h, int(size/14)), 1.4), art.Solid(red.Alpha(0.6)))
	c.PaintMask(m, art.Linear(0, baseline-size*0.72, 0, baseline, art.RGBA{R: 1, G: 0.36, B: 0.3, A: 1}, redDeep))
	return c
}

func selectSprites() {
	add(100, 0, 0, 0, background(
		glow{960, -200, 1450, art.Hex(0x9E0C1C, 0.42)},
		glow{0, 1080, 800, art.Hex(0x5A0A14, 0.3)},
		glow{1920, 1080, 800, art.Hex(0x0A2A5A, 0.3)},
	).Image(true))

	// Header: fades down from the top, with a slanted red title block.
	c := art.NewCanvas(W, 130)
	c.Fill(art.Linear(0, 0, 0, 130, ink.Alpha(0.85), ink.Alpha(0)))
	blk := art.ShapeMask(W, 130, art.Para(-40, 26, 660, 96, 22))
	c.PaintMask(art.ScaleMask(art.Blur(blk, W, 130, 10), 0.8), art.Solid(red.Alpha(0.35)))
	c.PaintMask(blk, art.Linear(0, 0, 660, 0, red, redDeep))
	c.PaintMask(art.ShapeMask(W, 130, art.Para(646, 26, 660, 96, 22)), art.Solid(redHot))
	c.PaintMask(art.TextMask(W, 130, art.Face(fontXB, 44), "CHARACTER SELECT", 64, 78, -1, 1.5), art.Solid(white))
	add(100, 1, 0, 0, c.Image(false))

	add(110, 0, 0, 0, panel(430, 820, red, "1P").Image(false))
	add(110, 1, 0, 0, panel(430, 820, blue, "2P").Image(false))
	c = art.NewCanvas(1000, 830)
	m := art.ShapeMask(1000, 830, logo.RoundRect(0, 0, 1000, 830, 20))
	c.PaintMask(m, art.Solid(art.Hex(0x08080B, 0.6)))
	c.PaintMask(art.ShapeMask(1000, 830, art.Ring(0, 0, 1000, 830, 20, 1.5)...), art.Solid(white.Alpha(0.07)))
	add(110, 2, 0, 0, c.Image(false))
	add(110, 3, 0, 0, plate(430, 84, red, redHot, 1).Image(false))
	add(110, 4, 430, 0, plate(430, 84, blue, blueHot, -1).Image(false))
	c = art.NewCanvas(1000, 64)
	c.PaintMask(art.ShapeMask(1000, 64, logo.RoundRect(0, 0, 1000, 64, 32)), art.Solid(art.Hex(0x0A0A0E, 0.82)))
	c.PaintMask(art.ShapeMask(1000, 64, art.Ring(0, 0, 1000, 64, 32, 1.5)...), art.Solid(white.Alpha(0.08)))
	add(110, 5, 0, 0, c.Image(false))

	// Cells (100 px; the screenpack scales them to the layout).
	add(120, 0, 0, 0, cell().Image(false))
	c = cell()
	compose(c, questionMark(100, 100, 72, 76))
	add(120, 1, 0, 0, c.Image(false))
	c = cell()
	for i := 0; i < 3; i++ {
		x := 34 + float64(i)*16
		c.PaintMask(art.ShapeMask(100, 100, logo.Circle(x, 50, 4, false)), art.Solid(white.Alpha(0.35)))
	}
	add(120, 2, 0, 0, c.Image(false))

	// Cursors: 4-frame pulse, and the confirmed ("done") state.
	for i, g := range []float64{1, 0.8, 0.6, 0.8} {
		add(130, i, 12, 38, cursor(red, redHot, "1P", false, g, false).Image(false))
		add(131, i, 12, 38, cursor(blue, blueHot, "2P", true, g, false).Image(false))
	}
	add(132, 0, 12, 38, cursor(red, redHot, "1P", false, 1, true).Image(false))
	add(133, 0, 12, 38, cursor(blue, blueHot, "2P", true, 1, true).Image(false))

	// Team menu.
	for i, col := range []art.RGBA{red, blue} {
		c = panel(420, 250, col, "")
		add(140+i, 0, 0, 0, c.Image(false))
		c = plate(400, 50, col, art.Mix(col, white, 0.5), 1-2*float64(i))
		add(140+i, 1, 0, 25, c.Image(false))
		add(140+i, 2, 11, 11, diamond(22, col, false).Image(false))
	}
	add(140, 3, 11, 11, diamond(22, white.Alpha(0.3), true).Image(false))

	// Big "random" and "loading" portraits.
	add(150, 0, 180, 180, questionMark(360, 360, 300, 290).Image(false))
	c = art.NewCanvas(360, 80)
	c.PaintMask(art.TextMask(360, 80, art.Face(fontSB, 26), "LOADING", 180, 50, 0, 8), art.Solid(white.Alpha(0.5)))
	add(150, 1, 180, 40, c.Image(false))
}

// compose draws src over dst (same size).
func compose(dst, src *art.Canvas) {
	for i := 0; i < len(dst.P); i += 4 {
		a := src.P[i+3]
		for k := 0; k < 4; k++ {
			dst.P[i+k] = src.P[i+k] + dst.P[i+k]*(1-a)
		}
	}
}

// ---------------------------------------------------------------- versus

func versusSprites() {
	c := background(
		glow{380, 540, 1150, art.Hex(0xB0101F, 0.5)},
		glow{1540, 540, 1150, art.Hex(0x1050C0, 0.5)},
	)
	// Split the two halves along a diagonal seam, tinting each side.
	seam := func(y float64) float64 { return 1010 - y/H*100 }
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			d := float64(x) + 0.5 - seam(float64(y)+0.5)
			side := red
			if d > 0 {
				side = blue
			}
			c.Over(x, y, side.Alpha(0.07), 1)
			c.Light(x, y, art.Mix(white, side, 0.3), 0.6*math.Exp(-d*d/(2*2.5*2.5)))
			c.Light(x, y, side, 0.12*math.Exp(-d*d/(2*45*45)))
		}
	}
	add(200, 0, 0, 0, c.Image(true))

	const w, h = 620, 400
	c = art.NewCanvas(w, h)
	f := art.Face(fontXBI, 290)
	m := art.TextMask(w, h, f, "VS", w/2, 305, 0, -8)
	halo := art.ScaleMask(art.Blur(m, w, h, 20), 1.5)
	c.PaintMask(halo, func(x, y float64) art.RGBA { return art.Mix(red, blue, art.Smooth((x-200)/220)).Alpha(0.7) })
	var shadow []float64 = art.Blur(art.TextMask(w, h, f, "VS", w/2+6, 317, 0, -8), w, h, 4)
	c.PaintMask(shadow, art.Solid(ink.Alpha(0.8)))
	c.PaintMask(m, art.Linear(0, 100, 0, 305, white, art.Hex(0xC8C8D2, 1)))
	add(200, 1, w/2, h/2, c.Image(false))
}

// ---------------------------------------------------------------- main

func main() {
	titleLayers()
	logoSprites()
	titleUI()
	selectSprites()
	versusSprites()

	var out []sffwrite.Sprite
	for _, s := range sprites {
		out = append(out, sffwrite.Sprite{
			Group: uint16(s.group), Number: uint16(s.number),
			AxisX: int16(s.axisX), AxisY: int16(s.axisY), Image: s.img,
		})
	}
	var b bytes.Buffer
	art.Check(sffwrite.Write(&b, out))
	path := filepath.Join("screenpack", "omugen", "system.sff")
	art.Check(os.MkdirAll(filepath.Dir(path), 0o755))
	art.Check(os.WriteFile(path, b.Bytes(), 0o644))
	fmt.Printf("wrote %s: %d sprites, %.1f MB\n", path, len(out), float64(b.Len())/1e6)

	// Preview PNGs for review (not packed).
	if os.Getenv("SCREENPACKGEN_PREVIEW") != "" {
		dir := os.Getenv("SCREENPACKGEN_PREVIEW")
		art.Check(os.MkdirAll(dir, 0o755))
		for _, s := range sprites {
			art.WritePNG(filepath.Join(dir, fmt.Sprintf("%d_%d.png", s.group, s.number)), s.img)
		}
	}
}
