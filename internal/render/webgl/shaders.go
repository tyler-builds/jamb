//go:build js

package webgl

// The shaders are GLSL ES 3.00 ports of the Vulkan renderer's GLSL
// (internal/render/vulkan/shaders), itself a port of the Direct3D 11
// renderer's HLSL; keep them in step. They differ in how they get their
// inputs: the instances of a frame are texels of one float texture (see
// Renderer.data), which each vertex reads with texelFetch, and the globals are
// uniforms. A page has no subpixel text, so there is no subpixel shader.

// dataWidth is the width of the instance texture, in texels of four floats.
const dataWidth = 2048

// version starts every shader; a vertex shader defines its STRIDE next.
const version = "#version 300 es\n"

const common = `precision highp float;
precision highp int;
precision highp sampler2D;

uniform vec2 u_viewport; // the target's size in pixels

const float PI = 3.14159265;

// WebGL's clip space has y pointing up, and pixels have it pointing down.
vec4 to_ndc(vec2 p) {
    return vec4(p.x / u_viewport.x * 2.0 - 1.0, 1.0 - p.y / u_viewport.y * 2.0, 0.0, 1.0);
}

vec4 premul(vec4 c) { return vec4(c.rgb * c.a, c.a); }

// radii = (tl, tr, br, bl); q is relative to the rect center.
float pick_radius(vec2 q, vec4 radii) {
    vec2 r = q.x < 0.0 ? vec2(radii.x, radii.w) : vec2(radii.y, radii.z);
    return q.y < 0.0 ? r.x : r.y;
}

float rrect_sdf(vec2 q, vec2 half_size, float r) {
    vec2 d = abs(q) - half_size + r;
    return length(max(d, 0.0)) + min(max(d.x, d.y), 0.0) - r;
}

// Anti-aliased coverage of the pixel centered at p by the clip rect (x0,y0,x1,y1)
// and, when radii is not zero, by the rounded clip shape (x,y,w,h) with corner
// radii (tl,tr,br,bl), which is not intersected with the clip rect.
float clip_coverage(vec2 p, vec4 clip, vec4 shape, vec4 radii) {
    vec2 a = clamp(p - clip.xy + 0.5, 0.0, 1.0);
    vec2 b = clamp(clip.zw - p + 0.5, 0.0, 1.0);
    float c = a.x * a.y * b.x * b.y;
    if (radii.x + radii.y + radii.z + radii.w > 0.0) {
        vec2 half_size = shape.zw * 0.5;
        vec2 q = p - (shape.xy + half_size);
        float r = min(pick_radius(q, radii), min(half_size.x, half_size.y));
        c *= clamp(0.5 - rrect_sdf(q, half_size, r), 0.0, 1.0);
    }
    return c;
}
`

// vertexCommon reads the instances. The draw's instances start at the texel
// u_base, and each takes STRIDE texels, which the shader defines first.
const vertexCommon = common + `
uniform sampler2D u_inst;
uniform int u_base;

vec4 inst(int k) {
    int i = u_base + gl_InstanceID * STRIDE + k;
    return texelFetch(u_inst, ivec2(i % 2048, i / 2048), 0); // 2048 is dataWidth
}

vec2 unit_corner() { return vec2(float(gl_VertexID & 1), float((gl_VertexID >> 1) & 1)); }
`

const quadVert = version + "#define STRIDE 11\n" + vertexCommon + `
out vec2 o_p;
flat out vec4 o_rect, o_clip, o_color, o_border, o_radii, o_params, o_color2, o_grad, o_sides, o_cshape, o_cradii;

void main() {
    vec4 rect = inst(0), clip = inst(1), params = inst(5);
    float pad = params.x > 0.0 ? ceil(params.x * 3.0) : 1.0;
    vec2 p = rect.xy - pad + unit_corner() * (rect.zw + 2.0 * pad);
    p = clamp(p, clip.xy - 1.0, clip.zw + 1.0);
    gl_Position = to_ndc(p);
    o_p = p;
    o_rect = rect; o_clip = clip; o_color = inst(2); o_border = inst(3); o_radii = inst(4);
    o_params = params; o_color2 = inst(6); o_grad = inst(7); o_sides = inst(8);
    o_cshape = inst(9); o_cradii = inst(10);
}
`

