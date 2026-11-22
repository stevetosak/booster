// compare.mjs compares the core's screen restore (charmbracelet/x/vt) with
// xterm.js. The truth is an @xterm/headless terminal fed a session's recorded
// output, as a window attached from the start would show it. The candidate
// is a fresh @xterm/headless terminal fed the x/vt snapshot of the same
// output. A control restores the truth through xterm.js's own serialize
// addon, to show what the comparison counts when the restore is exact.
//
//   node compare.mjs <id>.raw <snapshot.ansi>
//
// <id>.json (the recorded size and resizes) must sit beside <id>.raw.

import { readFileSync } from "node:fs";
import headless from "@xterm/headless";
import { SerializeAddon } from "@xterm/addon-serialize";

const { Terminal } = headless;
const [rawPath, snapPath] = process.argv.slice(2);
if (!rawPath || !snapPath) {
  console.error("usage: node compare.mjs <id>.raw <snapshot.ansi>");
  process.exit(2);
}
const meta = JSON.parse(readFileSync(rawPath.replace(/\.raw$/, ".json"), "utf8"));
const raw = readFileSync(rawPath);
const snapshot = readFileSync(snapPath);
const finalSize = meta.resizes?.length ? meta.resizes.at(-1).size : meta.size;

function terminal(size) {
  const t = new Terminal({ cols: size.cols, rows: size.rows, scrollback: 10000, allowProposedApi: true });
  t.title = "";
  t.onTitleChange((s) => (t.title = s));
  return t;
}
const write = (t, data) => new Promise((resolve) => t.write(data, resolve));

const truth = terminal(meta.size);
let at = 0;
for (const r of meta.resizes ?? []) {
  await write(truth, raw.subarray(at, r.offset));
  truth.resize(r.size.cols, r.size.rows);
  at = r.offset;
}
await write(truth, raw.subarray(at));

const candidate = terminal(finalSize);
await write(candidate, snapshot);

const serializer = new SerializeAddon();
truth.loadAddon(serializer);
const control = terminal(finalSize);
await write(control, serializer.serialize({ scrollback: 10000 }));

/** Every line of the active buffer, scrollback first, with its cells. */
function lines(t) {
  const b = t.buffer.active;
  const out = [];
  for (let y = 0; y < b.length; y++) out.push(b.getLine(y));
  return out;
}

/** A cell's text and style; an empty cell and a plain space are the same on screen. */
function cellKey(c) {
  return [
    c.getChars() || " ", c.getWidth(),
    c.getFgColorMode(), c.getFgColor(), c.getBgColorMode(), c.getBgColor(),
    c.isBold(), c.isItalic(), c.isDim(), c.isUnderline(), c.isInverse(), c.isStrikethrough(), c.isInvisible(),
  ].join("|");
}

const MODES = ["applicationCursorKeysMode", "applicationKeypadMode", "bracketedPasteMode", "insertMode",
  "mouseTrackingMode", "originMode", "reverseWraparoundMode", "sendFocusMode", "wraparoundMode"];

/** Compares b with a, aligned at the bottom of the buffer (the screen). */
function compare(a, b) {
  const la = lines(a), lb = lines(b);
  const rows = a.rows;
  const report = {
    buffer: [a.buffer.active.type, b.buffer.active.type],
    lines: [la.length, lb.length],
    screenTextDiffs: 0, scrollbackTextDiffs: 0, styledCellDiffs: 0,
    softWraps: [la.filter((l) => l.isWrapped).length, lb.filter((l) => l.isWrapped).length],
    cursor: [[a.buffer.active.cursorX, a.buffer.active.cursorY], [b.buffer.active.cursorX, b.buffer.active.cursorY]],
    modeDiffs: MODES.filter((m) => a.modes[m] !== b.modes[m]).map((m) => `${m}: ${a.modes[m]} → ${b.modes[m]}`),
    title: a.title === b.title ? "same" : `${JSON.stringify(a.title)} → ${JSON.stringify(b.title)}`,
    examples: [],
    styleExamples: [],
  };
  const n = Math.min(la.length, lb.length);
  for (let i = 1; i <= n; i++) {
    const A = la[la.length - i], B = lb[lb.length - i];
    const onScreen = i <= rows;
    const ta = A.translateToString(true).trimEnd(), tb = B.translateToString(true).trimEnd();
    if (ta !== tb) {
      onScreen ? report.screenTextDiffs++ : report.scrollbackTextDiffs++;
      if (report.examples.length < 8) report.examples.push({ fromBottom: i, onScreen, truth: ta, restored: tb });
      continue;
    }
    for (let x = 0; x < Math.min(A.length, B.length); x++) {
      const ka = cellKey(A.getCell(x)), kb = cellKey(B.getCell(x));
      if (ka === kb) continue;
      report.styledCellDiffs++;
      if (report.styleExamples.length < 6) report.styleExamples.push({ fromBottom: i, x, truth: ka, restored: kb });
    }
  }
  return report;
}

console.log(JSON.stringify({
  recording: rawPath,
  bytes: raw.length,
  snapshotBytes: snapshot.length,
  size: meta.size,
  resizes: meta.resizes?.length ?? 0,
  restore: compare(truth, candidate),
  control: compare(truth, control),
}, null, 2));
