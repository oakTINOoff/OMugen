package main

// Proto Arena: a rooftop dojo at night, in 1280x720, made of parallax
// layers (far to near): sky with the moon, misty mountains, the city, the
// dojo's pillars and railing, lanterns, and the wooden floor.

import (
	"image"
	"math"
	"math/rand"

	"github.com/oakTINOoff/OMugen/tools/internal/art"
	"github.com/oakTINOoff/OMugen/tools/internal/logo"
)

type stageSprite struct {
	g, n   int
	ax, ay int
	img    image.Image
}

var hx = func(v uint32) art.RGBA { return art.Hex(v, 1) }

// rect paints a filled rectangle.
func rect(c *art.Canvas, x0, y0, x1, y1 int, p art.Paint) {
	for y := max(0, y0); y < min(c.H, y1); y++ {
		for x := max(0, x0); x < min(c.W, x1); x++ {
			c.Over(x, y, p(float64(x)+0.5, float64(y)+0.5), 1)
		}
	}
}

// glow adds a soft round light.
func glow(c *art.Canvas, cx, cy, r float64, col art.RGBA, amt float64) {
	for y := int(cy - 3*r); y < int(cy+3*r); y++ {
		for x := int(cx - 3*r); x < int(cx+3*r); x++ {
			d := math.Hypot(float64(x)+0.5-cx, float64(y)+0.5-cy)
			c.Light(x, y, col, amt*math.Exp(-d*d/(2*r*r)))
		}
	}
}

// ridge is a mountain silhouette: heights from a few sines and noise.
func ridge(w int, base, amp float64, seed int64) []float64 {
	r := rand.New(rand.NewSource(seed))
	ph := [4]float64{r.Float64() * 6, r.Float64() * 6, r.Float64() * 6, r.Float64() * 6}
	hs := make([]float64, w)
	n := 0.0
	for x := range hs {
		n = n*0.92 + (r.Float64()-0.5)*0.9
		t := float64(x)
		hs[x] = base + amp*(0.55*math.Sin(t/310+ph[0])+0.3*math.Sin(t/131+ph[1])+0.15*math.Sin(t/47+ph[2])) + n*3
	}
	return hs
}

