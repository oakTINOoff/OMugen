package main

// Big portraits cut to the part a game's own screenpack shows.
//
// Some games draw both players' portraits in one full-screen sprite (9000,1)
// and let each side's p1.face.window / p2.face.window cut out its part (DBS
// KODAIKA: a 1280x720 sprite with the fighter at both edges). The OMugen
// screenpack fits the whole sprite into its portrait box, which then shows
// the fighter twice. Instead it cuts out the part the game's screenpack shows
// for that side (start.f_gameFaceCrop), trimmed to the drawn pixels, and
// draws only that part (Anim.crop).

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

// opaqueCell is the size of the cells an opaqueMask records.
const opaqueCell = 8

// opaqueMask records which opaqueCell x opaqueCell cells of a sprite have
// visible pixels. Kept for big portraits only, made while they are decoded.
type opaqueMask struct {
	w, h  int // in cells
	cells []bool
}

// wantsOpaqueMask: big portraits (9000,1 and up).
func wantsOpaqueMask(s *Sprite) bool { return s.Group == 9000 && s.Number >= 1 }

func newOpaqueMask(w, h int) *opaqueMask {
	m := &opaqueMask{w: (w + opaqueCell - 1) / opaqueCell, h: (h + opaqueCell - 1) / opaqueCell}
	m.cells = make([]bool, m.w*m.h)
	return m
}

// opaqueMaskRGBA maps the pixels with alpha of a w x h RGBA image.
func opaqueMaskRGBA(data []byte, w, h int) *opaqueMask {
	if w <= 0 || h <= 0 || len(data) < w*h*4 {
		return nil
	}
	m := newOpaqueMask(w, h)
	for y := 0; y < h; y++ {
		row, cells := data[y*w*4:(y+1)*w*4], m.cells[y/opaqueCell*m.w:]
		for x := 0; x < w; x++ {
			if row[x*4+3] != 0 {
				cells[x/opaqueCell] = true
			}
		}
	}
	return m
}

// opaqueMaskIndexed maps the pixels of a w x h paletted image that aren't
// color 0 (transparent in MUGEN).
func opaqueMaskIndexed(px []byte, w, h int) *opaqueMask {
	if w <= 0 || h <= 0 || len(px) < w*h {
		return nil
	}
	m := newOpaqueMask(w, h)
	for y := 0; y < h; y++ {
		row, cells := px[y*w:(y+1)*w], m.cells[y/opaqueCell*m.w:]
		for x, c := range row {
			if c != 0 {
				cells[x/opaqueCell] = true
			}
		}
	}
	return m
}

// bounds returns the part of [x0,x1) x [y0,y1) (sprite pixels) holding its
// visible pixels, to the cell; ok is false when there are none.
func (m *opaqueMask) bounds(x0, y0, x1, y1 float64) (bx0, by0, bx1, by1 float64, ok bool) {
	cx0, cy0 := int(math.Floor(x0/opaqueCell)), int(math.Floor(y0/opaqueCell))
	cx1, cy1 := int(math.Ceil(x1/opaqueCell)), int(math.Ceil(y1/opaqueCell))
	cx0, cy0 = Max(cx0, 0), Max(cy0, 0)
	cx1, cy1 = Min(cx1, m.w), Min(cy1, m.h)
	minX, minY, maxX, maxY := m.w, m.h, -1, -1
	for cy := cy0; cy < cy1; cy++ {
		for cx := cx0; cx < cx1; cx++ {
			if m.cells[cy*m.w+cx] {
				minX, maxX = Min(minX, cx), Max(maxX, cx)
				minY, maxY = Min(minY, cy), Max(maxY, cy)
			}
		}
	}
	if maxX < 0 {
		return 0, 0, 0, 0, false
	}
	return math.Max(x0, float64(minX*opaqueCell)), math.Max(y0, float64(minY*opaqueCell)),
		math.Min(x1, float64((maxX+1)*opaqueCell)), math.Min(y1, float64((maxY+1)*opaqueCell)), true
}

// cropWindow narrows window (pixels: x, y, w, h) to the screen rectangle of
// the anim's crop, the sprite drawn at scale xscl (negative: mirrored).
func (a *Anim) cropWindow(window [4]int32, xscl float32) [4]int32 {
	spr := a.anim.spr
	if spr == nil {
		return window
	}
	ax, ay := float32(spr.Offset[0]), float32(spr.Offset[1])
	// The sprite's axis, as Draw places it.
	ux := a.x + a.vel[0] + float32(int(sys.gameWidth-320)/2)
	uy := a.y + a.vel[1] + float32(int(sys.gameHeight-240))
	x0, x1 := ux+(a.crop[0]-ax)*xscl, ux+(a.crop[2]-ax)*xscl
	if x0 > x1 {
		x0, x1 = x1, x0
	}
	y0, y1 := uy+(a.crop[1]-ay)*a.yscl, uy+(a.crop[3]-ay)*a.yscl
	if y0 > y1 {
		y0, y1 = y1, y0
	}
	l := int32(math.Floor(float64(x0 * sys.widthScale)))
	t := int32(math.Floor(float64(y0 * sys.heightScale)))
	r := int32(math.Ceil(float64(x1 * sys.widthScale)))
	b := int32(math.Ceil(float64(y1 * sys.heightScale)))
	l, t = Max(l, window[0]), Max(t, window[1])
	r, b = Min(r, window[0]+window[2]), Min(b, window[1]+window[3])
	if r < l {
		r = l
	}
	if b < t {
		b = t
	}
	return [4]int32{l, t, r - l, b - t}
}

