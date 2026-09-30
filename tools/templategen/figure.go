package main

// Proto's body: a posable skeleton rendered as outlined, cel-shaded capsules
// straight into 8-bit paletted images, so the character can switch palettes
// like any MUGEN character.

import (
	"image"
	"image/color"
	"math"
)

type vec struct{ x, y float64 }

func (a vec) add(b vec) vec        { return vec{a.x + b.x, a.y + b.y} }
func (a vec) sub(b vec) vec        { return vec{a.x - b.x, a.y - b.y} }
func (a vec) mul(k float64) vec    { return vec{a.x * k, a.y * k} }
func (a vec) dot(b vec) float64    { return a.x*b.x + a.y*b.y }
func (a vec) length() float64      { return math.Hypot(a.x, a.y) }
func lerp(a, b, t float64) float64 { return a + (b-a)*t }

func rad(deg float64) float64 { return deg * math.Pi / 180 }

// dirDown is a unit vector at deg degrees from straight down, turning
// towards the front (+x) as deg grows: 0 down, 90 forward, 180 up.
func dirDown(deg float64) vec { r := rad(deg); return vec{math.Sin(r), math.Cos(r)} }

// Body measurements, in pixels of the 1280x720 character space.
const (
	torsoLen = 110.0
	neckLen  = 12.0
	headR    = 28.0
	upperArm = 68.0
	foreArm  = 62.0
	fistR    = 14.0
	thighLen = 88.0
	shinLen  = 84.0
	footLen  = 34.0
	footR    = 9.0
	outline  = 3.0
)

type Arm struct{ Sh, El float64 } // shoulder angle (from down, + forward, relative to the torso), elbow bend

// Leg is posed by IK (ankle target, knee pointing forward) or by angles.
type Leg struct {
	IK        bool
	Ankle     vec     // IK: ankle position (y negative is up, 0 the ground)
	Hip, Knee float64 // FK: thigh angle from down (+ forward), knee bend (+ bends backward)
	Foot      float64 // foot angle (+ toes up)
}

type Pose struct {
	Pelvis                   vec // pelvis position (y negative is up)
	Lean, Head               float64
	ArmN, ArmF               Arm // near (camera side) and far arm
	LegN, LegF               Leg
	Ground                   bool // shift the body so its lowest point touches the ground
	FarArmFront, FarLegFront bool // draw that far limb over the torso
	Wave                     float64
}

func lerpLeg(a, b Leg, t float64) Leg {
	l := Leg{IK: a.IK, Ankle: vec{lerp(a.Ankle.x, b.Ankle.x, t), lerp(a.Ankle.y, b.Ankle.y, t)},
		Hip: lerp(a.Hip, b.Hip, t), Knee: lerp(a.Knee, b.Knee, t), Foot: lerp(a.Foot, b.Foot, t)}
	if t >= 0.5 {
		l.IK = b.IK
	}
	return l
}

// lerpPose blends two poses (t = 0: a, 1: b).
func lerpPose(a, b Pose, t float64) Pose {
	p := a
	if t >= 0.5 {
		p = b
	}
	p.Pelvis = vec{lerp(a.Pelvis.x, b.Pelvis.x, t), lerp(a.Pelvis.y, b.Pelvis.y, t)}
	p.Lean, p.Head = lerp(a.Lean, b.Lean, t), lerp(a.Head, b.Head, t)
	p.ArmN = Arm{lerp(a.ArmN.Sh, b.ArmN.Sh, t), lerp(a.ArmN.El, b.ArmN.El, t)}
	p.ArmF = Arm{lerp(a.ArmF.Sh, b.ArmF.Sh, t), lerp(a.ArmF.El, b.ArmF.El, t)}
	p.LegN, p.LegF = lerpLeg(a.LegN, b.LegN, t), lerpLeg(a.LegF, b.LegF, t)
	p.Wave = lerp(a.Wave, b.Wave, t)
	return p
}

// skeleton holds the joint positions of a pose.
type skeleton struct {
	pelvis, neck, head, up, fwd, hup, hfwd vec
	shN, elN, haN, shF, elF, haF           vec
	hipN, knN, anN, toeN                   vec
	hipF, knF, anF, toeF                   vec
}

