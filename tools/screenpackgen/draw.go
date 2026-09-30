package main

// A small float compositor: straight-alpha colors painted through coverage
// masks onto premultiplied RGBA canvases, plus blur for glows.

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
	"golang.org/x/image/vector"

	"github.com/ikemen-engine/Ikemen-GO/tools/internal/logo"
)

type rgba struct{ r, g, b, a float64 }

func hex(v uint32, a float64) rgba {
	return rgba{float64(v>>16&0xFF) / 255, float64(v>>8&0xFF) / 255, float64(v&0xFF) / 255, a}
}

func (c rgba) alpha(a float64) rgba { c.a *= a; return c }

func mix(a, b rgba, t float64) rgba {
	t = clamp01(t)
	return rgba{a.r + (b.r-a.r)*t, a.g + (b.g-a.g)*t, a.b + (b.b-a.b)*t, a.a + (b.a-a.a)*t}
}

func clamp01(v float64) float64 { return math.Max(0, math.Min(1, v)) }

func smooth(t float64) float64 { t = clamp01(t); return t * t * (3 - 2*t) }

type paint func(x, y float64) rgba

func solid(c rgba) paint { return func(x, y float64) rgba { return c } }

// linear is a gradient from c0 at (x0, y0) to c1 at (x1, y1).
func linear(x0, y0, x1, y1 float64, c0, c1 rgba) paint {
	dx, dy := x1-x0, y1-y0
	l2 := dx*dx + dy*dy
	return func(x, y float64) rgba {
		return mix(c0, c1, ((x-x0)*dx+(y-y0)*dy)/l2)
	}
}

// canvas holds premultiplied RGBA in 0..1.
type canvas struct {
	w, h int
	p    []float64
}

func newCanvas(w, h int) *canvas { return &canvas{w, h, make([]float64, w*h*4)} }

func (c *canvas) over(x, y int, col rgba, cov float64) {
	a := col.a * cov
	if a <= 0 || x < 0 || y < 0 || x >= c.w || y >= c.h {
		return
	}
	i := (y*c.w + x) * 4
	c.p[i] = col.r*a + c.p[i]*(1-a)
	c.p[i+1] = col.g*a + c.p[i+1]*(1-a)
	c.p[i+2] = col.b*a + c.p[i+2]*(1-a)
	c.p[i+3] = a + c.p[i+3]*(1-a)
}

// light adds color (additive), for glows. Alpha grows with the added light
// so glows also work on transparent sprites.
func (c *canvas) light(x, y int, col rgba, amt float64) {
	a := col.a * amt
	if a <= 0 || x < 0 || y < 0 || x >= c.w || y >= c.h {
		return
	}
	i := (y*c.w + x) * 4
	c.p[i] += col.r * a
	c.p[i+1] += col.g * a
	c.p[i+2] += col.b * a
	c.p[i+3] = math.Min(1, c.p[i+3]+a*math.Max(col.r, math.Max(col.g, col.b)))
}

func (c *canvas) fill(p paint) {
	for y := 0; y < c.h; y++ {
		for x := 0; x < c.w; x++ {
			c.over(x, y, p(float64(x)+0.5, float64(y)+0.5), 1)
		}
	}
}

func (c *canvas) paintMask(m []float64, p paint) {
	for y := 0; y < c.h; y++ {
		for x := 0; x < c.w; x++ {
			if v := m[y*c.w+x]; v > 0 {
				c.over(x, y, p(float64(x)+0.5, float64(y)+0.5), v)
			}
		}
	}
}

func (c *canvas) lightMask(m []float64, col rgba, amt float64) {
	for i, v := range m {
		if v > 0 {
			c.light(i%c.w, i/c.w, col, v*amt)
		}
	}
}

// shapeMask rasterizes closed contours (nonzero winding) into a coverage mask.
func shapeMask(w, h int, contours ...[]logo.Pt) []float64 {
	z := vector.NewRasterizer(w, h)
	for _, c := range contours {
		if len(c) == 0 {
			continue
		}
		z.MoveTo(float32(c[0].X), float32(c[0].Y))
		for _, p := range c[1:] {
			z.LineTo(float32(p.X), float32(p.Y))
		}
		z.ClosePath()
	}
	a := image.NewAlpha(image.Rect(0, 0, w, h))
	z.Draw(a, a.Bounds(), image.Opaque, image.Point{})
	m := make([]float64, w*h)
	for i, v := range a.Pix {
		m[i] = float64(v) / 255
	}
	return m
}

func reversed(c []logo.Pt) []logo.Pt {
	out := make([]logo.Pt, len(c))
	for i, p := range c {
		out[len(c)-1-i] = p
	}
	return out
}

// ring is the outline of a rounded rectangle, width t, as two contours.
func ring(x0, y0, x1, y1, r, t float64) [][]logo.Pt {
	return [][]logo.Pt{
		logo.RoundRect(x0, y0, x1, y1, r),
		reversed(logo.RoundRect(x0+t, y0+t, x1-t, y1-t, math.Max(0, r-t))),
	}
}

// para is a parallelogram whose top edge is shifted right by slant.
func para(x0, y0, x1, y1, slant float64) []logo.Pt {
	return []logo.Pt{{X: x0 + slant, Y: y0}, {X: x1 + slant, Y: y0}, {X: x1, Y: y1}, {X: x0, Y: y1}}
}

func translate(cs [][]logo.Pt, dx, dy, s float64) [][]logo.Pt {
	out := make([][]logo.Pt, len(cs))
	for i, c := range cs {
		for _, p := range c {
			out[i] = append(out[i], logo.Pt{X: p.X*s + dx, Y: p.Y*s + dy})
		}
	}
	return out
}

