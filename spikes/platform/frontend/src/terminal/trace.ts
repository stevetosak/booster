import type { TerminalSession } from "./session";

const inputEvents = [
  "keydown",
  "keypress",
  "keyup",
  "beforeinput",
  "input",
  "compositionstart",
  "compositionupdate",
  "compositionend",
] as const;

const quote = (s: string) => JSON.stringify(s);

function describe(ev: Event): string {
  if (ev instanceof KeyboardEvent) {
    return `key=${quote(ev.key)} code=${ev.code} keyCode=${ev.keyCode} composing=${ev.isComposing} repeat=${ev.repeat}`;
  }
  if (ev instanceof InputEvent) {
    return `inputType=${ev.inputType} data=${quote(ev.data ?? "")} composing=${ev.isComposing}`;
  }
  if (ev instanceof CompositionEvent) {
    return `data=${quote(ev.data)}`;
  }
  return "";
}

/**
 * Writes the terminal's input and output events to log, with times.
 *
 * Input: every keyboard, input and composition event of the hidden textarea,
 * with its value, and every piece of data sent to the program.
 *
 * Output: each chunk received and parsed, and after each parse whether a
 * timer and an animation frame run, and when xterm.js renders. The order
 * shows where an update waits on its way to the screen.
 */
export function traceTerminal(session: TerminalSession, log: (line: string) => void): () => void {
  const { term } = session;
  const start = performance.now();
  const stamp = () => `+${(performance.now() - start).toFixed(1)}ms`;
  const cleanups: (() => void)[] = [];

  const textarea = term.textarea;
  if (textarea) {
    const onEvent = (ev: Event) => log(`${stamp()} ${ev.type} ${describe(ev)} value=${quote(textarea.value)}`);
    for (const type of inputEvents) {
      textarea.addEventListener(type, onEvent, true);
      cleanups.push(() => textarea.removeEventListener(type, onEvent, true));
    }
  }
  const sent = term.onData((d) => log(`${stamp()} SENT ${quote(d)}`));
  cleanups.push(() => sent.dispose());

  let chunk = 0;
  cleanups.push(session.onReceived((bytes) => log(`${stamp()} RECEIVED #${++chunk} ${bytes} bytes`)));
  cleanups.push(
    session.onParsed(() => {
      const n = chunk;
      log(`${stamp()} PARSED #${n} cursor=${term.buffer.active.cursorX},${term.buffer.active.cursorY}`);
      setTimeout(() => log(`${stamp()} TIMER after #${n}`), 0);
      requestAnimationFrame(() => log(`${stamp()} FRAME after #${n}`));
    }),
  );
  const rendered = term.onRender(({ start: from, end }) => log(`${stamp()} RENDERED rows ${from}-${end}`));
  cleanups.push(() => rendered.dispose());

  return () => cleanups.forEach((cleanup) => cleanup());
}
