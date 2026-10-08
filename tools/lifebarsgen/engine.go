package main

// The engine's own effects and icons, which upstream took from its
// screenpack repository: the common fx the engine's states call with the
// GO prefix (dizzy, guard break, tag switch; data/gofx) and the button and
// motion icons of the move list (data/glyphs.sff).

import (
	"fmt"
	"image"
	"image/draw"
	"math"
	"math/rand"
	"strings"

	xdraw "golang.org/x/image/draw"

	"github.com/oakTINOoff/OMugen/tools/internal/art"
	"github.com/oakTINOoff/OMugen/tools/internal/logo"
)

// ---------------------------------------------------------------- shapes

// star is a regular star with n points, outer radius r and inner radius ri,
// centered on (cx, cy), with a point straight up.
func star(cx, cy, r, ri float64, n int) []logo.Pt {
	var pts []logo.Pt
	for i := 0; i < 2*n; i++ {
		rr := r
		if i%2 == 1 {
			rr = ri
		}
		a := -math.Pi/2 + float64(i)*math.Pi/float64(n)
		pts = append(pts, logo.Pt{X: cx + rr*math.Cos(a), Y: cy + rr*math.Sin(a)})
	}
	return pts
}

func circle(cx, cy, r float64) []logo.Pt {
	var pts []logo.Pt
	for i := 0; i < 64; i++ {
		a := float64(i) / 64 * 2 * math.Pi
		pts = append(pts, logo.Pt{X: cx + r*math.Cos(a), Y: cy + r*math.Sin(a)})
	}
	return pts
}

func ellipse(cx, cy, rx, ry, rot float64) []logo.Pt {
	var pts []logo.Pt
	s, c := math.Sincos(rot)
	for i := 0; i < 64; i++ {
		a := float64(i) / 64 * 2 * math.Pi
		x, y := rx*math.Cos(a), ry*math.Sin(a)
		pts = append(pts, logo.Pt{X: cx + x*c - y*s, Y: cy + x*s + y*c})
	}
	return pts
}

// stroke is a thick polyline: one quad per segment and a disc per joint
// (all wound the same way, so their union fills).
func stroke(pts []logo.Pt, w float64) [][]logo.Pt {
	var cs [][]logo.Pt
	for i := 0; i+1 < len(pts); i++ {
		a, b := pts[i], pts[i+1]
		dx, dy := b.X-a.X, b.Y-a.Y
		l := math.Hypot(dx, dy)
		if l == 0 {
			continue
		}
		nx, ny := -dy/l*w/2, dx/l*w/2
		cs = append(cs, wound([]logo.Pt{{X: a.X + nx, Y: a.Y + ny}, {X: b.X + nx, Y: b.Y + ny}, {X: b.X - nx, Y: b.Y - ny}, {X: a.X - nx, Y: a.Y - ny}}))
	}
	for _, p := range pts[1 : len(pts)-1] {
		cs = append(cs, wound(circle(p.X, p.Y, w/2)))
	}
	return cs
}

// wound returns the contour wound one fixed way, so overlapping pieces of a
// shape add up under the nonzero rule instead of cancelling out.
func wound(c []logo.Pt) []logo.Pt {
	var area float64
	for i, p := range c {
		q := c[(i+1)%len(c)]
		area += p.X*q.Y - q.X*p.Y
	}
	if area < 0 {
		return art.Reversed(c)
	}
	return c
}

// head is an arrowhead at p pointing along angle a.
func head(p logo.Pt, a, size float64) []logo.Pt {
	s, c := math.Sincos(a)
	pt := func(x, y float64) logo.Pt { return logo.Pt{X: p.X + x*c - y*s, Y: p.Y + x*s + y*c} }
	return wound([]logo.Pt{pt(size*0.75, 0), pt(-size*0.45, size*0.6), pt(-size*0.45, -size*0.6)})
}

