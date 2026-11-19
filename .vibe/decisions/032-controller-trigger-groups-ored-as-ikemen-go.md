---
date: 2026-11-19
status: accepted
---
# A controller's trigger groups are ORed, evaluated as Ikemen GO does

Supersedes 003.

**Context:** Item 061. character v0.16.0 (character#080) gives `cns.Controller` `TriggerAll` and `TriggerGroups` (`[i]` = every `trigger<i+1>` line, dropped from the first missing number on), so the group structure decision 003 could not see is now available. Real controllers rely on OR, starting with `common1.cns:128-133` (walk animation) and `kfm.cmd:630-637` (light punch).

**Decision:** `statemachine.triggersPass` follows Ikemen GO's `stateCompileCNS` (`src/compiler.go`, commit `54d4148a`): every `triggerall` line first, stopping at the first false one (`:7443-7448`); then the groups in order, each stopping at its first false line, the whole stopping at the first true group (`:7451-7495`). A `:=` or `Random` in a skipped line never runs. Non-empty `Triggers` with no `TriggerGroups` means no `trigger1`, which Ikemen GO refuses ("Missing trigger1", `:7439-7441`): such a controller never fires. A controller with neither `TriggerAll` nor `TriggerGroups` falls back to ANDing its flat `Triggers` (empty: unconditional), so hand-built controllers (tests, `zssexec`'s HitDef copy) behave as before. The conformance harness neutralizes a controller by replacing its groups as well as its flat list.

**Divergence:** Ikemen GO rejects a "Missing trigger1" controller at load; the engine loads it and never fires it. A parsed controller with only `trigger2` (no `triggerall`) is indistinguishable from a hand-built flat one and is ANDed (unverified gap, see item 061 report).

**Rejected:** keep AND (decision 003) -- breaks every OR'd controller; regroup in engine from raw lines -- the data is character's to provide.
