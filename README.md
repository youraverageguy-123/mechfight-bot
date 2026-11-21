# ohio-ironclad — Mech Royale bot

A `mechfight/1` bot in TypeScript (Bun runtime in Docker, Node 22.18+ locally).

- `src/map.ts` — terrain + prisms rasterised to a 1 m grid, 2 m nav graph, line-of-sight.
- `src/nav.ts` — A* path planning, path following, stuck recovery.
- `src/aim.ts` — gun lead and mortar ballistic solutions.
- `src/brain.ts` — memory, target choice, weapons, movement modes (zone / retreat / fight / loot / farm / hold). Tunables in `P`.
- `src/main.ts` — entry point with a crash-safe fallback policy.
- `tools/sim.ts` — small local simulator for development (`node tools/sim.ts 10 0.6 laser`).

Check (from the kit folder): `python check/check.py -- node ../mechfight-bot/src/main.ts`
