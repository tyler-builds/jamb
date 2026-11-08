// Shared by every jamb Vulkan shader. It is a port of the HLSL of the Direct3D
// 11 renderer (internal/render/d3d11/shaders.go); keep the two in step.

layout(push_constant) uniform Globals {
    vec4 viewport; // x, y = target size in pixels
    vec4 textp;    // subpixel text: x = gamma, y = enhanced contrast, z = ClearType level
} g;

const float PI = 3.14159265;

// Vulkan's clip space has y pointing down, as pixels do.
vec4 to_ndc(vec2 p) {
    return vec4(p.x / g.viewport.x * 2.0 - 1.0, p.y / g.viewport.y * 2.0 - 1.0, 0.0, 1.0);
}

vec2 unit_corner(int vid) { return vec2(float(vid & 1), float((vid >> 1) & 1)); }

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
