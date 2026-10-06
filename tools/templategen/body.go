package main

// How Proto is drawn: each limb, the torso and the head are one piece with
// one outline (no lines between the segments of a limb), built from tapered
// capsules and ellipses, with seams where armor sits on the suit, five-tone
// cel shading, a rim light and darker far-side limbs. Everything is painted
// straight into palette indices, so the character switches palettes like any
// MUGEN character.

import (
	"image"
	"image/color"
	"math"
)

// Palette index layout shared by every palette: the outline, then five tones
// per material (deep shadow, shadow, mid, light, highlight) and three for
// the glow (dim, mid, hot).
const (
	idxOutline = 1
	matBody    = 2  // 2-6: the dark suit
	matPlate   = 7  // 7-11: armor
	matAccent  = 12 // 12-16: gloves, boots, headband, sash
	matGlow    = 17 // 17-19: visor, chest core, light strips
)

type partKind int

const (
	capsule partKind = iota // segment a-b, radius ra..rb, plus a bulge in the middle
	ellipse                 // center a, radius ra along the axis b (a unit vector), rb across
	plate                   // convex polygon pts, shaded flat with a bevel of width ra
)

type part struct {
	kind   partKind
	a, b   vec
	ra, rb float64
	bulge  float64
	mat    int
	flat   bool  // glow: no shading
	gloss  bool  // armor: gets the highlight tone
	seam   bool  // a thin dark line where it covers the parts under it
	pts    []vec // plate corners
}

func capsuleP(a, b vec, ra, rb float64, mat int) part {
	return part{kind: capsule, a: a, b: b, ra: ra, rb: rb, mat: mat}
}

func ellipseP(c, axis vec, ra, rb float64, mat int) part {
	return part{kind: ellipse, a: c, b: unit(axis), ra: ra, rb: rb, mat: mat}
}

func circleP(c vec, r float64, mat int) part { return ellipseP(c, vec{1, 0}, r, r, mat) }

// plateP is a flat armor plate (or a body shape) through the corners pts,
// in order; bevel is how far from the edge the shading curves.
func plateP(pts []vec, bevel float64, mat int) part {
	var area float64
	for i, p := range pts {
		q := pts[(i+1)%len(pts)]
		area += p.x*q.y - q.x*p.y
	}
	ps := append([]vec(nil), pts...)
	if area < 0 {
		for i, j := 0, len(ps)-1; i < j; i, j = i+1, j-1 {
			ps[i], ps[j] = ps[j], ps[i]
		}
	}
	return part{kind: plate, pts: ps, ra: bevel, mat: mat}
}

// plateDist is the signed distance from p to a convex plate (positive
// outside) and the outward normal of the nearest edge.
func plateDist(pts []vec, p vec) (float64, vec) {
	best, bn := math.Inf(-1), vec{}
	for i, a := range pts {
		b := pts[(i+1)%len(pts)]
		e := b.sub(a)
		l := e.length()
		if l == 0 {
			continue
		}
		n := vec{e.y / l, -e.x / l}
		if d := p.sub(a).dot(n); d > best {
			best, bn = d, n
		}
	}
	return best, bn
}

func (pt part) glossy() part          { pt.gloss = true; return pt }
func (pt part) seamed() part          { pt.seam = true; return pt }
func (pt part) bulged(b float64) part { pt.bulge = b; return pt }

func glowP(a, b vec, r float64) part {
	pt := capsuleP(a, b, r, r, matGlow)
	pt.flat = true
	return pt
}

func glowDot(c vec, r float64) part {
	pt := circleP(c, r, matGlow)
	pt.flat = true
	return pt
}

func unit(v vec) vec {
	if l := v.length(); l > 0 {
		return v.mul(1 / l)
	}
	return vec{0, 1}
}

