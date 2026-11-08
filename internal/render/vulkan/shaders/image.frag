#version 450
#include "common.glsl"

layout(location = 0) in vec2 i_p;
layout(location = 1) in vec2 i_uv;
layout(location = 2) flat in vec4 i_rect;
layout(location = 3) flat in vec4 i_clip;
layout(location = 4) flat in vec4 i_radii;
layout(location = 5) flat in vec4 i_params;
layout(location = 6) flat in vec4 i_cshape;
layout(location = 7) flat in vec4 i_cradii;

layout(set = 0, binding = 1) uniform sampler linear_sampler;
layout(set = 1, binding = 0) uniform texture2D tex0;

layout(location = 0) out vec4 o_color;

void main() {
    vec2 half_size = i_rect.zw * 0.5;
    vec2 q = i_p - (i_rect.xy + half_size);
    float r = min(pick_radius(q, i_radii), min(half_size.x, half_size.y));
    float cov = clamp(0.5 - rrect_sdf(q, half_size, r), 0.0, 1.0);
    vec4 texel = texture(sampler2D(tex0, linear_sampler), i_uv); // premultiplied
    o_color = texel * (i_params.x * cov * clip_coverage(i_p, i_clip, i_cshape, i_cradii));
}
