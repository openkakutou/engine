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
