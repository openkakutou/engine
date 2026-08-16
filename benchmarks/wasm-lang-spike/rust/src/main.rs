// Synthetic per-tick combat-simulation workload -- Rust counterpart of the
// Go version in ../go/main.go. Structurally mirrored 1:1 (same constants,
// same operation order) so the two produce an identical checksum, confirming
// they perform equivalent work. See roadmap .vibe/backlog/008.
use std::env;
use std::time::Instant;

const NUM_FIGHTERS: usize = 2;
const NUM_PROGRAMS: usize = 20;
const NUM_TRANSITIONS: usize = 5;
const NUM_HITBOXES: usize = 3;
const NUM_HURTBOXES: usize = 3;
const NUM_VARS: usize = 8;

#[derive(Clone, Copy, Default)]
struct Vec2 {
    x: f64,
    y: f64,
}

#[derive(Clone, Copy, Default)]
struct Box_ {
    min_x: f64,
    min_y: f64,
    max_x: f64,
    max_y: f64,
}

struct Fighter {
    pos: Vec2,
    vel: Vec2,
    state_id: i64,
    vars: [f64; NUM_VARS],
    hitboxes: [Box_; NUM_HITBOXES],
    hurtboxes: [Box_; NUM_HURTBOXES],
    health: f64,
    grounded: bool,
}

impl Default for Fighter {
    fn default() -> Self {
        Fighter {
            pos: Vec2::default(),
            vel: Vec2::default(),
            state_id: 0,
            vars: [0.0; NUM_VARS],
            hitboxes: [Box_::default(); NUM_HITBOXES],
            hurtboxes: [Box_::default(); NUM_HURTBOXES],
            health: 0.0,
            grounded: false,
        }
    }
}

const OP_PUSH_CONST: u8 = 0;
const OP_PUSH_VAR: u8 = 1;
const OP_ADD: u8 = 2;
#[allow(dead_code)]
const OP_SUB: u8 = 3;
#[allow(dead_code)]
const OP_MUL: u8 = 4;
const OP_GT: u8 = 5;
#[allow(dead_code)]
const OP_LT: u8 = 6;
#[allow(dead_code)]
const OP_AND: u8 = 7;

#[derive(Clone, Copy)]
struct Instr {
    op: u8,
    arg: f64,
}

fn build_programs() -> [Vec<Instr>; NUM_PROGRAMS] {
    std::array::from_fn(|p| {
        let v0 = (p % NUM_VARS) as f64;
        let v1 = ((p + 3) % NUM_VARS) as f64;
        vec![
            Instr { op: OP_PUSH_VAR, arg: v0 },
            Instr { op: OP_PUSH_CONST, arg: (p % 5) as f64 * 0.5 },
            Instr { op: OP_ADD, arg: 0.0 },
            Instr { op: OP_PUSH_VAR, arg: v1 },
            Instr { op: OP_GT, arg: 0.0 },
        ]
    })
}

// Fresh Vec allocation per call, deliberately mirroring the Go version's
// fresh-slice-per-call pattern (same "transient allocation" shape).
fn eval_program(p: &[Instr], f: &Fighter) -> f64 {
    let mut stack: Vec<f64> = Vec::with_capacity(8);
    for ins in p {
        match ins.op {
            OP_PUSH_CONST => stack.push(ins.arg),
            OP_PUSH_VAR => stack.push(f.vars[ins.arg as usize]),
            OP_ADD => {
                let b = stack.pop().unwrap();
                let a = stack.pop().unwrap();
                stack.push(a + b);
            }
            OP_SUB => {
                let b = stack.pop().unwrap();
                let a = stack.pop().unwrap();
                stack.push(a - b);
            }
            OP_MUL => {
                let b = stack.pop().unwrap();
                let a = stack.pop().unwrap();
                stack.push(a * b);
            }
            OP_GT => {
                let b = stack.pop().unwrap();
                let a = stack.pop().unwrap();
                stack.push(if a > b { 1.0 } else { 0.0 });
            }
            OP_LT => {
                let b = stack.pop().unwrap();
                let a = stack.pop().unwrap();
                stack.push(if a < b { 1.0 } else { 0.0 });
            }
            OP_AND => {
                let b = stack.pop().unwrap();
                let a = stack.pop().unwrap();
                stack.push(if a != 0.0 && b != 0.0 { 1.0 } else { 0.0 });
            }
            _ => unreachable!(),
        }
    }
    *stack.last().unwrap()
}

struct TransitionRecord {
    #[allow(dead_code)]
    fighter_idx: usize,
    #[allow(dead_code)]
    from_state: i64,
    #[allow(dead_code)]
    to_state: i64,
}

struct HitRecord {
    #[allow(dead_code)]
    attacker: usize,
    #[allow(dead_code)]
    defender: usize,
    #[allow(dead_code)]
    hb_idx: usize,
    #[allow(dead_code)]
    hurt_idx: usize,
}

