// A* path planning on the 2 m nav graph, path following and stuck recovery.
import type { GameMap } from "./map.ts"
import { now } from "./util.ts"

const NEI8: [number, number][] = [
  [1, 0], [-1, 0], [0, 1], [0, -1], [1, 1], [1, -1], [-1, 1], [-1, -1],
]
const SQ2 = Math.SQRT2

export class Planner {
  map: GameMap
  private g: Float32Array
  private from: Int32Array
  private stamp: Uint32Array
  private closed: Uint32Array
  private gen = 1
  private heapN: Int32Array
  private heapF: Float64Array
  private hs = 0
  nOffs: number[]

  constructor(map: GameMap) {
    this.map = map
    const M = map.NX * map.NY
    this.g = new Float32Array(M)
    this.from = new Int32Array(M)
    this.stamp = new Uint32Array(M)
    this.closed = new Uint32Array(M)
    this.heapN = new Int32Array(M * 2)
    this.heapF = new Float64Array(M * 2)
    this.nOffs = NEI8.map(([di, dj]) => dj * map.NX + di)
  }

  private push(n: number, f: number) {
    let i = this.hs++
    const H = this.heapN, F = this.heapF
    while (i > 0) {
      const p = (i - 1) >> 1
      if (F[p] <= f) break
      H[i] = H[p]
      F[i] = F[p]
      i = p
    }
    H[i] = n
    F[i] = f
  }

  private pop(): number {
    const H = this.heapN, F = this.heapF
    const top = H[0]
    const n = H[--this.hs]
    const f = F[this.hs]
    let i = 0
    const size = this.hs
    for (;;) {
      let c = 2 * i + 1
      if (c >= size) break
      if (c + 1 < size && F[c + 1] < F[c]) c++
      if (F[c] >= f) break
      H[i] = H[c]
      F[i] = F[c]
      i = c
    }
    H[i] = n
    F[i] = f
    return top
  }

  /** A* from start to goal node. Returns node list (start..goal) or null. `cost` adds per-node penalty. */
  plan(start: number, goal: number, maxExpand = 60000, cost?: (n: number) => number, budgetMs = 12): number[] | null {
    const m = this.map
    if (start < 0 || goal < 0) return null
    if (start === goal) return [start]
    this.gen++
    if (this.gen > 0xfffffff0) {
      this.stamp.fill(0)
      this.closed.fill(0)
      this.gen = 1
    }
    const gen = this.gen
    const NX = m.NX
    const gx = goal % NX, gy = Math.floor(goal / NX)
    const h = (n: number) => {
      const dx = Math.abs((n % NX) - gx)
      const dy = Math.abs(Math.floor(n / NX) - gy)
      return (dx + dy + (SQ2 - 2) * Math.min(dx, dy)) * 2
    }
    this.hs = 0
    this.g[start] = 0
    this.stamp[start] = gen
    this.from[start] = -1
    this.push(start, h(start))
    let expanded = 0
    let best = start
    let bestH = h(start)
    const t0 = now()
    while (this.hs > 0) {
      const n = this.pop()
      if (this.closed[n] === gen) continue
      this.closed[n] = gen
      if (n === goal) return this.unwind(goal)
      const hn = h(n)
      if (hn < bestH) {
        bestH = hn
        best = n
      }
      if (++expanded > maxExpand) break
      if ((expanded & 1023) === 0 && now() - t0 > budgetMs) break
      const mask = m.navMask[n]
      const gn = this.g[n]
      for (let k = 0; k < 8; k++) {
        if (!(mask & (1 << k))) continue
        const nb = n + this.nOffs[k]
        if (this.closed[nb] === gen) continue
        let c = gn + (k < 4 ? 2 : 2 * SQ2)
        if (cost) c += cost(nb)
        if (this.stamp[nb] !== gen || c < this.g[nb]) {
          this.stamp[nb] = gen
          this.g[nb] = c
          this.from[nb] = n
          this.push(nb, c + h(nb))
        }
      }
    }
    // partial path toward the closest node reached
    return best !== start ? this.unwind(best) : null
  }

  private unwind(n: number): number[] {
    const out: number[] = []
    while (n >= 0) {
      out.push(n)
      n = this.from[n]
    }
    out.reverse()
    return out
  }
}

