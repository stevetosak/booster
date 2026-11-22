import type { ControlClient, MemoryReport } from "../net/control";
import type { SpikeParams } from "../params";
import { TerminalSession } from "../terminal/session";
import { probeWebGL, type WebGLProbe } from "./gpu";
import { measureLatency, type BenchSlots, type LatencyCase, type LatencyResult } from "./latency";
import { measureThroughput, type ThroughputCase, type ThroughputResult } from "./throughput";
import { FrameSampler, sleep, type FrameStats } from "./timing";

export interface SuitePlan {
  throughput: ThroughputCase[];
  latency: LatencyCase[];
  /** Output rate of the stream during the memory reading under load. */
  memoryLoadKBps: number;
}

const MiB = 1024 * 1024;

export const defaultPlan: SuitePlan = {
  throughput: [
    { name: "plain text, 32 MiB", gen: "plain", bytes: 32 * MiB },
    { name: "colour and styles, 32 MiB", gen: "color", bytes: 32 * MiB },
  ],
  latency: [
    { name: "idle", samples: 150, loadKBps: 0 },
    { name: "beside an agent stream (256 KB/s)", samples: 120, loadKBps: 256 },
    { name: "beside a build log (2 MB/s)", samples: 120, loadKBps: 2048 },
  ],
  memoryLoadKBps: 2048,
};

/** The page's half of a run report; the Go side adds the environment and the build. */
export interface PageReport {
  label: string;
  userAgent: string;
  devicePixelRatio: number;
  window: { width: number; height: number };
  webgl: WebGLProbe;
  idleFrames: FrameStats;
  memory: { idle?: MemoryReport; underLoad?: MemoryReport };
  throughput: ThroughputResult[];
  latency: LatencyResult[];
  errors: string[];
}

const idleSampleMs = 2000;
const memorySettleMs = 2000;

/**
 * Runs every benchmark of the plan in order. A failed benchmark is recorded
 * in errors and the suite goes on, so one fault does not hide the others.
 */
export async function runSuite(
  slots: BenchSlots,
  params: SpikeParams,
  control: ControlClient,
  plan: SuitePlan,
  onProgress: (step: string) => void,
): Promise<PageReport> {
  const report: PageReport = {
    label: params.label,
    userAgent: navigator.userAgent,
    devicePixelRatio: window.devicePixelRatio,
    window: { width: window.innerWidth, height: window.innerHeight },
    webgl: probeWebGL(),
    idleFrames: { intervalsMs: null, longFrames: 0 },
    memory: {},
    throughput: [],
    latency: [],
    errors: [],
  };
  const step = async (name: string, run: () => Promise<void>) => {
    onProgress(name);
    try {
      await run();
    } catch (err) {
      report.errors.push(`${name}: ${err instanceof Error ? err.message : String(err)}`);
    }
  };

  await step("idle frames", async () => {
    const sampler = new FrameSampler();
    await sleep(idleSampleMs);
    report.idleFrames = sampler.stop();
  });
  await step("memory, idle", async () => {
    report.memory.idle = await control.memory();
  });
  for (const c of plan.throughput) {
    await step(`throughput: ${c.name}`, async () => {
      report.throughput.push(await measureThroughput(slots.primary, params, c));
    });
  }
  for (const c of plan.latency) {
    await step(`typing latency: ${c.name}`, async () => {
      report.latency.push(await measureLatency(slots, params, c));
    });
  }
  await step("memory, under load", async () => {
    report.memory.underLoad = await memoryUnderLoad(slots, params, control, plan.memoryLoadKBps);
  });
  return report;
}

/** Reads the memory while one terminal streams beside an idle one. */
async function memoryUnderLoad(slots: BenchSlots, params: SpikeParams, control: ControlClient, rateKBps: number): Promise<MemoryReport> {
  const idle = await TerminalSession.open(slots.primary, params, { kind: "cat" });
  try {
    const load = await TerminalSession.open(slots.secondary, params, { kind: "gen", gen: "color", bytes: 0, rateKBps });
    try {
      await sleep(memorySettleMs);
      return await control.memory();
    } finally {
      load.dispose();
    }
  } finally {
    idle.dispose();
  }
}
