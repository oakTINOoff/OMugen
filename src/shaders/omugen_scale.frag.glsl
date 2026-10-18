// OMugen final scaling (OpenGL 3.3). Paired with ident.vert.glsl
// (VertCoord -> texcoord). Draws the finished frame into the window when its
// size differs from the window's picture (otherwise the frame is copied 1:1).
// The source texture is sampled with linear filtering.
// mode 1: sharp bilinear. Each source pixel stays a sharp square; only the
//         one-pixel seams between them are blended, so pixel art scaled by an
//         uneven factor keeps even pixels without blurring.
// mode 2: bicubic (Catmull-Rom, 9 bilinear fetches): clean and sharp at any
//         factor, also when shrinking.
#ifdef GL_ES
	precision highp float;
#endif

out vec4 FragColor;
in vec2 texcoord;

uniform sampler2D Texture;
uniform vec2 srcSize; // source texture size in pixels
uniform vec2 dstSize; // output size in pixels
uniform int mode;

vec4 sharpBilinear(vec2 uv) {
	vec2 texel = uv * srcSize;
	vec2 scale = max(dstSize / srcSize, vec2(1.0));
	vec2 base = floor(texel);
	vec2 center = fract(texel) - 0.5;
	// Inside a pixel: its own color; across the last 1/scale of it: blend.
	vec2 region = 0.5 - 0.5 / scale;
	vec2 f = (center - clamp(center, -region, region)) * scale + 0.5;
	return texture(Texture, (base + f) / srcSize);
}

vec4 catmullRom(vec2 uv) {
	vec2 pos = uv * srcSize;
	vec2 t1 = floor(pos - 0.5) + 0.5;
	vec2 f = pos - t1;
	vec2 w0 = f * (-0.5 + f * (1.0 - 0.5 * f));
	vec2 w1 = 1.0 + f * f * (-2.5 + 1.5 * f);
	vec2 w2 = f * (0.5 + f * (2.0 - 1.5 * f));
	vec2 w3 = f * f * (-0.5 + 0.5 * f);
	vec2 w12 = w1 + w2;
	vec2 t0 = (t1 - 1.0) / srcSize;
	vec2 t3 = (t1 + 2.0) / srcSize;
	vec2 t12 = (t1 + w2 / w12) / srcSize;
	vec4 c = vec4(0.0);
	c += texture(Texture, vec2(t0.x, t0.y)) * w0.x * w0.y;
	c += texture(Texture, vec2(t12.x, t0.y)) * w12.x * w0.y;
	c += texture(Texture, vec2(t3.x, t0.y)) * w3.x * w0.y;
	c += texture(Texture, vec2(t0.x, t12.y)) * w0.x * w12.y;
	c += texture(Texture, vec2(t12.x, t12.y)) * w12.x * w12.y;
	c += texture(Texture, vec2(t3.x, t12.y)) * w3.x * w12.y;
	c += texture(Texture, vec2(t0.x, t3.y)) * w0.x * w3.y;
	c += texture(Texture, vec2(t12.x, t3.y)) * w12.x * w3.y;
	c += texture(Texture, vec2(t3.x, t3.y)) * w3.x * w3.y;
	return clamp(c, 0.0, 1.0);
}

void main(void) {
	vec4 c = mode == 1 ? sharpBilinear(texcoord) : catmullRom(texcoord);
	FragColor = vec4(c.rgb, 1.0);
}
