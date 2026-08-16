# wasm-lang-spike

Synthetic Go/WASM vs. Rust/WASM benchmark, run per roadmap `.vibe/backlog/008` to inform whether `engine` (and possibly `character`/`stage`/`sff`) should move to Rust for performance, before `engine`'s own backlog item `005` (physics/movement) starts. See [`RESULTS.md`](RESULTS.md) for findings.

## What this is

A synthetic per-tick combat-simulation workload (trigger evaluation + state transitions + hit detection + physics for 2 fighters), implemented twice — once in Go (`go/main.go`), once in Rust (`rust/src/main.rs`) — structurally mirrored 1:1 (same constants, same operation order, same allocation shape) so both produce an **identical checksum**, confirming they perform equivalent work. It is a proxy for `engine`'s real per-frame workload, not a copy of it — `engine`'s actual trigger evaluator/state machine/physics code is more complex than this, and this spike doesn't claim otherwise.

## Reproducing

```sh
# Go/WASM
cd go
GOOS=js GOARCH=wasm go build -o spike_go.wasm .
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" .   # path varies by Go version
cd ..
node run_go.mjs 500000        # arg = tick count

# Rust/WASM
cd rust
rustup target add wasm32-wasip1   # once
cargo build --release --target wasm32-wasip1
cd ..
node --experimental-wasi-unstable-preview1 run_rust.mjs 500000
```

Both print a single JSON line with `checksum`, `elapsed_ms` (self-measured, excludes WASM instantiation), and (Go only) GC stats. The Node harness scripts additionally print instantiation time and WASM binary size to stderr.

## What this does NOT cover

Run in Node (a plain WASM host), not inside an actual Tauri webview or Tauri's Android webview — the environment this was produced in has no display server, no Tauri CLI, and no Android SDK/emulator. Those two legs of `.vibe/backlog/008`'s acceptance criteria (webview-specific behavior, and the Android secure-context/WASM question) still need to be run on a real workstation with Android Studio and a GUI — not done here. See `RESULTS.md` for what is and isn't concluded from this data.
