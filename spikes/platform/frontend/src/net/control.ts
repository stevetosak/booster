import { endpointURL, type SpikeParams } from "../params";

/** Proportional set size of one process, as the Go side reads it. */
export interface ProcessMemory {
  pid: number;
  name: string;
  pssKB: number;
}

export interface MemoryReport {
  /** The spike process and WebKit's helper processes. */
  shellKB: number;
  /** The programs that run in the terminals. */
  programsKB: number;
  processes: ProcessMemory[];
}

interface ControlReply {
  op: string;
  memory?: MemoryReport;
  path?: string;
  error?: string;
}

interface Pending {
  resolve: (reply: ControlReply) => void;
  reject: (err: Error) => void;
}

/**
 * The control channel: memory readings and report saving. The server answers
 * the requests in order, so a queue of waiters matches replies to requests.
 */
export class ControlClient {
  private readonly pending: Pending[] = [];

  private constructor(private readonly socket: WebSocket) {
    socket.onmessage = (ev) => this.pending.shift()?.resolve(JSON.parse(String(ev.data)) as ControlReply);
    socket.onclose = () => {
      for (const p of this.pending.splice(0)) {
        p.reject(new Error("the control channel closed"));
      }
    };
  }

  static connect(params: SpikeParams): Promise<ControlClient> {
    return new Promise((resolve, reject) => {
      const socket = new WebSocket(endpointURL(params, "/control"));
      socket.onopen = () => resolve(new ControlClient(socket));
      socket.onerror = () => reject(new Error("cannot open the control channel"));
    });
  }

  async memory(): Promise<MemoryReport> {
    const reply = await this.request({ op: "memory" });
    if (!reply.memory) {
      throw new Error("the memory reply has no memory");
    }
    return reply.memory;
  }

  /** Writes a step of the run to the Go side's log, where a watcher sees it. */
  async progress(step: string): Promise<void> {
    await this.request({ op: "progress", text: step });
  }

  /** Writes one line of the terminal trace to the Go side's log. */
  async trace(line: string): Promise<void> {
    await this.request({ op: "trace", text: line });
  }

  /** Saves the page's report with the Go side's facts; returns the file path. */
  async saveReport(report: unknown): Promise<string> {
    const reply = await this.request({ op: "report", report });
    return reply.path ?? "";
  }

  private async request(body: object): Promise<ControlReply> {
    const reply = await new Promise<ControlReply>((resolve, reject) => {
      this.pending.push({ resolve, reject });
      this.socket.send(JSON.stringify(body));
    });
    if (reply.error) {
      throw new Error(reply.error);
    }
    return reply;
  }
}