// outlined paints shapes in a color with a dark outline and a soft glow.
func outlined(c *art.Canvas, m []float64, fill art.Paint, glow art.RGBA, out float64) {
	if glow.A > 0 {
		c.LightMask(art.Blur(m, c.W, c.H, 4), glow, 0.6)
	}
	if out > 0 {
		c.PaintMask(art.ScaleMask(art.Blur(m, c.W, c.H, int(out)), 5), art.Solid(ink))
	}
	c.PaintMask(m, fill)
}

func mask(c *art.Canvas, cs ...[]logo.Pt) []float64 { return art.ShapeMask(c.W, c.H, cs...) }

// union masks overlapping shapes as one (see wound).
func union(c *art.Canvas, cs ...[]logo.Pt) []float64 {
	for i := range cs {
		cs[i] = wound(cs[i])
	}
	return mask(c, cs...)
}

// ---------------------------------------------------------------- gofx

const gofxScale = 2 // drawn at twice the 320x240 size (localcoord 640,360)

type gofxAnim struct {
	no    int
	lines []string
}

var (
	gofxSprites []sprite
	gofxAnims   []gofxAnim
)

func addGofx(no int, im image.Image, ax, ay int, comment string, lines ...string) {
	gofxSprites = append(gofxSprites, sprite{no, 0, ax, ay, im})
	gofxAnims = append(gofxAnims, gofxAnim{no, append([]string{"; " + comment}, lines...)})
}

// dizzyIcon draws dizzy effect kind k (0-8) circling a dizzy character's
// head. Kinds 5 (note) and 6 (bell) are never mirrored.
func dizzyIcon(k int) image.Image {
	const n = 72
	c := art.NewCanvas(n, n)
	cx, cy := n/2.0, n/2.0
	solid := func(col art.RGBA) art.Paint { return art.Solid(col) }
	grad := func(top, bottom art.RGBA) art.Paint { return art.Linear(0, cy-24, 0, cy+24, top, bottom) }
	switch k {
	case 0: // gold star
		outlined(c, mask(c, star(cx, cy, 26, 11, 5)), grad(goldHot, gold), gold, 2)
	case 1: // white sparkle
		outlined(c, mask(c, star(cx, cy, 28, 7, 4)), solid(white), white, 2)
	case 2: // little bird
		m := union(c, ellipse(cx-2, cy+4, 18, 15, 0), circle(cx+10, cy-10, 11))
		outlined(c, m, grad(goldHot, gold), gold, 2)
		c.PaintMask(mask(c, []logo.Pt{{X: cx + 19, Y: cy - 13}, {X: cx + 30, Y: cy - 9}, {X: cx + 19, Y: cy - 5}}), solid(art.Hex(0xFF8A2A, 1)))
		c.PaintMask(mask(c, circle(cx+13, cy-13, 2.6)), solid(ink))
		c.PaintMask(mask(c, ellipse(cx-6, cy+4, 10, 6, -0.5)), solid(art.Hex(0xFFB020, 1)))
	case 3: // blue star
		outlined(c, mask(c, star(cx, cy, 25, 11, 5)), grad(blueHot, blue), blue, 2)
	case 4: // pink heart
		m := union(c, circle(cx-10, cy-6, 13), circle(cx+10, cy-6, 13),
			[]logo.Pt{{X: cx - 22, Y: cy}, {X: cx + 22, Y: cy}, {X: cx, Y: cy + 24}})
		outlined(c, m, grad(art.Hex(0xFFA6C9, 1), art.Hex(0xFF4D8D, 1)), art.Hex(0xFF4D8D, 1), 2)
	case 5: // music note
		m := union(c, ellipse(cx-8, cy+16, 11, 8, -0.4), art.Para(cx+1, cy-26, cx+6, cy+16, 0),
			[]logo.Pt{{X: cx + 1, Y: cy - 26}, {X: cx + 20, Y: cy - 16}, {X: cx + 20, Y: cy - 8}, {X: cx + 1, Y: cy - 18}})
		outlined(c, m, solid(white), white, 2)
	case 6: // bell
		m := union(c, []logo.Pt{{X: cx - 6, Y: cy - 22}, {X: cx + 6, Y: cy - 22}, {X: cx + 15, Y: cy - 6}, {X: cx + 19, Y: cy + 12},
			{X: cx + 24, Y: cy + 16}, {X: cx - 24, Y: cy + 16}, {X: cx - 19, Y: cy + 12}, {X: cx - 15, Y: cy - 6}},
			circle(cx, cy-22, 6), circle(cx, cy+20, 6))
		outlined(c, m, grad(goldHot, art.Hex(0xE09A20, 1)), gold, 2)
	case 7: // swirl
		var pts []logo.Pt
		for i := 0; i <= 60; i++ {
			t := float64(i) / 60
			a := t * 4 * math.Pi
			r := 3 + 22*t
			pts = append(pts, logo.Pt{X: cx + r*math.Cos(a), Y: cy + r*math.Sin(a)})
		}
		outlined(c, mask(c, stroke(pts, 6)...), solid(art.Hex(0x7FE6FF, 1)), art.Hex(0x2FC8FF, 1), 2)
	case 8: // red-orange star
		outlined(c, mask(c, star(cx, cy, 27, 12, 5)), grad(redHot, red), red, 2)
	}
	return img(c)
}

