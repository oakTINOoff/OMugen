// logogen draws the OMugen logo from vector shapes and writes:
//
//	external/icons/omugen.ico                  Windows icon (16-256 px)
//	external/icons/omugen_{256,96,48}.png      window icons
//	external/icons/omugen_logo.png             wordmark banner
//	external/icons/icon-src/omugen/*.svg       editable sources (same geometry)
//
// Run from the repo root: go run ./tools/logogen
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/image/vector"
)

// Palette.
var (
	redTop    = color.RGBA{0xFF, 0x3A, 0x3A, 0xFF}
	redBottom = color.RGBA{0x9E, 0x00, 0x0B, 0xFF}
	white     = color.RGBA{0xF7, 0xF7, 0xF7, 0xFF}
	bgDark    = color.RGBA{0x12, 0x12, 0x16, 0xFF}
	edge      = color.RGBA{0x3A, 0x06, 0x0A, 0xFF}
)

type pt struct{ x, y float64 }

// shape is a set of closed contours filled together (nonzero winding, so a
// reversed inner contour is a hole).
type shape struct {
	contours [][]pt
	fill     string // "red", "white", "bg", "edge"
}

// ---------------------------------------------------------------- geometry

// Letters on an 80-unit tall grid, stroke 20, chamfered corners.
var letters = map[rune][][]pt{
	'O': {
		{{10, 0}, {60, 0}, {70, 10}, {70, 70}, {60, 80}, {10, 80}, {0, 70}, {0, 10}},
		{{20, 20}, {20, 60}, {50, 60}, {50, 20}}, // hole (reversed)
	},
	'M': {{{0, 80}, {0, 0}, {20, 0}, {45, 36}, {70, 0}, {90, 0}, {90, 80}, {70, 80}, {70, 38}, {45, 72}, {20, 38}, {20, 80}}},
	'U': {{{0, 0}, {20, 0}, {20, 60}, {50, 60}, {50, 0}, {70, 0}, {70, 70}, {60, 80}, {10, 80}, {0, 70}}},
	'G': {{{10, 0}, {70, 0}, {70, 20}, {20, 20}, {20, 60}, {50, 60}, {50, 50}, {36, 50}, {36, 32}, {70, 32}, {70, 70}, {60, 80}, {10, 80}, {0, 70}, {0, 10}}},
	'E': {{{0, 0}, {66, 0}, {66, 18}, {20, 18}, {20, 31}, {56, 31}, {56, 49}, {20, 49}, {20, 62}, {66, 62}, {66, 80}, {0, 80}}},
	'N': {{{0, 80}, {0, 0}, {20, 0}, {50, 46}, {50, 0}, {70, 0}, {70, 80}, {50, 80}, {20, 34}, {20, 80}}},
}

var letterWidth = map[rune]float64{'O': 70, 'M': 90, 'U': 70, 'G': 70, 'E': 66, 'N': 70}

const italic = 0.22 // x shift per unit of height above the baseline

// place scales and italicizes a letter's contours.
func place(c [][]pt, ox, oy, s float64) [][]pt {
	out := make([][]pt, len(c))
	for i, contour := range c {
		for _, p := range contour {
			x := ox + (p.x+(80-p.y)*italic)*s
			y := oy + p.y*s
			out[i] = append(out[i], pt{x, y})
		}
	}
	return out
}

func circle(cx, cy, r float64, reverse bool) []pt {
	const n = 96
	var c []pt
	for i := 0; i < n; i++ {
		a := 2 * math.Pi * float64(i) / n
		if reverse {
			a = -a
		}
		c = append(c, pt{cx + r*math.Cos(a), cy + r*math.Sin(a)})
	}
	return c
}

func roundRect(x0, y0, x1, y1, r float64) []pt {
	var c []pt
	corner := func(cx, cy, a0 float64) {
		for i := 0; i <= 12; i++ {
			a := a0 + math.Pi/2*float64(i)/12
			c = append(c, pt{cx + r*math.Cos(a), cy + r*math.Sin(a)})
		}
	}
	corner(x1-r, y0+r, -math.Pi/2)
	corner(x1-r, y1-r, 0)
	corner(x0+r, y1-r, math.Pi/2)
	corner(x0+r, y0+r, math.Pi)
	return c
}

// slash is a thin band across the ring from lower-left to upper-right.
func slash(size, width float64) []pt {
	a := -math.Pi / 3 // 60 degrees up to the right
	dx, dy := math.Cos(a), math.Sin(a)
	nx, ny := -dy*width/2, dx*width/2
	c := size / 2
	l := size
	return []pt{
		{c - dx*l + nx, c - dy*l + ny}, {c + dx*l + nx, c + dy*l + ny},
		{c + dx*l - nx, c + dy*l - ny}, {c - dx*l - nx, c - dy*l - ny},
	}
}