func stageSprites() []stageSprite {
	var out []stageSprite
	rng := rand.New(rand.NewSource(11))

	// 0,0 Sky (delta 0.08): indigo to crimson, stars, the moon with clouds.
	const SW, SH = 1500, 720
	c := art.NewCanvas(SW, SH)
	top, mid, hor := hx(0x07071A), hx(0x231030), hx(0x6E1A2C)
	for y := 0; y < SH; y++ {
		t := float64(y) / 600
		col := art.Mix(top, mid, t/0.55)
		if t > 0.55 {
			col = art.Mix(mid, hor, (t-0.55)/0.45)
		}
		for x := 0; x < SW; x++ {
			c.Over(x, y, col, 1)
		}
	}
	for i := 0; i < 260; i++ {
		x, y := rng.Intn(SW), int(math.Pow(rng.Float64(), 1.6)*460)
		b := (0.25 + rng.Float64()*0.7) * (1 - float64(y)/520)
		c.Light(x, y, hx(0xFFF0F4), b)
		if rng.Intn(9) == 0 { // a few bright ones with a cross glint
			for d := 1; d <= 3; d++ {
				k := b * 0.45 / float64(d)
				c.Light(x+d, y, hx(0xFFF0F4), k)
				c.Light(x-d, y, hx(0xFFF0F4), k)
				c.Light(x, y+d, hx(0xFFF0F4), k)
				c.Light(x, y-d, hx(0xFFF0F4), k)
			}
		}
	}
	const mx, my, mr = 1060.0, 205.0, 86.0
	glow(c, mx, my, 150, hx(0xE0384A), 0.42)
	glow(c, mx, my, 70, hx(0xFFB098), 0.3)
	craters := []struct{ x, y, r float64 }{{-30, -22, 22}, {18, 10, 14}, {-8, 36, 18}, {34, -30, 10}, {-46, 18, 9}, {40, 34, 12}}
	for y := int(my - mr - 2); y <= int(my+mr+2); y++ {
		for x := int(mx - mr - 2); x <= int(mx+mr+2); x++ {
			dx, dy := float64(x)+0.5-mx, float64(y)+0.5-my
			d := math.Hypot(dx, dy)
			if d > mr+1 {
				continue
			}
			col := art.Mix(hx(0xFFF1E6), hx(0xF4A592), math.Pow(d/mr, 2.2))
			// Light from the upper left, a soft terminator to the lower right.
			col = art.Mix(col, hx(0xC86A6E), art.Smooth((dx+dy*0.6)/mr*0.8)*0.35)
			for _, cr := range craters {
				if e := math.Hypot(dx-cr.x, dy-cr.y) / cr.r; e < 1 {
					col = art.Mix(col, hx(0xD7837C), (1-e*e)*0.35)
				}
			}
			c.Over(x, y, col, art.Clamp01(mr+0.5-d))
		}
	}
	// Thin clouds drifting across the moon and the lower sky.
	clouds := make([]float64, SW*SH)
	for i := 0; i < 7; i++ {
		cx, cy := 300+rng.Float64()*1100, 230+rng.Float64()*170
		length, thick := 260+rng.Float64()*320, 6+rng.Float64()*8
		for k := 0; k < 26; k++ {
			px := cx + (float64(k)/25-0.5)*length
			py := cy + math.Sin(float64(k)*0.5+float64(i))*4
			rr := thick * (1 - math.Abs(float64(k)/25-0.5)*1.4)
			for y := int(py - rr); y <= int(py+rr); y++ {
				for x := int(px - rr*2); x <= int(px+rr*2); x++ {
					if x >= 0 && y >= 0 && x < SW && y < SH {
						e := math.Hypot((float64(x)-px)/2, float64(y)-py) / math.Max(rr, 1)
						clouds[y*SW+x] = math.Max(clouds[y*SW+x], 1-e)
					}
				}
			}
		}
	}
	clouds = art.Blur(clouds, SW, SH, 4)
	for y := 0; y < SH; y++ {
		for x := 0; x < SW; x++ {
			v := clouds[y*SW+x]
			if v <= 0.01 {
				continue
			}
			c.Over(x, y, hx(0x2A1230), math.Min(1, v*1.6)*0.75)
			// Moonlit edges.
			if d := math.Hypot(float64(x)-mx, float64(y)-my); d < 220 {
				c.Light(x, y, hx(0xFF9A88), v*(1-v)*1.4*(1-d/220))
			}
		}
	}
	out = append(out, stageSprite{0, 0, 0, 0, c.Image(true)})

	// 5,0 Mountains (delta 0.15): two misty ridges and a pagoda.
	const MW, MH = 1800, 260
	c = art.NewCanvas(MW, MH)
	back, front := ridge(MW, 120, 70, 3), ridge(MW, 175, 45, 4)
	for x := 0; x < MW; x++ {
		for y := int(back[x]); y < MH; y++ {
			c.Over(x, y, art.Mix(hx(0x4A2440), hx(0x5E2A40), float64(y)/MH), 1)
		}
	}
	// A pagoda on the far ridge.
	px := 640
	py := back[px] + 6
	for i, t := range []struct{ w, h float64 }{{70, 22}, {56, 20}, {44, 18}, {32, 16}} {
		base := py - float64(i)*24
		roof := []logo.Pt{{X: float64(px) - t.w, Y: base - 6}, {X: float64(px) + t.w, Y: base - 6},
			{X: float64(px) + t.w*0.62, Y: base - t.h}, {X: float64(px) - t.w*0.62, Y: base - t.h}}
		c.PaintMask(art.ShapeMask(MW, MH, roof), art.Solid(hx(0x3A1A34)))
		rect(c, px-int(t.w*0.45), int(base-6), px+int(t.w*0.45), int(base+18), art.Solid(hx(0x3A1A34)))
		if i%2 == 0 {
			glow(c, float64(px), base+4, 6, hx(0xFF9A5A), 0.35)
		}
	}
	rect(c, px-2, int(py-130), px+2, int(py-96), art.Solid(hx(0x3A1A34)))
	for x := 0; x < MW; x++ {
		for y := int(front[x]); y < MH; y++ {
			c.Over(x, y, art.Mix(hx(0x2E1530), hx(0x3E1C34), float64(y)/MH), 1)
		}
	}
	// Mist rising from the valleys.
	for y := 0; y < MH; y++ {
		k := art.Smooth((float64(y) - 150) / 110)
		for x := 0; x < MW; x++ {
			c.Over(x, y, hx(0x6A3048), k*0.55)
		}
	}
	out = append(out, stageSprite{5, 0, 0, MH, c.Image(true)})

	// 1,0 City (delta 0.3): a hazy back row, a dark front row with lit
	// windows, rooftop details and neon signs.
	const CW, CH = 2000, 380
	c = art.NewCanvas(CW, CH)
	building := func(x, w, h int, body art.RGBA, lit float64, neon bool) {
		top := CH - h
		rect(c, x, top, x+w, CH, art.Linear(0, float64(top), 0, CH, art.Mix(body, hx(0xFFFFFF), 0.04), body))
		rect(c, x, top, x+2, CH, art.Solid(art.Mix(body, hx(0xFF6A5A), 0.12))) // moonlit edge
		if rng.Intn(3) == 0 {                                                  // antenna
			ax := x + 8 + rng.Intn(max(1, w-16))
			rect(c, ax, top-24-rng.Intn(30), ax+2, top, art.Solid(body))
			c.Light(ax, top-26, hx(0xFF3040), 0.9)
		} else if rng.Intn(2) == 0 { // water tank
			tx := x + 6 + rng.Intn(max(1, w-26))
			rect(c, tx, top-16, tx+18, top, art.Solid(body))
		}
		for wy := top + 10; wy < CH-10; wy += 13 {
			for wx := x + 6; wx < x+w-8; wx += 11 {
				if rng.Float64() < lit {
					col := hx(0xFF6A44)
					if rng.Intn(3) == 0 {
						col = hx(0xFFC870)
					}
					rect(c, wx, wy, wx+5, wy+6, art.Solid(col.Alpha(0.55+rng.Float64()*0.4)))
				}
			}
		}
		if neon && h > 160 {
			nx, ny := x+w/2-5, top+30
			nh := 50 + rng.Intn(60)
			col := []art.RGBA{hx(0xFF3A5A), hx(0x3AD8FF), hx(0xFF6AD0)}[rng.Intn(3)]
			for y := ny; y < ny+nh; y++ {
				for xx := nx; xx < nx+10; xx++ {
					c.Over(xx, y, art.Mix(col, hx(0xFFFFFF), 0.35), 1)
				}
			}
			for y := ny - 30; y < ny+nh+30; y++ {
				for xx := nx - 40; xx < nx+50; xx++ {
					dx := math.Max(0, math.Max(float64(nx-xx), float64(xx-nx-10)))
					dy := math.Max(0, math.Max(float64(ny-y), float64(y-ny-nh)))
					d := math.Hypot(dx, dy)
					c.Light(xx, y, col, 0.35*math.Exp(-d*d/(2*14*14)))
				}
			}
		}
	}
	for x := -20; x < CW; { // back row
		w := 50 + rng.Intn(80)
		building(x, w, 150+rng.Intn(190), hx(0x2C1428), 0.06, false)
		x += w + rng.Intn(6)
	}
	for y := 0; y < CH; y++ { // haze between the rows
		for x := 0; x < CW; x++ {
			c.Over(x, y, hx(0x4A1E36), 0.35*art.Smooth(float64(y)/CH))
		}
	}
	for x := -10; x < CW; { // front row
		w := 60 + rng.Intn(100)
		building(x, w, 80+rng.Intn(200), hx(0x150A14), 0.22, rng.Intn(4) == 0)
		x += w + 4 + rng.Intn(14)
	}
	out = append(out, stageSprite{1, 0, 0, CH, c.Image(true)})

	// 2,0 Dojo (delta 0.6): a roof beam, pillars and a railing with a low
	// wooden wall; the city shows above the railing.
	const WW, WH = 2400, 560
	c = art.NewCanvas(WW, WH)
	wood := func(x0, x1 int, lit, dark art.RGBA) art.Paint {
		return art.Linear(float64(x0), 0, float64(x1), 0, lit, dark)
	}
	// Low wall with vertical boards, and the railing above it.
	rect(c, 0, WH-94, WW, WH, art.Linear(0, WH-94, 0, WH, hx(0x2A1612), hx(0x120806)))
	for x := 0; x < WW; x += 38 {
		rect(c, x, WH-94, x+2, WH, art.Solid(hx(0x0C0504).Alpha(0.8)))
	}
	for x := 14; x < WW; x += 30 {
		rect(c, x, WH-124, x+7, WH-94, wood(x, x+7, hx(0x4A2A1C), hx(0x1E0E0A)))
	}
	rect(c, 0, WH-136, WW, WH-122, art.Linear(0, WH-136, 0, WH-122, hx(0x5A3222), hx(0x24120C)))
	rect(c, 0, WH-136, WW, WH-134, art.Solid(hx(0xC8604A).Alpha(0.6)))
	rect(c, 0, WH-100, WW, WH-92, art.Linear(0, WH-100, 0, WH-92, hx(0x4A2618), hx(0x1A0C08)))
	// Pillars.
	for x := 40; x < WW; x += 480 {
		rect(c, x, 0, x+34, WH, wood(x, x+34, hx(0x5A3020), hx(0x1A0C08)))
		rect(c, x+3, 0, x+5, WH, art.Solid(hx(0xC8604A).Alpha(0.35)))
		rect(c, x-4, WH-18, x+38, WH, wood(x-4, x+38, hx(0x3A1E14), hx(0x120806)))
	}
	// Roof beam with a red lacquered trim.
	rect(c, 0, 0, WW, 30, art.Linear(0, 0, 0, 30, hx(0x0E0708), hx(0x1E0E0C)))
	rect(c, 0, 30, WW, 37, art.Linear(0, 30, 0, 37, hx(0xD02A32), hx(0x7A0E14)))
	for x := 0; x < WW; x++ {
		for y := 37; y < 50; y++ {
			c.Over(x, y, hx(0x000000), 0.4*(1-float64(y-37)/13))
		}
	}
	out = append(out, stageSprite{2, 0, 0, WH, c.Image(true)})

	// 3,0 Lanterns (delta 0.6, same row): paper lanterns between the pillars.
	const LW, LH = 2400, 220
	c = art.NewCanvas(LW, LH)
	for x := 280; x < LW; x += 480 {
		fx := float64(x)
		glow(c, fx, 100, 70, hx(0xFF5A30), 0.3)
		rect(c, x-1, 0, x+1, 52, art.Solid(hx(0x0A0506)))
		for y := 52; y < 140; y++ {
			for xx := -30; xx <= 30; xx++ {
				e := float64(xx) / 30
				f := (float64(y) - 96) / 44
				if e*e+f*f >= 1 {
					continue
				}
				d := math.Sqrt(e*e + f*f)
				col := art.Mix(hx(0xFFD49A), hx(0xD8301E), math.Pow(d, 1.4))
				if (xx+30)%10 < 2 { // paper ribs
					col = art.Mix(col, hx(0x7A140E), 0.45)
				}
				if y%11 == 0 {
					col = art.Mix(col, hx(0x9A1A12), 0.3)
				}
				c.Over(x+xx, y, col, art.Clamp01((1-d)*30))
			}
		}
		rect(c, x-14, 50, x+14, 58, art.Solid(hx(0x140A08)))
		rect(c, x-12, 134, x+12, 142, art.Solid(hx(0x140A08)))
		rect(c, x-1, 142, x+1, 162, art.Solid(hx(0xD8301E)))
		glow(c, fx, 96, 18, hx(0xFFC890), 0.35)
	}
	out = append(out, stageSprite{3, 0, 0, 0, c.Image(false)})

	// 4,0 Floor: polished planks in perspective with reflections.
	const FW, FH = 2600, 160
	c = art.NewCanvas(FW, FH)
	vx := float64(FW) / 2
	for y := 0; y < FH; y++ {
		t := float64(y) / FH
		for x := 0; x < FW; x++ {
			// Planks converge to the center: index by the x where this line
			// meets the far edge.
			u := (float64(x) - vx) / (1 + t*1.65)
			plank := int(math.Floor(u / 34))
			shade := []art.RGBA{hx(0x2E1812), hx(0x27140F), hx(0x331B14), hx(0x2A1611)}[(plank%4+4)%4]
			grain := 0.5 + 0.5*math.Sin(u*0.9+float64(plank)*3.1+math.Sin(t*14+float64(plank))*2)
			col := art.Mix(shade, hx(0x120806), t*0.55)
			col = art.Mix(col, hx(0x3E2219), grain*0.12)
			if math.Mod(u+340000, 34) < 1.2 {
				col = hx(0x0C0504)
			}
			c.Over(x, y, col, 1)
		}
	}
	for y := 0; y < FH; y++ { // lantern and moon reflections
		for x := 0; x < FW; x++ {
			v := 0.0
			for lx := 280.0 * 1.667; lx < FW; lx += 480 * 1.667 {
				d := math.Hypot((float64(x)-lx)/1.6, float64(y)*0.5)
				v += math.Exp(-d * d / (2 * 60 * 60))
			}
			c.Light(x, y, hx(0xFF5A2E), 0.28*v)
			c.Light(x, y, hx(0x7A5AA0), 0.06*(1-float64(y)/FH))
		}
	}
	for x := 0; x < FW; x++ { // red lacquered edge
		for y := 0; y < 5; y++ {
			c.Over(x, y, hx(0xC8202A), 1)
		}
		c.Over(x, 0, hx(0xFF8A7A), 0.6)
		for y := 5; y < 16; y++ {
			c.Light(x, y, hx(0xD01E2A), 0.35*math.Exp(-float64(y-5)/4))
		}
	}
	for y := 0; y < FH; y++ { // darker towards the sides
		for x := 0; x < FW; x++ {
			k := math.Abs(float64(x)-vx) / vx
			c.Over(x, y, hx(0x050203), 0.5*art.Smooth((k-0.45)/0.55))
		}
	}
	out = append(out, stageSprite{4, 0, 0, 0, c.Image(true)})
	return out
}
