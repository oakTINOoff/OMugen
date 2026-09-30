package main

// contactSheet renders chosen frames side by side on a floor line, with
// their collision boxes, for reviewing the art (TEMPLATEGEN_SHEET=file.png).

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
)

func contactSheet(path string, anims []anim, pal color.Palette) {
	pick := map[int][]int{0: {0}, 20: {0, 3}, 11: {0}, 41: {0}, 130: {0}, 200: {1}, 210: {2}, 230: {1}, 240: {2},
		410: {1}, 440: {1}, 640: {1}, 1000: {2}, 3000: {3}, 5002: {0}, 5050: {0}, 5080: {1}, 181: {2}}
	order := []int{0, 20, 11, 41, 130, 200, 210, 230, 240, 410, 440, 640, 1000, 3000, 5002, 5050, 5080, 181}
	const cellW, cellH, cols = 380, 440, 6
	var cells []Frame
	var hits [][]box
	for _, no := range order {
		for _, a := range anims {
			if a.no != no {
				continue
			}
			for _, i := range pick[no] {
				f := a.frames[i]
				r := render(f.pose, 1, pal)
				cells = append(cells, r)
				hits = append(hits, append(hurtBoxes(r.parts), hitBoxes(r.parts, f.hit, 0)...))
			}
		}
	}
	rows := (len(cells) + cols - 1) / cols
	sheet := image.NewRGBA(image.Rect(0, 0, cols*cellW, rows*cellH))
	draw.Draw(sheet, sheet.Bounds(), image.NewUniform(color.RGBA{58, 62, 72, 255}), image.Point{}, draw.Src)
	for i, f := range cells {
		ox, oy := (i%cols)*cellW+cellW/2, (i/cols)*cellH+cellH-30
		for x := ox - cellW/2 + 10; x < ox+cellW/2-10; x++ {
			sheet.Set(x, oy, color.RGBA{120, 120, 130, 255})
		}
		b := f.img.Bounds()
		draw.Draw(sheet, b.Add(image.Pt(ox-f.axisX, oy-f.axisY)), f.img, image.Point{}, draw.Over)
		for j, bx := range hits[i] {
			c := color.RGBA{60, 140, 255, 255}
			if j >= 3 {
				c = color.RGBA{255, 60, 60, 255}
			}
			for x := bx.x0; x <= bx.x1; x++ {
				sheet.Set(ox+x, oy+bx.y0, c)
				sheet.Set(ox+x, oy+bx.y1, c)
			}
			for y := bx.y0; y <= bx.y1; y++ {
				sheet.Set(ox+bx.x0, oy+y, c)
				sheet.Set(ox+bx.x1, oy+y, c)
			}
		}
	}
	f, err := os.Create(path)
	check(err)
	defer f.Close()
	check(png.Encode(f, sheet))
}
