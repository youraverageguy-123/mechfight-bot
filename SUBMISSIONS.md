# Submission log

| # | commit | change | practice score |
|---|--------|--------|----------------|
| 1 | 026c89d | v1: map/nav/LOS, lead aim, weapons, zone, retreat, explore, hearing | **4.60** |
| 2 | 14e36c6 (exp-a-nohunt) | no chasing | 4.35 or 4.52 (user mapping unclear) |
| 3 | 4ddebc4 (exp-b-open) | open-ground hold spots | 4.52 or 4.35 |
| 4 | 3b63335 (exp-c-cautious) | retreat 0.5 / recover 0.95 / farm 0.75 | 4.60 |

Finding from kit recording: house laser bots stand off at 100-150 m; charge is visible ~0.8 s before each 315-dmg hit.
| 5 | 1ffa5f6 (main) | duck behind cover from charging lasers + peek spots | 4.60 |
| 6 | 425eec5 (exp-d-duckonly) | duck only | 4.63 |
| 7 | 92215bf (exp-e-hugcover) | duck + peek + hold right against buildings | 4.46 |

Findings: 12k fuzzed observations -> no crashes. Roof camping (sim): survives longer but kills less, same points -> off.
Practice points are essentially our kills (house bots never fight each other).
Recorded laser bot: gun lead hit-rate vs jinking target 100% @50m, 73% @100m, 47% @150m, 21% @200m.
Sim (paired, 100 seeds): hunting drones + wider cube pickup = +0.97 +-0.41 pts.
| 8 | 05d2408 (main) | cube economy: hunt drones, wide cube pickup | 4.58 |
| 9 | e0c08b1 (exp-f-mildfarm) | milder drone hunting | 4.67 |
| 10 | 297cfbe (exp-g-closein) | cube economy + close in under 70 m | 4.67 |

Finding (track rules): the 15 house bots are 4 easy / 4 medium / 4 hard / 3 expert and hunt us
weakest first; the rest stay idle on the map. Our bot shot at and chased every mech in sight, so it
woke up stronger idle bots. Sim rebuilt with the full tiered roster (TIERS=1).

| 11 | b9bfc9a (main) | selective: fight only mechs that are after us, keep away from idle bots | **4.94** |
| 12 | 54e454e (exp-j-evasive) | selective + avoid idle bots within 140 m + no chasing | 4.42 |
| 13 | 62c243b (exp-k-noavoid) | selective targeting only, no avoidance | 4.75 |

Real evidence so far: every less aggressive variant lost points (no chase 4.35-4.52, evasive 4.42).
Map has almost no enclosed spots (30 of 32k have all sightlines < 60 m), so no closet ambush.
A roof lies in the final r25 circle in ~48% of zone draws (~25% within 8 m of the last point).

| 14 | 0292805 (exp-l-roof) | selective + rooftop camping from 45 s (do house bots climb?) | 4.17 |
| 15 | 5debac2 (exp-m-aggro) | selective + chase to 250 m, boost in, gun band 30-90 m, contest drops | 4.81 |
| 16 | 32ebfdb (exp-n-roof150) | selective + rooftop camping from 150 s | 4.92 |

Research (public repos of other teams, read-only): house bots tap-fire (0.4-0.6 s), aim sigma
6 deg (easy) -> 1.6 deg (expert), lead 0.5 -> 1.0, reaction 0.9 -> 0.3 s. Squad model: hunters =
weakest living, 1 + our kills, all of them from ~155-265 s. Sim rebuilt on these numbers (HOUSE=2)
reproduces 7 of 10 real scores within ~0.5. In it we die at ~95 s with 3-6 hunters within 120 m.
Per-match std 2.5 -> one real run of 48 matches carries about +-0.36 of luck.

Sweep (25 single changes) -> mostly noise. Kiting away from 2+ converging hunters is the first robust
gain: +0.77 / +1.23 / +0.71 (three house-bot models, 192 paired seeds). Silent creeping -1.0,
lightning-first -0.86, rooftops -0.5..-1.1.

| 19 | 90d734b (main) | kite away from converging hunters (200 m) + avoid idle bots 60 m | 4.81 (BB-2FD053E1) |
| 20 | 732b8ba (exp-q-kite-s1) | same as 19, RNG salt 1 (second draw) | 4.52 (BB-E6D2B1F7) |
| 21 | a2bde0a (exp-r-kite-s2) | same as 19, RNG salt 2 (third draw) | 4.71 (BB-E3377CEE) |
| 22 | ff4e37e (exp-s-kite-band) | 19 + gun band 60-160 m | 4.69 (BB-A2DA0247) |

Kiting lost in reality (mean 4.68 over 4 runs vs 4.94 / 4.92 without it): the sim overrated it.
Spread of the three same-strategy salt draws: 4.52-4.81 (sd ~0.15), so one real run separates
versions ~0.4 apart. exp-o / exp-p were never submitted.

Research round 2 (public repos, read-only): other teams plateau at 4.5-4.9 too (LanceHuang245:
4.75 / 4.88 / 4.69; their notes: identical code rescored identically, survivability dominates).
Board #3 giddygiddyboy (6.19) published an MIT bot. Its practice mode: fight movement that picks
spots keeping line of sight to the target while hidden from every other mech; retreat only while
losing; target the wounded and stick to them; finish at 20 m; grab cubes mid-fight; seek lasers.
Reimplemented those ideas in our own code (iso / loseRule / gTarget / gEngage / finishR /
fightCube / laserSeek / noMortar / gUp switches).

| 23 | ee50061 (main) | iso fighting + lose-rule retreat + finisher targeting + fight cubes + laser seek, no kiting | pending |
| 24 | 38f971e (exp-t-iso-nohunt) | 23 without chasing hidden enemies | pending |
| 25 | 0405f87 (exp-u-iso-s1) | 23, RNG salt 1 | pending |
| 26 | 39c6d7b (exp-v-noiso) | 23 without iso movement (old strafe) | pending |
| 27 | 54ff748 (exp-w-iso-oldretreat) | 23 with the old HP-threshold retreat | pending |
| 28 | 41594f4 (exp-x-iso-noavoid) | 23 without avoiding idle bots | pending |