// shard is a glass shard of a guard break, variant k.
func shard(k int) (image.Image, int, int) {
	sizes := [][2]int{{146, 84}, {114, 142}, {130, 114}}
	w, h := sizes[k][0], sizes[k][1]
	c := art.NewCanvas(w, h)
	r := rand.New(rand.NewSource(int64(400 + k)))
	cx, cy := float64(w)/2, float64(h)/2
	var pts []logo.Pt
	n := 5 + k
	for i := 0; i < n; i++ {
		a := float64(i)/float64(n)*2*math.Pi + r.Float64()*0.5
		rx, ry := float64(w)*0.45*(0.55+0.45*r.Float64()), float64(h)*0.45*(0.55+0.45*r.Float64())
		pts = append(pts, logo.Pt{X: cx + rx*math.Cos(a), Y: cy + ry*math.Sin(a)})
	}
	m := mask(c, pts)
	c.LightMask(art.Blur(m, w, h, 5), blueHot, 0.6)
	c.PaintMask(m, art.Linear(0, 0, float64(w), float64(h), art.Hex(0xE8F6FF, 0.95), art.Hex(0x7FB8E8, 0.75)))
	// Bright edges and a crack line.
	edge := art.ShapeMask(w, h, pts, art.Reversed(scalePts(pts, cx, cy, 0.86)))
	c.PaintMask(edge, art.Solid(white))
	c.PaintMask(mask(c, stroke([]logo.Pt{pts[0], {X: cx, Y: cy}, pts[n/2]}, 1.6)...), art.Solid(white.Alpha(0.8)))
	return img(c), w / 2, h / 2
}

func scalePts(pts []logo.Pt, cx, cy, k float64) []logo.Pt {
	out := make([]logo.Pt, len(pts))
	for i, p := range pts {
		out[i] = logo.Pt{X: cx + (p.X-cx)*k, Y: cy + (p.Y-cy)*k}
	}
	return out
}

// breakWave is the upright shockwave of a guard break (additive), seen from
// the side: the near half of a ring.
func breakWave() (image.Image, int, int) {
	w, h := 620, 1350
	c := art.NewCanvas(w, h)
	cx, cy := float64(w)*0.42, float64(h)/2
	rx, ry := float64(w)*0.36, float64(h)*0.47
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx, dy := (float64(x)+0.5-cx)/rx, (float64(y)+0.5-cy)/ry
			e := math.Sqrt(dx*dx + dy*dy)
			near := art.Smooth((dx + 0.3) / 0.9) // the near side is bright
			v := (math.Exp(-math.Pow((e-1)/0.05, 2)) + 0.4*math.Exp(-math.Pow((e-1)/0.2, 2))) * (0.25 + 0.75*near)
			if v > 0.004 {
				c.Light(x, y, art.Mix(blueHot, white, v*0.6), v)
			}
		}
	}
	return c.Additive(), int(cx), int(cy)
}

