//go:build !android

package main

// OMugen glow for the OpenGL 3.3 renderer.
//
// Additive gameplay sprites are drawn a second time into a full-resolution
// RGBA16F "glow" target (same transforms and blending). Before the next
// layer, that target is downsampled to half resolution, blurred with a
// separable Gaussian (twice), and added back onto the scene.

import (
	_ "embed" // glow shader

	gl "github.com/go-gl/gl/v3.3-core/gl"
)

//go:embed shaders/omugen_glow.frag.glsl
var omugenGlowFragShader string

type glowGL33 struct {
	failed   bool
	ready    bool
	w, h     int32 // capture size (internal resolution)
	hw, hh   int32 // blur size (half resolution)
	fbo      uint32
	tex      uint32
	halfFbo  [2]uint32
	halfTex  [2]uint32
	vao      uint32
	prog     *ShaderProgram_GL33
	captured bool // glow target holds sprites not yet composited
}

func glowTexture(w, h int32) uint32 {
	var t uint32
	gl.GenTextures(1, &t)
	textureSerialNumber++
	gl.BindTexture(gl.TEXTURE_2D, t)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	// Half float so stacked additive effects don't clip before the blur.
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA16F, w, h, 0, gl.RGBA, gl.FLOAT, nil)
	return t
}

func glowFramebuffer(tex uint32) (uint32, bool) {
	var f uint32
	gl.GenFramebuffers(1, &f)
	gl.BindFramebuffer(gl.FRAMEBUFFER, f)
	gl.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, tex, 0)
	return f, gl.CheckFramebufferStatus(gl.FRAMEBUFFER) == gl.FRAMEBUFFER_COMPLETE
}

// GlowReady creates the glow targets on first use. It returns false if glow
// is unavailable on this GPU (it then stays disabled for the session).
func (r *Renderer_GL33) GlowReady() bool {
	g := &r.glow
	if g.failed {
		return false
	}
	w, h := sys.scrrect[2], sys.scrrect[3]
	if g.ready && g.w == w && g.h == h {
		return true
	}
	if g.ready {
		r.glowRelease()
	}
	prog, err := r.newShaderProgram(identVertShader, omugenGlowFragShader, "", "OMugen Glow", false)
	if err != nil {
		LogMessage("OMugen: glow disabled, shader error: %v", err)
		g.failed = true
		return false
	}
	prog.RegisterAttributes("VertCoord")
	prog.RegisterUniforms("texel", "dir", "intensity", "mode")
	prog.RegisterTextures("Texture")
	g.prog = prog

	g.w, g.h = w, h
	g.hw, g.hh = Max(w/2, 1), Max(h/2, 1)
	r.SetActiveTexture0()
	g.tex = glowTexture(w, h)
	ok := true
	var fok bool
	g.fbo, fok = glowFramebuffer(g.tex)
	ok = ok && fok
	for i := range g.halfTex {
		g.halfTex[i] = glowTexture(g.hw, g.hh)
		g.halfFbo[i], fok = glowFramebuffer(g.halfTex[i])
		ok = ok && fok
	}
	gl.BindTexture(gl.TEXTURE_2D, 0)

	// Own VAO over the shared fullscreen quad, with this program's attribute location.
	gl.GenVertexArrays(1, &g.vao)
	gl.BindVertexArray(g.vao)
	gl.BindBuffer(gl.ARRAY_BUFFER, r.postVertBuffer)
	if loc, found := prog.attributes["VertCoord"]; found && loc >= 0 {
		gl.EnableVertexAttribArray(uint32(loc))
		gl.VertexAttribPointer(uint32(loc), 2, gl.FLOAT, false, 0, nil)
	}
	gl.BindVertexArray(0)
	gl.BindBuffer(gl.ARRAY_BUFFER, 0)

	gl.BindFramebuffer(gl.FRAMEBUFFER, r.fbo)
	if !ok {
		LogMessage("OMugen: glow disabled, framebuffer not supported")
		r.glowRelease()
		g.failed = true
		return false
	}
	g.ready = true
	g.captured = false
	return true
}

