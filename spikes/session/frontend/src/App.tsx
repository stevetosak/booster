import { useCallback, useEffect, useRef, useState } from "react";
import { Core, readParams, type Kind, type SessionInfo } from "./core";
import { AttachedTerminal, type AttachState } from "./terminal";

const core = new Core(readParams());

/**
 * The spike window: the core's sessions on the left, the selected session's
 * terminal on the right, and below it the search and the Action prompt.
 * Every session keeps its own terminal while the window is open; closing the
 * window leaves the sessions running in the core.
 */
export function App() {
  const [sessions, setSessions] = useState<SessionInfo[]>([]);
  const [active, setActive] = useState<string | null>(null);
  const [states, setStates] = useState<Record<string, AttachState>>({});
  const [error, setError] = useState<string>("");
  const [dir, setDir] = useState(core.params.dir);
  const terminals = useRef(new Map<string, AttachedTerminal>());
  const hosts = useRef(new Map<string, HTMLDivElement>());
  const stage = useRef<HTMLDivElement>(null);

  const refresh = useCallback(async () => {
    try {
      const list = await core.list();
      setSessions(list);
      setActive((a) => (a && list.some((s) => s.id === a) ? a : (list[0]?.id ?? null)));
    } catch (e) {
      setError(String(e));
    }
  }, []);

  useEffect(() => {
    void refresh();
    const t = setInterval(refresh, 2000);
    return () => clearInterval(t);
  }, [refresh]);

  // Attach a terminal to every listed session once its element exists.
  useEffect(() => {
    for (const s of sessions) {
      const host = hosts.current.get(s.id);
      if (!host || terminals.current.has(s.id)) continue;
      const t = new AttachedTerminal(host, core, s.id, (st) => setStates((m) => ({ ...m, [s.id]: st })));
      terminals.current.set(s.id, t);
      t.attach();
    }
    for (const [id, t] of terminals.current) {
      if (!sessions.some((s) => s.id === id)) {
        t.dispose();
        terminals.current.delete(id);
      }
    }
  }, [sessions]);

  useEffect(() => {
    if (!active) return;
    const t = terminals.current.get(active);
    requestAnimationFrame(() => {
      t?.fitToHost();
      t?.focus();
    });
  }, [active]);

  const create = async (kind: Kind) => {
    setError("");
    try {
      const size = stage.current ? AttachedTerminal.measure(stage.current) : { cols: 120, rows: 36 };
      const info = await core.create(kind, dir, size);
      await refresh();
      setActive(info.id);
    } catch (e) {
      setError(String(e));
    }
  };

  const current = sessions.find((s) => s.id === active) ?? null;
  const currentTerminal = active ? terminals.current.get(active) : undefined;

  return (
    <div className="app">
      <aside className="sidebar">
        <h1>Sessions</h1>
        <ul>
          {sessions.map((s) => (
            <li key={s.id} className={s.id === active ? "active" : ""} onClick={() => setActive(s.id)}>
              <span className="kind">{s.kind}</span>
              <span className="title">{s.title || s.dir}</span>
              <span className="meta">
                {s.id} · {stateLabel(states[s.id])}
              </span>
            </li>
          ))}
        </ul>
        <label className="dir">
          Folder
          <input value={dir} onChange={(e) => setDir(e.target.value)} spellCheck={false} />
        </label>
        <div className="new">
          <button onClick={() => create("claude")}>+ Claude Code</button>
          <button onClick={() => create("pi")}>+ pi</button>
          <button onClick={() => create("shell")}>+ shell</button>
        </div>
        {current && (
          <button className="danger" onClick={() => core.close(current.id).then(refresh, (e) => setError(String(e)))}>
            End {current.kind} {current.id}
          </button>
        )}
        {error && <p className="error">{error}</p>}
      </aside>
      <main>
        <div className="stage" ref={stage}>
          {sessions.map((s) => (
            <div
              key={s.id}
              className="term"
              style={{ display: s.id === active ? "block" : "none" }}
              ref={(el) => {
                if (el) hosts.current.set(s.id, el);
                else hosts.current.delete(s.id);
              }}
            />
          ))}
          {sessions.length === 0 && <p className="empty">No sessions. Start Claude Code, pi or a shell.</p>}
        </div>
        {current && currentTerminal && <SearchBar terminal={currentTerminal} key={`s-${current.id}`} />}
        {current && <ActionBar session={current} onError={setError} key={`a-${current.id}`} />}
      </main>
    </div>
  );
}

function stateLabel(s: AttachState | undefined): string {
  switch (s?.kind) {
    case undefined:
    case "attaching":
      return "attaching";
    case "attached":
      return `restored ${(s.snapshotBytes / 1024).toFixed(0)} KB in ${s.restoreMs} ms`;
    case "ended":
      return "ended";
    case "lost":
      return `lost: ${s.reason}`;
  }
}

function SearchBar({ terminal }: { terminal: AttachedTerminal }) {
  const [query, setQuery] = useState("");
  const [found, setFound] = useState<boolean | null>(null);
  const find = (back: boolean) => {
    if (!query) return;
    setFound(back ? terminal.search.findPrevious(query) : terminal.search.findNext(query));
  };
  return (
    <div className="bar">
      <input
        placeholder="Search the scrollback"
        value={query}
        onChange={(e) => {
          setQuery(e.target.value);
          setFound(null);
        }}
        onKeyDown={(e) => {
          if (e.key === "Enter") find(e.shiftKey);
        }}
      />
      <button onClick={() => find(true)}>↑</button>
      <button onClick={() => find(false)}>↓</button>
      <span className="hint">{found === null ? "" : found ? "found" : "not found"}</span>
    </div>
  );
}

function ActionBar({ session, onError }: { session: SessionInfo; onError: (e: string) => void }) {
  const [text, setText] = useState("");
  const [submit, setSubmit] = useState(true);
  const [sent, setSent] = useState("");
  const send = async () => {
    if (!text.trim()) return;
    try {
      await core.prompt(session.id, text, submit);
      setSent(`sent ${text.length} characters at ${new Date().toLocaleTimeString()}`);
      setText("");
    } catch (e) {
      onError(String(e));
    }
  };
  return (
    <div className="bar action">
      <textarea
        placeholder={`Action prompt for ${session.kind} ${session.id} (Ctrl+Enter sends)`}
        value={text}
        rows={2}
        onChange={(e) => setText(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter" && e.ctrlKey) void send();
        }}
      />
      <label>
        <input type="checkbox" checked={submit} onChange={(e) => setSubmit(e.target.checked)} /> submit
      </label>
      <button onClick={send}>Send Action</button>
      <span className="hint">{sent}</span>
    </div>
  );
}
