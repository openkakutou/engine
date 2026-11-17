---
status: done
---
# Real Character Conformance Harness

## Description
Add a harness that loads real MUGEN and Ikemen GO characters, runs a scripted match for N ticks, and reports which controller types and triggers were hit but unsupported. It turns "engine is early-stage" into a measurable coverage number and picks the next controllers to build.

## Acceptance Criteria
- [ ] A command (go test tag or cmd tool) runs a given character folder for a fixed tick count and lists unsupported controllers/triggers with occurrence counts
- [ ] At least 5 real characters (MUGEN and Ikemen GO) are run, with results recorded in docs
- [ ] Output is deterministic so coverage can be tracked over time
- [ ] No real character data is committed, only paths and aggregated results

## Notes
Same approach as character/stage corpus_compat tests. The acceptance characters of roadmap 013 should be among those used. Do not commit copyrighted assets.
Combat-completeness milestone, step order in roadmap decision `036`.
