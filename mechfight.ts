/**
 * mechfight/1 bot wrapper. One file, no dependencies: copy it next to your bot and run with
 * `bun bot.ts` or `node bot.ts` (Node 22.18+ runs TypeScript directly).
 *
 *   import { run, draw, type Hello, type Observation, type Action } from "./mechfight.ts"
 *   run((hello: Hello) => ({
 *     name: "my-bot",
 *     act(obs: Observation): Action {
 *       draw.label("hello") // optional debug drawing, only your team sees it
 *       return { move: [1, 0] }
 *     },
 *   }))
 *
 * What run() does for you (protocol.md section 2):
 * - answers `hello` with `ready` (your bot's `name`), then calls act() for each observation;
 * - if observations queue up, answers only the newest one;
 * - sets `tick` on your action (you don't have to);
 * - if your factory or act() throws, logs it to stderr and keeps going: it re-sends your last good
 *   move/aim/boost, so the mech keeps coasting instead of crashing;
 * - attaches whatever you drew with `draw` this tick as `action.debug`;
 * - exits on `end`.
 *
 * The types below are generated from mechfight.schema.json. Fields may be added to the protocol.
 */

// --- generated from mechfight.schema.json: do not edit ---
export type Action = {
  type?: "action"
  /** echo the observation tick; stale answers are dropped */
  tick: number
  /** desired horizontal direction in world frame, length <= 1 */
  move?: Vec2
  boost?: boolean
  jump?: boolean
  /** desired [yaw, pitch]; the torso slews toward it at turn_rate */
  aim?: Vec2
  fire?: [boolean, boolean]
  reload?: boolean
  /** index into the current offer's choices */
  pick?: number
  /** id of a weapon pickup within loot.pickup_radius */
  take?: number
  /** slot (0/1) to empty: with `take`, the slot the new weapon replaces; alone, just drops it */
  drop?: number
  /** debug drawing, shown only to your team; never affects the match */
  debug?: DebugDraw
}
export type ActorObs = {
  id: number
  kind: "mech" | "npc"
  pos: Vec3
  vel: Vec3
  aim: Vec2
  hp?: number
  max_hp?: number
  cubes?: number
  /** weapons held (empty slots omitted) */
  weapons?: WeaponKind[]
  /** laser charging (telegraph) */
  charging?: boolean
}
/** an instant shot (laser beam or lightning arc) by another mech, from its muzzle along `path` */
export type BeamObs = {
  by: number
  kind: "laser" | "lightning"
  path: Vec3[]
  age: number
}
/** Every message a bot sends the server. */
export type BotMessage = Ready | Action
/** A color: "#rgb" or "#rrggbb". Anything else is replaced by your mech's color. */
export type Color = string
export type Config = {
  match: Record<string, number>
  bot: Record<string, number>
  mech: MechStats
  vision: {
    range: number
    fov_deg: number
    near_range: number
    canopy_blocks: boolean
    show_enemy_hp: boolean
    memory_s?: number
  }
  sound: {
    bearing_noise_deg: number
    near: number
    mid: number
    step_speed: number
    step_every: number
    range: Record<string, number>
  }
  weapons: {
    gun: Record<string, number>
    mortar: Record<string, number>
    laser: Record<string, number>
    lightning: Record<string, number>
  }
  projectile: Record<string, number>
  cubes: {
    upgrade_thresholds: number[]
    [key: string]: number | number[]
  }
  crates: Record<string, number>
  loot: {
    pickup_radius: number
    equip_s: number
    floor_count: number
    floor_roof_fraction: number
    crate_weapon_chance: number
    death_drop: number
    weights: {
      gun: number
      mortar: number
      laser: number
      lightning: number
    }
  }
  npc: Record<string, number>
  /** Supply drops (optional: absent = none). */
  drops?: {
    times: number[]
    fall_s: number
    radius: number
    capture_s: number
    cubes: number
    repair: number
    spread: number
  }
  zone: {
    start_center: [number, number]
    start_radius: number
    phases: ZonePhase[]
  }
  replay: Record<string, number>
  /** Caps on action.debug: the server silently cuts each decision's drawing to these. */
  debug: DebugCaps
  upgrades: UpgradeDef[]
}
/** You took damage. */
export type DamagedEvent = {
  type: "damaged"
  amount: number
  /** gun, mortar, laser, lightning, npc or zone */
  weapon: string
  /** world-frame yaw toward the source (null for the zone) */
  bearing: number | null
  /** attacker id (null for the zone) */
  by: number | null
  /** it came from outside your front arc (x mech.rear_mult) */
  rear?: boolean
}
/** Caps on the debug drawing a bot may attach to an action. */
export type DebugCaps = {
  max_lines: number
  max_points: number
  max_text: number
  max_text_chars: number
  max_bytes: number
  max_match_bytes: number
  hold_s: number
}
/** Debug drawing for one decision. Stays on screen until your next decision that carries `debug`, or `debug.hold_s` seconds, whichever comes first; send `{}` to clear it early. The server truncates to the `[debug]` caps in hello.config (item counts, text length, bytes per decision, bytes per match) without telling you. It is recorded to a file only your team can download, never to the public replay, and has no effect on the match. */
export type DebugDraw = {
  lines?: DebugLine[]
  points?: DebugPoint[]
  text?: DebugText[]
  /** one short status line drawn above your mech */
  label?: string
}
/** A line segment from world point `a` to `b` (x east, y north, z up, meters). */
export type DebugLine = {
  a: Vec3
  b: Vec3
  color?: Color
}
/** A dot at a world point. `size` is its radius in screen pixels (1..12, default 4). */
export type DebugPoint = {
  at: Vec3
  color?: Color
  size?: number
}
/** A text label. With `at`, drawn at that world point; without, stacked above your mech. */
export type DebugText = {
  text: string
  at?: Vec3
  color?: Color
}
/** You captured supply drop `id`; `cubes` is your new total. */
export type DropEvent = {
  type: "drop"
  id: number
  cubes: number
}
/** A supply drop. Known to every mech from the moment it is called, like the zone. */
export type DropObs = {
  id: number
  pos: Vec3
  /** stand inside this, alone, for capture_s */
  radius: number
  /** seconds until it can be captured (0 = on the ground) */
  lands_in: number
  capture_s: number
  /** seconds the current holder has banked */
  progress: number
  /** mech currently capturing */
  holder: number | null
}
/** A weapon left your `slot` and lies on the ground as pickup `id`. */
export type DroppedEvent = {
  type: "dropped"
  id: number
  kind: WeaponKind
  slot: number
}
export type Effect = {
  path: string
  mul?: number
  add?: number
}
/** A signature effect landed on you. */
export type EffectEvent = {
  type: "effect"
  effect: EffectName
  by: number | null
}
/** cookoff: laser on a mortar carrier. short: lightning on a laser carrier. concuss: any mortar blast. */
export type EffectName = "cookoff" | "short" | "concuss"
export type EndMsg = {
  type: "end"
  place: number
  kills: number
}
export type GameEvent = DamagedEvent | HitEvent | EffectEvent | DropEvent | KillEvent | PickupEvent | UpgradeEvent | TookEvent | DroppedEvent | TakeFailedEvent
export type Hello = {
  type: "hello"
  protocol: "mechfight/1"
  /** your mech id, 0..15 */
  slot: number
  /** per-bot seed, use it for your own RNG to stay reproducible */
  seed: number
  map: {
    dir: string
    files: string[]
  }
  config: Config
}
/** You damaged `target` (a mech, NPC or crate id). */
export type HitEvent = {
  type: "hit"
  target: number
  amount: number
  weapon: string
  effect?: EffectName
  rear?: boolean
}
export type ItemObs = {
  id: number
  pos: Vec3
  hp?: number
}
/** You killed a mech. */
export type KillEvent = {
  type: "kill"
  target: number
}
export type MechStats = Record<string, number>
export type Observation = {
  type: "obs"
  tick: number
  /** match time, seconds */
  t: number
  /** mechs alive (not NPCs) */
  alive: number
  self: SelfObs
  /** visible mechs and NPCs */
  mechs: ActorObs[]
  projectiles: ProjectileObs[]
  crates: ItemObs[]
  cubes: ItemObs[]
  /** visible weapons on the ground */
  pickups: PickupObs[]
  sounds: SoundObs[]
  /** laser beams and lightning arcs by other mechs that you saw (any point of the path in view) since your last observation */
  beams: BeamObs[]
  events: GameEvent[]
  zone: ZoneObs
  offer: Offer | null
  /** supply drops called or on the ground */
  drops: DropObs[]
}
export type Offer = {
  id: number
  choices: OfferChoice[]
  expires: number
}
export type OfferChoice = {
  id: string
  name: string
  desc: string
}
/** You picked up a cube; `cubes` is your new total. */
export type PickupEvent = {
  type: "pickup"
  cubes: number
}
/** A weapon lying on the ground. Take it with the `take` action. */
export type PickupObs = {
  id: number
  kind: WeaponKind
  pos: Vec3
}
export type ProjectileObs = {
  id: number
  kind: "gun" | "mortar" | "npc"
  pos: Vec3
  vel: Vec3
}
export type Ready = {
  type: "ready"
  name?: string
}
export type SelfObs = {
  id: number
  pos: Vec3
  vel: Vec3
  /** [yaw, pitch] actual (slewed) aim */
  aim: Vec2
  hp: number
  max_hp: number
  energy: number
  max_energy: number
  grounded: boolean
  cubes: number
  upgrades: string[]
  /** null = empty slot */
  weapons: [WeaponObs | null, WeaponObs | null]
  in_zone: boolean
  /** your current [mech] numbers after upgrades */
  stats: Record<string, number>
}
/** Every message the server sends a bot. */
export type ServerMessage = Hello | Observation | EndMsg
export type SoundObs = {
  kind: string
  bearing: number
  dist: "near" | "mid" | "far"
  age: number
}
/** Your `take` didn't happen. */
export type TakeFailedEvent = {
  type: "take_failed"
  id: number
  reason: "gone" | "range" | "have" | "full" | "busy"
}
/** Your `take` succeeded: pickup `id` is now in `slot`. */
export type TookEvent = {
  type: "took"
  id: number
  kind: WeaponKind
  slot: number
}
export type UpgradeDef = {
  id: string
  name: string
  desc: string
  requires?: WeaponKind
  max: number
  effects: Effect[]
}
/** An upgrade was applied. */
export type UpgradeEvent = {
  type: "upgrade"
  id: string
}
export type Vec2 = [number, number]
export type Vec3 = [number, number, number]
export type WeaponKind = "gun" | "mortar" | "laser" | "lightning"
export type WeaponObs = {
  kind: WeaponKind
  /** seconds until it can fire again */
  cooldown: number
  /** gun only */
  ammo?: number
  /** gun only: seconds of reload left (0 = not reloading) */
  reload?: number
  /** laser only: seconds charged so far (0 = not charging); it fires at weapons.laser.charge, or on release after min_charge */
  charge?: number
  /** this weapon's [weapons.<kind>] numbers after upgrades */
  stats: Record<string, number>
}
export type ZoneObs = {
  center: Vec2
  radius: number
  next_center: Vec2
  next_radius: number
  /** match time (s) the current shrink starts (or started) */
  shrink_start: number
  shrink_end: number
  dps: number
}
export type ZonePhase = {
  hold_s: number
  shrink_s: number
  radius: number
  dps: number
}
// --- end generated ---