// playerColors are the controller indicator's player colors (P1-P8).
var playerColors = []art.RGBA{
	art.Hex(0xE8343F, 1), art.Hex(0x2F7BEA, 1), art.Hex(0x2DBE6C, 1), art.Hex(0xF2B630, 1),
	art.Hex(0xA05CF0, 1), art.Hex(0xFF8A2A, 1), art.Hex(0x22C7D8, 1), art.Hex(0xF25CA8, 1),
}

// tagIcon is the badge over a character who tags in: "AI" or "P1"-"P8",
// with a pointer under it (axis at the pointer's tip).
func tagIcon(label string, col art.RGBA) (image.Image, int, int) {
	const w, h = 132, 84
	c := art.NewCanvas(w, h)
	body := art.Para(14, 6, w-14-8, 58, 8)
	ptr := []logo.Pt{{X: w/2 - 12, Y: 57}, {X: w/2 + 12, Y: 57}, {X: w / 2, Y: 76}}
	m := union(c, body, ptr)
	c.PaintMask(art.Blur(m, w, h, 4), art.Solid(ink.Alpha(0.6)))
	c.PaintMask(m, art.Linear(0, 6, 0, 76, art.Mix(col, white, 0.25), col))
	c.PaintMask(art.ShapeMask(w, h, body, art.Reversed(art.Para(17, 9, w-17-8, 55, 8))), art.Solid(white.Alpha(0.5)))
	f := art.Face(fontXBI, 38)
	c.PaintMask(art.TextMask(w, h, f, label, w/2+2, 46, 0, 1), art.Solid(white))
	return img(c), w / 2, 77
}

func drawGofx() {
	for k := 0; k < 9; k++ {
		addGofx(5300+k, dizzyIcon(k), 36, 36, fmt.Sprintf("Dizzy effect %d (positioned, scaled and faded by data/dizzy.zss)", k),
			fmt.Sprintf("%d,0, 0,0, 1", 5300+k))
	}
	for k := 0; k < 3; k++ {
		im, x, y := shard(k)
		addGofx(5400+k, im, x, y, fmt.Sprintf("Guard break glass shard %d", k),
			fmt.Sprintf("%d,0, 0,0, 31, , AS256D0", 5400+k), "Interpolate Blend", fmt.Sprintf("%d,0, 0,0, 1, , AS8D248", 5400+k))
	}
	im, x, y := breakWave()
	addGofx(5410, im, x, y, "Guard break shockwave",
		"5410,0, 0,0, 15, , AS256D256, 1,1", "Interpolate Blend", "Interpolate Scale", "5410,0, 0,0, 1, , AS16D256, 1.5,1.5")
	labels := []string{"AI", "P1", "P2", "P3", "P4", "P5", "P6", "P7", "P8"}
	for i, l := range labels {
		col := gray
		if i > 0 {
			col = playerColors[i-1]
		}
		im, x, y := tagIcon(l, col)
		no := 5600 + i
		addGofx(no, im, x, y, "Tag switch icon "+l,
			fmt.Sprintf("%d,0, 0,0, 60, , AS256D0", no), fmt.Sprintf("%d,0, 0,0, 15, , AS256D0", no), "Interpolate Blend", fmt.Sprintf("%d,0, 0,0, 1, , AS16D240", no))
	}
}

func gofxAir() string {
	var b strings.Builder
	b.WriteString("; OMugen engine effects (generated by tools/lifebarsgen), called by the\n")
	b.WriteString("; engine's states with the GO prefix: dizzy 5300-5308, guard break\n")
	b.WriteString("; 5400-5402 and 5410, tag switch icons 5600 (AI) and 5601-5608 (P1-P8).\n")
	for _, a := range gofxAnims {
		fmt.Fprintf(&b, "\n%s\n[Begin Action %d]\n", a.lines[0], a.no)
		for _, l := range a.lines[1:] {
			b.WriteString(l + "\n")
		}
	}
	return b.String()
}

