---
date: 2026-08-16
status: accepted
---
# `.cmd` command-string modifier prefixes are recognized but not semantically implemented

**Context:** MUGEN/Ikemen GO `.cmd` command strings support prefix modifiers on a step: `~` (release detection), `$` (4-way direction guarantee), `/` (hold), and `>` (strict single-tick timing between steps). Backlog item 008's own acceptance criteria and "Done" example (`~D, DF, F, a`, a real quarter-circle-forward + punch motion) only require exact-sequence recognition within a buffered time window, close-sequence rejection, and the time-window edge case — none of which depend on these modifiers' full semantics.

**Decision:** `input`'s command-string parser strips a leading `~`/`$`/`/`/`>` from a step token and matches the remainder the same as an unmodified step — the modifier is recognized syntactically (so a real `.cmd` file's command strings parse without error) but has no distinct behavior of its own yet.

**Reason:** Full modifier semantics (true press/release edge detection, held-across-steps tracking, strict one-tick timing) add real complexity for behavior the current acceptance criteria don't exercise. Stripping and matching plain keeps the real quarter-circle-forward example (and any other command using these modifiers) parseable and functionally matchable now, without guessing at semantics that would need dedicated acceptance criteria of their own to validate correctly.

**Rejected alternatives:** Implementing full modifier semantics up front — rejected: speculative scope beyond what item 008 requires, per this org's standing preference against building ahead of a proven need (`vibe:review-overengineering`). Rejecting/erroring on a step carrying a modifier — rejected: real `.cmd` files (including this item's own reference example) use `~` routinely; erroring would make the parser reject exactly the input it's meant to recognize.

**Follow-up:** A future backlog item can implement true edge/hold/strict-timing semantics if real character `.cmd` files are found where the current simplification produces a wrong match/non-match.
