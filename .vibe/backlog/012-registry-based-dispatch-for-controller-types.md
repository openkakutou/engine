---
status: todo
---
# Registry Based Dispatch For Controller Types

## Description
The SOLID review agent noted that `statemachine.ApplyController`'s switch (`ChangeState`/`VarSet`) and `evaluator.evalIdentifier`/`evalCall`'s switches are not open for extension — adding a new controller type or trigger/function name requires modifying these functions directly. The overengineering agent in the same review pass found the current design intentionally avoids speculative abstraction (documented in this repo's own ADRs), so a registry-based dispatch was deliberately deferred rather than applied automatically. Revisit this once a further controller type or trigger/function name is actually being added.

## Acceptance Criteria
- [ ] Controller-type dispatch in `statemachine.ApplyController` is registry-based (e.g. a `map[string]func(...)`), so adding a controller type is an addition, not a modification of the switch
- [ ] Trigger/function-name dispatch in `evaluator.evalIdentifier`/`evalCall` is similarly registry-based
- [ ] Existing `statemachine` and `evaluator` test suites pass unchanged

## Notes
Deliberately deferred by `/vibe:review` on 2026-08-31 (commit `019027a`) — do not implement until a real second/third controller type or trigger is actually being added, to avoid reintroducing the premature generality this repo's own ADRs explicitly reject (see `.vibe/decisions/`, and the review's own overengineering-agent pass finding 0 issues in the current design).

Re-checked 2026-09-17 (`/vibe:auto`): precondition still does not hold. `git diff 019027a..HEAD -- statemachine/statemachine.go evaluator/eval.go` is empty — `ApplyController`'s switch is still only `ChangeState`/`VarSet`, and `evalIdentifier`/`evalCall`'s switches are unchanged. `ControllerTypeHitDef` does exist as a constant, but it was already introduced in the same `019027a` commit this deferral note was written against, and it is (and was, at deferral time) deliberately handled outside `ApplyController`'s switch, via its own classification in the root package's `tick.go` — not a new case added to the switch since the deferral. No new trigger/function name was added to the evaluator either. Left `status: todo`; do not implement yet.

Re-checked 2026-09-21 (`/vibe:fix 012 --auto`): precondition still does not hold. `git diff 019027a..HEAD -- statemachine/statemachine.go evaluator/eval.go` is still empty (21 commits landed since the deferral, none touching either file's dispatch). `ApplyController`'s switch remains exactly `ChangeState`/`VarSet`; `evalIdentifier` still handles only `time`/`stateno`/`anim`/`animtime`/`ctrl`/`command`, and `evalCall` only `var`/`sysvar`/`ifelse` — same sets as at the 2026-09-17 re-check. No new controller type or trigger/function name has been added. Left `status: todo`; do not implement yet.