const gofxDef = `; OMugen engine effects (generated by tools/lifebarsgen): the dizzy, guard
; break and tag switch effects of the engine's own states (data/*.zss), kept
; apart from a game's fightfx and common.snd so they never replace them.
; Games can point Config.Fx in save/config.ini at their own version.

[Info]
prefix = GO             ; animations and sounds are called as GO <number>
fx.scale = 1
localcoord = 640, 360   ; drawn at twice the 320x240 size

[Files]
sff = gofx.sff
air = gofx.air
snd = gofx.snd
`

// dizzySound loops while a character is dizzy: chirps and little bells.
func dizzySound() wave {
	w := newWave(1.15)
	for i, t := range []float64{0, 0.28, 0.57, 0.86} {
		f := 2600 + 300*float64(i%2)
		chirp := newWave(0.12)
		var ph float64
		for j := range chirp {
			x := float64(j) / float64(len(chirp))
			ph += 2 * math.Pi * f * (1 + 0.35*math.Sin(x*math.Pi)) / rate
			chirp[j] = math.Sin(ph) * math.Sin(x*math.Pi)
		}
		w = w.add(chirp, t, 0.6)
		w = w.add(tone(0.25, 1568*float64(1+i%2), 1568*float64(1+i%2), 0.002, 0.08), t+0.13, 0.3)
	}
	return normalize(w[:int(1.15*rate)], 0.7)
}

// shatterSound is a guard break: a thump and breaking glass.
func shatterSound() wave {
	w := newWave(0.1)
	w = w.add(tone(0.4, 140, 45, 0.002, 0.1), 0, 1)
	r := rand.New(rand.NewSource(54))
	for i := 0; i < 26; i++ {
		t := r.Float64() * 0.6 * r.Float64()
		f := 2500 + r.Float64()*5000
		w = w.add(tone(0.3, f, f*0.98, 0.0005, 0.03+r.Float64()*0.05), t, 0.25)
		w = w.add(noise(0.08, 12000, 3000, 0.0003, 0.012, int64(i)), t, 0.35)
	}
	return normalize(reverb(w, 0.6, 0.2), 0.9)
}

func gofxSounds() []sound {
	return []sound{{5300, 0, dizzySound()}, {5400, 0, shatterSound()}}
}

// ---------------------------------------------------------------- glyphs

// Move list icons, 64 pixels high (wider for words), axis at the bottom
// left; the group numbers are the ones of [Glyphs] in the default motif.
var glyphSprites []sprite

func addGlyph(no int, c *art.Canvas) {
	for _, s := range glyphSprites {
		if s.g == no {
			panic(fmt.Sprintf("glyph %d,0 drawn twice", no))
		}
	}
	glyphSprites = append(glyphSprites, sprite{no, 0, 0, c.H, img(c)})
}

var buttonColors = map[string]art.RGBA{
	"A": art.Hex(0x3FA9F5, 1), "X": art.Hex(0x3FA9F5, 1),
	"B": art.Hex(0xF5B82E, 1), "Y": art.Hex(0xF5B82E, 1),
	"C": art.Hex(0xF0414E, 1), "Z": art.Hex(0xF0414E, 1),
	"D": art.Hex(0x3CC874, 1), "W": art.Hex(0x3CC874, 1),
}

// button is a round button with a label.
func button(label string, col art.RGBA) *art.Canvas {
	c := art.NewCanvas(64, 64)
	m := mask(c, circle(32, 32, 27))
	c.PaintMask(art.Blur(mask(c, circle(32, 34, 28)), 64, 64, 2), art.Solid(ink.Alpha(0.8)))
	c.PaintMask(m, art.Linear(0, 6, 0, 58, art.Mix(col, white, 0.3), art.Mix(col, ink, 0.25)))
	c.PaintMask(mask(c, circle(32, 32, 27), art.Reversed(circle(32, 32, 24.5))), art.Solid(white.Alpha(0.55)))
	size := 34.0
	if len(label) > 1 {
		size = 23
	}
	f := art.Face(fontXB, size)
	m = art.TextMask(64, 64, f, label, 32, 32+size*0.36, 0, 0)
	c.PaintMask(art.ScaleMask(art.Blur(m, 64, 64, 1), 3), art.Solid(ink.Alpha(0.6)))
	c.PaintMask(m, art.Solid(white))
	return c
}

