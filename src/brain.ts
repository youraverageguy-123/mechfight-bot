// Decision making: world memory, target choice, weapons, movement modes.
import type { ActorObs, BotAction, Hello, Observation, ProjectileObs, SelfObs, WeaponKind, WeaponObs } from "../mechfight.ts"
import { draw } from "../mechfight.ts"
import { leadMortar, leadStraight, type AimSol } from "./aim.ts"
import type { GameMap } from "./map.ts"
import { Navigator } from "./nav.ts"
import { type Climb, type Roof, RoofIndex } from "./roof.ts"
import { angDiff, clamp, makeRng, norm2, now, TAU, wrapAngle } from "./util.ts"

// ---- tunables (A/B test these) ----
export const P = {
  retreatFrac: 0.38, // retreat below this HP fraction
  recoverFrac: 0.85, // stop retreating above this
  weaponRank: { laser: 3, lightning: 2, mortar: 1, gun: 0 } as Record<string, number>,
  lightningReserve: 12, // energy kept back
  farmUntil: 230, // farm crates / drones before this match time
  farmMinHp: 0.55,
  farmRange: 250, // walk this far for crates / drones
  droneValue: 250, // metres of detour a drone (2 cubes) is worth over a crate; 0 = don't seek drones
  cubeRange: 160, // walk this far for cubes when no enemy is in sight
  cubeFightRange: 30, // ...and this far while enemies are around
  exploreUntil: 240, // look for a second weapon until this match time
  multiRetreatFrac: 0.45,
  hunt: true, // chase enemies we lost sight of
  huntRange: 160, // ...up to this far
  huntMinHp: 0.55, // ...while above this HP fraction
  closeBoost: false, // boost while closing in on a target beyond our range band
  holdCoverW: 1.2, // hold-spot preference: + = next to buildings, - = open ground with long sightlines
  roof: false, // camp on rooftops (sim: survives longer but kills less, so off)
  roofFrom: 45, // earliest match time to head for a roof
  roofWeaponGrace: 60, // ...or this much later even with only the gun
  roofMaxTravel: 320, // don't walk further than this to reach a roof
  tickBudgetMs: 22, // optional searches stop once a decision has used this much time
  peek: true, // vs laser carriers, fight from a spot next to cover
  peekMinD: 20,
  duck: true, // hide from lasers being charged at us
  kiteMulti: true, // with 2+ threats close: back away from their centre while fighting the nearest
  kiteR: 200, // threats within this range count
  kiteMinN: 2, // ...when at least this many
  kiteAway: 0.85, // weight of backing away (vs strafing 0.5)
  targetHpW: 0.06, // target score = distance * 0.6 + hp * this (higher = finish wounded mechs first)
  silent: false, // walk under the step-sound speed when no threat is engaged
  lightningPush: false, // with lightning: boost straight at the target until it is inside arc range
  pushMaxD: 150, // ...when it is closer than this
  kite: false, // boost away from lightning carriers when we lack lightning (sim: no gain)
  gunBand: [60, 160] as [number, number],
  lightningBand: [14, 32] as [number, number],
  laserBand: [60, 200] as [number, number],
  upgrades: ["armor", "plating", "nanites", "gun_rate", "gun_damage", "lightning_efficiency", "laser_damage", "laser_charge",
    "gyros", "mortar_reload", "lightning_range", "lightning_chain", "gun_mag", "gun_velocity", "sensors", "servos", "capacitor",
    "reactor", "jets", "mortar_splash", "mortar_velocity", "laser_pierce"],
  dropMinDist: 70, // no enemy this close to a drop
  // Practice house bots hunt us weakest first; the others are idle. Only fight mechs that are after
  // us (hurt us, track us with their aim, or got hit by us) and keep away from the idle ones.
  selective: true,
  threatNear: 35, // anything closer than this counts as a threat
  avoidR: 60, // keep loot / farm / hold goals this far from idle bots, and hide from idle bots this close
  name: "ohio-ironclad",
  salt: 0, // mixed into our RNG seed: same strategy, different random choices
}

type Enemy = {
  id: number
  pos: [number, number, number]
  vel: [number, number, number]
  aim: [number, number]
  hp: number
  maxHp: number
  cubes: number
  weapons: string[]
  charging: boolean
  seen: number
  lastHurtUs: number
  aimCnt: number
  aimT: number
  weHitT: number
}

type Known = { id: number; kind?: string; pos: [number, number, number]; seen: number; hp?: number }

type Target = { id: number; kind: "mech" | "npc" | "crate"; pos: number[]; vel: number[]; hp: number; d: number; h: number; r: number }

export class Brain {
  name = P.name
  cfg: Hello["config"]
  map: GameMap
  nav: Navigator
  rnd: () => number
  myId: number
  enemies = new Map<number, Enemy>()
  pickups = new Map<number, Known>()
  crates = new Map<number, Known>()
  drones = new Map<number, Known>()
  cubes = new Map<number, Known>()
  badPickup = new Map<number, number>()
  lastHurt = -1e9
  lastHurtBearing = 0
  lastDealt = -1e9
  sounds: { b: number; t: number; kind: string; dist: string }[] = []
  retreating = false
  coverX = NaN
  coverY = NaN
  coverT = -1e9
  holdX = NaN
  holdY = NaN
  holdT = -1e9
  strafeSign = 1
  strafeFlip = 0
  shortUntil = -1
  scanYaw = 0
  missD: number[] = []
  roofs: RoofIndex
  roof: Roof | null = null
  roofPickT = -1e9
  climb: Climb | null = null
  climbing = false
  climbStart = 0
  badClimbs = new Set<string>()
  /** Roof we are camping on right now (movement must stay on it), or null. */
  campRoof: Roof | null = null
  descendComp = -1
  tickStart = 0
  engagedNow: (Enemy & { d: number })[] = []
  /** Milliseconds left for optional searches this decision (the hard limit is 50 ms). */
  private budget(): number {
    return P.tickBudgetMs - (now() - this.tickStart)
  }
  stats = { duckSeen: 0, duckFound: 0, charging: 0 }
  peekX = NaN
  peekY = NaN
  peekT = -1e9
  peekId = -1
  duckX = NaN
  duckY = NaN
  duckT = -1e9
  expX = NaN
  expY = NaN
  expT = -1e9
  visited = new Map<number, number>()
  escX = NaN
  escY = NaN
  escT = -1e9
  lastT = 0
  mode = "start"
  tickMs: number[] = []

  constructor(hello: Hello, map: GameMap) {
    this.cfg = hello.config
    this.map = map
    this.nav = new Navigator(map)
    this.roofs = new RoofIndex(map)
    this.rnd = makeRng((hello.seed ^ (P.salt * 2654435761)) >>> 0)
    this.myId = hello.slot
  }

