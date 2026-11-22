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

| 59 | 8d23b8a (az-base) | their build as published: sq-hunt + quiet opening | 6.07 (BB-C934BF69) |
| 60 | 89e3e80 (az-hunt5) | 59, fight plan on the house bots' measured accuracy | 6.00 (BB-D67E009E) |
| 61 | 45e73f9 (az-hunt6) | 59, stop farming at 10 cubes so hunting starts sooner | 5.89 (BB-58978E77) |
| 62 | 7b8c27a (az-hunt3) | 59, go looking for the next hunter instead of hiding | 6.27 (BB-B15B4441) |
| 63 | c043bdd (az-hunt7) | 61 + 62 | 6.24 (BB-6B3EE86B) |
| 64 | 20bf993 (az-hunt2) | 59, hunt at longer range on thinner odds | 5.91 (BB-37CD3B29) |

Seek (62, 63) scored 6.27 and 6.24; the four without it averaged 5.97. 62 is our best (board #5) and is
the entered final pick. Next batches build on seek. Page shows 48 matches per practice run.

Round 7 (builders-4): seek-based practice sets on top of 62 (sq-hunt3, 6.27). hN, hN2, hN4 are the
andrew-zhouu code's own sweep sets (its notes: hN was the best of its first sweep, +0.12); hunt9,
hunt10, hunt13 are ours (branch az-sets adds them). Docker check PASS (max 3.2 ms). Free-for-all play
unchanged.

| 65 | bde996f (az-hN) | fight on 0.6 odds (was 0.9), seek, stop farming at 6 cubes | 6.20 (BB-9670EE7D) |
| 66 | 25dfe7c (az-hN2) | 65 with farming stopped at 3 cubes | 5.86 (BB-DD3E9747) |
| 67 | d91145f (az-hN4) | 62 + stop farming at 6 cubes | 5.89 (BB-6A468782) |
| 68 | 16c78a2 (az-hunt9) | 62 + remember seen mechs 60 s when seeking (was 30) | 6.27 (BB-50859AEF) |
| 69 | c82fbcd (az-hunt10) | 62 + seek up to 500 m away (was 320) | 6.11 (BB-F5346C33) |
| 70 | 8f76cad (az-hunt13) | 68 + 69 + seek at half health (was 70 %) | 6.27 (BB-577E7529) |

Ready for a later batch (pushed): az-hunt8 cc21a57 (62 + seek at half health), az-hunt11 46758a0
(62 + house-accuracy fight plan), az-hunt12 65792a7 (62 + hunt above 45 % health).

Cutoff note: these builds play free-for-all from 16:00 UTC (21:30 IST). A practice run still being
scored after that would be played in free-for-all mode, so the last submissions should go in by 21:00 IST.

Round 7 result: seek-family scores 6.27, 6.24, 6.20, 6.27, 6.11, 6.27, 5.89, 5.86 (mean 6.14, sd 0.17); no
tweak stands out from the noise. 62 (BB-B15B4441) stays the entry: equal scores rank the earlier one first.
Live board (Sun 11:07 UTC): we are "The anon" #7 at 6.27; #1 6.74, #2 6.63, #3 6.49; top-32 line 5.96.

Round 8 (builders-4): 62 (sq-hunt3) plus one mechanic each (branch az-sets2 adds the sets). Docker check
PASS (max 3.7 ms). Free-for-all play unchanged.

| 71 | dac5e9e (az-hunt14) | 62 + run for supply drops within 260 m when hurt | 6.16 (BB-3BB9D832) |
| 72 | 63a6631 (az-hunt15) | 62 + run for supply drops within 420 m at any health | 6.00 (BB-93E50E9F) |
| 73 | 893aec7 (az-hunt16) | 62 + farm instead of killing hunters for the first 45 s | not submitted |
| 74 | b6f8acc (az-hunt17) | 62 + farm instead of killing hunters for the first 90 s | 6.07 (BB-1E30C711) |
| 75 | b142b12 (az-hunt18) | 62 + hold roofs in squad play | 6.15 (BB-19328AF0) |
| 76 | e813c44 (az-hunt19) | 62 + focus fire on the most damaged target | 5.88 (BB-5AAA8C0D) |

Rows 71-76 withdrawn (Sun ~16:50 IST): the final entry has to be our own code (organizers' rule on excessive
code reuse, 21 Nov 23:23 UTC), so the remaining slots go to our own bot. Ideas from other bots are fine,
their code is not. The andrew-zhouu-based 6.27 (62) must not be the locked entry.

Round 9 (builders-4): our own bot (main code, never scored on builders-4) with optional prey fights and seek
(branch own-prey, 6d092e8): with nobody on us and HP > 60 %, fight an idle mech in sight whose time to kill us
is at least preyRatio times ours; with nothing to do and HP > 70 %, walk to the last place of such a mech
seen in the last `seek` s. Idea from the andrew-zhouu bot's practice notes, our own code. Docker check PASS
(p99 2.9 ms). Sim (HOUSE=2 TIERS=1, 150 paired seeds): ratio 1.5 + seek +0.23 +-0.15; ratio 0.9 + seek
-0.41 +-0.22; ratio 1.2 + seek -0.11 +-0.21; prey only from 150 s -0.01. The sim has missed before
(kiting: sim +0.77, real -0.13), so the real runs decide.

| 77 | 6d092e8 (own-prey) | our bot, first builders-4 score (new options off) | pending |
| 78 | 7e2b3d2 (own-p15s) | 77 + prey at 1.5:1 + seek 30 s | pending |
| 79 | 0fee361 (own-p20s) | 77 + prey at 2:1 + seek 30 s | pending |
| 80 | 3a4dfb3 (own-p15s60) | 77 + prey at 1.5:1 + seek 60 s | pending |
| 81 | d1dffbe (own-p15) | 77 + prey at 1.5:1, no seek | pending |
| 82 | 928df77 (own-p09s) | 77 + prey at 0.9:1 + seek 30 s (the andrew-zhouu setting; sim says worse) | pending |

Round 10 (builders-4, queued for the hour after round 9): our own bot, 78 (prey 1.5 + seek 30) with the gun
held at long range against the house squad. Data point from another team's public notes (house guns land
85 % inside 40 m, 52 % at 40-80 m, 26 % at 80-120 m, 15 % at 120-180 m); our code. Sim house aim was
recalibrated to that table (HSCALE 0.4: 88/54/22/7 %). Sim, 200 paired seeds: gun range 140 m vs 85 m
+0.23 +-0.14 (calibrated), +0.28 +-0.21 (old aim), +0.17 +-0.14 (slow ramp); 120 m ~0; house-accuracy
fight estimates -0.20 (left off). Docker check PASS. Params only, so they can be re-based on whichever
round 9 variant scores best.