// disc is the dark round plate behind direction and motion icons.
func disc() *art.Canvas {
	c := art.NewCanvas(64, 64)
	c.PaintMask(mask(c, circle(32, 32, 28)), art.Linear(0, 4, 0, 60, art.Hex(0x2A2A34, 1), art.Hex(0x0E0E14, 1)))
	c.PaintMask(mask(c, circle(32, 32, 28), art.Reversed(circle(32, 32, 26))), art.Solid(gray.Alpha(0.7)))
	return c
}

// dirArrow is a straight arrow through the center along angle a (0 =
// forward, pi/2 = down).
func dirArrow(c *art.Canvas, a float64, col art.RGBA) {
	s, co := math.Sincos(a)
	from := logo.Pt{X: 32 - 15*co, Y: 32 - 15*s}
	to := logo.Pt{X: 32 + 9*co, Y: 32 + 9*s}
	m := mask(c, append(stroke([]logo.Pt{from, to}, 7), head(logo.Pt{X: 32 + 12*co, Y: 32 + 12*s}, a, 22))...)
	c.PaintMask(m, art.Solid(col))
}

// arcArrow is a curved arrow around the center from angle a0 to a1.
func arcArrow(c *art.Canvas, a0, a1, r float64, col art.RGBA) {
	var pts []logo.Pt
	n := int(math.Abs(a1-a0)/0.08) + 2
	for i := 0; i <= n; i++ {
		a := a0 + (a1-a0)*float64(i)/float64(n)
		if i == n {
			a = a1 - (a1-a0)*0.08 // leave room for the head
		}
		pts = append(pts, logo.Pt{X: 32 + r*math.Cos(a), Y: 32 + r*math.Sin(a)})
	}
	dir := 1.0
	if a1 < a0 {
		dir = -1
	}
	tip := logo.Pt{X: 32 + r*math.Cos(a1), Y: 32 + r*math.Sin(a1)}
	m := mask(c, append(stroke(pts, 6), head(tip, a1+dir*math.Pi/2, 17))...)
	c.PaintMask(m, art.Solid(col))
}

// word is a dark tag with a short word.
func word(text string) *art.Canvas {
	f := art.Face(fontXB, 22)
	w := int(math.Max(64, math.Ceil(art.TextWidth(f, text, 0.5)+20)))
	c := art.NewCanvas(w, 64)
	plate := logo.RoundRect(2, 16, float64(w-2), 50, 8)
	c.PaintMask(mask(c, plate), art.Linear(0, 16, 0, 50, art.Hex(0x34343F, 1), art.Hex(0x14141A, 1)))
	c.PaintMask(mask(c, plate, art.Reversed(logo.RoundRect(4, 18, float64(w-4), 48, 6))), art.Solid(red))
	c.PaintMask(art.TextMask(w, 64, f, text, float64(w)/2, 41, 0, 0.5), art.Solid(white))
	return c
}