  // ------------------------------------------------------------------ memory
  private remember(obs: Observation) {
    const t = obs.t
    const [sx, sy] = obs.self.pos
    for (const m of obs.mechs) {
      if (m.kind !== "mech") continue
      const e = this.enemies.get(m.id)
      const dd = Math.hypot(m.pos[0] - sx, m.pos[1] - sy)
      const aimAt = dd < 280 && Math.abs(angDiff(m.aim[0], Math.atan2(sy - m.pos[1], sx - m.pos[0]))) < Math.atan2(7, dd) + 0.04
      const aimCnt = aimAt ? Math.min(10, (e?.aimCnt ?? 0) + 1) : Math.max(0, (e?.aimCnt ?? 0) - 1)
      let aimT = e?.aimT ?? -1e9
      if (aimCnt >= 3 || (aimAt && m.charging)) aimT = t
      const ne: Enemy = {
        id: m.id,
        pos: [m.pos[0], m.pos[1], m.pos[2]],
        vel: [m.vel[0], m.vel[1], m.vel[2]],
        aim: [m.aim[0], m.aim[1]],
        hp: m.hp ?? e?.hp ?? this.cfg.mech.hp,
        maxHp: m.max_hp ?? e?.maxHp ?? this.cfg.mech.hp,
        cubes: m.cubes ?? 0,
        weapons: (m.weapons as string[] | undefined) ?? e?.weapons ?? ["gun"],
        charging: !!m.charging,
        seen: t,
        lastHurtUs: e?.lastHurtUs ?? -1e9,
        aimCnt,
        aimT,
        weHitT: e?.weHitT ?? -1e9,
      }
      this.enemies.set(m.id, ne)
    }
    for (const [id, e] of this.enemies) if (t - e.seen > 12) this.enemies.delete(id)
    const vis = (map: Map<number, Known>, list: { id: number; pos: number[]; kind?: string; hp?: number }[]) => {
      for (const it of list) map.set(it.id, { id: it.id, kind: it.kind, pos: [it.pos[0], it.pos[1], it.pos[2]], seen: t, hp: it.hp })
    }
    vis(this.pickups, obs.pickups)
    for (const m of obs.mechs) if (m.kind === "npc") this.drones.set(m.id, { id: m.id, pos: [m.pos[0], m.pos[1], m.pos[2]], seen: t, hp: m.hp })
    vis(this.crates, obs.crates)
    vis(this.cubes, obs.cubes)
    // forget remembered items that should be visible (close) but are not listed
    const [x, y] = obs.self.pos
    const forget = (map: Map<number, Known>, r: number) => {
      for (const [id, k] of map) if (k.seen < t && Math.hypot(k.pos[0] - x, k.pos[1] - y) < r) map.delete(id)
    }
    forget(this.pickups, 30)
    forget(this.crates, 30)
    forget(this.drones, 35)
    for (const [id, k] of this.drones) if (t - k.seen > 90) this.drones.delete(id)
    forget(this.cubes, 25)
    for (const [id, k] of this.cubes) if (t - k.seen > 60) this.cubes.delete(id)
    for (const ev of obs.events) {
      if (ev.type === "damaged") {
        if (ev.weapon !== "zone") {
          this.lastHurt = t
          if (ev.bearing != null) this.lastHurtBearing = ev.bearing
          if (ev.by != null) {
            const e = this.enemies.get(ev.by)
            if (e) e.lastHurtUs = t
          }
        }
      } else if (ev.type === "hit") {
        this.lastDealt = t
        const e = this.enemies.get(ev.target)
        if (e) e.weHitT = t
      }
      else if (ev.type === "effect" && ev.effect === "short") this.shortUntil = t + (this.cfg.weapons.lightning.short_s ?? 2)
      else if (ev.type === "take_failed") this.badPickup.set(ev.id, t + 4)
      else if (ev.type === "took") this.pickups.delete(ev.id)
    }
    for (const s of obs.sounds) {
      if (s.kind === "npc_gun" || s.kind === "crate" || s.kind === "drop" || s.kind === "death") continue
      this.sounds.push({ b: s.bearing, t: t - s.age, kind: s.kind, dist: s.dist })
    }
    this.sounds = this.sounds.filter((s) => t - s.t < 3)
  }

  // ------------------------------------------------------------------ zone
  private zoneAt(obs: Observation, tau: number): [number, number, number] {
    const z = obs.zone
    if (tau <= z.shrink_start) return [z.center[0], z.center[1], z.radius]
    if (tau >= z.shrink_end || z.shrink_end <= z.shrink_start) return [z.next_center[0], z.next_center[1], z.next_radius]
    const f = (tau - z.shrink_start) / (z.shrink_end - z.shrink_start)
    return [
      z.center[0] + (z.next_center[0] - z.center[0]) * f,
      z.center[1] + (z.next_center[1] - z.center[1]) * f,
      z.radius + (z.next_radius - z.radius) * f,
    ]
  }

  /** Is (x,y) safe for the next `horizon` seconds with `margin` m to spare? */
  private safe(obs: Observation, x: number, y: number, horizon: number, margin: number): boolean {
    const [cx, cy, r] = this.zoneAt(obs, obs.t + horizon)
    return Math.hypot(x - cx, y - cy) <= r - margin
  }

  /** Seconds until the moving edge reaches (x,y) (Infinity if never within this phase). */
  private zoneHitTime(obs: Observation, x: number, y: number): number {
    const z = obs.zone
    const t = obs.t
    for (let k = 0; k <= 30; k++) {
      const tau = t + k * 2
      const [cx, cy, r] = this.zoneAt(obs, tau)
      if (Math.hypot(x - cx, y - cy) > r - 4) return tau - t
      if (tau > z.shrink_end) break
    }
    return Infinity
  }

  // ------------------------------------------------------------------ helpers
  private weapon(me: SelfObs, kind: WeaponKind): { w: WeaponObs; slot: number } | null {
    for (let i = 0; i < me.weapons.length; i++) {
      const w = me.weapons[i]
      if (w && w.kind === kind) return { w, slot: i }
    }
    return null
  }

  private sizeOf(cubes: number): number {
    return 1 + (this.cfg.cubes.size_per_cube ?? 0.05) * cubes
  }

  private enemyDps(e: Enemy, d: number): number {
    const W = this.cfg.weapons
    const mul = 1 + 0.04 * Math.min(e.cubes, 10)
    let dps = 0
    for (const k of e.weapons) {
      if (k === "gun") dps += (W.gun.damage / W.gun.cooldown) * clamp(1 - d / 260, 0.25, 0.85)
      else if (k === "lightning" && d < W.lightning.range + 8) dps += (W.lightning.damage / W.lightning.cooldown) * 0.9
      else if (k === "laser" && d < W.laser.range) dps += (W.laser.damage / (W.laser.cooldown + W.laser.charge)) * 0.7
      else if (k === "mortar" && d < 230) dps += ((W.mortar.damage + W.mortar.splash) / 2 / W.mortar.cooldown) * 0.5
    }
    return Math.max(30, dps * mul)
  }

  private myDps(me: SelfObs, d: number): number {
    let dps = 0
    const mul = 1 + 0.04 * Math.min(me.cubes, 10)
    for (const w of me.weapons) {
      if (!w) continue
      const s = w.stats
      if (w.kind === "gun") dps += (s.damage / s.cooldown) * clamp(1 - d / 300, 0.3, 0.9)
      else if (w.kind === "lightning" && d < s.range) dps += (s.damage / s.cooldown) * 0.95
      else if (w.kind === "laser" && d < s.range) dps += (s.damage / (s.cooldown + s.charge)) * 0.75
      else if (w.kind === "mortar" && d < 230) dps += ((s.damage + s.splash) / 2 / s.cooldown) * 0.4
    }
    return Math.max(20, dps * mul)
  }

