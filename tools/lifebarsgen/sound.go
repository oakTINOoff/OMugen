package main

// Synthesized sounds: the common hit, guard and fall sounds characters play
// from common.snd, and the fight screen's round, fight and K.O. stingers
// (no voices). Mono 16-bit 44.1 kHz WAVs in Elecbyte SND files.

import (
	"bytes"
	"encoding/binary"
	"math"
	"math/rand"
)

const rate = 44100

type wave []float64

func newWave(sec float64) wave { return make(wave, int(sec*rate)) }

func (w wave) add(o wave, at float64, gain float64) wave {
	i0 := int(at * rate)
	if need := i0 + len(o); need > len(w) {
		w = append(w, make(wave, need-len(w))...)
	}
	for i, v := range o {
		w[i0+i] += v * gain
	}
	return w
}

// env is an attack/exponential-decay envelope.
func env(n int, attack, decay float64) []float64 {
	e := make([]float64, n)
	for i := range e {
		t := float64(i) / rate
		a := 1.0
		if t < attack {
			a = t / attack
		}
		e[i] = a * math.Exp(-t/decay)
	}
	return e
}

// noise is white noise through a one-pole lowpass whose cutoff glides from
// f0 to f1 Hz.
func noise(sec, f0, f1, attack, decay float64, seed int64) wave {
	r := rand.New(rand.NewSource(seed))
	w := newWave(sec)
	e := env(len(w), attack, decay)
	var lp float64
	for i := range w {
		t := float64(i) / float64(len(w))
		f := f0 * math.Pow(f1/f0, t)
		a := 1 - math.Exp(-2*math.Pi*f/rate)
		lp += a * (r.Float64()*2 - 1 - lp)
		w[i] = lp * e[i]
	}
	return w
}

// tone is a sine whose pitch glides from f0 to f1 Hz.
func tone(sec, f0, f1, attack, decay float64) wave {
	w := newWave(sec)
	e := env(len(w), attack, decay)
	var ph float64
	for i := range w {
		t := float64(i) / float64(len(w))
		f := f0 * math.Pow(f1/f0, t)
		ph += 2 * math.Pi * f / rate
		w[i] = math.Sin(ph) * e[i]
	}
	return w
}

// reverb is a small Schroeder reverb (four combs, two allpasses), mixed in.
func reverb(w wave, tail, wet float64) wave {
	out := append(w, make(wave, int(tail*rate))...)
	dry := append(wave(nil), out...)
	acc := make(wave, len(out))
	for _, d := range []float64{0.0297, 0.0371, 0.0411, 0.0437} {
		n := int(d * rate)
		g := math.Pow(0.001, d/tail)
		buf := make(wave, len(out))
		for i := range out {
			v := dry[i]
			if i >= n {
				v += buf[i-n] * g
			}
			buf[i] = v
			acc[i] += v * 0.25
		}
	}
	for _, d := range []float64{0.005, 0.0017} {
		n := int(d * rate)
		buf := make(wave, len(acc))
		for i := range acc {
			x := acc[i]
			var y float64
			if i >= n {
				y = -0.7*x + acc[i-n] + 0.7*buf[i-n]
			} else {
				y = -0.7 * x
			}
			buf[i] = y
		}
		acc = buf
	}
	for i := range out {
		out[i] = dry[i]*(1-wet) + acc[i]*wet
	}
	return out
}

// normalize scales to the given peak and fades out the last 10 ms.
func normalize(w wave, peak float64) wave {
	var m float64
	for _, v := range w {
		m = math.Max(m, math.Abs(v))
	}
	if m == 0 {
		return w
	}
	fade := int(0.01 * rate)
	for i := range w {
		w[i] *= peak / m
		if k := len(w) - i; k < fade {
			w[i] *= float64(k) / float64(fade)
		}
	}
	return w
}

// ---------------------------------------------------------------- sounds

