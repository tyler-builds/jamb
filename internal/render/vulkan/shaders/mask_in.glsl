// The inputs of the fragment shaders that draw coverage masks (mask.vert).
layout(location = 0) in vec2 i_p;
layout(location = 1) in vec2 i_uv;
layout(location = 2) flat in vec4 i_color;
layout(location = 3) flat in vec4 i_clip;
layout(location = 4) flat in vec4 i_cshape;
layout(location = 5) flat in vec4 i_cradii;

layout(set = 0, binding = 0) uniform sampler point_sampler;
layout(set = 1, binding = 0) uniform texture2D tex0;
