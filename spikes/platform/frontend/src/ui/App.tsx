import { useCallback, useEffect, useRef, useState } from "react";
import { ControlClient } from "../net/control";
import type { SpikeParams } from "../params";
import { BenchView } from "./BenchView";
import { MainTerminal } from "./MainTerminal";

type View = "terminal" | "bench";

export function App({ params }: { params: SpikeParams }) {
  const [view, setView] = useState<View>(params.autobench ? "bench" : "terminal");
  const [control, setControl] = useState<ControlClient | null>(null);
  const [controlError, setControlError] = useState<string | null>(null);
  const [benchRunning, setBenchRunning] = useState(false);
  const controlRef = useRef<ControlClient | null>(null);

  useEffect(() => {
    ControlClient.connect(params).then(
      (c) => {
        controlRef.current = c;
        setControl(c);
      },
      (err) => setControlError(String(err)),
    );
  }, [params]);

  // Stable, so the main terminal is not restarted when the control channel opens.
  const trace = useCallback((line: string) => void controlRef.current?.trace(line).catch(() => {}), []);

  return (
    <div className="app">
      <header className="topbar">
        <span className="title">booster platform spike</span>
        <span className="meta">
          {params.renderer} renderer{params.label ? ` · ${params.label}` : ""}
          {controlError ? ` · ${controlError}` : ""}
        </span>
        <nav className="tabs">
          <button aria-pressed={view === "terminal"} disabled={params.autobench || benchRunning} onClick={() => setView("terminal")}>
            Terminal
          </button>
          <button aria-pressed={view === "bench"} onClick={() => setView("bench")}>
            Benchmarks
          </button>
        </nav>
      </header>
      <main className="content">
        {!params.autobench && (
          <MainTerminal params={params} visible={view === "terminal"} onTrace={params.trace ? trace : undefined} />
        )}
        <BenchView
          params={params}
          control={control}
          visible={view === "bench"}
          autostart={params.autobench}
          onRunningChange={setBenchRunning}
        />
      </main>
    </div>
  );
}
