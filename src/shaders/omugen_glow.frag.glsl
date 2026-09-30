// OMugen glow (OpenGL 3.3). Paired with ident.vert.glsl (VertCoord -> texcoord).
// mode 0: 4-tap box downsample of the full-resolution glow capture
// mode 1: separable 9-tap Gaussian blur along dir (5 bilinear fetches)
// mode 2: composite, output = blurred glow * intensity (drawn with additive blending)
#ifdef GL_ES
	precision highp float;
#endif

out vec4 FragColor;
in vec2 texcoord;

uniform sampler2D Texture;
uniform vec2 texel;      // 1.0 / source texture size
uniform vec2 dir;        // blur direction, scaled by the blur radius
uniform float intensity; // composite strength
uniform int mode;

void main(void) {
	if (mode == 0) {
		vec4 c = texture(Texture, texcoord + texel * vec2(-0.5, -0.5));
		c += texture(Texture, texcoord + texel * vec2(0.5, -0.5));
		c += texture(Texture, texcoord + texel * vec2(-0.5, 0.5));
		c += texture(Texture, texcoord + texel * vec2(0.5, 0.5));
		FragColor = c * 0.25;
	} else if (mode == 1) {
		vec2 o1 = dir * texel * 1.3846153846;
		vec2 o2 = dir * texel * 3.2307692308;
		vec4 c = texture(Texture, texcoord) * 0.2270270270;
		c += (texture(Texture, texcoord + o1) + texture(Texture, texcoord - o1)) * 0.3162162162;
		c += (texture(Texture, texcoord + o2) + texture(Texture, texcoord - o2)) * 0.0702702703;
		FragColor = c;
	} else {
		vec3 glow = texture(Texture, texcoord).rgb * intensity;
		FragColor = vec4(glow, 1.0);
	}
}