// hitSound is a hit of strength s (0 light .. 4 strongest): a noise crack
// over a pitched-down thump.
func hitSound(s int) wave {
	k := float64(s)
	w := newWave(0.1)
	w = w.add(noise(0.12+0.06*k, 9000, 900, 0.0005, 0.025+0.012*k, int64(s)), 0, 1)
	w = w.add(tone(0.18+0.08*k, 180-15*k, 45, 0.001, 0.05+0.025*k), 0, 0.9+0.1*k)
	if s >= 2 {
		w = w.add(noise(0.3, 2500, 200, 0.002, 0.08+0.02*k, int64(10+s)), 0.005, 0.5)
	}
	return normalize(reverb(w, 0.25+0.05*k, 0.12), 0.9)
}

// guardSound is a short metallic block.
func guardSound() wave {
	w := newWave(0.05)
	for i, f := range []float64{1850, 2960, 4370, 6110} {
		w = w.add(tone(0.25, f, f*0.97, 0.0005, 0.06-0.01*float64(i)), 0, 0.5)
	}
	w = w.add(noise(0.05, 12000, 4000, 0.0003, 0.008, 6), 0, 1.2)
	w = w.add(tone(0.12, 220, 120, 0.001, 0.03), 0, 0.6)
	return normalize(reverb(w, 0.3, 0.15), 0.85)
}

// fallSound is a body hitting the floor (0 light .. 2 heavy).
func fallSound(s int) wave {
	k := float64(s)
	w := newWave(0.1)
	w = w.add(tone(0.35+0.1*k, 110-10*k, 38, 0.002, 0.07+0.03*k), 0, 1)
	w = w.add(noise(0.3+0.1*k, 1800, 150, 0.002, 0.05+0.02*k, int64(70+s)), 0, 0.8)
	return normalize(reverb(w, 0.3, 0.1), 0.9)
}

// whoosh is a swing of air.
func whoosh(sec float64, seed int64) wave {
	w := newWave(sec)
	r := rand.New(rand.NewSource(seed))
	var lp, bp float64
	for i := range w {
		t := float64(i) / float64(len(w))
		f := 400 + 3200*math.Sin(t*math.Pi)
		a := 1 - math.Exp(-2*math.Pi*f/rate)
		x := r.Float64()*2 - 1
		lp += a * (x - lp)
		bp += a * (lp - bp)
		w[i] = (lp - bp) * math.Pow(math.Sin(t*math.Pi), 2)
	}
	return normalize(w, 0.8)
}

// chord is a bright synth stab of the given frequencies.
func chord(sec, decay float64, freqs ...float64) wave {
	w := newWave(sec)
	for _, f := range freqs {
		for h, g := range []float64{1, 0.5, 0.33, 0.25} {
			w = w.add(tone(sec, f*float64(h+1), f*float64(h+1), 0.004, decay/(1+float64(h)*0.6)), 0, g/float64(len(freqs)))
		}
	}
	return w
}

// boom is a deep impact with a long tail.
func boom(sec float64, seed int64) wave {
	w := newWave(0.1)
	w = w.add(tone(sec, 90, 28, 0.003, sec/3), 0, 1.2)
	w = w.add(noise(sec, 5000, 120, 0.001, sec/5, seed), 0, 0.9)
	return w
}

func roundSound(n int) wave {
	w := whoosh(0.45, int64(n))
	w = w.add(hitSound(3), 0.4, 0.8)
	f := 220 * math.Pow(2, float64(n-1)/24)
	w = w.add(chord(0.8, 0.25, f, f*1.5), 0.4, 0.5)
	return normalize(reverb(w, 0.8, 0.25), 0.9)
}

func finalRoundSound() wave {
	w := whoosh(0.6, 99)
	w = w.add(boom(1.0, 98), 0.55, 0.9)
	w = w.add(chord(1.2, 0.4, 146.8, 220, 293.7), 0.55, 0.7)
	return normalize(reverb(w, 1.2, 0.3), 0.92)
}

func fightSound() wave {
	w := newWave(0.1)
	w = w.add(hitSound(4), 0, 1)
	w = w.add(chord(0.9, 0.3, 293.7, 440, 587.3), 0, 0.8)
	w = w.add(noise(0.5, 300, 9000, 0.25, 0.4, 5), 0, 0.25) // rising air
	return normalize(reverb(w, 1.0, 0.3), 0.95)
}

func koSound() wave {
	w := boom(1.6, 20)
	w = w.add(tone(1.4, 440, 110, 0.01, 0.6), 0.05, 0.25)
	return normalize(reverb(w, 1.8, 0.35), 0.95)
}

