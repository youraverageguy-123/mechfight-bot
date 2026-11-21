// Ballistics: lead solutions for the gun (straight, fast) and the mortar (parabolic).
import type { GameMap } from "./map.ts"

export type AimSol = { yaw: number; pitch: number; t: number; px: number; py: number; pz: number }

/** Lead a straight projectile from muzzle M (shot spawns `offset` m out) to a target moving at V. */
export function leadStraight(
  mx: number, my: number, mz: number,
  tx: number, ty: number, tz: number,
  vx: number, vy: number, vz: number,
  speed: number, offset: number, floorZ: number, gravity: number,
): AimSol {
  let t = Math.max(0, (Math.hypot(tx - mx, ty - my, tz - mz) - offset) / speed)
  let px = tx, py = ty, pz = tz
  for (let k = 0; k < 4; k++) {
    px = tx + vx * t
    py = ty + vy * t
    pz = tz + vz * t
    if (Math.abs(vz) > 0.5) pz -= 0.5 * gravity * t * t // airborne targets fall
    if (pz < floorZ) pz = floorZ
    const d = Math.hypot(px - mx, py - my, pz - mz)
    t = Math.max(0, (d - offset) / speed)
  }
  const yaw = Math.atan2(py - my, px - mx)
  const pitch = Math.atan2(pz - mz, Math.hypot(px - mx, py - my))
  return { yaw, pitch, t, px, py, pz }
}

/** Pitch(es) to hit horizontal distance D and height H with speed v under gravity g. */
function ballisticPitches(D: number, H: number, v: number, g: number): number[] {
  const v2 = v * v
  const disc = v2 * v2 - g * (g * D * D + 2 * H * v2)
  if (disc < 0) return []
  const s = Math.sqrt(disc)
  return [Math.atan2(v2 - s, g * D), Math.atan2(v2 + s, g * D)]
}

/** Does the shell path from L with (yaw,pitch) reach the end without hitting solid geometry? */
function arcClear(map: GameMap, lx: number, ly: number, lz: number, yaw: number, pitch: number, v: number, g: number, T: number): boolean {
  const cx = Math.cos(yaw) * Math.cos(pitch) * v
  const cy = Math.sin(yaw) * Math.cos(pitch) * v
  const cz = Math.sin(pitch) * v
  const n = Math.max(6, Math.ceil(T / 0.08))
  for (let k = 1; k < n - 1; k++) {
    const t = (T * k) / n
    const x = lx + cx * t, y = ly + cy * t, z = lz + cz * t - 0.5 * g * t * t
    const c = map.idx(x, y)
    if (z < map.terrain[c] - 0.2) return false
    if (z > map.sBase[c] && z < map.sTop[c]) return false
  }
  return true
}

/** Mortar aim at a (possibly moving) ground target. Returns null when out of reach. */
export function leadMortar(
  map: GameMap,
  lx: number, ly: number, lz: number,
  tx: number, ty: number, tz: number,
  vx: number, vy: number,
  v: number, g: number, minPitch: number, maxPitch: number,
): AimSol | null {
  let px = tx, py = ty
  let best: AimSol | null = null
  for (let iter = 0; iter < 3; iter++) {
    const D = Math.hypot(px - lx, py - ly)
    const H = tz - lz
    const pit = ballisticPitches(D, H, v, g)
    if (pit.length === 0) return null
    const yaw = Math.atan2(py - ly, px - lx)
    best = null
    for (const p of pit) {
      if (p < minPitch || p > maxPitch) continue
      const T = D / (v * Math.cos(p))
      if (iter === 2 && !arcClear(map, lx, ly, lz, yaw, p, v, g, T)) continue
      best = { yaw, pitch: p, t: T, px, py, pz: tz }
      break
    }
    if (!best) return null
    px = tx + vx * best.t
    py = ty + vy * best.t
  }
  return best
}
