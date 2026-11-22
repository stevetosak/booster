import { useEffect, useState } from "react";
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

  useEffect(() => {
    ControlClient.connect(params).then(setControl, (err) => setControlError(String(err)));
  }, [params]);

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
        {!params.autobench && <MainTerminal params={params} visible={view === "terminal"} />}
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
