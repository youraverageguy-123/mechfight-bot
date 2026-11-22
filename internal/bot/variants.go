package bot

// Variants are named parameter tweaks for A/B runs in the local arena (roster names).
func Variants() map[string]func(p *Params) {
	m := map[string]func(p *Params){
		// the published v0.5 behaviour (stealth in free-for-alls, hit rates as believed then)
		"v05": func(p *Params) { v05(p) },
		// v0.5 plus the visibility-map hideouts tried on Saturday morning
		"v05-hm": func(p *Params) { v05(p); p.UseHideMap = true; p.QuietHoldS = 3 },
		// never adapts to the lobby: always the squad (fighter) stance
		"fighter": func(p *Params) { v05(p); p.PracticeAware = false },
		// always the stealth stance, from the first tick
		"ghost": func(p *Params) { p.ForceFFA = true },
		// stealth but no looting at all
		"ghost-noloot": func(p *Params) { p.ForceFFA = true; p.FFALootRadius = 0 },
		// hideouts from the visibility map
		"nohm":     func(p *Params) { p.UseHideMap = false },
		"hm-near":  func(p *Params) { p.HideDistW = 0.30 },
		"hm-far":   func(p *Params) { p.HideDistW = 0.08 },
		"hm-hit":   func(p *Params) { p.HideSeenW, p.HideHitW = 0.2, 1.3 },
		"hm-seen":  func(p *Params) { p.HideSeenW, p.HideHitW = 1.2, 0.4 },
		"hm-noroof": func(p *Params) { p.UseRoofs = false },
		"sfr70":    func(p *Params) { p.StealthFightRange = 70 },
		"sfr100":   func(p *Params) { p.StealthFightRange = 100 },
		"sfr200":   func(p *Params) { p.StealthFightRange = 200 },
		// squad stance: farm first, kill later
		"patient60":  func(p *Params) { p.PatientSecs = 60 },
		"patient120": func(p *Params) { p.PatientSecs = 120 },
		"patient200": func(p *Params) { p.PatientSecs = 200; p.PatientCubes = 15 },
		// kiting several attackers
		"nokite":   func(p *Params) { p.KiteMin = 0 },
		"kite80":   func(p *Params) { p.KiteMin, p.KiteMax = 80, 130 },
		"kite140":  func(p *Params) { p.KiteMin, p.KiteMax = 140, 200 },
		// opening: roof before weapons
		"rooffirst":   func(p *Params) { p.RoofFirstSecs = 40; p.RoofFirstDist = 90 },
		"rooffirst60": func(p *Params) { p.RoofFirstSecs = 60; p.RoofFirstDist = 150 },
		// multi-attacker leave threshold
		"ttd6":  func(p *Params) { p.LeaveTTD = 6 },
		"ttd16": func(p *Params) { p.LeaveTTD = 16 },
		// never leave a one-on-one
		"duel0": func(p *Params) { p.UseRoofs = false; p.DuelDisengage = 0 },
		"duel25": func(p *Params) { p.UseRoofs = false; p.DuelDisengage = 0.25 },
		"duel75": func(p *Params) { p.UseRoofs = false; p.DuelDisengage = 0.75 },
		// no rooftop hiding
		"noroof": func(p *Params) { p.UseRoofs = false },
		// rooftops in the squad stance as well
		"roofsquad": func(p *Params) { p.RoofInSquad = true },
		// fighter stance without roofs (the pre-roof baseline for the squad stance)
		"fighter-noroof": func(p *Params) { p.PracticeAware = false; p.UseRoofs = false },
		// stealth, greedier looting
		"ghost-greedy": func(p *Params) { p.ForceFFA = true; p.FFALootRadius = 260; p.FFAWantCubes = 20 },

		// --- after the first real free-for-all matches (stealth scored 4.8 of 15) ---------------------
		// always fight, with the measured hit rates instead of the believed ones
		"f-hit": func(p *Params) { p.PracticeAware = false; p.HitModel = 1 },
		// ... and the fight plan that counts our own fire while changing range
		"f-plan": func(p *Params) { p.PracticeAware = false; p.HitModel = 1; p.PlanV2 = true },
		// ... and no backing away from several attackers without a laser
		"f-nk": func(p *Params) { p.PracticeAware = false; p.HitModel = 1; p.PlanV2 = true; p.KiteNeedsLaser = true },
		// ... and a laser is worth going for
		"f-las": func(p *Params) { brawl(p) },
		"f-las4": func(p *Params) { brawl(p); p.LaserValue = 4 },
		"f-las8": func(p *Params) { brawl(p); p.LaserValue = 8; p.ArmSwapMin = 0.5 },
		// ... and the running exchange corrects the estimate
		"f-ex":  func(p *Params) { brawl(p); p.ExchangeW = 0.6 },
		"f-ex1": func(p *Params) { brawl(p); p.ExchangeW = 1.0 },
		// leave a one-on-one sooner / later
		"f-d35": func(p *Params) { brawl(p); p.DuelDisengage = 0.35 },
		"f-d70": func(p *Params) { brawl(p); p.DuelDisengage = 0.70 },
		"f-d90": func(p *Params) { brawl(p); p.DuelDisengage = 0.90 },
		// pick fights more / less readily
		"f-e10": func(p *Params) { brawl(p); p.EngageRatio = 1.0 },
		"f-e16": func(p *Params) { brawl(p); p.EngageRatio = 1.6 },
		"f-nohunt": func(p *Params) { brawl(p); p.Hunt = false },
		// only the measured tables, old stance logic (does the model alone change anything?)
		"hit1": func(p *Params) { p.HitModel = 1; p.PlanV2 = true },

		// "brawl": practice path as published, free-for-all (and the first 24 s) as an armed farmer
		"brawl":    func(p *Params) { brawl2(p) },
		"b-stand":  func(p *Params) { brawl2(p); p.AvoidAdv = 0 },            // never slip: always fight back
		"b-av10":   func(p *Params) { brawl2(p); p.AvoidAdv = 1.0 },          // slip only from losing exchanges
		"b-av16":   func(p *Params) { brawl2(p); p.AvoidAdv = 1.6 },
		"b-avr60":  func(p *Params) { brawl2(p); p.AvoidRange = 60 },
		"b-avr120": func(p *Params) { brawl2(p); p.AvoidRange = 120 },
		"b-cov90":  func(p *Params) { brawl2(p); p.AvoidCover = 90 },
		"b-las3":   func(p *Params) { brawl2(p); p.LaserValue = 3 },
		"b-las10":  func(p *Params) { brawl2(p); p.LaserValue = 10; p.ArmSwapMin = 0.5 },
		"b-e10":    func(p *Params) { brawl2(p); p.BrawlEngage = 1.0 },
		"b-e18":    func(p *Params) { brawl2(p); p.BrawlEngage = 1.8 },
		"b-nohunt": func(p *Params) { brawl2(p); p.BrawlHunt = false },
		"b-hl200":  func(p *Params) { brawl2(p); p.HuntRangeLaser = 200 },
		"b-ex":     func(p *Params) { brawl2(p); p.ExchangeW = 0.7 },
		"b-c10":    func(p *Params) { brawl2(p); p.BrawlWantCubes = 10 },
		"b-c25":    func(p *Params) { brawl2(p); p.BrawlWantCubes = 25 },
		"b-lr120":  func(p *Params) { brawl2(p); p.BrawlLootRadius = 120 },
		"b-d35":    func(p *Params) { brawl2(p); p.DuelDisengage = 0.35 },
		"b-d75":    func(p *Params) { brawl2(p); p.DuelDisengage = 0.75 },
		"b-oldhit": func(p *Params) { brawl2(p); p.HitModel = 0 },
		// how long we assume a free-for-all before settling on the squad stance (999: brawl always)
		"b-open45":  func(p *Params) { p.OpenSecs = 45 },
		"b-open90":  func(p *Params) { p.OpenSecs = 90 },
		"b-open999": func(p *Params) { p.OpenSecs = 9999 },
		// gun lead: half lead at long flight times; scaled by persistence; learned per opponent
		"b-oldlead": func(p *Params) { oldLead(p) },
		"b-damp5":   func(p *Params) { oldLead(p); p.LeadDamp = 0.5 },
		"b-pers":    func(p *Params) { oldLead(p); p.LeadDamp = 0.5; p.LeadPersist = true },
		"b-cum":     func(p *Params) { oldLead(p); p.LeadDamp = 0.5; p.LeadCum = true },
		"b-perscum": func(p *Params) { oldLead(p); p.LeadDamp = 0.5; p.LeadPersist = true; p.LeadCum = true },
		"b-pc75":    func(p *Params) { p.LeadDamp = 0.75 },
		"b-pc100":   func(p *Params) { p.LeadDamp = 1.0 },
		// faster strafe reversals in a fight (the real jitterers reverse every 0.3-0.6 s)
		"b-jit":   func(p *Params) { p.StrafeMin, p.StrafeMax = 0.25, 0.55 },
		"b-jit2":  func(p *Params) { p.StrafeMin, p.StrafeMax = 0.2, 0.42 },
		"b-jitj":  func(p *Params) { p.StrafeMin, p.StrafeMax = 0.25, 0.55; p.Jink = 0.6 },
		// v0.7 and v0.7.1 exactly as published
		"v07":  func(p *Params) { v07(p) },
		"v071": func(p *Params) { v074(p); p.EdgeHold = 0; p.NoMortar = false; p.RunOnRange = 0; p.CoverMargin = 0 },
		"v072": func(p *Params) { v074(p); p.NoMortar = false; p.RunOnRange = 0; p.CoverMargin = 0 },
		"v073": func(p *Params) { v074(p); p.RunOnRange = 0; p.CoverMargin = 0 },
		"v074": func(p *Params) { v074(p) },
		"b-standrun": func(p *Params) { p.RetreatRunMax = 50 },
		// the opening rule (position before loot while a mech is in view): off, and other settings
		// hybrids: this build until the lobby is known to be a free-for-all, then another set
		"v05fix":     func(p *Params) { v05(p); p.PathFix = true; p.CreditFix = true },
		"h-v05":      func(p *Params) { p.FFASwap = "v05" },
		"h-v05fix":   func(p *Params) { p.FFASwap = "v05fix" },
		// ... and with the v0.5 opening as well: h3 = v0.5 opening then this build, h3s = v0.5 opening,
		// v0.5 in a free-for-all, this build only against the practice squad
		// v0.7.4 to the letter until a free-for-all is known (so its practice score, 6.23, stays), then:
		// p74a = the three fixes from its real matches; p74b = those, the roof first with farming once
		// it is quiet, and healing before farming
		"ffa-fix": func(p *Params) {
			p.QuietSlip, p.QuietSlipMin, p.LaserMechOnly, p.CrateGiveUp, p.NpcGiveUp = true, 40, true, 5, 8
		},
		"ffa-fixrf": func(p *Params) {
			p.QuietSlip, p.QuietSlipMin, p.LaserMechOnly, p.CrateGiveUp, p.NpcGiveUp = true, 40, true, 5, 8
			p.BrawlRoofAfter, p.RoofFarmAfter = 0, 45
			p.HealFirst, p.HealUntil = 0.75, 0.95
		},
		// p74c = p74b with the v0.5 play (path fix in) from detection to 50 s, and detection by the
		// gunfire of the opening as well as by deaths; p74d = the same without the gunfire detector
		"p74c": func(p *Params) {
			v074(p)
			p.SoundFFA, p.SoundFFAUntil = 2, 10
			p.FFASwap, p.FFALate, p.FFALateT = "v05fix", "ffa-fixrf", 50
		},
		"p74d": func(p *Params) { v074(p); p.FFASwap, p.FFALate, p.FFALateT = "v05fix", "ffa-fixrf", 50 },
		// p74f = p74d without mortars in the quiet phase (a laser hit does 60% more to a mortar carrier)
		"v05fixnm": func(p *Params) { v05(p); p.PathFix = true; p.CreditFix = true; p.NoMortarHard = true },
		"p74f":     func(p *Params) { v074(p); p.FFASwap, p.FFALate, p.FFALateT = "v05fixnm", "ffa-fixrf", 50 },
		// the quiet phase after detection, holding fire until touched as well
		"v05fixq": func(p *Params) {
			v05(p)
			p.PathFix, p.CreditFix, p.NoMortarHard, p.StealthQuiet = true, true, true, true
		},
		"p74q": func(p *Params) { v074(p); p.FFASwap, p.FFALate, p.FFALateT = "v05fixq", "ffa-fixrf", 50 },
		// ... and with a memory for who has hurt us. (A real match: a mech that had shot us five seconds
		// earlier walked back from 86 m to lightning range while we held fire, the four-second window
		// having made it a bystander again.)
		"v05fixq2": func(p *Params) {
			v05(p)
			p.PathFix, p.CreditFix, p.NoMortarHard, p.StealthQuiet = true, true, true, true
			p.HostileMem = 30
		},
		"ffa-fixrf2": func(p *Params) {
			p.QuietSlip, p.QuietSlipMin, p.LaserMechOnly, p.CrateGiveUp, p.NpcGiveUp = true, 40, true, 5, 8
			p.BrawlRoofAfter, p.RoofFarmAfter = 0, 45
			p.HealFirst, p.HealUntil = 0.75, 0.95
			p.HostileMem = 30
		},
		"p74q2":   func(p *Params) { v074(p); p.FFASwap, p.FFALate, p.FFALateT = "v05fixq2", "ffa-fixrf2", 50 },
		// ... and a hurt mech shoots the drone that is plinking at it (RetreatPest)
		"v05fixq3": func(p *Params) {
			v05(p)
			p.PathFix, p.CreditFix, p.NoMortarHard, p.StealthQuiet = true, true, true, true
			p.HostileMem, p.RetreatPest = 30, true
		},
		"ffa-fixrf3": func(p *Params) {
			p.QuietSlip, p.QuietSlipMin, p.LaserMechOnly, p.CrateGiveUp, p.NpcGiveUp = true, 40, true, 5, 8
			p.BrawlRoofAfter, p.RoofFarmAfter = 0, 45
			p.HealFirst, p.HealUntil = 0.75, 0.95
			p.HostileMem, p.RetreatPest = 30, true
		},
		"p74q3": func(p *Params) { v074(p); p.FFASwap, p.FFALate, p.FFALateT = "v05fixq3", "ffa-fixrf3", 50 },
		// ... and out towards the edge of the circle, away from the others (EdgeBias)
		"v05fixq4": func(p *Params) {
			v05(p)
			p.PathFix, p.CreditFix, p.NoMortarHard, p.StealthQuiet = true, true, true, true
			p.HostileMem, p.RetreatPest, p.EdgeBias = 30, true, 1
		},
		"ffa-fixrf4": func(p *Params) {
			p.QuietSlip, p.QuietSlipMin, p.LaserMechOnly, p.CrateGiveUp, p.NpcGiveUp = true, 40, true, 5, 8
			p.BrawlRoofAfter, p.RoofFarmAfter = 0, 45
			p.HealFirst, p.HealUntil = 0.75, 0.95
			p.HostileMem, p.RetreatPest, p.EdgeBias = 30, true, 1
		},
		"p74q4": func(p *Params) { v074(p); p.FFASwap, p.FFALate, p.FFALateT = "v05fixq4", "ffa-fixrf4", 50 },
		"v05fixq5": func(p *Params) {
			v05(p)
			p.PathFix, p.CreditFix, p.NoMortarHard, p.StealthQuiet = true, true, true, true
			p.HostileMem, p.RetreatPest, p.EdgeBias = 30, true, 2
		},
		"ffa-fixrf5": func(p *Params) {
			p.QuietSlip, p.QuietSlipMin, p.LaserMechOnly, p.CrateGiveUp, p.NpcGiveUp = true, 40, true, 5, 8
			p.BrawlRoofAfter, p.RoofFarmAfter = 0, 45
			p.HealFirst, p.HealUntil = 0.75, 0.95
			p.HostileMem, p.RetreatPest, p.EdgeBias = 30, true, 2
		},
		"p74q5": func(p *Params) { v074(p); p.FFASwap, p.FFALate, p.FFALateT = "v05fixq5", "ffa-fixrf5", 50 },
		"v05fixq6": func(p *Params) {
			v05(p)
			p.PathFix, p.CreditFix, p.NoMortarHard, p.StealthQuiet = true, true, true, true
			p.HostileMem, p.RetreatPest, p.EdgeBias, p.EdgeBand = 30, true, 2, 0.6
		},
		"ffa-fixrf6": func(p *Params) {
			p.QuietSlip, p.QuietSlipMin, p.LaserMechOnly, p.CrateGiveUp, p.NpcGiveUp = true, 40, true, 5, 8
			p.BrawlRoofAfter, p.RoofFarmAfter = 0, 45
			p.HealFirst, p.HealUntil = 0.75, 0.95
			p.HostileMem, p.RetreatPest, p.EdgeBias, p.EdgeBand = 30, true, 2, 0.6
		},
		"p74q6": func(p *Params) { v074(p); p.FFASwap, p.FFALate, p.FFALateT = "v05fixq6", "ffa-fixrf6", 50 },
		"v05fixq7": func(p *Params) {
			v05(p)
			p.PathFix, p.CreditFix, p.NoMortarHard, p.StealthQuiet = true, true, true, true
			p.HostileMem, p.RetreatPest, p.EdgeBias, p.EdgeBand = 30, true, 3, 0.75
		},
		"ffa-fixrf7": func(p *Params) {
			p.QuietSlip, p.QuietSlipMin, p.LaserMechOnly, p.CrateGiveUp, p.NpcGiveUp = true, 40, true, 5, 8
			p.BrawlRoofAfter, p.RoofFarmAfter = 0, 45
			p.HealFirst, p.HealUntil = 0.75, 0.95
			p.HostileMem, p.RetreatPest, p.EdgeBias, p.EdgeBand = 30, true, 3, 0.75
		},
		"p74q7": func(p *Params) { v074(p); p.FFASwap, p.FFALate, p.FFALateT = "v05fixq7", "ffa-fixrf7", 50 },
		"v05fixq8": func(p *Params) {
			v05(p)
			p.PathFix, p.CreditFix, p.NoMortarHard, p.StealthQuiet = true, true, true, true
			p.HostileMem, p.RetreatPest, p.EdgeBias, p.EdgeBand, p.EdgeClamp = 30, true, 3, 0.75, true
		},
		"ffa-fixrf8": func(p *Params) {
			p.QuietSlip, p.QuietSlipMin, p.LaserMechOnly, p.CrateGiveUp, p.NpcGiveUp = true, 40, true, 5, 8
			p.BrawlRoofAfter, p.RoofFarmAfter = 0, 45
			p.HealFirst, p.HealUntil = 0.75, 0.95
			p.HostileMem, p.RetreatPest, p.EdgeBias, p.EdgeBand, p.EdgeClamp = 30, true, 3, 0.75, true
		},
		"p74q8": func(p *Params) { v074(p); p.FFASwap, p.FFALate, p.FFALateT = "v05fixq8", "ffa-fixrf8", 50 },
		// q9 = q7 + care before a climb in the quiet phase as well, and for mechs seen in the last 4 s
		"v05fixq9": func(p *Params) {
			v05(p)
			p.PathFix, p.CreditFix, p.NoMortarHard, p.StealthQuiet = true, true, true, true
			p.HostileMem, p.RetreatPest, p.EdgeBias, p.EdgeBand = 30, true, 3, 0.75
			p.ClimbCheckAll, p.ClimbClearMem, p.ClimbClearRange = true, 4, 170
		},
		"ffa-fixrf9": func(p *Params) {
			p.QuietSlip, p.QuietSlipMin, p.LaserMechOnly, p.CrateGiveUp, p.NpcGiveUp = true, 40, true, 5, 8
			p.BrawlRoofAfter, p.RoofFarmAfter = 0, 45
			p.HealFirst, p.HealUntil = 0.75, 0.95
			p.HostileMem, p.RetreatPest, p.EdgeBias, p.EdgeBand = 30, true, 3, 0.75
			p.ClimbCheckAll, p.ClimbClearMem, p.ClimbClearRange = true, 4, 170
		},
		"p74q9": func(p *Params) { v074(p); p.FFASwap, p.FFALate, p.FFALateT = "v05fixq9", "ffa-fixrf9", 50 },
		// The other way to play the first minute of a known free-for-all: no stealth phase; arm and farm
		// at once, holding fire until touched, out towards the edge and away from the others (the brawl
		// stance of ffa-fixrf9, the roof only once armed for it). In the real matches the stealth phase
		// often has us unarmed and without a cube at 30-40 s when an armed aggressor finds us.
		"ffa-fixc9": func(p *Params) {
			p.QuietSlip, p.QuietSlipMin, p.LaserMechOnly, p.CrateGiveUp, p.NpcGiveUp = true, 40, true, 5, 8
			p.RoofFarmAfter = 45
			p.HealFirst, p.HealUntil = 0.75, 0.95
			p.HostileMem, p.RetreatPest, p.EdgeBias, p.EdgeBand = 30, true, 3, 0.75
			p.ClimbCheckAll, p.ClimbClearMem, p.ClimbClearRange = true, 4, 170
		},
		"p74c9": func(p *Params) { v074(p); p.FFASwap = "ffa-fixc9" },
		// q10 = q9 + keep away from where shooting was heard in the last 8 s (EdgeNoise)
		"v05fixq10": func(p *Params) {
			v05(p)
			p.PathFix, p.CreditFix, p.NoMortarHard, p.StealthQuiet = true, true, true, true
			p.HostileMem, p.RetreatPest, p.EdgeBias, p.EdgeBand = 30, true, 3, 0.75
			p.ClimbCheckAll, p.ClimbClearMem, p.ClimbClearRange = true, 4, 170
			p.EdgeNoise = 8
		},
		"ffa-fixrf10": func(p *Params) {
			p.QuietSlip, p.QuietSlipMin, p.LaserMechOnly, p.CrateGiveUp, p.NpcGiveUp = true, 40, true, 5, 8
			p.BrawlRoofAfter, p.RoofFarmAfter = 0, 45
			p.HealFirst, p.HealUntil = 0.75, 0.95
			p.HostileMem, p.RetreatPest, p.EdgeBias, p.EdgeBand = 30, true, 3, 0.75
			p.ClimbCheckAll, p.ClimbClearMem, p.ClimbClearRange = true, 4, 170
			p.EdgeNoise = 8
		},
		"p74q10": func(p *Params) { v074(p); p.FFASwap, p.FFALate, p.FFALateT = "v05fixq10", "ffa-fixrf10", 50 },
		// A third way: the quiet phase on the ground only (no roof climb, no walking about for a weapon:
		// the nearest hideout and stay there), as the teams that die least in the first minute do.
		"v05fixqd": func(p *Params) {
			v05(p)
			p.PathFix, p.CreditFix, p.NoMortarHard, p.StealthQuiet = true, true, true, true
			p.HostileMem, p.RetreatPest, p.EdgeBias, p.EdgeBand = 30, true, 3, 0.75
			p.ClimbCheckAll, p.ClimbClearMem, p.ClimbClearRange = true, 4, 170
			p.UseRoofs, p.NoStealthRoam = false, true
		},
		"p74d9": func(p *Params) { v074(p); p.FFASwap, p.FFALate, p.FFALateT = "v05fixqd", "ffa-fixrf9", 50 },
		// ... and the same with the roofs kept but no walking about
		"v05fixqe": func(p *Params) {
			v05(p)
			p.PathFix, p.CreditFix, p.NoMortarHard, p.StealthQuiet = true, true, true, true
			p.HostileMem, p.RetreatPest, p.EdgeBias, p.EdgeBand = 30, true, 3, 0.75
			p.ClimbCheckAll, p.ClimbClearMem, p.ClimbClearRange = true, 4, 170
			p.NoStealthRoam = true
		},
		"p74e9": func(p *Params) { v074(p); p.FFASwap, p.FFALate, p.FFALateT = "v05fixqe", "ffa-fixrf9", 50 },
		"o-quiet3": func(p *Params) { p.QuietSlip, p.QuietSlipMin, p.HostileMem = true, 40, 30 },
		// o-roof: the v0.5 opening (stealth: a weapon close by, then a roof, out of sight), holding fire
		// until touched. In the real matches v0.5 died inside a minute in 2 of 24 (10 AM to 1 PM) where
		// the ground opening of the later builds dies in a third; its practice cost was about 0.1 a match
		// (v0.9, v0.9.1), which the seat tests can now price seat by seat.
		"o-roof": func(p *Params) {
			v05(p)
			p.PathFix, p.CreditFix, p.NoMortarHard, p.StealthQuiet = true, true, true, true
			p.HostileMem = 30
		},
		"x-huntr":  func(p *Params) { pfx(p); p.SquadSet = "sq-hunt"; p.OpenSet = "o-roof" },
		"x-huntv5": func(p *Params) { pfx(p); p.SquadSet = "sq-hunt"; p.OpenSet = "v05fix" },
		"gp-c71":   func(p *Params) { pfx(p); p.SlotPlan = "hunt+quiet.0,hunt+quiet.1,hunt.0,hunt+quiet.0" },
		"gp-c88": func(p *Params) {
			pfx(p)
			p.SlotPlan = "hunt+quiet.0,hunt+quiet.1,hunt.0,hN2+quiet.2,hunt+quiet.0,hunt+quiet.1,hunt.0,hunt+quiet.0,hunt+quiet.0,hunt+quiet.1,hunt.0,hunt+quiet.0,hunt+quiet.0,hunt+quiet.1,hunt.0,hunt+quiet.0"
		},
		// Practice experiments: v0.7.4 for the first 24 s, the v0.13 play (no mortars) in a free-for-all,
		// and a different practice stance each. Only the official score can judge them; whatever they
		// score, free-for-all play is the same in all of them.
		"sq-roof":  func(p *Params) { p.RoofInSquad = true },
		"sq-hunt":  func(p *Params) { p.EngageRatio = 0.9; p.HuntRange = 180; p.HuntRangeLaser = 200 },
		// v0.24 (sq-hunt) scored 6.29, our best: more of the same, in two directions and both together
		// hunt2 = hunts at longer range, on thinner odds and with less health in hand
		"sq-hunt2": func(p *Params) { p.EngageRatio = 0.7; p.HuntRange = 220; p.HuntRangeLaser = 250; p.HuntHP = 0.5 },
		// hunt3 = sq-hunt, and goes looking for the next one instead of waiting in a hide
		"sq-hunt3": func(p *Params) { p.EngageRatio = 0.9; p.HuntRange = 180; p.HuntRangeLaser = 200; p.Seek = 30 },
		"sq-hunt4": func(p *Params) {
			p.EngageRatio = 0.7; p.HuntRange = 220; p.HuntRangeLaser = 250; p.HuntHP = 0.5
			p.Seek = 30
		},
		// hunt5 = sq-hunt with the fight plan on the house bots' measured accuracy; hunt6 = sq-hunt
		// stopping the farming at 10 cubes (stats stop growing there), so the hunting starts sooner
		"sq-hunt5": func(p *Params) { p.EngageRatio = 0.9; p.HuntRange = 180; p.HuntRangeLaser = 200; p.HouseModel = true },
		"sq-hunt6": func(p *Params) { p.EngageRatio = 0.9; p.HuntRange = 180; p.HuntRangeLaser = 200; p.WantCubes = 10 },
		"sq-hunt7": func(p *Params) {
			p.EngageRatio = 0.9; p.HuntRange = 180; p.HuntRangeLaser = 200
			p.Seek, p.WantCubes = 30, 10
		},
		// Opening sets (OpenSet: played until the lobby is known or the opening is over).
		// o-quiet: we do not fire first on a mech that has not touched us. Real matches, 4-8 PM Saturday:
		// our mech was hit by a mech inside 20 s in 52% of matches (TigerTown27: 19%), and in 64% of those
		// cases it had hit that mech first; three of its seven deaths inside a minute began that way.
		"o-quiet":  func(p *Params) { p.QuietSlip, p.QuietSlipMin = true, 40 },
		"o-quiet2": func(p *Params) { p.QuietSlip, p.QuietSlipMin, p.LaserMechOnly = true, 40, true },
		// o-shy: hold fire, and position before loot: while a mech we know of can see us, only loot that
		// is out of its sight and no nearer to it, else out of its sight first (the rule of the shy phase
		// after detection, here from the first second). o-shy12: the same for the first 12 s only.
		"o-shy": func(p *Params) {
			p.QuietSlip, p.QuietSlipMin = true, 40
			p.ShyOpen, p.ShySecs, p.ShyRange, p.ShyLaser, p.ShyMem, p.ShyBoost = true, 24, 220, 60, 6, 40
		},
		"o-shy12": func(p *Params) {
			p.QuietSlip, p.QuietSlipMin = true, 40
			p.ShyOpen, p.ShySecs, p.ShyRange, p.ShyLaser, p.ShyMem, p.ShyBoost = true, 12, 220, 60, 6, 40
		},
		// (emulation only) how much of the practice score hangs on the length of the "lobby not known" phase
		"x-ho0":     func(p *Params) { pfx(p); p.SquadSet = "sq-hunt"; p.OpenSecs = 0 },
		"x-ho8":     func(p *Params) { pfx(p); p.SquadSet = "sq-hunt"; p.OpenSecs = 8 },
		"x-ho16":    func(p *Params) { pfx(p); p.SquadSet = "sq-hunt"; p.OpenSecs = 16 },
		"x-ho40":    func(p *Params) { pfx(p); p.SquadSet = "sq-hunt"; p.OpenSecs = 40 },
		// o-far: hold fire, and prefer errands that do not take us towards a mech we know of
		"o-far":     func(p *Params) { p.QuietSlip, p.QuietSlipMin, p.OpenAway = true, 40, 0.6 },
		"o-far9":    func(p *Params) { p.QuietSlip, p.QuietSlipMin, p.OpenAway = true, 40, 0.9 },
		"x-huntf":   func(p *Params) { pfx(p); p.SquadSet = "sq-hunt"; p.OpenSet = "o-far" },
		"x-huntf9":  func(p *Params) { pfx(p); p.SquadSet = "sq-hunt"; p.OpenSet = "o-far9" },
		"x-hunts":   func(p *Params) { pfx(p); p.SquadSet = "sq-hunt"; p.OpenSet = "o-shy" },
		"x-hunts12": func(p *Params) { pfx(p); p.SquadSet = "sq-hunt"; p.OpenSet = "o-shy12" },
		"x-huntq":  func(p *Params) { pfx(p); p.SquadSet = "sq-hunt"; p.OpenSet = "o-quiet" },
		"x-huntq2": func(p *Params) { pfx(p); p.SquadSet = "sq-hunt"; p.OpenSet = "o-quiet2" },
		"x-hunt7q": func(p *Params) { pfx(p); p.SquadSet = "sq-hunt7"; p.OpenSet = "o-quiet" },
		"x-huntq3": func(p *Params) { pfx(p); p.SquadSet = "sq-hunt"; p.OpenSet = "o-quiet3" },
		// A free-for-all known from the first tick (what a build does once its FFAFrom time has passed):
		// g0 = the quiet set at once; g0s = with the position-first rule for the first 25 s; g0d, g0e =
		// the ground-only and no-roaming quiet sets; g0c = arm and farm at once.
		"v05fixq9s": func(p *Params) {
			v05(p)
			p.PathFix, p.CreditFix, p.NoMortarHard, p.StealthQuiet = true, true, true, true
			p.HostileMem, p.RetreatPest, p.EdgeBias, p.EdgeBand = 30, true, 3, 0.75
			p.ClimbCheckAll, p.ClimbClearMem, p.ClimbClearRange = true, 4, 170
			p.ShySecs, p.ShyRange, p.ShyLaser, p.ShyMem, p.ShyBoost = 25, 220, 60, 6, 40
		},
		// "bunker" quiet sets: stay by the spawn. (Re-enacted real matches: a mech that never leaves its
		// spawn spends 0.20 of the danger measure in the first 30 s; our quiet set from the first tick
		// 0.33; the real field 0.33; the two best free-for-all teams 0.24-0.26.)
		"v05fixb1": func(p *Params) {
			v05(p)
			p.PathFix, p.CreditFix, p.NoMortarHard, p.StealthQuiet = true, true, true, true
			p.HostileMem, p.RetreatPest, p.EdgeBias, p.EdgeBand = 30, true, 3, 0.75
			p.ClimbCheckAll, p.ClimbClearMem, p.ClimbClearRange = true, 4, 170
			p.UseRoofs, p.NoStealthRoam = false, true
			p.ArmRadius, p.FFALootRadius = 40, 40
		},
		"v05fixb2": func(p *Params) {
			v05(p)
			p.PathFix, p.CreditFix, p.NoMortarHard, p.StealthQuiet = true, true, true, true
			p.HostileMem, p.RetreatPest, p.EdgeBias, p.EdgeBand = 30, true, 3, 0.75
			p.ClimbCheckAll, p.ClimbClearMem, p.ClimbClearRange = true, 4, 170
			p.UseRoofs, p.NoStealthRoam = false, true
			p.ArmRadius, p.FFALootRadius = 25, 25
		},
		"v05fixb3": func(p *Params) {
			v05(p)
			p.PathFix, p.CreditFix, p.NoMortarHard, p.StealthQuiet = true, true, true, true
			p.HostileMem, p.RetreatPest, p.EdgeBias, p.EdgeBand = 30, true, 0, 0
			p.ClimbCheckAll, p.ClimbClearMem, p.ClimbClearRange = true, 4, 170
			p.UseRoofs, p.NoStealthRoam = false, true
			p.ArmRadius, p.FFALootRadius = 25, 25
			p.HideNearW = 1.0
		},
		// ... and with nearness to known mechs as a cost when choosing where to hide (KeepAway)
		"v05fixk1": func(p *Params) { bunker(p, 25, 1.0); p.KeepAway, p.KeepAwayMem, p.KeepAwayStay = 200, 12, 20 },
		"v05fixk2": func(p *Params) { bunker(p, 25, 0.5); p.KeepAway, p.KeepAwayMem, p.KeepAwayStay = 200, 12, 20 },
		"v05fixk3": func(p *Params) { bunker(p, 25, 0.5); p.KeepAway, p.KeepAwayMem, p.KeepAwayStay = 400, 12, 20 },
		"v05fixk4": func(p *Params) { bunker(p, 25, 0.3); p.KeepAway, p.KeepAwayMem, p.KeepAwayStay = 400, 20, 30 },
		"v05fixk5": func(p *Params) { bunker(p, 40, 0.5); p.KeepAway, p.KeepAwayMem, p.KeepAwayStay = 400, 12, 20 },
		"v05fixk6":  func(p *Params) { bunker(p, 40, 0.5); p.KeepAway, p.KeepAwayMem, p.KeepAwayStay = 800, 12, 20 },
		"v05fixk7":  func(p *Params) { bunker(p, 40, 0.3); p.KeepAway, p.KeepAwayMem, p.KeepAwayStay = 800, 20, 20 },
		"v05fixk8":  func(p *Params) { bunker(p, 40, 0.5); p.KeepAway, p.KeepAwayMem, p.KeepAwayStay = 400, 20, 10 },
		"v05fixk9":  func(p *Params) { bunker(p, 60, 0.5); p.KeepAway, p.KeepAwayMem, p.KeepAwayStay = 800, 20, 20 },
		"v05fixk10": func(p *Params) { bunker(p, 40, 0.2); p.KeepAway, p.KeepAwayMem, p.KeepAwayStay = 1500, 20, 30 },
		"v05fixk11": func(p *Params) { bunker(p, 80, 0.5); p.KeepAway, p.KeepAwayMem, p.KeepAwayStay = 800, 20, 20 },
		// k12 = k11 that, once on its way to a supply drop, stays on it (re-enacted, k11 turned between
		// "hold" and the drop four times a match: the "not yet" rule, see DropStick)
		"v05fixk12": func(p *Params) {
			bunker(p, 80, 0.5)
			p.KeepAway, p.KeepAwayMem, p.KeepAwayStay = 800, 20, 20
			p.DropStick = true
		},
		// the published two-rule build with its gate open and closed, for the local checks
		"gg-open2": func(p *Params) {
			pfx(p)
			p.SlotPlan, p.FFAFrom = "hunt+quiet3.0", nowUnix()-60
			p.GateSwap, p.GateLate, p.GateLateT, p.GateDoubt = "v05fixk12", "ffa-fixrf9", 60, gateDoubtS
			p.GateAlt, p.GateLateT2, p.GateLateAlive2 = true, 240, 4
		},
		"gg-c2": func(p *Params) {
			pfx(p)
			p.SlotPlan, p.FFAFrom = "hunt+quiet3.0", 1795363200
			p.GateSwap, p.GateLate, p.GateLateT, p.GateDoubt = "v05fixk12", "ffa-fixrf9", 60, gateDoubtS
			p.GateAlt, p.GateLateT2, p.GateLateAlive2 = true, 240, 4
		},
		// k13 = k12 + out of a laser's sight at once when we hold none (LaserRespect), and a mech that
		// hurt us stays a pair of eyes for 30 s (ThreatMem)
		"v05fixk13": func(p *Params) {
			bunker(p, 80, 0.5)
			p.KeepAway, p.KeepAwayMem, p.KeepAwayStay = 800, 20, 20
			p.DropStick = true
			p.LaserRespect, p.ThreatMem = 60, 30
		},
		"gg-open3": func(p *Params) {
			pfx(p)
			p.SlotPlan, p.FFAFrom = "hunt+quiet3.0", nowUnix()-60
			p.GateSwap, p.GateLate, p.GateLateT, p.GateDoubt = "v05fixk13", "ffa-fixrf9", 60, gateDoubtS
			p.GateAlt, p.GateLateT2, p.GateLateAlive2 = true, 240, 4
		},
		"gg-c3": func(p *Params) {
			pfx(p)
			p.SlotPlan, p.FFAFrom = "hunt+quiet3.0", 1795363200
			p.GateSwap, p.GateLate, p.GateLateT, p.GateDoubt = "v05fixk13", "ffa-fixrf9", 60, gateDoubtS
			p.GateAlt, p.GateLateT2, p.GateLateAlive2 = true, 240, 4
		},
		"x-g0k13":   func(p *Params) { gateTest(p, "v05fixk13", 60) },
		"x-g0k13a4": func(p *Params) { gateTest(p, "v05fixk13", 240); p.FFALateAlive = 4 },
		"x-g0k12":   func(p *Params) { gateTest(p, "v05fixk12", 60) },
		"x-g0k12a4": func(p *Params) { gateTest(p, "v05fixk12", 240); p.FFALateAlive = 4 },
		"v05fixqd6": func(p *Params) { quietGround(p); p.KeepAway, p.KeepAwayMem, p.KeepAwayStay = 800, 20, 20 },
		// the published gated builds, for the local identity checks: gg-c with the gate at the cutoff
		// (closed until then: must play exactly as gp:hunt+quiet3.0), gg-open with a gate that is open
		"gg-c": func(p *Params) {
			pfx(p)
			p.SlotPlan, p.FFAFrom = "hunt+quiet3.0", 1795363200
			p.GateSwap, p.GateLate, p.GateLateT, p.GateDoubt = "v05fixk9", "ffa-fixrf9", 60, gateDoubtS
		},
		"gg-open": func(p *Params) {
			pfx(p)
			p.SlotPlan, p.FFAFrom = "hunt+quiet3.0", nowUnix()-60
			p.GateSwap, p.GateLate, p.GateLateT, p.GateDoubt = "v05fixk9", "ffa-fixrf9", 60, gateDoubtS
		},
		// (the same without the second thoughts, to see what they are worth against the squad)
		"gg-open0": func(p *Params) {
			pfx(p)
			p.SlotPlan, p.FFAFrom = "hunt+quiet3.0", nowUnix()-60
			p.GateSwap, p.GateLate, p.GateLateT = "v05fixk9", "ffa-fixrf9", 60
		},
		// ... staying in the keep-away set until few are left (or a late hour)
		"x-g0k11a6": func(p *Params) { gateTest(p, "v05fixk11", 240); p.FFALateAlive = 6 },
		"x-g0k11a4": func(p *Params) { gateTest(p, "v05fixk11", 240); p.FFALateAlive = 4 },
		"x-g0k11a3": func(p *Params) { gateTest(p, "v05fixk11", 300); p.FFALateAlive = 3 },
		"x-g0k11l90":  func(p *Params) { gateTest(p, "v05fixk11", 90) },
		"x-g0k11l120": func(p *Params) { gateTest(p, "v05fixk11", 120) },
		"x-g0k11l180": func(p *Params) { gateTest(p, "v05fixk11", 180) },
		"x-g0k11l300": func(p *Params) { gateTest(p, "v05fixk11", 300) },
		"x-g0k9l60": func(p *Params) { gateTest(p, "v05fixk9", 60) },
		"x-g0k9l45": func(p *Params) { gateTest(p, "v05fixk9", 45) },
		"x-g0k11":   func(p *Params) { gateTest(p, "v05fixk11", 60) },
		"x-g0d6":    func(p *Params) { gateTest(p, "v05fixqd6", 50) },
		// no gate: the same quiet sets from detection on (what a build without FFAFrom plays)
		"x-hk9":  func(p *Params) { pfx(p); p.SquadSet, p.OpenSet, p.FFASwap, p.FFALateT = "sq-hunt", "o-quiet3", "v05fixk9", 60 },
		"x-hk11": func(p *Params) { pfx(p); p.SquadSet, p.OpenSet, p.FFASwap, p.FFALateT = "sq-hunt", "o-quiet3", "v05fixk11", 60 },
		"x-hd6":  func(p *Params) { pfx(p); p.SquadSet, p.OpenSet, p.FFASwap = "sq-hunt", "o-quiet3", "v05fixqd6" },
		"x-g0k6":    func(p *Params) { gateTest(p, "v05fixk6", 75) },
		"x-g0k7":    func(p *Params) { gateTest(p, "v05fixk7", 75) },
		"x-g0k8":    func(p *Params) { gateTest(p, "v05fixk8", 75) },
		"x-g0k9":    func(p *Params) { gateTest(p, "v05fixk9", 75) },
		"x-g0k10":   func(p *Params) { gateTest(p, "v05fixk10", 75) },
		"x-g0k1": func(p *Params) { gateTest(p, "v05fixk1", 75) },
		"x-g0k2": func(p *Params) { gateTest(p, "v05fixk2", 75) },
		"x-g0k3": func(p *Params) { gateTest(p, "v05fixk3", 75) },
		"x-g0k4": func(p *Params) { gateTest(p, "v05fixk4", 75) },
		"x-g0k5": func(p *Params) { gateTest(p, "v05fixk5", 75) },
		// around the ground-only quiet set (qd), the best of the first round in the re-enactment
		"v05fixqd1": func(p *Params) { quietGround(p); p.ArmRadius = 60 },
		"v05fixqd2": func(p *Params) {
			quietGround(p)
			p.ShySecs, p.ShyRange, p.ShyLaser, p.ShyMem, p.ShyBoost = 25, 220, 60, 6, 40
		},
		"v05fixqd3": func(p *Params) { quietGround(p); p.KeepAway, p.KeepAwayMem, p.KeepAwayStay = 300, 12, 20 },
		"v05fixqd4": func(p *Params) { quietGround(p); p.FFALootRadius = 80 },
		"v05fixqd5": func(p *Params) { quietGround(p); p.EdgeBias, p.EdgeBand = 0, 0 },
		"x-g0d1":    func(p *Params) { gateTest(p, "v05fixqd1", 50) },
		"x-g0d2":    func(p *Params) { gateTest(p, "v05fixqd2", 50) },
		"x-g0d3":    func(p *Params) { gateTest(p, "v05fixqd3", 50) },
		"x-g0d4":    func(p *Params) { gateTest(p, "v05fixqd4", 50) },
		"x-g0d5":    func(p *Params) { gateTest(p, "v05fixqd5", 50) },
		"x-g0dl35":  func(p *Params) { gateTest(p, "v05fixqd", 35) },
		"x-g0dl70":  func(p *Params) { gateTest(p, "v05fixqd", 70) },
		"x-g0b1": func(p *Params) { pfx(p); p.SquadSet = "sq-hunt"; p.OpenSet = "o-quiet3"; p.ForceFFA = true; p.FFASwap = "v05fixb1" },
		"x-g0b2": func(p *Params) { pfx(p); p.SquadSet = "sq-hunt"; p.OpenSet = "o-quiet3"; p.ForceFFA = true; p.FFASwap = "v05fixb2" },
		"x-g0b3": func(p *Params) { pfx(p); p.SquadSet = "sq-hunt"; p.OpenSet = "o-quiet3"; p.ForceFFA = true; p.FFASwap = "v05fixb3" },
		"x-g0b3l": func(p *Params) {
			pfx(p)
			p.SquadSet, p.OpenSet, p.ForceFFA, p.FFASwap, p.FFALateT = "sq-hunt", "o-quiet3", true, "v05fixb3", 75
		},
		"x-g0":  func(p *Params) { pfx(p); p.SquadSet = "sq-hunt"; p.OpenSet = "o-quiet3"; p.ForceFFA = true },
		"x-g0s": func(p *Params) { pfx(p); p.SquadSet = "sq-hunt"; p.OpenSet = "o-quiet3"; p.ForceFFA = true; p.FFASwap = "v05fixq9s" },
		"x-g0d": func(p *Params) { pfx(p); p.SquadSet = "sq-hunt"; p.OpenSet = "o-quiet3"; p.ForceFFA = true; p.FFASwap = "v05fixqd" },
		"x-g0e": func(p *Params) { pfx(p); p.SquadSet = "sq-hunt"; p.OpenSet = "o-quiet3"; p.ForceFFA = true; p.FFASwap = "v05fixqe" },
		"x-g0c": func(p *Params) {
			pfx(p)
			p.SquadSet, p.OpenSet, p.ForceFFA = "sq-hunt", "o-quiet3", true
			p.FFASwap, p.FFALate = "ffa-fixc9", ""
		},
		"gp-hq":    func(p *Params) { pfx(p); p.SlotPlan = "hunt+quiet.0,hunt+quiet.0,hunt+quiet.0,hunt+quiet.0" },
		// the plans of v0.55, v0.58 and of their composite v0.67, for the local identity checks
		"gp-c55": func(p *Params) { pfx(p); p.SlotPlan = "hunt+quiet.0,hunt.0,hunt.0,hunt.0" },
		"gp-c58": func(p *Params) { pfx(p); p.SlotPlan = "hunt.0,hunt.0,hunt.0,hunt+quiet.0" },
		"gp-c67": func(p *Params) { pfx(p); p.SlotPlan = "hunt+quiet.0,hunt.0,hunt.0,hunt+quiet.0" },
		"x-hunt2": func(p *Params) { pfx(p); p.SquadSet = "sq-hunt2" },
		"x-hunt3": func(p *Params) { pfx(p); p.SquadSet = "sq-hunt3" },
		"x-hunt4": func(p *Params) { pfx(p); p.SquadSet = "sq-hunt4" },
		"x-hunt5": func(p *Params) { pfx(p); p.SquadSet = "sq-hunt5" },
		"x-hunt6": func(p *Params) { pfx(p); p.SquadSet = "sq-hunt6" },
		"x-hunt7": func(p *Params) { pfx(p); p.SquadSet = "sq-hunt7" },
		"gp-hid":  func(p *Params) { pfx(p); p.SlotPlan = "hunt.0,hunt.0,hunt.0,hunt.0" },
		"sq-c10":   func(p *Params) { p.WantCubes = 10 },
		"sq-offst": func(p *Params) { p.UpOffense = 0.2; p.DuelDisengage = 0; p.LeaveTTD = 0; p.RetreatHP = 0 },
		// seat plans (see applySlotPlan), for the local checks of that mechanism
		"gp-id":   func(p *Params) { pfx(p); p.SlotPlan = "-.0,-.0,-.0,-.0" },
		"gp-a1":   func(p *Params) { pfx(p); p.SlotPlan = "-.1,-.0,-.0,-.0" },
		"gp-c1":   func(p *Params) { pfx(p); p.SlotPlan = "-.0,-.0,-.1,-.0" },
		"gp-h2":   func(p *Params) { pfx(p); p.SlotPlan = "-.0,house.0,-.0,-.0" },
		"p74f-s1": func(p *Params) { pfx(p); p.Salt = 1 },
		// the free-for-all brawl stance for the whole practice match (the lobby stays "not known yet")
		"sq-open":   func(p *Params) { p.OpenSecs = 1e9 },
		"x-open":    func(p *Params) { pfx(p); p.SquadSet = "sq-open" },
		"sq-drop2":  func(p *Params) { p.DropGreed = 1.5 },
		"x-drop":    func(p *Params) { pfx(p); p.SquadSet = "sq-drop2" },
		"sq-house":  func(p *Params) { p.HouseModel = true },
		"x-house":   func(p *Params) { pfx(p); p.SquadSet = "sq-house" },
		"sq-housek": func(p *Params) { p.HouseModel = true; p.KillFocus = 4; p.UpOffense = 0.2 },
		"x-housek":  func(p *Params) { pfx(p); p.SquadSet = "sq-housek" },
		"sq-kill": func(p *Params) {
			p.KillFocus, p.UpOffense, p.WantCubes = 4, 0.2, 10
			p.DuelDisengage, p.LeaveTTD, p.RetreatHP = 0, 0, 0
		},
		"x-kill":   func(p *Params) { pfx(p); p.SquadSet = "sq-kill" },
		"sq-focus": func(p *Params) { p.KillFocus = 4 },
		"x-focus":  func(p *Params) { pfx(p); p.SquadSet = "sq-focus" },
		"x-off":    func(p *Params) { pfx(p); p.SquadSet = "sq-off" },
		"x-stand":  func(p *Params) { pfx(p); p.SquadSet = "sq-stand" },
		"x-roof":   func(p *Params) { pfx(p); p.SquadSet = "sq-roof" },
		"x-hunt":   func(p *Params) { pfx(p); p.SquadSet = "sq-hunt" },
		"x-c10":    func(p *Params) { pfx(p); p.SquadSet = "sq-c10" },
		"x-heal":   func(p *Params) { pfx(p); p.SquadSet = "sq-heal" },
		// p74e = out of sight after detection: cover when a known mech can see us, loot only what it
		// cannot see, lie low otherwise, no fights of our own, all until 50 s; then the fixed game
		"ffa-shy": func(p *Params) {
			p.QuietSlip, p.QuietSlipMin, p.LaserMechOnly, p.CrateGiveUp, p.NpcGiveUp = true, 40, true, 5, 8
			p.ShySecs, p.ShyRange, p.ShyLaser, p.ShyMem, p.ShyBoost = 50, 220, 60, 6, 40
			p.HuntAfter = 50
			p.HealFirst, p.HealUntil = 0.75, 0.95
		},
		"p74e": func(p *Params) { v074(p); p.FFASwap = "ffa-shy" },
		"p74a": func(p *Params) { v074(p); p.FFASwap = "ffa-fix" },
		"p74b": func(p *Params) { v074(p); p.FFASwap = "ffa-fixrf" },
		// roof first, farm later: the v0.5 opening, then this build with roofs wanted at once
		"ffa-rf":     func(p *Params) { p.BrawlRoofAfter = 0; p.RoofFarmAfter = 60 },
		"ffa-rf45":   func(p *Params) { p.BrawlRoofAfter = 0; p.RoofFarmAfter = 45 },
		"ffa-rf90":   func(p *Params) { p.BrawlRoofAfter = 0; p.RoofFarmAfter = 90 },
		"h3r":        func(p *Params) { p.OpenSet = "v05"; p.FFASwap = "ffa-rf" },
		"h3r45":      func(p *Params) { p.OpenSet = "v05"; p.FFASwap = "ffa-rf45" },
		"h3r90":      func(p *Params) { p.OpenSet = "v05"; p.FFASwap = "ffa-rf90" },
		"h1r":        func(p *Params) { p.FFASwap = "ffa-rf" },
		// the same with the path fix in the opening: v0.5 then walks to a weapon close by before the roof
		"h3r45f":     func(p *Params) { p.OpenSet = "v05fix"; p.FFASwap = "ffa-rf45" },
		"h3":         func(p *Params) { p.OpenSet = "v05" },
		"h3s":        func(p *Params) { p.OpenSet = "v05"; p.FFASwap = "v05" },
		"b-noquiet":  func(p *Params) { p.QuietSlip = false },
		"v075":       func(p *Params) { p.LaserMechOnly = false; p.CrateGiveUp = 0; p.NpcGiveUp = 0 },
		"v076":       func(p *Params) { p.CrateGiveUp = 0; p.NpcGiveUp = 0 },
		// practice-only sets (applied through SquadSet) and the builds that use them
		"sq-off":      func(p *Params) { p.UpOffense = 0.2 },
		"sq-stand":    func(p *Params) { p.DuelDisengage = 0; p.LeaveTTD = 0; p.RetreatHP = 0 },
		"sq-offstand": func(p *Params) { p.UpOffense = 0.2; p.DuelDisengage = 0; p.LeaveTTD = 0; p.RetreatHP = 0 },
		"sq-drop":     func(p *Params) { p.DropGreed = 1.5 },
		// heal before farming: everywhere, and against the practice squad only
		"b-heal":      func(p *Params) { p.HealFirst = 0.75; p.HealUntil = 0.95 },
		"b-heal6":     func(p *Params) { p.HealFirst = 0.6; p.HealUntil = 0.9 },
		"b-heal9":     func(p *Params) { p.HealFirst = 0.9; p.HealUntil = 0.98 },
		"sq-heal":     func(p *Params) { p.HealFirst = 0.75; p.HealUntil = 0.95 },
		"q-heal":      func(p *Params) { p.SquadSet = "sq-heal" },
		"q-off":       func(p *Params) { p.SquadSet = "sq-off" },
		"q-stand":     func(p *Params) { p.SquadSet = "sq-stand" },
		"q-offstand":  func(p *Params) { p.SquadSet = "sq-offstand" },
		"q-drop":      func(p *Params) { p.SquadSet = "sq-drop" },
		"s1":         func(p *Params) { p.Salt = 1 },
		"s2":         func(p *Params) { p.Salt = 2 },
		"s3":         func(p *Params) { p.Salt = 3 },
		"s4":         func(p *Params) { p.Salt = 4 },
		"s5":         func(p *Params) { p.Salt = 5 },
		"s6":         func(p *Params) { p.Salt = 6 },
		"b-shy":      func(p *Params) { p.SlotRule = true; p.ShySecs = 25 },
		"b-slotonly": func(p *Params) { p.SlotRule = true },
		"b-ffa0":     func(p *Params) { p.ForceFFA = true },
		"b-shy40":    func(p *Params) { p.SlotRule = true; p.ShySecs = 40 },
		"b-shyb":     func(p *Params) { p.SlotRule = true; p.ShySecs = 25; p.ShyBoost = 40 },
		"b-shyr90":   func(p *Params) { p.SlotRule = true; p.ShySecs = 25; p.ShyLootRadius = 90 },
		"b-shyall":   func(p *Params) { p.SlotRule = true; p.ShySecs = 25; p.ShyBoost = 40; p.ShyLootRadius = 90; p.ShyExposure = 0.6 },
		"b-nocredit2": func(p *Params) { p.CreditOpenS = 0 },
		// only run for cover that is seconds away: other reaches
		"b-rr35": func(p *Params) { p.RetreatRunMax = 35 },
		"b-rr70": func(p *Params) { p.RetreatRunMax = 70 },
		// cover that lasts (cover.go): the parts on their own, and other settings
		"b-run0":   func(p *Params) { p.RunOnRange = 0 },
		"b-cm0":    func(p *Params) { p.CoverMargin = 0 },
		"b-cm14":   func(p *Params) { p.CoverMargin = 14 },
		"b-run120": func(p *Params) { p.RunOnRange = 120 },
		// the rules read off the real first-contact data: cover unless laser-against-none; hunt only then
		"b-real":    func(p *Params) { p.CoverFirst = true; p.HuntLaserEdge = true },
		"b-realh":   func(p *Params) { p.CoverFirst = true; p.HuntLaserEdge = true; p.HuntRangeLaser = 200 },
		"b-realc":   func(p *Params) { p.CoverFirst = true },
		"b-realx":   func(p *Params) { p.HuntLaserEdge = true },
		"b-realb5":  func(p *Params) { p.CoverFirst = true; p.HuntLaserEdge = true; p.LootHiddenBias = 0.5 },
		"b-realb8":  func(p *Params) { p.CoverFirst = true; p.HuntLaserEdge = true; p.LootHiddenBias = 0.8 },
		"b-cf":      func(p *Params) { p.CoverFirst = true },
		"b-cf60":    func(p *Params) { p.CoverFirst = true; p.CoverMinRange = 60 },
		"b-cf90c":   func(p *Params) { p.CoverFirst = true; p.AvoidCover = 90 },
		"b-cf15":    func(p *Params) { p.CoverFirst = true; p.AvoidAdv = 1.5 },
		"b-cf20":    func(p *Params) { p.CoverFirst = true; p.AvoidAdv = 2.0 },
		"b-cf30":    func(p *Params) { p.CoverFirst = true; p.AvoidAdv = 3.0 },
		"b-cf99":    func(p *Params) { p.CoverFirst = true; p.AvoidAdv = 99 },
		"b-cf20h":   func(p *Params) { p.CoverFirst = true; p.AvoidAdv = 2.0; p.BrawlEngage = 2.0 },
		"b-cf99h":   func(p *Params) { p.CoverFirst = true; p.AvoidAdv = 99; p.BrawlHunt = false },
		"b-cfpeek":  func(p *Params) { p.CoverFirst = true; p.Peek = true },
		"b-peek":    func(p *Params) { p.Peek = true },
		"b-peek14":  func(p *Params) { p.Peek = true; p.PeekReach = 14; p.PeekCover = 40 },
		"b-peekh":   func(p *Params) { p.Peek = true; p.HuntRangeLaser = 200 },
		"b-noslack": func(p *Params) { p.ZoneSlack = 0 },
		"b-oe2":     func(p *Params) { p.OverlandEarly = 2.5 },
		"b-oe4":     func(p *Params) { p.OverlandEarly = 4 },
		"b-rc3":     func(p *Params) { p.RoofCenterW = 0.3 },
		"b-rc6":     func(p *Params) { p.RoofCenterW = 0.6 },
		"b-rlq":     func(p *Params) { p.RoofLeaveQuiet = 25 },
		"b-rc3lq":   func(p *Params) { p.RoofCenterW = 0.3; p.RoofLeaveQuiet = 25 },
		"b-noroof":  func(p *Params) { p.BrawlRoofs = false },
		"b-nopre":   func(p *Params) { p.LaserPreempt = false; p.LaserAimRange = 0; p.AvoidCoverLaser = 0 },
		"b-c15":     func(p *Params) { p.BrawlWantCubes = 15 },
		// brawl opening, then wait on a roof (the published v0.5 was safe on roofs in its real matches)
		"b-roof":    func(p *Params) { p.BrawlRoofs = true },
		"b-roofpre": func(p *Params) { p.BrawlRoofs = true; p.LaserPreempt = true; p.LaserAimRange = 285 },
		"b-roof3":   func(p *Params) { p.BrawlRoofs = true; p.BrawlRoofCubes = 3; p.BrawlRoofAfter = 40 },
		"b-roof10":  func(p *Params) { p.BrawlRoofs = true; p.BrawlRoofCubes = 10; p.BrawlRoofAfter = 100 },
		// zig-zag while walking under fire
		"b-jink5": func(p *Params) { p.Jink = 0.5 },
		"b-jink8": func(p *Params) { p.Jink = 0.8 },
		// candidates for v0.7
		"b-hm":     func(p *Params) { p.HoldHideMap = true },
		"b-las285": func(p *Params) { p.LaserAimRange = 285 },
		"b-hm285":  func(p *Params) { p.HoldHideMap = true; p.LaserAimRange = 285 },
		"b-pre":    func(p *Params) { p.LaserPreempt = true; p.LaserAimRange = 285 },
		"b-pre90":  func(p *Params) { p.LaserPreempt = true; p.LaserAimRange = 285; p.AvoidCoverLaser = 90 },
		"b-cov90l": func(p *Params) { p.AvoidCoverLaser = 90 },
		// ablations of the newest rules
		"b-noprov2":  func(p *Params) { p.ProvokeRule = false },
		"b-noplink2": func(p *Params) { p.PlinkMaxDPS = 0 },
		"b-nosprint": func(p *Params) { p.LootBoost = 0 },
		"b-nocredit": func(p *Params) { p.CreditFix = false },
		// v0.6 exactly as published (with the first-goal path bug), and v0.6.1 (bug fixed)
		"v06":  func(p *Params) { v061(p); p.PathFix = false },
		"v061": func(p *Params) { v061(p) },
		// experiments on top of the v0.6 brawl: sprint to lasers; leave mortars alone
		"b-sprint":   func(p *Params) { brawl2(p); p.LootBoost = 30 },
		"b-sprint10": func(p *Params) { brawl2(p); p.LootBoost = 30; p.LaserValue = 10; p.ArmSwapMin = 0.5 },
		"b-nomortar": func(p *Params) { brawl2(p); p.NoMortar = true },
		// experiments on top of the v0.6 brawl: hold fire until touched; keep farming under light fire
		"b-prov":      func(p *Params) { brawl2(p); p.ProvokeRule = true },
		"b-plink":     func(p *Params) { brawl2(p); p.PlinkMaxDPS = 60 },
		"b-provplink": func(p *Params) { brawl2(p); p.ProvokeRule = true; p.PlinkMaxDPS = 60 },
		"b-pps":       func(p *Params) { brawl2(p); p.ProvokeRule = true; p.PlinkMaxDPS = 60; p.LootBoost = 30 },
		"b-pp40":      func(p *Params) { brawl2(p); p.ProvokeRule = true; p.PlinkMaxDPS = 40 },
		"b-pp90":      func(p *Params) { brawl2(p); p.ProvokeRule = true; p.PlinkMaxDPS = 90 },
	}
	// the hunting sweep: every entry is the v0.24 practice set (sq-hunt) with one more change, as a
	// practice set "sq-<name>" and as a whole build "x-<name>"
	for name, f := range huntSweep {
		name, f := name, f
		m["sq-"+name] = func(p *Params) { p.EngageRatio, p.HuntRange, p.HuntRangeLaser = 0.9, 180, 200; f(p) }
		m["x-"+name] = func(p *Params) { pfx(p); p.SquadSet = "sq-" + name }
	}
	return m
}

