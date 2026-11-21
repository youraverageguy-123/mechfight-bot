// "Closets": walkable spots whose sightlines are all short (alleys, courtyards, nooks between
// buildings). A bot that wants to shoot us there has to come close first, which beats the house
// bots that keep their distance with lasers.
import type { GameMap } from "./map.ts"

const DIRS = 24

export class ClosetIndex {
  /** Per nav node: longest sightline (m) from eye height, or -1 when not measured. */
  maxSight: Float32Array
  /** Per nav node: number of directions with a sightline longer than `longLane`. */
  lanes: Uint8Array
  /** Measured nodes with maxSight <= `good`, for quick search. */
  spots: number[] = []
  buildMs = 0

  constructor(m: GameMap, R = 120, good = 60, longLane = 45, stride = 2) {
    const t0 = performance.now()
    const N = m.NX * m.NY
    this.maxSight = new Float32Array(N).fill(-1)
    this.lanes = new Uint8Array(N)
    const cs: number[] = [], sn: number[] = []
    for (let k = 0; k < DIRS; k++) {
      cs.push(Math.cos((k / DIRS) * Math.PI * 2))
      sn.push(Math.sin((k / DIRS) * Math.PI * 2))
    }
    const { terrain, sBase, sTop } = m
    for (let j = 0; j < m.NY; j += stride)
      for (let i = 0; i < m.NX; i += stride) {
        const n = j * m.NX + i
        if (!m.isMain(n)) continue
        const x = m.nodeX(n), y = m.nodeY(n), zz = m.nodeZ(n) + 8
        let mx = 0, lanes = 0
        for (let k = 0; k < DIRS; k++) {
          let d = 3
          for (; d < R; d += 2) {
            const c = m.idx(x + cs[k] * d, y + sn[k] * d)
            if (terrain[c] > zz - 0.3 || (zz > sBase[c] && zz < sTop[c])) break
          }
          if (d > mx) mx = d
          if (d > longLane) lanes++
          if (mx >= R && lanes > 6) break // wide open: stop early
        }
        this.maxSight[n] = mx
        this.lanes[n] = lanes
        if (mx <= good) this.spots.push(n)
      }
    this.buildMs = performance.now() - t0
  }
}
