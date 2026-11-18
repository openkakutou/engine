---
status: in_progress
---
# Triggers that read the opponent, hit contacts and animation elements

## Description
Second slice of item 028 (split by family, 2026-11-18). 028 delivered the triggers that read only the fighter itself (Life, Power, Alive, Facing, PrevStateNo, RoundNo, RoundState, IsHelper, Pos/Vel X/Y, StateType, MoveType, Random with a seed in match state, the math functions). What real characters still use, by exposure in the conformance run, and was left out:
- **Opponent**: `P2Dist X/Y`, `P2BodyDist X/Y`, `P2StateNo`, `P2MoveType`, `P2StateType`, `P2Life`, `NumEnemy`, `NumTarget`. Needs the opponent's fighter state on `evaluator.Context`.
- **Hit contacts**: `MoveContact`, `MoveHit`, `MoveGuarded`, `HitDefAttr`, `HitFall`, `CanRecover`, plus the rest of `GetHitVar` (`yvel`, `fall`, ...), following MUGEN timing, with a documented Ikemen GO divergence table.
- **Animation**: `AnimElem` (`AnimElem = n, op t`), `AnimElemTime(n)`, `AnimElemNo`, `AnimExist`, `SelfAnimExist`.
- **Other**: `Const(name)` (e.g. `Const(movement.stand.friction.threshold)`, reading the character's `[Const]`/`[Data]`/`[Size]`/`[Velocity]`/`[Movement]` values), `FVar(n)`, `SysFVar(n)`, `PalNo`, `NumHelper`, `NumExplod`, `Time` redirections, `AILevel`/`AILevelF` (first trigger kfm_zss evaluates, in its command.zss state -1; found by item 054).

Ground each trigger in Ikemen GO's source (commit `54d4148a`), file and line, as 028 did. Compat is a MUST (roadmap decision `038`).

## Acceptance Criteria
- [ ] Each trigger above evaluates against live state and is covered by a test with an independently derived expected value
- [ ] AnimElem and the MoveContact family follow MUGEN timing semantics, with a documented Ikemen GO divergence table
- [ ] Conformance run: the unknown-trigger list no longer names any of them

## Notes
Found while splitting 028. The conformance run after 028 lists `const()`, `animelem`, `animelemtime()`, `fvar()`, `palno`, `numtarget` among the most exposed names left.
Still open from 028, smaller: `RoundState` is a constant 2 (Tick only runs while fighting) and `Alive` is health > 0 until KO flags exist; `Pos X` ignores the camera; `round.ResetRound` restarts the random seed at 0 instead of carrying it across rounds.