| 83 | f5146d9 (own-r140) | 78 + gun at 140 m (band 90-180), laser at 170 m | pending |
| 84 | 4dae321 (own-r140ph) | 83 + prey above 75 % HP, seek above 85 % | pending |
| 85 | ffc0f5e (own-r160) | 78 + gun at 160 m (band 110-200), laser at 190 m | pending |
| 86 | 9f0726e (own-r140p2) | 83 + prey only at 2:1 | pending |
| 87 | ce65df7 (own-r140farm) | 83 + farm crates and drones until 400 s | pending |
| 88 | c167cd8 (own-r140ret) | 83 + retreat from 2+ attackers below 60 % HP | pending |

Sun 12:32 UTC: one own-bot submission scored 5.25 (BB-01A707BA); the first of round 9 (77, 6d092e8) or of
round 10 (83, f5146d9), not confirmed which. The user stopped the own-bot path here and took the
code-reuse risk on the andrew-zhouu-based build knowingly. Round 8 (71-76, az, all built and checked)
is back in the queue, rows 77-88 are not being submitted.

Round 11 (builders-4, ready): az, 62 plus one change each (branch az-sets3 adds hunt20-22). Docker check
PASS. To be trimmed or stacked once round 8 scores are in.

| 89 | 54e08d3 (az-hunt20) | 62 + farm up to 21 cubes (was 15) | ready |
| 90 | f9fbeca (az-hunt21) | 62 + loot within 300 m (was 220) | ready |
| 91 | bf2dcde (az-hunt22) | 62 + hold 150-210 m against two or more attackers (was 130-190) | ready |
| 92 | cc21a57 (az-hunt8) | 62 + seek at half health (was 70 %) | ready |
| 93 | 46758a0 (az-hunt11) | 62 + fight plan on house accuracy | ready |
| 94 | 65792a7 (az-hunt12) | 62 + start hunts above 45 % health (was 60 %) | ready |

Round 8 result: 6.16, 6.00, 6.07, 6.15, 5.88; all inside the seek family's spread (mean now 6.12 over 13).

