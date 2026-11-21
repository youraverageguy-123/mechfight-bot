// Static map model: terrain + prisms rasterised to a 1 m grid, a 2 m navigation graph,
// and line-of-sight ray marching. Built once at `hello` from hello.map.dir.
import { readFileSync } from "node:fs"

type Prism = { kind: string; base: number; top: number; ring: number[]; holes?: number[][] }

const NEI8: [number, number][] = [
  [1, 0], [-1, 0], [0, 1], [0, -1], [1, 1], [1, -1], [-1, 1], [-1, -1],
]

export class GameMap {
  // 1 m vertex grid
  nx = 0
  ny = 0
  minX = 0
  minY = 0
  bminX = 0
  bmaxX = 0
  bminY = 0
  bmaxY = 0
  terrain!: Float32Array
  surf!: Float32Array // walkable surface (terrain raised by prisms stacked on it)
  head!: Float32Array // lowest overhang base above surf (Infinity = open sky)
  sBase!: Float32Array // merged solid interval per cell (for LOS / collision)
  sTop!: Float32Array
  walk1!: Uint8Array // a mech centre can stand here
  // 2 m nav grid: node (I,J) sits on 1 m cell (2I, 2J)
  NX = 0
  NY = 0
  navOk!: Uint8Array
  navMask!: Uint8Array // bit k = edge to NEI8[k] allowed
  comp!: Int32Array
  mainComp = 0
  cover!: Uint8Array
  stepUp = 1.2
  mechH = 9
  mechR = 2.5

  constructor(dir: string, mechCfg: Record<string, number>) {
    this.stepUp = mechCfg.step_up ?? 1.2
    this.mechH = mechCfg.height ?? 9
    this.mechR = mechCfg.radius ?? 2.5
    const world = JSON.parse(readFileSync(dir + "/v2/world.json", "utf8"))
    const t = world.terrain
    this.nx = t.nx
    this.ny = t.ny
    this.minX = t.minX
    this.minY = t.minY
    const b = world.bounds
    this.bminX = b.minX
    this.bmaxX = b.maxX
    this.bminY = b.minY
    this.bmaxY = b.maxY
    const N = this.nx * this.ny
    const raw = readFileSync(dir + "/v2/" + (t.file ?? "terrain.i16"))
    const dv = new DataView(raw.buffer, raw.byteOffset, raw.byteLength)
    const scale = t.scale ?? 0.01
    this.terrain = new Float32Array(N)
    for (let k = 0; k < N; k++) this.terrain[k] = dv.getInt16(k * 2, true) * scale
    this.surf = new Float32Array(this.terrain)
    this.head = new Float32Array(N).fill(Infinity)
    this.sBase = new Float32Array(N).fill(Infinity)
    this.sTop = new Float32Array(N).fill(-Infinity)
    const prisms: Prism[] = (world.prisms as Prism[]).slice().sort((a, b) => a.base - b.base)
    for (const p of prisms) this.rasterise(p)
    this.buildWalk()
    this.buildNav()
    this.buildCover()
  }

  idx(x: number, y: number): number {
    let i = Math.round(x - this.minX)
    let j = Math.round(y - this.minY)
    if (i < 0) i = 0
    else if (i >= this.nx) i = this.nx - 1
    if (j < 0) j = 0
    else if (j >= this.ny) j = this.ny - 1
    return j * this.nx + i
  }

  groundAt(x: number, y: number): number {
    const fx = x - this.minX
    const fy = y - this.minY
    let i = Math.floor(fx)
    let j = Math.floor(fy)
    if (i < 0) i = 0
    if (j < 0) j = 0
    if (i > this.nx - 2) i = this.nx - 2
    if (j > this.ny - 2) j = this.ny - 2
    const u = Math.min(1, Math.max(0, fx - i))
    const v = Math.min(1, Math.max(0, fy - j))
    const T = this.terrain
    const k = j * this.nx + i
    return (T[k] * (1 - u) + T[k + 1] * u) * (1 - v) + (T[k + this.nx] * (1 - u) + T[k + this.nx + 1] * u) * v
  }

  surfAt(x: number, y: number): number {
    return this.surf[this.idx(x, y)]
  }

