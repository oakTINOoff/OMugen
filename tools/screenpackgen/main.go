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

	"github.com/ikemen-engine/Ikemen-GO/src/sffwrite"
	"github.com/ikemen-engine/Ikemen-GO/tools/internal/logo"
)

const W, H = 1920, 1080

var (
	ink      = hex(0x07070A, 1)
	red      = hex(0xFF2A3A, 1)
	redHot   = hex(0xFF7A66, 1)
	redDeep  = hex(0x9E000B, 1)
	blue     = hex(0x2F8CFF, 1)
	blueHot  = hex(0x8CC4FF, 1)
	blueDeep = hex(0x0A3A8A, 1)
	white    = hex(0xF5F5F7, 1)
	gray     = hex(0xA0A0AC, 1)
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
	col     rgba
}

// background is a dark gradient with soft glows, fine diagonal hatching and
// a vignette.
func background(glows ...glow) *canvas {
	c := newCanvas(W, H)
	c.fill(linear(0, 0, 0, H, hex(0x0F0609, 1), hex(0x040406, 1)))
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			fx, fy := float64(x)+0.5, float64(y)+0.5
			for _, g := range glows {
				d := math.Hypot(fx-g.x, fy-g.y) / g.r
				if d < 1 {
					c.light(x, y, g.col, math.Pow(1-d, 2.2))
				}
			}
		}
	}
	hatch(c, 18, white.alpha(0.022))
	vignette(c, 0.6)
	return c
}

// hatch draws 1px diagonal lines every spacing pixels.
func hatch(c *canvas, spacing float64, col rgba) {
	sin, cos := math.Sincos(-math.Pi / 3)
	for y := 0; y < c.h; y++ {
		for x := 0; x < c.w; x++ {
			f := float64(x)*cos + float64(y)*sin
			u := math.Mod(f, spacing)
			if u < 0 {
				u += spacing
			}
			d := math.Min(u, spacing-u)
			if d < 1 {
				c.over(x, y, col, 1-d)
			}
		}
	}
}

func vignette(c *canvas, amt float64) {
	for y := 0; y < c.h; y++ {
		for x := 0; x < c.w; x++ {
			nx := (float64(x)/float64(c.w) - 0.5) * 2
			ny := (float64(y)/float64(c.h) - 0.5) * 2
			c.over(x, y, ink, amt*smooth((nx*nx*0.55+ny*ny*0.9-0.25)/1.2))
		}
	}
}

// streak draws a glowing line segment that fades at both ends, repeated
// every wrap pixels horizontally so the layer tiles.
func streak(c *canvas, x0, y0, x1, y1, th float64, col rgba, amt, wrap float64) {
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
				core := clamp01(th/2 + 0.5 - d)
				halo := math.Exp(-d*d/(2*th*th*4)) * 0.35
				c.light(x, y, col, amt*fade*math.Max(core, halo))
			}
		}
	}
}

func titleLayers() {
	add(0, 0, 0, 0, background(
		glow{1400, 380, 1150, hex(0xB0101F, 0.55)},
		glow{1650, 250, 420, hex(0xFF4A3A, 0.18)},
		glow{250, 1080, 700, hex(0x5A0A14, 0.35)},
	).image(true))

	// Light streaks (trans = add, scrolling).
	rng := rand.New(rand.NewSource(7))
	c := newCanvas(W, H)
	for i := 0; i < 22; i++ {
		x := rng.Float64() * W
		y := rng.Float64() * H
		l := 250 + rng.Float64()*800
		a := -0.21
		col := mix(red, white, rng.Float64()*0.3)
		streak(c, x, y, x+l*math.Cos(a), y+l*math.Sin(a), 0.7+rng.Float64()*1.6, col, 0.12+rng.Float64()*0.35, W)
	}
	add(0, 1, 0, 0, c.additive())

	// Embers (trans = add, rising, tiles both ways).
	const T = 512
	c = newCanvas(T, T)
	for i := 0; i < 34; i++ {
		cx, cy := rng.Float64()*T, rng.Float64()*T
		r := 0.7 + rng.Float64()*2.0
		col := mix(red, redHot, rng.Float64())
		if rng.Intn(6) == 0 {
			col = white
		}
		amt := 0.2 + rng.Float64()*0.5
		for y := int(cy - r*5); y <= int(cy+r*5); y++ {
			for x := int(cx - r*5); x <= int(cx+r*5); x++ {
				d := math.Hypot(float64(x)+0.5-cx, float64(y)+0.5-cy)
				v := math.Exp(-d*d/(2*r*r)) + 0.25*math.Exp(-d*d/(2*r*r*9))
				c.light((x%T+T)%T, (y%T+T)%T, col, amt*v)
			}
		}
	}
	add(0, 2, 0, 0, c.additive())

	// Light sweep: a tilted soft beam (trans = add).
	c = newCanvas(700, H)
	for y := 0; y < H; y++ {
		for x := 0; x < 700; x++ {
			cx := 350 + (float64(y)-H/2)*0.3
			d := (float64(x) + 0.5 - cx) / 90
			c.light(x, y, mix(red, white, 0.3), 0.16*math.Exp(-d*d/2))
		}
	}
	add(0, 3, 0, 0, c.additive())
}