  // ------------------------------------------------------------------ main
  act(obs: Observation): BotAction {
    const t0 = now()
    this.tickStart = t0
    this.remember(obs)
    const me = obs.self
    const t = obs.t
    const dt = Math.max(0.05, Math.min(0.5, t - this.lastT))
    this.lastT = t
    const [x, y, z] = me.pos
    const st = me.stats
    const size = this.sizeOf(me.cubes)
    const muzzleZ = z + (st.muzzle_height ?? 7)
    const eyeZ = z + (st.eye_height ?? 8)
    const hpFrac = me.hp / me.max_hp
    const W = this.cfg.weapons
    const act: BotAction = {}

    // ---- upgrade offer
    if (obs.offer) {
      let pick = 0
      let bestRank = 1e9
      obs.offer.choices.forEach((c, i) => {
        const r = P.upgrades.indexOf(c.id)
        const rr = r < 0 ? 500 : r
        if (rr < bestRank) {
          bestRank = rr
          pick = i
        }
      })
      act.pick = pick
    }

    // ---- threats
    const gun = this.weapon(me, "gun")
    const lightning = this.weapon(me, "lightning")
    const laser = this.weapon(me, "laser")
    const mortar = this.weapon(me, "mortar")
    const gunRange = gun ? gun.w.stats.range : 0
    const maxDirect = Math.max(gunRange, laser ? laser.w.stats.range : 0, lightning ? lightning.w.stats.range : 0)

    const mechs: (Enemy & { d: number; los: boolean })[] = []
    for (const e of this.enemies.values()) {
      if (t - e.seen > 6) continue
      const d = Math.hypot(e.pos[0] - x, e.pos[1] - y)
      const ez = e.pos[2] + 4.5 * this.sizeOf(e.cubes)
      const los = t - e.seen < 0.15 && d < 260 && this.map.clear(x, y, muzzleZ, e.pos[0], e.pos[1], ez, 3, 2)
      mechs.push({ ...e, d, los })
    }
    // a threat is a mech that is after us; with P.selective, idle house bots are left alone
    const isThreat = (e: Enemy & { d: number }) =>
      !P.selective || t - e.lastHurtUs < 12 || t - e.aimT < 3 || t - e.weHitT < 12 || e.d < P.threatNear
    const engaged = mechs.filter((e) => isThreat(e) && ((e.los && e.d < 220) || t - e.lastHurtUs < 4 || e.d < 50))
    const idle: (Enemy & { d: number; los: boolean })[] = []
    if (P.selective)
      for (const e of this.enemies.values()) {
        if (t - e.seen > 20) continue
        const d = Math.hypot(e.pos[0] - x, e.pos[1] - y)
        const me2 = mechs.find((q) => q.id === e.id)
        if (me2 ? !isThreat(me2) : !isThreat({ ...e, d })) idle.push(me2 ?? { ...e, d, los: false })
      }
    /** Would going to (px,py) take us near an idle bot? */
    const nearIdle = (px: number, py: number) =>
      idle.some((e) => Math.hypot(e.pos[0] - px, e.pos[1] - py) < P.avoidR && Math.hypot(px - x, py - y) > 12)
    const recentHurt = t - this.lastHurt < 3

    // ---- choose a target
    let target: Target | null = null
    let bestScore = Infinity
    for (const e of mechs) {
      if (!e.los || e.d > maxDirect + 20 || !isThreat(e)) continue
      let s = e.d * 0.6 + e.hp * P.targetHpW
      if (t - e.lastHurtUs < 3) s -= 40
      if (s < bestScore) {
        bestScore = s
        const es = this.sizeOf(e.cubes)
        target = { id: e.id, kind: "mech", pos: e.pos, vel: e.vel, hp: e.hp, d: e.d, h: 9 * es, r: 2.5 * es }
      }
    }
    const fighting = engaged.length > 0
    // footsteps, boosting or gunfire close by: someone is coming, stop looking at crates
    const heardThreat = this.sounds.some((s) => t - s.t < 2 && s.dist !== "far" && s.kind !== "explosion")
    const farmOk = !fighting && hpFrac >= P.farmMinHp && !recentHurt && !heardThreat
    if (!target) {
      // drones that are shooting us, or farmable ones
      for (const m of obs.mechs) {
        if (m.kind !== "npc") continue
        const d = Math.hypot(m.pos[0] - x, m.pos[1] - y)
        if (d > Math.min(gunRange, 140)) continue
        if (!(farmOk && t < P.farmUntil) && !(recentHurt && d < 90)) continue
        if (!this.map.clear(x, y, muzzleZ, m.pos[0], m.pos[1], m.pos[2] + 1.75, 3, 1)) continue
        const s = d + (m.hp ?? 350) * 0.05
        if (s < bestScore) {
          bestScore = s
          target = { id: m.id, kind: "npc", pos: m.pos, vel: m.vel, hp: m.hp ?? 350, d, h: 3.5, r: 1.8 }
        }
      }
    }
    if (!target && farmOk && t < P.farmUntil) {
      for (const c of obs.crates) {
        const d = Math.hypot(c.pos[0] - x, c.pos[1] - y)
        if (d > Math.min(gunRange, 120)) continue
        if (!this.map.clear(x, y, muzzleZ, c.pos[0], c.pos[1], c.pos[2] + 1.2, 3, 0.8)) continue
        if (d < bestScore) {
          bestScore = d
          target = { id: c.id, kind: "crate", pos: c.pos, vel: [0, 0, 0], hp: c.hp ?? 300, d, h: 2.4, r: 1.4 }
        }
      }
    }

    // ---- fight or flight
    let focus: (Enemy & { d: number; los: boolean }) | null = null
    for (const e of engaged) if (!focus || e.d < focus.d) focus = e
    if (target && target.kind === "mech") focus = engaged.find((e) => e.id === target!.id) ?? focus
    if (fighting) {
      let theirDps = 0
      for (const e of engaged) theirDps += this.enemyDps(e, e.d)
      const ttkUs = me.hp / theirDps
      const f = focus!
      const ttkThem = f.hp / this.myDps(me, f.d)
      const finishing = f.los && f.hp < 0.35 * me.hp && ttkThem < 2.5 && ttkThem < ttkUs * 0.8
      if (!this.retreating) {
        const shooters = engaged.filter((e) => e.los || t - e.lastHurtUs < 3).length
        if ((hpFrac < P.retreatFrac && !finishing) || (ttkUs < ttkThem * 0.6 && hpFrac < 0.7 && !finishing) ||
          (shooters >= 2 && hpFrac < P.multiRetreatFrac && !finishing)) {
          this.retreating = true
          this.coverT = -1e9
        }
      } else if (finishing && hpFrac > 0.2) this.retreating = false
    }
    if (this.retreating && hpFrac >= P.recoverFrac) this.retreating = false

    // ---- aim + fire
    let aimYaw = me.aim[0]
    let aimPitch = 0
    const fire = [false, false]
    let aimSet = false
    let mortarMode = false
    if (target) {
      const tz = target.pos[2] + target.h * 0.5
      const floor = this.map.surfAt(target.pos[0], target.pos[1]) + target.h * 0.3
      const gs = gun ? gun.w.stats.speed : 252
      const sol = leadStraight(x, y, muzzleZ, target.pos[0], target.pos[1], tz, target.vel[0], target.vel[1], target.vel[2], gs, 3, floor, this.cfg.mech.gravity)
      aimYaw = sol.yaw
      aimPitch = sol.pitch
      aimSet = true
      const eYaw = Math.abs(angDiff(me.aim[0], sol.yaw))
      const ePitch = Math.abs(me.aim[1] - sol.pitch)
      const dd = Math.max(1, Math.hypot(sol.px - x, sol.py - y))
      const tolY = Math.atan2(target.r * 0.8, dd) + 0.004
      const tolP = Math.atan2(target.h * 0.35, dd) + 0.004
      const aimed = eYaw < tolY && ePitch < tolP
      if (gun && aimed && target.d <= gun.w.stats.range && (gun.w.ammo ?? 1) > 0 && !(gun.w.reload ?? 0)) fire[gun.slot] = true
      if (lightning) {
        const L = lightning.w.stats
        const cone = ((L.cone_deg ?? 30) * Math.PI) / 360
        const yawTo = Math.atan2(target.pos[1] - y, target.pos[0] - x)
        const inCone = Math.abs(angDiff(me.aim[0], yawTo)) < cone * 0.85
        if (inCone && target.d < L.range - 1.5 && me.energy >= (L.energy ?? 3) + (target.kind === "mech" ? P.lightningReserve * 0.5 : P.lightningReserve))
          fire[lightning.slot] = true
      }
      if (laser && target.kind !== "crate") {
        const Ls = laser.w.stats
        const charge = laser.w.charge ?? 0
        const yawTo = Math.atan2(target.pos[1] - y, target.pos[0] - x)
        const pitchTo = Math.atan2(tz - muzzleZ, target.d)
        const eY = Math.abs(angDiff(me.aim[0], yawTo))
        const eP = Math.abs(me.aim[1] - pitchTo)
        const ok = eY < Math.atan2(target.r * 0.7, target.d) && eP < Math.atan2(target.h * 0.3, target.d)
        if (charge > 0) {
          // keep holding until full and on target, then release
          const full = charge >= Ls.charge - 0.02
          fire[laser.slot] = !(full && ok)
          // fast movers: aim straight at them for the hitscan, not led
          if (target.kind === "mech" || target.kind === "npc") {
            aimYaw = yawTo
            aimPitch = pitchTo
          }
        } else if (laser.w.cooldown <= 0.05 && t > this.shortUntil && target.d < Ls.range - 5 && (target.kind === "mech" || target.d < 120)) {
          const lightningNear = mechs.some((e) => e.d < 60 && e.weapons.includes("lightning"))
          if (!lightningNear || target.d < 40) fire[laser.slot] = true
        }
      }
    }
    // laser that is charging with no target: keep it (it auto-fires later) unless shorted
    if (!target && laser && (laser.w.charge ?? 0) > 0) fire[laser.slot] = true

    // mortar on hidden-but-known enemies
    if (mortar && mortar.w.cooldown <= 0.3) {
      let mt: (Enemy & { d: number; los: boolean }) | null = null
      for (const e of mechs) {
        if (e.d < 28 || e.d > 230 || t - e.seen > 4 || !isThreat(e)) continue
        if (e.los && gun && (gun.w.ammo ?? 0) > 0 && e.d < gunRange) continue
        if (!mt || e.d < mt.d) mt = e
      }
      if (mt && !(target && target.kind === "mech" && target.d < 45)) {
        const ms = mortar.w.stats
        const lz = z + (this.cfg.mech.height * size + 0.5)
        const sol = leadMortar(this.map, x, y, lz, mt.pos[0], mt.pos[1], mt.pos[2] + 0.5, mt.vel[0], mt.vel[1], ms.speed, ms.gravity,
          this.cfg.mech.min_pitch, this.cfg.mech.max_pitch)
        if (sol && Math.hypot(sol.px - x, sol.py - y) > 20) {
          mortarMode = true
          aimYaw = sol.yaw
          aimPitch = sol.pitch
          aimSet = true
          fire[0] = fire[1] = false
          if (mortar.w.cooldown <= 0 && Math.abs(angDiff(me.aim[0], sol.yaw)) < 0.03 && Math.abs(me.aim[1] - sol.pitch) < 0.025)
            fire[mortar.slot] = true
          // keep laser charge alive if any
          if (laser && (laser.w.charge ?? 0) > 0) fire[laser.slot] = true
        }
      }
    }
    void mortarMode

    if (!aimSet) {
      if (focus) {
        aimYaw = Math.atan2(focus.pos[1] - y, focus.pos[0] - x)
        aimPitch = Math.atan2(focus.pos[2] + 4.5 - muzzleZ, Math.max(1, focus.d))
      } else if (recentHurt) {
        aimYaw = this.lastHurtBearing
        aimPitch = 0
      } else if (this.sounds.length > 0) {
        const s = this.sounds[this.sounds.length - 1]
        aimYaw = s.b
        aimPitch = 0
      } else {
        this.scanYaw = wrapAngle(this.scanYaw + 1.8 * dt)
        aimYaw = this.scanYaw
        aimPitch = 0
      }
    }
    if (aimSet || focus) this.scanYaw = aimYaw

    // reload when idle
    if (gun && !target && (gun.w.ammo ?? 99) < gun.w.stats.mag * 0.5 && !(gun.w.reload ?? 0)) act.reload = true

    // ---- weapon pickups
    const reach = this.cfg.loot.pickup_radius
    const have = new Set(me.weapons.filter((w) => w).map((w) => w!.kind))
    const slot1 = me.weapons[1]
    const wantRank = (k: string) => P.weaponRank[k] ?? 0
    const curRank = slot1 ? wantRank(slot1.kind) : -1
    const laserBusy = laser && (laser.w.charge ?? 0) > 0
    for (const p of obs.pickups) {
      if ((this.badPickup.get(p.id) ?? 0) > t) continue
      if (have.has(p.kind)) continue
      if (Math.hypot(p.pos[0] - x, p.pos[1] - y) > reach - 0.3 || Math.abs(p.pos[2] - z) > 3.8) continue
      if (!slot1 && p.kind !== "gun") {
        act.take = p.id
        break
      }
      if (slot1 && wantRank(p.kind) > curRank && !laserBusy && slot1.kind !== "gun") {
        act.take = p.id
        act.drop = 1
        break
      }
    }

    // ---- movement
    let move: [number, number] = [0, 0]
    let boost = false
    let jump = false
    const walk = st.walk_speed ?? 11.7
    let goal: [number, number] | null = null
    let goalBoost = false
    this.mode = "hold"

    // zone pressure
    const zn = obs.zone
    const hit = this.zoneHitTime(obs, x, y)
    const needNext = Math.hypot(x - zn.next_center[0], y - zn.next_center[1]) - Math.max(0, zn.next_radius - Math.min(25, zn.next_radius * 0.3))
    const eta = Math.max(0, needNext) * 1.35 / walk
    const zoneUrgent = !me.in_zone || hit < eta + 10
    if (zoneUrgent && !this.roofs.roofAt(this.map, x, y, z)) {
      const dx = x - zn.next_center[0], dy = y - zn.next_center[1]
      const dl = Math.hypot(dx, dy) || 1
      const rr = Math.max(0, zn.next_radius - Math.min(30, zn.next_radius * 0.4))
      goal = [zn.next_center[0] + (dx / dl) * rr, zn.next_center[1] + (dy / dl) * rr]
      goalBoost = !me.in_zone || hit < eta + 3
      this.mode = "zone"
    }

    // rooftops
    const curRoof = this.roofs.roofAt(this.map, x, y, z)
    const roofActive = P.roof && t >= P.roofFrom && (!!slot1 || t >= P.roofFrom + P.roofWeaponGrace) && !zoneUrgent
    this.campRoof = null
    let roofBusy = false
    if (!roofActive) {
      this.roof = null
      this.climbing = false
    } else if (!this.roof || t - this.roofPickT > 4 || !this.roofOk(obs, this.roof)) {
      const r = this.pickRoof(obs, x, y, curRoof)
      if (r !== this.roof) {
        this.climb = null
        this.climbing = false
      }
      this.roof = r
      this.roofPickT = t
    }
    if (this.roof && roofActive) {
      const r = this.roof
      if (curRoof === r) {
        this.climbing = false
        this.climb = null
        this.campRoof = r
        roofBusy = true
        this.mode = "roof"
        if (Math.hypot(r.campX - x, r.campY - y) > 2.5) goal = [r.campX, r.campY]
      } else if (!curRoof && (!fighting || this.climbing || !(focus && focus.los))) {
        if (!this.climb) this.climb = this.pickClimb(r, x, y)
        const c = this.climb
        if (c) {
          roofBusy = true
          const dg = Math.hypot(c.gx - x, c.gy - y)
          if (this.climbing) this.mode = "climb"
          else if (dg > 3) {
            goal = [c.gx, c.gy]
            this.mode = "to-roof"
          } else {
            const need = Math.min(95, (c.ez - z) * 1.8 + 15)
            if (me.energy >= need) {
              this.climbing = true
              this.climbStart = t
              this.mode = "climb"
            } else this.mode = "roof-wait"
          }
        } else this.roof = null
      }
    }

    // on a roof we are not camping on (zone moved, or a better roof): walk to the edge nearest
    // where we are going and step off it. Never jet: that only lifts us higher.
    let descend: [number, number] | null = null
    if (curRoof && this.campRoof !== curRoof && !this.climbing) {
      const tgt = this.roof ? this.pickClimb(this.roof, x, y) : null
      const tx = tgt ? tgt.gx : zn.next_center[0], ty = tgt ? tgt.gy : zn.next_center[1]
      let best: Climb | null = null
      let bs = Infinity
      for (const c of curRoof.climbs) {
        const s = Math.hypot(c.gx - tx, c.gy - ty) + Math.hypot(c.ex - x, c.ey - y) * 0.7
        if (s < bs) {
          bs = s
          best = c
        }
      }
      if (best) {
        roofBusy = true
        this.mode = "descend"
        this.descendComp = curRoof.comp
        if (Math.hypot(best.ex - x, best.ey - y) > 2.5) goal = [best.ex, best.ey]
        else descend = norm2(best.gx - x, best.gy - y)
      }
    }

    // retreat to cover
    if (!goal && this.retreating && fighting && !roofBusy) {
      if (t - this.coverT > 1.5 || Number.isNaN(this.coverX)) this.pickCover(obs, engaged)
      if (!Number.isNaN(this.coverX)) {
        goal = [this.coverX, this.coverY]
        goalBoost = me.energy > 25
        this.mode = "retreat"
      }
    }

    // supply drop
    if (!goal && !this.retreating && !roofBusy) {
      for (const dr of obs.drops ?? []) {
        if (dr.holder != null && dr.holder !== this.myId) continue
        const d = Math.hypot(dr.pos[0] - x, dr.pos[1] - y)
        if (d / walk > dr.lands_in + 12) continue
        if (d > 260) continue
        if (mechs.some((e) => Math.hypot(e.pos[0] - dr.pos[0], e.pos[1] - dr.pos[1]) < P.dropMinDist)) continue
        if (hpFrac < 0.45 && fighting) continue
        if (!this.safe(obs, dr.pos[0], dr.pos[1], Math.max(10, d / walk + 6), 6)) continue
        goal = d < dr.radius - 2.5 ? [x, y] : [dr.pos[0], dr.pos[1]]
        this.mode = "drop"
        break
      }
    }

    // fighting movement (range band + strafe) is computed below; here we pick approach goals
    let fightMove = false
    if (!goal && fighting && focus && !this.campRoof && !this.climbing) {
      const band = this.band(me)
      if (focus.los) fightMove = true
      else if (P.hunt && !this.retreating && hpFrac > P.huntMinHp && t - focus.seen < 5 && focus.d < P.huntRange) {
        goal = [focus.pos[0], focus.pos[1]]
        this.mode = "hunt"
      } else fightMove = true
      void band
    }

    // an idle bot has us in sight close by: get out of its view instead of provoking it
    if (P.selective && !goal && !fightMove && !roofBusy && !this.campRoof) {
      const watcher = idle.filter((e) => e.los && e.d < P.avoidR && t - e.seen < 0.5)
      if (watcher.length > 0) {
        if (t - this.coverT > 1.5 || Number.isNaN(this.coverX)) this.pickCover(obs, watcher)
        if (!Number.isNaN(this.coverX)) {
          goal = [this.coverX, this.coverY]
          this.mode = "avoid"
        }
      }
    }

    // cubes nearby
    if (!goal && !fightMove && !roofBusy) {
      let best: Known | null = null
      let bd = fighting ? (engaged.some((e) => e.los) ? P.cubeFightRange : Math.max(P.cubeFightRange, 40)) : P.cubeRange
      for (const c of this.cubes.values()) {
        const d = Math.hypot(c.pos[0] - x, c.pos[1] - y)
        if (d < bd && Math.abs(c.pos[2] - this.map.surfAt(c.pos[0], c.pos[1])) < 3 && this.safe(obs, c.pos[0], c.pos[1], d / walk + 5, 5) && !nearIdle(c.pos[0], c.pos[1])) {
          bd = d
          best = c
        }
      }
      if (best) {
        goal = [best.pos[0], best.pos[1]]
        this.mode = "cube"
      }
    }

    // weapon loot
    if (!goal && !fighting && !roofBusy) {
      let best: Known | null = null
      let bs = Infinity
      for (const p of this.pickups.values()) {
        if ((this.badPickup.get(p.id) ?? 0) > t || have.has(p.kind as WeaponKind) || p.kind === "gun") continue
        const r = wantRank(p.kind!)
        if (slot1 && r <= curRank) continue
        const ground = this.map.surfAt(p.pos[0], p.pos[1])
        if (Math.abs(p.pos[2] - ground) > 2.5) continue // on a roof we can't reach
        const n = this.map.nodeOf(p.pos[0], p.pos[1])
        const nn = this.map.nearestNode(p.pos[0], p.pos[1], 2, (k) => this.map.isMain(k))
        if (nn < 0 || (!this.map.isMain(n) && Math.hypot(this.map.nodeX(nn) - p.pos[0], this.map.nodeY(nn) - p.pos[1]) > 3)) continue
        const d = Math.hypot(p.pos[0] - x, p.pos[1] - y)
        if (d > (slot1 ? 120 : 260)) continue
        if (!this.safe(obs, p.pos[0], p.pos[1], d / walk + 5, 5) || nearIdle(p.pos[0], p.pos[1])) continue
        const s = d - r * 25
        if (s < bs) {
          bs = s
          best = p
        }
      }
      if (best) {
        goal = [best.pos[0], best.pos[1]]
        this.mode = "loot"
      }
    }

    // farm: walk toward remembered crates / drones
    if (!goal && farmOk && t < P.farmUntil && !target && !roofBusy) {
      let best: Known | null = null
      let bs = Infinity
      let bestIsDrone = false
      const consider = (c: Known, value: number, drone: boolean) => {
        const d = Math.hypot(c.pos[0] - x, c.pos[1] - y)
        if (d > P.farmRange || Math.abs(c.pos[2] - this.map.surfAt(c.pos[0], c.pos[1])) > 3) return
        if (!this.safe(obs, c.pos[0], c.pos[1], d / walk + 5, 8) || nearIdle(c.pos[0], c.pos[1])) return
        const s = d - value
        if (s < bs) {
          bs = s
          best = c
          bestIsDrone = drone
        }
      }
      for (const c of this.crates.values()) consider(c, 0, false)
      if (P.droneValue > 0 && hpFrac > 0.6) for (const c of this.drones.values()) consider(c, P.droneValue, true)
      const b = best as Known | null
      if (b && Math.hypot(b.pos[0] - x, b.pos[1] - y) > (bestIsDrone ? 55 : 25)) {
        goal = [b.pos[0], b.pos[1]]
        this.mode = bestIsDrone ? "farm-drone" : "farm"
      }
    }

    // explore for a weapon while we only have the gun
    this.markVisited(x, y, t)
    if (!goal && !fightMove && !fighting && !target && !slot1 && hpFrac > 0.4 && t < P.exploreUntil && !roofBusy) {
      const reached = Math.hypot(this.expX - x, this.expY - y) < 12
      if (Number.isNaN(this.expX) || reached || t - this.expT > 30 || !this.safe(obs, this.expX, this.expY, 25, 15) || nearIdle(this.expX, this.expY)) this.pickExplore(obs)
      if (!Number.isNaN(this.expX)) {
        goal = [this.expX, this.expY]
        this.mode = "explore"
      }
    }

    // shooting a drone or crate with nobody around: stand still (silent, steadier aim)
    const farmShoot = !goal && !fightMove && !fighting && !!target && target.kind !== "mech" && P.droneValue > 0
    if (farmShoot) this.mode = "farm-shoot"

    // hold near cover inside the next circle
    if (!goal && !fightMove && !roofBusy && !farmShoot) {
      if (t - this.holdT > 6 || Number.isNaN(this.holdX) || !this.safe(obs, this.holdX, this.holdY, 20, 6)) this.pickHold(obs)
      if (!Number.isNaN(this.holdX)) {
        if (Math.hypot(this.holdX - x, this.holdY - y) > 3) goal = [this.holdX, this.holdY]
        this.mode = target ? "farm-shoot" : "hold"
      }
    }

    const wantComp = this.mode === "roof" && this.campRoof ? this.campRoof.comp : this.mode === "descend" ? this.descendComp : -1
    if (wantComp !== this.nav.goalComp) {
      this.nav.goalComp = wantComp
      this.nav.lastPlan = -1e9
    }
    if (goal) {
      this.nav.budgetMs = Math.max(2, Math.min(10, this.budget() - 4))
      this.nav.setGoal(goal[0], goal[1], t, x, y)
      move = this.nav.steer(x, y)
      boost = goalBoost
    }

    if (fightMove && focus) {
      this.mode = this.retreating ? "retreat-fight" : "fight"
      this.engagedNow = engaged
      move = this.fightMove(me, focus, t)
      boost = me.energy > 60 && !lightning
      // lightning shorts lasers and out-damages everything up close: rush in
      if (P.lightningPush && lightning && !this.retreating && focus.d > P.lightningBand[1] + 4 && focus.d < P.pushMaxD && me.energy > 22) boost = true
      // the gun only hits jinking targets reliably inside ~70 m: get there fast
      if (P.closeBoost && !this.retreating && focus.d > this.band(me)[1] + 10 && me.energy > 25) boost = true
      // kite lightning carriers we can't match up close
      if (P.kite && !lightning && focus.weapons.includes("lightning") && focus.d < 60 && me.energy > 8) boost = true
    } else if (this.campRoof && fighting && focus && !goal) {
      // on the roof: small side-steps around the camp spot, never off the edge
      this.updateStrafe(t)
      const by = Math.atan2(focus.pos[1] - y, focus.pos[0] - x)
      const sx = -Math.sin(by) * this.strafeSign, sy = Math.cos(by) * this.strafeSign
      const far = Math.hypot(this.campRoof.campX - x, this.campRoof.campY - y) > 5
      const m2: [number, number] = far ? norm2(this.campRoof.campX - x, this.campRoof.campY - y) : [sx, sy]
      if (this.walkOk(x, y, x + m2[0] * 3, y + m2[1] * 3)) move = m2
      else this.strafeSign = -this.strafeSign
    } else if (fighting && goal && focus && this.mode !== "zone") {
      // moving with enemies around: blend in a bit of strafe
      const by = Math.atan2(focus.pos[1] - y, focus.pos[0] - x)
      this.updateStrafe(t)
      const sx = -Math.sin(by) * this.strafeSign, sy = Math.cos(by) * this.strafeSign
      const m2 = norm2(move[0] + sx * 0.35, move[1] + sy * 0.35)
      if (this.walkOk(x, y, x + m2[0] * 3, y + m2[1] * 3)) move = m2
    }

    // dodge incoming fire
    const dodge = this.dodge(obs, me)
    if (dodge) {
      const m2 = norm2(move[0] * 0.4 + dodge[0], move[1] * 0.4 + dodge[1])
      if (this.walkOk(x, y, x + m2[0] * 3, y + m2[1] * 3)) {
        move = m2
        if (me.energy > 15) boost = true
      }
    }

    // a laser is being charged at us: it is hitscan and the shooter tracks us, so strafing can't
    // dodge it. The charge shows for ~0.8 s; get the whole body behind something solid in that time.
    if (P.duck) {
      this.stats.charging += obs.mechs.filter((m) => m.kind === "mech" && m.charging).length
      const chargers = obs.mechs.filter((m) => m.kind === "mech" && m.charging && this.aimedAtUs(m, x, y, z))
      if (chargers.length > 0) {
        const spot = this.duckSpot(t, chargers, x, y, z)
        this.stats.duckSeen++
        if (spot) this.stats.duckFound++
        else this.missD.push(Math.round(Math.hypot(chargers[0].pos[0] - x, chargers[0].pos[1] - y)) + (this.mode === "peek" ? 1000 : 0))
        if (spot) {
          move = norm2(spot[0] - x, spot[1] - y)
          if (Math.hypot(spot[0] - x, spot[1] - y) < 0.8) move = [0, 0]
          boost = me.energy > 5
          this.mode = "duck"
        }
      } else this.duckT = -1e9
    }

    if (descend) {
      move = descend
      jump = false
      boost = false
    }
    if (this.climbing && this.climb) {
      const c = this.climb
      const r = this.roof!
      let [mx, my] = norm2(c.ex - x, c.ey - y)
      if (Math.hypot(c.ex - x, c.ey - y) < 1.5) [mx, my] = norm2(r.campX - x, r.campY - y)
      move = [mx, my]
      jump = me.grounded && z < c.ez - 0.5
      boost = !me.grounded && z < c.ez + 2.5 && me.energy > 0.5
      this.mode = "climb"
      const fellBack = me.grounded && t - this.climbStart > 1.2 && z < c.gz + 1.5 && me.energy < 10
      if (t - this.climbStart > 7 || fellBack) {
        this.badClimbs.add(c.ex + "," + c.ey)
        this.climbing = false
        this.climb = null
      }
    }

    // silent creeping: under the step-sound speed unless fighting, fleeing or racing the zone
    if (P.silent && !fighting && !this.retreating && !boost && !this.climbing && !descend &&
      this.mode !== "zone" && this.mode !== "escape" && this.mode !== "drop") {
      const sl = (this.cfg.sound.step_speed ?? 5) * 0.92 / (st.walk_speed ?? 11.7)
      const ml = Math.hypot(move[0], move[1])
      if (ml > sl) move = [(move[0] / ml) * sl, (move[1] / ml) * sl]
    }

    // stuck recovery
    const commanded = Math.hypot(move[0], move[1]) > 0.3
    if (!this.climbing && !this.campRoof && this.mode !== "descend" && this.nav.updateStuck(t, x, y, commanded && this.mode !== "fight", move, this.rnd)) {
      move = this.nav.stuckDir
      if (me.grounded && this.nav.stuckCount % 2 === 0) jump = true
      if (!me.grounded && me.energy > 10) boost = true
    }
    // stranded off the main nav graph (stadium bowl, a roof, a pit): head for the nearest main
    // node, jumping and jetting over whatever is in the way
    const isMain = (n: number) => this.map.isMain(n)
    const onMain = this.map.nearestNode(x, y, 1, isMain) >= 0
    if (!onMain && !fightMove && !this.climbing && !this.campRoof && this.mode !== "descend" && (!goal || this.nav.partial || this.nav.path.length === 0)) {
      if (t - this.escT > 1 || Number.isNaN(this.escX)) {
        const n = this.map.nearestNode(x, y, 70, isMain)
        this.escT = t
        if (n >= 0) {
          this.escX = this.map.nodeX(n)
          this.escY = this.map.nodeY(n)
        }
      }
      if (!Number.isNaN(this.escX)) {
        const [ex, ey] = norm2(this.escX - x, this.escY - y)
        move = [ex, ey]
        const sp = Math.hypot(me.vel[0], me.vel[1])
        this.mode = "escape"
        if (me.grounded && sp < 3) jump = true
        if (!me.grounded && me.energy > 3 && me.vel[2] < 9) boost = true
      }
    }

    act.move = [move[0], move[1]]
    act.boost = boost
    act.jump = jump
    act.aim = [wrapAngle(aimYaw), clamp(aimPitch, this.cfg.mech.min_pitch, this.cfg.mech.max_pitch)]
    act.fire = fire
    draw.label(`${this.mode} hp${Math.round(me.hp)} e${Math.round(me.energy)}${target ? " T" + target.id : ""}`)
    const ms = now() - t0
    this.tickMs.push(ms)
    return act
  }

