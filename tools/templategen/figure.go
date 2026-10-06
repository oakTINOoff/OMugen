package main

// Proto's skeleton: poses, joint positions and leg IK. body.go draws it.

import (
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
