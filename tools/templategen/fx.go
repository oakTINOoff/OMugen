package main

// Proto's effects (true-color, drawn with trans = add) and portraits.

import (
	"image"
	"image/color"
	"math"

	"github.com/oakTINOoff/OMugen/tools/internal/art"
)

var (
	fxHot  = art.Hex(0xFFF2EC, 1)
	fxRed  = art.Hex(0xFF3A2E, 1)
	fxDeep = art.Hex(0x9E0A14, 1)
)

// glowDisc adds a soft disc of light: a bright core fading to a halo.
func glowDisc(c *art.Canvas, cx, cy, core, halo float64, col art.RGBA, amt float64) {
	x0, x1 := int(cx-halo*2.5), int(cx+halo*2.5)
	y0, y1 := int(cy-halo*2.5), int(cy+halo*2.5)
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			d := math.Hypot(float64(x)+0.5-cx, float64(y)+0.5-cy)
			v := math.Exp(-d*d/(2*halo*halo)) * 0.55
			if d < core {
				v += 1
			} else {
				v += math.Exp(-(d-core)*(d-core)/(2*3*3)) * 0.8
			}
			c.Light(x, y, col, amt*v)
		}
	}
}

// orb is the Proto Blast projectile, frame i of 4 (a turning swirl).
func orb(i int) *image.NRGBA {
	const S = 128
	c := art.NewCanvas(S, S)
	glowDisc(c, S/2, S/2, 0, 30, fxDeep, 0.9)
	glowDisc(c, S/2, S/2, 16, 18, fxRed, 0.9)
	// Two swirl arms.
	for arm := 0; arm < 2; arm++ {
		for k := 0; k < 60; k++ {
			t := float64(k) / 60
			a := float64(i)*math.Pi/4 + float64(arm)*math.Pi + t*2.4
			r := 14 + t*30
			glowDisc(c, S/2+math.Cos(a)*r, S/2+math.Sin(a)*r, 0, 2.2, art.Mix(fxHot, fxRed, t), 0.35*(1-t))
		}
	}
	glowDisc(c, S/2, S/2, 9, 5, fxHot, 1)
	return c.Additive()
}

// burst is the projectile's impact, frame i of 5.
func burst(i int) *image.NRGBA {
	const S = 200
	c := art.NewCanvas(S, S)
	t := float64(i) / 4
	r := 16 + t*72
	fade := 1 - t*0.8
	for y := 0; y < S; y++ {
		for x := 0; x < S; x++ {
			d := math.Hypot(float64(x)+0.5-S/2, float64(y)+0.5-S/2)
			ringV := math.Exp(-(d-r)*(d-r)/(2*(4+6*t)*(4+6*t))) * fade
			core := math.Exp(-d*d/(2*(20-12*t)*(20-12*t))) * (1 - t)
			c.Light(x, y, art.Mix(fxHot, fxRed, t+0.2), ringV)
			c.Light(x, y, fxHot, core)
		}
	}
	return c.Additive()
}

// beam is a 1000 px segment of the Proto Overdrive beam, frame i of 3.
func beam(i int) *image.NRGBA {
	const W, H = 1000, 180
	c := art.NewCanvas(W, H)
	th := []float64{24, 30, 26}[i]
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			fx := float64(x) + 0.5
			wob := 3 * math.Sin(fx/37+float64(i)*2.1)
			d := math.Abs(float64(y) + 0.5 - H/2 - wob)
			start := math.Min(1, fx/40)
			// The halo fades to nothing before the sprite's top and bottom
			// edges, so the segment never shows as a lit rectangle.
			edge := math.Max(0, 1-d*d/((H/2-6)*(H/2-6)))
			core := math.Exp(-d*d/(2*th*th*0.25)) * start
			halo := math.Exp(-d*d/(2*(th*2)*(th*2))) * 0.6 * start * edge * edge
			c.Light(x, y, fxHot, core)
			c.Light(x, y, fxRed, halo)
		}
	}
	return c.Additive()
}

// flare is the light at the hands while the beam fires, frame i of 3.
func flare(i int) *image.NRGBA {
	const S = 220
	c := art.NewCanvas(S, S)
	glowDisc(c, S/2, S/2, 22+float64(i)*4, 36+float64(i)*6, fxRed, 1)
	glowDisc(c, S/2, S/2, 14, 10, fxHot, 1)
	for k := 0; k < 6; k++ { // light spikes
		a := float64(k)*math.Pi/3 + float64(i)*0.3
		for s := 0.0; s < 90; s += 1.5 {
			glowDisc(c, S/2+math.Cos(a)*s, S/2+math.Sin(a)*s, 0, 1.6, fxHot, 0.12*(1-s/90))
		}
	}
	return c.Additive()
}

// crop cuts a w x h window (top-left x, y) out of a paletted image.
func crop(src *image.Paletted, x, y, w, h int) *image.Paletted {
	dst := image.NewPaletted(image.Rect(0, 0, w, h), src.Palette)
	for yy := 0; yy < h; yy++ {
		for xx := 0; xx < w; xx++ {
			sx, sy := x+xx, y+yy
			if sx >= 0 && sy >= 0 && sx < src.Bounds().Dx() && sy < src.Bounds().Dy() {
				dst.Pix[yy*dst.Stride+xx] = src.Pix[sy*src.Stride+sx]
			}
		}
	}
	return dst
}

// portraits returns the small (9000,0: 100x100, a 25x25 MUGEN portrait at
// this localcoord) and big (9000,1) portraits.
func portraits(pal color.Palette) (*image.Paletted, *image.Paletted) {
	p := stance(0)
	p.Head = -8
	const small = 0.7
	f := render(p, small, pal)
	head := p.skeleton().head
	// head position inside the cropped frame
	hx := float64(f.axisX) + head.x*small
	hy := float64(f.axisY) + head.y*small
	sm := crop(f.img, int(hx)-46, int(hy)-34, 100, 100)

	big := with(p, func(q *Pose) { q.ArmN = Arm{60, 100}; q.ArmF = Arm{36, 112}; q.Head = -10 })
	return sm, render(big, 1.35, pal).img
}
