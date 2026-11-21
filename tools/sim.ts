// Small local simulator (dev only, not used in matches). Runs our Brain against scripted hunters
// on the real map grids to catch navigation dead-ends, aim bugs and timing regressions.
//   node tools/sim.ts [seeds=10] [hunterSkill=0.6] [giveWeapon=none|lightning|laser|mortar]
import { readFileSync } from "node:fs"
import { gunzipSync } from "node:zlib"
import { Brain, P } from "../src/brain.ts"
import { GameMap } from "../src/map.ts"
import { Navigator } from "../src/nav.ts"
import { leadStraight } from "../src/aim.ts"
import { angDiff, makeRng, wrapAngle } from "../src/util.ts"

const KIT = "../mechfight-kit"
const helloLine = gunzipSync(readFileSync(KIT + "/check/stream.jsonl.gz")).toString("utf8").split("\n")[0]
const HELLO = JSON.parse(helloLine)
HELLO.map.dir = KIT + "/arena"
const cfg = HELLO.config
const map = new GameMap(HELLO.map.dir, cfg.mech)
const DT = 1 / cfg.match.tick_hz

type Npc = { id: number; home: number[]; pos: number[]; hp: number; cd: number; aggroT: number }
type Item = { id: number; kind: string; pos: number[]; crate?: boolean; hp?: number }
type Shot = { pos: number[]; vel: number[]; owner: number; dmg: number; life: number }
type Mech = {
  id: number; pos: number[]; vel: number[]; aim: number[]; cmdAim: number[]; hp: number; maxHp: number; energy: number
  weapons: any[]; alive: boolean; active: boolean; lastDmg: number; lastSpend: number; cubes: number
  nav?: Navigator; strafe: number; flip: number; skill: number; act: any; seen: Map<number, number>
  events: any[]; diedAt: number; grounded: boolean
  hitByUs: number; wgoal: number[] | null; wgoalT: number
}

function gunW() {
  return { kind: "gun", cooldown: 0, ammo: cfg.weapons.gun.mag, reload: 0, stats: { ...cfg.weapons.gun } }
}

function zoneInfo(t: number) {
  const Z = cfg.zone
  let c = Z.start_center.slice(), r = Z.start_radius
  let T = 0
  for (const ph of Z.phases) {
    const ss = T + ph.hold_s, se = ss + ph.shrink_s
    // next centre: deterministic shrink toward origin-ish for the sim
    const nc = [c[0] * 0.6 + (-20) * 0.4, c[1] * 0.6 + 0 * 0.4]
    if (t < se) {
      return { center: c, radius: r, next_center: nc, next_radius: ph.radius, shrink_start: ss, shrink_end: se, dps: ph.dps }
    }
    c = nc
    r = ph.radius
    T = se
  }
  return { center: c, radius: 0, next_center: c, next_radius: 0, shrink_start: T, shrink_end: T, dps: 300 }
}
function zoneAt(z: any, t: number) {
  if (t <= z.shrink_start) return [z.center[0], z.center[1], z.radius]
  if (t >= z.shrink_end) return [z.next_center[0], z.next_center[1], z.next_radius]
  const f = (t - z.shrink_start) / (z.shrink_end - z.shrink_start)
  return [z.center[0] + (z.next_center[0] - z.center[0]) * f, z.center[1] + (z.next_center[1] - z.center[1]) * f, z.radius + (z.next_radius - z.radius) * f]
}