// blur is three box blurs (close to a gaussian of sigma ~ r).
func blur(m []float64, w, h int, r int) []float64 {
	if r < 1 {
		return m
	}
	out := append([]float64(nil), m...)
	tmp := make([]float64, len(m))
	for pass := 0; pass < 3; pass++ {
		boxH(out, tmp, w, h, r)
		boxV(tmp, out, w, h, r)
	}
	return out
}

func boxH(src, dst []float64, w, h, r int) {
	inv := 1 / float64(2*r+1)
	for y := 0; y < h; y++ {
		row := src[y*w : (y+1)*w]
		var acc float64
		for x := -r; x <= r; x++ {
			acc += row[clampi(x, 0, w-1)]
		}
		for x := 0; x < w; x++ {
			dst[y*w+x] = acc * inv
			acc += row[clampi(x+r+1, 0, w-1)] - row[clampi(x-r, 0, w-1)]
		}
	}
}

func boxV(src, dst []float64, w, h, r int) {
	inv := 1 / float64(2*r+1)
	for x := 0; x < w; x++ {
		var acc float64
		for y := -r; y <= r; y++ {
			acc += src[clampi(y, 0, h-1)*w+x]
		}
		for y := 0; y < h; y++ {
			dst[y*w+x] = acc * inv
			acc += src[clampi(y+r+1, 0, h-1)*w+x] - src[clampi(y-r, 0, h-1)*w+x]
		}
	}
}

func clampi(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func scaleMask(m []float64, k float64) []float64 {
	out := make([]float64, len(m))
	for i, v := range m {
		out[i] = math.Min(1, v*k)
	}
	return out
}

// ---------------------------------------------------------------- text

var faces = map[string]*opentype.Font{}

func loadFont(file string) *opentype.Font {
	if f := faces[file]; f != nil {
		return f
	}
	data, err := os.ReadFile(file)
	check(err)
	f, err := opentype.Parse(data)
	check(err)
	faces[file] = f
	return f
}

func face(file string, size float64) font.Face {
	f, err := opentype.NewFace(loadFont(file), &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingNone})
	check(err)
	return f
}

// textWidth measures s with extra tracking (pixels between letters).
func textWidth(f font.Face, s string, tracking float64) float64 {
	var w fixed.Int26_6
	prev := rune(-1)
	n := 0
	for _, r := range s {
		if prev >= 0 {
			w += f.Kern(prev, r)
		}
		a, _ := f.GlyphAdvance(r)
		w += a
		prev = r
		n++
	}
	return float64(w)/64 + tracking*float64(n-1)
}

// textMask draws s with its baseline starting at (x, y); align -1 left,
// 0 center, 1 right of x.
func textMask(w, h int, f font.Face, s string, x, y float64, align int, tracking float64) []float64 {
	tw := textWidth(f, s, tracking)
	switch align {
	case 0:
		x -= tw / 2
	case 1:
		x -= tw
	}
	a := image.NewAlpha(image.Rect(0, 0, w, h))
	d := font.Drawer{Dst: a, Src: image.Opaque, Face: f}
	prev := rune(-1)
	for _, r := range s {
		if prev >= 0 {
			x += float64(f.Kern(prev, r)) / 64
		}
		d.Dot = fixed.Point26_6{X: fixed.Int26_6(x * 64), Y: fixed.Int26_6(y * 64)}
		d.DrawString(string(r))
		adv, _ := f.GlyphAdvance(r)
		x += float64(adv)/64 + tracking
		prev = r
	}
	m := make([]float64, w*h)
	for i, v := range a.Pix {
		m[i] = float64(v) / 255
	}
	return m
}

// ---------------------------------------------------------------- output

// bayer4 is a 4x4 ordered dither matrix, used to hide banding in dark
// gradients.
var bayer4 = [16]float64{0, 8, 2, 10, 12, 4, 14, 6, 3, 11, 1, 9, 15, 7, 13, 5}

// image converts to straight alpha. dither adds ordered noise (in 8-bit
// steps) to opaque backgrounds.
func (c *canvas) image(dither bool) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, c.w, c.h))
	for y := 0; y < c.h; y++ {
		for x := 0; x < c.w; x++ {
			i := (y*c.w + x) * 4
			a := clamp01(c.p[i+3])
			if a <= 0 {
				continue
			}
			d := 0.5
			if dither {
				d = (bayer4[(y&3)*4+(x&3)] + 0.5) / 16
			}
			q := func(v float64) uint8 {
				return uint8(math.Min(255, math.Max(0, math.Floor(clamp01(v/a)*255+d))))
			}
			img.SetNRGBA(x, y, color.NRGBA{q(c.p[i]), q(c.p[i+1]), q(c.p[i+2]), uint8(math.Round(a * 255))})
		}
	}
	return img
}

// additive converts a light layer (drawn with trans = add): the color is the
// light itself and lit pixels are opaque, so it adds the same light whether
// or not the renderer multiplies by alpha.
func (c *canvas) additive() *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, c.w, c.h))
	for y := 0; y < c.h; y++ {
		for x := 0; x < c.w; x++ {
			i := (y*c.w + x) * 4
			q := func(v float64) uint8 { return uint8(math.Round(clamp01(v) * 255)) }
			r, g, b := q(c.p[i]), q(c.p[i+1]), q(c.p[i+2])
			if r|g|b == 0 {
				continue
			}
			img.SetNRGBA(x, y, color.NRGBA{r, g, b, 255})
		}
	}
	return img
}

func writePNG(path string, img image.Image) {
	f, err := os.Create(path)
	check(err)
	defer f.Close()
	check(png.Encode(f, img))
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
