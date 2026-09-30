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
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/image/vector"

	"github.com/ikemen-engine/Ikemen-GO/tools/internal/logo"
)

// ---------------------------------------------------------------- raster

func fillImage(fill string, w, h int) image.Image {
	switch fill {
	case "white":
		return image.NewUniform(logo.White)
	case "bg":
		return image.NewUniform(logo.BgDark)
	case "edge":
		return image.NewUniform(logo.Edge)
	}
	// Vertical red gradient.
	g := image.NewRGBA(image.Rect(0, 0, 1, h))
	for y := 0; y < h; y++ {
		t := float64(y) / float64(h-1)
		lerp := func(a, b uint8) uint8 { return uint8(float64(a) + (float64(b)-float64(a))*t) }
		g.SetRGBA(0, y, color.RGBA{lerp(logo.RedTop.R, logo.RedBottom.R), lerp(logo.RedTop.G, logo.RedBottom.G), lerp(logo.RedTop.B, logo.RedBottom.B), 255})
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

func render(shapes []logo.Shape, srcW, srcH, outW, outH int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, outW, outH))
	sx, sy := float64(outW)/float64(srcW), float64(outH)/float64(srcH)
	for _, sh := range shapes {
		z := vector.NewRasterizer(outW, outH)
		for _, c := range sh.Contours {
			z.MoveTo(float32(c[0].X*sx), float32(c[0].Y*sy))
			for _, p := range c[1:] {
				z.LineTo(float32(p.X*sx), float32(p.Y*sy))
			}
			z.ClosePath()
		}
		z.DrawOp = draw.Over
		z.Draw(dst, dst.Bounds(), fillImage(sh.Fill, outW, outH), image.Point{})
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

func writeSVG(path string, shapes []logo.Shape, w, h int) {
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" width="%d" height="%d">`+"\n", w, h, w, h)
	fmt.Fprintf(&b, `  <defs><linearGradient id="red" x1="0" y1="0" x2="0" y2="1">`+
		`<stop offset="0" stop-color="#%02X%02X%02X"/><stop offset="1" stop-color="#%02X%02X%02X"/>`+
		`</linearGradient></defs>`+"\n", logo.RedTop.R, logo.RedTop.G, logo.RedTop.B, logo.RedBottom.R, logo.RedBottom.G, logo.RedBottom.B)
	fills := map[string]string{"red": "url(#red)", "white": "#F7F7F7", "bg": "#121216", "edge": "#3A060A"}
	for _, sh := range shapes {
		b.WriteString(`  <path fill-rule="nonzero" fill="` + fills[sh.Fill] + `" d="`)
		for _, c := range sh.Contours {
			fmt.Fprintf(&b, "M%.2f %.2f", c[0].X, c[0].Y)
			for _, p := range c[1:] {
				fmt.Fprintf(&b, "L%.2f %.2f", p.X, p.Y)
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

	icon := logo.IconShapes()
	var icoImgs []*image.RGBA
	for _, n := range []int{16, 24, 32, 48, 64, 128, 256} {
		icoImgs = append(icoImgs, render(icon, 256, 256, n, n))
	}
	writeICO(filepath.Join(dir, "omugen.ico"), icoImgs)
	for _, n := range []int{256, 96, 48} {
		writePNG(filepath.Join(dir, fmt.Sprintf("omugen_%d.png", n)), render(icon, 256, 256, n, n))
	}
	writeSVG(filepath.Join(src, "omugen_icon.svg"), icon, 256, 256)

	word, w, h := logo.WordmarkShapes()
	writePNG(filepath.Join(dir, "omugen_logo.png"), render(word, w, h, w, h))
	writeSVG(filepath.Join(src, "omugen_logo.svg"), word, w, h)
}
