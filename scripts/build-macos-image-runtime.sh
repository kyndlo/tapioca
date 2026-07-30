#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
package_root="${repo_root}/internal/imageruntime"
output_root="${1:-${repo_root}/dist/runtime/image}"

if ! xcrun -sdk macosx --find metallib >/dev/null 2>&1; then
  xcodebuild -downloadComponent MetalToolchain
fi

swift build -c release --package-path "${package_root}"

binary_dir="${package_root}/.build/release"
cmlx_root="${package_root}/.build/checkouts/mlx-swift/Source/Cmlx"
shader_root="${cmlx_root}/mlx-generated/metal"
air_root="${package_root}/.build/tapioca-metal-air"
mkdir -p "${air_root}" "${output_root}"

shaders=(
  arg_reduce.metal
  conv.metal
  gemv.metal
  layer_norm.metal
  random.metal
  rms_norm.metal
  rope.metal
  scaled_dot_product_attention.metal
  steel/attn/kernels/steel_attention.metal
)

air_files=()
for index in "${!shaders[@]}"; do
  shader="${shaders[$index]}"
  name=$(basename "${shader}" .metal)
  air="${air_root}/$(printf '%02d' "${index}")-${name}.air"
  xcrun -sdk macosx metal \
    -x metal -Wall -Wextra -fno-fast-math \
    -Wno-c++17-extensions -Wno-c++20-extensions \
    -mmacosx-version-min=14.0 \
    -c "${shader_root}/${shader}" -I"${cmlx_root}" -o "${air}"
  air_files+=("${air}")
done

xcrun -sdk macosx metallib "${air_files[@]}" -o "${output_root}/mlx.metallib"
cp "${binary_dir}/tapioca-image-runtime" "${output_root}/tapioca-image-runtime"
chmod +x "${output_root}/tapioca-image-runtime"
