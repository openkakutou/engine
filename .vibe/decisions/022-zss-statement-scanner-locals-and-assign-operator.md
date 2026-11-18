---
date: 2026-11-18
status: accepted
---
# .zss: statement scanner over the whole body, `:=` in the evaluator, locals as a frame window

**Context:** Item 048. The line-based regex parser rejected 34 of the 62 blocks of Ikemen GO's `common1.cns.zss`. The file uses `else if`, statements spanning several lines, `let`/`$x` locals, functions with arguments and return values, `persistent(N)`, `ignoreHitPause` and `:=`. One of its conditions assigns with `:=` (line 316).
**Decision:**
- zssexec reads the body as one continuous text, ending each statement at `;` or `{`. It mirrors Ikemen GO's `blockAttribSet`, `subBlock`, `stateBlock`, `callFunc` and `letAssign` (src/compiler.go:7545-8558, commit `54d4148a`). Expression text stays a string for the evaluator (decision 008).
- `:=` is an evaluator operator assigning var/sysvar, usable inside any expression, as in Ikemen GO. Locals are read-only in expressions.
- Locals are passed as `evaluator.Context.Locals`: a name list fixed at parse time, plus a window into a zssexec value stack that only grows. Ikemen GO gives each Statedef or Function one flat table of slots, and each call gets its own frame (arguments, then return values, then lets).
- The `persistent(N)` counters live in a fixed-size array in the Context and are reset on a state change.
- `ignoreHitPause` is parsed and recorded with Ikemen GO's inheritance rules. Running it during a freeze stays the gap of decision 021.
**Reason:** It is the closest to Ikemen GO's grammar and errors, it has no per-tick heap allocation, and it changes the evaluator least.
**Rejected alternatives:**
- *Compiling each statement into a closure*: same cost, but harder to test and unlike the rest of the code.
- *Resolving locals to slot numbers at parse time*: matches Ikemen's `OC_localvar`, but it needs a second parse-cache key in the evaluator. To revisit if profiling shows the name lookup.
- *A fixed `[256]Value` array on the Context*: 4 KB copied on every controller call.
- *A map*: it hashes on every read and has to be cleared on every run.
