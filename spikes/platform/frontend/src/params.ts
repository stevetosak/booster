// The Go side opens the window with the facts the page needs in its URL query.

export type RendererKind = "webgl" | "dom";

export interface SpikeParams {
  /** Base URL of the loopback server, for example ws://127.0.0.1:41234. */
  wsBase: string;
  /** Random token that every connection to the server must carry. */
  token: string;
  renderer: RendererKind;
  label: string;
  /** Run the benchmarks at once, save the report and let the app quit. */
  autobench: boolean;
}

export function readParams(search: string = window.location.search): SpikeParams {
  const q = new URLSearchParams(search);
  const wsBase = q.get("ws");
  const token = q.get("token");
  if (!wsBase || !token) {
    throw new Error("the page URL has no ws or token parameter; start the page from the spike binary");
  }
  return {
    wsBase,
    token,
    renderer: q.get("renderer") === "dom" ? "dom" : "webgl",
    label: q.get("label") ?? "",
    autobench: q.get("autobench") === "1",
  };
}

/** The URL of one server endpoint, with the token and the given query. */
export function endpointURL(params: SpikeParams, path: string, query: Record<string, string | number> = {}): string {
  const url = new URL(path, params.wsBase);
  url.searchParams.set("token", params.token);
  for (const [k, v] of Object.entries(query)) {
    url.searchParams.set(k, String(v));
  }
  return url.toString();
}