let SR: () => number = Math.random
// TIERS=1: every house bot is on the map from the start (visible, hittable). Hunters are activated
// weakest first (ids 1-4 easy, 5-8 medium, 9-12 hard, 13-15 expert); the rest wander inside the zone.
// NH=passive: idle bots fight back only once we hit them. NH=aggressive: they also shoot when they see us.
const TIERS = process.env.TIERS === "1"
const NH = process.env.NH ?? "passive"
const TIER_SKILL = (process.env.SKILLS ?? "0.2,0.5,0.75,0.95").split(",").map(Number)
const AIMN = Number(process.env.AIMN ?? 0.05)
const present = (m: { active: boolean }) => TIERS || m.active
function runMatch(seed: number, skill: number, give: string, nHunters: number, armed: boolean) {
  SR = makeRng(seed * 104729 + 7)
  const rnd = makeRng(seed * 7919 + 1)
  const spawns: number[][] = []
  const z0 = zoneInfo(0)
  while (spawns.length < nHunters + 1) {
    const a = rnd() * Math.PI * 2, r = Math.sqrt(rnd()) * z0.radius * 0.75
    const x = z0.center[0] + Math.cos(a) * r, y = z0.center[1] + Math.sin(a) * r
    const n = map.nodeOf(x, y)
    if (!map.isMain(n)) continue
    if (spawns.some((s) => Math.hypot(s[0] - x, s[1] - y) < 130)) continue
    spawns.push([map.nodeX(n), map.nodeY(n), map.nodeZ(n)])
  }
  const mechs: Mech[] = spawns.map((s, i) => ({
    id: i, pos: s.slice(), vel: [0, 0, 0], aim: [0, 0], cmdAim: [0, 0], hp: cfg.mech.hp, maxHp: cfg.mech.hp, energy: 100,
    weapons: [gunW(), null], alive: true, active: i === 0, lastDmg: -99, lastSpend: -99, cubes: 0,
    strafe: 1, flip: 0, skill: TIERS && i > 0 ? TIER_SKILL[Math.min(3, Math.floor((i - 1) / 4))] : skill, act: {}, seen: new Map(), events: [],
    diedAt: Infinity, grounded: true, hitByUs: -99, wgoal: null, wgoalT: 0,
  }))
  if (give !== "none") mechs[0].weapons[1] = { kind: give, cooldown: 0, charge: 0, stats: { ...cfg.weapons[give] } }
  for (let i = 1; i < mechs.length; i++) mechs[i].nav = new Navigator(map)
  const KINDS = ["laser", "lightning"] // no mortar physics in this sim
  const mkW = (k: string) => ({ kind: k, cooldown: 0, charge: 0, stats: { ...cfg.weapons[k] } })
  if (armed)
    for (let i = 1; i < mechs.length; i++) {
      const r = rnd()
      if (r < 0.35) mechs[i].weapons[1] = mkW("laser")
      else if (r < 0.7) mechs[i].weapons[1] = mkW("lightning")
    }
  const items: Item[] = []
  let nextId = 8000
  const randNode = () => {
    for (;;) {
      const x = map.bminX + rnd() * (map.bmaxX - map.bminX), y = map.bminY + rnd() * (map.bmaxY - map.bminY)
      const n = map.nodeOf(x, y)
      if (map.isMain(n)) return [map.nodeX(n), map.nodeY(n), map.nodeZ(n)]
    }
  }
  for (let k = 0; k < 30; k++) items.push({ id: nextId++, kind: KINDS[Math.floor(rnd() * KINDS.length)], pos: randNode() })
  for (let k = 0; k < 27; k++) items.push({ id: 1000 + k, kind: "crate", pos: randNode(), crate: true, hp: 300 })
  const npcs: Npc[] = []
  for (let k = 0; k < cfg.npc.count; k++) {
    const h = randNode()
    npcs.push({ id: 100 + k, home: h.slice(), pos: h.slice(), hp: cfg.npc.hp, cd: 0, aggroT: -99 })
  }
  const cubes: Item[] = []
  let cubeId = 5000
  const dropCubes = (pos: number[], n: number) => {
    for (let k = 0; k < n; k++) cubes.push({ id: cubeId++, kind: "cube", pos: [pos[0] + (rnd() - 0.5) * 3, pos[1] + (rnd() - 0.5) * 3, pos[2]] })
  }
  let gotCubes = 0
  let dronesKilled = 0
  const UPG = ["armor", "plating", "nanites", "gun_rate", "gun_damage", "sensors", "servos", "gyros"]
  const thresholds: number[] = cfg.cubes.upgrade_thresholds
  let offers = 0
  let pendingOffer: any = null
  const applied: Record<string, number> = {}
  const brain = new Brain({ ...HELLO, slot: 0, seed: seed }, map)
  const shots: Shot[] = []
  let kills = 0
  let dmgIn = 0, dmgOut = 0
  const byW: Record<string, number> = {}
  let laserShots = 0
  let stuckTicks = 0
  let maxMs = 0
  const tms: number[] = []
  let lastPos = mechs[0].pos.slice()
  let still = 0
  const dur = cfg.match.duration_s
  let activated = 1
  const world: any = { npcs: [], cubes: [], offer: null }
  const pickedAt: number[] = []
  let cratesBroken = 0
  const fired = [0, 0], hits = [0, 0]
  const modes: Record<string, number> = {}

  const damage = (vi: Mech, amt: number, by: Mech | null, weapon: string, fromX: number, fromY: number, t: number) => {
    const bearing = Math.atan2(fromY - vi.pos[1], fromX - vi.pos[0])
    const rear = by && Math.abs(angDiff(bearing, vi.aim[0])) > Math.PI / 2
    if (rear) amt *= cfg.mech.rear_mult
    if (vi.id === 0) amt *= Math.max(0.5, 1 - 0.08 * (applied.armor ?? 0))
    vi.hp -= amt
    vi.lastDmg = t
    if (by) by.lastDmg = t
    vi.events.push({ type: "damaged", amount: amt, weapon, bearing: by ? bearing : null, by: by ? by.id : null, rear: !!rear })
    if (by) by.events.push({ type: "hit", target: vi.id, amount: amt, weapon })
    if (vi.id === 0) { dmgIn += amt; byW[weapon] = (byW[weapon] ?? 0) + Math.round(amt) }
    if (by && by.id === 0) { dmgOut += amt; vi.hitByUs = t }
    if (vi.hp <= 0 && vi.alive) {
      vi.alive = false
      vi.diedAt = t
      dropCubes(vi.pos, vi.cubes + cfg.cubes.death_drop_extra)
      if (by && by.id === 0) {
        kills++
        by.events.push({ type: "kill", target: vi.id })
      }
    }
  }

  for (let tick = 0; tick < dur * cfg.match.tick_hz; tick++) {
    const t = tick * DT
    const me = mechs[0]
    if (!me.alive) break
    const zone = zoneInfo(t)
    // activation: one early, more as they die, all late
    const deadHunters = mechs.filter((m) => m.id > 0 && !m.alive).length
    const want = Math.min(nHunters, 1 + deadHunters + (t > 150 ? 1 : 0) + (t > 240 ? 2 : 0) + (t > 330 ? 99 : 0))
    while (activated < want + 1 && activated < mechs.length) mechs[activated++].active = true

    if (tick % 2 === 0) {
      // ---- decisions
      for (const m of mechs) {
        if (!m.alive) continue
        if (m.id === 0) {
          const obs = buildObs(m, mechs, shots, zone, t, tick, items, world)
          const t0 = performance.now()
          m.act = brain.act(obs)
          modes[brain.mode] = (modes[brain.mode] ?? 0) + 1
          if (process.env.TRACE && tick % 10 === 0) {
            const en = mechs.filter((q) => q.id > 0 && q.alive && q.active).map((q) => `${q.id}:${q.weapons[1]?.kind?.[0] ?? "g"}:${Math.round(Math.hypot(q.pos[0] - m.pos[0], q.pos[1] - m.pos[1]))}m:${Math.round(q.hp)}`).join(" ")
            console.log(`t${t.toFixed(0)} ${brain.mode} z${m.pos[2].toFixed(1)} e${Math.round(m.energy)} hp${Math.round(m.hp)} w:${m.weapons[1]?.kind ?? "-"} fire:${JSON.stringify(m.act.fire)} | ${en}`)
          }
          const ms = performance.now() - t0
          tms.push(ms)
          if (ms > maxMs) maxMs = ms
          m.events = []
          const tk = m.act.take
          if (tk != null) {
            const ii = items.findIndex((it) => it.id === tk && !it.crate)
            const it = items[ii]
            if (it && Math.hypot(it.pos[0] - m.pos[0], it.pos[1] - m.pos[1]) < cfg.loot.pickup_radius && !m.weapons.some((w) => w && w.kind === it.kind)) {
              const slot = m.weapons[1] == null ? 1 : m.act.drop != null ? m.act.drop : -1
              if (slot >= 0) {
                if (m.weapons[slot]) items.push({ id: nextId++, kind: m.weapons[slot].kind, pos: m.pos.slice() })
                const w: any = mkW(it.kind)
                w.cooldown = Math.max(cfg.loot.equip_s, w.stats.cooldown ?? 0)
                m.weapons[slot] = w
                items.splice(ii, 1)
                m.events.push({ type: "took", id: it.id, kind: it.kind, slot })
                pickedAt.push(Math.round(t))
              } else m.events.push({ type: "take_failed", id: tk, reason: "full" })
            } else m.events.push({ type: "take_failed", id: tk, reason: "range" })
          }
        } else if (m.active) m.act = hunterAct(m, me, t, zone)
        else if (TIERS) m.act = idleAct(m, me, t, zone)
        else m.act = {}
      }
    }
    // ---- physics
    for (const m of mechs) {
      if (!m.alive) continue
      const a = m.act
      if (a.aim) m.cmdAim = a.aim
      const tr = cfg.mech.turn_rate * DT, pr = cfg.mech.pitch_rate * DT
      const dy = angDiff(m.cmdAim[0], m.aim[0])
      m.aim[0] = wrapAngle(m.aim[0] + Math.max(-tr, Math.min(tr, dy)))
      m.aim[1] += Math.max(-pr, Math.min(pr, m.cmdAim[1] - m.aim[1]))
      let mv = a.move ?? [0, 0]
      const ml = Math.hypot(mv[0], mv[1])
      if (ml > 1) mv = [mv[0] / ml, mv[1] / ml]
      const charging = m.weapons.some((w) => w && w.kind === "laser" && w.charge > 0)
      let sp = a.boost && m.energy > 0 && ml > 0.1 && m.grounded ? cfg.mech.boost_speed : cfg.mech.walk_speed
      if (charging) sp *= 0.5
      if (a.boost && ml > 0.1 && m.grounded) {
        m.energy -= cfg.mech.boost_drain * DT
        m.lastSpend = t
      }
      const acc = cfg.mech.ground_accel * DT
      for (let k = 0; k < 2; k++) {
        const dv = mv[k] * sp - m.vel[k]
        m.vel[k] += Math.max(-acc, Math.min(acc, dv))
      }
      const nx = m.pos[0] + m.vel[0] * DT, ny = m.pos[1] + m.vel[1] * DT
      const okAt = (x: number, y: number) => {
        const c = map.idx(x, y)
        if (map.surf[c] - m.pos[2] > cfg.mech.step_up + 0.05) return false // wall (feet below its top)
        if (map.head[c] < m.pos[2] + 9) return false // overhang
        return map.walk1[c] || map.surf[c] < m.pos[2] - 2 // dropping off an edge is fine
      }
      if (okAt(nx, ny)) {
        m.pos[0] = nx
        m.pos[1] = ny
      } else if (okAt(nx, m.pos[1])) {
        m.pos[0] = nx
        m.vel[1] = 0
      } else if (okAt(m.pos[0], ny)) {
        m.pos[1] = ny
        m.vel[0] = 0
      } else {
        m.vel[0] = m.vel[1] = 0
      }
      const floor = map.surfAt(m.pos[0], m.pos[1])
      if (m.grounded && a.jump) {
        m.vel[2] = cfg.mech.jump_speed
        m.grounded = false
      }
      if (!m.grounded) {
        if (a.boost && m.energy > 0) {
          m.vel[2] = Math.min(cfg.mech.max_rise_speed, m.vel[2] + cfg.mech.jet_accel * DT)
          m.energy -= cfg.mech.jet_drain * DT
          m.lastSpend = t
        }
        m.vel[2] -= cfg.mech.gravity * DT
        m.pos[2] += m.vel[2] * DT
        if (m.pos[2] <= floor) {
          m.pos[2] = floor
          m.vel[2] = 0
          m.grounded = true
        }
      } else if (floor < m.pos[2] - 1.5) {
        m.grounded = false // walked off an edge
        m.vel[2] = 0
      } else m.pos[2] = floor
      if (t - m.lastSpend > cfg.mech.energy_regen_delay) m.energy = Math.min(100, m.energy + cfg.mech.energy_regen * DT)
      // regen + zone
      const [zx, zy, zr] = zoneAt(zone, t)
      const inZone = Math.hypot(m.pos[0] - zx, m.pos[1] - zy) <= zr
      if (!inZone) {
        damage(m, zone.dps * DT, null, "zone", m.pos[0], m.pos[1], t)
      } else if (t - m.lastDmg > cfg.mech.regen_delay) m.hp = Math.min(m.maxHp, m.hp + cfg.mech.regen_per_s * DT)
      // weapons
      const fire = a.fire ?? [false, false]
      for (let s = 0; s < 2; s++) {
        const w = m.weapons[s]
        if (!w) continue
        w.cooldown = Math.max(0, w.cooldown - DT)
        if (w.kind === "gun") {
          if (w.reload > 0) {
            w.reload -= DT
            if (w.reload <= 0) {
              w.reload = 0
              w.ammo = w.stats.mag
            }
          }
          if (a.reload && w.reload === 0 && w.ammo < w.stats.mag) w.reload = w.stats.reload
          if (fire[s] && w.cooldown <= 0 && w.ammo > 0 && w.reload === 0) {
            w.cooldown = w.stats.cooldown
            w.ammo--
            if (w.ammo === 0) w.reload = w.stats.reload
            const sp = (w.stats.spread_deg * Math.PI) / 180
            const yaw = m.aim[0] + gauss(rnd) * sp, pit = m.aim[1] + gauss(rnd) * sp
            const d = [Math.cos(pit) * Math.cos(yaw), Math.cos(pit) * Math.sin(yaw), Math.sin(pit)]
            const o = [m.pos[0] + d[0] * 3, m.pos[1] + d[1] * 3, m.pos[2] + 7 + d[2] * 3]
            fired[m.id === 0 ? 0 : 1]++
            shots.push({ pos: o, vel: d.map((v) => v * w.stats.speed), owner: m.id, dmg: w.stats.damage * (1 + 0.04 * Math.min(m.cubes, 10)) * (m.id === 0 ? Math.pow(1.15, applied.gun_damage ?? 0) : 1), life: w.stats.range / w.stats.speed })
          }
        } else if (w.kind === "lightning") {
          if (fire[s] && w.cooldown <= 0 && m.energy >= w.stats.energy) {
            w.cooldown = w.stats.cooldown
            m.energy -= w.stats.energy
            m.lastSpend = t
            let best: Mech | null = null, bd = Infinity
            for (const o of mechs) {
              if (o === m || !o.alive || !present(o)) continue
              if (m.id !== 0 && o.id !== 0) continue // house bots never hit each other
              const d = Math.hypot(o.pos[0] - m.pos[0], o.pos[1] - m.pos[1])
              const b = Math.atan2(o.pos[1] - m.pos[1], o.pos[0] - m.pos[0])
              if (d > w.stats.range || Math.abs(angDiff(b, m.aim[0])) > (w.stats.cone_deg * Math.PI) / 360) continue
              if (!map.clear(m.pos[0], m.pos[1], m.pos[2] + 7, o.pos[0], o.pos[1], o.pos[2] + 4.5, 3, 2)) continue
              if (d < bd) { bd = d; best = o }
            }
            if (best) {
              damage(best, w.stats.damage * (1 + 0.04 * Math.min(m.cubes, 10)), m, "lightning", m.pos[0], m.pos[1], t)
              // short: a laser it hits loses its charge and can't charge for short_s
              const lw = best.weapons.find((q: any) => q && q.kind === "laser")
              if (lw) {
                lw.charge = 0
                lw.shortUntil = t + (w.stats.short_s ?? 2)
                if (best.id === 0) best.events.push({ type: "effect", effect: "short", by: m.id })
              }
            }
          }
        } else if (w.kind === "laser") {
          if (fire[s] && w.cooldown <= 0 && !(t < (w.shortUntil ?? -1))) w.charge = Math.min(w.stats.charge + w.stats.hold_s, w.charge + DT)
          const release = (!fire[s] && w.charge >= w.stats.min_charge) || w.charge >= w.stats.charge + w.stats.hold_s
          if (release && w.charge > 0) {
            if (m.id !== 0) laserShots++
            const frac = Math.min(1, w.charge / w.stats.charge)
            const dmg = w.stats.damage * (w.stats.min_frac + (1 - w.stats.min_frac) * frac)
            w.charge = 0
            w.cooldown = w.stats.cooldown
            // hitscan along aim
            const d = [Math.cos(m.aim[1]) * Math.cos(m.aim[0]), Math.cos(m.aim[1]) * Math.sin(m.aim[0]), Math.sin(m.aim[1])]
            const o = [m.pos[0], m.pos[1], m.pos[2] + 7]
            let hitM: Mech | null = null, ht = Infinity
            for (const q of mechs) {
              if (q === m || !q.alive || !present(q)) continue
              if (m.id !== 0 && q.id !== 0) continue
              const tt = rayCyl(o, d, q.pos, 2.5, 9)
              if (tt !== null && tt < ht && tt < w.stats.range) { ht = tt; hitM = q }
            }
            if (hitM && map.clear(o[0], o[1], o[2], o[0] + d[0] * ht, o[1] + d[1] * ht, o[2] + d[2] * ht, 3, 1)) damage(hitM, dmg * (1 + 0.04 * Math.min(m.cubes, 10)), m, "laser", m.pos[0], m.pos[1], t)
          }
        }
      }
    }
    // shots
    for (let i = shots.length - 1; i >= 0; i--) {
      const s = shots[i]
      let dead = false
      const sub = 4
      for (let k = 0; k < sub && !dead; k++) {
        const ox = s.pos[0], oy = s.pos[1], oz = s.pos[2]
        s.pos[0] += (s.vel[0] * DT) / sub
        s.pos[1] += (s.vel[1] * DT) / sub
        s.pos[2] += (s.vel[2] * DT) / sub
        if (!map.clear(ox, oy, oz, s.pos[0], s.pos[1], s.pos[2], 0, 0)) { dead = true; break }
        for (let ci = items.length - 1; ci >= 0 && !dead; ci--) {
          const c = items[ci]
          if (!c.crate) continue
          if (Math.hypot(s.pos[0] - c.pos[0], s.pos[1] - c.pos[1]) < 1.7 && s.pos[2] > c.pos[2] - 0.2 && s.pos[2] < c.pos[2] + 2.5) {
            dead = true
            c.hp! -= s.dmg
            if (s.owner === 0) mechs[0].events.push({ type: "hit", target: c.id, amount: s.dmg, weapon: "gun" })
            if (c.hp! <= 0) {
              items.splice(ci, 1)
              if (s.owner === 0) cratesBroken++
              if (rnd() < cfg.loot.crate_weapon_chance) items.push({ id: nextId++, kind: KINDS[Math.floor(rnd() * KINDS.length)], pos: c.pos.slice() })
            }
          }
        }
        for (let ni = npcs.length - 1; ni >= 0 && !dead; ni--) {
          const q = npcs[ni]
          if (Math.hypot(s.pos[0] - q.pos[0], s.pos[1] - q.pos[1]) < 1.8 + 0.3 && s.pos[2] > q.pos[2] - 0.2 && s.pos[2] < q.pos[2] + 3.6) {
            dead = true
            q.hp -= s.dmg
            q.aggroT = t
            if (q.hp <= 0) {
              npcs.splice(ni, 1)
              dropCubes(q.pos, cfg.npc.cubes)
              if (s.owner === 0) dronesKilled++
            }
          }
        }
        if (dead) break
        for (const m of mechs) {
          if (!m.alive || m.id === s.owner || !present(m)) continue
          if (s.owner !== 0 && m.id !== 0) continue
          if (Math.hypot(s.pos[0] - m.pos[0], s.pos[1] - m.pos[1]) < 2.5 + 0.3 && s.pos[2] > m.pos[2] && s.pos[2] < m.pos[2] + 9) {
            hits[s.owner === 0 ? 0 : 1]++
            damage(m, s.dmg, mechs[s.owner], "gun", s.pos[0] - s.vel[0] * 0.05, s.pos[1] - s.vel[1] * 0.05, t)
            dead = true
            break
          }
        }
      }
      s.life -= DT
      if (dead || s.life <= 0) shots.splice(i, 1)
    }
    // drones: shoot our bot if close (or attacked recently), hit 50% of the time
    for (const q of npcs) {
      q.cd -= DT
      const d = Math.hypot(me.pos[0] - q.pos[0], me.pos[1] - q.pos[1])
      const aggro = (t - q.aggroT < 10 && d < cfg.npc.deaggro_range) || d < cfg.npc.aggro_range
      if (aggro && q.cd <= 0 && map.clear(q.pos[0], q.pos[1], q.pos[2] + 2, me.pos[0], me.pos[1], me.pos[2] + 4.5, 2, 2)) {
        q.cd = cfg.npc.cooldown
        if (rnd() < 0.5) damage(me, cfg.npc.damage, null, "npc", q.pos[0], q.pos[1], t)
      }
    }
    for (let ci = cubes.length - 1; ci >= 0; ci--) {
      const c = cubes[ci]
      if (Math.hypot(c.pos[0] - me.pos[0], c.pos[1] - me.pos[1]) < cfg.cubes.pickup_radius && Math.abs(c.pos[2] - me.pos[2]) < 4) {
        cubes.splice(ci, 1)
        me.cubes++
        gotCubes++
        if (me.cubes <= cfg.cubes.stat_cap) {
          const add = me.maxHp * cfg.cubes.hp_per_cube
          me.maxHp += add
          me.hp += add
        }
        me.events.push({ type: "pickup", cubes: me.cubes })
        if (thresholds.includes(me.cubes)) offers++
      }
    }
    if (!pendingOffer && offers > 0) {
      const ch = UPG.filter((u) => (applied[u] ?? 0) < 3).sort(() => rnd() - 0.5).slice(0, 3)
      pendingOffer = { id: offers, expires: t + 5, choices: ch.map((c) => ({ id: c, name: c, desc: c })) }
    }
    if (pendingOffer && me.act.pick != null) {
      const u = pendingOffer.choices[me.act.pick]?.id
      if (u) {
        applied[u] = (applied[u] ?? 0) + 1
        if (u === "plating") { me.maxHp *= 1.15; me.hp *= 1.15 }
        if (u === "gun_rate" && me.weapons[0]) me.weapons[0].stats.cooldown *= 0.87
      }
      pendingOffer = null
      offers--
      me.act.pick = undefined
    }
    world.npcs = npcs
    world.cubes = cubes
    world.offer = pendingOffer
    // stuck metric for our bot
    if (tick % 20 === 0) {
      const mv = mechs[0].act.move ?? [0, 0]
      const moved = Math.hypot(me.pos[0] - lastPos[0], me.pos[1] - lastPos[1])
      if (Math.hypot(mv[0], mv[1]) > 0.5 && moved < 1) still++
      else still = 0
      if (still >= 3) stuckTicks++
      lastPos = me.pos.slice()
    }
  }
  const me = mechs[0]
  const surv = me.alive ? dur : me.diedAt
  // placement: hunters that died before us
  const before = mechs.filter((m) => m.id > 0 && m.diedAt < me.diedAt).length
  const place = 16 - before
  tms.sort((a, b) => a - b)
  return { seed, surv, kills, place, pts: 16 - place, dmgIn: Math.round(dmgIn), dmgOut: Math.round(dmgOut), stuck: stuckTicks, p99: +(tms[Math.floor(tms.length * 0.99)] ?? 0).toFixed(2), maxMs: +maxMs.toFixed(1), cubes: gotCubes, drones: dronesKilled, upg: applied, byW, laserShots, st: brain.stats, missD: brain.missD, picked: pickedAt, crates: cratesBroken, acc: [+(hits[0] / Math.max(1, fired[0])).toFixed(2), +(hits[1] / Math.max(1, fired[1])).toFixed(2)], fired, modes }
}