  private band(me: SelfObs): [number, number] {
    if (this.weapon(me, "lightning")) return P.lightningBand
    if (this.weapon(me, "laser")) return P.laserBand
    return P.gunBand
  }

  private updateStrafe(t: number) {
    if (t >= this.strafeFlip) {
      this.strafeSign = -this.strafeSign
      this.strafeFlip = t + 0.45 + this.rnd() * 0.75
    }
  }

  private fightMove(me: SelfObs, e: Enemy & { d: number }, t: number): [number, number] {
    const [x, y] = me.pos
    const band = this.band(me)
    const by = Math.atan2(e.pos[1] - y, e.pos[0] - x)
    const fx = Math.cos(by), fy = Math.sin(by)
    if (P.kiteMulti) {
      // several hunters closing in: back away from their centre so they reach us one at a time
      const near = this.engagedNow.filter((q) => q.d < P.kiteR)
      if (near.length >= P.kiteMinN) {
        let cx = 0, cy = 0
        for (const q of near) {
          cx += q.pos[0] / near.length
          cy += q.pos[1] / near.length
        }
        const [ax, ay] = norm2(x - cx, y - cy)
        this.updateStrafe(t)
        for (let attempt = 0; attempt < 2; attempt++) {
          const m = norm2(ax * P.kiteAway - fy * this.strafeSign * 0.5, ay * P.kiteAway + fx * this.strafeSign * 0.5)
          if (this.walkOk(x, y, x + m[0] * 4, y + m[1] * 4)) {
            this.mode = "kite"
            return m
          }
          this.strafeSign = -this.strafeSign
        }
      }
    }
    let radial = 0
    if (this.retreating) radial = -0.8
    else if (e.d > band[1]) radial = 0.8
    else if (e.d < band[0]) radial = -0.7
    // enemy holds lightning and we don't: keep out of its arc
    if (!this.weapon(me, "lightning") && e.weapons.includes("lightning") && e.d < 55) radial = -0.9
    if (P.lightningPush && this.weapon(me, "lightning") && !this.retreating && e.d > band[1] && e.d < P.pushMaxD) {
      // rush: straight in with a light weave
      this.updateStrafe(t)
      const m = norm2(fx - fy * this.strafeSign * 0.3, fy + fx * this.strafeSign * 0.3)
      if (this.walkOk(x, y, x + m[0] * 4, y + m[1] * 4)) return m
      this.nav.setGoal(e.pos[0], e.pos[1], t, x, y)
      return this.nav.steer(x, y)
    }
    // against a laser at range, fight from a spot with a hiding place next to it
    if (P.peek && e.weapons.includes("laser") && e.d > P.peekMinD && !this.retreating) {
      const pk = this.peekSpot(me, e, t)
      if (pk) {
        const dx = pk[0] - x, dy = pk[1] - y
        const dl = Math.hypot(dx, dy)
        this.mode = "peek"
        if (dl > 1.5) {
          if (this.walkOk(x, y, pk[0], pk[1])) return [dx / dl, dy / dl]
          this.nav.setGoal(pk[0], pk[1], t, x, y)
          return this.nav.steer(x, y)
        }
        return [0, 0]
      }
    }
    this.updateStrafe(t)
    for (let attempt = 0; attempt < 2; attempt++) {
      const sx = -fy * this.strafeSign, sy = fx * this.strafeSign
      const m = norm2(sx + fx * radial, sy + fy * radial)
      if (this.walkOk(x, y, x + m[0] * 4, y + m[1] * 4)) return m
      this.strafeSign = -this.strafeSign
      this.strafeFlip = t + 0.6 + this.rnd() * 0.5
    }
    // boxed in: just move radially if possible
    const m = norm2(fx * (radial || 0.5), fy * (radial || 0.5))
    if (this.walkOk(x, y, x + m[0] * 4, y + m[1] * 4)) return m
    return [0, 0]
  }

