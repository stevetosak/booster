import type { SpikeParams } from "../params";
import { TerminalSession, type RendererInfo } from "../terminal/session";
import { summarize, type Summary } from "./stats";
import { FrameSampler, nextFrame, nextRender, sleep, type FrameStats } from "./timing";

export interface LatencyCase {
  name: string;
  samples: number;
  /** Output rate of a second terminal that streams while the user types; 0 is none. */
  loadKBps: number;
}

export interface LatencyResult {
  name: string;
  loadKBps: number;
  samples: number;
  /** Keys whose echo did not arrive or did not render in time. */
  timeouts: number;
  /** Key to echo parsed into the buffer. */
  parsedMs: Summary | null;
  /** Key to echo drawn by the renderer. */
  renderedMs: Summary | null;
  /** Key to the start of the frame after the draw: an upper bound of the time to the screen. */
  frameMs: Summary | null;
  frames: FrameStats;
  /** The typing terminal's renderer at the end of the run. */
  renderer: RendererInfo;
}

export interface BenchSlots {
  primary: HTMLElement;
  secondary: HTMLElement;
}

// The terminal runs cat in canonical mode: the kernel's line discipline echoes
// each key at once, so the measured path is the webview's and not a program's.
const lineLength = 60;
const newlineSettleMs = 150;
const loadSettleMs = 500;
const echoTimeoutMs = 2000;
const minGapMs = 40;
const maxGapMs = 90;

const letters = "abcdefghijklmnopqrstuvwxyz";

interface KeySample {
  parsed: number;
  rendered: number;
  frame: number;
}

/**
 * Types keys into a terminal one by one, at a typing pace, and times each
 * key's echo; optionally while a second terminal streams output.
 */
export async function measureLatency(slots: BenchSlots, params: SpikeParams, c: LatencyCase): Promise<LatencyResult> {
  const typing = await TerminalSession.open(slots.primary, params, { kind: "cat" });
  let load: TerminalSession | null = null;
  try {
    if (c.loadKBps > 0) {
      load = await TerminalSession.open(slots.secondary, params, { kind: "gen", gen: "color", bytes: 0, rateKBps: c.loadKBps });
      await sleep(loadSettleMs);
    }
    const frames = new FrameSampler();
    const samples: KeySample[] = [];
    for (let i = 0; i < c.samples; i++) {
      if (i > 0 && i % lineLength === 0) {
        typing.term.input("\r");
        await sleep(newlineSettleMs);
      }
      const sample = await typeKey(typing, letters[i % letters.length]);
      if (sample) {
        samples.push(sample);
      }
      await sleep(minGapMs + Math.random() * (maxGapMs - minGapMs));
    }
    return {
      name: c.name,
      loadKBps: c.loadKBps,
      samples: c.samples,
      timeouts: c.samples - samples.length,
      parsedMs: summarize(samples.map((s) => s.parsed)),
      renderedMs: summarize(samples.map((s) => s.rendered)),
      frameMs: summarize(samples.map((s) => s.frame)),
      frames: frames.stop(),
      renderer: typing.rendererInfo(),
    };
  } finally {
    load?.dispose();
    typing.dispose();
  }
}

async function typeKey(session: TerminalSession, key: string): Promise<KeySample | null> {
  const echoed = nextParsed(session, echoTimeoutMs);
  const start = performance.now();
  session.term.input(key);
  const parsed = await echoed;
  if (parsed === null) {
    return null;
  }
  const rendered = await nextRender(session.term, echoTimeoutMs);
  if (rendered === null) {
    return null;
  }
  await nextFrame();
  const frame = performance.now();
  return { parsed: parsed - start, rendered: rendered - start, frame: frame - start };
}

function nextParsed(session: TerminalSession, timeoutMs: number): Promise<number | null> {
  return new Promise((resolve) => {
    const timer = setTimeout(() => {
      off();
      resolve(null);
    }, timeoutMs);
    const off = session.onParsed(() => {
      clearTimeout(timer);
      off();
      resolve(performance.now());
    });
  });
}