// ---------------------------------------------------------------- logo

func logoSprites() {
	shapes, lw, lh := logo.WordmarkShapes()
	const target = 1000.0
	s := target / float64(lw)
	const pad = 70
	w := int(float64(lw)*s) + 2*pad
	h := int(float64(lh)*s) + 2*pad + 40
	c := newCanvas(w, h)
	var all [][]logo.Pt
	for _, sh := range shapes {
		all = append(all, translate(sh.Contours, pad, pad, s)...)
	}
	mask := shapeMask(w, h, all...)
	// Red halo and a drop shadow.
	c.paintMask(scaleMask(blur(mask, w, h, 18), 1.4), solid(red.alpha(0.55)))
	var shadow [][]logo.Pt
	for _, sh := range shapes {
		shadow = append(shadow, translate(sh.Contours, pad, pad+12, s)...)
	}
	c.paintMask(blur(shapeMask(w, h, shadow...), w, h, 6), solid(ink.alpha(0.8)))
	for _, sh := range shapes {
		m := shapeMask(w, h, translate(sh.Contours, pad, pad, s)...)
		_, _, minY, maxY := logo.Bounds(translate(sh.Contours, pad, pad, s))
		p := linear(0, minY, 0, maxY, white, hex(0xCFCFD8, 1))
		if sh.Fill == "red" {
			p = linear(0, minY, 0, maxY, rgba{1, 0.29, 0.25, 1}, redDeep)
		}
		c.paintMask(m, p)
	}
	// Tagline, right-aligned under the wordmark.
	f := face(fontSB, 26)
	_, maxX, _, maxY := logo.Bounds(all)
	c.paintMask(textMask(w, h, f, "64-BIT  MUGEN-COMPATIBLE  ENGINE", maxX-30, maxY+44, 1, 5), solid(gray.alpha(0.9)))
	add(10, 0, pad, pad, c.image(false))

	// App icon, 200 px.
	icon := logo.IconShapes()
	const is = 200
	c = newCanvas(is, is)
	for _, sh := range icon {
		m := shapeMask(is, is, translate(sh.Contours, 0, 0, is/256.0)...)
		switch sh.Fill {
		case "red":
			c.paintMask(m, linear(0, 0, 0, is, rgba{1, 0.23, 0.23, 1}, redDeep))
		case "white":
			c.paintMask(m, solid(white))
		default:
			c.paintMask(m, solid(hex(0x121216, 1)))
		}
	}
	add(10, 1, 0, 0, c.image(false))
}

// ---------------------------------------------------------------- title UI

func titleUI() {
	// Active menu item: a slanted bar that fades out to the right.
	const bw, bh = 760, 70
	c := newCanvas(bw, bh)
	bar := shapeMask(bw, bh, para(0, 8, 700, 64, 14))
	c.paintMask(scaleMask(blur(bar, bw, bh, 6), 0.6), linear(0, 0, 700, 0, red.alpha(0.5), red.alpha(0)))
	c.paintMask(bar, func(x, y float64) rgba {
		t := x / 700
		return mix(red.alpha(0.95), redDeep.alpha(0), smooth(t*1.1))
	})
	c.paintMask(shapeMask(bw, bh, para(0, 8, 9, 64, 14)), solid(redHot))
	c.paintMask(shapeMask(bw, bh, para(8, 8, 520, 10, 0.4)), linear(0, 0, 520, 0, white.alpha(0.35), white.alpha(0)))
	add(20, 0, 0, 36, c.image(false))

	// Footer strip.
	c = newCanvas(W, 56)
	c.fill(solid(ink.alpha(0.78)))
	for x := 0; x < W; x++ {
		t := math.Abs(float64(x)/W-0.5) * 2
		for y := 0; y < 2; y++ {
			c.over(x, y, red, 1-smooth(t))
		}
	}
	add(30, 0, 0, 0, c.image(false))
}