  /** Sidestep for bullets on a collision course, mortar shells landing near, and lasers aimed at us. */
  private dodge(obs: Observation, me: SelfObs): [number, number] | null {
    const [x, y, z] = me.pos
    const cz = z + 4.5
    const g = this.cfg.weapons.mortar.gravity ?? 22
    let ax = 0, ay = 0
    let any = false
    for (const p of obs.projectiles as ProjectileObs[]) {
      const [px, py, pz] = p.pos
      const [vx, vy, vz] = p.vel
      if (p.kind === "mortar") {
        // simulate to ground
        let lx = px, ly = py
        for (let k = 1; k <= 60; k++) {
          const tt = k * 0.05
          const qx = px + vx * tt, qy = py + vy * tt, qz = pz + vz * tt - 0.5 * g * tt * tt
          lx = qx
          ly = qy
          const c = this.map.idx(qx, qy)
          if (qz < this.map.terrain[c] || (qz > this.map.sBase[c] && qz < this.map.sTop[c])) break
        }
        const d = Math.hypot(lx - x, ly - y)
        const R = (this.cfg.weapons.mortar.splash_radius ?? 7) + 3
        if (d < R) {
          const [ux, uy] = d > 0.3 ? [(x - lx) / d, (y - ly) / d] : [Math.cos(me.aim[0] + Math.PI / 2), Math.sin(me.aim[0] + Math.PI / 2)]
          ax += ux * 2
          ay += uy * 2
          any = true
        }
        continue
      }
      const rx = x - px, ry = y - py, rz = cz - pz
      const vv = vx * vx + vy * vy + vz * vz
      if (vv < 1) continue
      const tc = (rx * vx + ry * vy + rz * vz) / vv
      if (tc < 0 || tc > 0.8) continue
      const qx = px + vx * tc - x, qy = py + vy * tc - y
      const miss = Math.hypot(qx, qy)
      if (miss > 3.5) continue
      // move perpendicular to its path, away from the closest-approach point
      const [nx, ny] = norm2(-vy, vx)
      const side = nx * -qx + ny * -qy >= 0 ? 1 : -1
      ax += nx * side
      ay += ny * side
      any = true
    }
    // lasers being charged at us
    for (const m of obs.mechs) {
      if (m.kind !== "mech" || !m.charging) continue
      const d = Math.hypot(m.pos[0] - x, m.pos[1] - y)
      const yawTo = Math.atan2(y - m.pos[1], x - m.pos[0])
      if (Math.abs(angDiff(m.aim[0], yawTo)) < Math.atan2(6, d) + 0.05) {
        ax += -Math.sin(yawTo) * this.strafeSign * 1.5
        ay += Math.cos(yawTo) * this.strafeSign * 1.5
        any = true
      }
    }
    return any ? norm2(ax, ay) : null
  }

