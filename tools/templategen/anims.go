package main

// Proto's poses and animations (MUGEN action numbers), and the AIR file.

import (
	"fmt"
	"image/color"
	"math"
	"strings"
)

// Palettes: body, accent, glow and armor-plate base colors.
var protoColors = []struct{ body, accent, glow, plate uint32 }{
	{0x363A48, 0xD6202C, 0xFF5A48, 0xA8ADBC}, // 1 Proto: graphite, red
	{0xD4DAE6, 0x2878F0, 0x5AD2FF, 0x3C4254}, // 2 white, blue
	{0x26262C, 0xDCA028, 0xFFB040, 0x6E5A3A}, // 3 black, gold
	{0x2A3A5C, 0x28C878, 0x96FF6E, 0x8C98AC}, // 4 navy, green
	{0x3E2A52, 0xD83CB4, 0xFF78DC, 0xB4A0C8}, // 5 purple, pink
	{0x5A5E66, 0xF07818, 0xFFD23C, 0x282C34}, // 6 steel, orange
}

func rgb(v uint32) color.RGBA { return color.RGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 255} }

func scaleRGB(c color.RGBA, k float64) color.RGBA {
	f := func(v uint8) uint8 { return uint8(math.Max(0, math.Min(255, float64(v)*k))) }
	return color.RGBA{f(c.R), f(c.G), f(c.B), 255}
}

func tint(c color.RGBA, t float64) color.RGBA { // towards white
	f := func(v uint8) uint8 { return uint8(float64(v) + (255-float64(v))*t) }
	return color.RGBA{f(c.R), f(c.G), f(c.B), 255}
}

// protoPalette builds palette i (0-based) in the shared index layout.
func protoPalette(i int) []color.RGBA {
	c := protoColors[i]
	p := make([]color.RGBA, 256)
	// Five tones: shadows lean towards a cool violet, highlights towards white.
	cool := color.RGBA{40, 30, 72, 255}
	ramp := func(at int, base color.RGBA) {
		p[at] = mixRGB(scaleRGB(base, 0.42), cool, 0.3)
		p[at+1] = mixRGB(scaleRGB(base, 0.68), cool, 0.14)
		p[at+2] = base
		p[at+3] = tint(base, 0.22)
		p[at+4] = tint(base, 0.6)
	}
	body := rgb(c.body)
	p[idxOutline] = mixRGB(scaleRGB(body, 0.2), color.RGBA{10, 8, 18, 255}, 0.6)
	ramp(matBody, body)
	ramp(matPlate, rgb(c.plate))
	ramp(matAccent, rgb(c.accent))
	g := rgb(c.glow)
	p[matGlow], p[matGlow+1], p[matGlow+2] = scaleRGB(g, 0.8), g, tint(g, 0.72)
	return p
}

func imagePalette(p []color.RGBA) color.Palette {
	cp := make(color.Palette, len(p))
	for i, c := range p {
		cp[i] = c
	}
	cp[0] = color.RGBA{}
	return cp
}

// ---------------------------------------------------------------- poses

const standY = -158.0

func stance(phi float64) Pose {
	b := math.Sin(phi)
	return Pose{
		Pelvis: vec{0, standY + 3*b}, Lean: 4 + 1.5*b, Head: -4,
		ArmN: Arm{55 + 3*b, 95}, ArmF: Arm{32 + 3*b, 108},
		LegN: Leg{IK: true, Ankle: vec{42, -footR}},
		LegF: Leg{IK: true, Ankle: vec{-46, -footR}},
		Wave: phi,
	}
}

func walk(phi float64) Pose {
	p := stance(phi * 2)
	s, c := math.Sin(phi), math.Cos(phi)
	p.Pelvis.y = standY + 4 - 7*(1-math.Abs(c))
	p.Lean = 9
	p.LegN.Ankle = vec{16 + 30*c, -footR - 20*math.Max(0, -s)}
	p.LegF.Ankle = vec{-18 - 30*c, -footR - 20*math.Max(0, s)}
	p.ArmN.Sh -= 8 * c
	p.ArmF.Sh += 8 * c
	return p
}

