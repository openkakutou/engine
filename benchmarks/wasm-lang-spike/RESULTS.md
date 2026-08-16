# Results — 2026-08-16

Node v22.13.0, Go 1.26.1, Rust 1.97.1 (`wasm32-wasip1`), x86_64 Linux. 3 runs each at 500,000 ticks (checksum identical across every run and every language, confirming both implementations do equivalent work).

| | Go/WASM | Rust/WASM | Ratio |
|---|---|---|---|
| Binary size | 2,700,392 bytes (2.58 MiB) | 119,288 bytes (116 KiB) | **Rust ~22.6× smaller** |
| Instantiation time (avg) | 11.0 ms | 3.4 ms | **Rust ~3.2× faster** |
| Execution time, 500k ticks (avg) | 3317.4 ms | 1453.7 ms | **Rust ~2.28× faster** |
| Per-tick cost (both fighters) | 6.63 µs | 2.91 µs | — |
| GC pauses over the run | 277 pauses, 16.68 ms total (avg 60 µs/pause) | 0 (no GC) | — |
| Total bytes allocated over the run | 851,124,256 (851 MiB, reclaimed) | n/a (no heap alloc tracking for this comparison) | — |

Raw run output: see the commit this file was added in, or reproduce via the README.

## What this supports

- **Rust is faster and smaller here, clearly.** ~2.3× faster execution, ~3.2× faster cold-start, ~22.6× smaller binary. Consistent with the general Go-vs-Rust-WASM expectation from `.vibe/decisions/020` and industry data — this spike's numbers are specific to this org's workload shape, not borrowed from someone else's benchmark.
- **Binary size and startup time are the clearest wins for Rust**, and matter directly for a desktop/mobile app's first-launch experience (ties back to the Wails-vs-Tauri migration anecdote cited in decision `020` — smaller/faster launch was the headline result there too).

## What this does NOT support

- **Go's GC pauses do not look dangerous to frame timing at this workload's scale.** 277 pauses averaging ~60 µs each, against a 16.67 ms budget for 60 fps (a pause would need to approach that order of magnitude to actually risk a dropped frame) — none of the individual pauses come close. The "GC pauses could cause frame drops" concern raised while deciding `.vibe/decisions/020` is **not confirmed** by this data; it may still matter at the scale of `engine`'s real workload (more simultaneous hitboxes, fuller trigger trees per real character, possibly rollback re-simulation), which is likely much heavier than this synthetic proxy — but it is not something this spike found evidence for, and should not be asserted as settled either way.
- **Neither language is anywhere near a 60fps budget for this specific workload.** 6.63 µs (Go) or 2.91 µs (Rust) per tick, against a 16,670 µs budget — over 2,500× and 5,700× headroom respectively. At this workload's size, the language choice has no bearing on hitting 60fps; it would only start to matter if the real `engine` workload turns out to be roughly 100-1000× heavier than this proxy (plausible for a full character roster with complete movesets, but unverified).
- **Nothing here says whether Rust is *worth* the switch**, only that it's faster on this proxy. That's a cost/benefit call (rewrite effort across `engine` and possibly `character`/`stage`/`sff`, splitting the team's Go/Rust attention, `engine`'s 4 already-done Go backlog items becoming throwaway if a full rewrite is chosen) that this data feeds into, not one it makes on its own.

## Not covered by this spike (see `.vibe/backlog/008`)

- **Tauri webview execution** (desktop: WebView2/WebKit/WebKitGTK) — not run; the environment this spike was produced in has no display server or Tauri CLI. Node's WASM host is not guaranteed to match a webview's JS engine's WASM performance characteristics exactly (different JIT, different GC tuning for the JS side hosting Go's runtime).
- **Tauri Android webview execution** — not run; no Android SDK/emulator available. This is the one still fully open item: whether WASM (Go or Rust) loads and executes at all given Android WebView's handling of Tauri's asset origin as a non-secure context (see decision `020`'s later amendment and `wry` issues #1709/#1710). Needs a real Android environment to answer.
- **Realistic engine-shaped load.** This workload is a plausible proxy, not `engine`'s actual trigger evaluator/state machine/hit detection code — those don't fully exist yet (backlog `005`/`006` are still `todo`). Revisiting with the real implementation once it exists (as this item's acceptance criteria originally called for) would be more conclusive than this proxy, at the cost of the throwaway-work risk already discussed in `.vibe/backlog/008`.

## Recommendation

Data supports Rust being the faster, leaner choice *if* the org decides raw performance and binary size are worth prioritizing over keeping everything in Go — but doesn't manufacture urgency: the GC-pause fear specifically isn't backed up by this data, and both languages have enormous headroom against a 60fps budget at this workload's scale. The Tauri-Android-webview question remains the one unresolved, potentially blocking unknown (independent of Go vs. Rust) and should be validated on a real machine before treating any of the five targets as fully confirmed.
