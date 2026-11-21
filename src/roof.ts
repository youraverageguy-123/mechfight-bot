// Rooftops: which roofs are worth camping on, where to stand on them, and where to climb up.
import type { GameMap } from "./map.ts"

export type Climb = { gx: number; gy: number; gz: number; ex: number; ey: number; ez: number }
export type Roof = {
  comp: number
  campNode: number
  campX: number
  campY: number
  campZ: number
  edgeDist: number // metres from the camp spot to the nearest roof edge
  climbs: Climb[]
}

/** Distance (in nav nodes) from each node to the edge of its own component. */
function edgeDistance(m: GameMap): Int32Array {
  const NX = m.NX
  const M = NX * m.NY
  const dist = new Int32Array(M).fill(-1)
  const q = new Int32Array(M)
  let qn = 0
  const D4 = [1, -1, NX, -NX]
  for (let n = NX; n < M - NX; n++) {
    if (m.comp[n] < 0) continue
    for (const d of D4)
      if (m.comp[n + d] !== m.comp[n]) {
        dist[n] = 0
        q[qn++] = n
        break
      }
  }
  for (let h = 0; h < qn; h++) {
    const n = q[h]
    for (const d of D4) {
      const k = n + d
      if (k < 0 || k >= M) continue
      if (m.comp[k] === m.comp[n] && dist[k] < 0) {
        dist[k] = dist[n] + 1
        q[qn++] = k
      }
    }
  }
  return dist
}

export class RoofIndex {
  roofs: Roof[] = []
  byComp = new Map<number, Roof>()
  edge: Int32Array

  constructor(m: GameMap, maxClimb = 40) {
    const NX = m.NX
    const M = NX * m.NY
    this.edge = edgeDistance(m)
    const size = new Map<number, number>()
    const best = new Map<number, number>()
    for (let n = 0; n < M; n++) {
      const c = m.comp[n]
      if (c < 0 || c === m.mainComp) continue
      size.set(c, (size.get(c) ?? 0) + 1)
      const b = best.get(c)
      if (b === undefined || this.edge[n] > this.edge[b]) best.set(c, n)
    }
    for (const [c, s] of size) {
      if (s < 120) continue
      const camp = best.get(c)!
      const ed = this.edge[camp] * 2
      if (ed < 10) continue
      this.roofs.push({ comp: c, campNode: camp, campX: m.nodeX(camp), campY: m.nodeY(camp), campZ: m.nodeZ(camp), edgeDist: ed, climbs: [] })
    }
    for (const r of this.roofs) this.byComp.set(r.comp, r)
    // climb points: an edge node of the roof with a main-graph ground node 2-6 m outside it
    const stride = new Map<number, number>()
    for (let n = 0; n < M; n++) {
      const r = this.byComp.get(m.comp[n])
      if (!r || this.edge[n] !== 0) continue
      const k = (stride.get(r.comp) ?? 0) + 1
      stride.set(r.comp, k)
      if (k % 3 !== 0) continue // thin them out
      const ex = m.nodeX(n), ey = m.nodeY(n), ez = m.nodeZ(n)
      let bestG = -1
      let bd = Infinity
      for (let dj = -3; dj <= 3; dj++)
        for (let di = -3; di <= 3; di++) {
          const g = n + dj * NX + di
          if (g < 0 || g >= M || !m.isMain(g)) continue
          const d = Math.hypot(di, dj) * 2
          if (d < 2 || d > 6.5) continue
          const h = ez - m.nodeZ(g)
          if (h < 3 || h > maxClimb) continue
          // the wall face between them must be open sky above the roof height (no eave)
          const mx = (ex + m.nodeX(g)) / 2, my = (ey + m.nodeY(g)) / 2
          if (m.head[m.idx(mx, my)] < ez + 10) continue
          if (d < bd) {
            bd = d
            bestG = g
          }
        }
      if (bestG >= 0) r.climbs.push({ gx: m.nodeX(bestG), gy: m.nodeY(bestG), gz: m.nodeZ(bestG), ex, ey, ez })
    }
    this.roofs = this.roofs.filter((r) => r.climbs.length > 0 && r.campZ - Math.min(...r.climbs.map((c) => c.gz)) >= 4)
    this.byComp.clear()
    for (const r of this.roofs) this.byComp.set(r.comp, r)
  }

  /** The roof component under (x,y,z), if we are standing on one we know. */
  roofAt(m: GameMap, x: number, y: number, z: number): Roof | null {
    const n = m.nodeOf(x, y)
    const r = this.byComp.get(m.comp[n])
    if (r && z > m.nodeZ(n) - 2) return r
    return null
  }
}