export type BotAction = Omit<Action, "tick"> & { tick?: number }
export type Bot = { name?: string; act(obs: Observation): BotAction }

/** print to stderr: stdout is the protocol. Your team sees stderr after each match. */
export const log = (...args: unknown[]) => console.error(...args)

/**
 * Debug drawing for the current decision. World coordinates (x east, y north, z up, meters).
 * Colors are "#rgb" or "#rrggbb"; without one, your mech's color. The server caps it
 * (hello.config.debug); this helper stops adding at the same caps so nothing is wasted.
 */
export class Draw {
  caps: Partial<DebugCaps> = {}
  private d: DebugDraw = {}
  private room(list: unknown[] | undefined, cap: number | undefined) {
    return (list?.length ?? 0) < (cap ?? Infinity)
  }
  line(a: Vec3, b: Vec3, color?: Color) {
    if (this.room(this.d.lines, this.caps.max_lines)) (this.d.lines ??= []).push(color ? { a, b, color } : { a, b })
  }
  point(at: Vec3, color?: Color, size?: number) {
    if (this.room(this.d.points, this.caps.max_points)) (this.d.points ??= []).push({ at, ...(color && { color }), ...(size !== undefined && { size }) })
  }
  /** Text at a world point, or (without `at`) stacked above your mech. */
  text(text: string, at?: Vec3, color?: Color) {
    if (this.room(this.d.text, this.caps.max_text)) (this.d.text ??= []).push({ text: String(text), ...(at && { at }), ...(color && { color }) })
  }
  /** One short status line above your mech. */
  label(text: string) {
    this.d.label = String(text)
  }
  clear() {
    this.d = {}
  }
  take(): DebugDraw | undefined {
    const d = this.d
    this.d = {}
    return Object.keys(d).length ? d : undefined
  }
}
export const draw = new Draw()