function visList(m: any, list: any[], hz: number): any[] {
  return list.filter((it) => {
    const d = Math.hypot(it.pos[0] - m.pos[0], it.pos[1] - m.pos[1])
    const b = Math.atan2(it.pos[1] - m.pos[1], it.pos[0] - m.pos[0])
    const inFov = d < cfg.vision.range && Math.abs(angDiff(b, m.aim[0])) < (cfg.vision.fov_deg * Math.PI) / 360
    return (d < cfg.vision.near_range || inFov) && map.clear(m.pos[0], m.pos[1], m.pos[2] + 8, it.pos[0], it.pos[1], it.pos[2] + hz, 0.5, 0.5)
  })
}

function gauss(r: () => number) {
  return Math.sqrt(-2 * Math.log(r() + 1e-12)) * Math.cos(2 * Math.PI * r())
}

function rayCyl(o: number[], d: number[], p: number[], r: number, h: number): number | null {
  const ox = o[0] - p[0], oy = o[1] - p[1]
  const a = d[0] * d[0] + d[1] * d[1]
  const b = 2 * (ox * d[0] + oy * d[1])
  const c = ox * ox + oy * oy - r * r
  const disc = b * b - 4 * a * c
  if (disc < 0 || a < 1e-9) return null
  const t = (-b - Math.sqrt(disc)) / (2 * a)
  if (t < 0) return null
  const z = o[2] + d[2] * t
  return z >= p[2] && z <= p[2] + h ? t : null
}

