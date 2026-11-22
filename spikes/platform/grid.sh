#!/usr/bin/env bash
# Runs the platform spike's benchmark grid: each renderer (WebGL, DOM), on each
# monitor, with the default GL and with NVIDIA PRIME offload. Each run writes
# one JSON report; summary.sh turns a folder of reports into one table.
#
# Usage: ./grid.sh [report folder]   (BIN overrides the binary, default bin/platform)
set -euo pipefail

bin=${BIN:-bin/platform}
out=${1:-reports/$(date +%Y%m%d-%H%M%S)}
run_timeout=300 # seconds; a run that hangs must not stop the grid
offset=40       # window distance from the monitor's top-left corner

# glvnd reads only these variables: they make GL and EGL load NVIDIA's driver.
nvidia_env=(
  __NV_PRIME_RENDER_OFFLOAD=1
  __GLX_VENDOR_LIBRARY_NAME=nvidia
  __EGL_VENDOR_LIBRARY_FILENAMES=/usr/share/glvnd/egl_vendor.d/10_nvidia.json
)

[[ -x $bin ]] || { echo "no binary at $bin: build it first (wails3 task build)" >&2; exit 1; }
mkdir -p "$out"

# One line per monitor: "<name> <x> <y>", from lines such as
# " 0: +*HDMI-1-0 1920/527x1080/296+1920+0  HDMI-1-0".
mapfile -t monitors < <(xrandr --listmonitors | awk 'NR > 1 { split($3, g, "+"); print $4, g[2], g[3] }')

failed=0
for monitor in "${monitors[@]}"; do
  read -r name x y <<<"$monitor"
  for gl in default nvidia; do
    gl_env=()
    [[ $gl == nvidia ]] && gl_env=("${nvidia_env[@]}")
    for renderer in webgl dom; do
      label="$name $gl-gl $renderer"
      report="$out/${name}_${gl}_${renderer}.json"
      echo "run: $label"
      start=$SECONDS
      if env "${gl_env[@]}" timeout "$run_timeout" "$bin" \
        -autobench "$report" -renderer "$renderer" -label "$label" \
        -x $((x + offset)) -y $((y + offset)) >"$out/${name}_${gl}_${renderer}.log" 2>&1; then
        echo "done: $label ($((SECONDS - start)) s)"
      else
        echo "FAILED: $label (exit $?, log: $out/${name}_${gl}_${renderer}.log)"
        failed=$((failed + 1))
      fi
    done
  done
done

echo "reports: $out ($failed failed)"
"$(dirname "$0")/summary.sh" "$out"
