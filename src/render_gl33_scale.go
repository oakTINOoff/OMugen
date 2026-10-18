//go:build !android

package main

// OMugen final scaling for the OpenGL 3.3 renderer. The frame normally has
// the size of the window's picture and is copied to it 1:1 (System.renderSize).
// When it doesn't (Keep Aspect Ratio off stretches it, a 4:3 fight inside a
// 16:9 frame, a fixed resolution), it is resampled with a sharp bilinear
// filter (Window Scale Mode: Nearest) or a bicubic one (Linear) instead of
// nearest pixels, which made uneven factors jagged.

import (
	_ "embed" // scaling shader

	gl "github.com/go-gl/gl/v3.3-core/gl"
)

//go:embed shaders/omugen_scale.frag.glsl
var omugenScaleFragShader string

type scaleGL33 struct {
	failed bool
	prog   *ShaderProgram_GL33
	vao    uint32
}

// finalScaleMode is how a srcW x srcH frame reaches a dstW x dstH picture:
// 0 as it is (same size, or nearest pixels by whole factors when Nearest is
// chosen), 1 sharp bilinear (Nearest, uneven factors), 2 bicubic (Linear,
// and any shrinking).
func finalScaleMode(srcW, srcH, dstW, dstH int32, linear bool) int32 {
	switch {
	case srcW <= 0 || srcH <= 0 || dstW <= 0 || dstH <= 0:
		return 0
	case srcW == dstW && srcH == dstH:
		return 0
	case linear || dstW < srcW || dstH < srcH:
		return 2
	case dstW%srcW == 0 && dstH%srcH == 0:
		return 0
	}
	return 1
}

func (r *Renderer_GL33) scaleReady() bool {
	s := &r.scale
	if s.prog != nil {
		return true
	}
	if s.failed {
		return false
	}
	prog, err := r.newShaderProgram(identVertShader, omugenScaleFragShader, "", "OMugen Scale", false)
	if err != nil {
		LogMessage("OMugen: final scaling filter unavailable (%v): nearest pixels", err)
		s.failed = true
		return false
	}
	prog.RegisterAttributes("VertCoord")
	prog.RegisterUniforms("srcSize", "dstSize", "mode")
	prog.RegisterTextures("Texture")
	s.prog = prog
	// Own VAO over the shared fullscreen quad, with this program's attribute location.
	gl.GenVertexArrays(1, &s.vao)
	gl.BindVertexArray(s.vao)
	gl.BindBuffer(gl.ARRAY_BUFFER, r.postVertBuffer)
	if loc, found := prog.attributes["VertCoord"]; found && loc >= 0 {
		gl.EnableVertexAttribArray(uint32(loc))
		gl.VertexAttribPointer(uint32(loc), 2, gl.FLOAT, false, 0, nil)
	}
	gl.BindVertexArray(0)
	gl.BindBuffer(gl.ARRAY_BUFFER, 0)
	return true
}

// drawScaled draws the texture bound to unit 0 (a srcW x srcH frame) into the
// current viewport (dstW x dstH) with the scaling filter, when the sizes call
// for one. It returns false when the frame should be drawn as it is.
func (r *Renderer_GL33) drawScaled(srcW, srcH, dstW, dstH int32) bool {
	mode := finalScaleMode(srcW, srcH, dstW, dstH, sys.cfg.Video.WindowScaleMode)
	if mode == 0 || !r.scaleReady() {
		return false
	}
	s := &r.scale
	r.ChangeProgram(s.prog.program)
	gl.BindVertexArray(s.vao)
	u := s.prog.uniforms
	gl.Uniform1i(u["Texture"], 0)
	gl.Uniform2f(u["srcSize"], float32(srcW), float32(srcH))
	gl.Uniform2f(u["dstSize"], float32(dstW), float32(dstH))
	gl.Uniform1i(u["mode"], mode)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.DrawArrays(gl.TRIANGLE_STRIP, 0, 4)
	return true
}