func legIK(hip, target vec) (knee, ankle vec) {
	d := target.sub(hip)
	dist := math.Min(d.length(), thighLen+shinLen-0.01)
	if dist < 1 {
		dist = 1
	}
	base := math.Atan2(d.y, d.x)
	c := (thighLen*thighLen + dist*dist - shinLen*shinLen) / (2 * thighLen * dist)
	off := math.Acos(math.Max(-1, math.Min(1, c)))
	k1 := hip.add(vec{math.Cos(base + off), math.Sin(base + off)}.mul(thighLen))
	k2 := hip.add(vec{math.Cos(base - off), math.Sin(base - off)}.mul(thighLen))
	knee = k1
	if k2.x > k1.x { // knees point forward
		knee = k2
	}
	return knee, hip.add(vec{math.Cos(base), math.Sin(base)}.mul(dist))
}

func (p Pose) skeleton() skeleton {
	var s skeleton
	s.pelvis = p.Pelvis
	L := rad(p.Lean)
	s.up = vec{math.Sin(L), -math.Cos(L)}
	s.fwd = vec{math.Cos(L), math.Sin(L)}
	s.neck = s.pelvis.add(s.up.mul(torsoLen))
	H := rad(p.Lean + p.Head)
	s.hup = vec{math.Sin(H), -math.Cos(H)}
	s.hfwd = vec{math.Cos(H), math.Sin(H)}
	s.head = s.neck.add(s.hup.mul(neckLen + headR))
	sh := s.pelvis.add(s.up.mul(torsoLen - 14))
	s.shN, s.shF = sh.add(s.fwd.mul(10)), sh.sub(s.fwd.mul(12))
	arm := func(sh vec, a Arm) (vec, vec) {
		el := sh.add(dirDown(a.Sh + p.Lean).mul(upperArm))
		return el, el.add(dirDown(a.Sh + p.Lean + a.El).mul(foreArm))
	}
	s.elN, s.haN = arm(s.shN, p.ArmN)
	s.elF, s.haF = arm(s.shF, p.ArmF)
	s.hipN, s.hipF = s.pelvis.add(vec{9, 0}), s.pelvis.add(vec{-9, 0})
	leg := func(hip vec, l Leg) (kn, an, toe vec) {
		var shinDir float64
		if l.IK {
			kn, an = legIK(hip, l.Ankle)
			d := an.sub(kn)
			shinDir = math.Atan2(d.x, d.y) * 180 / math.Pi
		} else {
			kn = hip.add(dirDown(l.Hip).mul(thighLen))
			shinDir = l.Hip - l.Knee
			an = kn.add(dirDown(shinDir).mul(shinLen))
		}
		// The foot is perpendicular to the shin, pointing forward.
		toe = an.add(dirDown(shinDir + 90 + l.Foot).mul(footLen))
		return
	}
	s.knN, s.anN, s.toeN = leg(s.hipN, p.LegN)
	s.knF, s.anF, s.toeF = leg(s.hipF, p.LegF)
	return s
}

// Palette index layout shared by every palette.
const (
	idxOutline = 1
	matBody    = 2  // 2-4: shadow, mid, light
	matAccent  = 5  // 5-7
	matGlow    = 8  // 8-10
	matPlate   = 11 // 11-13
)

type partKind int

const (
	capsule partKind = iota
	circle
)

type part struct {
	kind   partKind
	a, b   vec
	ra, rb float64
	mat    int
	flat   bool // glow: no shading bands
}

func cap2(a, b vec, ra, rb float64, mat int) part {
	return part{kind: capsule, a: a, b: b, ra: ra, rb: rb, mat: mat}
}
func circ(c vec, r float64, mat int) part { return part{kind: circle, a: c, ra: r, mat: mat} }

