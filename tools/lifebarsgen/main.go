// lifebarsgen makes OMugen's own lifebars: the fight screen art (life and
// power bars, portrait frames, timer, win icons, round and K.O.
// announcements), its sounds, and the common hit sparks (fightfx) and hit
// sounds (common.snd) MUGEN characters use. Everything is generated from
// code (shapes, gradients, Open Sans text, synthesized sound), so the
// lifebars have no third-party art or sound.
//
// Run from the repo root: go run ./tools/lifebarsgen
//
// It writes screenpack/lifebars: fight.def, fight.sff, fight.snd,
// fightfx.sff, fightfx.air, common.snd and fonts/. OMugen.exe embeds them as
// the lifebars of games whose own lifebars fail to load, and -newgame copies
// them into each new game (data/omugen/lifebars), ready to edit.
//
// It also writes screenpack/engine: the engine's own effects (gofx: dizzy,
// guard break, tag switch) and the move list icons (glyphs.sff).
//
// LIFEBARSGEN_PREVIEW=file.png also renders a mock fight screen, and
// LIFEBARSGEN_GLYPHS=file.png a sheet of the move list icons and effects.
package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/oakTINOoff/OMugen/src/sffwrite"
	"github.com/oakTINOoff/OMugen/tools/internal/art"
)

// The fight screen's coordinate space.
const W, H = 1920, 1080

// Layout of player 1's side; player 2's mirrors it around the center.
var (
	life           = barGeom{l: 680, h: 38, slant: 16, pad: 14}
	lifeX, lifeY   = 200, 50 // top-left corner of the life bar
	power          = barGeom{l: 430, h: 20, slant: 10, pad: 12}
	powerX         = 124
	powerY         = 1022
	gauge          = barGeom{l: 300, h: 10, slant: 5, pad: 8} // guard and stun bars
	guardY, stunY  = 996, 974
	face           = faceGeom{s: 136, cut: 26, pad: 14}
	faceX, faceY   = 34, 26
	timerX         = W / 2
	timerY         = 84
	nameX, nameY   = 206, 132
	winX, winY     = 870, 110
	winStep        = 26
	comboX, comboY = 60, 430
)

type sprite struct {
	g, n   int
	ax, ay int
	img    image.Image
}

var sprites []sprite

func add(g, n int, img image.Image, ax, ay int) {
	sprites = append(sprites, sprite{g, n, ax, ay, img})
}

func get(g, n int) sprite {
	for _, s := range sprites {
		if s.g == g && s.n == n {
			return s
		}
	}
	panic(fmt.Sprintf("no sprite %d,%d", g, n))
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "lifebarsgen:", err)
		os.Exit(1)
	}
}

// Life fill colors by remaining life: full, 50% and up, 25% and up, below.
var lifeColors = [4][2]art.RGBA{
	{goldHot, art.Hex(0xFFD860, 1)},
	{art.Hex(0xFFE27A, 1), art.Hex(0xFFAE24, 1)},
	{art.Hex(0xFFB054, 1), art.Hex(0xFF6A1E, 1)},
	{art.Hex(0xFF6A5A, 1), art.Hex(0xD0101E, 1)},
}

