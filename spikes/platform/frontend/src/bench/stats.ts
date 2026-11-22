/** A summary of a set of samples, in the samples' unit. */
export interface Summary {
  n: number;
  min: number;
  p50: number;
  p95: number;
  p99: number;
  max: number;
  mean: number;
}

const round = (v: number) => Math.round(v * 100) / 100;

/** Nearest-rank percentile of sorted values. */
function percentile(sorted: number[], p: number): number {
  const rank = Math.ceil((p / 100) * sorted.length);
  return sorted[Math.min(sorted.length, Math.max(1, rank)) - 1];
}

export function summarize(values: number[]): Summary | null {
  if (values.length === 0) {
    return null;
  }
  const sorted = [...values].sort((a, b) => a - b);
  const mean = sorted.reduce((sum, v) => sum + v, 0) / sorted.length;
  return {
    n: sorted.length,
    min: round(sorted[0]),
    p50: round(percentile(sorted, 50)),
    p95: round(percentile(sorted, 95)),
    p99: round(percentile(sorted, 99)),
    max: round(sorted[sorted.length - 1]),
    mean: round(mean),
  };
}