// sample returns whether p is inside the part (grown by grow pixels) and a
// normal-like vector for shading (length <= 1).
func (pt part) sample(p vec, grow float64) (bool, vec) {
	switch pt.kind {
	case circle:
		d := p.sub(pt.a)
		r := pt.ra + grow
		if d.length() > r {
			return false, vec{}
		}
		return true, d.mul(1 / math.Max(pt.ra, 1))
	default:
		ab := pt.b.sub(pt.a)
		l2 := ab.dot(ab)
		t := 0.0
		if l2 > 0 {
			t = math.Max(0, math.Min(1, p.sub(pt.a).dot(ab)/l2))
		}
		c := pt.a.add(ab.mul(t))
		r := lerp(pt.ra, pt.rb, t)
		d := p.sub(c)
		if d.length() > r+grow {
			return false, vec{}
		}
		return true, d.mul(1 / math.Max(r, 1))
	}
}

func (pt part) bounds(grow float64) (x0, y0, x1, y1 float64) {
	r := math.Max(pt.ra, pt.rb) + grow
	if pt.kind == circle {
		return pt.a.x - r, pt.a.y - r, pt.a.x + r, pt.a.y + r
	}
	return math.Min(pt.a.x, pt.b.x) - r, math.Min(pt.a.y, pt.b.y) - r, math.Max(pt.a.x, pt.b.x) + r, math.Max(pt.a.y, pt.b.y) + r
}

// bodyParts lists the parts of a pose in drawing order (back to front), and
// named groups used for collision boxes.
type bodyParts struct {
	draw                          []part
	head, torso, armN, armF, legs []part
	fistN, fistF, footN, footF    []part
}

func (p Pose) parts() bodyParts {
	s := p.skeleton()
	var bp bodyParts

	armParts := func(sh, el, ha vec, pad bool) []part {
		ps := []part{
			cap2(sh, el, 17, 13, matBody),
			cap2(el, ha, 13, 12, matPlate),
			circ(ha, fistR+2, matAccent),
		}
		if pad {
			ps = append([]part{circ(sh, 20, matPlate)}, ps...)
		}
		return ps
	}
	legParts := func(hip, kn, an, toe vec) []part {
		heel := an.sub(toe.sub(an).mul(0.25))
		return []part{
			cap2(hip, kn, 23, 16, matBody),
			cap2(kn, an, 16, 12, matPlate),
			circ(kn, 13, matPlate),
			cap2(heel, toe, footR+2, footR, matAccent),
		}
	}
	bp.armF = armParts(s.shF, s.elF, s.haF, false)
	bp.armN = armParts(s.shN, s.elN, s.haN, true)
	legF := legParts(s.hipF, s.knF, s.anF, s.toeF)
	legN := legParts(s.hipN, s.knN, s.anN, s.toeN)
	bp.legs = append(append([]part{}, legF...), legN...)
	bp.fistN, bp.fistF = bp.armN[len(bp.armN)-1:], bp.armF[len(bp.armF)-1:]
	bp.footN = []part{legN[1], legN[3]}
	bp.footF = []part{legF[1], legF[3]}

	at := func(f, side float64) vec { return s.pelvis.add(s.up.mul(torsoLen * f)).add(s.fwd.mul(side)) }
	bp.torso = []part{
		cap2(s.hipF, s.hipN, 22, 22, matBody),
		cap2(s.pelvis, at(0.45, 0), 23, 25, matBody),
		cap2(at(0.5, 0), at(0.78, 0), 32, 34, matBody),
		cap2(s.shF, s.shN, 18, 18, matBody),
		cap2(s.neck, s.neck.add(s.hup.mul(neckLen)), 12, 12, matBody),
		cap2(at(0.52, 9), at(0.8, 12), 21, 23, matPlate),
		cap2(at(0.06, -22), at(0.06, 22), 8, 8, matAccent),
	}
	h, hu, hf := s.head, s.hup, s.hfwd
	visor := cap2(h.add(hf.mul(headR*0.22)).add(hu.mul(headR*0.02)), h.add(hf.mul(headR*0.92)).add(hu.mul(headR*0.08)), headR*0.2, headR*0.17, matGlow)
	visor.flat = true
	bp.head = []part{
		circ(h, headR, matBody),
		circ(h.add(hf.mul(headR*0.3)).sub(hu.mul(headR*0.52)), headR*0.5, matPlate),
		cap2(h.sub(hf.mul(headR*0.98)).add(hu.mul(headR*0.3)), h.add(hf.mul(headR*0.72)).add(hu.mul(headR*0.48)), headR*0.17, headR*0.15, matAccent),
		visor,
	}
	// Headband tails trail behind the head.
	back := h.sub(hf.mul(headR * 0.95)).add(hu.mul(headR * 0.3))
	w := math.Sin(p.Wave)
	tails := []part{
		cap2(back, back.sub(hf.mul(38)).sub(hu.mul(14+6*w)), 6, 3, matAccent),
		cap2(back.sub(hu.mul(4)), back.sub(hf.mul(26)).sub(hu.mul(28+5*math.Sin(p.Wave+1.3))), 5, 3, matAccent),
	}

	add := func(ps ...part) { bp.draw = append(bp.draw, ps...) }
	if !p.FarArmFront {
		add(bp.armF...)
	}
	if !p.FarLegFront {
		add(legF...)
	}
	add(tails...)
	add(bp.torso...)
	add(bp.head...)
	if p.FarLegFront {
		add(legF...)
	}
	add(legN...)
	if p.FarArmFront {
		add(bp.armF...)
	}
	add(bp.armN...)
	return bp
}

