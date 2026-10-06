package main

// The common effects MUGEN characters call by number from fightfx.air (hit,
// guard and super sparks, shockwaves, dust). They are drawn for a 1080p
// fight screen ([FightFx] scale = 1 at localcoord 1920,1080) and, except
// the dust, shown with additive blending.

import (
	"fmt"
	"image"
	"math"
	"math/rand"
	"strings"

	"github.com/oakTINOoff/OMugen/tools/internal/art"
)

// fxAnim is one fightfx.air action.
type fxAnim struct {
	no      int
	comment string
	frames  []fxFrame
	loop    bool
}

type fxFrame struct {
	n, time int
	add     bool
}

var (
	fxAnims   []fxAnim
	fxSprites []sprite // fightfx.sff
)

// spark is frame i of n of a starburst of diameter d: a white-hot core,
// rays and a ring that grow and fade, tinted by col.
func spark(d float64, i, n int, col art.RGBA, rays int, seed int64) *art.Canvas {
	size := int(d * 1.15)
	c := art.NewCanvas(size, size)
	cx, cy := float64(size)/2, float64(size)/2
	t := float64(i) / float64(n-1) // 0..1
	grow := 1 - math.Pow(1-t, 2.2) // fast out
	fade := 1 - art.Smooth(t)
	r := rand.New(rand.NewSource(seed))
	type ray struct{ a, l, w float64 }
	rs := make([]ray, rays)
	for k := range rs {
		rs[k] = ray{float64(k)*2*math.Pi/float64(rays) + r.Float64()*0.5, 0.55 + r.Float64()*0.45, 0.06 + r.Float64()*0.05}
	}
	core := d * (0.10 + 0.12*(1-t))
	ringR := d * 0.5 * (0.25 + 0.75*grow)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx, dy := float64(x)+0.5-cx, float64(y)+0.5-cy
			dist := math.Hypot(dx, dy)
			ang := math.Atan2(dy, dx)
			// Core: bright, shrinking.
			v := math.Exp(-dist*dist/(2*core*core)) * (0.6 + 0.4*fade) * 1.3
			// Rays: long thin spikes that shoot out and thin away.
			for _, rr := range rs {
				da := math.Abs(math.Remainder(ang-rr.a, 2*math.Pi))
				reach := d * 0.5 * rr.l * (0.35 + 0.65*grow)
				if dist > reach {
					continue
				}
				width := rr.w * (1 - dist/reach) * (1.2 - 0.6*t)
				if da < width {
					v += (1 - da/width) * (1 - dist/reach) * fade * 1.1
				}
			}
			// Ring: a thin shell that expands and fades.
			ring := math.Exp(-math.Pow((dist-ringR)/(d*0.025+1), 2)) * fade * 0.7
			v += ring
			if v <= 0.003 {
				continue
			}
			// Hot center fades to the tint.
			hot := art.Clamp01(1 - dist/(d*0.32))
			c.Light(x, y, art.Mix(col, art.Hex(0xFFFFFF, 1), hot*0.8), math.Min(1.4, v))
		}
	}
	return c
}

// shockwave is an expanding elliptical ring (flat for the ground, upright
// for walls), frame i of n.
func shockwave(w, h float64, i, n int, col art.RGBA) *art.Canvas {
	cw, ch := int(w*1.1), int(h*1.1)
	c := art.NewCanvas(cw, ch)
	t := float64(i) / float64(n-1)
	grow := 0.2 + 0.8*(1-math.Pow(1-t, 2))
	fade := 1 - art.Smooth(t)
	rx, ry := w/2*grow, h/2*grow
	th := 0.05 + 0.08*(1-t)
	cx, cy := float64(cw)/2, float64(ch)/2
	for y := 0; y < ch; y++ {
		for x := 0; x < cw; x++ {
			dx, dy := (float64(x)+0.5-cx)/rx, (float64(y)+0.5-cy)/ry
			e := math.Sqrt(dx*dx + dy*dy)
			v := math.Exp(-math.Pow((e-1)/th, 2)) * fade
			v += math.Exp(-e*e*4) * fade * 0.35 * (1 - t)
			if v > 0.003 {
				c.Light(x, y, art.Mix(col, art.Hex(0xFFFFFF, 1), v*0.5), v)
			}
		}
	}
	return c
}