fn boxes_overlap(a: &Box_, b: &Box_) -> bool {
    a.min_x < b.max_x && a.max_x > b.min_x && a.min_y < b.max_y && a.max_y > b.min_y
}

fn main() {
    let args: Vec<String> = env::args().collect();
    let ticks: i64 = args
        .get(1)
        .and_then(|s| s.parse().ok())
        .unwrap_or(2_000_000);

    let programs = build_programs();

    let mut fighters: [Fighter; NUM_FIGHTERS] = std::array::from_fn(|_| Fighter::default());
    for f in fighters.iter_mut() {
        f.health = 1000.0;
        f.grounded = true;
        for h in f.hitboxes.iter_mut() {
            *h = Box_ { min_x: 10.0, min_y: 0.0, max_x: 20.0, max_y: 10.0 };
        }
        for h in f.hurtboxes.iter_mut() {
            *h = Box_ { min_x: 0.0, min_y: 0.0, max_x: 15.0, max_y: 20.0 };
        }
    }
    fighters[1].pos.x = 100.0;

    let start = Instant::now();

    let mut checksum: f64 = 0.0;
    let mut transition_count: i64 = 0;
    let mut hit_count: i64 = 0;
    let mut trigger_true_count: i64 = 0;

    const GRAVITY: f64 = -0.5;
    const GROUND_Y: f64 = 0.0;
    const STAGE_MIN_X: f64 = -200.0;
    const STAGE_MAX_X: f64 = 200.0;

    for t in 0..ticks {
        for fi in 0..NUM_FIGHTERS {
            let base = ((t * 7 + fi as i64 * 13) % 97) as f64 * 0.1;

            // 1. Trigger evaluation
            for p in 0..NUM_PROGRAMS {
                let v = eval_program(&programs[p], &fighters[fi]);
                if v != 0.0 {
                    trigger_true_count += 1;
                }
            }

            // 2. State transitions
            let mut transitions: Vec<TransitionRecord> = Vec::with_capacity(2);
            {
                let f = &mut fighters[fi];
                for c in 0..NUM_TRANSITIONS {
                    let cond = f.vars[c % NUM_VARS] + base;
                    if cond % 3.0 < 1.0 {
                        let from = f.state_id;
                        f.state_id = (f.state_id + c as i64 + 1) % 40;
                        f.vars[c % NUM_VARS] += 0.01;
                        transitions.push(TransitionRecord {
                            fighter_idx: fi,
                            from_state: from,
                            to_state: f.state_id,
                        });
                    }
                }
            }
            transition_count += transitions.len() as i64;

            // 3. Physics
            let f = &mut fighters[fi];
            if !f.grounded {
                f.vel.y += GRAVITY;
            }
            f.pos.x += f.vel.x;
            f.pos.y += f.vel.y;
            if f.pos.y <= GROUND_Y {
                f.pos.y = GROUND_Y;
                f.vel.y = 0.0;
                f.grounded = true;
            } else {
                f.grounded = false;
            }
            if f.pos.x < STAGE_MIN_X {
                f.pos.x = STAGE_MIN_X;
            }
            if f.pos.x > STAGE_MAX_X {
                f.pos.x = STAGE_MAX_X;
            }
        }

        // 4. Hit detection
        let mut hits: Vec<HitRecord> = Vec::with_capacity(2);
        for hb in 0..NUM_HITBOXES {
            for hu in 0..NUM_HURTBOXES {
                if boxes_overlap(&fighters[0].hitboxes[hb], &fighters[1].hurtboxes[hu]) {
                    hits.push(HitRecord { attacker: 0, defender: 1, hb_idx: hb, hurt_idx: hu });
                    fighters[1].health -= 1.0;
                }
                if boxes_overlap(&fighters[1].hitboxes[hb], &fighters[0].hurtboxes[hu]) {
                    hits.push(HitRecord { attacker: 1, defender: 0, hb_idx: hb, hurt_idx: hu });
                    fighters[0].health -= 1.0;
                }
            }
        }
        hit_count += hits.len() as i64;

        let shift = (t % 40) as f64 - 20.0;
        fighters[0].hitboxes[0].min_x = 10.0 + shift;
        fighters[0].hitboxes[0].max_x = 20.0 + shift;
    }

    let elapsed = start.elapsed();

    for f in fighters.iter() {
        checksum += f.pos.x + f.pos.y + f.health + f.state_id as f64;
        for v in f.vars.iter() {
            checksum += v;
        }
    }

    println!(
        "{{\"lang\":\"rust\",\"ticks\":{},\"elapsed_ms\":{:.3},\"checksum\":{:.6},\"transitions\":{},\"hits\":{},\"triggers_true\":{},\"gc_count\":0,\"gc_pause_total_ns\":0,\"total_alloc_bytes\":null}}",
        ticks,
        elapsed.as_micros() as f64 / 1000.0,
        checksum,
        transition_count,
        hit_count,
        trigger_true_count,
    );
}
