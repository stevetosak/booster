import { Terminal, type ITheme } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import { WebglAddon } from "@xterm/addon-webgl";
import { SearchAddon } from "@xterm/addon-search";
import type { Core, Size } from "./core";

const theme: ITheme = {
  background: "#18181b",
  foreground: "#e4e4e7",
  cursor: "#e4e4e7",
  selectionBackground: "#3f3f46",
};

const encoder = new TextEncoder();

export type AttachState =
  | { kind: "attaching" }
  | { kind: "attached"; snapshotBytes: number; restoreMs: number }
  | { kind: "ended" }
  | { kind: "lost"; reason: string };

/**
 * One xterm.js terminal attached to one session of the core. The core sends
 * a "hello" with the snapshot's size, the snapshot, then the live output; the
 * terminal takes the snapshot's size, writes it, and then fits itself to its
 * element, which resizes the session.
 */
export class AttachedTerminal {
  readonly term: Terminal;
  readonly search = new SearchAddon();
  private readonly fit = new FitAddon();
  private readonly observer: ResizeObserver;
  private socket: WebSocket | null = null;
  private restoring = false;
  private disposed = false;

  constructor(
    private readonly host: HTMLElement,
    private readonly core: Core,
    readonly id: string,
    private readonly onState: (s: AttachState) => void,
  ) {
    this.term = new Terminal({
      fontFamily: '"JetBrainsMono Nerd Font Mono", "DejaVu Sans Mono", monospace',
      fontSize: 14,
      scrollback: 10_000,
      theme,
    });
    this.term.loadAddon(this.fit);
    this.term.loadAddon(this.search);
    this.term.open(host);
    try {
      const webgl = new WebglAddon();
      webgl.onContextLoss(() => webgl.dispose());
      this.term.loadAddon(webgl);
    } catch (e) {
      console.warn("WebGL renderer not available, using the DOM renderer", e);
    }
    this.term.onData((d) => this.send(encoder.encode(d)));
    this.term.onBinary((d) => this.send(Uint8Array.from(d, (c) => c.charCodeAt(0))));
    this.term.onResize(({ cols, rows }) => {
      if (!this.restoring) this.sendResize({ cols, rows });
    });
    this.observer = new ResizeObserver(() => requestAnimationFrame(() => this.fitToHost()));
    this.observer.observe(host);
  }

  /** The size the terminal would have in its element. */
  static measure(host: HTMLElement): Size {
    const t = new Terminal({ fontFamily: '"JetBrainsMono Nerd Font Mono", "DejaVu Sans Mono", monospace', fontSize: 14 });
    const fit = new FitAddon();
    t.loadAddon(fit);
    t.open(host);
    const d = fit.proposeDimensions();
    t.dispose();
    return { cols: d?.cols ?? 120, rows: d?.rows ?? 36 };
  }

  attach(): void {
    if (this.disposed) return;
    this.onState({ kind: "attaching" });
    const ws = new WebSocket(this.core.attachURL(this.id));
    ws.binaryType = "arraybuffer";
    this.socket = ws;
    let gotSnapshot = false;
    let ended = false;
    ws.onmessage = (ev) => {
      if (typeof ev.data === "string") {
        const msg = JSON.parse(ev.data);
        if (msg.type === "hello") {
          // A fresh terminal for the snapshot: the old content goes.
          this.restoring = true;
          this.term.reset();
          this.term.clear();
          this.term.resize(msg.size.cols, msg.size.rows);
        } else if (msg.type === "end") {
          ended = true;
          if (msg.reason === "too-slow") {
            this.term.write("\r\n\x1b[33m[booster: fell behind the output, attaching again]\x1b[m\r\n");
            setTimeout(() => this.attach(), 100);
          } else {
            this.onState({ kind: "ended" });
          }
        }
        return;
      }
      const bytes = new Uint8Array(ev.data as ArrayBuffer);
      if (!gotSnapshot) {
        gotSnapshot = true;
        const start = performance.now();
        this.term.write(bytes, () => {
          this.restoring = false;
          this.onState({ kind: "attached", snapshotBytes: bytes.length, restoreMs: Math.round(performance.now() - start) });
          // Fit to the element; when that changes the size, the session follows.
          const before = { cols: this.term.cols, rows: this.term.rows };
          this.fitToHost();
          if (this.term.cols === before.cols && this.term.rows === before.rows) {
            this.sendResize(before);
          }
        });
        return;
      }
      this.term.write(bytes);
    };
    ws.onclose = (ev) => {
      if (this.socket === ws) this.socket = null;
      if (!ended && !this.disposed) this.onState({ kind: "lost", reason: ev.reason || `closed (${ev.code})` });
    };
  }

  focus(): void {
    this.term.focus();
  }

  fitToHost(): void {
    if (this.disposed || this.restoring || this.host.offsetParent === null) return;
    this.fit.fit();
  }

  dispose(): void {
    this.disposed = true;
    this.observer.disconnect();
    this.socket?.close();
    this.term.dispose();
  }

  private send(data: Uint8Array): void {
    if (this.socket?.readyState === WebSocket.OPEN && !this.restoring) this.socket.send(data);
  }

  private sendResize(size: Size): void {
    if (this.socket?.readyState === WebSocket.OPEN) this.socket.send(JSON.stringify({ type: "resize", ...size }));
  }
}
