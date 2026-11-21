// Feed the Brain the recorded stream, mutated to exercise every message field and event type.
import { readFileSync } from "node:fs"
import { gunzipSync } from "node:zlib"
import { Brain } from "../src/brain.ts"
import { GameMap } from "../src/map.ts"
import { makeRng } from "../src/util.ts"

const KIT = "../mechfight-kit"
const lines = gunzipSync(readFileSync(KIT + "/check/stream.jsonl.gz")).toString("utf8").split("\n").filter((l) => l.trim())
const hello = JSON.parse(lines[0])
const map = new GameMap(KIT + "/arena", hello.config.mech)
const cfg = hello.config
const obsList = lines.slice(1).map((l) => JSON.parse(l)).filter((o) => o.type === "obs")
const kinds = ["gun", "mortar", "laser", "lightning"]
let errors = 0
const seen = new Map<string, number>()
let maxMs = 0
for (let round = 0; round < 40; round++) {
  const rnd = makeRng(round + 1)
  const pick = <T,>(a: T[]) => a[Math.floor(rnd() * a.length)]
  const brain = new Brain({ ...hello, seed: round }, map)
  for (const base of obsList) {
    const o = JSON.parse(JSON.stringify(base))
    const [x, y, z] = o.self.pos
    const near = (r: number) => [x + (rnd() - 0.5) * r, y + (rnd() - 0.5) * r, z + (rnd() < 0.2 ? 15 : 0.1)]
    o.t = o.t + round * 10 // cover later match times too
    if (rnd() < 0.5) {
      const k = pick(["mortar", "laser", "lightning"])
      o.self.weapons[1] = { kind: k, cooldown: rnd() * 2, charge: k === "laser" ? rnd() : undefined, stats: { ...cfg.weapons[k] } }
    }
    if (rnd() < 0.15) o.self.weapons[0] = null
    if (rnd() < 0.2) o.self.weapons = [o.self.weapons[1] ?? null, o.self.weapons[0]]
    o.self.cubes = Math.floor(rnd() * 20)
    o.self.hp = rnd() * o.self.max_hp
    o.self.energy = rnd() * 100
    o.self.grounded = rnd() < 0.8
    o.self.in_zone = rnd() < 0.8
    if (rnd() < 0.3) o.offer = { id: 1, expires: o.t + 5, choices: [0, 1, 2].map(() => { const u = pick(cfg.upgrades as any[]); return { id: u.id, name: u.name, desc: u.desc } }) }
    for (let k = 0; k < 3; k++) if (rnd() < 0.3) o.mechs.push({ id: 100 + k, kind: "npc", pos: near(120), vel: [1, 0, 0], aim: [0, 0], hp: 350, max_hp: 350 })
    for (let k = 0; k < 3; k++) if (rnd() < 0.3) o.mechs.push({ id: 5 + k, kind: "mech", pos: near(200), vel: [rnd() * 10, 0, 0], aim: [rnd() * 6 - 3, 0], hp: rnd() * 1400, max_hp: 1400, cubes: Math.floor(rnd() * 5), weapons: ["gun", pick(kinds)], charging: rnd() < 0.3 })
    if (rnd() < 0.3) o.mechs.push({ id: 9, kind: "mech", pos: near(40), vel: [0, 0, 0], aim: [0, 0] }) // minimal fields
    for (let k = 0; k < 3; k++) if (rnd() < 0.3) o.crates.push({ id: 1000 + k, pos: near(120), hp: 300 })
    for (let k = 0; k < 3; k++) if (rnd() < 0.3) o.cubes.push({ id: 5000 + k, pos: near(60) })
    for (let k = 0; k < 3; k++) if (rnd() < 0.3) o.pickups.push({ id: 8000 + k, kind: pick(kinds), pos: near(rnd() < 0.5 ? 6 : 150) })
    if (rnd() < 0.3) o.projectiles.push({ id: 1, kind: pick(["gun", "mortar", "npc"]), pos: near(80), vel: [rnd() * 60 - 30, rnd() * 60 - 30, rnd() * 30] })
    if (rnd() < 0.2) o.beams.push({ by: 5, kind: pick(["laser", "lightning"]), path: [near(50), near(50)], age: 0.1 })
    for (const s of ["step", "boost", "gun", "laser_charge", "lightning", "explosion", "death", "jet", "reload", "crate", "drop", "npc_gun", "mortar", "laser"]) if (rnd() < 0.05) o.sounds.push({ kind: s, bearing: rnd() * 6 - 3, dist: pick(["near", "mid", "far"]), age: 0.05 })
    const evs = [
      { type: "damaged", amount: 50, weapon: pick(["gun", "zone", "npc", "laser"]), bearing: rnd() < 0.2 ? null : 1, by: rnd() < 0.2 ? null : 5, rear: true },
      { type: "hit", target: 5, amount: 45, weapon: "gun", effect: "short" },
      { type: "effect", effect: pick(["cookoff", "short", "concuss"]), by: 5 },
      { type: "drop", id: 20000, cubes: 3 }, { type: "kill", target: 5 }, { type: "pickup", cubes: 2 },
      { type: "upgrade", id: "armor" }, { type: "took", id: 8000, kind: "laser", slot: 1 },
      { type: "dropped", id: 8001, kind: "mortar", slot: 1 }, { type: "take_failed", id: 8002, reason: "range" },
    ]
    for (const e of evs) if (rnd() < 0.1) o.events.push(e)
    if (rnd() < 0.5) o.drops = [{ id: 20000, pos: near(150), radius: 8, lands_in: rnd() < 0.5 ? 0 : 10, capture_s: 4, progress: rnd() * 4, holder: rnd() < 0.5 ? null : pick([5, o.self.id]) }]
    if (rnd() < 0.3) { o.zone.shrink_start = o.t - 5; o.zone.shrink_end = o.t + 20; o.zone.next_radius = pick([0, 25, 70, 140]) }
    try {
      const t0 = performance.now()
      const a = brain.act(o)
      const ms = performance.now() - t0
      if (ms > maxMs) maxMs = ms
      JSON.stringify(a)
      for (const v of [...(a.move ?? []), ...(a.aim ?? [])]) if (!Number.isFinite(v)) throw new Error("non-finite move/aim " + JSON.stringify(a))
    } catch (e: any) {
      errors++
      const key = String(e?.stack ?? e).split("\n").slice(0, 3).join(" | ")
      seen.set(key, (seen.get(key) ?? 0) + 1)
    }
  }
}
console.log(`errors ${errors} of ${40 * obsList.length}; max ms ${maxMs.toFixed(1)}`)
for (const [k, n] of seen) console.log(n, k)