const quadFrag = version + common + `
in vec2 o_p;
flat in vec4 o_rect, o_clip, o_color, o_border, o_radii, o_params, o_color2, o_grad, o_sides, o_cshape, o_cradii;

out vec4 frag;

// Closed-form blurred rounded rectangle (after Evan Wallace).
vec2 erf2(vec2 v) {
    vec2 s = sign(v);
    vec2 a = abs(v);
    vec2 r1 = 1.0 + (0.278393 + (0.230389 + (0.000972 + 0.078108 * a) * a) * a) * a;
    vec2 r2 = r1 * r1;
    return s - s / (r2 * r2);
}

float gaussian(float x, float sigma) {
    return exp(-(x * x) / (2.0 * sigma * sigma)) / (sqrt(2.0 * PI) * sigma);
}

float blur_along_x(float x, float y, float sigma, float corner, vec2 half_size) {
    float delta = min(half_size.y - corner - abs(y), 0.0);
    float curved = half_size.x - corner + sqrt(max(0.0, corner * corner - delta * delta));
    vec2 integral = 0.5 + 0.5 * erf2((x + vec2(-curved, curved)) * (sqrt(0.5) / sigma));
    return integral.y - integral.x;
}

float shadow_alpha(vec2 q, vec2 half_size, float corner, float sigma) {
    float low = q.y - half_size.y;
    float high = q.y + half_size.y;
    float start = clamp(-3.0 * sigma, low, high);
    float end = clamp(3.0 * sigma, low, high);
    float stp = (end - start) / 4.0;
    float y = start + stp * 0.5;
    float alpha = 0.0;
    for (int k = 0; k < 4; k++) {
        alpha += blur_along_x(q.x, q.y - y, sigma, corner, half_size) * gaussian(y, sigma) * stp;
        y += stp;
    }
    return alpha;
}

vec3 srgb_to_linear(vec3 c) {
    return mix(pow((c + 0.055) / 1.055, vec3(2.4)), c / 12.92, vec3(lessThanEqual(c, vec3(0.04045))));
}

vec3 linear_to_srgb(vec3 c) {
    return mix(1.055 * pow(max(c, 0.0), vec3(1.0 / 2.4)) - 0.055, c * 12.92, vec3(lessThanEqual(c, vec3(0.0031308))));
}

vec3 linear_to_oklab(vec3 c) {
    float l = 0.4122214708 * c.r + 0.5363325363 * c.g + 0.0514459929 * c.b;
    float m = 0.2119034982 * c.r + 0.6806995451 * c.g + 0.1073969566 * c.b;
    float s = 0.0883024619 * c.r + 0.2817188376 * c.g + 0.6299787005 * c.b;
    l = sign(l) * pow(abs(l), 1.0 / 3.0);
    m = sign(m) * pow(abs(m), 1.0 / 3.0);
    s = sign(s) * pow(abs(s), 1.0 / 3.0);
    return vec3(0.2104542553 * l + 0.7936177850 * m - 0.0040720468 * s,
                1.9779984951 * l - 2.4285922050 * m + 0.4505937099 * s,
                0.0259040371 * l + 0.7827717662 * m - 0.8086757660 * s);
}

vec3 oklab_to_linear(vec3 lab) {
    float l = lab.x + 0.3963377774 * lab.y + 0.2158037573 * lab.z;
    float m = lab.x - 0.1055613458 * lab.y - 0.0638541728 * lab.z;
    float s = lab.x - 0.0894841775 * lab.y - 1.2914855480 * lab.z;
    l = l * l * l;
    m = m * m * m;
    s = s * s * s;
    return vec3( 4.0767416621 * l - 3.3077115913 * m + 0.2309699292 * s,
                -1.2684380046 * l + 2.6097574011 * m - 0.3413193965 * s,
                -0.0041960863 * l - 0.7034186147 * m + 1.7076147010 * s);
}

vec4 oklab_mix(vec4 a, vec4 b, float t) {
    vec3 lab = mix(linear_to_oklab(srgb_to_linear(a.rgb)), linear_to_oklab(srgb_to_linear(b.rgb)), t);
    return vec4(linear_to_srgb(clamp(oklab_to_linear(lab), 0.0, 1.0)), mix(a.a, b.a, t));
}

vec4 gradient_fill(vec2 q, vec2 half_size) {
    vec2 dir = o_grad.xy;
    float len = 2.0 * (abs(half_size.x * dir.x) + abs(half_size.y * dir.y));
    float t = dot(q, dir) / max(len, 1e-4) + 0.5;
    float span = o_grad.w - o_grad.z;
    t = clamp(span > 1e-6 ? (t - o_grad.z) / span : step(o_grad.z, t), 0.0, 1.0);
    if (o_params.z > 0.0) {
        return oklab_mix(o_color, o_color2, t);
    }
    return mix(o_color, o_color2, t);
}

float inner_coverage(vec2 q, vec2 half_size) {
    vec4 w = o_sides; // top, right, bottom, left
    vec2 inner_half = max(half_size - vec2(w.w + w.y, w.x + w.z) * 0.5, 0.0);
    vec2 center = vec2(w.w - w.y, w.x - w.z) * 0.5;
    float adj = 0.5 * ((q.x < 0.0 ? w.w : w.y) + (q.y < 0.0 ? w.x : w.z));
    float r = min(max(pick_radius(q, o_radii) - adj, 0.0), min(inner_half.x, inner_half.y));
    return clamp(0.5 - rrect_sdf(q - center, inner_half, r), 0.0, 1.0);
}

void main() {
    vec2 half_size = o_rect.zw * 0.5;
    vec2 q = o_p - (o_rect.xy + half_size);
    float r = min(pick_radius(q, o_radii), min(half_size.x, half_size.y));
    float cc = clip_coverage(o_p, o_clip, o_cshape, o_cradii);
    if (o_params.x > 0.0) {
        frag = premul(o_color) * shadow_alpha(q, half_size, r, o_params.x) * cc;
        return;
    }
    float d = rrect_sdf(q, half_size, r);
    float outer = clamp(0.5 - d, 0.0, 1.0);
    vec4 fill = premul(o_params.y > 0.0 ? gradient_fill(q, half_size) : o_color);
    if (any(greaterThan(o_sides, vec4(0.0)))) {
        float inner = min(inner_coverage(q, half_size), outer);
        vec4 b = premul(o_border);
        vec4 over = b + fill * (1.0 - b.a);
        frag = (fill * inner + over * (outer - inner)) * cc;
        return;
    }
    frag = fill * (outer * cc);
}
`