// iconShapes is the app icon on a 256x256 canvas.
func iconShapes() []shape {
	const s = 256.0
	m := place(letters['M'], 0, 0, 1)
	// Center the M inside the ring.
	minX, maxX, minY, maxY := bounds(m)
	sc := 94 / (maxX - minX)
	ox := s/2 - (minX+(maxX-minX)/2)*sc
	oy := s/2 - (minY+(maxY-minY)/2)*sc
	m = place(letters['M'], 0, 0, sc)
	for _, c := range m {
		for i := range c {
			c[i].x += ox
			c[i].y += oy
		}
	}
	return []shape{
		{contours: [][]pt{roundRect(0, 0, s, s, 52)}, fill: "bg"},
		{contours: [][]pt{circle(s/2, s/2, 112, false), circle(s/2, s/2, 76, true)}, fill: "red"},
		{contours: [][]pt{slash(s, 11)}, fill: "bg"},
		{contours: m, fill: "white"},
	}
}

// wordmarkShapes is "OMUGEN" with a red O; returns shapes and canvas size.
func wordmarkShapes() ([]shape, int, int) {
	const sc, gap, pad = 2.4, 14.0, 40.0
	x := pad
	var shapes []shape
	for i, r := range "OMUGEN" {
		fill := "white"
		if i == 0 {
			fill = "red"
		}
		c := place(letters[r], x, pad, sc)
		shapes = append(shapes, shape{contours: c, fill: fill})
		x += (letterWidth[r] + gap) * sc
	}
	w := int(x - gap*sc + 80*italic*sc + pad)
	h := int(80*sc + 2*pad)
	return shapes, w, h
}

func bounds(cs [][]pt) (minX, maxX, minY, maxY float64) {
	minX, minY = math.Inf(1), math.Inf(1)
	maxX, maxY = math.Inf(-1), math.Inf(-1)
	for _, c := range cs {
		for _, p := range c {
			minX, maxX = math.Min(minX, p.x), math.Max(maxX, p.x)
			minY, maxY = math.Min(minY, p.y), math.Max(maxY, p.y)
		}
	}
	return
}

// ---------------------------------------------------------------- raster

func fillImage(fill string, w, h int) image.Image {
	switch fill {
	case "white":
		return image.NewUniform(white)
	case "bg":
		return image.NewUniform(bgDark)
	case "edge":
		return image.NewUniform(edge)
	}
	// Vertical red gradient.
	g := image.NewRGBA(image.Rect(0, 0, 1, h))
	for y := 0; y < h; y++ {
		t := float64(y) / float64(h-1)
		lerp := func(a, b uint8) uint8 { return uint8(float64(a) + (float64(b)-float64(a))*t) }
		g.SetRGBA(0, y, color.RGBA{lerp(redTop.R, redBottom.R), lerp(redTop.G, redBottom.G), lerp(redTop.B, redBottom.B), 255})
	}
	return &stretchX{g, w}
}

// stretchX repeats a 1-pixel-wide image horizontally.
type stretchX struct {
	img *image.RGBA
	w   int
}

func (s *stretchX) ColorModel() color.Model { return color.RGBAModel }
func (s *stretchX) Bounds() image.Rectangle {
	return image.Rect(0, 0, s.w, s.img.Bounds().Dy())
}
func (s *stretchX) At(x, y int) color.Color { return s.img.At(0, y) }

func render(shapes []shape, srcW, srcH, outW, outH int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, outW, outH))
	sx, sy := float64(outW)/float64(srcW), float64(outH)/float64(srcH)
	for _, sh := range shapes {
		z := vector.NewRasterizer(outW, outH)
		for _, c := range sh.contours {
			z.MoveTo(float32(c[0].x*sx), float32(c[0].y*sy))
			for _, p := range c[1:] {
				z.LineTo(float32(p.x*sx), float32(p.y*sy))
			}
			z.ClosePath()
		}
		z.DrawOp = draw.Over
		z.Draw(dst, dst.Bounds(), fillImage(sh.fill, outW, outH), image.Point{})
	}
	return dst
}

// ---------------------------------------------------------------- output

func writePNG(path string, img image.Image) {
	var b bytes.Buffer
	check(png.Encode(&b, img))
	check(os.WriteFile(path, b.Bytes(), 0o644))
	fmt.Println("wrote", path)
}

