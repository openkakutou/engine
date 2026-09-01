#!/usr/bin/env node
// smoke.mjs is a Node.js verification harness for the WASM entrypoint built
// from this directory (see main.go) — it exercises the module the same way
// a browser consumer would (fetch/instantiate the .wasm, call the exposed
// global functions, read back the result), without requiring an actual
// browser. It is not part of `go test` — syscall/js glue cannot run under
// the plain Go toolchain — and doubles as a minimal usage example for a JS
// consumer, mirroring character/cmd/wasm's own smoke.mjs.
//
// Usage: node cmd/wasm/smoke.mjs [path/to/engine.wasm]
// (defaults to ./engine.wasm, relative to the repo root)

import { execSync } from "node:child_process";
import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..", "..");
const wasmPath = path.resolve(process.argv[2] || path.join(repoRoot, "engine.wasm"));

const goroot = execSync("go env GOROOT").toString().trim();
const wasmExecPath = path.join(goroot, "lib", "wasm", "wasm_exec.js");

// wasm_exec.js defines a global `Go` constructor; importing it for its
// side effect is the same pattern used to load it in a browser <script> tag.
await import(`file://${wasmExecPath}`);

const go = new globalThis.Go();
const { instance } = await WebAssembly.instantiate(readFileSync(wasmPath), go.importObject);
go.run(instance); // does not return: keeps the Go runtime (and its registered functions) alive

function assert(condition, message) {
	if (!condition) {
		console.error(`FAIL: ${message}`);
		process.exitCode = 1;
	} else {
		console.log(`ok - ${message}`);
	}
}

function call(fn, request) {
	const result = fn(JSON.stringify(request));
	return { data: result.data === null ? null : JSON.parse(result.data), error: result.error };
}

// --- fixture data: a minimal, self-authored two-state character (idle ->
// attack), the same "styled after real MUGEN/Ikemen idioms, not a literal
// downloaded file" convention this repo's own kfm_idle.cns/kfm_idle.zss
// testdata fixtures already established (see statemachine/testdata,
// zssexec/testdata) -- see the integration test's own testdata for the
// richer, file-based version of the same idea. ---
const attackerStates = {
	0: {
		number: 0, type: "S", moveType: "I", physics: "S", anim: 0, ctrl: true,
		controllers: [
			{ type: "ChangeState", triggers: ['Command = "atk"'], parameters: { value: "200" } },
		],
	},
	200: {
		number: 200, type: "S", moveType: "A", physics: "S", anim: 200, ctrl: false,
		controllers: [
			{ type: "HitDef", triggers: ["Time = 0"], parameters: { damage: "30" } },
		],
	},
};
const defenderStates = {
	0: { number: 0, type: "S", moveType: "I", physics: "S", anim: 0, ctrl: true, controllers: [] },
};
const attackerAnimations = [
	{ number: 0, frames: [{ group: 0, image: 0, x: 0, y: 0, time: -1, clsn1: [], clsn2: [] }], loopStart: 0 },
	{ number: 200, frames: [{ group: 0, image: 0, x: 0, y: 0, time: 100, clsn1: [{ left: -5, top: -50, right: 5, bottom: 0 }], clsn2: [] }], loopStart: 0 },
];
const defenderAnimations = [
	{ number: 0, frames: [{ group: 0, image: 0, x: 0, y: 0, time: -1, clsn1: [], clsn2: [{ left: -5, top: -50, right: 5, bottom: 0 }] }], loopStart: 0 },
];
const attackerCommands = { remap: {}, defaults: { time: 15, bufferTime: 1 }, commands: [{ name: "atk", input: "a", time: 0, bufferTime: 0 }], states: [] };
const emptyCommands = { remap: {}, defaults: { time: 0, bufferTime: 0 }, commands: [], states: [] };

