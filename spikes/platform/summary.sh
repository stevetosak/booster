#!/usr/bin/env bash
# Prints one Markdown table from a folder of platform spike reports.
# Typing latency is key to the next frame after the echo is drawn, p50 / p95 in ms.
# GL drivers are those WebKit's web process loaded (where the page renders); a loaded
# driver is not proof that it renders: nvidia-smi shows the process on the dGPU.
#
# Usage: ./summary.sh <report folder>
set -euo pipefail

dir=${1:?usage: summary.sh <report folder>}
shopt -s nullglob
reports=("$dir"/*.json)
((${#reports[@]})) || { echo "no reports in $dir" >&2; exit 1; }

echo "| Run | Renderer used | GL drivers loaded | DMA-BUF off | Plain MB/s | Colour MB/s | Typing, idle | Typing, beside 256 KB/s | Typing, beside 2 MB/s | Frames over 50 ms | Shell PSS idle (MB) | Errors |"
echo "|---|---|---|---|---|---|---|---|---|---|---|---|"
jq -r '
  def latency($load):
    [.page.latency[] | select(.loadKBps == $load) | .frameMs] | first
    | if . then "\(.p50) / \(.p95)" else "–" end;
  # The GL drivers WebKit'"'"'s web process loaded: radeonsi (AMD), nvidia, swrast (CPU).
  def gl_drivers:
    [.gpu[]? | select(.name | startswith("WebKitWebProces")) | .libraries[]] as $libs
    | [ (if any($libs[]; startswith("radeonsi")) then "radeonsi" else empty end),
        (if any($libs[]; startswith("libnvidia-eglcore")) then "nvidia" else empty end),
        (if any($libs[]; startswith("swrast") or startswith("kms_swrast")) then "swrast" else empty end) ]
    | if length == 0 then "–" else join(" + ") end;
  def throughput($gen):
    [.page.throughput[] | select(.name | startswith($gen)) | .mbPerSec] | first // "–";
  [ .label,
    ([.page.throughput[], .page.latency[] | .renderer.used] | unique | join(", ")),
    gl_drivers,
    (.env.WEBKIT_DISABLE_DMABUF_RENDERER // "–"),
    throughput("plain"), throughput("colour"),
    latency(0), latency(256), latency(2048),
    ([.page.throughput[], .page.latency[] | .frames.longFrames] | add // 0),
    (if .page.memory.idle then (.page.memory.idle.shellKB / 1024 | floor) else "–" end),
    (.page.errors | length)
  ] | map(tostring | gsub("\\|"; "/")) | "| " + join(" | ") + " |"
' "${reports[@]}"