  /** Nearest reachable spot hidden from every engaged enemy, inside the safe zone. */
  private pickCover(obs: Observation, threats: Enemy[]) {
    const [x, y] = obs.self.pos
    const m = this.map
    this.coverT = obs.t
    let best = -1
    let bs = Infinity
    const radii = [8, 14, 22, 32, 45, 60]
    for (const r of radii) {
      if (this.budget() < 4) break
      for (let a = 0; a < 16; a++) {
        const ang = (a / 16) * TAU
        const cx = x + Math.cos(ang) * r, cy = y + Math.sin(ang) * r
        const n = m.nearestNode(cx, cy, 1, (k) => m.isMain(k))
        if (n < 0) continue
        const nx = m.nodeX(n), ny = m.nodeY(n), nz = m.nodeZ(n)
        if (!this.safe(obs, nx, ny, 15, 6)) continue
        let hidden = true
        let minD = Infinity
        for (const e of threats) {
          const ez = e.pos[2] + 8
          minD = Math.min(minD, Math.hypot(e.pos[0] - nx, e.pos[1] - ny))
          if (m.clear(e.pos[0], e.pos[1], ez, nx, ny, nz + 8.5, 1, 0.5) || m.clear(e.pos[0], e.pos[1], ez, nx, ny, nz + 4.5, 1, 0.5)) {
            hidden = false
            break
          }
        }
        if (!hidden) continue
        // moving toward the enemy to reach cover is bad
        let s = r - m.cover[n] * 0.15
        if (minD < 25) s += 40
        if (s < bs) {
          bs = s
          best = n
        }
      }
    }
    if (best >= 0) {
      this.coverX = m.nodeX(best)
      this.coverY = m.nodeY(best)
    } else {
      // nothing hidden: back away from the closest threat toward the zone centre
      let tx = 0, ty = 0
      for (const e of threats) {
        const [ux, uy] = norm2(x - e.pos[0], y - e.pos[1])
        tx += ux
        ty += uy
      }
      const zc = obs.zone.next_center
      const [zx, zy] = norm2(zc[0] - x, zc[1] - y)
      const [ux, uy] = norm2(tx + zx * 0.5, ty + zy * 0.5)
      this.coverX = x + ux * 30
      this.coverY = y + uy * 30
    }
  }

