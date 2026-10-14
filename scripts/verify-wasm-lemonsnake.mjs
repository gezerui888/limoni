// Smoke-tests Lemon Snake's WebAssembly build by running it, as
// verify-wasm-lemondrop.mjs does for Lemon Drop.
//
// It boots the module under Node with a stub xterm.js bridge and a stub Web
// Audio context, types a name, starts a run, turns, pauses with Esc and
// goes on, and quits with Q from the pause. Each step asserts on what
// reached the screen: the name dialog, the title with the leaderboard, the
// board in truecolor half blocks, the pause, and that Esc did not end the
// program but Q did.
//
//   node scripts/verify-wasm-lemonsnake.mjs "$(go env GOROOT)" path/to/lemonsnake.wasm

import { readFileSync } from "node:fs";
import path from "node:path";

const goroot = process.argv[2];
const wasmPath = process.argv[3];

globalThis.crypto ??= (await import("node:crypto")).webcrypto;
await import(path.join(goroot, "lib/wasm/wasm_exec.js"));

let captured = "";
globalThis.__limoni_output = (s) => {
  captured += s;
};

// Just enough of an AudioContext to count what the game does with it.
const audio = { buffers: 0, started: 0 };
const node = () => ({ connect: () => {}, gain: {}, pan: {} });
globalThis.__limoni_audio = {
  destination: {},
  createBuffer: () => {
    audio.buffers++;
    return { copyToChannel: () => {} };
  },
  createBufferSource: () => ({ ...node(), start: () => audio.started++ }),
  createGain: node,
  createStereoPanner: node,
};

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const go = new globalThis.Go();
const { instance } = await WebAssembly.instantiate(readFileSync(wasmPath), go.importObject);
let exited = false;
const onExit = () => {
  exited = true;
  for (const timer of go._scheduledTimeouts.values()) clearTimeout(timer);
};
go.run(instance).then(onExit, onExit);

const since = (mark) => captured.slice(mark);
const send = (s) => globalThis.__limoni_input?.(s);

await sleep(600);
globalThis.__limoni_resize?.(100, 30);
await sleep(900); // the name dialog opens
const asked = captured;

let mark = captured.length;
send("Tester\r"); // the name, then on to the title (Node has no localStorage)
await sleep(900);
const title = since(mark);

mark = captured.length;
send("\r"); // Enter plays
await sleep(500);
const run = since(mark);

mark = captured.length;
for (const key of ["\x1b[A", "\x1b[D", "\x1b[B", " "]) {
  send(key);
  await sleep(150);
}
await sleep(300);
const played = since(mark);

mark = captured.length;
send("\x1b"); // Esc, alone, as xterm.js sends it: a pause, not the end
await sleep(900);
const paused = since(mark);
const stillRunning = !exited;
send("\x1b"); // and again: the run goes on
await sleep(300);

send("p"); // P pauses as well, and Q quits from the pause
await sleep(200);
send("q");
await sleep(800);

const checks = [
  ["alternate screen (?1049h)", asked.includes("\x1b[?1049h")],
  ["truecolor SGR (38;2 / 48;2)", /\x1b\[[0-9;]*?[34]8;2;/.test(asked)],
  ["half blocks (▀)", asked.includes("▀")],
  ["the name dialog first (YOUR NAME), and the panel (LEMON SNAKE)", ["YOUR NAME", "LEMON SNAKE"].every((s) => asked.includes(s))],
  ["the title dialog with the name (playing as, Tester, TOP TEN)", ["playing as", "Tester", "TOP TEN"].every((s) => title.includes(s))],
  ["a run after Enter redrew the board", run.length > 1000],
  ["the snake moved under the keys", played.length > 500],
  ["Esc pauses, and does not end the game (PAUSED)", paused.includes("PAUSED") && stillRunning],
  ["clips loaded into Web Audio", audio.buffers > 0],
  ["sounds played (start, turns)", audio.started >= 3],
  ["Q in the pause ended the program", exited],
];

console.log(`captured ${captured.length} bytes from the engine\n`);
let missing = 0;
for (const [label, ok] of checks) {
  if (!ok) missing++;
  console.log(`  ${ok ? "OK     " : "MISSING"}  ${label}`);
}
console.log(`\n  Web Audio: ${audio.buffers} clips, ${audio.started} played`);
process.exit(missing === 0 ? 0 : 1);