/** Keeps a path to a goal, replans when needed and returns a steering direction. */
export class Navigator {
  map: GameMap
  planner: Planner
  path: number[] = []
  idx = 0
  goalX = NaN
  goalY = NaN
  goalNode = -1
  lastPlan = -1e9
  partial = false
  // stuck detection
  private hist: { t: number; x: number; y: number; cmd: boolean }[] = []
  stuckUntil = -1
  stuckDir: [number, number] = [0, 0]
  stuckCount = 0

  constructor(map: GameMap) {
    this.map = map
    this.planner = new Planner(map)
  }

  setGoal(x: number, y: number, t: number, px: number, py: number, force = false) {
    const m = this.map
    const moved = Math.hypot(x - this.goalX, y - this.goalY)
    const needs = force || !(moved < 6) || this.path.length === 0 || t - this.lastPlan > (this.partial ? 1.0 : 2.5)
    if (!needs) return
    this.goalX = x
    this.goalY = y
    const goal = m.nearestNode(x, y, 25, (n) => m.isMain(n))
    const start = m.nearestNode(px, py, 4, (n) => m.navOk[n] === 1 && m.navMask[n] !== 0)
    this.lastPlan = t
    this.goalNode = goal
    if (goal < 0 || start < 0) {
      this.path = []
      return
    }
    const p = this.planner.plan(start, goal)
    this.partial = !p || p[p.length - 1] !== goal
    this.path = p ?? []
    this.idx = 0
  }

  /** Unit steering vector toward the goal (straight line if no path). */
  steer(px: number, py: number): [number, number] {
    const m = this.map
    if (this.path.length === 0) {
      if (Number.isNaN(this.goalX)) return [0, 0]
      const dx = this.goalX - px, dy = this.goalY - py
      const l = Math.hypot(dx, dy)
      return l > 1 ? [dx / l, dy / l] : [0, 0]
    }
    // advance past reached waypoints
    while (this.idx < this.path.length - 1) {
      const n = this.path[this.idx]
      if (Math.hypot(m.nodeX(n) - px, m.nodeY(n) - py) < 3) this.idx++
      else break
    }
    // look ahead: farthest waypoint reachable in a straight walkable line (string pulling)
    let target = this.idx
    const lim = Math.min(this.path.length - 1, this.idx + 12)
    for (let k = lim; k > this.idx; k--) {
      const n = this.path[k]
      if (m.walkLine(px, py, m.nodeX(n), m.nodeY(n))) {
        target = k
        break
      }
    }
    const n = this.path[target]
    let tx = m.nodeX(n), ty = m.nodeY(n)
    if (target === this.path.length - 1 && !this.partial) {
      tx = this.goalX
      ty = this.goalY
    }
    const dx = tx - px, dy = ty - py
    const l = Math.hypot(dx, dy)
    if (l < 0.8) return [0, 0]
    return [dx / l, dy / l]
  }

  remaining(px: number, py: number): number {
    const m = this.map
    if (this.path.length === 0) return Math.hypot(this.goalX - px, this.goalY - py)
    let d = 0
    let lx = px, ly = py
    for (let k = this.idx; k < this.path.length; k++) {
      const n = this.path[k]
      d += Math.hypot(m.nodeX(n) - lx, m.nodeY(n) - ly)
      lx = m.nodeX(n)
      ly = m.nodeY(n)
    }
    return d
  }

  /** Record motion; returns true while a stuck-escape manoeuvre should run. */
  updateStuck(t: number, x: number, y: number, commanded: boolean, wantDir: [number, number], rnd: () => number): boolean {
    this.hist.push({ t, x, y, cmd: commanded })
    while (this.hist.length > 0 && this.hist[0].t < t - 1.6) this.hist.shift()
    if (t < this.stuckUntil) return true
    const h0 = this.hist[0]
    if (this.hist.length >= 12 && this.hist.every((h) => h.cmd) && Math.hypot(x - h0.x, y - h0.y) < 1.5) {
      this.stuckCount++
      const side = rnd() < 0.5 ? 1 : -1
      // alternate: slide sideways, then jet forward
      if (this.stuckCount % 2 === 1) this.stuckDir = [-wantDir[1] * side, wantDir[0] * side]
      else this.stuckDir = wantDir
      this.stuckUntil = t + 0.8
      this.hist.length = 0
      this.lastPlan = -1e9 // replan after
      return true
    }
    return false
  }
}