func drawSprites() {
	lax, lay := life.axis()
	for i, s := range sides {
		add(10, i, barBack(life, s), lax, lay)
	}
	add(11, 0, barTop(life), lax, lay)
	add(12, 0, barFill(life, white, art.Hex(0xFFC0B4, 1)), lax, lay) // damage
	for i, c := range lifeColors {
		add(13, i, barFill(life, c[0], c[1]), lax, lay)
	}

	pax, pay := power.axis()
	for i, s := range sides {
		add(20, i, barBack(power, s), pax, pay)
		add(23, i, barFill(power, s.hot, s.col), pax, pay)
	}
	add(21, 0, barTop(power), pax, pay)
	add(22, 0, barFill(power, white, gray), pax, pay)

	// Guard and stun bars (shown when GuardBreak / Dizzy are on).
	gax, gay := gauge.axis()
	for i, s := range sides {
		add(24, i, barBack(gauge, s), gax, gay)
	}
	add(25, 0, barTop(gauge), gax, gay)
	add(26, 0, barFill(gauge, white, blueHot), gax, gay)
	add(27, 0, barFill(gauge, goldHot, art.Hex(0xFF9A2A, 1)), gax, gay)
	add(28, 0, barFill(gauge, white, art.Hex(0xFFC0B4, 1)), gax, gay)

	fp := int(face.pad)
	for i, s := range sides {
		add(30, i, faceBack(face, s), fp, fp)
		add(31, i, faceTop(face, s), fp, fp)
	}

	tb, tx, ty := timerBack(112, 84, 14)
	add(40, 0, tb, tx, ty)

	wi, wx, wy := winIcon(gray, white)
	add(50, 0, wi, wx, wy)
	wi, wx, wy = winIcon(gold, goldHot)
	add(50, 1, wi, wx, wy)

	// Round announcements.
	for r := 1; r <= 9; r++ {
		im, x, y := announce(fmt.Sprintf("ROUND %d", r), 150, white, art.Hex(0xC8C8D2, 1), red)
		add(500, r, im, x, y)
	}
	im, x, y := announce("FINAL ROUND", 150, goldHot, gold, red)
	add(500, 10, im, x, y)
	im, x, y = announce("FIGHT!", 210, white, redHot, red)
	add(520, 0, im, x, y)
	im, x, y = announce("K.O.", 240, white, redHot, red)
	add(530, 0, im, x, y)
	im, x, y = announce("DOUBLE K.O.", 170, white, redHot, red)
	add(531, 0, im, x, y)
	im, x, y = announce("TIME OVER", 170, white, blueHot, blue)
	add(532, 0, im, x, y)
	im, x, y = announce("DRAW GAME", 170, white, art.Hex(0xC8C8D2, 1), blue)
	add(533, 0, im, x, y)
	im, x, y = announce("PERFECT", 150, goldHot, gold, gold)
	add(534, 0, im, x, y)
	for i, s := range sides {
		im, x, y = slash(1500, 120, s.col)
		add(590, i, im, x, y)
	}
}

func main() {
	out := filepath.Join("screenpack", "lifebars")
	check(os.MkdirAll(filepath.Join(out, "fonts"), 0o755))
	write := func(name string, data []byte) {
		check(os.WriteFile(filepath.Join(out, name), data, 0o644))
		fmt.Printf("  %s (%.1f KB)\n", filepath.ToSlash(filepath.Join(out, name)), float64(len(data))/1024)
	}
	crlf := func(s string) []byte { return []byte(strings.ReplaceAll(s, "\n", "\r\n")) }

	drawSprites()
	write("fight.sff", sff(sprites))
	write("fight.def", crlf(fightDef()))
	write("fight.snd", snd(fightSounds()))
	for _, f := range fontDefs {
		write(filepath.Join("fonts", f.file), crlf(fontDef(f.name, f.ttf, f.size)))
	}

	drawFx()
	write("fightfx.sff", sff(fxSprites))
	write("fightfx.air", crlf(fightfxAir()))
	write("common.snd", snd(commonSounds()))

	// The engine's own effects and move list icons (screenpack/engine).
	out = filepath.Join("screenpack", "engine")
	check(os.MkdirAll(filepath.Join(out, "gofx"), 0o755))
	drawGofx()
	write(filepath.Join("gofx", "gofx.def"), crlf(gofxDef))
	write(filepath.Join("gofx", "gofx.sff"), sff(gofxSprites))
	write(filepath.Join("gofx", "gofx.air"), crlf(gofxAir()))
	write(filepath.Join("gofx", "gofx.snd"), snd(gofxSounds()))
	drawGlyphs()
	sort.Slice(glyphSprites, func(i, j int) bool { return glyphSprites[i].g < glyphSprites[j].g })
	write("glyphs.sff", sff(glyphSprites))
	if path := os.Getenv("LIFEBARSGEN_GLYPHS"); path != "" {
		glyphSheet(path)
	}

	if path := os.Getenv("LIFEBARSGEN_PREVIEW"); path != "" {
		preview(path)
	}
}