// sample returns whether p is inside the part (grown by grow pixels) and a
// normal-like vector for shading (length <= 1 inside).
func (pt part) sample(p vec, grow float64) (bool, vec) {
	switch pt.kind {
	case plate:
		d, n := plateDist(pt.pts, p)
		if d > grow {
			return false, vec{}
		}
		// Flat in the middle, curving away within the bevel.
		k := 0.0
		if depth := -d; depth < pt.ra {
			k = 1 - math.Max(0, depth)/pt.ra
		}
		return true, n.mul(0.95 * k)
	case ellipse:
		d := p.sub(pt.a)
		u := pt.b
		v := vec{-u.y, u.x}
		lu, lv := d.dot(u), d.dot(v)
		gu, gv := lu/(pt.ra+grow), lv/(pt.rb+grow)
		if gu*gu+gv*gv > 1 {
			return false, vec{}
		}
		return true, u.mul(lu / math.Max(pt.ra, 1)).add(v.mul(lv / math.Max(pt.rb, 1)))
	default:
		ab := pt.b.sub(pt.a)
		l2 := ab.dot(ab)
		t := 0.0
		if l2 > 0 {
			t = math.Max(0, math.Min(1, p.sub(pt.a).dot(ab)/l2))
		}
		c := pt.a.add(ab.mul(t))
		r := lerp(pt.ra, pt.rb, t) + pt.bulge*math.Sin(math.Pi*t)
		d := p.sub(c)
		if d.length() > r+grow {
			return false, vec{}
		}
		return true, d.mul(1 / math.Max(r, 1))
	}
}

func (pt part) bounds(grow float64) (x0, y0, x1, y1 float64) {
	if pt.kind == plate {
		x0, y0, x1, y1 = math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
		for _, q := range pt.pts {
			x0, y0, x1, y1 = math.Min(x0, q.x), math.Min(y0, q.y), math.Max(x1, q.x), math.Max(y1, q.y)
		}
		return x0 - grow, y0 - grow, x1 + grow, y1 + grow
	}
	r := math.Max(pt.ra, pt.rb) + pt.bulge + grow
	if pt.kind == ellipse {
		return pt.a.x - r, pt.a.y - r, pt.a.x + r, pt.a.y + r
	}
	return math.Min(pt.a.x, pt.b.x) - r, math.Min(pt.a.y, pt.b.y) - r, math.Max(pt.a.x, pt.b.x) + r, math.Max(pt.a.y, pt.b.y) + r
}

// piece is a body part drawn as one shape with one outline (an arm, a leg,
// the torso...). Its parts are listed bottom to top.
type piece struct {
	parts []part
	far   bool // the far side of the body: one tone darker
}

// bodyParts lists the pieces of a pose in drawing order (back to front), and
// named groups used for collision boxes.
type bodyParts struct {
	draw                          []piece
	head, torso, armN, armF, legs []part
	fistN, fistF, footN, footF    []part
}