var huntSweep = map[string]func(p *Params){
	"hA": func(p *Params) { p.EngageRatio = 0.7 },
	"hB": func(p *Params) { p.EngageRatio = 0.5 },
	"hC": func(p *Params) { p.HuntRange, p.HuntRangeLaser = 230, 260 },
	"hD": func(p *Params) { p.Seek, p.SeekHP = 30, 0.5 },
	"hE": func(p *Params) { p.Seek, p.SeekHP = 30, 0.85 },
	"hF": func(p *Params) { p.LootRadius = 300 },
	"hG": func(p *Params) { p.LootRadius = 140 },
	"hH": func(p *Params) { p.WantCubes = 21 },
	"hI": func(p *Params) { p.HuntHP = 0.4 },
	"hJ": func(p *Params) { p.KillFocus = 4 },
	"hK": func(p *Params) { p.UpOffense = 0.2 },
	"hL": func(p *Params) { p.PatientSecs = 45 },
	"hM": func(p *Params) { p.Seek, p.WantCubes, p.HouseModel = 30, 10, true },
	"hN": func(p *Params) { p.EngageRatio, p.Seek, p.WantCubes = 0.6, 30, 6 },
	// second round, around hN (the best of the first: +0.12): less farming, more seeking
	"hN2": func(p *Params) { p.EngageRatio, p.Seek, p.WantCubes = 0.6, 30, 3 },
	"hN3": func(p *Params) { p.EngageRatio, p.Seek, p.WantCubes = 0.6, 30, 0 },
	"hN4": func(p *Params) { p.Seek, p.WantCubes = 30, 6 },
	"hN5": func(p *Params) { p.EngageRatio, p.Seek, p.WantCubes, p.SeekHP = 0.6, 60, 6, 0.5 },
	"hN6": func(p *Params) { p.EngageRatio, p.Seek, p.WantCubes, p.SeekHP, p.HuntHP = 0.6, 30, 6, 0.5, 0.4 },
	"hN7": func(p *Params) {
		p.EngageRatio, p.Seek, p.WantCubes = 0.6, 30, 6
		p.HuntRange, p.HuntRangeLaser = 230, 260
	},
	"hN8": func(p *Params) { p.EngageRatio, p.Seek, p.WantCubes, p.HouseModel = 0.6, 30, 6, true },
	// supply drops in practice: break off for the full repair (hd: when hurt, within 260 m; hd2: from
	// farther and whatever our health; hd3: setting off earlier; hd4: only when clearly hurt)
	"hd":  func(p *Params) { p.DropRun = 260 },
	"hd2": func(p *Params) { p.DropRun, p.DropRunHP = 420, 1.01 },
	"hd3": func(p *Params) { p.DropRun, p.DropRunLead = 420, 10 },
	"hd4": func(p *Params) { p.DropRun, p.DropRunHP = 420, 0.6 },
	"hO": func(p *Params) { p.RoofInSquad = true },
	"hP": func(p *Params) { p.KiteMin, p.KiteMax = 90, 150 },
}