// ---------------------------------------------------------------- select screen

// panel is a translucent rounded card with a colored top accent.
func panel(w, h int, accent rgba, watermark string) *canvas {
	c := newCanvas(w, h)
	fw, fh := float64(w), float64(h)
	m := shapeMask(w, h, logo.RoundRect(0, 0, fw, fh, 20))
	c.paintMask(m, linear(0, 0, 0, fh, hex(0x181820, 0.82), hex(0x0A0A0E, 0.9)))
	c.paintMask(m, func(x, y float64) rgba {
		if y < 7 {
			return accent
		}
		return accent.alpha(0.22 * math.Exp(-(y-7)/60))
	})
	if watermark != "" {
		f := face(fontXB, 250)
		c.paintMask(textMask(w, h, f, watermark, fw-26, fh-40, 1, -6), solid(white.alpha(0.045)))
	}
	c.paintMask(shapeMask(w, h, ring(0, 0, fw, fh, 20, 1.5)...), solid(white.alpha(0.09)))
	return c
}

// plate is a slanted name bar; dir 1 fades to the right, -1 to the left.
func plate(w, h int, col, hot rgba, dir float64) *canvas {
	c := newCanvas(w, h)
	fw, fh := float64(w), float64(h)
	x0, x1 := 0.0, fw-24
	m := shapeMask(w, h, para(x0, 8, x1, fh-2, 16))
	c.paintMask(m, func(x, y float64) rgba {
		t := x / fw
		if dir < 0 {
			t = 1 - t
		}
		return mix(col.alpha(0.9), col.alpha(0), smooth(t*1.15))
	})
	if dir > 0 {
		c.paintMask(shapeMask(w, h, para(x0, 8, x0+8, fh-2, 16)), solid(hot))
	} else {
		c.paintMask(shapeMask(w, h, para(x1-8, 8, x1, fh-2, 16)), solid(hot))
	}
	return c
}

// cell is the empty character box, drawn for 100 px cells.
func cell() *canvas {
	c := newCanvas(100, 100)
	m := shapeMask(100, 100, logo.RoundRect(0, 0, 100, 100, 12))
	c.paintMask(m, linear(0, 0, 0, 100, hex(0x24242E, 1), hex(0x121218, 1)))
	c.paintMask(shapeMask(100, 100, ring(0, 0, 100, 100, 12, 2)...), solid(white.alpha(0.07)))
	c.paintMask(shapeMask(100, 100, logo.RoundRect(6, 3, 94, 5, 1)), solid(white.alpha(0.06)))
	return c
}

// cursor frames are 124x150 with the 100 px cell at (12, 38): the label pill
// sits above the cell.
func cursor(col, hot rgba, label string, right bool, glowAmt float64, done bool) *canvas {
	const w, h = 124, 150
	const cx0, cy0, cx1, cy1 = 12.0, 38.0, 112.0, 138.0
	c := newCanvas(w, h)
	if done {
		c.paintMask(shapeMask(w, h, logo.RoundRect(cx0, cy0, cx1, cy1, 12)), solid(col.alpha(0.3)))
	}
	frame := shapeMask(w, h, ring(cx0-3, cy0-3, cx1+3, cy1+3, 15, 8)...)
	c.paintMask(scaleMask(blur(frame, w, h, 6), 1.8), solid(col.alpha(0.9*glowAmt)))
	c.paintMask(frame, solid(col))
	// Bright corners.
	for _, k := range [][4]float64{{cx0 - 3, cy0 - 3, 1, 1}, {cx1 + 3, cy0 - 3, -1, 1}, {cx0 - 3, cy1 + 3, 1, -1}, {cx1 + 3, cy1 + 3, -1, -1}} {
		x, y, sx, sy := k[0], k[1], k[2], k[3]
		arm := []logo.Pt{{X: x, Y: y}, {X: x + sx*28, Y: y}, {X: x + sx*28, Y: y + sy*8}, {X: x + sx*8, Y: y + sy*8}, {X: x + sx*8, Y: y + sy*28}, {X: x, Y: y + sy*28}}
		if sx*sy < 0 {
			arm = reversed(arm)
		}
		c.paintMask(shapeMask(w, h, arm), solid(hot))
	}
	// Player label.
	px0 := cx0 - 2
	if right {
		px0 = cx1 + 2 - 50
	}
	c.paintMask(shapeMask(w, h, logo.RoundRect(px0, 4, px0+50, 32, 14)), solid(col))
	f := face(fontXB, 19)
	c.paintMask(textMask(w, h, f, label, px0+25, 25, 0, 0.5), solid(white))
	return c
}

