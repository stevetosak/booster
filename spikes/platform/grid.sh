#!/usr/bin/env bash
# Runs the platform spike's benchmark grid: each renderer (WebGL, DOM), on each
# monitor, in four GL modes. Each run writes one JSON report and one log;
# summary.sh turns a folder of reports into one table.
#
# GL modes (GL_MODES chooses some, default all):
#   default  what Wails chooses. glvnd tries NVIDIA's EGL first, so WebKit's web
#            process renders on the NVIDIA GPU, and on X11 with the nvidia module
#            loaded Wails turns WebKit's DMA-BUF renderer off
#   dmabuf   the same, with the DMA-BUF renderer kept on (Wails does not
#            override a value that is set)
#   nvidia   PRIME offload to the NVIDIA GPU (Wails turns DMA-BUF off)
#   amd      EGL and GLX see only Mesa, so the AMD GPU renders; Wails then
#            keeps DMA-BUF on
#
# Usage: [GL_MODES="amd"] ./grid.sh [report folder]   (BIN overrides the binary, default bin/platform)
# Runs take about 50 s each (16 runs for all modes); the windows open on the monitors, so leave them alone.
set -euo pipefail

bin=${BIN:-bin/platform}
out=${1:-reports/$(date +%Y%m%d-%H%M%S)}
run_timeout=300 # seconds; a run that hangs must not stop the grid
offset=40       # window distance from the monitor's top-left corner

read -r -a gl_modes <<<"${GL_MODES:-default dmabuf nvidia amd}"

# glvnd reads only these variables: they choose the vendor libraries of GL and EGL.
nvidia_env=(
  __NV_PRIME_RENDER_OFFLOAD=1
  __GLX_VENDOR_LIBRARY_NAME=nvidia
  __EGL_VENDOR_LIBRARY_FILENAMES=/usr/share/glvnd/egl_vendor.d/10_nvidia.json
)
amd_env=(
  __GLX_VENDOR_LIBRARY_NAME=mesa
  __EGL_VENDOR_LIBRARY_FILENAMES=/usr/share/glvnd/egl_vendor.d/50_mesa.json
)

[[ -x $bin ]] || { echo "no binary at $bin: build it first (wails3 task build)" >&2; exit 1; }
mkdir -p "$out"

# One line per monitor: "<name> <x> <y>", from lines such as
# " 0: +*HDMI-1-0 1920/527x1080/296+1920+0  HDMI-1-0".
mapfile -t monitors < <(xrandr --listmonitors | awk 'NR > 1 { split($3, g, "+"); print $4, g[2], g[3] }')

failed=0
for monitor in "${monitors[@]}"; do
  read -r name x y <<<"$monitor"
  for gl in "${gl_modes[@]}"; do
    case $gl in
      default) gl_env=() ;;
      dmabuf) gl_env=(WEBKIT_DISABLE_DMABUF_RENDERER=0) ;;
      nvidia) gl_env=("${nvidia_env[@]}") ;;
      amd) gl_env=("${amd_env[@]}") ;;
      *) echo "unknown GL mode $gl" >&2; exit 2 ;;
    esac
    for renderer in webgl dom; do
      label="$name $gl $renderer"
      base="$out/${name}_${gl}_${renderer}"
      echo "run: $label"
      start=$SECONDS
      # The full log goes to a file; faults and the end of the run also go to
      # stdout, so a watcher sees them at once.
      set +e
      env "${gl_env[@]}" timeout "$run_timeout" "$bin" \
        -autobench "$base.json" -renderer "$renderer" -label "$label" \
        -x $((x + offset)) -y $((y + offset)) 2>&1 |
        tee "$base.log" | grep --line-buffered -E 'progress: done|refused|rror|panic|fatal|stopped' | sed -u 's/^/  /'
      status=${PIPESTATUS[0]}
      set -e
      if ((status == 0)) && [[ -s $base.json ]]; then
        echo "done: $label ($((SECONDS - start)) s)"
      else
        echo "FAILED: $label (exit $status, log: $base.log)"
        failed=$((failed + 1))
      fi
    done
  done
done

echo "reports: $out ($failed failed)"
"$(dirname "$0")/summary.sh" "$out"
