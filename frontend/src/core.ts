/** The core's HTTP API over loopback TCP, as the window's page reaches it. */

export type Kind = "claude" | "pi" | "shell";

export interface Size {
  cols: number;
  rows: number;
}

export interface SessionInfo {
  id: string;
  kind: Kind;
  dir: string;
  title: string;
  size: Size;
  started: string;
  clients: number;
}

/** Where the core listens, from the page URL that the window opened. */
export interface CoreParams {
  base: string;
  token: string;
  dir: string;
}

export function readParams(): CoreParams {
  const q = new URLSearchParams(location.search);
  const base = q.get("core");
  const token = q.get("token");
  if (!base || !token) {
    throw new Error("the page URL has no core or token parameter; open it from the booster binary");
  }
  return { base, token, dir: q.get("dir") ?? "" };
}

export class Core {
  constructor(readonly params: CoreParams) {}

  list(): Promise<SessionInfo[]> {
    return this.call("GET", "/api/sessions");
  }

  create(kind: Kind, dir: string, size: Size): Promise<SessionInfo> {
    return this.call("POST", "/api/sessions", { kind, dir, size });
  }

  close(id: string): Promise<void> {
    return this.call("DELETE", `/api/sessions/${id}`);
  }

  prompt(id: string, text: string, submit: boolean): Promise<void> {
    return this.call("POST", `/api/sessions/${id}/prompt`, { text, submit });
  }

  /** The attach WebSocket: a page cannot set headers on it, so the token is in the query. */
  attachURL(id: string): string {
    const url = new URL(`/api/sessions/${id}/attach`, this.params.base);
    url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
    url.searchParams.set("token", this.params.token);
    return url.toString();
  }

  private async call<T>(method: string, path: string, body?: unknown): Promise<T> {
    const res = await fetch(new URL(path, this.params.base), {
      method,
      headers: {
        Authorization: `Bearer ${this.params.token}`,
        ...(body === undefined ? {} : { "Content-Type": "application/json" }),
      },
      body: body === undefined ? undefined : JSON.stringify(body),
    });
    if (!res.ok) {
      throw new Error(`${method} ${path}: ${res.status} ${(await res.text()).trim()}`);
    }
    return (await res.json()) as T;
  }
}
