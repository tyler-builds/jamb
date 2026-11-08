#version 450
#include "common.glsl"
#include "mask_in.glsl"

layout(location = 0) out vec4 o_color;

void main() {
    float a = texture(sampler2D(tex0, point_sampler), i_uv).r;
    // Contrast enhancement (as in WebRender and GPUI). Coverage is blended
    // in sRGB space, which makes anti-aliased edges too faint and text look
    // thin and soft, more so for light text on dark backgrounds. Boosting
    // mid-coverage pixels restores the intended weight; k grows with the
    // text's brightness.
    float brightness = dot(i_color.rgb, vec3(0.30, 0.59, 0.11));
    float k = mix(0.3, 1.0, brightness);
    a = a * (k + 1.0) / (a * k + 1.0);
    o_color = premul(i_color) * (a * clip_coverage(i_p, i_clip, i_cshape, i_cradii));
}