// v05 restores the parameters of the published v0.5.
func v05(p *Params) {
	p.Brawl = false
	p.HitModel = 0
	p.PlanV2 = false
	p.KiteNeedsLaser = false
	p.LaserValue = 3.0
	p.AvoidAdv = 0
	p.UseHideMap = false
	p.QuietHoldS = 0
	p.PathFix = false
	p.CreditFix = false
	p.LootBoost = 0
	p.ProvokeRule = false
	p.PlinkMaxDPS = 0
	p.ZoneSlack = 0
	p.DropStick = false
	p.RoofLeaveQuiet = 0
	p.RoofStandCheck = false
	p.RunOnRange = 0
	p.CoverMargin = 0
	p.RetreatRunMax = 0
	p.CreditOpenS = 0
	oldLead(p)
}

// pfx is the base of the practice experiments: v0.7.4 until the lobby is known, then (free-for-all) the
// v0.13 play without mortars.
// bunker is the quiet set that stays by the spawn: on the ground, no roaming, a weapon or loot only
// within reach metres, and hideouts weighed at distW points per metre of travel.
func bunker(p *Params, reach, distW float64) {
	v05(p)
	p.PathFix, p.CreditFix, p.NoMortarHard, p.StealthQuiet = true, true, true, true
	p.HostileMem, p.RetreatPest, p.EdgeBias, p.EdgeBand = 30, true, 0, 0
	p.ClimbCheckAll, p.ClimbClearMem, p.ClimbClearRange = true, 4, 170
	p.UseRoofs, p.NoStealthRoam = false, true
	p.ArmRadius, p.FFALootRadius = reach, reach
	p.HideNearW = distW
}

