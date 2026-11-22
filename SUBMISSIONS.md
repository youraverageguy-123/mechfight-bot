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

| 23 | ee50061 (main) | iso fighting + lose-rule retreat + finisher targeting + fight cubes + laser seek, no kiting | 5.10 (BB-B8AD2E5E) |
| 24 | 38f971e (exp-t-iso-nohunt) | 23 without chasing hidden enemies | 5.00 (BB-133A8606) |
| 25 | 0405f87 (exp-u-iso-s1) | 23, RNG salt 1 | **5.50** (BB-FE172E24) |
| 26 | 39c6d7b (exp-v-noiso) | 23 without iso movement (old strafe) | 4.79 (BB-667B4A8E) |
| 27 | 54ff748 (exp-w-iso-oldretreat) | 23 with the old HP-threshold retreat | 5.35 (BB-CE7A1B96) |
| 28 | 41594f4 (exp-x-iso-noavoid) | 23 without avoiding idle bots | 5.08 (BB-8CB1FDBC) |

Iso movement is the first real gain: the five iso variants scored 5.00-5.50 (mean 5.21), the one
without it 4.79, every earlier version 4.5-4.95. Same-strategy salt draws 5.10 / 5.50 (sd ~0.25).
Retreat rule, chasing and idle-bot avoidance: no visible difference.

| 29 | dec59e6 (exp-y-cover) | 23 + retreat cover hidden from all known mechs, hold when hidden, re-pick 0.5 s | 5.13 (BB-A6D12DAC) |
| 30 | 2b4cd99 (exp-z-rings2) | 23 + iso candidates at 6 m and 12 m | 5.46 (BB-A07C640E) |
| 31 | bec2d97 (exp-aa-watch50) | 23 + iso watcher penalty 50 (was 30) | 4.96 (BB-76177633) |
| 32 | 622b22b (exp-ab-hide-all) | 29 + 30 + 31 | 4.98 (BB-A22DE1FA) |
| 33 | f0c8b43 (exp-ac-iso-s2) | 23, RNG salt 2 | 5.00 (BB-A9DEF2C2) |
| 34 | ce10b3e (exp-ad-iso-s3) | 23, RNG salt 3 | 5.17 (BB-F1BFF625) |

Iso main over four salt draws: 5.10 / 5.50 / 5.00 / 5.17 (mean 5.19, sd 0.22). Of the hiding changes
only the 12 m ring (5.46) is above that mean, and by less than the noise. Cover-from-all 5.13 and
watcher 50 4.96 show nothing; all three together 4.98.

Board check (Sun 07:15 IST): #1 6.44, #9 giddygiddyboy 6.31, #32 ~6.13. The qualifying line has risen
past 6.1. Same-code salt draws differ by sd ~0.2, so a board best is roughly mean + 2 sd of many draws.

Round 3: the 6.19 bot's shooting micro, rebuilt in our code: lead from where both mechs will be when
the shot leaves (75 ms), fire the gun when the torso will be on target after this tick's turn, release a
partly charged laser once it kills. Optional: use a landed supply drop as a heal station below 70 % HP
(within 150 m, alone in the ring; full HP, full energy, 3 cubes). Docker check PASS (max 6 ms on reruns),
fuzz 0 errors / 12000.

| 35 | 27bf3b7 (main) | 23 + latency-compensated aim + slew firing + lethal laser | pending |
| 36 | 848b2e2 (exp-af-fire-r2) | 35 + iso rings 6 m and 12 m | pending |
| 37 | 5fb3d63 (exp-ah-fire-drop) | 35 + drop heal below 70 % | pending |
| 38 | 51691c2 (exp-ag-fire-r2-drop) | 35 + rings 2 + drop heal | pending |
| 39 | e589abd (exp-ai-fire-r2-s1) | 36, RNG salt 1 | pending |
| 40 | 5f57edc (exp-aj-fire-drop-s1) | 37, RNG salt 1 | pending |

Round 4 (builders-3, 48 fixed seeds): giddygiddyboy's MIT Go bot, credited, with per-slot RNG salts (SLOTS.md).