func diamond(size float64, col rgba, outline bool) *canvas {
	n := int(size)
	c := newCanvas(n, n)
	s := size / 2
	shape := []logo.Pt{{X: s, Y: 1}, {X: size - 1, Y: s}, {X: s, Y: size - 1}, {X: 1, Y: s}}
	if outline {
		inner := []logo.Pt{{X: s, Y: 5}, {X: 5, Y: s}, {X: s, Y: size - 5}, {X: size - 5, Y: s}}
		c.paintMask(shapeMask(n, n, shape, inner), solid(col))
	} else {
		c.paintMask(scaleMask(blur(shapeMask(n, n, shape), n, n, 2), 1.3), solid(col.alpha(0.6)))
		c.paintMask(shapeMask(n, n, shape), solid(col))
	}
	return c
}

func questionMark(w, h int, size float64, baseline float64) *canvas {
	c := newCanvas(w, h)
	f := face(fontXB, size)
	m := textMask(w, h, f, "?", float64(w)/2, baseline, 0, 0)
	c.paintMask(scaleMask(blur(m, w, h, int(size/14)), 1.4), solid(red.alpha(0.6)))
	c.paintMask(m, linear(0, baseline-size*0.72, 0, baseline, rgba{1, 0.36, 0.3, 1}, redDeep))
	return c
}

func selectSprites() {
	add(100, 0, 0, 0, background(
		glow{960, -200, 1450, hex(0x9E0C1C, 0.42)},
		glow{0, 1080, 800, hex(0x5A0A14, 0.3)},
		glow{1920, 1080, 800, hex(0x0A2A5A, 0.3)},
	).image(true))

	// Header: fades down from the top, with a slanted red title block.
	c := newCanvas(W, 130)
	c.fill(linear(0, 0, 0, 130, ink.alpha(0.85), ink.alpha(0)))
	blk := shapeMask(W, 130, para(-40, 26, 660, 96, 22))
	c.paintMask(scaleMask(blur(blk, W, 130, 10), 0.8), solid(red.alpha(0.35)))
	c.paintMask(blk, linear(0, 0, 660, 0, red, redDeep))
	c.paintMask(shapeMask(W, 130, para(646, 26, 660, 96, 22)), solid(redHot))
	c.paintMask(textMask(W, 130, face(fontXB, 44), "CHARACTER SELECT", 64, 78, -1, 1.5), solid(white))
	add(100, 1, 0, 0, c.image(false))

	add(110, 0, 0, 0, panel(430, 820, red, "1P").image(false))
	add(110, 1, 0, 0, panel(430, 820, blue, "2P").image(false))
	c = newCanvas(1000, 830)
	m := shapeMask(1000, 830, logo.RoundRect(0, 0, 1000, 830, 20))
	c.paintMask(m, solid(hex(0x08080B, 0.6)))
	c.paintMask(shapeMask(1000, 830, ring(0, 0, 1000, 830, 20, 1.5)...), solid(white.alpha(0.07)))
	add(110, 2, 0, 0, c.image(false))
	add(110, 3, 0, 0, plate(430, 84, red, redHot, 1).image(false))
	add(110, 4, 430, 0, plate(430, 84, blue, blueHot, -1).image(false))
	c = newCanvas(1000, 64)
	c.paintMask(shapeMask(1000, 64, logo.RoundRect(0, 0, 1000, 64, 32)), solid(hex(0x0A0A0E, 0.82)))
	c.paintMask(shapeMask(1000, 64, ring(0, 0, 1000, 64, 32, 1.5)...), solid(white.alpha(0.08)))
	add(110, 5, 0, 0, c.image(false))

	// Cells (100 px; the screenpack scales them to the layout).
	add(120, 0, 0, 0, cell().image(false))
	c = cell()
	compose(c, questionMark(100, 100, 72, 76))
	add(120, 1, 0, 0, c.image(false))
	c = cell()
	for i := 0; i < 3; i++ {
		x := 34 + float64(i)*16
		c.paintMask(shapeMask(100, 100, logo.Circle(x, 50, 4, false)), solid(white.alpha(0.35)))
	}
	add(120, 2, 0, 0, c.image(false))

	// Cursors: 4-frame pulse, and the confirmed ("done") state.
	for i, g := range []float64{1, 0.8, 0.6, 0.8} {
		add(130, i, 12, 38, cursor(red, redHot, "1P", false, g, false).image(false))
		add(131, i, 12, 38, cursor(blue, blueHot, "2P", true, g, false).image(false))
	}
	add(132, 0, 12, 38, cursor(red, redHot, "1P", false, 1, true).image(false))
	add(133, 0, 12, 38, cursor(blue, blueHot, "2P", true, 1, true).image(false))

	// Team menu.
	for i, col := range []rgba{red, blue} {
		c = panel(420, 250, col, "")
		add(140+i, 0, 0, 0, c.image(false))
		c = plate(400, 50, col, mix(col, white, 0.5), 1-2*float64(i))
		add(140+i, 1, 0, 25, c.image(false))
		add(140+i, 2, 11, 11, diamond(22, col, false).image(false))
	}
	add(140, 3, 11, 11, diamond(22, white.alpha(0.3), true).image(false))

	// Big "random" and "loading" portraits.
	add(150, 0, 180, 180, questionMark(360, 360, 300, 290).image(false))
	c = newCanvas(360, 80)
	c.paintMask(textMask(360, 80, face(fontSB, 26), "LOADING", 180, 50, 0, 8), solid(white.alpha(0.5)))
	add(150, 1, 180, 40, c.image(false))
}