func run(phi float64) Pose {
	p := walk(phi)
	s, c := math.Sin(phi), math.Cos(phi)
	p.Lean = 26
	p.Head = -14
	p.Pelvis.y = standY + 6 - 10*(1-math.Abs(c))
	p.LegN.Ankle = vec{20 + 52*c, -footR - 34*math.Max(0, -s)}
	p.LegF.Ankle = vec{-22 - 52*c, -footR - 34*math.Max(0, s)}
	p.ArmN = Arm{60 - 30*c, 90}
	p.ArmF = Arm{40 + 30*c, 95}
	return p
}

func crouch() Pose {
	p := stance(0)
	p.Pelvis.y = -92
	p.Lean = 16
	p.Head = -10
	p.ArmN = Arm{72, 92}
	p.ArmF = Arm{42, 104}
	p.LegN.Ankle = vec{44, -footR}
	p.LegF.Ankle = vec{-48, -footR}
	return p
}

func airBase(lean float64) Pose {
	return Pose{
		Pelvis: vec{0, standY - 15}, Lean: lean, Head: -6,
		ArmN: Arm{62, 100}, ArmF: Arm{36, 110},
		LegN: Leg{Hip: 72, Knee: 112}, LegF: Leg{Hip: 34, Knee: 98},
	}
}

func airDown(lean float64) Pose {
	p := airBase(lean)
	p.LegN = Leg{Hip: 34, Knee: 38, Foot: -10}
	p.LegF = Leg{Hip: -6, Knee: 28, Foot: -10}
	p.ArmN = Arm{70, 80}
	return p
}

func guard(p Pose) Pose {
	p.ArmN = Arm{78, 122}
	p.ArmF = Arm{62, 126}
	p.Lean -= 6
	p.Head = 6
	return p
}

func hitHigh(k float64) Pose {
	p := stance(0)
	p.Lean = 2 - 9*k
	p.Head = -12 - 7*k
	p.Pelvis = vec{-6 * k, standY + 4*k}
	p.ArmN = Arm{25 + 12*k, 40}
	p.ArmF = Arm{-10 - 5*k, 30}
	return p
}

func hitLow(k float64) Pose {
	p := stance(0)
	p.Lean = 16 + 8*k
	p.Head = 12
	p.Pelvis = vec{-5 * k, standY + 6*k}
	p.ArmN = Arm{12, 20}
	p.ArmF = Arm{-4, 18}
	return p
}

func crouchHit(k float64) Pose {
	p := crouch()
	p.Lean = 12 - 10*k
	p.Head = -10 - 6*k
	p.ArmN = Arm{30, 50}
	p.ArmF = Arm{0, 40}
	return p
}

func airHit(lean float64) Pose {
	return Pose{
		Pelvis: vec{0, standY - 15}, Lean: lean, Head: -18,
		ArmN: Arm{150, 25}, ArmF: Arm{128, 15},
		LegN: Leg{Hip: 42, Knee: 34}, LegF: Leg{Hip: 14, Knee: 22},
	}
}

func lying(lift float64) Pose {
	return Pose{
		Pelvis: vec{0, -lift}, Lean: -88, Head: 4,
		ArmN: Arm{168, 12}, ArmF: Arm{160, 20},
		LegN: Leg{Hip: 88, Knee: 4, Foot: 30}, LegF: Leg{Hip: 82, Knee: 10, Foot: 30},
		Ground: true,
	}
}

func sitting() Pose {
	return Pose{
		Pelvis: vec{0, -30}, Lean: -28, Head: 10,
		ArmN: Arm{-20, 10}, ArmF: Arm{-30, 10},
		LegN: Leg{Hip: 80, Knee: 90}, LegF: Leg{Hip: 70, Knee: 110},
		Ground: true,
	}
}