func doubleKOSound() wave {
	w := boom(1.0, 21)
	w = w.add(boom(1.6, 22), 0.35, 1)
	return normalize(reverb(w, 1.8, 0.35), 0.95)
}

func timeOverSound() wave {
	w := newWave(0.1)
	for i, f := range []float64{587.3, 493.9, 392} {
		w = w.add(chord(0.25, 0.1, f), 0.18*float64(i), 0.9)
	}
	w = w.add(boom(1.0, 23), 0.54, 0.7)
	return normalize(reverb(w, 1.2, 0.3), 0.9)
}

func winSound() wave {
	w := newWave(0.1)
	for i, f := range []float64{392, 493.9, 587.3, 784} {
		w = w.add(chord(0.5, 0.18, f), 0.09*float64(i), 0.8)
	}
	w = w.add(chord(1.2, 0.5, 392, 493.9, 587.3, 784), 0.36, 0.9)
	return normalize(reverb(w, 1.2, 0.3), 0.9)
}

func drawSound() wave {
	w := chord(0.6, 0.25, 293.7, 349.2)
	w = w.add(chord(1.0, 0.4, 261.6, 329.6), 0.3, 1)
	return normalize(reverb(w, 1.0, 0.3), 0.85)
}

// ---------------------------------------------------------------- SND file

type sound struct {
	g, n int
	w    wave
}

func wav(w wave) []byte {
	var b bytes.Buffer
	n := len(w) * 2
	le := binary.LittleEndian
	b.WriteString("RIFF")
	binary.Write(&b, le, uint32(36+n))
	b.WriteString("WAVEfmt ")
	binary.Write(&b, le, uint32(16))
	binary.Write(&b, le, uint16(1)) // PCM
	binary.Write(&b, le, uint16(1)) // mono
	binary.Write(&b, le, uint32(rate))
	binary.Write(&b, le, uint32(rate*2))
	binary.Write(&b, le, uint16(2))
	binary.Write(&b, le, uint16(16))
	b.WriteString("data")
	binary.Write(&b, le, uint32(n))
	for _, v := range w {
		binary.Write(&b, le, int16(math.Max(-1, math.Min(1, v))*32767))
	}
	return b.Bytes()
}

// snd encodes an Elecbyte SND file: a 512-byte header, then each sound as a
// 16-byte subheader (next offset, length, group, number) and its WAV.
func snd(sounds []sound) []byte {
	var b bytes.Buffer
	le := binary.LittleEndian
	b.WriteString("ElecbyteSnd\x00")
	binary.Write(&b, le, uint16(0))
	binary.Write(&b, le, uint16(1))
	binary.Write(&b, le, uint32(len(sounds)))
	binary.Write(&b, le, uint32(512))
	b.Write(make([]byte, 512-b.Len()))
	for i, s := range sounds {
		data := wav(s.w)
		next := uint32(0)
		if i < len(sounds)-1 {
			next = uint32(b.Len() + 16 + len(data))
		}
		binary.Write(&b, le, next)
		binary.Write(&b, le, uint32(len(data)))
		binary.Write(&b, le, int32(s.g))
		binary.Write(&b, le, int32(s.n))
		b.Write(data)
	}
	return b.Bytes()
}

func commonSounds() []sound {
	var s []sound
	for k := 0; k < 5; k++ {
		s = append(s, sound{5, k, hitSound(k)})
	}
	s = append(s, sound{6, 0, guardSound()})
	for k := 0; k < 3; k++ {
		s = append(s, sound{7, k, fallSound(k)})
	}
	s = append(s, sound{20, 0, whoosh(0.35, 20)})
	return s
}

func fightSounds() []sound {
	var s []sound
	for n := 1; n <= 9; n++ {
		s = append(s, sound{0, n, roundSound(n)})
	}
	s = append(s, sound{0, 10, finalRoundSound()})
	s = append(s, sound{1, 0, fightSound()})
	s = append(s, sound{2, 0, koSound()})
	s = append(s, sound{2, 1, doubleKOSound()})
	s = append(s, sound{2, 2, timeOverSound()})
	s = append(s, sound{3, 0, winSound()})
	s = append(s, sound{3, 1, drawSound()})
	return s
}