// icoDIB encodes one icon image as a 32-bit BGRA DIB with an AND mask, the
// classic format every Windows component understands.
func icoDIB(img *image.RGBA) []byte {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	var b bytes.Buffer
	le := func(v interface{}) { check(binary.Write(&b, binary.LittleEndian, v)) }
	maskStride := ((w + 31) / 32) * 4
	le(uint32(40))                   // biSize
	le(int32(w))                     // biWidth
	le(int32(h * 2))                 // biHeight: color + mask
	le(uint16(1))                    // biPlanes
	le(uint16(32))                   // biBitCount
	le(uint32(0))                    // BI_RGB
	le(uint32(w*h*4 + maskStride*h)) // biSizeImage
	le(int32(0))
	le(int32(0))
	le(uint32(0))
	le(uint32(0))
	for y := h - 1; y >= 0; y-- { // bottom-up
		for x := 0; x < w; x++ {
			c := img.RGBAAt(x, y)
			// Un-premultiply: image.RGBA is premultiplied, DIB alpha is straight.
			r, g, bl := c.R, c.G, c.B
			if c.A > 0 && c.A < 255 {
				r = uint8(uint16(c.R) * 255 / uint16(c.A))
				g = uint8(uint16(c.G) * 255 / uint16(c.A))
				bl = uint8(uint16(c.B) * 255 / uint16(c.A))
			}
			b.Write([]byte{bl, g, r, c.A})
		}
	}
	for y := h - 1; y >= 0; y-- { // AND mask: 1 = transparent
		row := make([]byte, maskStride)
		for x := 0; x < w; x++ {
			if img.RGBAAt(x, y).A == 0 {
				row[x/8] |= 0x80 >> (x % 8)
			}
		}
		b.Write(row)
	}
	return b.Bytes()
}

// writeICO packs DIB images, plus PNG for 256 px (supported since Vista).
func writeICO(path string, imgs []*image.RGBA) {
	var pngs [][]byte
	for _, img := range imgs {
		if img.Bounds().Dx() < 256 {
			pngs = append(pngs, icoDIB(img))
			continue
		}
		var b bytes.Buffer
		check(png.Encode(&b, img))
		pngs = append(pngs, b.Bytes())
	}
	var out bytes.Buffer
	le := func(v interface{}) { check(binary.Write(&out, binary.LittleEndian, v)) }
	le(uint16(0))
	le(uint16(1))
	le(uint16(len(imgs)))
	offset := uint32(6 + 16*len(imgs))
	for i, img := range imgs {
		d := img.Bounds().Dx()
		dim := uint8(d)
		if d >= 256 {
			dim = 0
		}
		out.Write([]byte{dim, dim, 0, 0})
		le(uint16(1))
		le(uint16(32))
		le(uint32(len(pngs[i])))
		le(offset)
		offset += uint32(len(pngs[i]))
	}
	for _, p := range pngs {
		out.Write(p)
	}
	check(os.WriteFile(path, out.Bytes(), 0o644))
	fmt.Println("wrote", path)
}

func writeSVG(path string, shapes []shape, w, h int) {
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" width="%d" height="%d">`+"\n", w, h, w, h)
	fmt.Fprintf(&b, `  <defs><linearGradient id="red" x1="0" y1="0" x2="0" y2="1">`+
		`<stop offset="0" stop-color="#%02X%02X%02X"/><stop offset="1" stop-color="#%02X%02X%02X"/>`+
		`</linearGradient></defs>`+"\n", redTop.R, redTop.G, redTop.B, redBottom.R, redBottom.G, redBottom.B)
	fills := map[string]string{"red": "url(#red)", "white": "#F7F7F7", "bg": "#121216", "edge": "#3A060A"}
	for _, sh := range shapes {
		b.WriteString(`  <path fill-rule="nonzero" fill="` + fills[sh.fill] + `" d="`)
		for _, c := range sh.contours {
			fmt.Fprintf(&b, "M%.2f %.2f", c[0].x, c[0].y)
			for _, p := range c[1:] {
				fmt.Fprintf(&b, "L%.2f %.2f", p.x, p.y)
			}
			b.WriteString("Z")
		}
		b.WriteString(`"/>` + "\n")
	}
	b.WriteString("</svg>\n")
	check(os.WriteFile(path, []byte(b.String()), 0o644))
	fmt.Println("wrote", path)
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	dir := filepath.Join("external", "icons")
	src := filepath.Join(dir, "icon-src", "omugen")
	check(os.MkdirAll(src, 0o755))

	icon := iconShapes()
	var icoImgs []*image.RGBA
	for _, n := range []int{16, 24, 32, 48, 64, 128, 256} {
		icoImgs = append(icoImgs, render(icon, 256, 256, n, n))
	}
	writeICO(filepath.Join(dir, "omugen.ico"), icoImgs)
	for _, n := range []int{256, 96, 48} {
		writePNG(filepath.Join(dir, fmt.Sprintf("omugen_%d.png", n)), render(icon, 256, 256, n, n))
	}
	writeSVG(filepath.Join(src, "omugen_icon.svg"), icon, 256, 256)

	word, w, h := wordmarkShapes()
	writePNG(filepath.Join(dir, "omugen_logo.png"), render(word, w, h, w, h))
	writeSVG(filepath.Join(src, "omugen_logo.svg"), word, w, h)
}