const maskVert = version + "#define STRIDE 6\n" + vertexCommon + `
out vec2 o_p, o_uv;
flat out vec4 o_color, o_clip, o_cshape, o_cradii;

void main() {
    vec2 c = unit_corner();
    vec4 rect = inst(0), uv = inst(1);
    vec2 p = rect.xy + c * rect.zw;
    gl_Position = to_ndc(p);
    o_p = p;
    o_uv = mix(uv.xy, uv.zw, c);
    o_color = inst(2); o_clip = inst(3); o_cshape = inst(4); o_cradii = inst(5);
}
`

const maskFrag = version + common + `
in vec2 o_p, o_uv;
flat in vec4 o_color, o_clip, o_cshape, o_cradii;
uniform sampler2D u_tex;

out vec4 frag;

void main() {
    float a = texture(u_tex, o_uv).r;
    // Contrast enhancement, as in the other renderers.
    float brightness = dot(o_color.rgb, vec3(0.30, 0.59, 0.11));
    float k = mix(0.3, 1.0, brightness);
    a = a * (k + 1.0) / (a * k + 1.0);
    frag = premul(o_color) * (a * clip_coverage(o_p, o_clip, o_cshape, o_cradii));
}
`

const imageVert = version + "#define STRIDE 7\n" + vertexCommon + `
out vec2 o_p, o_uv;
flat out vec4 o_rect, o_clip, o_radii, o_params, o_cshape, o_cradii;

void main() {
    vec2 c = unit_corner();
    vec4 rect = inst(0), uv = inst(1);
    vec2 p = rect.xy + c * rect.zw;
    gl_Position = to_ndc(p);
    o_p = p;
    o_uv = mix(uv.xy, uv.zw, c);
    o_rect = rect; o_clip = inst(2); o_radii = inst(3); o_params = inst(4);
    o_cshape = inst(5); o_cradii = inst(6);
}
`

const imageFrag = version + common + `
in vec2 o_p, o_uv;
flat in vec4 o_rect, o_clip, o_radii, o_params, o_cshape, o_cradii;
uniform sampler2D u_tex;

out vec4 frag;

void main() {
    vec2 half_size = o_rect.zw * 0.5;
    vec2 q = o_p - (o_rect.xy + half_size);
    float r = min(pick_radius(q, o_radii), min(half_size.x, half_size.y));
    float cov = clamp(0.5 - rrect_sdf(q, half_size, r), 0.0, 1.0);
    vec4 texel = texture(u_tex, o_uv); // premultiplied
    frag = texel * (o_params.x * cov * clip_coverage(o_p, o_clip, o_cshape, o_cradii));
}
`
