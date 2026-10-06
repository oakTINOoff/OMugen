// Package art is a small float compositor used by OMugen's art generators:
// straight-alpha colors painted through coverage masks onto premultiplied
// RGBA canvases, plus blur for glows and Open Sans text.
package art

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

	"github.com/oakTINOoff/OMugen/tools/internal/logo"
)

type RGBA struct{ R, G, B, A float64 }

func Hex(v uint32, a float64) RGBA {
	return RGBA{float64(v>>16&0xFF) / 255, float64(v>>8&0xFF) / 255, float64(v&0xFF) / 255, a}
}

func (c RGBA) Alpha(a float64) RGBA { c.A *= a; return c }

func Mix(a, b RGBA, t float64) RGBA {
	t = Clamp01(t)
	return RGBA{a.R + (b.R-a.R)*t, a.G + (b.G-a.G)*t, a.B + (b.B-a.B)*t, a.A + (b.A-a.A)*t}
}

func Clamp01(v float64) float64 { return math.Max(0, math.Min(1, v)) }

func Smooth(t float64) float64 { t = Clamp01(t); return t * t * (3 - 2*t) }

type Paint func(x, y float64) RGBA

func Solid(c RGBA) Paint { return func(x, y float64) RGBA { return c } }

// linear is a gradient from c0 at (x0, y0) to c1 at (x1, y1).
func Linear(x0, y0, x1, y1 float64, c0, c1 RGBA) Paint {
	dx, dy := x1-x0, y1-y0
	l2 := dx*dx + dy*dy
	return func(x, y float64) RGBA {
		return Mix(c0, c1, ((x-x0)*dx+(y-y0)*dy)/l2)
	}
}

// canvas holds premultiplied RGBA in 0..1.
type Canvas struct {
	W, H int
	P    []float64
}

func NewCanvas(w, h int) *Canvas { return &Canvas{w, h, make([]float64, w*h*4)} }

func (c *Canvas) Over(x, y int, col RGBA, cov float64) {
	a := col.A * cov
	if a <= 0 || x < 0 || y < 0 || x >= c.W || y >= c.H {
		return
	}
	i := (y*c.W + x) * 4
	c.P[i] = col.R*a + c.P[i]*(1-a)
	c.P[i+1] = col.G*a + c.P[i+1]*(1-a)
	c.P[i+2] = col.B*a + c.P[i+2]*(1-a)
	c.P[i+3] = a + c.P[i+3]*(1-a)
}

// light adds color (additive), for glows. Alpha grows with the added light
// so glows also work on transparent sprites.
func (c *Canvas) Light(x, y int, col RGBA, amt float64) {
	a := col.A * amt
	if a <= 0 || x < 0 || y < 0 || x >= c.W || y >= c.H {
		return
	}
	i := (y*c.W + x) * 4
	c.P[i] += col.R * a
	c.P[i+1] += col.G * a
	c.P[i+2] += col.B * a
	c.P[i+3] = math.Min(1, c.P[i+3]+a*math.Max(col.R, math.Max(col.G, col.B)))
}

func (c *Canvas) Fill(p Paint) {
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			c.Over(x, y, p(float64(x)+0.5, float64(y)+0.5), 1)
		}
	}
}

func (c *Canvas) PaintMask(m []float64, p Paint) {
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			if v := m[y*c.W+x]; v > 0 {
				c.Over(x, y, p(float64(x)+0.5, float64(y)+0.5), v)
			}
		}
	}
}

func (c *Canvas) LightMask(m []float64, col RGBA, amt float64) {
	for i, v := range m {
		if v > 0 {
			c.Light(i%c.W, i/c.W, col, v*amt)
		}
	}
}

// shapeMask rasterizes closed contours (nonzero winding) into a coverage mask.
func ShapeMask(w, h int, contours ...[]logo.Pt) []float64 {
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

func Reversed(c []logo.Pt) []logo.Pt {
	out := make([]logo.Pt, len(c))
	for i, p := range c {
		out[len(c)-1-i] = p
	}
	return out
}

// ring is the outline of a rounded rectangle, width t, as two contours.
func Ring(x0, y0, x1, y1, r, t float64) [][]logo.Pt {
	return [][]logo.Pt{
		logo.RoundRect(x0, y0, x1, y1, r),
		Reversed(logo.RoundRect(x0+t, y0+t, x1-t, y1-t, math.Max(0, r-t))),
	}
}

// para is a parallelogram whose top edge is shifted right by slant.
func Para(x0, y0, x1, y1, slant float64) []logo.Pt {
	return []logo.Pt{{X: x0 + slant, Y: y0}, {X: x1 + slant, Y: y0}, {X: x1, Y: y1}, {X: x0, Y: y1}}
}

func Translate(cs [][]logo.Pt, dx, dy, s float64) [][]logo.Pt {
	out := make([][]logo.Pt, len(cs))
	for i, c := range cs {
		for _, p := range c {
			out[i] = append(out[i], logo.Pt{X: p.X*s + dx, Y: p.Y*s + dy})
		}
	}
	return out
}

// blur is three box blurs (close to a gaussian of sigma ~ r).
func Blur(m []float64, w, h int, r int) []float64 {
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

func ScaleMask(m []float64, k float64) []float64 {
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
	Check(err)
	f, err := opentype.Parse(data)
	Check(err)
	faces[file] = f
	return f
}

func Face(file string, size float64) font.Face {
	f, err := opentype.NewFace(loadFont(file), &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingNone})
	Check(err)
	return f
}

// textWidth measures s with extra tracking (pixels between letters).
func TextWidth(f font.Face, s string, tracking float64) float64 {
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
func TextMask(w, h int, f font.Face, s string, x, y float64, align int, tracking float64) []float64 {
	tw := TextWidth(f, s, tracking)
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
func (c *Canvas) Image(dither bool) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, c.W, c.H))
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			i := (y*c.W + x) * 4
			a := Clamp01(c.P[i+3])
			if a <= 0 {
				continue
			}
			d := 0.5
			if dither {
				d = (bayer4[(y&3)*4+(x&3)] + 0.5) / 16
			}
			q := func(v float64) uint8 {
				return uint8(math.Min(255, math.Max(0, math.Floor(Clamp01(v/a)*255+d))))
			}
			img.SetNRGBA(x, y, color.NRGBA{q(c.P[i]), q(c.P[i+1]), q(c.P[i+2]), uint8(math.Round(a * 255))})
		}
	}
	return img
}

// additive converts a light layer (drawn with trans = add): the color is the
// light itself and lit pixels are opaque, so it adds the same light whether
// or not the renderer multiplies by alpha.
func (c *Canvas) Additive() *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, c.W, c.H))
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			i := (y*c.W + x) * 4
			q := func(v float64) uint8 { return uint8(math.Round(Clamp01(v) * 255)) }
			r, g, b := q(c.P[i]), q(c.P[i+1]), q(c.P[i+2])
			if r|g|b == 0 {
				continue
			}
			img.SetNRGBA(x, y, color.NRGBA{r, g, b, 255})
		}
	}
	return img
}

func WritePNG(path string, img image.Image) {
	f, err := os.Create(path)
	Check(err)
	defer f.Close()
	Check(png.Encode(f, img))
}

func Check(err error) {
	if err != nil {
		panic(err)
	}
}
