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
| 13 | 62c243b (exp-k-noavoid) | selective targeting only, no avoidance | pending |

Real evidence so far: every less aggressive variant lost points (no chase 4.35-4.52, evasive 4.42).
Map has almost no enclosed spots (30 of 32k have all sightlines < 60 m), so no closet ambush.
A roof lies in the final r25 circle in ~48% of zone draws (~25% within 8 m of the last point).

| 14 | 0292805 (exp-l-roof) | selective + rooftop camping from 45 s (do house bots climb?) | pending |
| 15 | 5debac2 (exp-m-aggro) | selective + chase to 250 m, boost in, gun band 30-90 m, contest drops | pending |
| 16 | 32ebfdb (exp-n-roof150) | selective + rooftop camping from 150 s | pending |
