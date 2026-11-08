// Coverage correction of subpixel (ClearType) glyphs; see the HLSL in
// internal/render/d3d11/shaders.go for how the knots were fitted.

// Piecewise-linear interpolation of a value given at text intensity 0, 0.6, 1.
float knot(vec3 v, float f) {
    return f < 0.6 ? mix(v.x, v.y, f / 0.6) : mix(v.y, v.z, (f - 0.6) / 0.4);
}

float correct_channel(float a, float f, float gscale, float cscale) {
    float k = max(0.0, knot(vec3(0.25, 0.45, 0.07), f)) * cscale;
    a = a * (k + 1.0) / (a * k + 1.0);
    float x = knot(vec3(-0.6184, 0.0252, 0.0654), f);
    float y = knot(vec3(-0.7300, -0.6821, -0.7300), f);
    float z = knot(vec3(0.8439, 1.5000, 1.1021), f);
    float w = knot(vec3(-0.2500, -0.0500, -0.2500), f);
    return a + a * (1.0 - a) * gscale * ((x * f + y) * a + (z * f + w));
}

vec3 correct_coverage(vec3 cov, vec3 fg) {
    float gscale = (g.textp.x - 1.0) / 0.8;
    float cscale = g.textp.y / 0.5;
    vec3 c = vec3(correct_channel(cov.r, fg.r, gscale, cscale),
                  correct_channel(cov.g, fg.g, gscale, cscale),
                  correct_channel(cov.b, fg.b, gscale, cscale));
    float gray = dot(c, vec3(1.0 / 3.0));
    return clamp(mix(vec3(gray), c, g.textp.z), 0.0, 1.0);
}
