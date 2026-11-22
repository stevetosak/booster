import type { SpikeParams } from "../params";
import { TerminalSession, type RendererInfo } from "../terminal/session";
import { FrameSampler, nextRender, type FrameStats } from "./timing";

export interface ThroughputCase {
  name: string;
  gen: "plain" | "color";
  bytes: number;
}

export interface ThroughputResult {
  name: string;
  bytes: number;
  /** From the program start to the last byte parsed and rendered. */
  ms: number;
  mbPerSec: number;
  frames: FrameStats;
  renderer: RendererInfo;
}

const renderTimeoutMs = 1000;

/**
 * Streams a fixed amount of output through one terminal at full speed and
 * measures how long the terminal takes to parse and render all of it.
 */
export async function measureThroughput(host: HTMLElement, params: SpikeParams, c: ThroughputCase): Promise<ThroughputResult> {
  const session = await TerminalSession.open(host, params, { kind: "gen", gen: c.gen, bytes: c.bytes, rateKBps: 0 });
  try {
    const start = performance.now();
    const frames = new FrameSampler();
    await session.exited;
    await session.drained();
    const rendered = (await nextRender(session.term, renderTimeoutMs)) ?? performance.now();
    const ms = rendered - start;
    const bytes = session.bytesReceived();
    return {
      name: c.name,
      bytes,
      ms: Math.round(ms),
      mbPerSec: Math.round((bytes / (1024 * 1024) / (ms / 1000)) * 100) / 100,
      frames: frames.stop(),
      renderer: session.rendererInfo(),
    };
  } finally {
    session.dispose();
  }
}
