// Package logo holds the OMugen logo geometry (vector shapes), shared by the
// icon generator (tools/logogen) and the screenpack generator
// (tools/screenpackgen).
package logo

import (
	"image/color"
	"math"
)

// Palette.
var (
	RedTop    = color.RGBA{0xFF, 0x3A, 0x3A, 0xFF}
	RedBottom = color.RGBA{0x9E, 0x00, 0x0B, 0xFF}
	White     = color.RGBA{0xF7, 0xF7, 0xF7, 0xFF}
	BgDark    = color.RGBA{0x12, 0x12, 0x16, 0xFF}
	Edge      = color.RGBA{0x3A, 0x06, 0x0A, 0xFF}
)

type Pt struct{ X, Y float64 }

// Shape is a set of closed contours filled together (nonzero winding, so a
// reversed inner contour is a hole).
type Shape struct {
	Contours [][]Pt
	Fill     string // "red", "white", "bg", "edge"
}

// Letters on an 80-unit tall grid, stroke 20, chamfered corners.
var Letters = map[rune][][]Pt{
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

var LetterWidth = map[rune]float64{'O': 70, 'M': 90, 'U': 70, 'G': 70, 'E': 66, 'N': 70}

const Italic = 0.22 // x shift per unit of height above the baseline

// Place scales and italicizes a letter's contours.
func Place(c [][]Pt, ox, oy, s float64) [][]Pt {
	out := make([][]Pt, len(c))
	for i, contour := range c {
		for _, p := range contour {
			x := ox + (p.X+(80-p.Y)*Italic)*s
			y := oy + p.Y*s
			out[i] = append(out[i], Pt{x, y})
		}
	}
	return out
}

func Circle(cx, cy, r float64, reverse bool) []Pt {
	const n = 96
	var c []Pt
	for i := 0; i < n; i++ {
		a := 2 * math.Pi * float64(i) / n
		if reverse {
			a = -a
		}
		c = append(c, Pt{cx + r*math.Cos(a), cy + r*math.Sin(a)})
	}
	return c
}

func RoundRect(x0, y0, x1, y1, r float64) []Pt {
	var c []Pt
	corner := func(cx, cy, a0 float64) {
		for i := 0; i <= 12; i++ {
			a := a0 + math.Pi/2*float64(i)/12
			c = append(c, Pt{cx + r*math.Cos(a), cy + r*math.Sin(a)})
		}
	}
	corner(x1-r, y0+r, -math.Pi/2)
	corner(x1-r, y1-r, 0)
	corner(x0+r, y1-r, math.Pi/2)
	corner(x0+r, y0+r, math.Pi)
	return c
}

// Slash is a thin band across the ring from lower-left to upper-right.
func Slash(size, width float64) []Pt {
	a := -math.Pi / 3 // 60 degrees up to the right
	dx, dy := math.Cos(a), math.Sin(a)
	nx, ny := -dy*width/2, dx*width/2
	c := size / 2
	l := size
	return []Pt{
		{c - dx*l + nx, c - dy*l + ny}, {c + dx*l + nx, c + dy*l + ny},
		{c + dx*l - nx, c + dy*l - ny}, {c - dx*l - nx, c - dy*l - ny},
	}
}

// IconShapes is the app icon on a 256x256 canvas.
func IconShapes() []Shape {
	const s = 256.0
	m := Place(Letters['M'], 0, 0, 1)
	// Center the M inside the ring.
	minX, maxX, minY, maxY := Bounds(m)
	sc := 94 / (maxX - minX)
	ox := s/2 - (minX+(maxX-minX)/2)*sc
	oy := s/2 - (minY+(maxY-minY)/2)*sc
	m = Place(Letters['M'], 0, 0, sc)
	for _, c := range m {
		for i := range c {
			c[i].X += ox
			c[i].Y += oy
		}
	}
	return []Shape{
		{Contours: [][]Pt{RoundRect(0, 0, s, s, 52)}, Fill: "bg"},
		{Contours: [][]Pt{Circle(s/2, s/2, 112, false), Circle(s/2, s/2, 76, true)}, Fill: "red"},
		{Contours: [][]Pt{Slash(s, 11)}, Fill: "bg"},
		{Contours: m, Fill: "white"},
	}
}

// WordmarkShapes is "OMUGEN" with a red O; returns shapes and canvas size.
func WordmarkShapes() ([]Shape, int, int) {
	const sc, gap, pad = 2.4, 14.0, 40.0
	x := pad
	var shapes []Shape
	for i, r := range "OMUGEN" {
		fill := "white"
		if i == 0 {
			fill = "red"
		}
		c := Place(Letters[r], x, pad, sc)
		shapes = append(shapes, Shape{Contours: c, Fill: fill})
		x += (LetterWidth[r] + gap) * sc
	}
	w := int(x - gap*sc + 80*Italic*sc + pad)
	h := int(80*sc + 2*pad)
	return shapes, w, h
}

func Bounds(cs [][]Pt) (minX, maxX, minY, maxY float64) {
	minX, minY = math.Inf(1), math.Inf(1)
	maxX, maxY = math.Inf(-1), math.Inf(-1)
	for _, c := range cs {
		for _, p := range c {
			minX, maxX = math.Min(minX, p.X), math.Max(maxX, p.X)
			minY, maxY = math.Min(minY, p.Y), math.Max(maxY, p.Y)
		}
	}
	return
}
