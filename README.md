# ohio-ironclad — Mech Royale bot

A `mechfight/1` bot in TypeScript (Bun runtime in Docker, Node 22.18+ locally).

- `src/map.ts` — terrain + prisms rasterised to a 1 m grid, 2 m nav graph, line-of-sight.
- `src/nav.ts` — A* path planning, path following, stuck recovery.
- `src/aim.ts` — gun lead and mortar ballistic solutions.
- `src/brain.ts` — memory, target choice, weapons, movement modes (zone / retreat / fight / loot / farm / hold). Tunables in `P`.
- `src/main.ts` — entry point with a crash-safe fallback policy.
- `tools/sim.ts` — small local simulator for development (`node tools/sim.ts 10 0.6 laser`).

## Practice build on this branch: giddygiddyboy's bot (MIT) + per-slot salts

On this branch the top-level `Dockerfile` builds `giddy/`, the Go bot published by team giddygiddyboy
(https://github.com/justnau/giddygiddyboy-mech-royale, MIT License, copyright (c) 2026 giddygiddyboy, kept in
`giddy/LICENSE`). We took it from the MIT copy vendored in https://github.com/charliescull/mech-royale-bot
(`drivers/giddy/`, upstream `e19e3c8`, which includes the end-game hide rule from SandeepMadhavarapu/BattleRoyale,
MIT). Our changes: the bot name, a `/arena` fallback for the map directory, and per-slot RNG salts
(`giddy/cmd/bot/salt.go`). The TypeScript bot in `src/` is our own work and is unchanged.

Check (from the kit folder): `python check/check.py -- node ../mechfight-bot/src/main.ts`
