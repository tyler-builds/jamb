#version 450
#include "common.glsl"
#include "mask_in.glsl"
#include "subpixel.glsl"

// Dual-source output: color is the text color premultiplied by each channel's
// coverage, coverage is what the blend state removes from the destination.
layout(location = 0, index = 0) out vec4 o_color;
layout(location = 0, index = 1) out vec4 o_coverage;

void main() {
    vec3 cov = correct_coverage(texture(sampler2D(tex0, point_sampler), i_uv).rgb, i_color.rgb);
    vec3 a = cov * (i_color.a * clip_coverage(i_p, i_clip, i_cshape, i_cradii));
    o_color = vec4(i_color.rgb * a, dot(a, vec3(1.0 / 3.0)));
    o_coverage = vec4(a, dot(a, vec3(1.0 / 3.0)));
}