function hunterAct(m: Mech, me: Mech, t: number, zone: any) {
  const [x, y] = m.pos
  const d = Math.hypot(me.pos[0] - x, me.pos[1] - y)
  const los = d < 200 && map.clear(x, y, m.pos[2] + 7, me.pos[0], me.pos[1], me.pos[2] + 4.5, 3, 2)
  const act: any = {}
  const kiter = m.weapons[1]?.kind === "laser"
  if (los && d < (kiter ? 220 : 150)) {
    // strafe and shoot with (imperfect) lead
    if (t > m.flip) {
      m.strafe = -m.strafe
      m.flip = t + 0.6 + SR() * 1.2
    }
    const b = Math.atan2(me.pos[1] - y, me.pos[0] - x)
    const rad = kiter ? (d > 150 ? 0.7 : d < 100 ? -0.8 : 0) : d > 90 ? 0.7 : d < 40 ? -0.5 : 0
    act.move = [-Math.sin(b) * m.strafe + Math.cos(b) * rad, Math.cos(b) * m.strafe + Math.sin(b) * rad]
    const v = [me.vel[0] * m.skill, me.vel[1] * m.skill, 0]
    const sol = leadStraight(x, y, m.pos[2] + 7, me.pos[0], me.pos[1], me.pos[2] + 4.5, v[0], v[1], 0, 252, 3, me.pos[2] + 2, 22)
    const nz = TIERS ? gauss(SR) * AIMN * (1 - m.skill) : (SR() - 0.5) * 0.02 * (1 - m.skill)
    act.aim = [sol.yaw + nz, sol.pitch]
    const err = Math.abs(angDiff(m.aim[0], sol.yaw + nz))
    act.fire = [err < 0.05, false]
    const w1 = m.weapons[1]
    if (w1 && w1.kind === "lightning") {
      act.fire[1] = d < w1.stats.range - 2 && err < 0.2
      if (d > 35) act.move = [Math.cos(b) * 0.9 - Math.sin(b) * m.strafe * 0.4, Math.sin(b) * 0.9 + Math.cos(b) * m.strafe * 0.4]
    } else if (w1 && w1.kind === "laser") {
      const full = w1.charge >= w1.stats.charge
      const yawTo = Math.atan2(me.pos[1] - y, me.pos[0] - x)
      const lz = TIERS ? nz : 0
      act.aim = [yawTo + lz, Math.atan2(me.pos[2] + 4.5 - m.pos[2] - 7, d)]
      const e2 = Math.abs(angDiff(m.aim[0], yawTo + lz))
      act.fire[1] = w1.cooldown <= 0 && !(full && e2 < Math.atan2(2, d) * (0.5 + m.skill))
    }
  } else {
    m.nav!.setGoal(me.pos[0], me.pos[1], t, x, y)
    act.move = m.nav!.steer(x, y)
    act.aim = [Math.atan2(me.pos[1] - y, me.pos[0] - x), 0]
    act.boost = d > 150
  }
  if (me.pos[2] > m.pos[2] + 4 && d < 60 && !los) {
    const b2 = Math.atan2(me.pos[1] - y, me.pos[0] - x)
    act.move = [Math.cos(b2), Math.sin(b2)]
    act.jump = m.grounded
    act.boost = !m.grounded && m.energy > 1
  }
  // stay in zone
  const [zx, zy, zr] = zoneAt(zone, t + 10)
  if (Math.hypot(x - zx, y - zy) > zr - 10) {
    m.nav!.setGoal(zone.next_center[0], zone.next_center[1], t, x, y)
    act.move = m.nav!.steer(x, y)
  }
  return act
}

