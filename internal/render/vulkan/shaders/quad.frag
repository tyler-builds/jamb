#version 450
#include "common.glsl"

layout(location = 0) in vec2 i_p;
layout(location = 1) flat in vec4 i_rect;
layout(location = 2) flat in vec4 i_clip;
layout(location = 3) flat in vec4 i_color;
layout(location = 4) flat in vec4 i_border;
layout(location = 5) flat in vec4 i_radii;
layout(location = 6) flat in vec4 i_params;
layout(location = 7) flat in vec4 i_color2;
layout(location = 8) flat in vec4 i_grad;
layout(location = 9) flat in vec4 i_sides;
layout(location = 10) flat in vec4 i_cshape;
layout(location = 11) flat in vec4 i_cradii;

layout(location = 0) out vec4 o_color;

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

// Gradient colors are mixed in linear light (sRGB mixing is the default, as in
// GPUI) or in Oklab, which keeps mixes between saturated colors bright.
vec3 srgb_to_linear(vec3 c) {
    return mix(pow((c + 0.055) / 1.055, vec3(2.4)), c / 12.92, lessThanEqual(c, vec3(0.04045)));
}

vec3 linear_to_srgb(vec3 c) {
    return mix(1.055 * pow(max(c, 0.0), vec3(1.0 / 2.4)) - 0.055, c * 12.92, lessThanEqual(c, vec3(0.0031308)));
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

// Straight-alpha colors a and b mixed by t in Oklab; alpha is mixed linearly.
vec4 oklab_mix(vec4 a, vec4 b, float t) {
    vec3 lab = mix(linear_to_oklab(srgb_to_linear(a.rgb)), linear_to_oklab(srgb_to_linear(b.rgb)), t);
    return vec4(linear_to_srgb(clamp(oklab_to_linear(lab), 0.0, 1.0)), mix(a.a, b.a, t));
}

// The fill color at q (relative to the box center). The gradient line runs
// through the center along the direction, with the length CSS uses: the box
// projected onto it, so the corners get the end colors.
vec4 gradient_fill(vec2 q, vec2 half_size) {
    vec2 dir = i_grad.xy;
    float len = 2.0 * (abs(half_size.x * dir.x) + abs(half_size.y * dir.y));
    float t = dot(q, dir) / max(len, 1e-4) + 0.5;
    float span = i_grad.w - i_grad.z;
    t = clamp(span > 1e-6 ? (t - i_grad.z) / span : step(i_grad.z, t), 0.0, 1.0);
    if (i_params.z > 0.0) {
        return oklab_mix(i_color, i_color2, t);
    }
    return mix(i_color, i_color2, t);
}

// The coverage of the border's inner edge at q (relative to the box center):
// the box inset by each side's width, with each corner's radius reduced by the
// average of its two sides (the rule innerCorners follows in the ui package).
float inner_coverage(vec2 q, vec2 half_size) {
    vec4 w = i_sides; // top, right, bottom, left
    vec2 inner_half = max(half_size - vec2(w.w + w.y, w.x + w.z) * 0.5, 0.0);
    vec2 center = vec2(w.w - w.y, w.x - w.z) * 0.5;
    float adj = 0.5 * ((q.x < 0.0 ? w.w : w.y) + (q.y < 0.0 ? w.x : w.z));
    float r = min(max(pick_radius(q, i_radii) - adj, 0.0), min(inner_half.x, inner_half.y));
    return clamp(0.5 - rrect_sdf(q - center, inner_half, r), 0.0, 1.0);
}

void main() {
    vec2 half_size = i_rect.zw * 0.5;
    vec2 q = i_p - (i_rect.xy + half_size);
    float r = min(pick_radius(q, i_radii), min(half_size.x, half_size.y));
    float cc = clip_coverage(i_p, i_clip, i_cshape, i_cradii);
    if (i_params.x > 0.0) {
        o_color = premul(i_color) * shadow_alpha(q, half_size, r, i_params.x) * cc;
        return;
    }
    float d = rrect_sdf(q, half_size, r);
    float outer = clamp(0.5 - d, 0.0, 1.0);
    vec4 fill = premul(i_params.y > 0.0 ? gradient_fill(q, half_size) : i_color);
    if (any(greaterThan(i_sides, vec4(0.0)))) {
        // The fill covers the inner edge and the border color the ring between
        // the two edges, so a side of zero width shows no border at its edge.
        float inner = min(inner_coverage(q, half_size), outer);
        vec4 b = premul(i_border);
        vec4 over = b + fill * (1.0 - b.a);
        o_color = (fill * inner + over * (outer - inner)) * cc;
        return;
    }
    o_color = fill * (outer * cc);
}