// compose draws src over dst (same size).
func compose(dst, src *canvas) {
	for i := 0; i < len(dst.p); i += 4 {
		a := src.p[i+3]
		for k := 0; k < 4; k++ {
			dst.p[i+k] = src.p[i+k] + dst.p[i+k]*(1-a)
		}
	}
}

// ---------------------------------------------------------------- versus

func versusSprites() {
	c := background(
		glow{380, 540, 1150, hex(0xB0101F, 0.5)},
		glow{1540, 540, 1150, hex(0x1050C0, 0.5)},
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
			c.over(x, y, side.alpha(0.07), 1)
			c.light(x, y, mix(white, side, 0.3), 0.6*math.Exp(-d*d/(2*2.5*2.5)))
			c.light(x, y, side, 0.12*math.Exp(-d*d/(2*45*45)))
		}
	}
	add(200, 0, 0, 0, c.image(true))

	const w, h = 620, 400
	c = newCanvas(w, h)
	f := face(fontXBI, 290)
	m := textMask(w, h, f, "VS", w/2, 305, 0, -8)
	halo := scaleMask(blur(m, w, h, 20), 1.5)
	c.paintMask(halo, func(x, y float64) rgba { return mix(red, blue, smooth((x-200)/220)).alpha(0.7) })
	var shadow []float64 = blur(textMask(w, h, f, "VS", w/2+6, 317, 0, -8), w, h, 4)
	c.paintMask(shadow, solid(ink.alpha(0.8)))
	c.paintMask(m, linear(0, 100, 0, 305, white, hex(0xC8C8D2, 1)))
	add(200, 1, w/2, h/2, c.image(false))
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
	check(sffwrite.Write(&b, out))
	path := filepath.Join("screenpack", "omugen", "system.sff")
	check(os.MkdirAll(filepath.Dir(path), 0o755))
	check(os.WriteFile(path, b.Bytes(), 0o644))
	fmt.Printf("wrote %s: %d sprites, %.1f MB\n", path, len(out), float64(b.Len())/1e6)

	// Preview PNGs for review (not packed).
	if os.Getenv("SCREENPACKGEN_PREVIEW") != "" {
		dir := os.Getenv("SCREENPACKGEN_PREVIEW")
		check(os.MkdirAll(dir, 0o755))
		for _, s := range sprites {
			writePNG(filepath.Join(dir, fmt.Sprintf("%d_%d.png", s.group, s.number)), s.img)
		}
	}
}
