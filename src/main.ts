// Entry point: build the map at hello, then let the Brain decide every tick.
// If anything throws, fall back to a simple "walk to the zone and shoot" policy.
import { type BotAction, type Hello, log, type Observation, run } from "../mechfight.ts"
import { Brain, P } from "./brain.ts"
import { GameMap } from "./map.ts"

function fallback(obs: Observation): BotAction {
  const me = obs.self
  const [x, y, z] = me.pos
  const g = obs.zone.next_center
  const yaw = Math.atan2(g[1] - y, g[0] - x)
  const far = Math.hypot(g[0] - x, g[1] - y) > obs.zone.next_radius * 0.5
  const act: BotAction = { move: far ? [Math.cos(yaw), Math.sin(yaw)] : [0, 0], aim: [yaw, 0], boost: !me.in_zone }
  if (obs.offer) act.pick = 0
  let best: number[] | null = null
  let bd = Infinity
  for (const m of obs.mechs) {
    if (m.kind !== "mech") continue
    const d = Math.hypot(m.pos[0] - x, m.pos[1] - y)
    if (d < bd) {
      bd = d
      best = [m.pos[0] + m.vel[0] * (d / 252), m.pos[1] + m.vel[1] * (d / 252), m.pos[2]]
    }
  }
  if (best && bd < 230) {
    act.aim = [Math.atan2(best[1] - y, best[0] - x), Math.atan2(best[2] + 4.5 - z - 7, bd)]
    act.fire = [true, true]
  }
  return act
}

run((hello: Hello) => {
  let brain: Brain | null = null
  try {
    const t0 = performance.now()
    const map = new GameMap(hello.map.dir, hello.config.mech)
    brain = new Brain(hello, map)
    log(`map built in ${(performance.now() - t0).toFixed(0)} ms`)
  } catch (e) {
    log("init failed, using fallback", e)
  }
  let errors = 0
  return {
    name: P.name,
    act(obs: Observation): BotAction {
      if (brain) {
        try {
          return brain.act(obs)
        } catch (e) {
          errors++
          if (errors <= 5 || errors % 200 === 0) log(`brain error #${errors} at t=${obs.t}:`, e)
        }
      }
      return fallback(obs)
    },
  }
})