const send = (o: unknown) => process.stdout.write(JSON.stringify(o) + "\n")

/** Run a bot until the match ends. `factory(hello)` returns `{ name?, act(obs) }`. */
export function run(factory: (hello: Hello) => Bot) {
  let bot: Bot | null = null
  let last: BotAction = {}
  let errors = 0
  let buf = ""
  let queued = false
  const handle = (lines: string[]) => {
    let latest: Observation | null = null
    for (const line of lines) {
      let msg: ServerMessage
      try {
        msg = JSON.parse(line)
      } catch {
        continue
      }
      if (msg.type === "hello") {
        draw.caps = msg.config.debug ?? {}
        try {
          bot = factory(msg)
        } catch (e) {
          log(e)
        }
        send({ type: "ready", name: String(bot?.name ?? "bot").slice(0, 32) })
      } else if (msg.type === "obs") latest = msg
      else if (msg.type === "end") process.exit(0)
    }
    if (!latest) return
    let action: BotAction = {}
    draw.clear()
    if (bot)
      try {
        action = { ...bot.act(latest) }
        last = { move: action.move, aim: action.aim, boost: action.boost }
      } catch (e) {
        errors++
        if (errors <= 5 || errors % 100 === 0) log(`act() failed at tick ${latest.tick} (${errors} so far):`, e)
        action = { ...last }
      }
    action.tick = latest.tick
    const d = draw.take()
    if (d && !action.debug) action.debug = d
    send(action)
  }
  process.stdin.setEncoding("utf8")
  process.stdin.on("data", (chunk: string) => {
    buf += chunk
    if (queued) return
    // Handle on the next turn, so every chunk that arrived while act() ran is read first and a
    // backlog collapses to the newest observation.
    queued = true
    setImmediate(flush)
  })
  const flush = () => {
    queued = false
    const lines = buf.split("\n")
    buf = lines.pop() ?? ""
    handle(lines.filter((l) => l.trim()))
  }
  process.stdin.on("end", () => {
    buf += "\n"
    flush()
  })
}