const newMatchRequest = {
	programs: [
		{ states: attackerStates, animations: attackerAnimations, commands: attackerCommands },
		{ states: defenderStates, animations: defenderAnimations, commands: emptyCommands },
	],
	starting: [
		{ side: 0, position: { x: 0, y: 0 }, facing: 0, velocity: { x: 0, y: 0 }, stateNo: 0, health: 1000 },
		{ side: 1, position: { x: 0, y: 0 }, facing: 1, velocity: { x: 0, y: 0 }, stateNo: 0, health: 20 },
	],
	roundTimer: 1000,
	bestOf: 3,
	bounds: { left: -1000, right: 1000, topBound: 0, bottomBound: 0 },
	gravity: 0,
	comboWindow: 60,
};

// --- nominal path: start a match ---
const created = call(globalThis.OpenKakutouEngine.newMatch, newMatchRequest);
assert(created.error === null, `newMatch reports no error (got: ${created.error})`);
assert(typeof created.data?.matchId === "number", "newMatch returns a numeric matchId");
assert(created.data?.state?.round === 1, `new match starts at round 1 (got: ${created.data?.state?.round})`);
assert(created.data?.progress?.bestOf === 3, "new match's progress carries the requested bestOf");
assert(created.data?.animations?.[0]?.animNo === 0, `newMatch reports P1's starting animNo as 0 (got: ${created.data?.animations?.[0]?.animNo})`);
assert(created.data?.animations?.[0]?.animTime === 0, `newMatch reports P1's starting animTime as 0 (got: ${created.data?.animations?.[0]?.animTime})`);
assert(created.data?.animations?.[1]?.animNo === 0, `newMatch reports P2's starting animNo as 0 (got: ${created.data?.animations?.[1]?.animNo})`);

const matchId = created.data.matchId;

// --- tick with no input: no state change, no hit, round still in progress ---
const idleTick = call(globalThis.OpenKakutouEngine.tick, { matchId, inputs: [{}, {}] });
assert(idleTick.error === null, `idle tick reports no error (got: ${idleTick.error})`);
assert(idleTick.data?.round?.outcome === 0, `idle tick reports OutcomeNone (got: ${idleTick.data?.round?.outcome})`);
assert(idleTick.data?.matchOver === false, "match is not over after an idle tick");
assert(idleTick.data?.animations?.[0]?.animNo === 0, `idle tick: P1's animNo is still 0, no transition happened (got: ${idleTick.data?.animations?.[0]?.animNo})`);
assert(idleTick.data?.animations?.[0]?.animTime === 1, `idle tick: P1's animTime advanced to 1 (got: ${idleTick.data?.animations?.[0]?.animTime})`);

// --- tick with P1's attack button held: P1's ChangeState controller
// (state 0) fires, moving it into its attack state (200) -- a controller
// evaluated on the source state can only ever transition *into* a target
// state, never also evaluate that target's own controllers in the same
// call, so no hit lands yet on this tick. ---
const transitionTick = call(globalThis.OpenKakutouEngine.tick, { matchId, inputs: [{ buttons: { a: true } }, {}] });
assert(transitionTick.error === null, `transition tick reports no error (got: ${transitionTick.error})`);
assert(transitionTick.data?.round?.outcome === 0, `transition tick reports OutcomeNone -- no hit yet (got: ${transitionTick.data?.round?.outcome})`);
assert(transitionTick.data?.state?.fighters?.[1]?.health === 20, "P2 is untouched on the transition tick");
assert(transitionTick.data?.animations?.[0]?.animNo === 200, `transition tick: P1's animNo switches to its new state's 200 (got: ${transitionTick.data?.animations?.[0]?.animNo})`);
assert(transitionTick.data?.animations?.[0]?.animTime === 0, `transition tick: P1's animTime resets to 0 on the transition (got: ${transitionTick.data?.animations?.[0]?.animTime})`);
assert(transitionTick.data?.animations?.[1]?.animNo === 0, `transition tick: P2 (untouched) keeps animNo 0 (got: ${transitionTick.data?.animations?.[1]?.animNo})`);
assert(transitionTick.data?.animations?.[1]?.animTime === 2, `transition tick: P2's animTime keeps advancing, now 2 (got: ${transitionTick.data?.animations?.[1]?.animTime})`);

