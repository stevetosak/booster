import { createRoot } from "react-dom/client";
import "@xterm/xterm/css/xterm.css";
import { readParams } from "./params";
import { App } from "./ui/App";

// No StrictMode: its second effect run in development would start every
// terminal program twice.
const root = createRoot(document.getElementById("root")!);
try {
  root.render(<App params={readParams()} />);
} catch (err) {
  root.render(<p className="fatal">{err instanceof Error ? err.message : String(err)}</p>);
}