// ---------------------------------------------------------------- animations

type frame struct {
	pose Pose
	time int
	hit  string // "fistN", "fistF", "footN", "footF": this frame hits
	beam int    // extra hitbox length in front of the hands (super beam)
}

type anim struct {
	no     int
	frames []frame
	loop   int // Loopstart frame index (-1: none)
}

func seq(no int, frames ...frame) anim { return anim{no: no, frames: frames, loop: -1} }

func cycle(no, n, t int, f func(phi float64) Pose) anim {
	a := anim{no: no, loop: -1}
	for i := 0; i < n; i++ {
		a.frames = append(a.frames, frame{pose: f(2 * math.Pi * float64(i) / float64(n)), time: t})
	}
	return a
}

func fr(p Pose, t int) frame { return frame{pose: p, time: t} }
func hitFr(p Pose, t int, limb string) frame {
	return frame{pose: p, time: t, hit: limb}
}

func with(p Pose, f func(p *Pose)) Pose { f(&p); return p }

// mustEnd lists the animations whose state waits for the animation to end
// (AnimTime = 0): the common states (crouching, guard start, knockdown,
// getting up...) and Proto's own moves in proto.cns. An animation that ends
// in a -1 frame never ends, which leaves the character stuck in its state.
var mustEnd = []int{5, 6, 10, 12, 40, 47, 110, 115, 120, 121, 122, 5100, 5120, 5170,
	190, 195, 200, 210, 230, 240, 400, 410, 430, 440, 1000, 3000}

func checkAnimsEnd(as []anim) {
	byNo := map[int]anim{}
	for _, a := range as {
		byNo[a.no] = a
	}
	for _, no := range mustEnd {
		a, ok := byNo[no]
		if !ok {
			panic(fmt.Sprintf("animation %d is missing", no))
		}
		if a.loop >= 0 || a.frames[len(a.frames)-1].time < 0 {
			panic(fmt.Sprintf("animation %d must end (no -1 frame, no loop)", no))
		}
	}
}