// quietGround is the ground-only quiet set (v05fixqd): no roof climb, no walking about for a weapon.
func quietGround(p *Params) {
	v05(p)
	p.PathFix, p.CreditFix, p.NoMortarHard, p.StealthQuiet = true, true, true, true
	p.HostileMem, p.RetreatPest, p.EdgeBias, p.EdgeBand = 30, true, 3, 0.75
	p.ClimbCheckAll, p.ClimbClearMem, p.ClimbClearRange = true, 4, 170
	p.UseRoofs, p.NoStealthRoam = false, true
}

// gateTest is a build whose gate is open: a free-for-all from the first tick, with swap as the quiet
// set until lateT (for the local tools; a published build gets there through FFAFrom).
func gateTest(p *Params, swap string, lateT float64) {
	pfx(p)
	p.SquadSet, p.OpenSet, p.ForceFFA = "sq-hunt", "o-quiet3", true
	p.FFASwap, p.FFALateT = swap, lateT
}

func pfx(p *Params) {
	v074(p)
	// (from v0.67 on the quiet phase after detection also holds its fire until touched: v05fixq; from
	// v0.96 on a mech that has hurt us stays an attacker for 30 s instead of 4: v05fixq2, ffa-fixrf2.
	// Free-for-all play only: practice never gets there, so practice scores are as before.)
	// From v0.104 on: v05fixq3, ffa-fixrf3 (a hurt mech also shoots the drone that is plinking at it).
	// From v0.119 on: v05fixq7, ffa-fixrf7 (and stays out towards the edge of the next circle, away from
	// the others: EdgeBias 3, EdgeBand 0.75).
	// From v0.123 on: v05fixq9, ffa-fixrf9 (and no climb with a mech seen inside 170 m in the last 4 s,
	// in the quiet phase as well).
	p.FFASwap, p.FFALate, p.FFALateT = "v05fixq9", "ffa-fixrf9", 50
}

