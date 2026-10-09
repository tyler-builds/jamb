//go:build windows

package d3d11

// shaderSource contains all three pipelines. It is compiled once at startup
// with d3dcompiler_47.dll (part of Windows 10+) and cached on disk.
//
// Every primitive is an instanced 4-vertex triangle strip; per-instance data
// arrives through vertex buffer slot 0. All colors are output premultiplied.
const shaderSource = `
cbuffer Globals : register(b0) {
    float4 viewport;
    float4 textp; // subpixel text: x = gamma, y = enhanced contrast, z = ClearType level
};

Texture2D tex0 : register(t0);
SamplerState pointSampler : register(s0);
SamplerState linearSampler : register(s1);

static const float PI = 3.14159265;

float4 to_ndc(float2 p) {
    return float4(p.x / viewport.x * 2.0 - 1.0, 1.0 - p.y / viewport.y * 2.0, 0.0, 1.0);
}

float2 unit_corner(uint vid) { return float2(vid & 1, (vid >> 1) & 1); }

float4 premul(float4 c) { return float4(c.rgb * c.a, c.a); }

// radii = (tl, tr, br, bl); q is relative to the rect center.
float pick_radius(float2 q, float4 radii) {
    float2 r = q.x < 0.0 ? float2(radii.x, radii.w) : float2(radii.y, radii.z);
    return q.y < 0.0 ? r.x : r.y;
}

float rrect_sdf(float2 q, float2 half_size, float r) {
    float2 d = abs(q) - half_size + r;
    return length(max(d, 0.0)) + min(max(d.x, d.y), 0.0) - r;
}

// Anti-aliased coverage of the pixel centered at p by the clip rect (x0,y0,x1,y1)
// and, when radii is not zero, by the rounded clip shape (x,y,w,h) with corner
// radii (tl,tr,br,bl), which is not intersected with the clip rect.
float clip_coverage(float2 p, float4 clip, float4 shape, float4 radii) {
    float2 a = saturate(p - clip.xy + 0.5);
    float2 b = saturate(clip.zw - p + 0.5);
    float c = a.x * a.y * b.x * b.y;
    if (radii.x + radii.y + radii.z + radii.w > 0.0) {
        float2 half_size = shape.zw * 0.5;
        float2 q = p - (shape.xy + half_size);
        float r = min(pick_radius(q, radii), min(half_size.x, half_size.y));
        c *= saturate(0.5 - rrect_sdf(q, half_size, r));
    }
    return c;
}

// Closed-form blurred rounded rectangle (after Evan Wallace).
float2 erf2(float2 v) {
    float2 s = sign(v);
    float2 a = abs(v);
    float2 r1 = 1.0 + (0.278393 + (0.230389 + (0.000972 + 0.078108 * a) * a) * a) * a;
    float2 r2 = r1 * r1;
    return s - s / (r2 * r2);
}

float gaussian(float x, float sigma) {
    return exp(-(x * x) / (2.0 * sigma * sigma)) / (sqrt(2.0 * PI) * sigma);
}

float blur_along_x(float x, float y, float sigma, float corner, float2 half_size) {
    float delta = min(half_size.y - corner - abs(y), 0.0);
    float curved = half_size.x - corner + sqrt(max(0.0, corner * corner - delta * delta));
    float2 integral = 0.5 + 0.5 * erf2((x + float2(-curved, curved)) * (sqrt(0.5) / sigma));
    return integral.y - integral.x;
}

float shadow_alpha(float2 q, float2 half_size, float corner, float sigma) {
    float low = q.y - half_size.y;
    float high = q.y + half_size.y;
    float start = clamp(-3.0 * sigma, low, high);
    float end = clamp(3.0 * sigma, low, high);
    float step = (end - start) / 4.0;
    float y = start + step * 0.5;
    float alpha = 0.0;
    [unroll] for (int k = 0; k < 4; k++) {
        alpha += blur_along_x(q.x, q.y - y, sigma, corner, half_size) * gaussian(y, sigma) * step;
        y += step;
    }
    return alpha;
}

// ---- quads, borders, shadows and gradients ----

// Gradient colors are mixed in linear light (sRGB mixing is the default, as in
// GPUI) or in Oklab, which keeps mixes between saturated colors bright.
float3 srgb_to_linear(float3 c) {
    return c <= 0.04045 ? c / 12.92 : pow((c + 0.055) / 1.055, 2.4);
}

float3 linear_to_srgb(float3 c) {
    return c <= 0.0031308 ? c * 12.92 : 1.055 * pow(max(c, 0.0), 1.0 / 2.4) - 0.055;
}

float3 linear_to_oklab(float3 c) {
    float l = 0.4122214708 * c.r + 0.5363325363 * c.g + 0.0514459929 * c.b;
    float m = 0.2119034982 * c.r + 0.6806995451 * c.g + 0.1073969566 * c.b;
    float s = 0.0883024619 * c.r + 0.2817188376 * c.g + 0.6299787005 * c.b;
    l = sign(l) * pow(abs(l), 1.0 / 3.0);
    m = sign(m) * pow(abs(m), 1.0 / 3.0);
    s = sign(s) * pow(abs(s), 1.0 / 3.0);
    return float3(0.2104542553 * l + 0.7936177850 * m - 0.0040720468 * s,
                  1.9779984951 * l - 2.4285922050 * m + 0.4505937099 * s,
                  0.0259040371 * l + 0.7827717662 * m - 0.8086757660 * s);
}

float3 oklab_to_linear(float3 lab) {
    float l = lab.x + 0.3963377774 * lab.y + 0.2158037573 * lab.z;
    float m = lab.x - 0.1055613458 * lab.y - 0.0638541728 * lab.z;
    float s = lab.x - 0.0894841775 * lab.y - 1.2914855480 * lab.z;
    l = l * l * l;
    m = m * m * m;
    s = s * s * s;
    return float3( 4.0767416621 * l - 3.3077115913 * m + 0.2309699292 * s,
                  -1.2684380046 * l + 2.6097574011 * m - 0.3413193965 * s,
                  -0.0041960863 * l - 0.7034186147 * m + 1.7076147010 * s);
}

// Straight-alpha colors a and b mixed by t in Oklab; alpha is mixed linearly.
float4 oklab_mix(float4 a, float4 b, float t) {
    float3 lab = lerp(linear_to_oklab(srgb_to_linear(a.rgb)), linear_to_oklab(srgb_to_linear(b.rgb)), t);
    return float4(linear_to_srgb(saturate(oklab_to_linear(lab))), lerp(a.a, b.a, t));
}

struct QuadIn {
    float4 rect   : RECT;
    float4 clip   : CLIP;
    float4 color  : COLOR0;
    float4 border : COLOR1;
    float4 radii  : RADII;
    float4 params : PARAMS; // x = shadow sigma, y = gradient, z = Oklab
    float4 color2 : COLOR2; // gradient end color
    float4 grad   : GRAD;   // gradient: x, y = unit direction, z, w = stop positions
    float4 sides  : SIDES;  // border widths: top, right, bottom, left
    float4 cshape : CSHAPE; // rounded clip: x, y, w, h
    float4 cradii : CRADII; // rounded clip: corner radii
    uint vid : SV_VertexID;
};

struct QuadOut {
    float4 pos : SV_Position;
    float2 p : PIXEL;
    nointerpolation float4 rect   : RECT;
    nointerpolation float4 clip   : CLIP;
    nointerpolation float4 color  : COLOR0;
    nointerpolation float4 border : COLOR1;
    nointerpolation float4 radii  : RADII;
    nointerpolation float4 params : PARAMS;
    nointerpolation float4 color2 : COLOR2;
    nointerpolation float4 grad   : GRAD;
    nointerpolation float4 sides  : SIDES;
    nointerpolation float4 cshape : CSHAPE;
    nointerpolation float4 cradii : CRADII;
};

QuadOut quad_vs(QuadIn i) {
    QuadOut o;
    float pad = i.params.x > 0.0 ? ceil(i.params.x * 3.0) : 1.0;
    float2 p = i.rect.xy - pad + unit_corner(i.vid) * (i.rect.zw + 2.0 * pad);
    p = clamp(p, i.clip.xy - 1.0, i.clip.zw + 1.0);
    o.pos = to_ndc(p);
    o.p = p;
    o.rect = i.rect; o.clip = i.clip; o.color = i.color; o.border = i.border;
    o.radii = i.radii; o.params = i.params; o.color2 = i.color2; o.grad = i.grad;
    o.sides = i.sides; o.cshape = i.cshape; o.cradii = i.cradii;
    return o;
}

// The fill color at q (relative to the box center). The gradient line runs
// through the center along the direction, with the length CSS uses: the box
// projected onto it, so the corners get the end colors.
float4 gradient_fill(QuadOut i, float2 q, float2 half_size) {
    float2 dir = i.grad.xy;
    float len = 2.0 * (abs(half_size.x * dir.x) + abs(half_size.y * dir.y));
    float t = dot(q, dir) / max(len, 1e-4) + 0.5;
    float span = i.grad.w - i.grad.z;
    t = saturate(span > 1e-6 ? (t - i.grad.z) / span : step(i.grad.z, t));
    if (i.params.z > 0.0) {
        return oklab_mix(i.color, i.color2, t);
    }
    return lerp(i.color, i.color2, t);
}

// The coverage of the border's inner edge at q (relative to the box center):
// the box inset by each side's width, with each corner's radius reduced by the
// average of its two sides (the rule innerCorners follows in the ui package).
float inner_coverage(QuadOut i, float2 q, float2 half_size) {
    float4 w = i.sides; // top, right, bottom, left
    float2 inner_half = max(half_size - float2(w.w + w.y, w.x + w.z) * 0.5, 0.0);
    float2 center = float2(w.w - w.y, w.x - w.z) * 0.5;
    float adj = 0.5 * ((q.x < 0.0 ? w.w : w.y) + (q.y < 0.0 ? w.x : w.z));
    float r = min(max(pick_radius(q, i.radii) - adj, 0.0), min(inner_half.x, inner_half.y));
    return saturate(0.5 - rrect_sdf(q - center, inner_half, r));
}

float4 quad_ps(QuadOut i) : SV_Target {
    float2 half_size = i.rect.zw * 0.5;
    float2 q = i.p - (i.rect.xy + half_size);
    float r = min(pick_radius(q, i.radii), min(half_size.x, half_size.y));
    float cc = clip_coverage(i.p, i.clip, i.cshape, i.cradii);
    if (i.params.x > 0.0) {
        return premul(i.color) * shadow_alpha(q, half_size, r, i.params.x) * cc;
    }
    float d = rrect_sdf(q, half_size, r);
    float outer = saturate(0.5 - d);
    float4 fill = premul(i.params.y > 0.0 ? gradient_fill(i, q, half_size) : i.color);
    if (any(i.sides > 0.0)) {
        // The fill covers the inner edge and the border color the ring between
        // the two edges, so a side of zero width shows no border at its edge.
        float inner = min(inner_coverage(i, q, half_size), outer);
        float4 b = premul(i.border);
        float4 over = b + fill * (1.0 - b.a);
        return (fill * inner + over * (outer - inner)) * cc;
    }
    return fill * (outer * cc);
}

// ---- coverage masks: glyphs and icons ----

struct MaskIn {
    float4 rect   : RECT;
    float4 uv     : UV;
    float4 color  : COLOR0;
    float4 clip   : CLIP;
    float4 cshape : CSHAPE;
    float4 cradii : CRADII;
    uint vid : SV_VertexID;
};

struct MaskOut {
    float4 pos : SV_Position;
    float2 p : PIXEL;
    float2 uv : UV;
    nointerpolation float4 color  : COLOR0;
    nointerpolation float4 clip   : CLIP;
    nointerpolation float4 cshape : CSHAPE;
    nointerpolation float4 cradii : CRADII;
};

MaskOut mask_vs(MaskIn i) {
    MaskOut o;
    float2 c = unit_corner(i.vid);
    float2 p = i.rect.xy + c * i.rect.zw;
    o.pos = to_ndc(p);
    o.p = p;
    o.uv = lerp(i.uv.xy, i.uv.zw, c);
    o.color = i.color;
    o.clip = i.clip;
    o.cshape = i.cshape;
    o.cradii = i.cradii;
    return o;
}

float4 mask_ps(MaskOut i) : SV_Target {
    float a = tex0.Sample(pointSampler, i.uv).r;
    // Contrast enhancement (as in WebRender and GPUI). Coverage is blended
    // in sRGB space, which makes anti-aliased edges too faint and text look
    // thin and soft, more so for light text on dark backgrounds. Boosting
    // mid-coverage pixels restores the intended weight; k grows with the
    // text's brightness.
    float brightness = dot(i.color.rgb, float3(0.30, 0.59, 0.11));
    float k = lerp(0.3, 1.0, brightness);
    a = a * (k + 1.0) / (a * k + 1.0);
    return premul(i.color) * (a * clip_coverage(i.p, i.clip, i.cshape, i.cradii));
}

// ---- subpixel (ClearType) glyphs ----

// Piecewise-linear interpolation of a value given at text intensity 0, 0.6, 1.
float knot(float3 v, float f) {
    return f < 0.6 ? lerp(v.x, v.y, f / 0.6) : lerp(v.y, v.z, (f - 0.6) / 0.4);
}

// Coverage correction for one channel, for blending in sRGB space. It has the
// form of DirectWrite's: an enhanced-contrast step, then a gamma-dependent
// alpha correction a + a(1-a)((x f + y) a + (z f + w)) driven by the text
// color's channel value f. DirectWrite's coefficient table is not public, so
// the knots below were fitted against DirectWrite ClearType output (see
// TestClearTypeReference) at its default gamma 1.8 and contrast 0.5. The
// system's gamma scales the correction (none at 1.0) and contrast scales k.
float correct_channel(float a, float f, float gscale, float cscale) {
    float k = max(0.0, knot(float3(0.25, 0.45, 0.07), f)) * cscale;
    a = a * (k + 1.0) / (a * k + 1.0);
    float x = knot(float3(-0.6184, 0.0252, 0.0654), f);
    float y = knot(float3(-0.7300, -0.6821, -0.7300), f);
    float z = knot(float3(0.8439, 1.5000, 1.1021), f);
    float w = knot(float3(-0.2500, -0.0500, -0.2500), f);
    return a + a * (1.0 - a) * gscale * ((x * f + y) * a + (z * f + w));
}

float3 correct_coverage(float3 cov, float3 fg) {
    float gscale = (textp.x - 1.0) / 0.8;
    float cscale = textp.y / 0.5;
    float3 c = float3(correct_channel(cov.r, fg.r, gscale, cscale),
                      correct_channel(cov.g, fg.g, gscale, cscale),
                      correct_channel(cov.b, fg.b, gscale, cscale));
    float gray = dot(c, 1.0 / 3.0);
    return saturate(lerp(gray, c, textp.z));
}

struct SubpixelOut {
    float4 color : SV_Target0;
    float4 coverage : SV_Target1;
};

// Dual-source output: color is the text color premultiplied by each channel's
// coverage, coverage is what the blend state removes from the destination.
SubpixelOut subpixel_ps(MaskOut i) {
    float3 cov = correct_coverage(tex0.Sample(pointSampler, i.uv).rgb, i.color.rgb);
    float3 a = cov * (i.color.a * clip_coverage(i.p, i.clip, i.cshape, i.cradii));
    SubpixelOut o;
    o.color = float4(i.color.rgb * a, dot(a, 1.0 / 3.0));
    o.coverage = float4(a, dot(a, 1.0 / 3.0));
    return o;
}

// Fallback without dual-source blending: average the channels to grayscale.
float4 subpixel_gray_ps(MaskOut i) : SV_Target {
    float a = dot(correct_coverage(tex0.Sample(pointSampler, i.uv).rgb, i.color.rgb), 1.0 / 3.0);
    return premul(i.color) * (a * clip_coverage(i.p, i.clip, i.cshape, i.cradii));
}

// ---- images ----

struct ImageIn {
    float4 rect   : RECT;
    float4 uv     : UV;
    float4 clip   : CLIP;
    float4 radii  : RADII;
    float4 params : PARAMS; // x = opacity
    float4 cshape : CSHAPE;
    float4 cradii : CRADII;
    uint vid : SV_VertexID;
};

struct ImageOut {
    float4 pos : SV_Position;
    float2 p : PIXEL;
    float2 uv : UV;
    nointerpolation float4 rect   : RECT;
    nointerpolation float4 clip   : CLIP;
    nointerpolation float4 radii  : RADII;
    nointerpolation float4 params : PARAMS;
    nointerpolation float4 cshape : CSHAPE;
    nointerpolation float4 cradii : CRADII;
};

ImageOut image_vs(ImageIn i) {
    ImageOut o;
    float2 c = unit_corner(i.vid);
    float2 p = i.rect.xy + c * i.rect.zw;
    o.pos = to_ndc(p);
    o.p = p;
    o.uv = lerp(i.uv.xy, i.uv.zw, c);
    o.rect = i.rect; o.clip = i.clip; o.radii = i.radii; o.params = i.params;
    o.cshape = i.cshape; o.cradii = i.cradii;
    return o;
}

float4 image_ps(ImageOut i) : SV_Target {
    float2 half_size = i.rect.zw * 0.5;
    float2 q = i.p - (i.rect.xy + half_size);
    float r = min(pick_radius(q, i.radii), min(half_size.x, half_size.y));
    float cov = saturate(0.5 - rrect_sdf(q, half_size, r));
    float4 texel = tex0.Sample(linearSampler, i.uv); // premultiplied
    return texel * (i.params.x * cov * clip_coverage(i.p, i.clip, i.cshape, i.cradii));
}
`