// protoAnims is Proto's animation table.
func protoAnims() []anim {
	st := stance(0)
	cr := crouch()
	var as []anim
	add := func(a ...anim) { as = append(as, a...) }

	// Standing, crouching, walking, running
	add(cycle(0, 8, 7, stance))
	add(seq(5, fr(with(st, func(p *Pose) { p.Lean = 0; p.ArmN.Sh = 30 }), 4)))
	add(seq(6, fr(with(cr, func(p *Pose) { p.Lean = 4 }), 4)))
	add(seq(10, fr(lerpPose(st, cr, 0.5), 2), fr(cr, 2)))
	add(cycle(11, 1, -1, func(float64) Pose { return cr }))
	add(seq(12, fr(lerpPose(st, cr, 0.5), 2), fr(st, 2)))
	add(cycle(20, 10, 4, walk))
	back := cycle(21, 10, 4, walk)
	for i, j := 0, len(back.frames)-1; i < j; i, j = i+1, j-1 {
		back.frames[i], back.frames[j] = back.frames[j], back.frames[i]
	}
	add(back)
	add(cycle(100, 8, 3, run))
	hop := airBase(-12)
	add(seq(105, fr(hop, 4), fr(airDown(-10), -1)))
	add(seq(110, fr(hop, 12)), seq(115, fr(airDown(-8), 12)))

	// Jumping
	pre := lerpPose(st, cr, 0.45)
	add(seq(40, fr(pre, 3)))
	add(seq(41, fr(airBase(4), 6), fr(lerpPose(airBase(4), airDown(4), 0.4), -1)))
	add(seq(42, fr(airBase(14), 6), fr(lerpPose(airBase(14), airDown(12), 0.4), -1)))
	add(seq(43, fr(airBase(-8), 6), fr(lerpPose(airBase(-8), airDown(-6), 0.4), -1)))
	add(seq(44, fr(airDown(4), -1)), seq(45, fr(airDown(10), -1)), seq(46, fr(airDown(-6), -1)))
	add(seq(47, fr(pre, 3), fr(lerpPose(pre, st, 0.5), 3)))

	// Guarding
	gs, gc, ga := guard(st), guard(cr), guard(airBase(0))
	add(seq(120, fr(lerpPose(st, gs, 0.5), 2), fr(gs, 2)))
	add(seq(121, fr(lerpPose(cr, gc, 0.5), 2), fr(gc, 2)))
	add(seq(122, fr(lerpPose(airBase(0), ga, 0.5), 2), fr(ga, 2)))
	add(seq(130, fr(gs, -1)), seq(131, fr(gc, -1)), seq(132, fr(ga, -1)))
	add(seq(140, fr(lerpPose(gs, st, 0.5), 3)), seq(141, fr(lerpPose(gc, cr, 0.5), 3)), seq(142, fr(lerpPose(ga, airBase(0), 0.5), 3)))
	shake := func(p Pose) Pose { return with(p, func(q *Pose) { q.Lean -= 6; q.Pelvis.x -= 4 }) }
	add(seq(150, fr(shake(gs), -1)), seq(151, fr(shake(gc), -1)), seq(152, fr(shake(ga), -1)))

	// Match end, intro, taunt
	sad := with(st, func(p *Pose) { p.Lean = 12; p.Head = 26; p.ArmN = Arm{4, 12}; p.ArmF = Arm{-2, 10} })
	add(seq(170, fr(lerpPose(st, sad, 0.5), 10), fr(sad, -1)))
	add(seq(175, fr(lerpPose(st, sad, 0.5), 10), fr(with(sad, func(p *Pose) { p.Head = 10 }), -1)))
	pump := with(st, func(p *Pose) { p.Lean = -4; p.Head = -16; p.ArmN = Arm{172, 18}; p.ArmF = Arm{20, 110} })
	add(seq(181, fr(st, 8), fr(lerpPose(st, pump, 0.5), 4), fr(pump, 50), fr(lerpPose(st, pump, 0.85), 6), fr(pump, -1)))
	bow := with(st, func(p *Pose) { p.Lean = 22; p.Head = 18; p.ArmN = Arm{10, 20}; p.ArmF = Arm{5, 20} })
	add(seq(190, fr(bow, 30), fr(lerpPose(bow, st, 0.5), 6), fr(st, 20)))
	taunt := func(e float64) Pose {
		return with(st, func(p *Pose) { p.Lean = -2; p.Head = -10; p.ArmN = Arm{95, e}; p.ArmF = Arm{10, 30} })
	}
	add(seq(195, fr(taunt(40), 6), fr(taunt(95), 6), fr(taunt(40), 6), fr(taunt(95), 6), fr(taunt(40), 6), fr(st, 8)))

	// Normal attacks (x/y punches, a/b kicks)
	jab := with(st, func(p *Pose) { p.Lean = 14; p.Head = -8; p.ArmN = Arm{88, 4}; p.LegN.Ankle.x = 58 })
	add(seq(200, fr(with(st, func(p *Pose) { p.ArmN = Arm{70, 70} }), 2), hitFr(jab, 3, "fistN"),
		fr(lerpPose(jab, st, 0.5), 4), fr(st, 4)))
	cross := with(st, func(p *Pose) {
		p.Lean = 20
		p.Head = -10
		p.Pelvis.x = 8
		p.ArmF = Arm{94, 0}
		p.ArmN = Arm{40, 112}
		p.FarArmFront = true
		p.LegN.Ankle.x = 64
	})
	add(seq(210, fr(with(st, func(p *Pose) { p.Lean = 0; p.ArmF = Arm{20, 112} }), 3),
		fr(lerpPose(st, cross, 0.5), 3), hitFr(cross, 4, "fistF"),
		fr(lerpPose(cross, st, 0.4), 6), fr(lerpPose(cross, st, 0.8), 5)))
	chamber := with(st, func(p *Pose) { p.Lean = 2; p.LegN = Leg{Hip: 52, Knee: 96}; p.LegF.Ankle.x = -30 })
	snap := with(chamber, func(p *Pose) { p.Lean = -6; p.LegN = Leg{Hip: 82, Knee: 10, Foot: 10} })
	add(seq(230, fr(chamber, 3), hitFr(snap, 4, "footN"), fr(chamber, 4), fr(st, 4)))
	windup := with(st, func(p *Pose) {
		p.Lean = -4
		p.FarLegFront = true
		p.LegF = Leg{Hip: 34, Knee: 100}
		p.LegN.Ankle.x = 14
	})
	round := with(windup, func(p *Pose) { p.Lean = -22; p.Head = 10; p.LegF = Leg{Hip: 106, Knee: 8}; p.ArmN = Arm{40, 100} })
	add(seq(240, fr(windup, 4), fr(lerpPose(windup, round, 0.5), 3), hitFr(round, 5, "footF"),
		fr(with(windup, func(p *Pose) { p.LegF = Leg{Hip: 80, Knee: 64}; p.Lean = -12 }), 6), fr(lerpPose(windup, st, 0.6), 6)))

	// Crouching attacks
	cjab := with(cr, func(p *Pose) { p.Lean = 20; p.ArmN = Arm{84, 4} })
	add(seq(400, fr(with(cr, func(p *Pose) { p.ArmN = Arm{70, 70} }), 2), hitFr(cjab, 3, "fistN"), fr(lerpPose(cjab, cr, 0.5), 4), fr(cr, 3)))
	upper := with(cr, func(p *Pose) {
		p.Pelvis.y = -118
		p.Lean = 8
		p.Head = -14
		p.ArmF = Arm{156, 22}
		p.FarArmFront = true
	})
	add(seq(410, fr(with(cr, func(p *Pose) { p.ArmF = Arm{30, 122}; p.FarArmFront = true }), 4),
		hitFr(upper, 4, "fistF"), fr(with(upper, func(p *Pose) { p.ArmF = Arm{168, 10} }), 6),
		fr(lerpPose(upper, cr, 0.6), 6)))
	ckick := with(cr, func(p *Pose) { p.Lean = 24; p.LegN = Leg{Hip: 82, Knee: 6, Foot: -6} })
	add(seq(430, fr(with(cr, func(p *Pose) { p.LegN = Leg{Hip: 50, Knee: 110} }), 3), hitFr(ckick, 4, "footN"), fr(cr, 5)))
	sweep := with(cr, func(p *Pose) {
		p.Pelvis.y = -74
		p.Lean = 34
		p.FarLegFront = true
		p.LegF = Leg{Hip: 92, Knee: 2, Foot: -4}
		p.ArmN = Arm{20, 40}
	})
	add(seq(440, fr(with(sweep, func(p *Pose) { p.LegF = Leg{Hip: 40, Knee: 60} }), 4),
		hitFr(sweep, 5, "footF"), fr(with(sweep, func(p *Pose) { p.LegF = Leg{Hip: 100, Knee: 30} }), 6), fr(cr, 7)))

	// Air attacks
	ab := airBase(8)
	add(seq(600, fr(with(ab, func(p *Pose) { p.ArmN = Arm{50, 80} }), 3), hitFr(with(ab, func(p *Pose) { p.ArmN = Arm{62, 8} }), 8, "fistN"), fr(ab, -1)))
	add(seq(610, fr(with(ab, func(p *Pose) { p.ArmF = Arm{20, 110}; p.FarArmFront = true }), 4),
		hitFr(with(ab, func(p *Pose) { p.Lean = 16; p.ArmF = Arm{66, 4}; p.FarArmFront = true }), 10, "fistF"), fr(ab, -1)))
	add(seq(630, fr(ab, 3), hitFr(with(ab, func(p *Pose) { p.LegN = Leg{Hip: 62, Knee: 22, Foot: 10} }), 10, "footN"), fr(ab, -1)))
	dive := with(ab, func(p *Pose) { p.Lean = -10; p.LegN = Leg{Hip: 70, Knee: 2, Foot: 5}; p.LegF = Leg{Hip: 22, Knee: 112} })
	add(seq(640, fr(ab, 4), hitFr(dive, 14, "footN"), fr(ab, -1)))

	// Special: Proto Blast (projectile)
	charge := with(st, func(p *Pose) { p.Lean = -6; p.ArmN = Arm{-12, 104}; p.ArmF = Arm{-18, 108} })
	push := with(st, func(p *Pose) {
		p.Lean = 12
		p.Head = -8
		p.ArmN = Arm{90, 0}
		p.ArmF = Arm{86, 6}
		p.FarArmFront = true
		p.LegN.Ankle.x = 60
	})
	add(seq(1000, fr(lerpPose(st, charge, 0.5), 4), fr(charge, 8), fr(push, 20), fr(lerpPose(push, st, 0.5), 8), fr(st, 6)))

	// Super: Proto Overdrive (beam)
	pow := with(st, func(p *Pose) { p.Lean = -10; p.Head = 12; p.ArmN = Arm{-30, 90}; p.ArmF = Arm{-36, 94} })
	fire := with(push, func(p *Pose) { p.Lean = 18; p.Pelvis.x = 10; p.LegN.Ankle.x = 70; p.LegF.Ankle.x = -64 })
	beam := func(t int) frame { return frame{pose: fire, time: t, hit: "fistN", beam: 900} }
	add(seq(3000, fr(lerpPose(st, pow, 0.5), 5), fr(pow, 35), fr(lerpPose(pow, fire, 0.5), 3),
		beam(6), beam(6), beam(6), beam(6), beam(6), beam(6), beam(6), beam(6),
		fr(fire, 10), fr(lerpPose(fire, st, 0.5), 10), fr(st, 6)))

	// Getting hit
	for i, k := range []float64{1, 2, 3} {
		add(seq(5000+i, fr(hitHigh(k), 3), fr(hitHigh(k*0.7), -1)))
		add(seq(5005+i, fr(lerpPose(hitHigh(k*0.7), st, 0.5), 4), fr(st, -1)))
		add(seq(5010+i, fr(hitLow(k), 3), fr(hitLow(k*0.7), -1)))
		add(seq(5015+i, fr(lerpPose(hitLow(k*0.7), st, 0.5), 4), fr(st, -1)))
		add(seq(5020+i, fr(crouchHit(k), 3), fr(crouchHit(k*0.7), -1)))
		add(seq(5025+i, fr(lerpPose(crouchHit(k*0.7), cr, 0.5), 4), fr(cr, -1)))
	}
	add(seq(5030, fr(airHit(-40), 3), fr(airHit(-48), -1)))
	add(seq(5035, fr(airHit(-58), 4), fr(airHit(-66), -1)))
	add(seq(5040, fr(airBase(-20), 4), fr(airBase(0), -1)))
	add(seq(5050, fr(airHit(-72), 6), fr(airHit(-80), -1)))
	add(seq(5051, fr(airHit(-86), -1)), seq(5060, fr(airHit(-88), -1)), seq(5061, fr(airHit(-88), -1)))
	trip := with(airHit(-55), func(p *Pose) { p.LegN = Leg{Hip: 104, Knee: 20}; p.LegF = Leg{Hip: 84, Knee: 30} })
	add(seq(5070, fr(trip, -1)))
	add(seq(5080, fr(lying(4), 4), fr(lying(0), -1)))
	add(seq(5090, fr(lying(0), -1)))
	add(seq(5100, fr(lying(10), 3), fr(lying(0), 3)))
	add(seq(5101, fr(with(airHit(-76), func(p *Pose) { p.Pelvis.y = -60 }), -1)))
	add(seq(5110, fr(lying(0), -1)), seq(5150, fr(lying(0), -1)), seq(5160, fr(lying(0), -1)), seq(5170, fr(lying(10), 3), fr(lying(0), 3)))
	add(seq(5120, fr(lying(0), 8), fr(sitting(), 8), fr(cr, 8), fr(lerpPose(cr, st, 0.5), 6), fr(st, 2)))
	add(seq(5200, fr(airBase(-10), 3), fr(cr, 4), fr(st, -1)))
	add(seq(5210, fr(airBase(-30), 3), fr(airBase(-10), -1)))
	dizzy := func(sway float64) Pose {
		return with(st, func(p *Pose) { p.Lean = -4 + sway; p.Head = 18 - sway; p.ArmN = Arm{8, 16}; p.ArmF = Arm{-4, 14} })
	}
	add(seq(5300, fr(dizzy(0), 12), fr(dizzy(6), 12), fr(dizzy(0), 12), fr(dizzy(-5), 12)))
	return as
}

