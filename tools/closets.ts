// Closet statistics: how many enclosed spots the map has and how open the rest is.
import { readFileSync } from "node:fs"
import { gunzipSync } from "node:zlib"
import { GameMap } from "../src/map.ts"
import { ClosetIndex } from "../src/closet.ts"

const KIT = "../mechfight-kit"
const hello = JSON.parse(gunzipSync(readFileSync(KIT + "/check/stream.jsonl.gz")).toString("utf8").split("\n")[0])
const m = new GameMap(KIT + "/arena", hello.config.mech)
const R = Number(process.argv[2] ?? 120), good = Number(process.argv[3] ?? 60), lane = Number(process.argv[4] ?? 45)
const ci = new ClosetIndex(m, R, good, lane)
console.log("build ms", ci.buildMs.toFixed(0), "spots", ci.spots.length)
const hist: Record<string, number> = {}
let measured = 0
for (let n = 0; n < ci.maxSight.length; n++) {
  if (ci.maxSight[n] < 0) continue
  measured++
  const b = Math.min(120, Math.floor(ci.maxSight[n] / 10) * 10)
  hist[b] = (hist[b] ?? 0) + 1
}
console.log("measured", measured, JSON.stringify(hist))
const laneHist: Record<number, number> = {}
for (let n = 0; n < ci.maxSight.length; n++) if (ci.maxSight[n] >= 0 && ci.maxSight[n] < R) laneHist[ci.lanes[n]] = (laneHist[ci.lanes[n]] ?? 0) + 1
console.log("lanes (non-open spots)", JSON.stringify(laneHist))
// spatial spread of good spots (100 m cells)
const cells = new Map<string, number>()
for (const n of ci.spots) {
  const k = Math.floor(m.nodeX(n) / 100) + "," + Math.floor(m.nodeY(n) / 100)
  cells.set(k, (cells.get(k) ?? 0) + 1)
}
console.log("100m cells with spots", cells.size, [...cells.entries()].slice(0, 40).map(([k, v]) => k + ":" + v).join(" "))
