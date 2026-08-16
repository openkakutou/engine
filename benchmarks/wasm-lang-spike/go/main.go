// Synthetic per-tick combat-simulation workload, mirroring the shape of
// engine's real per-frame work (trigger evaluation + state transitions +
// hit detection + physics) closely enough to compare Go/WASM vs Rust/WASM
// execution characteristics -- NOT a claim of byte-for-byte fidelity to
// engine's actual (still partly unwritten) implementation. See roadmap
// .vibe/backlog/008.
package main

import (
	"fmt"
	"math"
	"os"
	"runtime"
	"strconv"
	"time"
)

const (
	numFighters      = 2
	numPrograms      = 20
	numTransitions   = 5
	numHitboxes      = 3
	numHurtboxes     = 3
	numVars          = 8
)

type vec2 struct{ x, y float64 }

type box struct{ minX, minY, maxX, maxY float64 }

type fighter struct {
	pos, vel  vec2
	stateID   int
	vars      [numVars]float64
	hitboxes  [numHitboxes]box
	hurtboxes [numHurtboxes]box
	health    float64
	grounded  bool
}

// Tiny stack-based bytecode VM standing in for CNS trigger/expression
// evaluation. Opcodes:
const (
	opPushConst = iota
	opPushVar
	opAdd
	opSub
	opMul
	opGT
	opLT
	opAnd
)

type instr struct {
	op  int
	arg float64 // constant value or var index (as float64)
}

// 20 fixed programs, ~6 instructions each -- built once, evaluated every tick.
func buildPrograms() [numPrograms][]instr {
	var progs [numPrograms][]instr
	for p := 0; p < numPrograms; p++ {
		v0 := float64(p % numVars)
		v1 := float64((p + 3) % numVars)
		progs[p] = []instr{
			{opPushVar, v0},
			{opPushConst, float64(p%5) * 0.5},
			{opAdd, 0},
			{opPushVar, v1},
			{opGT, 0},
		}
	}
	return progs
}

// evalProgram allocates a fresh stack each call -- deliberate, mirrors a
// realistic per-evaluation transient allocation instead of a pre-warmed
// reused buffer, since that's the pattern most likely to show up in real
// interpreter-style evaluation code.
func evalProgram(p []instr, f *fighter) float64 {
	stack := make([]float64, 0, 8)
	for _, ins := range p {
		switch ins.op {
		case opPushConst:
			stack = append(stack, ins.arg)
		case opPushVar:
			stack = append(stack, f.vars[int(ins.arg)])
		case opAdd:
			b := stack[len(stack)-1]
			a := stack[len(stack)-2]
			stack = stack[:len(stack)-2]
			stack = append(stack, a+b)
		case opSub:
			b := stack[len(stack)-1]
			a := stack[len(stack)-2]
			stack = stack[:len(stack)-2]
			stack = append(stack, a-b)
		case opMul:
			b := stack[len(stack)-1]
			a := stack[len(stack)-2]
			stack = stack[:len(stack)-2]
			stack = append(stack, a*b)
		case opGT:
			b := stack[len(stack)-1]
			a := stack[len(stack)-2]
			stack = stack[:len(stack)-2]
			if a > b {
				stack = append(stack, 1)
			} else {
				stack = append(stack, 0)
			}
		case opLT:
			b := stack[len(stack)-1]
			a := stack[len(stack)-2]
			stack = stack[:len(stack)-2]
			if a < b {
				stack = append(stack, 1)
			} else {
				stack = append(stack, 0)
			}
		case opAnd:
			b := stack[len(stack)-1]
			a := stack[len(stack)-2]
			stack = stack[:len(stack)-2]
			if a != 0 && b != 0 {
				stack = append(stack, 1)
			} else {
				stack = append(stack, 0)
			}
		}
	}
	return stack[len(stack)-1]
}

type transitionRecord struct {
	fighterIdx int
	fromState  int
	toState    int
}

type hitRecord struct {
	attacker, defender int
	hbIdx, hurtIdx      int
}

func boxesOverlap(a, b box) bool {
	return a.minX < b.maxX && a.maxX > b.minX && a.minY < b.maxY && a.maxY > b.minY
}