/** A house bot that is not hunting yet: wanders inside the zone; fights back once we hit it
 * (and with NH=aggressive, shoots whenever it sees us, without chasing). */
function idleAct(m: Mech, me: Mech, t: number, zone: any) {
  const [x, y] = m.pos
  const d = Math.hypot(me.pos[0] - x, me.pos[1] - y)
  const hostile = NH !== "none" && t - m.hitByUs < 20
  if (hostile || NH === "aggressive") {
    const los = d < 160 && map.clear(x, y, m.pos[2] + 8, me.pos[0], me.pos[1], me.pos[2] + 4.5, 1, 1)
    // defend: shoot back while in sight, never chase
    if (los || (hostile && NH === "passive")) return hunterAct(m, me, t, zone)
  }
  const [zx, zy, zr] = zoneAt(zone, t + 20)
  if (!m.wgoal || Math.hypot(m.wgoal[0] - x, m.wgoal[1] - y) < 6 || t > m.wgoalT) {
    for (let k = 0; k < 20; k++) {
      const a = SR() * Math.PI * 2, r = 20 + SR() * 60
      let gx = x + Math.cos(a) * r, gy = y + Math.sin(a) * r
      if (Math.hypot(gx - zx, gy - zy) > zr * 0.7) {
        gx = x + (zx - x) * 0.35
        gy = y + (zy - y) * 0.35
      }
      const n = map.nodeOf(gx, gy)
      if (map.isMain(n)) {
        m.wgoal = [map.nodeX(n), map.nodeY(n)]
        break
      }
    }
    m.wgoalT = t + 20
  }
  const act: any = {}
  if (m.wgoal) {
    m.nav!.setGoal(m.wgoal[0], m.wgoal[1], t, x, y)
    act.move = m.nav!.steer(x, y)
    act.aim = [Math.atan2(act.move[1], act.move[0]) || m.aim[0], 0]
  }
  return act
}

