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
