import { Terminal, type ITheme } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import { WebglAddon } from "@xterm/addon-webgl";
import { endpointURL, type RendererKind, type SpikeParams } from "../params";

/** What runs in a terminal. The page names a kind; the Go side maps it to a program. */
export type TerminalSpec =
  | { kind: "main" }
  | { kind: "cat" }
  | { kind: "gen"; gen: "plain" | "color"; bytes: number; rateKBps: number };

export interface RendererInfo {
  requested: RendererKind;
  used: RendererKind;
  /** Why WebGL was not used, when it was asked for. */
  error?: string;
  /** The WebGL context was lost and the terminal fell back to the DOM renderer. */
  contextLost: boolean;
}

const theme: ITheme = {
  background: "#18181b",
  foreground: "#e4e4e7",
  cursor: "#e4e4e7",
  selectionBackground: "#3f3f46",
};

const encoder = new TextEncoder();

function specQuery(spec: TerminalSpec): Record<string, string | number> {
  switch (spec.kind) {
    case "gen":
      return { kind: "gen", gen: spec.gen, bytes: spec.bytes, rate: spec.rateKBps };
    default:
      return { kind: spec.kind };
  }
}

/**
 * One xterm.js terminal on one program in a PTY on the Go side. Output
 * arrives as binary WebSocket messages; input leaves as binary messages, and
 * a resize as one JSON text message.
 */
export class TerminalSession {
  readonly term: Terminal;
  readonly exited: Promise<string>;
  private readonly fit = new FitAddon();
  private readonly resizeObserver: ResizeObserver;
  private readonly parsedListeners = new Set<() => void>();
  private readonly receivedListeners = new Set<(bytes: number) => void>();
  private drainWaiters: (() => void)[] = [];
  private renderer: RendererInfo;
  private socket: WebSocket | null = null;
  private received = 0;
  private pendingWrites = 0;
  private markExited: (reason: string) => void = () => {};

  private constructor(private readonly host: HTMLElement, requested: RendererKind) {
    this.term = new Terminal({
      fontFamily: '"JetBrainsMono Nerd Font Mono", "DejaVu Sans Mono", monospace',
      fontSize: 14,
      scrollback: 10_000,
      theme,
    });
    this.term.loadAddon(this.fit);
    this.term.open(host);
    this.renderer = this.loadRenderer(requested);
    this.fitToHost();

    this.exited = new Promise((resolve) => (this.markExited = resolve));
    this.resizeObserver = new ResizeObserver(() => requestAnimationFrame(() => this.fitToHost()));
    this.resizeObserver.observe(host);
    this.term.onData((data) => this.send(encoder.encode(data)));
    this.term.onBinary((data) => this.send(Uint8Array.from(data, (c) => c.charCodeAt(0))));
    this.term.onResize(({ cols, rows }) => this.sendText(JSON.stringify({ type: "resize", cols, rows })));
  }

  /** Opens a terminal in host and starts its program; resolves when the program runs. */
  static async open(host: HTMLElement, params: SpikeParams, spec: TerminalSpec): Promise<TerminalSession> {
    const session = new TerminalSession(host, params.renderer);
    const { cols, rows } = session.term;
    try {
      await session.connect(endpointURL(params, "/pty", { ...specQuery(spec), cols, rows }));
    } catch (err) {
      session.dispose();
      throw err;
    }
    return session;
  }

  rendererInfo(): RendererInfo {
    return { ...this.renderer };
  }

  bytesReceived(): number {
    return this.received;
  }

  /** Calls listener each time a chunk of output arrives, before it is parsed. */
  onReceived(listener: (bytes: number) => void): () => void {
    this.receivedListeners.add(listener);
    return () => this.receivedListeners.delete(listener);
  }

  /** Calls listener each time a chunk of output is parsed into the buffer. */
  onParsed(listener: () => void): () => void {
    this.parsedListeners.add(listener);
    return () => this.parsedListeners.delete(listener);
  }

  /** Resolves when every chunk received so far is parsed. */
  drained(): Promise<void> {
    if (this.pendingWrites === 0) {
      return Promise.resolve();
    }
    return new Promise((resolve) => this.drainWaiters.push(resolve));
  }

  dispose(): void {
    this.resizeObserver.disconnect();
    this.socket?.close();
    this.term.dispose();
  }

  private connect(url: string): Promise<void> {
    return new Promise((resolve, reject) => {
      const socket = new WebSocket(url);
      socket.binaryType = "arraybuffer";
      socket.onopen = () => {
        this.socket = socket;
        resolve();
      };
      socket.onerror = () => reject(new Error(`cannot open the terminal connection ${new URL(url).pathname}`));
      socket.onmessage = (ev) => this.write(new Uint8Array(ev.data as ArrayBuffer));
      socket.onclose = (ev) => this.markExited(ev.reason || `closed (${ev.code})`);
    });
  }

  private write(data: Uint8Array): void {
    this.received += data.byteLength;
    for (const listener of this.receivedListeners) {
      listener(data.byteLength);
    }
    this.pendingWrites++;
    this.term.write(data, () => {
      this.pendingWrites--;
      for (const listener of this.parsedListeners) {
        listener();
      }
      if (this.pendingWrites === 0) {
        for (const resolve of this.drainWaiters.splice(0)) {
          resolve();
        }
      }
    });
  }

  private send(data: Uint8Array): void {
    if (this.socket?.readyState === WebSocket.OPEN) {
      this.socket.send(data);
    }
  }

  private sendText(text: string): void {
    if (this.socket?.readyState === WebSocket.OPEN) {
      this.socket.send(text);
    }
  }

  private fitToHost(): void {
    // A hidden host has no size; fitting it would shrink the terminal to nothing.
    if (this.host.clientWidth > 0 && this.host.clientHeight > 0) {
      this.fit.fit();
    }
  }

  private loadRenderer(requested: RendererKind): RendererInfo {
    const info: RendererInfo = { requested, used: "dom", contextLost: false };
    if (requested === "dom") {
      return info;
    }
    try {
      const webgl = new WebglAddon();
      webgl.onContextLoss(() => {
        this.renderer = { ...this.renderer, used: "dom", contextLost: true };
        webgl.dispose();
      });
      this.term.loadAddon(webgl);
      return { ...info, used: "webgl" };
    } catch (err) {
      return { ...info, error: err instanceof Error ? err.message : String(err) };
    }
  }
}