// superFlash is the flash behind a super move: a big radial burst with
// slowly turning rays, frame i of n.
func superFlash(d float64, i, n int, col art.RGBA) *art.Canvas {
	size := int(d)
	c := art.NewCanvas(size, size)
	cx, cy := d/2, d/2
	t := float64(i) / float64(n-1)
	in := art.Smooth(t / 0.15)
	out := 1 - art.Smooth((t-0.55)/0.45)
	amt := in * out
	spin := t * 0.6
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx, dy := float64(x)+0.5-cx, float64(y)+0.5-cy
			dist := math.Hypot(dx, dy) / (d / 2)
			if dist >= 1 {
				continue
			}
			ang := math.Atan2(dy, dx) + spin
			rays := math.Pow(math.Abs(math.Cos(ang*8)), 12) * (1 - dist) * 0.8
			glow := math.Exp(-dist*dist*6) * 0.9
			v := (rays + glow) * amt
			if v > 0.003 {
				c.Light(x, y, art.Mix(col, art.Hex(0xFFFFFF, 1), glow), v)
			}
		}
	}
	return c
}

// dust is a soft gray puff at the floor (axis at its bottom center), frame
// i of n, drawn normally.
func dust(w float64, i, n int) *art.Canvas {
	cw, ch := int(w), int(w*0.6)
	c := art.NewCanvas(cw, ch)
	t := float64(i) / float64(n-1)
	r := rand.New(rand.NewSource(7))
	fade := 1 - art.Smooth(t)
	for k := 0; k < 7; k++ {
		px := float64(cw)/2 + (r.Float64()-0.5)*w*0.6*(0.4+t)
		py := float64(ch) - w*0.08 - r.Float64()*w*0.18*(0.5+t)
		pr := w * (0.08 + r.Float64()*0.06) * (0.6 + 0.8*t)
		for y := 0; y < ch; y++ {
			for x := 0; x < cw; x++ {
				d := math.Hypot(float64(x)+0.5-px, float64(y)+0.5-py) / pr
				if d < 1 {
					c.Over(x, y, art.Hex(0xC9C2B8, 1), (1-d*d)*0.45*fade)
				}
			}
		}
	}
	return c
}

// powerUp is a looping column of rising light, frame i of n.
func powerUp(w, h float64, i, n int, col art.RGBA) *art.Canvas {
	cw, ch := int(w), int(h)
	c := art.NewCanvas(cw, ch)
	phase := float64(i) / float64(n)
	r := rand.New(rand.NewSource(11))
	// A soft aura.
	for y := 0; y < ch; y++ {
		fy := float64(y) / h
		for x := 0; x < cw; x++ {
			dx := (float64(x) + 0.5 - w/2) / (w / 2)
			v := math.Exp(-dx*dx*3) * math.Pow(fy, 1.5) * (0.35 + 0.1*math.Sin((phase+fy)*2*math.Pi))
			if v > 0.003 {
				c.Light(x, y, col, v)
			}
		}
	}
	// Rising sparkles (positions wrap with the loop).
	for k := 0; k < 18; k++ {
		x0 := w/2 + (r.Float64()-0.5)*w*0.8
		speed := 0.7 + r.Float64()*0.6
		off := r.Float64()
		p := math.Mod(off+phase*speed, 1)
		py := h * (1 - p)
		a := math.Sin(p * math.Pi)
		for y := int(py - 10); y < int(py+10); y++ {
			for x := int(x0 - 10); x < int(x0+10); x++ {
				d := math.Hypot(float64(x)+0.5-x0, float64(y)+0.5-py)
				if d < 8 {
					c.Light(x, y, art.Mix(col, art.Hex(0xFFFFFF, 1), 0.6), (1-d/8)*a)
				}
			}
		}
	}
	return c
}

// addFx registers frames as sprites group no and the action no.
func addFx(no int, comment string, frames []*art.Canvas, times []int, axis func(c *art.Canvas) (int, int), additive, loop bool) {
	a := fxAnim{no: no, comment: comment, loop: loop}
	for i, c := range frames {
		var im image.Image
		if additive {
			im = c.Additive()
		} else {
			im = c.Image(false)
		}
		ax, ay := axis(c)
		fxSprites = append(fxSprites, sprite{no, i, ax, ay, im})
		a.frames = append(a.frames, fxFrame{n: i, time: times[i], add: additive})
	}
	fxAnims = append(fxAnims, a)
}

