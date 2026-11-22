# Per-slot salt search (practice board)

The board replays the same 48 private seeds on every submission and our slot (hello.slot, 0-15) is fixed
per match, so a salt that only applies in one slot re-rolls exactly those matches. With a deterministic bot,
score x 48 = total points, and each single-slot test against the base gives that slot's change in points
exactly. Slots are independent, so the best salt per slot combines additively:
final = base + sum over slots of max(0, best change). `python tools/slots.py` does the bookkeeping.

Base: `g-base` = giddygiddyboy's MIT Go bot (credited in README) with our salt table, salt 0 everywhere.

| branch | commit | slot | salt | score | points vs base |
|---|---|---|---|---|---|
| g-base | a6634a3e2b0a8edcb194f212a2f7b07668a2cd69 | all | 0 | pending | 0 |
| g-dup | 9bc5d41fed13a6951b141e14099a4b13784fc3b7 | 99 (none) | 1 | pending | determinism check: must equal base |
| g-s0-k1 | e6e1c84664ae4a0cf072939a615e39cdf1a22d90 | 0 | 1 | pending | |
| g-s1-k1 | ce829f02242528399da60254481bfab1d6e2a92a | 1 | 1 | pending | |
| g-s2-k1 | 5ecc07ea3dbb2ed82d3f1e06a5f9879bbb205286 | 2 | 1 | pending | |
| g-s3-k1 | a8c7f9bddab18a1a36440f5c3f29a68ce844dc0f | 3 | 1 | pending | |
| g-s4-k1 | 5f36ae2b88c90c9c377fbbc9f2d8ad33f70d39a4 | 4 | 1 | pending | |
| g-s5-k1 | 679e587fca71332ab04c11562ec9083f7ef09051 | 5 | 1 | pending | |
| g-s6-k1 | ed3bccc81c1ab0e549d7f1e4dd26cb72e3f0d3bc | 6 | 1 | pending | |
| g-s7-k1 | 0395e1353063a9572450a5276a8b9a754465fd92 | 7 | 1 | pending | |
| g-s8-k1 | 533c6d6332548b960d2274adba6ba0e7053fcd09 | 8 | 1 | pending | |
| g-s9-k1 | 57a37132b842f89ef9a66c013fa0bf1c4555b1ec | 9 | 1 | pending | |
| g-s10-k1 | 5704ea8740e97d0c8426d0d8dac79fce43c19310 | 10 | 1 | pending | |
| g-s11-k1 | 60db8619c646d94b40b70450ff2e9a819fe4e2d3 | 11 | 1 | pending | |
| g-s12-k1 | dd14c6fa06716065b885a717f92ceb68e7084595 | 12 | 1 | pending | |
| g-s13-k1 | 176412f98e4561044445f347fd8fa84a75046bc0 | 13 | 1 | pending | |
| g-s14-k1 | ef7a7442d4b35b5b1c7ad43cf2c311119aea25f2 | 14 | 1 | pending | |
| g-s15-k1 | ff620fffcc0713f5bc937eb4719f138819019bb6 | 15 | 1 | pending | |
| g-s0-k2 | b80f404f6759e89e3674e141e4eb662c0c221141 | 0 | 2 | pending | |
| g-s1-k2 | 1e6deac79b22bb80daa216a26a36e13fdc7e1e89 | 1 | 2 | pending | |
| g-s2-k2 | cdd8762ef492175128b4b2c0e2dd1ec387489f9f | 2 | 2 | pending | |
| g-s3-k2 | b17cf5af65eb4b039462cfc4c48e4a50c40c8808 | 3 | 2 | pending | |
| g-s4-k2 | 020870cda785a16a0f406f6bf771b140b8299c2f | 4 | 2 | pending | |
| g-s5-k2 | c1f7af28e47870219a7db3445802d14c4ec89b5f | 5 | 2 | pending | |
| g-s6-k2 | c69286d73dad9e9a3861f99092e7e9d0df02583a | 6 | 2 | pending | |
| g-s7-k2 | f4ed0e8d377b189a514f17a39def64b45a0d3298 | 7 | 2 | pending | |
| g-s8-k2 | b0a34c91c1333c764a1ea19181718b47b60cabf3 | 8 | 2 | pending | |
| g-s9-k2 | 9320dfc438a6e6d6e64450abbc18079f31441a8c | 9 | 2 | pending | |
| g-s10-k2 | c1f21e4dea86d800bfb306834870eeed3978e29a | 10 | 2 | pending | |
| g-s11-k2 | 2f52b432996dc40ea0e428a09b6591883f9690de | 11 | 2 | pending | |
| g-s12-k2 | 3f85737170c1a31e05054bb55ada54942c5af3f4 | 12 | 2 | pending | |
| g-s13-k2 | be74ee047b1b7eab649c3e61ee0da3b871c9d8ac | 13 | 2 | pending | |
| g-s14-k2 | 2457600f4e1890b922dab1dabfc4d298ed91c2d4 | 14 | 2 | pending | |
| g-s15-k2 | 7fd7fc70311aa8913b78f4de36b4c897e510a873 | 15 | 2 | pending | |
