import type { MemoryReport } from "../net/control";
import type { Summary } from "../bench/stats";
import type { PageReport } from "../bench/suite";

const ms = (s: Summary | null, key: "p50" | "p95" | "p99") => (s ? s[key].toFixed(1) : "–");
const mb = (kb: number | undefined) => (kb === undefined ? "–" : (kb / 1024).toFixed(0));

function memoryText(m: MemoryReport | undefined): string {
  return m ? `shell ${mb(m.shellKB)} MB, programs ${mb(m.programsKB)} MB` : "–";
}

/** The results of one run, in the units the plan's comparison table uses. */
export function ResultsTable({ report, savedPath }: { report: PageReport; savedPath: string | null }) {
  const renderers = new Set([...report.throughput, ...report.latency].map((r) => r.renderer.used));
  const lost = [...report.throughput, ...report.latency].some((r) => r.renderer.contextLost);
  return (
    <div className="results">
      <h2>Environment</h2>
      <table>
        <tbody>
          <tr><th>Renderer used</th><td>{[...renderers].join(", ") || "–"}{lost ? " (WebGL context lost)" : ""}</td></tr>
          <tr><th>WebGL</th><td>{report.webgl.webgl2 ? `${report.webgl.vendor} / ${report.webgl.renderer}${report.webgl.software ? " (software)" : ""}` : "no WebGL2"}</td></tr>
          <tr><th>Frame interval, idle</th><td>p50 {ms(report.idleFrames.intervalsMs, "p50")} ms</td></tr>
          <tr><th>Window</th><td>{report.window.width}×{report.window.height} at {report.devicePixelRatio}× scale</td></tr>
          <tr><th>Memory (PSS), idle</th><td>{memoryText(report.memory.idle)}</td></tr>
          <tr><th>Memory (PSS), under load</th><td>{memoryText(report.memory.underLoad)}</td></tr>
        </tbody>
      </table>

      <h2>Throughput</h2>
      <table>
        <thead>
          <tr><th>Output</th><th>MB/s</th><th>Time (ms)</th><th>Frame p95 (ms)</th><th>Frames over 50 ms</th></tr>
        </thead>
        <tbody>
          {report.throughput.map((r) => (
            <tr key={r.name}>
              <td>{r.name}</td><td>{r.mbPerSec}</td><td>{r.ms}</td>
              <td>{ms(r.frames.intervalsMs, "p95")}</td><td>{r.frames.longFrames}</td>
            </tr>
          ))}
        </tbody>
      </table>

      <h2>Typing latency (ms, p50 / p95)</h2>
      <table>
        <thead>
          <tr><th>Case</th><th>Echo parsed</th><th>Echo drawn</th><th>Next frame</th><th>Timeouts</th><th>Frames over 50 ms</th></tr>
        </thead>
        <tbody>
          {report.latency.map((r) => (
            <tr key={r.name}>
              <td>{r.name}</td>
              <td>{ms(r.parsedMs, "p50")} / {ms(r.parsedMs, "p95")}</td>
              <td>{ms(r.renderedMs, "p50")} / {ms(r.renderedMs, "p95")}</td>
              <td>{ms(r.frameMs, "p50")} / {ms(r.frameMs, "p95")}</td>
              <td>{r.timeouts}</td><td>{r.frames.longFrames}</td>
            </tr>
          ))}
        </tbody>
      </table>

      {report.errors.length > 0 && (
        <>
          <h2>Errors</h2>
          <ul>{report.errors.map((e) => <li key={e}>{e}</li>)}</ul>
        </>
      )}
      {savedPath && <p className="saved">Report saved: {savedPath}</p>}
    </div>
  );
}