// groundShift is the vertical shift that puts the lowest point on y = 0.
func groundShift(ps []part) float64 {
	maxY := math.Inf(-1)
	for _, pt := range ps {
		_, _, _, y1 := pt.bounds(0)
		maxY = math.Max(maxY, y1)
	}
	return -maxY
}

func shiftParts(ps []part, dy float64) {
	for i := range ps {
		ps[i].a.y += dy
		ps[i].b.y += dy
	}
}

// light direction for cel shading: from the upper front.
var lightDir = vec{0.42, -0.9}

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
	all := [][]part{bp.draw, bp.head, bp.torso, bp.armN, bp.armF, bp.legs, bp.fistN, bp.fistF, bp.footN, bp.footF}
	for _, g := range all {
		shiftParts(g, dy)
	}
	// Canvas in scaled pixels; origin (0,0) of pose space at (ox, oy).
	const W, H, ox, oy = 700, 640, 350, 560
	img := image.NewPaletted(image.Rect(0, 0, W, H), pal)
	for _, pt := range bp.draw {
		paintPart(img, pt, scale, ox, oy)
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

func paintPart(img *image.Paletted, pt part, scale float64, ox, oy int) {
	x0, y0, x1, y1 := pt.bounds(outline)
	b := img.Bounds()
	px0 := max(b.Min.X, int(math.Floor(x0*scale))+ox)
	px1 := min(b.Max.X-1, int(math.Ceil(x1*scale))+ox)
	py0 := max(b.Min.Y, int(math.Floor(y0*scale))+oy)
	py1 := min(b.Max.Y-1, int(math.Ceil(y1*scale))+oy)
	for pass := 0; pass < 2; pass++ {
		grow := outline
		if pass == 1 {
			grow = 0
		}
		for y := py0; y <= py1; y++ {
			for x := px0; x <= px1; x++ {
				p := vec{(float64(x-ox) + 0.5) / scale, (float64(y-oy) + 0.5) / scale}
				in, n := pt.sample(p, grow)
				if !in {
					continue
				}
				idx := uint8(idxOutline)
				if pass == 1 {
					idx = uint8(pt.mat + 1)
					if !pt.flat {
						lit := n.dot(lightDir)
						if lit > 0.4 {
							idx = uint8(pt.mat + 2)
						} else if lit < -0.32 {
							idx = uint8(pt.mat)
						}
					} else if n.length() < 0.55 {
						idx = uint8(pt.mat + 2)
					}
				}
				img.Pix[y*img.Stride+x] = idx
			}
		}
	}
}

// box is a collision box in character space (relative to the axis).
type box struct{ x0, y0, x1, y1 int }

func boundsBox(ps []part, grow float64) box {
	if len(ps) == 0 {
		return box{}
	}
	fx0, fy0, fx1, fy1 := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, pt := range ps {
		a, b, c, d := pt.bounds(grow)
		fx0, fy0, fx1, fy1 = math.Min(fx0, a), math.Min(fy0, b), math.Max(fx1, c), math.Max(fy1, d)
	}
	return box{int(math.Floor(fx0)), int(math.Floor(fy0)), int(math.Ceil(fx1)), int(math.Ceil(fy1))}
}