// --- next tick: P1 is now actually in state 200, so its own HitDef
// controller (trigger "Time = 0", true on the first tick spent in that
// state) evaluates, landing on P2 (whose Clsn2 box overlaps P1's Clsn1 box
// at the same position) and reducing P2's health from 20 to 0 -- a KO,
// deciding round 1 for P1. ---
const hitTick = call(globalThis.OpenKakutouEngine.tick, { matchId, inputs: [{}, {}] });
assert(hitTick.error === null, `hit tick reports no error (got: ${hitTick.error})`);
assert(hitTick.data?.state?.fighters?.[1]?.health === 0, `P2 health drops to 0 (got: ${hitTick.data?.state?.fighters?.[1]?.health})`);
assert(hitTick.data?.round?.outcome === 1, `hit tick reports OutcomeKO (got: ${hitTick.data?.round?.outcome})`);
assert(hitTick.data?.round?.winner === 0, `P1 (side 0) wins the round (got: ${hitTick.data?.round?.winner})`);
assert(hitTick.data?.progress?.wins?.[0] === 1, "P1's round win is recorded in progress");
assert(hitTick.data?.matchOver === false, "bestOf 3 is not decided after a single round win");
assert(hitTick.data?.animations?.[0]?.animNo === 200, `hit tick: P1 stays in animNo 200, no further transition (got: ${hitTick.data?.animations?.[0]?.animNo})`);
assert(hitTick.data?.animations?.[0]?.animTime === 1, `hit tick: P1's animTime advances to 1 within animNo 200 (got: ${hitTick.data?.animations?.[0]?.animTime})`);

// --- resetRound: both fighters restored for round 2 ---
const reset = call(globalThis.OpenKakutouEngine.resetRound, {
	matchId,
	roundTimer: 1000,
	starting: newMatchRequest.starting,
});
assert(reset.error === null, `resetRound reports no error (got: ${reset.error})`);
assert(reset.data?.state?.round === 2, `resetRound advances to round 2 (got: ${reset.data?.state?.round})`);
assert(reset.data?.state?.fighters?.[1]?.health === 20, `resetRound restores P2's health to 20 (got: ${reset.data?.state?.fighters?.[1]?.health})`);
assert(reset.data?.animations?.[0]?.animNo === 0, `resetRound restores P1's animNo to its round-start state 0 (got: ${reset.data?.animations?.[0]?.animNo})`);
assert(reset.data?.animations?.[0]?.animTime === 0, `resetRound restores P1's animTime to 0 (got: ${reset.data?.animations?.[0]?.animTime})`);

// --- error path: an unknown match ID must not crash the module ---
const unknown = call(globalThis.OpenKakutouEngine.tick, { matchId: 999999, inputs: [{}, {}] });
assert(unknown.data === null, "tick on an unknown matchId: data is null");
assert(typeof unknown.error === "string" && unknown.error.length > 0, `tick on an unknown matchId: error is a non-empty string (got: ${unknown.error})`);

// --- closeMatch: releases the session; the same matchId is then unknown ---
const closed = call(globalThis.OpenKakutouEngine.closeMatch, { matchId });
assert(closed.error === null, `closeMatch reports no error (got: ${closed.error})`);

const tickAfterClose = call(globalThis.OpenKakutouEngine.tick, { matchId, inputs: [{}, {}] });
assert(tickAfterClose.data === null, "tick on a closed matchId: data is null");
assert(typeof tickAfterClose.error === "string" && tickAfterClose.error.length > 0, `tick on a closed matchId: error is a non-empty string (got: ${tickAfterClose.error})`);

// --- error path: closing an already-unknown match ID reports an error, not a crash ---
const closeUnknown = call(globalThis.OpenKakutouEngine.closeMatch, { matchId: 999999 });
assert(closeUnknown.data === null, "closeMatch on an unknown matchId: data is null");
assert(typeof closeUnknown.error === "string" && closeUnknown.error.length > 0, `closeMatch on an unknown matchId: error is a non-empty string (got: ${closeUnknown.error})`);

// --- error path: a malformed request (wrong argument count) must not crash the module ---
let threw = false;
try {
	globalThis.OpenKakutouEngine.newMatch();
} catch {
	threw = true;
}
assert(!threw, "calling newMatch with no arguments does not throw");

if (process.exitCode) {
	console.error("smoke test FAILED");
} else {
	console.log("smoke test OK");
}
