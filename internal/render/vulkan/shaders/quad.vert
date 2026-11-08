#version 450
#include "common.glsl"

layout(location = 0) in vec4 i_rect;
layout(location = 1) in vec4 i_clip;
layout(location = 2) in vec4 i_color;
layout(location = 3) in vec4 i_border;
layout(location = 4) in vec4 i_radii;
layout(location = 5) in vec4 i_params; // x = shadow sigma, y = gradient, z = Oklab
layout(location = 6) in vec4 i_color2; // gradient end color
layout(location = 7) in vec4 i_grad;   // gradient: x, y = unit direction, z, w = stop positions
layout(location = 8) in vec4 i_sides;  // border widths: top, right, bottom, left
layout(location = 9) in vec4 i_cshape; // rounded clip: x, y, w, h
layout(location = 10) in vec4 i_cradii; // rounded clip: corner radii

layout(location = 0) out vec2 o_p;
layout(location = 1) flat out vec4 o_rect;
layout(location = 2) flat out vec4 o_clip;
layout(location = 3) flat out vec4 o_color;
layout(location = 4) flat out vec4 o_border;
layout(location = 5) flat out vec4 o_radii;
layout(location = 6) flat out vec4 o_params;
layout(location = 7) flat out vec4 o_color2;
layout(location = 8) flat out vec4 o_grad;
layout(location = 9) flat out vec4 o_sides;
layout(location = 10) flat out vec4 o_cshape;
layout(location = 11) flat out vec4 o_cradii;

void main() {
    float pad = i_params.x > 0.0 ? ceil(i_params.x * 3.0) : 1.0;
    vec2 p = i_rect.xy - pad + unit_corner(gl_VertexIndex) * (i_rect.zw + 2.0 * pad);
    p = clamp(p, i_clip.xy - 1.0, i_clip.zw + 1.0);
    gl_Position = to_ndc(p);
    o_p = p;
    o_rect = i_rect; o_clip = i_clip; o_color = i_color; o_border = i_border;
    o_radii = i_radii; o_params = i_params; o_color2 = i_color2; o_grad = i_grad;
    o_sides = i_sides; o_cshape = i_cshape; o_cradii = i_cradii;
}