func drawGlyphs() {
	for no, l := range map[int]string{1: "A", 2: "B", 3: "C", 4: "D", 23: "W", 24: "X", 25: "Y", 26: "Z"} {
		addGlyph(no, button(l, buttonColors[l]))
	}
	addGlyph(51, button("S", art.Hex(0x5A5A66, 1)))
	addGlyph(52, button("M", art.Hex(0x5A5A66, 1)))
	addGlyph(53, button("P", art.Hex(0xFF8A2A, 1)))
	addGlyph(54, button("K", art.Hex(0xA05CF0, 1)))
	for no, l := range map[int]string{57: "LP", 58: "MP", 59: "HP"} {
		addGlyph(no, button(l, buttonColors[string("XYZ"[no-57])]))
	}
	for no, l := range map[int]string{60: "LK", 61: "MK", 62: "HK"} {
		addGlyph(no, button(l, buttonColors[string("ABC"[no-60])]))
	}
	addGlyph(63, button("3K", art.Hex(0xA05CF0, 1)))
	addGlyph(64, button("3P", art.Hex(0xFF8A2A, 1)))
	addGlyph(65, button("2K", art.Hex(0xA05CF0, 1)))
	addGlyph(66, button("2P", art.Hex(0xFF8A2A, 1)))

	// Directions (forward is right, as the move list faces right) and their
	// held versions (gold, 92-99: there is no 45, so not a fixed offset).
	holdGlyph := map[int]int{41: 92, 42: 93, 43: 94, 44: 95, 46: 96, 47: 97, 48: 98, 49: 99}
	dirs := map[int]float64{41: 3 * math.Pi / 4, 42: math.Pi / 2, 43: math.Pi / 4, 44: math.Pi, 46: 0,
		47: -3 * math.Pi / 4, 48: -math.Pi / 2, 49: -math.Pi / 4}
	for no, a := range dirs {
		c := disc()
		dirArrow(c, a, white)
		addGlyph(no, c)
		h := disc()
		h.PaintMask(mask(h, circle(32, 32, 28), art.Reversed(circle(32, 32, 24))), art.Solid(gold))
		dirArrow(h, a, goldHot)
		addGlyph(holdGlyph[no], h)
	}

	// Motions: arcs from the first direction to the last (angles grow
	// clockwise on screen: 0 forward, pi/2 down, pi back, 3pi/2 up).
	const F, D, B, U = 0.0, math.Pi / 2, math.Pi, 3 * math.Pi / 2
	motions := map[int][2]float64{
		100: {F, B},           // half circle back (F, D, B)
		101: {B, 2 * math.Pi}, // half circle forward, over the top (B, U, F)
		102: {B, F},           // half circle forward (B, D, F)
		103: {2 * math.Pi, B}, // half circle back, over the top (F, U, B)
		104: {F, D}, 105: {D, B}, 106: {B, U}, 107: {U, 2 * math.Pi},
		108: {B, D}, 109: {D, F}, 110: {2 * math.Pi, U}, 111: {U, B},
		112: {F, F + 2*math.Pi - 0.3}, 113: {B, B + 2*math.Pi - 0.3}, // full circles, clockwise
		114: {F + 2*math.Pi, F + 0.3}, 115: {B + 2*math.Pi, B + 0.3}, // counterclockwise
	}
	for no, m := range motions {
		c := disc()
		arcArrow(c, m[0], m[1], 17, white)
		addGlyph(no, c)
	}
	for no, a := range map[int]float64{116: 0, 117: math.Pi} {
		c := disc()
		s := 1.0
		if a != 0 {
			s = -1
		}
		for _, off := range []float64{-7, 7} {
			m := mask(c, stroke([]logo.Pt{{X: 32 + s*(off-6), Y: 22}, {X: 32 + s*(off+4), Y: 32}, {X: 32 + s*(off-6), Y: 42}}, 6)...)
			c.PaintMask(m, art.Solid(white))
		}
		addGlyph(no, c)
	}
	for no, sgn := range map[int]float64{118: 1, 119: -1} {
		// Dragon screw: a full turn that ends pointing forward (or back).
		c := disc()
		start := math.Pi / 2
		arcArrow(c, start, start+sgn*(2*math.Pi-0.4), 15, white)
		addGlyph(no, c)
	}

	for no, t := range map[int]string{121: "AIR", 122: "TAP", 123: "MAX", 124: "EX", 136: "CHARGE", 137: "HOLD",
		138: "RELEASE", 139: "MASH", 140: "CLOSE", 141: "FAR", 142: "AIR OK", 143: "THROW", 144: "COUNTER"} {
		addGlyph(no, word(t))
	}

	sym := func(no int, f func(c *art.Canvas)) {
		c := art.NewCanvas(64, 64)
		f(c)
		addGlyph(no, c)
	}
	sym(39, func(c *art.Canvas) { // +
		c.PaintMask(mask(c, stroke([]logo.Pt{{X: 20, Y: 32}, {X: 44, Y: 32}}, 8)[0], stroke([]logo.Pt{{X: 32, Y: 20}, {X: 32, Y: 44}}, 8)[0]), art.Solid(white))
	})
	sym(40, func(c *art.Canvas) { // ...
		c.PaintMask(mask(c, circle(16, 40, 4.5), circle(32, 40, 4.5), circle(48, 40, 4.5)), art.Solid(white))
	})
	sym(90, func(c *art.Canvas) { // tap the next button right away
		c.PaintMask(mask(c, append(stroke([]logo.Pt{{X: 12, Y: 32}, {X: 42, Y: 32}}, 6), head(logo.Pt{X: 46, Y: 32}, 0, 20))...), art.Solid(white))
	})
	sym(91, func(c *art.Canvas) { // continue into the next move
		arcArrow(c, math.Pi, 2*math.Pi-0.1, 16, gold)
	})
	sym(127, func(c *art.Canvas) { // in the air
		c.PaintMask(mask(c, stroke([]logo.Pt{{X: 18, Y: 40}, {X: 32, Y: 24}, {X: 46, Y: 40}}, 7)...), art.Solid(blueHot))
	})
	sym(128, func(c *art.Canvas) { // crouching
		c.PaintMask(mask(c, stroke([]logo.Pt{{X: 18, Y: 26}, {X: 32, Y: 40}, {X: 46, Y: 26}}, 7)...), art.Solid(blueHot))
		c.PaintMask(mask(c, stroke([]logo.Pt{{X: 16, Y: 48}, {X: 48, Y: 48}}, 5)...), art.Solid(blueHot))
	})
	sym(129, func(c *art.Canvas) { // close: arrows toward each other
		c.PaintMask(mask(c, head(logo.Pt{X: 24, Y: 32}, 0, 20), head(logo.Pt{X: 40, Y: 32}, math.Pi, 20)), art.Solid(white))
	})
	sym(130, func(c *art.Canvas) { // away: arrows apart
		c.PaintMask(mask(c, head(logo.Pt{X: 18, Y: 32}, math.Pi, 20), head(logo.Pt{X: 46, Y: 32}, 0, 20)), art.Solid(white))
	})
	sym(135, func(c *art.Canvas) { // small dot
		c.PaintMask(mask(c, circle(32, 40, 4)), art.Solid(white))
	})
}