func main() {
	ticks := 2_000_000
	if len(os.Args) > 1 {
		if n, err := strconv.Atoi(os.Args[1]); err == nil {
			ticks = n
		}
	}

	programs := buildPrograms()

	var fighters [numFighters]fighter
	for i := range fighters {
		fighters[i].health = 1000
		fighters[i].grounded = true
		for h := 0; h < numHitboxes; h++ {
			fighters[i].hitboxes[h] = box{minX: 10, minY: 0, maxX: 20, maxY: 10}
		}
		for h := 0; h < numHurtboxes; h++ {
			fighters[i].hurtboxes[h] = box{minX: 0, minY: 0, maxX: 15, maxY: 20}
		}
	}
	fighters[1].pos.x = 100

	var runtimeGCBefore runtime.MemStats
	runtime.ReadMemStats(&runtimeGCBefore)

	start := time.Now()

	var checksum float64
	transitionCount := 0
	hitCount := 0
	triggerTrueCount := 0

	const gravity = -0.5
	const groundY = 0.0
	const stageMinX, stageMaxX = -200.0, 200.0

	for t := 0; t < ticks; t++ {
		for fi := 0; fi < numFighters; fi++ {
			f := &fighters[fi]
			base := float64((t*7+fi*13)%97) * 0.1

			// 1. Trigger evaluation (20 programs/tick/fighter)
			for p := 0; p < numPrograms; p++ {
				v := evalProgram(programs[p], f)
				if v != 0 {
					triggerTrueCount++
				}
			}

			// 2. State transitions (5 candidates/tick/fighter)
			transitions := make([]transitionRecord, 0, 2)
			for c := 0; c < numTransitions; c++ {
				cond := f.vars[c%numVars] + base
				if math.Mod(cond, 3.0) < 1.0 {
					from := f.stateID
					f.stateID = (f.stateID + c + 1) % 40
					f.vars[c%numVars] += 0.01
					transitions = append(transitions, transitionRecord{fi, from, f.stateID})
				}
			}
			transitionCount += len(transitions)

			// 3. Physics: integrate velocity, gravity, ground clamp, stage clamp
			if !f.grounded {
				f.vel.y += gravity
			}
			f.pos.x += f.vel.x
			f.pos.y += f.vel.y
			if f.pos.y <= groundY {
				f.pos.y = groundY
				f.vel.y = 0
				f.grounded = true
			} else {
				f.grounded = false
			}
			if f.pos.x < stageMinX {
				f.pos.x = stageMinX
			}
			if f.pos.x > stageMaxX {
				f.pos.x = stageMaxX
			}
		}

		// 4. Hit detection: fighter 0's hitboxes vs fighter 1's hurtboxes (and vice versa)
		hits := make([]hitRecord, 0, 2)
		for hb := 0; hb < numHitboxes; hb++ {
			for hu := 0; hu < numHurtboxes; hu++ {
				if boxesOverlap(fighters[0].hitboxes[hb], fighters[1].hurtboxes[hu]) {
					hits = append(hits, hitRecord{0, 1, hb, hu})
					fighters[1].health -= 1
				}
				if boxesOverlap(fighters[1].hitboxes[hb], fighters[0].hurtboxes[hu]) {
					hits = append(hits, hitRecord{1, 0, hb, hu})
					fighters[0].health -= 1
				}
			}
		}
		hitCount += len(hits)

		// Nudge hitbox positions deterministically so overlap varies over time
		shift := float64(t%40) - 20
		fighters[0].hitboxes[0].minX = 10 + shift
		fighters[0].hitboxes[0].maxX = 20 + shift
	}

	elapsed := time.Since(start)

	var runtimeGCAfter runtime.MemStats
	runtime.ReadMemStats(&runtimeGCAfter)

	for i := range fighters {
		checksum += fighters[i].pos.x + fighters[i].pos.y + fighters[i].health + float64(fighters[i].stateID)
		for _, v := range fighters[i].vars {
			checksum += v
		}
	}

	numGC := runtimeGCAfter.NumGC - runtimeGCBefore.NumGC
	pauseTotalNs := runtimeGCAfter.PauseTotalNs - runtimeGCBefore.PauseTotalNs
	totalAllocBytes := runtimeGCAfter.TotalAlloc - runtimeGCBefore.TotalAlloc

	fmt.Printf(
		`{"lang":"go","ticks":%d,"elapsed_ms":%.3f,"checksum":%.6f,"transitions":%d,"hits":%d,"triggers_true":%d,"gc_count":%d,"gc_pause_total_ns":%d,"total_alloc_bytes":%d}`+"\n",
		ticks, float64(elapsed.Microseconds())/1000.0, checksum, transitionCount, hitCount, triggerTrueCount,
		numGC, pauseTotalNs, totalAllocBytes,
	)
}
