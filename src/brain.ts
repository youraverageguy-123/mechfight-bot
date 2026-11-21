// Decision making: world memory, target choice, weapons, movement modes.
import type { ActorObs, BotAction, Hello, Observation, ProjectileObs, SelfObs, WeaponKind, WeaponObs } from "../mechfight.ts"
import { draw } from "../mechfight.ts"
import { leadMortar, leadStraight, type AimSol } from "./aim.ts"
import type { GameMap } from "./map.ts"
import { Navigator } from "./nav.ts"
import { angDiff, clamp, makeRng, norm2, now, TAU, wrapAngle } from "./util.ts"

// ---- tunables (A/B test these) ----
export const P = {
  retreatFrac: 0.38, // retreat below this HP fraction
  recoverFrac: 0.85, // stop retreating above this
  weaponRank: { laser: 3, lightning: 2, mortar: 1, gun: 0 } as Record<string, number>,
  lightningReserve: 12, // energy kept back
  farmUntil: 230, // farm crates / drones before this match time
  farmMinHp: 0.55,
  exploreUntil: 240, // look for a second weapon until this match time
  multiRetreatFrac: 0.45,
  kite: false, // boost away from lightning carriers when we lack lightning (sim: no gain)
  gunBand: [35, 120] as [number, number],
  lightningBand: [14, 32] as [number, number],
  laserBand: [60, 200] as [number, number],
  upgrades: ["armor", "plating", "nanites", "gun_rate", "gun_damage", "lightning_efficiency", "laser_damage", "laser_charge",
    "gyros", "mortar_reload", "lightning_range", "lightning_chain", "gun_mag", "gun_velocity", "sensors", "servos", "capacitor",
    "reactor", "jets", "mortar_splash", "mortar_velocity", "laser_pierce"],
  dropMinDist: 70, // no enemy this close to a drop
  name: "ohio-ironclad",
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
    this.rnd = makeRng(hello.seed)
    this.myId = hello.slot
  }

  // ------------------------------------------------------------------ memory
  private remember(obs: Observation) {
    const t = obs.t
    for (const m of obs.mechs) {
      if (m.kind !== "mech") continue
      const e = this.enemies.get(m.id)
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
      }
      this.enemies.set(m.id, ne)
    }
    for (const [id, e] of this.enemies) if (t - e.seen > 12) this.enemies.delete(id)
    const vis = (map: Map<number, Known>, list: { id: number; pos: number[]; kind?: string; hp?: number }[]) => {
      for (const it of list) map.set(it.id, { id: it.id, kind: it.kind, pos: [it.pos[0], it.pos[1], it.pos[2]], seen: t, hp: it.hp })
    }
    vis(this.pickups, obs.pickups)
    vis(this.crates, obs.crates)
    vis(this.cubes, obs.cubes)
    // forget remembered items that should be visible (close) but are not listed
    const [x, y] = obs.self.pos
    const forget = (map: Map<number, Known>, r: number) => {
      for (const [id, k] of map) if (k.seen < t && Math.hypot(k.pos[0] - x, k.pos[1] - y) < r) map.delete(id)
    }
    forget(this.pickups, 30)
    forget(this.crates, 30)
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
      } else if (ev.type === "hit") this.lastDealt = t
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
    const engaged = mechs.filter((e) => (e.los && e.d < 220) || t - e.lastHurtUs < 4 || e.d < 50)
    const recentHurt = t - this.lastHurt < 3

    // ---- choose a target
    let target: Target | null = null
    let bestScore = Infinity
    for (const e of mechs) {
      if (!e.los || e.d > maxDirect + 20) continue
      let s = e.d * 0.6 + e.hp * 0.06
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
        if (e.d < 28 || e.d > 230 || t - e.seen > 4) continue
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
    if (zoneUrgent) {
      const dx = x - zn.next_center[0], dy = y - zn.next_center[1]
      const dl = Math.hypot(dx, dy) || 1
      const rr = Math.max(0, zn.next_radius - Math.min(30, zn.next_radius * 0.4))
      goal = [zn.next_center[0] + (dx / dl) * rr, zn.next_center[1] + (dy / dl) * rr]
      goalBoost = !me.in_zone || hit < eta + 3
      this.mode = "zone"
    }

    // retreat to cover
    if (!goal && this.retreating && fighting) {
      if (t - this.coverT > 1.5 || Number.isNaN(this.coverX)) this.pickCover(obs, engaged)
      if (!Number.isNaN(this.coverX)) {
        goal = [this.coverX, this.coverY]
        goalBoost = me.energy > 25
        this.mode = "retreat"
      }
    }

    // supply drop
    if (!goal && !this.retreating) {
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
    if (!goal && fighting && focus) {
      const band = this.band(me)
      if (focus.los) fightMove = true
      else if (!this.retreating && hpFrac > 0.55 && t - focus.seen < 5 && focus.d < 160) {
        goal = [focus.pos[0], focus.pos[1]]
        this.mode = "hunt"
      } else fightMove = true
      void band
    }

    // cubes nearby
    if (!goal && !fightMove) {
      let best: Known | null = null
      let bd = fighting ? 12 : 90
      for (const c of this.cubes.values()) {
        const d = Math.hypot(c.pos[0] - x, c.pos[1] - y)
        if (d < bd && Math.abs(c.pos[2] - this.map.surfAt(c.pos[0], c.pos[1])) < 3 && this.safe(obs, c.pos[0], c.pos[1], d / walk + 5, 5)) {
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
    if (!goal && !fighting) {
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
        if (!this.safe(obs, p.pos[0], p.pos[1], d / walk + 5, 5)) continue
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
    if (!goal && farmOk && t < P.farmUntil && !target) {
      let best: Known | null = null
      let bd = 150
      for (const c of this.crates.values()) {
        const d = Math.hypot(c.pos[0] - x, c.pos[1] - y)
        if (d < bd && Math.abs(c.pos[2] - this.map.surfAt(c.pos[0], c.pos[1])) < 2.5 && this.safe(obs, c.pos[0], c.pos[1], d / walk + 5, 8)) {
          bd = d
          best = c
        }
      }
      if (best && bd > 25) {
        goal = [best.pos[0], best.pos[1]]
        this.mode = "farm"
      }
    }

    // explore for a weapon while we only have the gun
    this.markVisited(x, y, t)
    if (!goal && !fightMove && !fighting && !target && !slot1 && hpFrac > 0.4 && t < P.exploreUntil) {
      const reached = Math.hypot(this.expX - x, this.expY - y) < 12
      if (Number.isNaN(this.expX) || reached || t - this.expT > 30 || !this.safe(obs, this.expX, this.expY, 25, 15)) this.pickExplore(obs)
      if (!Number.isNaN(this.expX)) {
        goal = [this.expX, this.expY]
        this.mode = "explore"
      }
    }

    // hold near cover inside the next circle
    if (!goal && !fightMove) {
      if (t - this.holdT > 6 || Number.isNaN(this.holdX) || !this.safe(obs, this.holdX, this.holdY, 20, 6)) this.pickHold(obs)
      if (!Number.isNaN(this.holdX)) {
        if (Math.hypot(this.holdX - x, this.holdY - y) > 3) goal = [this.holdX, this.holdY]
        this.mode = target ? "farm-shoot" : "hold"
      }
    }

    if (goal) {
      this.nav.setGoal(goal[0], goal[1], t, x, y)
      move = this.nav.steer(x, y)
      boost = goalBoost
    }

    if (fightMove && focus) {
      this.mode = this.retreating ? "retreat-fight" : "fight"
      move = this.fightMove(me, focus, t)
      boost = me.energy > 60 && !lightning
      // kite lightning carriers we can't match up close
      if (P.kite && !lightning && focus.weapons.includes("lightning") && focus.d < 60 && me.energy > 8) boost = true
    } else if (fighting && goal && focus && this.mode !== "zone") {
      // moving with enemies around: blend in a bit of strafe
      const by = Math.atan2(focus.pos[1] - y, focus.pos[0] - x)
      this.updateStrafe(t)
      const sx = -Math.sin(by) * this.strafeSign, sy = Math.cos(by) * this.strafeSign
      const m2 = norm2(move[0] + sx * 0.35, move[1] + sy * 0.35)
      if (this.map.walkLine(x, y, x + m2[0] * 3, y + m2[1] * 3)) move = m2
    }

    // dodge incoming fire
    const dodge = this.dodge(obs, me)
    if (dodge) {
      const m2 = norm2(move[0] * 0.4 + dodge[0], move[1] * 0.4 + dodge[1])
      if (this.map.walkLine(x, y, x + m2[0] * 3, y + m2[1] * 3)) {
        move = m2
        if (me.energy > 15) boost = true
      }
    }

    // stuck recovery
    const commanded = Math.hypot(move[0], move[1]) > 0.5
    if (this.nav.updateStuck(t, x, y, commanded && this.mode !== "fight", move, this.rnd)) {
      move = this.nav.stuckDir
      if (me.grounded && this.nav.stuckCount % 2 === 0) jump = true
      if (!me.grounded && me.energy > 10) boost = true
    }
    // stranded off the main nav graph (stadium bowl, a roof, a pit): head for the nearest main
    // node, jumping and jetting over whatever is in the way
    const isMain = (n: number) => this.map.isMain(n)
    const onMain = this.map.nearestNode(x, y, 1, isMain) >= 0
    if (!onMain && !fightMove && (!goal || this.nav.partial || this.nav.path.length === 0)) {
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
    let radial = 0
    if (this.retreating) radial = -0.8
    else if (e.d > band[1]) radial = 0.8
    else if (e.d < band[0]) radial = -0.7
    // enemy holds lightning and we don't: keep out of its arc
    if (!this.weapon(me, "lightning") && e.weapons.includes("lightning") && e.d < 55) radial = -0.9
    this.updateStrafe(t)
    for (let attempt = 0; attempt < 2; attempt++) {
      const sx = -fy * this.strafeSign, sy = fx * this.strafeSign
      const m = norm2(sx + fx * radial, sy + fy * radial)
      if (this.map.walkLine(x, y, x + m[0] * 4, y + m[1] * 4)) return m
      this.strafeSign = -this.strafeSign
      this.strafeFlip = t + 0.6 + this.rnd() * 0.5
    }
    // boxed in: just move radially if possible
    const m = norm2(fx * (radial || 0.5), fy * (radial || 0.5))
    if (this.map.walkLine(x, y, x + m[0] * 4, y + m[1] * 4)) return m
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
    for (const r of radii)
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
      const s = Math.hypot(nx - x, ny - y) * 0.6 + dc * 0.25 - Math.min(m.cover[n], 60) * 1.2
      if (s < bs) {
        bs = s
        best = n
      }
    }
    if (best >= 0) {
      // keep the old hold point unless the new one is clearly better
      if (!Number.isNaN(this.holdX) && this.safe(obs, this.holdX, this.holdY, 20, 6)) {
        const on = m.nodeOf(this.holdX, this.holdY)
        const os = Math.hypot(this.holdX - x, this.holdY - y) * 0.6 + Math.hypot(this.holdX - cx, this.holdY - cy) * 0.25 - Math.min(m.cover[on], 60) * 1.2
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