function buildObs(m: Mech, mechs: Mech[], shots: Shot[], zone: any, t: number, tick: number, items: Item[], world: any) {
  const pk: any[] = [], cr: any[] = []
  for (const it of items) {
    const d = Math.hypot(it.pos[0] - m.pos[0], it.pos[1] - m.pos[1])
    const b = Math.atan2(it.pos[1] - m.pos[1], it.pos[0] - m.pos[0])
    const inFov = d < cfg.vision.range && Math.abs(angDiff(b, m.aim[0])) < (cfg.vision.fov_deg * Math.PI) / 360
    if (!(d < cfg.vision.near_range || inFov)) continue
    if (!map.clear(m.pos[0], m.pos[1], m.pos[2] + 8, it.pos[0], it.pos[1], it.pos[2] + 1, 0.5, 0.5)) continue
    if (it.crate) cr.push({ id: it.id, pos: it.pos.slice(), hp: it.hp })
    else pk.push({ id: it.id, kind: it.kind, pos: it.pos.slice() })
  }
  const vis: any[] = []
  for (const o of mechs) {
    if (o === m || !o.alive || !present(o)) continue
    const d = Math.hypot(o.pos[0] - m.pos[0], o.pos[1] - m.pos[1])
    const b = Math.atan2(o.pos[1] - m.pos[1], o.pos[0] - m.pos[0])
    const inFov = d < cfg.vision.range && Math.abs(angDiff(b, m.aim[0])) < (cfg.vision.fov_deg * Math.PI) / 360
    const see = (d < cfg.vision.near_range || inFov) && map.clear(m.pos[0], m.pos[1], m.pos[2] + 8, o.pos[0], o.pos[1], o.pos[2] + 4.5, 0.5, 1)
    if (see) m.seen.set(o.id, t)
    if (t - (m.seen.get(o.id) ?? -99) <= cfg.vision.memory_s)
      vis.push({ id: o.id, kind: "mech", pos: o.pos.slice(), vel: o.vel.slice(), aim: o.aim.slice(), hp: o.hp, max_hp: o.maxHp, cubes: o.cubes, weapons: o.weapons.filter((w) => w).map((w) => w.kind), charging: o.weapons.some((w) => w && w.kind === "laser" && w.charge > 0) })
  }
  const projectiles = shots.filter((s) => s.owner !== m.id && Math.hypot(s.pos[0] - m.pos[0], s.pos[1] - m.pos[1]) < 200).map((s, i) => ({ id: i, kind: "gun", pos: s.pos.slice(), vel: s.vel.slice() }))
  const sounds: any[] = []
  for (const o of mechs) {
    if (o === m || !o.alive || !present(o)) continue
    const d = Math.hypot(o.pos[0] - m.pos[0], o.pos[1] - m.pos[1])
    const sp = Math.hypot(o.vel[0], o.vel[1])
    const kind = sp > 15 ? "boost" : sp > 5 ? "step" : null
    if (!kind || tick % 12 !== 0) continue
    if (d > cfg.sound.range[kind]) continue
    const b = Math.atan2(o.pos[1] - m.pos[1], o.pos[0] - m.pos[0]) + (SR() - 0.5) * 0.2
    sounds.push({ kind, bearing: b, dist: d < 60 ? "near" : d < 160 ? "mid" : "far", age: 0.05 })
  }
  const [zx, zy, zr] = zoneAt(zone, t)
  return {
    type: "obs", tick, t, alive: mechs.filter((q) => q.alive).length + (16 - mechs.length),
    self: {
      id: m.id, pos: m.pos.slice(), vel: m.vel.slice(), aim: m.aim.slice(), hp: m.hp, max_hp: m.maxHp, energy: m.energy, max_energy: 100,
      grounded: m.grounded, cubes: m.cubes, upgrades: [], weapons: m.weapons.map((w) => (w ? { ...w } : null)), in_zone: Math.hypot(m.pos[0] - zx, m.pos[1] - zy) <= zr,
      stats: { ...cfg.mech },
    },
    mechs: vis.concat(visList(m, world.npcs, 3.5).map((q: any) => ({ id: q.id, kind: "npc", pos: q.pos.slice(), vel: [0, 0, 0], aim: [0, 0], hp: q.hp, max_hp: 350 }))),
    projectiles, crates: cr, cubes: visList(m, world.cubes, 0.5).map((c: any) => ({ id: c.id, pos: c.pos.slice() })), pickups: pk, sounds, beams: [], events: m.events.slice(), drops: [], zone, offer: world.offer,
  } as any
}

const seeds = Number(process.argv[2] ?? 10)
const skill = Number(process.argv[3] ?? 0.6)
const give = process.argv[4] ?? "none"
const nH = Number(process.argv[5] ?? 15)
const armed = (process.argv[6] ?? "1") === "1"
if (process.argv[7]) Object.assign(P, JSON.parse(process.argv[7]))
const res = []
for (let s = 1; s <= seeds; s++) {
  const r = runMatch(s, skill, give, nH, armed)
  res.push(r)
  console.log(JSON.stringify(r))
}
const mean = (k: string) => (res.reduce((a, r: any) => a + r[k], 0) / res.length).toFixed(2)
console.log(`MEAN pts ${mean("pts")} kills ${mean("kills")} surv ${mean("surv")} dmgIn ${mean("dmgIn")} dmgOut ${mean("dmgOut")} stuck ${mean("stuck")} p99ms ${mean("p99")} max ${Math.max(...res.map((r) => r.maxMs)).toFixed(1)}`)