// v074 restores the parameters of the published v0.7.4.
func v074(p *Params) {
	p.RetreatRunMax = 0
	p.CreditOpenS = 0
	p.ShySecs = 0
	p.SlotRule = false
	p.QuietSlip = false
	p.LaserMechOnly = false
	p.CrateGiveUp = 0
	p.NpcGiveUp = 0
}

// oldLead is the gun lead of every build up to v0.6.1: instantaneous velocity, damped to 75%.
func oldLead(p *Params) {
	p.LeadDamp = 0.75
	p.LeadPersist = false
	p.LeadCum = false
}

// v061 restores the parameters of the published v0.6.1.
func v061(p *Params) {
	brawl2(p)
	p.LootBoost = 0
	p.ProvokeRule = false
	p.PlinkMaxDPS = 0
	p.CreditFix = false
	p.ZoneSlack = 0
	p.BrawlWantCubes = 15
	p.BrawlRoofs = false
	p.LaserPreempt = false
	p.LaserAimRange = 0
	p.AvoidCoverLaser = 0
	p.RoofLeaveQuiet = 0
	p.DropStick = false
	p.CoverFirst = false
	p.HuntLaserEdge = false
	p.LootHiddenBias = 0
	p.EdgeHold = 0
	p.NoMortar = false
	p.RunOnRange = 0
	p.CoverMargin = 0
	p.RetreatRunMax = 0
	p.CreditOpenS = 0
	oldLead(p)
}

// v07 restores the parameters of the published v0.7.
func v07(p *Params) {
	p.CoverFirst = false
	p.HuntLaserEdge = false
	p.LootHiddenBias = 0
	p.EdgeHold = 0
	p.NoMortar = false
	p.RunOnRange = 0
	p.CoverMargin = 0
	p.RetreatRunMax = 0
	p.CreditOpenS = 0
}

// brawl2 is the armed-farmer parameter set (the defaults since v0.6): the published practice behaviour
// (squad stance after the opening) with the stealth stances replaced.
func brawl2(p *Params) {
	p.Brawl = true
	p.HitModel = 1
	p.PlanV2 = true
	p.KiteNeedsLaser = true
	p.LaserValue = 6
	p.AvoidAdv = 1.25
}

// brawl is the always-fight parameter set built after the first real matches.
func brawl(p *Params) {
	p.PracticeAware = false
	p.HitModel = 1
	p.PlanV2 = true
	p.KiteNeedsLaser = true
	p.LaserValue = 6
}