func center(c *art.Canvas) (int, int)       { return c.W / 2, c.H / 2 }
func bottomCenter(c *art.Canvas) (int, int) { return c.W / 2, c.H - int(float64(c.H)*0.12) }

// even returns n durations of t ticks, with extra ticks added to the last.
func even(n, t, extra int) []int {
	ts := make([]int, n)
	for i := range ts {
		ts[i] = t
	}
	ts[n-1] += extra
	return ts
}

func drawFx() {
	frames := func(n int, f func(i int) *art.Canvas) []*art.Canvas {
		out := make([]*art.Canvas, n)
		for i := range out {
			out[i] = f(i)
		}
		return out
	}
	hit := art.Hex(0xFFC46A, 1)
	sizes := []float64{150, 210, 280, 360}
	names := []string{"Light", "Medium", "Strong", "Very strong"}
	for k, d := range sizes {
		k, d := k, d
		n := 7 + k
		addFx(k, names[k]+" hit spark", frames(n, func(i int) *art.Canvas { return spark(d, i, n, hit, 9+2*k, int64(k)) }), even(n, 2, 0), center, true, false)
	}
	blood := art.Hex(0xFF3A3A, 1)
	for k := 0; k < 3; k++ {
		k := k
		d := sizes[k] * 0.9
		n := 6 + k
		addFx(10+k, names[k]+" red hit spark", frames(n, func(i int) *art.Canvas { return spark(d, i, n, blood, 7+2*k, int64(10+k)) }), even(n, 3, 0), center, true, false)
	}
	addFx(40, "Guard spark", frames(6, func(i int) *art.Canvas { return spark(200, i, 6, blueHot, 6, 40) }), even(6, 2, 0), center, true, false)
	for k := 0; k < 3; k++ {
		k := k
		w := 300 + 150*float64(k)
		n := 5 + k
		addFx(60+k, names[k]+" ground shockwave", frames(n, func(i int) *art.Canvas { return shockwave(w, w*0.22, i, n, hit) }), even(n, 2, k), center, true, false)
		addFx(70+k, names[k]+" wall-hit shockwave", frames(n+2, func(i int) *art.Canvas { return shockwave(w*0.3, w, i, n+2, hit) }), even(n+2, 2, 1), center, true, false)
	}
	supers := []art.RGBA{goldHot, art.Hex(0xFFA040, 1), redHot}
	for k, col := range supers {
		col := col
		d := 520 + 90*float64(k)
		addFx(100+k, fmt.Sprintf("Level %d super flash", k+1), frames(10, func(i int) *art.Canvas { return superFlash(d, i, 10, col) }), even(10, 3, k), center, true, false)
	}
	addFx(120, "Small floor dust", frames(6, func(i int) *art.Canvas { return dust(220, i, 6) }), []int{3, 4, 4, 5, 5, 6}, bottomCenter, false, false)
	addFx(130, "Power up (loops)", frames(11, func(i int) *art.Canvas { return powerUp(260, 420, i, 11, goldHot) }),
		even(11, 2, 0), func(c *art.Canvas) (int, int) { return c.W / 2, c.H }, true, true)
}

// fightfxAir writes fightfx.air.
func fightfxAir() string {
	var b strings.Builder
	b.WriteString("; OMugen common effects (generated by tools/lifebarsgen).\n")
	b.WriteString("; Characters call these by number: hit sparks 0-3, red hit sparks 10-12,\n")
	b.WriteString("; guard spark 40, shockwaves 60-62 (ground) and 70-72 (wall), super\n")
	b.WriteString("; flashes 100-102, floor dust 120, power up 130.\n")
	for _, a := range fxAnims {
		fmt.Fprintf(&b, "\n; %s\n[Begin Action %d]\n", a.comment, a.no)
		if a.loop {
			b.WriteString("LoopStart\n")
		}
		for _, f := range a.frames {
			trans := ""
			if f.add {
				trans = ", , A"
			}
			fmt.Fprintf(&b, "%d,%d, 0,0, %d%s\n", a.no, f.n, f.time, trans)
		}
	}
	return b.String()
}