| 41 | a6634a3 (g-base) | giddy bot, no salt | 5.40 (BB-13A0AB92) |
| 42 | 9bc5d41 (g-dup) | 41, salt only for a slot that does not exist (determinism check) | 5.59 (BB-A5E69B4E) |
| 43 | e6e1c84 (g-s0-k1) | 41, slot 0 salt 1 | 5.51 (BB-211CD8F0) |
| 44 | ce829f0 (g-s1-k1) | 41, slot 1 salt 1 | 5.66 (BB-DC3CB257) |
| 45 | 5ecc07e (g-s2-k1) | 41, slot 2 salt 1 | 5.59 (BB-7792D6F1) |
| 46 | a8c7f9b (g-s3-k1) | 41, slot 3 salt 1 | 5.26 (BB-F87112B3) |

41 and 42 differ, so matches do not replay exactly: the salt search is closed. Giddy bot mean 5.50 (sd 0.15)
over six draws, about +0.3 over our own bot's round-2 mean.

Scorer change (Sun 11:30 IST): builders-4 plays 96 fresh random seeds per submission, identical code keeps
its first builders-4 score, and best builders-3 entries are being rescored. Board (Sun 13:36 IST): #1 6.74,
#2 6.39, #3 6.21, giddygiddyboy 5.73, top-32 line ~5.78. Per-submission sd on 96 fresh seeds is about 0.3.

Round 5 (builders-4): real parameter variants of the giddy bot.

| 47 | a6634a3 (g-base) | giddy bot, first builders-4 score | pending |
| 48 | f0a8fe4 (g4-drops) | 47 + take supply drops (full HP, energy, +3 cubes) | pending |
| 49 | f006d04 (g4-eg0) | 47 + never switch to late hiding | pending |
| 50 | 9407d03 (g4-eg70) | 47 + late hiding from the 70 m circle (was 140 m) | pending |
| 51 | 2de4210 (g4-eg240) | 47 + late hiding from the 240 m circle | pending |
| 52 | 7af8246 (g4-drops-eg70) | 48 + 50 | pending |
| 53 | 09fdc0e (g4-lr170) | 47 + hold 170 m with a laser against the squad (was 120 m; house guns land 15-26 % past 80 m) | pending |
| 54 | 8558718 (g4-gr110) | 47 + gun fights at 110 m (was 85 m) | pending |
| 55 | c437874 (g4-nolose) | 47 + HP retreat rule instead of the losing-fight rule in squad play | pending |
| 56 | 09e3f09 (g4-drops-lr170) | 48 + 53 | pending |
| 57 | eb92e44 (g4-lr170-gr110) | 53 + 54 | pending |
| 58 | faf8cf1 (g4-lr90) | 47 + hold 90 m with a laser | pending |

Note: in squad play the giddy bot's losing-fight rule replaces the HP retreat thresholds, so RetreatHP
variants would change nothing there (not queued).

Rows 53-58 are on hold: the next batches use the andrew-zhouu bot instead (approved Sun ~14:40 IST).

Round 6 (builders-4): andrew-zhouu's MIT Go bot v0.167 (https://github.com/andrew-zhouu/builders-mech-royale-2026,
commit 6f9e678), credited in the branch README, our display name only. Docker check PASS (max 3.7 ms).
The variants change only the practice set in its build string (dice kept at 2); free-for-all play is
the same in all of them, and the bot plays free-for-all from the first tick after the cutoff.

| 59 | 8d23b8a (az-base) | their build as published: sq-hunt + quiet opening | pending |
| 60 | 89e3e80 (az-hunt5) | 59, fight plan on the house bots' measured accuracy | pending |
| 61 | 45e73f9 (az-hunt6) | 59, stop farming at 10 cubes so hunting starts sooner | pending |
| 62 | 7b8c27a (az-hunt3) | 59, go looking for the next hunter instead of hiding | pending |
| 63 | c043bdd (az-hunt7) | 61 + 62 | pending |
| 64 | 20bf993 (az-hunt2) | 59, hunt at longer range on thinner odds | pending |

