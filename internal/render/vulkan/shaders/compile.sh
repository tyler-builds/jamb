#!/bin/sh
# Compiles the GLSL shaders to the SPIR-V files the Vulkan renderer embeds
# (spv/*.spv). Run it from any directory; it needs glslc (shaderc). The
# shaders workflow (.github/workflows/shaders.yml) runs it on every pull
# request that changes a shader and commits the result, so nobody needs
# glslc installed. The output depends on glslc's version: the workflow's
# (Ubuntu 24.04's shaderc) is the reference.
set -eu
cd "$(dirname "$0")"
mkdir -p spv
for src in *.vert *.frag; do
	glslc --target-env=vulkan1.1 -O -Werror -o "spv/$src.spv" "$src"
done
