# Module: combat
**Role:** Applies `hitdetect`'s `HitEvent`s to actual match state each simulation tick: damage subtracted from the defender's health (floored at zero), and a per-defender combo counter tracked across ticks. Interprets a MUGEN `HitDef`'s raw `character/cns` `"damage"` parameter numerically for the first time in the backlog, falling back to `DefaultDamage` when it's missing or unreadable. Does not drive any state-machine transition — see `.vibe/decisions/010`.
**Files:** `combat/combat.go`
**Exports:** `ApplyHits(state match.MatchState, events []hitdetect.HitEvent, attacker match.Side, hitDef cns.Controller, combo ComboState, tick, comboWindow int) (match.MatchState, ComboState, HitResult)`, `ResetCombo() ComboState`, `ComboState`, `HitResult`, `DefaultDamage`
**Depends on:** `modules/match.md` (`MatchState`/`FighterState`/`Side`), `modules/hitdetect.md` (`HitEvent`), `github.com/openkakutou/character/cns` (`Controller`, external module, read-only)