// gameFace is how the game's own screenpack draws a side's big portrait.
type gameFace struct {
	ok         bool
	spr        [2]int32
	offset     [2]float32
	scale      [2]float32
	facing     float32
	window     [4]float32
	localcoord float32 // the screenpack's width
}

var gameFaces = map[string]*[2]gameFace{} // per game motif

// omugenGameFace reads p<side>.face from the game's own screenpack, when the
// OMugen screenpack replaces it and the game's cuts its portraits with a
// window.
func omugenGameFace(side int) (gameFace, bool) {
	if side < 1 || side > 2 || !isOMugenMotif(sys.motif.Def) || omugenUI.gameDef == "" {
		return gameFace{}, false
	}
	def := omugenUI.gameDef
	faces, ok := gameFaces[def]
	if !ok {
		faces = &[2]gameFace{}
		for i := range faces {
			faces[i] = readGameFace(def, i+1)
		}
		gameFaces[def] = faces
		for i, f := range faces {
			if f.ok {
				LogMessage("OMugen screenpack: P%d portraits cut like %s's (window %v)", i+1, mugenSlash(def), f.window)
			}
		}
	}
	return faces[side-1], faces[side-1].ok
}

func readGameFace(def string, side int) gameFace {
	key := func(k string) string { return readMotifKey(def, "Select Info", fmt.Sprintf("p%d.face.%s", side, k)) }
	f := gameFace{spr: [2]int32{9000, 1}, scale: [2]float32{1, 1}, facing: 1}
	if side == 2 {
		f.facing = -1
	}
	w := motifFloats(key("window"))
	if len(w) < 4 {
		return f
	}
	f.window = [4]float32{Min(w[0], w[2]), Min(w[1], w[3]), Max(w[0], w[2]), Max(w[1], w[3])}
	if v := motifFloats(key("spr")); len(v) >= 2 {
		f.spr = [2]int32{int32(v[0]), int32(v[1])}
	}
	if v := motifFloats(key("offset")); len(v) >= 2 {
		f.offset = [2]float32{v[0], v[1]}
	}
	if v := motifFloats(key("scale")); len(v) >= 2 {
		f.scale = [2]float32{v[0], v[1]}
	}
	if v := motifFloats(key("facing")); len(v) >= 1 && v[0] != 0 {
		f.facing = v[0]
	}
	f.localcoord = float32(motifDefLocalcoord(def)[0])
	f.ok = true
	return f
}

// motifFloats parses "a, b, c" (nil when a value isn't a number).
func motifFloats(s string) []float32 {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var out []float32
	for _, p := range strings.Split(s, ",") {
		v, err := strconv.ParseFloat(strings.TrimSpace(p), 32)
		if err != nil {
			return nil
		}
		out = append(out, float32(v))
	}
	return out
}

func omugenPortraitCropScriptInit(l *lua.LState) {
	luaRegister(l, "omugenGameFace", func(l *lua.LState) int {
		f, ok := omugenGameFace(int(numArg(l, 1)))
		if !ok {
			return 0
		}
		nums := func(v ...float32) *lua.LTable {
			t := l.NewTable()
			for _, x := range v {
				t.Append(lua.LNumber(x))
			}
			return t
		}
		t := l.NewTable()
		t.RawSetString("spr", nums(float32(f.spr[0]), float32(f.spr[1])))
		t.RawSetString("offset", nums(f.offset[:]...))
		t.RawSetString("scale", nums(f.scale[:]...))
		t.RawSetString("facing", lua.LNumber(f.facing))
		t.RawSetString("window", nums(f.window[:]...))
		t.RawSetString("localcoord", lua.LNumber(f.localcoord))
		l.Push(t)
		return 1
	})
	// animGetOpaqueBounds(anim, x0, y0, x1, y1): the part of that rectangle of
	// the anim's big portrait holding its drawn pixels, or nothing.
	luaRegister(l, "animGetOpaqueBounds", func(l *lua.LState) int {
		a, ok := toUserData(l, 1).(*Anim)
		if !ok || a.anim == nil {
			return 0
		}
		spr := a.anim.spr
		if spr == nil && a.anim.sff != nil && len(a.anim.frames) > 0 {
			// Not drawn yet: its first frame's sprite.
			if f := a.anim.frames[0]; f.Group >= 0 && f.Number >= 0 {
				spr = a.anim.sff.GetSprite(uint16(f.Group), uint16(f.Number))
			}
		}
		if spr == nil || spr.opaque == nil {
			return 0
		}
		x0, y0, x1, y1, ok := spr.opaque.bounds(float64(numArg(l, 2)), float64(numArg(l, 3)),
			float64(numArg(l, 4)), float64(numArg(l, 5)))
		if !ok {
			return 0
		}
		for _, v := range []float64{x0, y0, x1, y1} {
			l.Push(lua.LNumber(v))
		}
		return 4
	})
	// animSetCrop(anim, x0, y0, x1, y1): draw only that part of the sprite
	// (sprite pixels); animSetCrop(anim) draws all of it again.
	luaRegister(l, "animSetCrop", func(l *lua.LState) int {
		a, ok := toUserData(l, 1).(*Anim)
		if !ok {
			userDataError(l, 1, a)
		}
		if nilArg(l, 5) {
			a.crop = [4]float32{}
			return 0
		}
		a.crop = [4]float32{float32(numArg(l, 2)), float32(numArg(l, 3)), float32(numArg(l, 4)), float32(numArg(l, 5))}
		return 0
	})
}
