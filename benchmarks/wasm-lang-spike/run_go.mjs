// Runs the Go/WASM build under Node via the standard wasm_exec.js glue --
// the same mechanism this org's browser apps use to load character/stage/
// sff/engine WASM builds.
import fs from "node:fs";
import "./go/wasm_exec.js";

const ticks = process.argv[2] || "2000000";
const wasmBuf = fs.readFileSync(new URL("./go/spike_go.wasm", import.meta.url));

const go = new Go();
go.argv = ["spike", ticks];

const wallStart = performance.now();
const { instance } = await WebAssembly.instantiate(wasmBuf, go.importObject);
const instantiateEnd = performance.now();
await go.run(instance);
const wallEnd = performance.now();

console.error(
  `[node harness] instantiate_ms=${(instantiateEnd - wallStart).toFixed(3)} total_wall_ms=${(wallEnd - wallStart).toFixed(3)} wasm_bytes=${wasmBuf.length}`,
);