func (p Pose) parts() bodyParts {
	s := p.skeleton()
	var bp bodyParts
	at := func(f, side float64) vec { return s.pelvis.add(s.up.mul(torsoLen * f)).add(s.fwd.mul(side)) }

	// tp maps torso-space corners (side, height as a fraction of the torso).
	tp := func(c ...[2]float64) []vec {
		out := make([]vec, len(c))
		for i, q := range c {
			out[i] = at(q[1], q[0])
		}
		return out
	}

	// local maps corners (along u, along v) around the point o.
	local := func(o, u, v vec) func(c ...[2]float64) []vec {
		return func(c ...[2]float64) []vec {
			out := make([]vec, len(c))
			for i, q := range c {
				out[i] = o.add(u.mul(q[0])).add(v.mul(q[1]))
			}
			return out
		}
	}
	// Arm: upper arm, pauldron, gauntlet (covering the elbow) with a light
	// strip, wrist cuff and a block-shaped glove.
	arm := func(sh, el, ha vec) (all, fist []part) {
		ua, fa := unit(el.sub(sh)), unit(ha.sub(el))
		fv := vec{-fa.y, fa.x}
		lf := local(ha, fa, fv)
		glove := plateP(lf([2]float64{-7, -11}, [2]float64{1, -14.5}, [2]float64{10, -13}, [2]float64{14, -7},
			[2]float64{14, 7}, [2]float64{10, 13}, [2]float64{1, 14.5}, [2]float64{-7, 11}), 7, matAccent).glossy().seamed()
		return []part{
			capsuleP(sh, el, 15.5, 12.5, matBody).bulged(3.5),
			ellipseP(sh.add(ua.mul(6)).add(s.up.mul(3)), ua, 22, 13.5, matPlate).glossy().seamed(),
			capsuleP(el.sub(fa.mul(3)), ha.sub(fa.mul(7)), 12.5, 15, matPlate).glossy().seamed(),
			glowP(el.add(fa.mul(16)), ha.sub(fa.mul(16)), 2.2),
			capsuleP(ha.sub(fa.mul(9)), ha.sub(fa.mul(4)), 15.5, 15.5, matAccent).seamed(),
			glove,
		}, []part{glove}
	}
	// Leg: thigh, shin guard with a light strip, knee guard, ankle cuff, boot.
	leg := func(hip, kn, an, toe vec) (all, foot []part) {
		sd := unit(an.sub(kn))
		shin := capsuleP(kn.add(sd.mul(2)), an.sub(sd.mul(6)), 15, 12.5, matPlate).glossy().seamed()
		lb := local(an, unit(toe.sub(an)), sd.mul(-1))
		boot := plateP(lb([2]float64{-11, 13}, [2]float64{14, 9}, [2]float64{34, 3}, [2]float64{40, -4},
			[2]float64{37, -12}, [2]float64{-12, -12}, [2]float64{-15, -2}), 7, matAccent).glossy().seamed()
		sole := plateP(lb([2]float64{-12, -12}, [2]float64{37, -12}, [2]float64{35.5, -7.5}, [2]float64{-12.5, -7.5}), 2, matBody).seamed()
		return []part{
			capsuleP(hip, kn, 22, 15.5, matBody).bulged(3.5),
			shin,
			glowP(kn.add(sd.mul(24)), an.sub(sd.mul(22)), 2.2),
			ellipseP(kn.add(sd.mul(3)), sd, 17, 13, matPlate).glossy().seamed(),
			capsuleP(an.sub(sd.mul(6)), an, 12.5, 12.5, matAccent).seamed(),
			boot,
			sole,
		}, []part{shin, boot}
	}
	var armF, armN, legF, legN []part
	armF, bp.fistF = arm(s.shF, s.elF, s.haF)
	armN, bp.fistN = arm(s.shN, s.elN, s.haN)
	legF, bp.footF = leg(s.hipF, s.knF, s.anF, s.toeF)
	legN, bp.footN = leg(s.hipN, s.knN, s.anN, s.toeN)
	bp.armF, bp.armN = armF, armN
	bp.legs = append(append([]part{}, legF...), legN...)

	// Torso: a V-shaped suit with an armored chest, a glowing core and a
	// sash. Corners are (side, height): side + forward, height up the torso.
	bp.torso = []part{
		capsuleP(s.hipF, s.hipN, 21, 21, matBody),
		capsuleP(s.neck, s.neck.add(s.hup.mul(neckLen)), 12, 12, matBody),
		plateP(tp([2]float64{-19, 0.04}, [2]float64{22, 0.04}, [2]float64{27, 0.32}, [2]float64{33, 0.6},
			[2]float64{33, 0.84}, [2]float64{22, 0.99}, [2]float64{-17, 0.99}, [2]float64{-27, 0.86},
			[2]float64{-27, 0.56}, [2]float64{-21, 0.28}), 24, matBody),
		plateP(tp([2]float64{-18, 0.56}, [2]float64{29, 0.55}, [2]float64{36, 0.7}, [2]float64{34, 0.86},
			[2]float64{20, 0.97}, [2]float64{-14, 0.96}, [2]float64{-24, 0.82}), 9, matPlate).glossy().seamed(),
		plateP(tp([2]float64{-12, 0.36}, [2]float64{22, 0.36}, [2]float64{26, 0.5}, [2]float64{-16, 0.5}), 5, matBody).seamed(), // abs plate
		glowDot(at(0.74, 30), 6),
		glowP(at(0.62, 31), at(0.62, 16), 1.8),
		capsuleP(at(0.1, -22), at(0.1, 24), 9.5, 9.5, matAccent).seamed(),
		ellipseP(at(0.1, 18), s.up, 10, 8, matAccent).glossy().seamed(),
	}
	h, hu, hf := s.head, s.hup, s.hfwd
	bp.head = []part{
		ellipseP(h.add(hu.mul(1)), hu, headR+3, headR+1, matPlate).glossy(), // helmet
		plateP(local(h, hf, hu.mul(-1))([2]float64{4, 2}, [2]float64{29, 1}, [2]float64{27, 13}, [2]float64{17, 23}, [2]float64{3, 22}, [2]float64{-2, 11}), 6, matBody).seamed(), // jaw guard
		glowP(h.add(hf.mul(5)).add(hu.mul(3)), h.add(hf.mul(headR-2)).add(hu.mul(5)), 5.5),                                                                                        // visor
		capsuleP(h.sub(hf.mul(headR-2)).add(hu.mul(12)), h.add(hf.mul(headR-7)).add(hu.mul(16)), 5, 4.5, matAccent).seamed(),                                                      // headband
	}
	// Headband and sash tails sway with the pose's wave.
	back := h.sub(hf.mul(headR * 0.95)).add(hu.mul(headR * 0.4))
	w := math.Sin(p.Wave)
	tails := []part{
		capsuleP(back, back.sub(hf.mul(38)).sub(hu.mul(14+6*w)), 6, 3, matAccent),
		capsuleP(back.sub(hu.mul(4)), back.sub(hf.mul(26)).sub(hu.mul(28+5*math.Sin(p.Wave+1.3))), 5, 3, matAccent),
	}
	knot := at(0.1, 18)
	down := s.up.mul(-1)
	sash := []part{
		capsuleP(knot, knot.add(down.mul(44)).add(s.fwd.mul(4+5*w)), 6, 3.5, matAccent).seamed(),
		capsuleP(knot, knot.add(down.mul(36)).add(s.fwd.mul(12+4*math.Sin(p.Wave+1.1))), 5.5, 3, matAccent).seamed(),
	}

	add := func(far bool, ps []part) {
		bp.draw = append(bp.draw, piece{parts: append([]part(nil), ps...), far: far})
	}
	if !p.FarArmFront {
		add(true, armF)
	}
	if !p.FarLegFront {
		add(true, legF)
	}
	add(false, tails)
	add(false, bp.torso)
	add(false, bp.head)
	if p.FarLegFront {
		add(true, legF)
	}
	add(false, legN)
	add(false, sash)
	if p.FarArmFront {
		add(true, armF)
	}
	add(false, armN)
	return bp
}

