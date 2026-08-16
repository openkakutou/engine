// Runs the Rust/wasm32-wasip1 build under Node's built-in WASI implementation.
import fs from "node:fs";
import { WASI } from "node:wasi";

const ticks = process.argv[2] || "2000000";
const wasmBuf = fs.readFileSync(
  new URL("./rust/target/wasm32-wasip1/release/spike.wasm", import.meta.url),
);

const wasi = new WASI({
  version: "preview1",
  args: ["spike", ticks],
  env: {},
  stdout: 1,
  stderr: 2,
});

const wallStart = performance.now();
const { instance } = await WebAssembly.instantiate(wasmBuf, wasi.getImportObject());
const instantiateEnd = performance.now();
wasi.start(instance);
const wallEnd = performance.now();

console.error(
  `[node harness] instantiate_ms=${(instantiateEnd - wallStart).toFixed(3)} total_wall_ms=${(wallEnd - wallStart).toFixed(3)} wasm_bytes=${wasmBuf.length}`,
);
