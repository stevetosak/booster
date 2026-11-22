import type { Terminal } from "@xterm/xterm";
import { summarize, type Summary } from "./stats";

export const sleep = (ms: number) => new Promise<void>((resolve) => setTimeout(resolve, ms));

/** Resolves at the start of the next animation frame. */
export const nextFrame = () => new Promise<number>((resolve) => requestAnimationFrame(resolve));

/**
 * Resolves with the time of the terminal's next render, or null when nothing
 * renders within timeoutMs.
 */
export function nextRender(term: Terminal, timeoutMs: number): Promise<number | null> {
  return new Promise((resolve) => {
    const timer = setTimeout(() => {
      sub.dispose();
      resolve(null);
    }, timeoutMs);
    const sub = term.onRender(() => {
      clearTimeout(timer);
      sub.dispose();
      resolve(performance.now());
    });
  });
}

/** Frame times while a phase runs; long frames are what a user sees as stutter. */
export interface FrameStats {
  intervalsMs: Summary | null;
  /** Frames that took longer than 50 ms. */
  longFrames: number;
}

const longFrameMs = 50;

/** Records the intervals between animation frames until stop is called. */
export class FrameSampler {
  private readonly intervals: number[] = [];
  private last = 0;
  private running = true;

  constructor() {
    requestAnimationFrame((t) => this.tick(t));
  }

  stop(): FrameStats {
    this.running = false;
    return {
      intervalsMs: summarize(this.intervals),
      longFrames: this.intervals.filter((v) => v > longFrameMs).length,
    };
  }

  private tick(t: number): void {
    if (!this.running) {
      return;
    }
    if (this.last > 0) {
      this.intervals.push(t - this.last);
    }
    this.last = t;
    requestAnimationFrame((next) => this.tick(next));
  }
}
