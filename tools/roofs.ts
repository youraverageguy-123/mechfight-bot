import { GameMap } from "../src/map.ts"
import { RoofIndex } from "../src/roof.ts"
const m = new GameMap("../mechfight-kit/arena", { step_up: 1.2, height: 9, radius: 2.5 })
const t0 = performance.now()
const R = new RoofIndex(m)
console.log("roof index ms", (performance.now() - t0).toFixed(0), "roofs", R.roofs.length)
for (const r of R.roofs.sort((a, b) => b.edgeDist - a.edgeDist).slice(0, 12)) console.log(r.comp, "camp", r.campX, r.campY, "z", r.campZ.toFixed(1), "edge", r.edgeDist, "climbs", r.climbs.length, "minH", (r.campZ - Math.max(...r.climbs.map((c) => c.gz))).toFixed(1))
