#version 450
#include "common.glsl"

layout(location = 0) in vec4 i_rect;
layout(location = 1) in vec4 i_uv;
layout(location = 2) in vec4 i_clip;
layout(location = 3) in vec4 i_radii;
layout(location = 4) in vec4 i_params; // x = opacity
layout(location = 5) in vec4 i_cshape;
layout(location = 6) in vec4 i_cradii;

layout(location = 0) out vec2 o_p;
layout(location = 1) out vec2 o_uv;
layout(location = 2) flat out vec4 o_rect;
layout(location = 3) flat out vec4 o_clip;
layout(location = 4) flat out vec4 o_radii;
layout(location = 5) flat out vec4 o_params;
layout(location = 6) flat out vec4 o_cshape;
layout(location = 7) flat out vec4 o_cradii;

void main() {
    vec2 c = unit_corner(gl_VertexIndex);
    vec2 p = i_rect.xy + c * i_rect.zw;
    gl_Position = to_ndc(p);
    o_p = p;
    o_uv = mix(i_uv.xy, i_uv.zw, c);
    o_rect = i_rect; o_clip = i_clip; o_radii = i_radii; o_params = i_params;
    o_cshape = i_cshape; o_cradii = i_cradii;
}
