import { useEffect, useRef, useState } from "react";
import type { SpikeParams } from "../params";
import { traceTerminal } from "../terminal/trace";
import { TerminalSession } from "../terminal/session";

interface Props {
  params: SpikeParams;
  visible: boolean;
  /** Receives the trace lines when the spike runs with -trace. */
  onTrace?: (line: string) => void;
}

/** The main terminal: the program the spike was started with, such as claude or pi. */
export function MainTerminal({ params, visible, onTrace }: Props) {
  const host = useRef<HTMLDivElement>(null);
  const session = useRef<TerminalSession | null>(null);
  const [status, setStatus] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    TerminalSession.open(host.current!, params, { kind: "main" }).then(
      (s) => {
        if (cancelled) {
          s.dispose();
          return;
        }
        session.current = s;
        if (onTrace) {
          traceTerminal(s, onTrace);
        }
        s.term.focus();
        s.exited.then((reason) => setStatus(`The program ended: ${reason}`));
      },
      (err) => setStatus(`The terminal did not start: ${err instanceof Error ? err.message : String(err)}`),
    );
    return () => {
      cancelled = true;
      session.current?.dispose();
      session.current = null;
    };
  }, [params, onTrace]);

  useEffect(() => {
    if (visible) {
      session.current?.term.focus();
    }
  }, [visible]);

  return (
    <div className="pane" hidden={!visible}>
      <div className="term-host" ref={host} />
      {status && <div className="pane-status">{status}</div>}
    </div>
  );
}
