---
date: 2026-08-09
status: accepted
---
# A controller's triggers are combined with AND, not MUGEN's OR-of-groups

**Context:** Implementing backlog item 003 (state machine execution): for each state controller, decide whether its trigger condition passes before applying its effect. Real MUGEN/Ikemen groups a controller's `triggerN` lines by their numeric suffix — multiple lines sharing one number are ANDed, and the different numbered groups are ORed together, while `triggerall` lines AND across every group. `character/cns`'s `Controller.Triggers` (item 020 in `character`) deliberately does not preserve which numeric group each trigger string came from: it is a flat `[]string` in file order (see `character`'s decision 011 on `Controller.Parameters`/`Triggers` staying untyped, unevaluated data).

**Decision:** `statemachine.Step` evaluates a controller's trigger condition by evaluating every string in `Controller.Triggers` and requiring all of them to be true (logical AND across the whole flat list), rather than attempting to reconstruct MUGEN's group-based OR/AND structure.

**Reason:** The group-number information genuinely does not exist in the data this item consumes — reconstructing it would require reopening `character/cns`'s already-closed parser to retain the `triggerN` suffix per string, which is out of this item's scope and out of `engine`'s control over another repo's already-shipped model. AND-across-the-flat-list is the closest safe default: every real `.cns` controller that uses only `triggerall`, or only a single `trigger1` group, behaves identically under this rule and under full MUGEN semantics — the divergence only shows up for a controller that actually relies on multiple OR'd trigger groups, which this item's acceptance criteria do not require supporting yet.

**Rejected alternatives:**
- **Request `character/cns` to retain the `triggerN` group number:** correct long-term fix, but reopens an already-closed, already-tested item in a different repo outside this item's scope; can be revisited in a future item if a real state actually needs OR'd trigger groups.
- **Treat trigger evaluation as OR across the flat list:** wrong default — would make a controller far too eager to apply for the common single-group/`triggerall` case, which is the overwhelming majority of real controllers.
