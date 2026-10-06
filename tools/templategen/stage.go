package main

// Proto Arena: a night dojo courtyard at 1280x720, in parallax layers.

import (
	"image"
	"math"
	"math/rand"

	"github.com/oakTINOoff/OMugen/tools/internal/art"
)

type stageSprite struct {
	g, n   int
	ax, ay int
	img    image.Image
}

func stageSprites() []stageSprite {
	var out []stageSprite
	rng := rand.New(rand.NewSource(11))

	// 0,0 Sky: deep night gradient, a red moon and stars (delta 0.08).
	const SW, SH = 1500, 720
	c := art.NewCanvas(SW, SH)
	c.Fill(art.Linear(0, 0, 0, 560, art.Hex(0x07050C, 1), art.Hex(0x3A0C16, 1)))
	for i := 0; i < 160; i++ {
		x, y := rng.Intn(SW), rng.Intn(420)
		b := 0.25 + rng.Float64()*0.6
		c.Light(x, y, art.Hex(0xFFE6E0, 1), b)
		if rng.Intn(5) == 0 {
			c.Light(x+1, y, art.Hex(0xFFE6E0, 1), b*0.5)
			c.Light(x, y+1, art.Hex(0xFFE6E0, 1), b*0.5)
		}
	}
	for y := 0; y < SH; y++ {
		for x := 0; x < SW; x++ {
			d := math.Hypot(float64(x)-1020, float64(y)-190)
			if d < 64 {
				c.Over(x, y, art.Mix(art.Hex(0xFFD6C8, 1), art.Hex(0xE8664E, 1), d/64), math.Min(1, 64-d))
			}
			c.Light(x, y, art.Hex(0xD02030, 1), 0.35*math.Exp(-d*d/(2*160*160)))
		}
	}
	out = append(out, stageSprite{0, 0, 0, 0, c.Image(true)})

	// 1,0 Far city: silhouettes with lit windows (delta 0.3).
	const CW, CH = 2000, 360
	c = art.NewCanvas(CW, CH)
	for x := 0; x < CW; {
		w := 40 + rng.Intn(90)
		h := 70 + rng.Intn(230)
		shade := art.Hex(0x14090F, 1)
		for yy := CH - h; yy < CH; yy++ {
			for xx := x; xx < x+w && xx < CW; xx++ {
				c.Over(xx, yy, shade, 1)
			}
		}
		for wy := CH - h + 10; wy < CH-8; wy += 14 {
			for wx := x + 6; wx < x+w-6; wx += 12 {
				if rng.Intn(5) == 0 {
					col := art.Hex(0xFF5A3A, 1)
					if rng.Intn(3) == 0 {
						col = art.Hex(0xFFC46A, 1)
					}
					for yy := 0; yy < 5; yy++ {
						for xx := 0; xx < 4; xx++ {
							c.Over(wx+xx, wy+yy, col.Alpha(0.75), 1)
						}
					}
				}
			}
		}
		x += w + rng.Intn(12)
	}
	out = append(out, stageSprite{1, 0, 0, CH, c.Image(true)})

	// 2,0 Dojo wall with wooden posts and a roof edge (delta 0.6).
	const WW, WH = 2400, 300
	c = art.NewCanvas(WW, WH)
	c.Fill(art.Linear(0, 0, 0, WH, art.Hex(0x1C1014, 1), art.Hex(0x120A0E, 1)))
	for x := 0; x < WW; x += 240 {
		for yy := 0; yy < WH; yy++ {
			for xx := 0; xx < 26; xx++ {
				c.Over(x+xx, yy, art.Linear(float64(x), 0, float64(x+26), 0, art.Hex(0x3A1E14, 1), art.Hex(0x1E0E0A, 1))(float64(x+xx), 0), 1)
			}
		}
	}
	for yy := 0; yy < 34; yy++ { // roof beam
		for xx := 0; xx < WW; xx++ {
			c.Over(xx, yy, art.Hex(0x0A0508, 1), 1)
		}
	}
	for xx := 0; xx < WW; xx++ { // red trim
		for yy := 34; yy < 40; yy++ {
			c.Over(xx, yy, art.Hex(0xA0141E, 1), 1)
		}
	}
	out = append(out, stageSprite{2, 0, 0, WH, c.Image(true)})

	// 3,0 Paper lanterns hanging from the roof (delta 0.6, same row).
	const LW, LH = 2400, 200
	c = art.NewCanvas(LW, LH)
	for x := 120; x < LW; x += 480 {
		for yy := 0; yy < 60; yy++ {
			c.Over(x, yy, art.Hex(0x0A0508, 1), 1)
		}
		for yy := 60; yy < 130; yy++ {
			for xx := -26; xx <= 26; xx++ {
				e := (float64(xx) / 26) * (float64(xx) / 26)
				f := (float64(yy) - 95) / 35
				if e+f*f < 1 {
					c.Over(x+xx, yy, art.Mix(art.Hex(0xFFB27A, 1), art.Hex(0xD2321E, 1), math.Sqrt(e+f*f)), 1)
				}
			}
		}
		for yy := 0; yy < LH; yy++ {
			for xx := -120; xx <= 120; xx++ {
				d := math.Hypot(float64(xx), float64(yy-95))
				c.Light(x+xx, yy, art.Hex(0xFF6A3A, 1), 0.28*math.Exp(-d*d/(2*60*60)))
			}
		}
	}
	out = append(out, stageSprite{3, 0, 0, 0, c.Image(false)})

	// 4,0 Floor: polished dark wood planks in perspective with a red line.
	const FW, FH = 2600, 150
	c = art.NewCanvas(FW, FH)
	c.Fill(art.Linear(0, 0, 0, FH, art.Hex(0x2A1510, 1), art.Hex(0x120806, 1)))
	for i := -40; i <= 40; i++ { // plank seams converge to the center
		x0 := float64(FW)/2 + float64(i)*34
		x1 := float64(FW)/2 + float64(i)*90
		for y := 0; y < FH; y++ {
			t := float64(y) / FH
			x := x0 + (x1-x0)*t
			for d := -1; d <= 0; d++ {
				c.Over(int(x)+d, y, art.Hex(0x080302, 1), 0.7)
			}
		}
	}
	for y := 0; y < FH; y++ { // reflection of the lanterns
		for x := 0; x < FW; x++ {
			v := 0.0
			for lx := 120.0 + 100; lx < FW; lx += 480 * 1.667 {
				d := math.Hypot(float64(x)-lx, float64(y)*3)
				v += math.Exp(-d * d / (2 * 90 * 90))
			}
			c.Light(x, y, art.Hex(0xFF5A2E, 1), 0.22*v)
		}
	}
	for x := 0; x < FW; x++ {
		for y := 0; y < 4; y++ {
			c.Over(x, y, art.Hex(0xD01E2A, 1), 1)
		}
		for y := 4; y < 14; y++ {
			c.Light(x, y, art.Hex(0xD01E2A, 1), 0.4*math.Exp(-float64(y-4)/4))
		}
	}
	out = append(out, stageSprite{4, 0, 0, 0, c.Image(true)})
	return out
}