  private rasterise(p: Prism) {
    const rings: number[][] = [p.ring, ...(p.holes ?? [])]
    let y0 = Infinity
    let y1 = -Infinity
    for (const r of rings)
      for (let k = 1; k < r.length; k += 2) {
        if (r[k] < y0) y0 = r[k]
        if (r[k] > y1) y1 = r[k]
      }
    const j0 = Math.max(0, Math.ceil(y0 - this.minY))
    const j1 = Math.min(this.ny - 1, Math.floor(y1 - this.minY))
    const xs: number[] = []
    for (let j = j0; j <= j1; j++) {
      const y = this.minY + j
      xs.length = 0
      for (const r of rings) {
        const n = r.length >> 1
        for (let a = 0, bb = n - 1; a < n; bb = a++) {
          const ax = r[a * 2], ay = r[a * 2 + 1], bx = r[bb * 2], by = r[bb * 2 + 1]
          if (ay > y !== by > y) xs.push(ax + ((y - ay) / (by - ay)) * (bx - ax))
        }
      }
      xs.sort((a, b) => a - b)
      for (let s = 0; s + 1 < xs.length; s += 2) {
        const i0 = Math.max(0, Math.ceil(xs[s] - this.minX))
        const i1 = Math.min(this.nx - 1, Math.floor(xs[s + 1] - this.minX))
        for (let i = i0; i <= i1; i++) this.mark(j * this.nx + i, p.base, p.top)
      }
    }
  }

  private mark(c: number, base: number, top: number) {
    if (base < this.sBase[c]) this.sBase[c] = base
    if (top > this.sTop[c]) this.sTop[c] = top
    const s = this.surf[c]
    if (base <= s + this.stepUp) {
      if (top > s) this.surf[c] = top
    } else if (base < this.head[c]) this.head[c] = base
  }

  private buildWalk() {
    const { nx, ny } = this
    const N = nx * ny
    this.walk1 = new Uint8Array(N)
    const R = this.mechR
    const offs: number[] = []
    const ri = Math.ceil(R)
    for (let dj = -ri; dj <= ri; dj++)
      for (let di = -ri; di <= ri; di++) if (di * di + dj * dj <= R * R) offs.push(dj * nx + di)
    const marginX0 = Math.ceil(this.bminX - this.minX + R + 0.5)
    const marginX1 = Math.floor(this.bmaxX - this.minX - R - 0.5)
    const marginY0 = Math.ceil(this.bminY - this.minY + R + 0.5)
    const marginY1 = Math.floor(this.bmaxY - this.minY - R - 0.5)
    const { surf, head, sBase, sTop } = this
    for (let j = Math.max(ri, marginY0); j <= Math.min(ny - 1 - ri, marginY1); j++) {
      for (let i = Math.max(ri, marginX0); i <= Math.min(nx - 1 - ri, marginX1); i++) {
        const c = j * nx + i
        const feet = surf[c]
        if (head[c] - feet < this.mechH) continue
        let ok = 1
        const lo = feet + this.stepUp
        const hi = feet + this.mechH
        for (let k = 0; k < offs.length; k++) {
          const d = c + offs[k]
          if ((sTop[d] > lo && sBase[d] < hi) || (head[d] < hi && head[d] > lo)) {
            ok = 0
            break
          }
        }
        this.walk1[c] = ok
      }
    }
  }

  private buildNav() {
    const NX = (this.NX = ((this.nx - 1) >> 1) + 1)
    const NY = (this.NY = ((this.ny - 1) >> 1) + 1)
    const M = NX * NY
    this.navOk = new Uint8Array(M)
    this.navMask = new Uint8Array(M)
    const { nx, walk1, surf } = this
    for (let J = 0; J < NY; J++)
      for (let I = 0; I < NX; I++) this.navOk[J * NX + I] = walk1[2 * J * nx + 2 * I]
    const su = this.stepUp
    for (let J = 1; J < NY - 1; J++)
      for (let I = 1; I < NX - 1; I++) {
        const n = J * NX + I
        if (!this.navOk[n]) continue
        const ca = 2 * J * nx + 2 * I
        let mask = 0
        for (let k = 0; k < 8; k++) {
          const [di, dj] = NEI8[k]
          const m = (J + dj) * NX + (I + di)
          if (!this.navOk[m]) continue
          const cm = ca + dj * nx + di // 1 m midpoint cell
          const cb = ca + 2 * dj * nx + 2 * di
          if (!walk1[cm]) continue
          if (surf[cm] - surf[ca] > su || surf[cb] - surf[cm] > su) continue
          if (di !== 0 && dj !== 0) {
            if (!this.navOk[J * NX + I + di] || !this.navOk[(J + dj) * NX + I]) continue
          }
          mask |= 1 << k
        }
        this.navMask[n] = mask
      }
    // undirected components over two-way edges
    const comp = (this.comp = new Int32Array(M).fill(-1))
    const stack = new Int32Array(M)
    let best = 0
    let bestSize = 0
    let id = 0
    for (let s = 0; s < M; s++) {
      if (!this.navOk[s] || comp[s] >= 0) continue
      let sp = 0
      stack[sp++] = s
      comp[s] = id
      let size = 0
      while (sp > 0) {
        const n = stack[--sp]
        size++
        const mask = this.navMask[n]
        for (let k = 0; k < 8; k++) {
          if (!(mask & (1 << k))) continue
          const m = n + NEI8[k][1] * NX + NEI8[k][0]
          if (comp[m] >= 0) continue
          // reverse edge must exist too
          const rk = k ^ 1
          if (!(this.navMask[m] & (1 << rk))) continue
          comp[m] = id
          stack[sp++] = m
        }
      }
      if (size > bestSize) {
        bestSize = size
        best = id
      }
      id++
    }
    this.mainComp = best
  }