// groundShift is the vertical shift that puts the lowest point on y = 0.
func groundShift(pcs []piece) float64 {
	maxY := math.Inf(-1)
	for _, pc := range pcs {
		for _, pt := range pc.parts {
			_, _, _, y1 := pt.bounds(0)
			maxY = math.Max(maxY, y1)
		}
	}
	return -maxY
}

func shiftParts(ps []part, dy float64) {
	for i := range ps {
		ps[i].a.y += dy
		if ps[i].kind == capsule {
			ps[i].b.y += dy
		}
		if ps[i].kind == plate {
			// A new slice: copies of a part share the old one.
			moved := make([]vec, len(ps[i].pts))
			for j, q := range ps[i].pts {
				moved[j] = vec{q.x, q.y + dy}
			}
			ps[i].pts = moved
		}
	}
}

// light for the cel shading: from the upper front, towards the viewer.
var light = struct{ x, y, z float64 }{0.5, -0.68, 0.54}

// Frame is a rendered pose: a paletted image and its axis (the ground point
// under the pelvis origin).
type Frame struct {
	img       *image.Paletted
	axisX     int
	axisY     int
	parts     bodyParts
	dy, scale float64
	x0, y0    int
}

// render draws a pose at the given scale, cropped to its pixels.
func render(p Pose, scale float64, pal color.Palette) Frame {
	bp := p.parts()
	dy := 0.0
	if p.Ground {
		dy = groundShift(bp.draw)
	}
	for i := range bp.draw {
		shiftParts(bp.draw[i].parts, dy)
	}
	for _, g := range [][]part{bp.head, bp.torso, bp.armN, bp.armF, bp.legs, bp.fistN, bp.fistF, bp.footN, bp.footF} {
		shiftParts(g, dy)
	}
	// Canvas in scaled pixels; origin (0,0) of pose space at (ox, oy).
	const W, H, ox, oy = 700, 640, 350, 560
	img := image.NewPaletted(image.Rect(0, 0, W, H), pal)
	for _, pc := range bp.draw {
		paintPiece(img, pc, scale, ox, oy)
	}
	// Crop to content.
	minX, minY, maxX, maxY := W, H, -1, -1
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			if img.Pix[y*img.Stride+x] != 0 {
				minX, minY = min(minX, x), min(minY, y)
				maxX, maxY = max(maxX, x), max(maxY, y)
			}
		}
	}
	if maxX < 0 {
		minX, minY, maxX, maxY = 0, 0, 0, 0
	}
	crop := image.NewPaletted(image.Rect(0, 0, maxX-minX+1, maxY-minY+1), pal)
	for y := minY; y <= maxY; y++ {
		copy(crop.Pix[(y-minY)*crop.Stride:], img.Pix[y*img.Stride+minX:y*img.Stride+maxX+1])
	}
	return Frame{img: crop, axisX: ox - minX, axisY: oy - minY, parts: bp, scale: scale, x0: minX, y0: minY}
}

