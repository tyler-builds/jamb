#version 450
#include "common.glsl"
#include "mask_in.glsl"
#include "subpixel.glsl"

// Fallback without dual-source blending: average the channels to grayscale.
layout(location = 0) out vec4 o_color;

void main() {
    float a = dot(correct_coverage(texture(sampler2D(tex0, point_sampler), i_uv).rgb, i_color.rgb), vec3(1.0 / 3.0));
    o_color = premul(i_color) * (a * clip_coverage(i_p, i_clip, i_cshape, i_cradii));
}