func sff(list []sprite) []byte {
	var ss []sffwrite.Sprite
	for _, s := range list {
		ss = append(ss, sffwrite.Sprite{Group: uint16(s.g), Number: uint16(s.n), AxisX: int16(s.ax), AxisY: int16(s.ay), Image: s.img})
	}
	var b bytes.Buffer
	check(sffwrite.Write(&b, ss))
	return b.Bytes()
}

// ---------------------------------------------------------------- preview

// preview renders a mock fight screen: both sides at about 60% life, a
// round announcement and the combo counter.
func preview(path string) {
	dst := image.NewNRGBA(image.Rect(0, 0, W, H))
	bg := art.NewCanvas(W, H)
	bg.Fill(art.Linear(0, 0, 0, H, art.Hex(0x3A4A66, 1), art.Hex(0x7A6A5A, 1)))
	draw.Draw(dst, dst.Bounds(), bg.Image(true), image.Point{}, draw.Src)

	put := func(s sprite, x, y int, mirror bool, frac float64) {
		src := s.img
		ax := s.ax
		if frac < 1 {
			src = scaleX(src, ax, frac)
		}
		if mirror {
			src = flip(src)
			ax = src.Bounds().Dx() - ax
		}
		r := src.Bounds().Add(image.Pt(x-ax, y-s.ay))
		if s.g < 590 {
			draw.Draw(dst, r, src, src.Bounds().Min, draw.Over)
			return
		}
		// Light streaks are drawn with additive blending.
		b := src.Bounds()
		for yy := b.Min.Y; yy < b.Max.Y; yy++ {
			for xx := b.Min.X; xx < b.Max.X; xx++ {
				sr, sg, sb, _ := src.At(xx, yy).RGBA()
				px, py := r.Min.X+xx-b.Min.X, r.Min.Y+yy-b.Min.Y
				if !image.Pt(px, py).In(dst.Rect) {
					continue
				}
				d := dst.NRGBAAt(px, py)
				add := func(a uint8, v uint32) uint8 { return uint8(math.Min(255, float64(a)+float64(v>>8))) }
				dst.SetNRGBA(px, py, color.NRGBA{add(d.R, sr), add(d.G, sg), add(d.B, sb), 255})
			}
		}
	}
	for p := 0; p < 2; p++ {
		m := p == 1
		x := func(v int) int {
			if m {
				return W - v
			}
			return v
		}
		put(get(30, p), x(faceX), faceY, m, 1)
		put(get(31, p), x(faceX), faceY, m, 1)
		put(get(10, p), x(lifeX), lifeY, m, 1)
		put(get(12, 0), x(lifeX), lifeY, m, 0.72)
		put(get(13, 1+p), x(lifeX), lifeY, m, 0.62-0.3*float64(p))
		put(get(11, 0), x(lifeX), lifeY, m, 1)
		put(get(20, p), x(powerX), powerY, m, 1)
		put(get(23, p), x(powerX), powerY, m, 0.4+0.4*float64(p))
		put(get(21, 0), x(powerX), powerY, m, 1)
		put(get(50, p), x(winX), winY, m, 1)
	}
	put(get(40, 0), timerX, timerY, false, 1)
	put(get(590, 0), W/2, 520, false, 1)
	put(get(520, 0), W/2, 520, false, 1)
	art.WritePNG(path, dst)
	fmt.Println("  preview:", path)
}

// scaleX scales an image horizontally by k around column ax (nearest pixel).
func scaleX(src image.Image, ax int, k float64) image.Image {
	b := src.Bounds()
	dst := image.NewNRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			sx := ax + int(float64(x-ax)/k)
			if sx >= b.Min.X && sx < b.Max.X {
				dst.Set(x, y, src.At(sx, y))
			}
		}
	}
	return dst
}

func flip(src image.Image) image.Image {
	b := src.Bounds()
	dst := image.NewNRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			dst.Set(b.Max.X-1-(x-b.Min.X), y, src.At(x, y))
		}
	}
	return dst
}
