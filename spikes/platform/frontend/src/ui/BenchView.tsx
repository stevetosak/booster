import { useCallback, useEffect, useRef, useState } from "react";
import type { ControlClient } from "../net/control";
import type { SpikeParams } from "../params";
import { defaultPlan, runSuite, type PageReport } from "../bench/suite";
import { ResultsTable } from "./ResultsTable";

interface Props {
  params: SpikeParams;
  control: ControlClient | null;
  visible: boolean;
  /** Start one run as soon as the control channel is open. */
  autostart: boolean;
  onRunningChange: (running: boolean) => void;
}

/**
 * Two terminal slots side by side: the benchmarks open their terminals in
 * them, so every run measures terminals of the same size.
 */
export function BenchView({ params, control, visible, autostart, onRunningChange }: Props) {
  const primary = useRef<HTMLDivElement>(null);
  const secondary = useRef<HTMLDivElement>(null);
  const started = useRef(false);
  const [step, setStep] = useState<string | null>(null);
  const [report, setReport] = useState<PageReport | null>(null);
  const [savedPath, setSavedPath] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const run = useCallback(async () => {
    if (!control || !primary.current || !secondary.current) {
      return;
    }
    const slots = { primary: primary.current, secondary: secondary.current };
    setReport(null);
    setSavedPath(null);
    setError(null);
    onRunningChange(true);
    try {
      const onProgress = (step: string) => {
        setStep(step);
        void control.progress(step).catch(() => {}); // the log is a help, not a condition
      };
      const result = await runSuite(slots, params, control, defaultPlan, onProgress);
      void control.progress(`done, ${result.errors.length} errors`).catch(() => {});
      setReport(result);
      setSavedPath(await control.saveReport(result));
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setStep(null);
      onRunningChange(false);
    }
  }, [control, params, onRunningChange]);

  useEffect(() => {
    if (autostart && control && !started.current) {
      started.current = true;
      void run();
    }
  }, [autostart, control, run]);

  const running = step !== null;
  return (
    <div className="pane bench" hidden={!visible}>
      <div className="bench-bar">
        <button onClick={() => void run()} disabled={running || !control}>
          Run benchmarks
        </button>
        <span className="bench-status">
          {running ? `Running: ${step}` : error ? `Failed: ${error}` : control ? "" : "Connecting to the control channel"}
        </span>
      </div>
      <div className="bench-slots">
        <div className="term-slot"><div className="term-host" ref={primary} /></div>
        <div className="term-slot"><div className="term-host" ref={secondary} /></div>
        {report && !running && <ResultsTable report={report} savedPath={savedPath} />}
      </div>
    </div>
  );
}