// ---------------------------------------------------------------- output

type spriteKey struct{ g, n int }

// airFrame is a sprite reference plus its collision boxes.
type airFrame struct {
	spr        spriteKey
	time       int
	hurt, hits []box
	trans      string
}

func hurtBoxes(bp bodyParts) []box {
	return []box{
		boundsBox(append(append([]part{}, bp.head...), bp.torso...), 0),
		boundsBox(bp.legs, 0),
		boundsBox(append(append([]part{}, bp.armN...), bp.armF...), 0),
	}
}

func hitBoxes(bp bodyParts, limb string, beam int) []box {
	var ps []part
	switch limb {
	case "fistN":
		ps = bp.fistN
	case "fistF":
		ps = bp.fistF
	case "footN":
		ps = bp.footN
	case "footF":
		ps = bp.footF
	}
	if len(ps) == 0 {
		return nil
	}
	b := boundsBox(ps, 6)
	if beam > 0 {
		b.x1 += beam
		b.y0 -= 20
		b.y1 += 20
	}
	return []box{b}
}

func writeAir(sb *strings.Builder, no int, frames []airFrame, loop int) {
	fmt.Fprintf(sb, "\n[Begin Action %d]\n", no)
	for i, f := range frames {
		if i == loop {
			sb.WriteString("Loopstart\n")
		}
		if len(f.hurt) > 0 {
			fmt.Fprintf(sb, "Clsn2: %d\n", len(f.hurt))
			for j, b := range f.hurt {
				fmt.Fprintf(sb, "  Clsn2[%d] = %d, %d, %d, %d\n", j, b.x0, b.y0, b.x1, b.y1)
			}
		}
		if len(f.hits) > 0 {
			fmt.Fprintf(sb, "Clsn1: %d\n", len(f.hits))
			for j, b := range f.hits {
				fmt.Fprintf(sb, "  Clsn1[%d] = %d, %d, %d, %d\n", j, b.x0, b.y0, b.x1, b.y1)
			}
		}
		line := fmt.Sprintf("%d,%d, 0,0, %d", f.spr.g, f.spr.n, f.time)
		if f.trans != "" {
			line += ", , " + f.trans
		}
		sb.WriteString(line + "\n")
	}
}

func mixRGB(a, b color.RGBA, t float64) color.RGBA {
	f := func(x, y uint8) uint8 { return uint8(float64(x) + (float64(y)-float64(x))*t) }
	return color.RGBA{f(a.R, b.R), f(a.G, b.G), f(a.B, b.B), 255}
}
