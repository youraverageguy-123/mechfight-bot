// Small math helpers shared by every module.

export const TAU = Math.PI * 2

export function wrapAngle(a: number): number {
  a = (a + Math.PI) % TAU
  if (a < 0) a += TAU
  return a - Math.PI
}

export function angDiff(a: number, b: number): number {
  return wrapAngle(a - b)
}

export function clamp(v: number, lo: number, hi: number): number {
  return v < lo ? lo : v > hi ? hi : v
}

export function dist2d(ax: number, ay: number, bx: number, by: number): number {
  return Math.hypot(bx - ax, by - ay)
}

export function norm2(x: number, y: number): [number, number] {
  const l = Math.hypot(x, y)
  return l > 1e-9 ? [x / l, y / l] : [0, 0]
}

/** Deterministic RNG (mulberry32). */
export function makeRng(seed: number): () => number {
  let s = (seed >>> 0) || 0x9e3779b9
  return () => {
    s = (s + 0x6d2b79f5) >>> 0
    let t = s
    t = Math.imul(t ^ (t >>> 15), t | 1)
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61)
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296
  }
}

export function now(): number {
  return performance.now()
}