func (r *Renderer_GL33) glowRelease() {
	g := &r.glow
	gl.DeleteFramebuffers(1, &g.fbo)
	gl.DeleteTextures(1, &g.tex)
	gl.DeleteFramebuffers(2, &g.halfFbo[0])
	gl.DeleteTextures(2, &g.halfTex[0])
	gl.DeleteVertexArrays(1, &g.vao)
	if g.prog != nil {
		gl.DeleteProgram(g.prog.program)
		g.prog = nil
	}
	g.ready = false
}

// GlowCaptureBegin redirects drawing to the glow target (cleared on the first
// capture after a composite). The caller then repeats the sprite draw.
func (r *Renderer_GL33) GlowCaptureBegin() {
	g := &r.glow
	gl.BindFramebuffer(gl.FRAMEBUFFER, g.fbo)
	if !g.captured {
		scissor := r.scissorEnabled
		if scissor {
			gl.Disable(gl.SCISSOR_TEST)
		}
		gl.ClearColor(0, 0, 0, 0)
		gl.Clear(gl.COLOR_BUFFER_BIT)
		if scissor {
			gl.Enable(gl.SCISSOR_TEST)
		}
		g.captured = true
	}
}

// GlowHasContent reports whether glow was captured since the last composite.
func (r *Renderer_GL33) GlowHasContent() bool {
	return r.glow.captured
}

// GlowCaptureEnd returns drawing to the scene framebuffer.
func (r *Renderer_GL33) GlowCaptureEnd() {
	gl.BindFramebuffer(gl.FRAMEBUFFER, r.fbo)
}

// GlowComposite blurs what was captured and adds it onto the scene.
func (r *Renderer_GL33) GlowComposite(intensity, radius float32) {
	g := &r.glow
	if !g.ready || !g.captured {
		return
	}
	g.captured = false

	r.DisableScissor()
	r.DisableBlending()
	r.SetDepthTest(false)
	r.ChangeProgram(g.prog.program)
	gl.BindVertexArray(g.vao)
	r.SetActiveTexture0()
	u := g.prog.uniforms
	gl.Uniform1i(u["Texture"], 0)

	pass := func(dstFbo uint32, w, h int32, src uint32, srcW, srcH int32, mode int32, dx, dy float32) {
		gl.BindFramebuffer(gl.FRAMEBUFFER, dstFbo)
		gl.Viewport(0, 0, w, h)
		gl.BindTexture(gl.TEXTURE_2D, src)
		gl.Uniform1i(u["mode"], mode)
		gl.Uniform2f(u["texel"], 1/float32(srcW), 1/float32(srcH))
		gl.Uniform2f(u["dir"], dx, dy)
		gl.DrawArrays(gl.TRIANGLE_STRIP, 0, 4)
	}
	// Downsample, then two separable blur iterations at half resolution.
	pass(g.halfFbo[0], g.hw, g.hh, g.tex, g.w, g.h, 0, 0, 0)
	for i := 0; i < 2; i++ {
		pass(g.halfFbo[1], g.hw, g.hh, g.halfTex[0], g.hw, g.hh, 1, radius, 0)
		pass(g.halfFbo[0], g.hw, g.hh, g.halfTex[1], g.hw, g.hh, 1, 0, radius)
	}
	// Screen-blend onto the scene: glow*(1-scene) + scene. Bright areas approach
	// white smoothly instead of blowing out, so the light spreads as a halo.
	r.EnableBlending(BlendAdd, BlendOneMinusDstColor, BlendOne)
	gl.Uniform1f(u["intensity"], intensity)
	pass(r.fbo, g.w, g.h, g.halfTex[0], g.hw, g.hh, 2, 0, 0)

	gl.BindTexture(gl.TEXTURE_2D, 0)
	gl.BindVertexArray(0)
	gl.Viewport(0, 0, g.w, g.h)
}