// glyphSheet renders the move list icons, then the gofx sprites (the
// shockwave scaled down), on a dark background, for review.
func glyphSheet(path string) {
	const cell, cols = 110, 16
	list := append(append([]sprite(nil), glyphSprites...), gofxSprites...)
	rows := (len(list) + cols - 1) / cols
	dst := image.NewNRGBA(image.Rect(0, 0, cols*cell, rows*cell))
	bg := art.NewCanvas(dst.Rect.Dx(), dst.Rect.Dy())
	bg.Fill(art.Solid(art.Hex(0x1C1C24, 1)))
	draw.Draw(dst, dst.Bounds(), bg.Image(false), image.Point{}, draw.Src)
	for i, s := range list {
		src := s.img
		b := src.Bounds()
		k := math.Min(1, float64(cell-10)/math.Max(float64(b.Dx()), float64(b.Dy())))
		w, h := int(float64(b.Dx())*k), int(float64(b.Dy())*k)
		x, y := (i%cols)*cell+(cell-w)/2, (i/cols)*cell+(cell-h)/2
		xdraw.ApproxBiLinear.Scale(dst, image.Rect(x, y, x+w, y+h), src, b, draw.Over, nil)
	}
	art.WritePNG(path, dst)
	fmt.Println("  glyph sheet:", path)
}