// paintPiece draws a piece: the union of its parts with one outline around
// it, seams where a seamed part covers the parts under it, five-tone
// shading and a rim light on the edges facing away from the light.
func paintPiece(img *image.Paletted, pc piece, scale float64, ox, oy int) {
	if len(pc.parts) == 0 {
		return
	}
	fx0, fy0, fx1, fy1 := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, pt := range pc.parts {
		a, b, c, d := pt.bounds(outline)
		fx0, fy0, fx1, fy1 = math.Min(fx0, a), math.Min(fy0, b), math.Max(fx1, c), math.Max(fy1, d)
	}
	bnd := img.Bounds()
	px0 := max(bnd.Min.X, int(math.Floor(fx0*scale))+ox-1)
	px1 := min(bnd.Max.X-1, int(math.Ceil(fx1*scale))+ox+1)
	py0 := max(bnd.Min.Y, int(math.Floor(fy0*scale))+oy-1)
	py1 := min(bnd.Max.Y-1, int(math.Ceil(fy1*scale))+oy+1)
	w, h := px1-px0+1, py1-py0+1
	if w <= 0 || h <= 0 {
		return
	}
	const outside, edge = -1, -2
	owner := make([]int, w*h)
	nrm := make([]vec, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			k := y*w + x
			p := vec{(float64(x+px0-ox) + 0.5) / scale, (float64(y+py0-oy) + 0.5) / scale}
			owner[k] = outside
			for i := len(pc.parts) - 1; i >= 0; i-- {
				if in, n := pc.parts[i].sample(p, 0); in {
					owner[k], nrm[k] = i, n
					break
				}
			}
			if owner[k] == outside {
				for _, pt := range pc.parts {
					if in, _ := pt.sample(p, outline); in {
						owner[k] = edge
						break
					}
				}
			}
		}
	}
	at := func(x, y int) int {
		if x < 0 || y < 0 || x >= w || y >= h {
			return outside
		}
		return owner[y*w+x]
	}
	ll := math.Hypot(light.x, light.y)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			o := owner[y*w+x]
			if o == outside {
				continue
			}
			idx := uint8(idxOutline)
			if o >= 0 {
				pt := pc.parts[o]
				seam := false
				if pt.seam {
					for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
						if n := at(x+d[0], y+d[1]); n >= 0 && n < o && pc.parts[n].mat != matGlow {
							seam = true
							break
						}
					}
				}
				if !seam {
					rim := false
					for _, d := range [8][2]int{{2, 0}, {-2, 0}, {0, 2}, {0, -2}, {1, 1}, {-1, 1}, {1, -1}, {-1, -1}} {
						if at(x+d[0], y+d[1]) < 0 {
							rim = true
							break
						}
					}
					n := nrm[y*w+x]
					// The rim light only lights the side away from the light.
					rim = rim && -(n.x*light.x+n.y*light.y)/ll > 0.45*n.length()
					idx = tone(pt, n, rim, pc.far)
				}
			}
			img.Pix[(y+py0)*img.Stride+x+px0] = idx
		}
	}
}

// tone picks a part's palette index for a surface normal.
func tone(pt part, n vec, rim, far bool) uint8 {
	if pt.flat {
		if n.length() < 0.5 {
			return uint8(matGlow + 2)
		}
		return uint8(matGlow + 1)
	}
	nz := math.Sqrt(math.Max(0, 1-n.dot(n)))
	v := n.x*light.x + n.y*light.y + nz*light.z
	t := 2
	switch {
	case v < -0.2:
		t = 0
	case v < 0.2:
		t = 1
	case v < 0.62:
		t = 2
	case v < 0.86 || !pt.gloss:
		t = 3
	default:
		t = 4
	}
	if rim {
		t = 3
	}
	if far && t > 0 {
		t--
	}
	return uint8(pt.mat + t)
}