  /** Straight walk that also stays on the roof we camp on (away from its edge). */
  private walkOk(x: number, y: number, px: number, py: number): boolean {
    if (!this.map.walkLine(x, y, px, py)) return false
    const r = this.campRoof
    if (!r) return true
    const n = this.map.nodeOf(px, py)
    return this.map.comp[n] === r.comp && this.roofs.edge[n] >= 2
  }

  private roofOk(obs: Observation, r: Roof): boolean {
    const zn = obs.zone
    return this.safe(obs, r.campX, r.campY, 40, 12) && Math.hypot(r.campX - zn.next_center[0], r.campY - zn.next_center[1]) <= zn.next_radius - 8
  }

  private pickRoof(obs: Observation, x: number, y: number, cur: Roof | null): Roof | null {
    if (cur && this.roofOk(obs, cur)) return cur
    let best: Roof | null = null
    let bs = Infinity
    for (const r of this.roofs.roofs) {
      if (!this.roofOk(obs, r)) continue
      let d = Infinity
      for (const c of r.climbs) if (!this.badClimbs.has(c.ex + "," + c.ey)) d = Math.min(d, Math.hypot(c.gx - x, c.gy - y))
      if (d > P.roofMaxTravel) continue
      const s = d - r.edgeDist * 2.5
      if (s < bs) {
        bs = s
        best = r
      }
    }
    return best
  }

  private pickClimb(r: Roof, x: number, y: number): Climb | null {
    let best: Climb | null = null
    let bd = Infinity
    for (const c of r.climbs) {
      if (this.badClimbs.has(c.ex + "," + c.ey)) continue
      const d = Math.hypot(c.gx - x, c.gy - y)
      if (d < bd) {
        bd = d
        best = c
      }
    }
    return best
  }

  private aimedAtUs(m: ActorObs, x: number, y: number, z: number): boolean {
    const d = Math.hypot(m.pos[0] - x, m.pos[1] - y)
    if (d > (this.cfg.weapons.laser.range ?? 280) + 15) return false
    const yawTo = Math.atan2(y - m.pos[1], x - m.pos[0])
    if (Math.abs(angDiff(m.aim[0], yawTo)) > Math.atan2(6, d) + 0.06) return false
    return this.map.clear(m.pos[0], m.pos[1], m.pos[2] + 7, x, y, z + 4.5, 3, 2)
  }