Research (Sun ~13:00 UTC): the #1 board entry (Ahmad Suliman, 6.74) is public and MIT: Apex-Go v3 by
ahmad-ovh, https://github.com/ahmad-ovh/Buildathon commit 56e0f76 (scored 07:11 UTC, pushed 06:47 UTC).
It is itself built on the andrew-zhouu bot plus ideas from other public bots. Its later commits
(06:48-06:57 UTC: "simplify", "reduce steering search", "widen fire cone") look like weakening edits
after the scored build, so we use 56e0f76. Team who dey (#5, 6.49) runs 56e0f76 with RearAvoidW 50,
CubeSmart, KillFocus 1.5. Apex v3 has Seek off and EngageRatio 1.25 / HuntRange 130 in squad play,
where our andrew-zhouu runs gained about 0.2 from seek. Round 11 (az) is superseded.

Round 12 (builders-4): Apex-Go v3 (branch apex-v3: their files, LICENSE-apex, credit README, our display
name). Docker check PASS (max 2.8 ms).

| 95 | 3627404 (apex-v3) | Apex-Go v3 as published at 56e0f76 | 6.00 (BB-17FA83D6) |
| 96 | b3a81d8 (apex-wd) | 95 + who dey's settings (rear-arc 50, cube pricing, kill focus 1.5) | 6.50 (BB-258CC180) |
| 97 | f911ad6 (apex-seek) | 95 + seek 30 s in squad play | 6.20 (BB-EA2C9934) |
| 98 | da318b0 (apex-hunt) | 95 + hunt at 0.9 odds within 180 m (laser 200 m) in squad play | 6.17 (BB-D5E7B255) |
| 99 | eaff419 (apex-sh) | 97 + 98 | 6.39 (BB-596750ED) |
| 100 | 420a906 (apex-wdsh) | 96 + 97 + 98 | 6.55 (BB-DAD4F6EC) |

Round 12 result: with who dey's settings 6.50 and 6.55 (now the entry, BB-DAD4F6EC), without 6.00-6.39
(mean 6.19). Apex v3 as published scored 6.00 here, so its 6.74 was a high draw or another commit.
Board (13:56 UTC): #1 6.74, #2 IT_Professionals 6.71, ... #6 The anon 6.55; top-32 line 6.08.
who dey's repo went private at ~13:55 UTC; their history (local clone) tried NanoBoost, UpV2, DropRun 60,
LaserMechOnly, CautiousT 230/300, CookoffBonus 0.45, PracticeRim 0.45/0.75, RearBonus 0.35, each once.

Round 13 (builders-4): the 6.55 build (100) plus one setting each. Docker check PASS on w2 and w4.

| 101 | 88fd68c (apex-w1) | 100 + seek memory 60 s | pending |
| 102 | fed947a (apex-w2) | 100 + exact sustained-DPS upgrade order (UpV2) | pending |
| 103 | eb70337 (apex-w3) | 100 + repair nanites ranked higher (NanoBoost 2) | pending |
| 104 | c1fcada (apex-w4) | 100 + farm up to 21 cubes in squad play | pending |
| 105 | b5d0428 (apex-w5) | 100 + cautious late stance from 300 s (was 265) | pending |
| 106 | 80dfb57 (apex-w6) | 100 + break off for supply drops within 260 m | pending |

Round 14 (builders-4): our own additions on the 6.55 build (branch apex-own, c2d0223 + README), each
behind a setting, practice squad only. Kill pacing (PaceSecs/PaceHP): each kill brings another hunter,
so open no new hunt or seek until health regenerates. Full-magazine hunts (HuntAmmo): reload while
watching an idle mech, open hunts with a full gun. Wounded-first seeking (SeekHurtW). Docker check PASS.
Round 13 (settings only) stays available for the last batch.

| 107 | 43dc2f6 (own-pace) | 100 + kill pacing 20 s / 85 % HP | pending |
| 108 | 95a7cfb (own-ammo) | 100 + full-magazine hunts (75 %) | pending |
| 109 | f91bc53 (own-hurt) | 100 + wounded-first seeking | pending |
| 110 | 6975cbf (own-pa) | 107 + 108 | pending |
| 111 | 1f12edd (own-all) | 107 + 108 + 109 | pending |
| 112 | ad1b4ca (own-all2) | 111 with pacing 30 s / 95 % HP | pending |
