import { readFileSync } from "node:fs"
import { gunzipSync } from "node:zlib"
import { GameMap } from "../src/map.ts"
import { RoofIndex } from "../src/roof.ts"
const KIT = "../mechfight-kit"
const hello = JSON.parse(gunzipSync(readFileSync(KIT + "/check/stream.jsonl.gz")).toString("utf8").split("\n")[0])
const m = new GameMap(KIT + "/arena", hello.config.mech)
const ri = new RoofIndex(m)
// all roof nodes with edge >= 3 (6 m in from the edge)
const pts: number[][] = []
for (let n = 0; n < m.NX * m.NY; n++) { const r = ri.byComp.get(m.comp[n]); if (r && ri.edge[n] >= 3) pts.push([m.nodeX(n), m.nodeY(n), m.nodeZ(n), r.comp]) }
console.log("roofs", ri.roofs.length, "roof nodes", pts.length, "heights", ri.roofs.map(r => Math.round(r.campZ - Math.min(...r.climbs.map(c => c.gz)))).join(","))
const Z = hello.config.zone
let hit70 = 0, hit25 = 0, hit0 = 0
const N = 4000
let seed = 7; const rnd = () => ((seed = (seed * 1103515245 + 12345) % 2147483648) / 2147483648)
for (let k = 0; k < N; k++) {
  let c = Z.start_center.slice(), r = Z.start_radius
  const cs: number[][] = []
  for (const ph of Z.phases) {
    const R = r - ph.radius
    let nx, ny
    for (;;) { const a = rnd() * 2 * Math.PI, d = Math.sqrt(rnd()) * R; nx = c[0] + Math.cos(a) * d; ny = c[1] + Math.sin(a) * d; break }
    c = [nx, ny]; r = ph.radius; cs.push([nx, ny, r])
  }
  const [c70, c25, c0] = [cs[3], cs[4], cs[5]]
  if (pts.some(p => Math.hypot(p[0] - c70[0], p[1] - c70[1]) < 60)) hit70++
  if (pts.some(p => Math.hypot(p[0] - c25[0], p[1] - c25[1]) < 18)) hit25++
  if (pts.some(p => Math.hypot(p[0] - c0[0], p[1] - c0[1]) < 8)) hit0++
}
console.log("P(roof within r70 circle)", (hit70 / N).toFixed(2), "P(within r25)", (hit25 / N).toFixed(2), "P(roof within 8 m of final point)", (hit0 / N).toFixed(2))