  /** How much tall solid surrounds each nav node (0..255): good places to hide near. */
  private buildCover() {
    const { NX, NY, nx } = this
    const M = NX * NY
    this.cover = new Uint8Array(M)
    const R = 7
    const offs: number[] = []
    for (let dj = -R; dj <= R; dj++)
      for (let di = -R; di <= R; di++) if (di * di + dj * dj <= R * R && di * di + dj * dj > 4) offs.push(dj * nx + di)
    const N = this.nx * this.ny
    for (let n = 0; n < M; n++) {
      if (this.comp[n] !== this.mainComp) continue
      const c = 2 * Math.floor(n / NX) * nx + 2 * (n % NX)
      const z = this.surf[c]
      let cnt = 0
      for (let k = 0; k < offs.length; k++) {
        const d = c + offs[k]
        if (d < 0 || d >= N) continue
        if (this.sTop[d] > z + 8 && this.sBase[d] < z + 3) cnt++
      }
      this.cover[n] = Math.min(255, cnt)
    }
  }

  // ---- nav helpers ----
  nodeOf(x: number, y: number): number {
    let I = Math.round((x - this.minX) / 2)
    let J = Math.round((y - this.minY) / 2)
    I = Math.max(0, Math.min(this.NX - 1, I))
    J = Math.max(0, Math.min(this.NY - 1, J))
    return J * this.NX + I
  }
  nodeX(n: number): number {
    return this.minX + 2 * (n % this.NX)
  }
  nodeY(n: number): number {
    return this.minY + 2 * Math.floor(n / this.NX)
  }
  nodeZ(n: number): number {
    return this.surf[2 * Math.floor(n / this.NX) * this.nx + 2 * (n % this.NX)]
  }

  /** Nearest nav node to (x,y) that passes `pred`, searching rings up to maxR nodes. */
  nearestNode(x: number, y: number, maxR: number, pred: (n: number) => boolean): number {
    const n0 = this.nodeOf(x, y)
    if (pred(n0)) return n0
    const I0 = n0 % this.NX
    const J0 = Math.floor(n0 / this.NX)
    for (let r = 1; r <= maxR; r++) {
      let best = -1
      let bd = Infinity
      for (let dj = -r; dj <= r; dj++)
        for (let di = -r; di <= r; di++) {
          if (Math.max(Math.abs(di), Math.abs(dj)) !== r) continue
          const I = I0 + di
          const J = J0 + dj
          if (I < 0 || J < 0 || I >= this.NX || J >= this.NY) continue
          const n = J * this.NX + I
          if (!pred(n)) continue
          const d = Math.hypot(this.nodeX(n) - x, this.nodeY(n) - y)
          if (d < bd) {
            bd = d
            best = n
          }
        }
      if (best >= 0) return best
    }
    return -1
  }

  isMain(n: number): boolean {
    return n >= 0 && this.comp[n] === this.mainComp
  }

  /** Straight walk check on the 1 m grid (no wall, no big step up). */
  walkLine(ax: number, ay: number, bx: number, by: number): boolean {
    const len = Math.hypot(bx - ax, by - ay)
    const steps = Math.max(1, Math.ceil(len / 0.8))
    let prev = this.surf[this.idx(ax, ay)]
    for (let s = 1; s <= steps; s++) {
      const f = s / steps
      const c = this.idx(ax + (bx - ax) * f, ay + (by - ay) * f)
      if (!this.walk1[c]) return false
      const h = this.surf[c]
      if (h - prev > this.stepUp) return false
      prev = h
    }
    return true
  }

  // ---- line of sight ----
  /** True if nothing solid lies between the two points (skips `skipA`/`skipB` m at the ends). */
  clear(ax: number, ay: number, az: number, bx: number, by: number, bz: number, skipA = 0.5, skipB = 1.5): boolean {
    const dx = bx - ax, dy = by - ay, dz = bz - az
    const len = Math.hypot(dx, dy, dz)
    if (len < 1e-6) return true
    const step = 1
    const t0 = skipA
    const t1 = len - skipB
    const { terrain, sBase, sTop } = this
    for (let tt = t0; tt <= t1; tt += step) {
      const f = tt / len
      const x = ax + dx * f, y = ay + dy * f, z = az + dz * f
      const c = this.idx(x, y)
      if (z < terrain[c] - 0.3) return false
      if (z > sBase[c] && z < sTop[c]) return false
    }
    return true
  }
}