  /** Is a mech standing at (px,py,pz) fully hidden from every shooter? Checks the body's centre line and both sides. */
  private bodyHidden(shooters: { pos: number[] }[], px: number, py: number, pz: number): boolean {
    for (const s of shooters) {
      const sx = s.pos[0], sy = s.pos[1], sz = s.pos[2] + 7
      const [ux, uy] = norm2(px - sx, py - sy)
      const ox = -uy * 2.3, oy = ux * 2.3
      const pts: [number, number, number][] = [
        [px, py, pz + 4.5], [px, py, pz + 8.6], [px, py, pz + 1],
        [px + ox, py + oy, pz + 4.5], [px - ox, py - oy, pz + 4.5],
      ]
      for (const [qx, qy, qz] of pts) if (this.map.clear(sx, sy, sz, qx, qy, qz, 3, 0.3)) return false
    }
    return true
  }

  /** A spot near us that can see `e` and has a fully hidden spot within a few metres (cached ~1 s). */
  private peekSpot(me: SelfObs, e: Enemy, t: number): [number, number] | null {
    if (this.peekId === e.id && t - this.peekT < 1 && !Number.isNaN(this.peekX)) return [this.peekX, this.peekY]
    this.peekId = e.id
    this.peekT = t
    this.peekX = NaN
    const m = this.map
    const [x, y] = me.pos
    const ez = e.pos[2] + 7
    type S = { x: number; y: number; vis: boolean; hid: boolean }
    const pts: S[] = []
    const radii = [0, 4, 8, 12, 16, 21, 27]
    for (const r of radii) {
      if (this.budget() < 4) break
      for (let k = 0; k < (r === 0 ? 1 : 12); k++) {
        const a = (k / 12) * TAU + r * 0.1
        const px = x + Math.cos(a) * r, py = y + Math.sin(a) * r
        const c = m.idx(px, py)
        if (!m.walk1[c]) continue
        const n = m.nodeOf(px, py)
        if (r > 0 && !m.isMain(n) && m.nearestNode(px, py, 1, (q) => m.isMain(q)) < 0) continue
        const pz = m.surf[c]
        const vis = m.clear(e.pos[0], e.pos[1], ez, px, py, pz + 4.5, 3, 1)
        const hid = !vis && this.bodyHidden([e], px, py, pz)
        pts.push({ x: px, y: py, vis, hid })
      }
    }
    const hidden = pts.filter((p) => p.hid)
    let best: S | null = null
    let bs = Infinity
    for (const p of pts) {
      if (!p.vis) continue
      let near = false
      for (const h of hidden) {
        const d = Math.hypot(h.x - p.x, h.y - p.y)
        if (d <= 6.5 && m.walkLine(p.x, p.y, h.x, h.y)) {
          near = true
          break
        }
      }
      if (!near) continue
      const s = Math.hypot(p.x - x, p.y - y)
      if (s < bs) {
        bs = s
        best = p
      }
    }
    if (!best) return null
    this.peekX = best.x
    this.peekY = best.y
    return [best.x, best.y]
  }

  /** Nearest spot we can walk to in a straight line that hides us from the chargers (cached briefly). */
  private duckSpot(t: number, chargers: ActorObs[], x: number, y: number, z: number): [number, number] | null {
    const m = this.map
    if (t - this.duckT < 0.35 && !Number.isNaN(this.duckX)) {
      const c = m.idx(this.duckX, this.duckY)
      if (this.bodyHidden(chargers, this.duckX, this.duckY, m.surf[c])) return [this.duckX, this.duckY]
    }
    this.duckT = t
    if (this.bodyHidden(chargers, x, y, z)) {
      this.duckX = x
      this.duckY = y
      return [x, y]
    }
    // prefer directions away from the shooter at equal distance
    const c0 = chargers[0]
    const away = Math.atan2(y - c0.pos[1], x - c0.pos[0])
    for (const r of [2.5, 4, 5.5, 7, 8.5, 10]) {
      if (this.budget() < 2) break
      let best: [number, number] | null = null
      let bs = Infinity
      for (let k = 0; k < 16; k++) {
        const a = (k / 16) * TAU
        const px = x + Math.cos(a) * r, py = y + Math.sin(a) * r
        const c = m.idx(px, py)
        if (!m.walk1[c] || !this.walkOk(x, y, px, py)) continue
        if (!this.bodyHidden(chargers, px, py, m.surf[c])) continue
        const s = Math.abs(angDiff(a, away))
        if (s < bs) {
          bs = s
          best = [px, py]
        }
      }
      if (best) {
        this.duckX = best[0]
        this.duckY = best[1]
        return best
      }
    }
    this.duckX = NaN
    return null
  }

  private visitKey(x: number, y: number): number {
    const i = Math.floor((x - this.map.minX) / 40)
    const j = Math.floor((y - this.map.minY) / 40)
    return j * 1000 + i
  }

  /** Remember which 40 m cells we have looked at (we see ~45 m around us). */
  private markVisited(x: number, y: number, t: number) {
    for (let dj = -1; dj <= 1; dj++)
      for (let di = -1; di <= 1; di++) {
        const cx = x + di * 30, cy = y + dj * 30
        this.visited.set(this.visitKey(cx, cy), t)
      }
  }

  /** A reachable spot in an area we have not seen yet, inside where the zone will be. */
  private pickExplore(obs: Observation) {
    const [x, y] = obs.self.pos
    const m = this.map
    const t = obs.t
    this.expT = t
    let best = -1
    let bs = Infinity
    for (let k = 0; k < 40; k++) {
      const r = 50 + this.rnd() * 170
      const ang = this.rnd() * TAU
      const sx = x + Math.cos(ang) * r, sy = y + Math.sin(ang) * r
      const n = m.nearestNode(sx, sy, 2, (q) => m.isMain(q))
      if (n < 0) continue
      const nx = m.nodeX(n), ny = m.nodeY(n)
      if (!this.safe(obs, nx, ny, 30, 25)) continue
      const seen = this.visited.get(this.visitKey(nx, ny))
      const age = seen === undefined ? 300 : t - seen
      // unseen ground first, nearer is better, drift toward the next circle
      const zc = obs.zone.next_center
      const s = -Math.min(age, 300) + r * 0.6 + Math.hypot(nx - zc[0], ny - zc[1]) * 0.15
      if (s < bs) {
        bs = s
        best = n
      }
    }
    if (best >= 0) {
      this.expX = m.nodeX(best)
      this.expY = m.nodeY(best)
    }
  }

  /** A sheltered spot inside the next circle, not far from where we are. */
  private pickHold(obs: Observation) {
    const [x, y] = obs.self.pos
    const m = this.map
    this.holdT = obs.t
    const zn = obs.zone
    const nr = zn.next_radius
    // aim for the next circle (or the current one early in a long hold)
    const cx = zn.next_center[0], cy = zn.next_center[1]
    const lim = Math.max(8, nr * 0.75)
    let best = -1
    let bs = Infinity
    for (let k = 0; k < 90; k++) {
      // sample around us and around the next centre
      const around = k % 2 === 0
      const r = around ? 5 + this.rnd() * 70 : this.rnd() * lim
      const ang = this.rnd() * TAU
      const sx = (around ? x : cx) + Math.cos(ang) * r
      const sy = (around ? y : cy) + Math.sin(ang) * r
      const n = m.nearestNode(sx, sy, 1, (q) => m.isMain(q))
      if (n < 0) continue
      const nx = m.nodeX(n), ny = m.nodeY(n)
      const dc = Math.hypot(nx - cx, ny - cy)
      if (dc > lim) continue
      const s = Math.hypot(nx - x, ny - y) * 0.6 + dc * 0.25 - Math.min(m.cover[n], 60) * P.holdCoverW
      if (s < bs) {
        bs = s
        best = n
      }
    }
    if (best >= 0) {
      // keep the old hold point unless the new one is clearly better
      if (!Number.isNaN(this.holdX) && this.safe(obs, this.holdX, this.holdY, 20, 6)) {
        const on = m.nodeOf(this.holdX, this.holdY)
        const os = Math.hypot(this.holdX - x, this.holdY - y) * 0.6 + Math.hypot(this.holdX - cx, this.holdY - cy) * 0.25 - Math.min(m.cover[on], 60) * P.holdCoverW
        if (os <= bs + 10) return
      }
      this.holdX = m.nodeX(best)
      this.holdY = m.nodeY(best)
    } else {
      this.holdX = cx
      this.holdY = cy
    }
  }
}
