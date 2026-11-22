package bot

import (
	"fmt"
	"math"

	"apexbot/internal/proto"
)

// navCost is the extra per-cell path cost: keep inside the zone, and (in stealth) keep out of the open.
func (b *Bot) navCost() func(k int) float32 {
	z := &b.o.Zone
	cx, cy, r := z.Center[0], z.Center[1], z.Radius
	stealth := b.st.stealth
	var exp interface{ At(int) float32 }
	if stealth {
		exp = b.Nav.Exposure()
	}
	return func(k int) float32 {
		x, y := b.Nav.Pos(k)
		c := float32(0)
		if hyp(x-cx, y-cy) > r-3 {
			c += 6
		}
		if stealth {
			if e := exp.At(k); e <= 1 {
				c += e * 2.5
			}
		}
		return c
	}
}

// think is the per-decision policy.
func (b *Bot) think(act *proto.Action) {
	b.pickUpgrade(act)
	b.manageWeapons(act)
	st := b.belief()
	b.st = st
	if b.o.Tick%6 == 0 {
		b.markSeen()
	}
	zi := b.zoneInfo()
	ths := b.assess()
	inc := b.gatherIncoming()
	hpFrac := b.hp / b.maxHP

	// --- who would we shoot
	var tgt *threat
	bestS := -1.0
	engagedDPS := 0.0
	nEngaged := 0
	for i := range ths {
		th := &ths[i]
		if th.engaged {
			engagedDPS += th.dps
			nEngaged++
		}
		if !th.tr.Listed {
			continue
		}
		s := 1 / (1 + th.d/60)
		if th.tr.LOS {
			s *= 3
		}
		if th.engaged {
			s *= 1.6
		}
		if th.tr.MaxHP > 0 {
			kf := 1.0
			if b.P.KillFocus > 0 {
				kf = b.P.KillFocus
			}
			s *= 1 + kf*(1-th.tr.HP/th.tr.MaxHP)
		}
		if th.tr.ID == b.targetID {
			s *= 1.2
		}
		if b.P.RearBonus > 0 && b.isRear(th.tr) {
			s *= 1 + b.P.RearBonus
		}
		if b.P.CookoffBonus > 0 && b.has("laser") >= 0 && th.tr.Mortar && stat(b.C.Weapons["laser"], "cookoff", 0) > 0 {
			s *= 1 + b.P.CookoffBonus
		}
		if b.P.ThreatFocus {
			if th.tr.Light && th.d < stat(b.C.Weapons["lightning"], "range", 45)+35 {
				s *= 1.25
			} else if th.tr.Laser {
				s *= 1.18
			}
			if th.tr.MaxHP > 0 && th.tr.HP > 0 && th.tr.HP/th.tr.MaxHP < 0.35 {
				s *= 1.25
			}
		}
		if s > bestS {
			bestS, tgt = s, th
		}
	}
	if tgt != nil {
		b.targetID = tgt.tr.ID
	} else {
		b.targetID = -1
	}
	watchers := b.seenBy(ths, 230)

	// --- fight or flight
	underFire := b.t-b.lastWeaponDT < 3.0
	danger := nEngaged > 0 || underFire
	overall := 10.0
	if engagedDPS > 1 {
		// can we out-trade everyone who is on us, taking them one at a time
		mine := 1.0
		if tgt != nil {
			mine = b.myDPS(tgt.d)
		}
		need := 0.0
		for i := range ths {
			if ths[i].engaged {
				need += math.Max(ths[i].tr.HP, 20) / math.Max(mine, 1)
			}
		}
		overall = (b.hp / (engagedDPS * (1 - b.armor))) / math.Max(need, 0.1)
		// one opponent: the range-aware plan is the better estimate
		if nEngaged == 1 && tgt != nil && tgt.engaged {
			overall = tgt.adv
		}
	}
	// Running from someone already on top of us just means being shot while not shooting: up close we
	// only leave a fight we are clearly losing, whatever the stance says about far-off trouble.
	if st.stealth && tgt != nil && tgt.engaged {
		f := clamp((tgt.d-60)/60, 0, 1)
		st.disengage = b.P.CloseDisengage + (st.disengage-b.P.CloseDisengage)*f
	}
	visibleFoe := tgt != nil && tgt.engaged && tgt.tr.LOS && tgt.tr.Listed
	knownFoe := tgt != nil && tgt.engaged && tgt.tr.Listed
	if b.retreating {
		healed := hpFrac >= b.P.ReturnHP
		winnable := overall > math.Max(st.disengage*1.25, 0.95)
		switch {
		case healed && (!danger || winnable):
			b.retreating = false
		case !danger && hpFrac >= b.P.ReturnHP-0.1:
			b.retreating = false
		case winnable && hpFrac > 0.3 && nEngaged <= 1 && knownFoe:
			// the odds turned (we healed a bit, or the others left): fight
			b.retreating = false
		case visibleFoe && tgt.d < 45 && b.t-b.retreatT > 2.0 && nEngaged <= 1 && overall > 0.35:
			// cornered by one pursuer we cannot shake: shooting back beats being shot in the back
			b.retreating = false
			b.corneredT = b.t
		case b.P.RetreatRunMax > 0 && st.avoid && visibleFoe && b.t-b.retreatT > 0.5 && !b.coverClose(ths, tgt):
			// In its sights with nothing to get behind for seconds: in the real matches a hurt mech
			// still in its attacker's sight 8 s later lived 34% of the time when it stood and shot
			// back, 24% when it shot back on the run, 10% when it only ran (56% when it got out of
			// sight in time). So no long runs in the open: have it out.
			b.retreating = false
			b.corneredT = b.t
			b.standT = b.t
		}
	} else if danger && b.t-b.corneredT > 4 {
		leave := false
		phantoms := 0
		for i := range ths {
			if ths[i].phantom {
				phantoms++
			}
		}
		// how fast we are actually losing health, and how long we last at that rate
		dmgRate := b.recentDmg / 3.0
		ttd := b.hp / math.Max(dmgRate, 1)
		switch {
		case nEngaged <= 1 && knownFoe:
			// one attacker we know about (in sight, or just stepped out of it): leaving mid-duel
			// hands it free hits, so only go when the duel is clearly lost and it is not on top of us
			leave = overall < b.P.DuelDisengage && tgt.d > 30 && ttd < 25
		case nEngaged >= 2:
			// several on us: what matters is whether it is hurting. A stray round from a second
			// shooter is no reason to walk out of an exchange we are winning.
			leave = (ttd < b.P.LeaveTTD && hpFrac < 0.8) || (hpFrac < b.P.RetreatHP && ttd < 2*b.P.LeaveTTD)
		default:
			// shot by something we have not seen
			leave = underFire && phantoms > 0 && (ttd < 1.5*b.P.LeaveTTD || hpFrac < 0.6)
		}
		if leave && b.P.RetreatRunMax > 0 && st.avoid && visibleFoe && !b.coverClose(ths, tgt) {
			leave = false // (see above: running is only worth it with cover seconds away)
			b.standT = b.t
		}
		if leave {
			b.retreating = true
			b.retreatT = b.t
			b.coverCell = -1
		}
	}
	// never walk away from a kill that is one burst from done
	if tgt != nil && tgt.tr.LOS && tgt.tr.Listed && tgt.tr.HP > 0 && tgt.tr.HP < b.myDPS(tgt.d)*1.2+40 && hpFrac > 0.12 {
		b.retreating = false
	}

	// "Patient" opening against the squad (experimental, off by default): killing hunters is what
	// escalates the hunt, so first build up on crates, drones and drops and only swat a hunter that
	// gets close; start killing once we are strong or the clock says so.
	patient := st.name == "squad" && b.P.PatientSecs > 0 && b.t < b.P.PatientSecs && b.o.Self.Cubes < b.P.PatientCubes
	wantFight := false
	slip := false       // attacked from range without an edge: go behind cover, shooting back
	plink := false      // lightly attacked from range: keep farming and shoot back in between
	unprovoked := false // watched from range by someone who has not touched us: do not start it
	roofDuck := false   // on our roof and attacked without an edge: step back behind the roof's edge
	fightWhy := ""      // label suffix: why the brawl stance chose to stand and fight
	onRoof := b.onRoof() >= 0 && b.o.Self.Grounded
	roofWanted := b.roofWanted(st, zi)
	huntR := b.P.HuntRange
	if b.has("laser") >= 0 && b.P.HuntRangeLaser > huntR {
		huntR = b.P.HuntRangeLaser
	}
	if tgt != nil && !b.retreating {
		switch {
		case patient && nEngaged <= 1 && tgt.d > b.P.PatientRange && hpFrac > 0.55:
			// leave it alone for now (handled as "not fighting" below: we keep farming, guns quiet)
		case tgt.engaged:
			// someone is on us and we are not clearly losing (or we would be retreating): fight back.
			// Slipping away only works against a far-off shooter that we have somewhere to hide from.
			switch {
			case st.stealth:
				if tgt.d < b.P.StealthFightRange || tgt.adv >= st.engageRatio {
					wantFight = true
				}
			case st.avoid:
				// Free-for-all: an exchange at range without an edge is a coin flip that leaves even
				// the winner weak, and being looked at is not being attacked.
				provoked := !b.P.ProvokeRule || b.t-tgt.tr.HurtMeT < 4 || tgt.d < b.P.AvoidRange ||
					(tgt.tr.Laser && tgt.tr.Charging && b.t-tgt.tr.AimAtMeT < 0.4) ||
					// a laser pointed our way is a third of our health about to go: do not wait for it
					(b.P.LaserPreempt && tgt.tr.Laser && tgt.tr.LOS && b.t-tgt.tr.AimAtMeT < 0.6)
				far := tgt.d >= b.P.AvoidRange && tgt.adv < b.P.AvoidAdv
				// heavy fire (a laser, or a gun that is actually landing) needs cover; light fire does not
				heavy := tgt.tr.Laser || b.recentDmg/3.0 > b.P.PlinkMaxDPS || hpFrac < 0.5 || nEngaged >= 2
				// Cover first (real matches, 1,463 first hits on a mech, all teams): those who got out
				// of the attacker's line within a few seconds were alive 20 s later 77% of the time and
				// finished on 10.9 points; those who stayed in view and shot back, 55% and 9.4; against a
				// laser with none of their own, 69% against 24%. Standing to fight matched cover only
				// for a mech with a laser against one without.
				// (the edge that counts: our laser against none, or an estimate so lopsided that the
				// fight is nearly over)
				// (with memory: an estimate hovering at the threshold must not have us turning between
				// the firing line and the cover several times a second, doing neither)
				edgeThr, edgeBig := b.P.AvoidAdv, b.P.CoverEdgeAdv
				if b.P.EdgeHold > 0 {
					switch {
					case b.t-b.edgeOnT < b.P.EdgeHold:
						edgeThr, edgeBig = edgeThr*0.75, edgeBig*0.75 // standing on an edge: stay unless it is clearly gone
					case b.t-b.slipOnT < b.P.EdgeHold:
						edgeThr, edgeBig = edgeThr*1.3, edgeBig*1.3 // going for cover: do not turn back for a marginal one
					}
				}
				edge := (b.laserEdge(tgt.tr) && tgt.adv >= edgeThr) || tgt.adv >= edgeBig
				if edge && provoked {
					b.edgeOnT = b.t
				}
				coverFirst := b.P.CoverFirst && provoked && !edge && tgt.d >= b.P.CoverMinRange
				switch {
				case !provoked:
					unprovoked = true // carry on; the torso keeps an eye on it, guns quiet
				case coverFirst && !(onRoof && roofWanted) && b.slipFromX(tgt, ths, true):
					slip = true
				case onRoof && roofWanted && !b.enemyOnRoof(ths):
					// up here we do what the published bot did in its real matches (9 damage a second
					// on roofs against 32 elsewhere): out of sight behind the edge, shooting back on
					// the way (falls back to fighting from the roof if it has no hidden spot)
					roofDuck = true
				case far && heavy && b.slipFrom(tgt, ths):
					slip = true
				case far && !heavy && b.P.PlinkMaxDPS > 0:
					plink = true // keep farming, shoot back when there is nothing better to shoot
				case far && b.slipFrom(tgt, ths):
					slip = true
				default:
					wantFight = true
					// (why we stand and fight, for the telemetry label)
					switch {
					case tgt.adv >= b.P.AvoidAdv:
						fightWhy = ".edge"
					case tgt.d < b.P.AvoidRange && !(b.P.CoverFirst && tgt.d >= b.P.CoverMinRange):
						fightWhy = ".close"
					default:
						fightWhy = ".nocover"
					}
					if b.t-b.standT < 1.0 {
						fightWhy = ".stand" // we would have run, had there been anything close to run to
					}
				}
			default:
				wantFight = true
			}
		case st.hunt && tgt.adv >= st.engageRatio && tgt.d < huntR && tgt.tr.LOS && hpFrac > 0.6 &&
			b.paceOK(st, hpFrac) && b.huntAmmoOK(st) &&
			// (real matches: the one who fires first comes out ahead only with a laser against none:
			// 11.1 final points against the victim's 9.1; level otherwise, and 2 points behind when it
			// is the victim that has the laser)
			(!b.P.HuntLaserEdge || !st.avoid || b.laserEdge(tgt.tr) || tgt.adv >= b.P.CoverEdgeAdv):
			wantFight = true
			fightWhy = ".hunt"
		case st.name == "duel" && hpFrac > 0.5:
			wantFight = true
		case st.stealth && tgt.tr.LOS && tgt.d < 85 && tgt.tr.HP < 0.25*math.Max(tgt.tr.MaxHP, 1) && tgt.adv > 1.5:
			// a nearly dead mech in reach: finish it for the cubes
			wantFight = true
		}
	}
	// With a laser and cover at hand, fight from behind it (see peek.go) instead of trading in the
	// open, running for cover or just ducking.
	peek := false
	peekLi := b.has("laser")
	if b.P.Peek && (st.avoid || b.P.SquadPeek) && peekLi >= 0 && tgt != nil && !b.retreating && tgt.tr.Listed && !tgt.tr.NPC &&
		(wantFight || slip || roofDuck) && tgt.d > b.P.PeekMinRange && tgt.d < 272 {
		roofID := -1
		if onRoof && roofWanted {
			roofID = b.onRoof()
		}
		if b.peekPlanFor(tgt, ths, roofID) {
			peek = true
			wantFight, slip, roofDuck = false, false, false
		}
	}
	if plink && b.chooseLoot(true, st) == nil {
		// nothing to farm: standing about under fire is pointless, so hide from it or have it out
		plink = false
		if b.slipFrom(tgt, ths) {
			slip = true
		} else {
			wantFight = true
		}
	}
	quiet := len(watchers) == 0 && !danger
	if quiet {
		for i := range ths {
			if b.t-ths[i].tr.LastSeen < 6 && ths[i].d < 180 {
				quiet = false
				break
			}
		}
	}
	// "quiet" must last: a sight line that opens and closes as we take a step would otherwise flip us
	// between going for loot and going back to hide, several times a second, in the open
	if !quiet {
		b.unquietT = b.t
	} else if b.t-b.unquietT < b.P.QuietHoldS {
		quiet = false
	}

	// --- movement
	rq := steerReq{threats: inc}
	if li := b.has("laser"); li >= 0 && b.slots[li].charge > 0 {
		rq.slow = true
	}
	// a laser charging at us: value positions that break its line (and sidestep the beam ray in the open)
	laserThreatNow := false
	for i := range ths {
		tr := ths[i].tr
		if !tr.Laser || !tr.Charging || !tr.LOS {
			continue
		}
		aimMatch := b.t-tr.AimAtMeT < 0.25
		if b.P.LaserDodgeV2 && !aimMatch {
			bear := math.Atan2(b.pos[1]-tr.Pos[1], b.pos[0]-tr.Pos[0])
			off := math.Abs(angDiff(tr.Aim[0], bear))
			aimMatch = b.t-tr.AimAtMeT < 0.55 || off < 0.42
		}
		if aimMatch {
			laserThreatNow = true
			sc := tr.scale(b.sizePerCube)
			ex, ey, ez := tr.Pos[0], tr.Pos[1], tr.Pos[2]+7*sc
			dmg := stat(b.C.Weapons["laser"], "damage", 315) * b.cubeDamageMul(tr.Cubes) * 0.85
			if b.has("mortar") >= 0 {
				dmg *= 1 + stat(b.C.Weapons["laser"], "cookoff", 0)
			}
			aimCos, aimSin := math.Cos(tr.Aim[0]), math.Sin(tr.Aim[0])
			beamW := b.P.LaserBeamW
			beamHalf := b.radius + 3.0
			lasRange := stat(b.C.Weapons["laser"], "range", 280)
			prev := rq.posCost
			rq.posCost = func(x, y float64) float64 {
				c := 0.0
				if prev != nil {
					c = prev(x, y)
				}
				if b.W.Visible(ex, ey, ez, x, y, b.pos[2]+b.height*0.5) || b.W.Visible(ex, ey, ez, x, y, b.pos[2]+b.height*0.9) {
					c += dmg
					if b.P.LaserDodgeV2 && beamW > 0 {
						// perpendicular distance to the charging enemy's aim ray: forces boosted sidestep even in open terrain
						bx, by := ex+aimCos*lasRange, ey+aimSin*lasRange
						perp, _ := segPointDist2D(x, y, ex, ey, bx, by)
						if perp < beamHalf*2.5 {
							c += beamW * (1 - perp/(beamHalf*2.5))
						}
					}
				}
				return c
			}
			rq.lookTicks = 12
			rq.dodgeBoost = true
		}
	}
	// Apex-Go v2: SquadLaserCover — even in squad stance, duck behind very close cover while an enemy laser charges at us
	if b.P.SquadLaserCover && laserThreatNow && !st.avoid && !b.retreating && !peek && !slip && !(onRoof && roofWanted) {
		finishSoon := tgt != nil && tgt.tr.LOS && tgt.tr.HP > 0 && tgt.tr.HP < b.myDPS(tgt.d)*1.1+40
		if !finishSoon {
			if c := b.findCover(ths, 24); c >= 0 {
				b.slipCell = c
				b.slipT = b.t
				b.slipOnT = b.t
				slip = true
				wantFight = false
			}
		}
	}
	// Apex-Go v2: Anti-Lightning forcefield (all carriers, primary OR secondary)
	if b.P.AntiLightningW > 0 {
		haveLight := b.has("lightning") >= 0 && b.energy > 18
		if !haveLight || hpFrac < 0.55 {
			lightBase := stat(b.C.Weapons["lightning"], "range", 45)
			pad := b.P.AntiLightningPad
			if pad <= 0 {
				pad = 10
			}
			for i := range ths {
				tr := ths[i].tr
				if !tr.Light || (!tr.Listed && b.t-tr.LastSeen > 2.5) {
					continue
				}
				safeR := lightBase + pad
				if tr.Cubes >= 6 {
					safeR += 6 // margin for lightning_range upgrade
				}
				if ths[i].d > safeR+22 {
					continue
				}
				lx, ly := tr.Pos[0], tr.Pos[1]
				w := b.P.AntiLightningW
				prev := rq.posCost
				rq.posCost = func(x, y float64) float64 {
					c := 0.0
					if prev != nil {
						c = prev(x, y)
					}
					d := hyp(x-lx, y-ly)
					if d < safeR {
						f := (safeR - d) / safeR
						c += w * f * (1 + (safeR-d)/20)
					}
					return c
				}
				if ths[i].d < safeR+4 && tr.LOS {
					rq.dodgeBoost = true
					if rq.lookTicks < 10 {
						rq.lookTicks = 10
					}
				}
			}
		}
	}
	// Apex-Go v3: RearAvoidW (SandeepMadhavarapu 4f3c7d9) — keep extra attackers out of our 1.5x rear arc
	if b.P.RearAvoidW > 0 && tgt != nil && tgt.tr.LOS && nEngaged >= 2 {
		halfFront := b.mech("front_arc_deg", 180) * 0.5 * math.Pi / 180
		tx, ty := tgt.tr.Pos[0], tgt.tr.Pos[1]
		var extras [][2]float64
		for i := range ths {
			th := &ths[i]
			if th.engaged && th.tr.ID != tgt.tr.ID && th.tr.LOS && th.d < 210 {
				extras = append(extras, [2]float64{th.tr.Pos[0], th.tr.Pos[1]})
			}
		}
		if len(extras) > 0 {
			w := b.P.RearAvoidW
			prev := rq.posCost
			rq.posCost = func(x, y float64) float64 {
				c := 0.0
				if prev != nil {
					c = prev(x, y)
				}
				face := math.Atan2(ty-y, tx-x)
				for _, e := range extras {
					bear := math.Atan2(e[1]-y, e[0]-x)
					if math.Abs(angDiff(bear, face)) > halfFront {
						c += w
					}
				}
				return c
			}
		}
	}
	heading, haveHeading := 0.0, false
	var shootItem *aimTarget
	shootHP := 0.0
	goalOK := true     // the last goal we set can be walked to
	navigating := false // we are walking somewhere (as opposed to strafing or standing)
	var lastGoal [2]float64
	setGoal := func(gx, gy, w float64, boost bool) float64 {
		dx, dy, d, ok := b.navDir(gx, gy)
		goalOK = ok
		navigating = true
		lastGoal = [2]float64{gx, gy}
		rq.wantX, rq.wantY, rq.wantW, rq.boost = dx, dy, w, boost
		if dx != 0 || dy != 0 {
			heading, haveHeading = math.Atan2(dy, dx), true
		}
		return d
	}
	holdFire := false
	peekCharge := false // peek fight: start charging the laser now, out of sight
	direct := false     // the action's movement was written directly (jet climb): skip the ground planner
	// jetting up a wall in someone's sights is how we died in our first real match
	b.holdClimb = st.avoid && (len(watchers) > 0 || underFire)
	if st.avoid && !b.holdClimb {
		// ... nor with anyone in plain sight inside 150 m, whichever way it happens to be looking
		for i := range ths {
			if ths[i].tr.Listed && ths[i].tr.LOS && ths[i].d < b.P.ClimbClearRange {
				b.holdClimb = true
				break
			}
		}
	}
	// ... and climbing down from a roof under someone's eyes is how it died later on: leave a roof
	// that the zone will take only once nobody is looking, or when the edge is nearly here
	b.holdRoofMove = b.P.RoofLeaveQuiet > 0 && st.avoid && onRoof && (len(watchers) > 0 || underFire) && zi.tPass > b.P.RoofLeaveQuiet && !zi.urgent
	roofCoverCell := -1
	wantRoofCover := st.stealth && (b.retreating || wantFight || len(watchers) > 0 || underFire)
	if st.avoid && (roofDuck || b.retreating || len(watchers) > 0 || underFire) {
		wantRoofCover = true
		b.roofCoverHoldT = b.t
	} else if st.avoid && b.t-b.roofCoverHoldT < b.P.RoofCoverHold {
		// stay ducked for a few seconds after the last pair of eyes: stepping back out the moment a
		// watcher turns its head had us walking to and fro along the roof
		wantRoofCover = true
	}
	if onRoof && roofWanted && wantRoofCover && !b.enemyOnRoof(ths) {
		if b.t-b.roofCoverT > 0.4 {
			b.roofCoverC = b.roofCover(b.onRoof(), ths)
			b.roofCoverT = b.t
		}
		roofCoverCell = b.roofCoverC
	} else {
		b.roofCoverC = -1
		b.roofCoverT = -100
	}
	if roofDuck && roofCoverCell < 0 && !b.retreating {
		wantFight = true // nowhere to duck up here: have it out from the roof
	}
	// opening option: get onto a roof before anything else (weapons come later, when it is quiet)
	if b.roofFirstOK && !b.roofFirstChecked && b.P.RoofFirstSecs > 0 && roofWanted {
		// decided once, at the start: is there a roof close enough to be worth it
		b.roofFirstChecked = true
		_, acc, ok := b.chooseRoof()
		if ok {
			gx, gy := b.Nav.Pos(acc.Ground)
			ok = hyp(gx-b.pos[0], gy-b.pos[1]) <= b.P.RoofFirstDist
		}
		b.roofFirstOK = ok
	}
	roofFirst := roofWanted && !onRoof && b.t < b.P.RoofFirstSecs && b.roofFirstOK && b.roofFirstChecked
	// Opening: where we stand ten seconds in decides more than what we hold. In the real matches (mapan
	// opening, 857 mechs alive at 10 s) those out of the nearest mech's sight and 150 m+ from it were
	// dead inside a minute 18% of the time and finished on 10.0 points; those inside 100 m and in its
	// sight, 58% and 5.8; a laser in hand by 15 s was worth 1.2 to 2.2 points within each of those
	// classes: less than either step. Line of sight works both ways, so "seen" is simply "we can see
	// a mech": while that is so, we loot only what lies out of its sight and no nearer to it (the
	// loot chooser is strict in this phase), and failing that we get out of its sight first.
	shy := false
	var shyLoot *loot
	if b.shyPhase(st) && !wantFight && !slip && !peek && !roofDuck && !b.retreating && !onRoof {
		// (judged from where we last saw each of them, not from whether it is in our 120 degree
		// wedge this instant; someone looking at us or plinking from afar counts all the more)
		shy = b.exposedTo(b.P.ShyMem, b.P.ShyRange)
		if shy {
			shyLoot = b.chooseLoot(false, st)
			if shyLoot != nil && dist2(b.pos, shyLoot.pos) <= 60 {
				shy = false // something safe close by: taking it hides us as well
			}
		}
	}
	// a roof approach that other business interrupted for a while starts over
	if b.roof.state == roofApproach && b.t-b.roof.stepT > 2.5 {
		b.roof.state = roofNone
	}

	switch {
	case zi.critical || (zi.urgent && (!wantFight || tgt == nil || tgt.d > 60)) || (zi.urgent && zi.tPass < 5):
		// (a fight at range does not stop us walking: the torso keeps shooting on the way)
		b.mode = "zone"
		b.roof.state = roofNone
		if b.P.ZoneSlack > 0 && b.lootID >= 0 {
			// the loot we were walking to is where the zone is about to be: forget it for a while
			b.banUntil[b.lootID] = b.t + 20
			b.lootID = -1
		}
		setGoal(zi.goal[0], zi.goal[1], 3.0, zi.critical && b.energy > 15)
		rq.zoneFree = true
		if !zi.walkable {
			// the circle is somewhere we cannot walk to: go straight at it and jet over what is in the way
			b.mode = "zone-over"
			b.overland(act, zi.goal[0], zi.goal[1])
			direct = true
		}

	case b.roof.state == roofClimb:
		// half way up a wall is no place to change our mind
		b.mode = "climb"
		b.roofClimbStep(act)
		direct = true

	case peek:
		// laser fight from behind cover: hidden while charging and cooling, out for the shot
		b.mode = "peek"
		cell, charge := b.peekStep(ths, peekLi, hpFrac)
		peekCharge = charge
		gx, gy := b.Nav.Pos(cell)
		if d := setGoal(gx, gy, 2.6, false); d < 1.0 {
			rq.wantW = 0
			rq.hold = true
		}
		rq.fenceRoof = onRoof && roofWanted

	case roofCoverCell >= 0:
		// up here the roof edge is our cover: step to where they cannot see us instead of trading
		b.mode = "roof-cover"
		x, y := b.Nav.Pos(roofCoverCell)
		if d := setGoal(x, y, 2.4, false); d < 1.8 {
			rq.wantW = 0
			rq.hold = true
		}
		rq.fenceRoof = true
		holdFire = tgt == nil || !tgt.engaged || tgt.d > b.P.FFAReturnFire

	case slip:
		// not our fight: get something solid between us, shooting back on the way
		b.mode = "slip"
		// (first to the nearest spot out of their sight, then on from that edge of the shadow to a
		// place that stays hidden when they take a step sideways: cover.go)
		deepSlip := b.P.CoverMargin > 0 && b.deepCell >= 0 && b.t-b.deepT < 8 && b.t-b.lastWeaponDT > 0.45 && b.cellHidden(b.deepCell, ths)
		if !deepSlip {
			b.deepCell = -1
		}
		if deepSlip {
			cx, cy := b.Nav.Pos(b.deepCell)
			if d := setGoal(cx, cy, 2.5, false); d < 1.5 || !goalOK {
				rq.wantW = 0
				rq.hold = true
				b.mode = "ambush"
			}
		} else {
			cx, cy := b.Nav.Pos(b.slipCell)
			if d := setGoal(cx, cy, 2.5, b.energy > 25 && underFire); d < 0.9 || (d < 3.0 && b.hiddenHere(ths)) {
				if b.P.CoverMargin > 0 && b.t-b.deepTryT > 0.6 {
					b.deepTryT = b.t
					if c := b.deeperCover(ths, 45, false); c >= 0 {
						b.deepCell, b.deepT = c, b.t
					}
				}
				if b.deepCell >= 0 {
					cx, cy = b.Nav.Pos(b.deepCell)
					setGoal(cx, cy, 2.5, false)
				} else {
					rq.wantW = 0
					rq.hold = true
					b.mode = "ambush"
				}
			}
		}

	case b.retreating:
		b.mode = "retreat"
		if b.P.RetreatPest && nEngaged == 0 {
			if tr := b.pest(); tr != nil && tr.LOS && dist2(b.pos, tr.Pos) < 120 {
				at := b.targetOf(tr)
				shootItem = &at
				shootHP = tr.HP
			}
		}
		// a leg deeper into cover, once begun, is walked to its end unless it stops being cover
		deeper := (b.P.RunOnRange > 0 || b.P.CoverMargin > 0) && st.avoid // (free-for-all stances only)
		deep := deeper && b.deepCell >= 0 && b.t-b.deepT < 8
		if deep && (b.t-b.lastWeaponDT < 0.45 || !b.cellHidden(b.deepCell, ths)) {
			deep = false // hit on the way, or it can be seen now: nearest cover first
		}
		if !deep {
			b.deepCell = -1
			if b.t-b.coverT > 0.5 {
				b.coverCell = b.findCover(ths, 90)
				b.coverT = b.t
			}
		}
		if deep {
			b.mode = "retreat-on"
			cx, cy := b.Nav.Pos(b.deepCell)
			if d := setGoal(cx, cy, 2.5, b.energy > 35 && b.nearestEye(ths) < 60); d < 1.5 || !goalOK {
				b.deepCell = -1 // there: look again from here
				b.coverT = -100
			}
		} else if b.coverCell >= 0 {
			cx, cy := b.Nav.Pos(b.coverCell)
			d := setGoal(cx, cy, 2.5, b.energy > 20 && underFire)
			// "there" means actually out of their sight, not merely near the hidden cell
			if d < 0.9 || (d < 3.0 && b.hiddenHere(ths)) {
				// The nearest hidden spot is the very edge of the shadow (one step sideways and they see
				// us again), whoever was shooting knows which corner we went behind, and health comes
				// back just as fast on the move: go on to somewhere that lasts (cover.go)
				if deeper && b.t-b.deepTryT > 0.6 {
					b.deepTryT = b.t
					if c := b.deeperCover(ths, 75, true); c >= 0 {
						b.deepCell, b.deepT = c, b.t
					}
				}
				if b.deepCell >= 0 {
					b.mode = "retreat-on"
					cx, cy = b.Nav.Pos(b.deepCell)
					setGoal(cx, cy, 2.5, b.energy > 35 && b.nearestEye(ths) < 60)
				} else {
					rq.wantW = 0
					rq.hold = true
					b.mode = "heal"
				}
			}
		} else if from := b.fleeFrom(tgt, ths); from != nil {
			// nowhere to hide: back away from the closest threat while facing it
			dx, dy := norm2(b.pos[0]-from.tr.Pos[0], b.pos[1]-from.tr.Pos[1])
			gx, gy := b.pos[0]+dx*30, b.pos[1]+dy*30
			setGoal(gx, gy, 2.0, b.energy > 30)
		} else if b.P.CoverMargin > 0 && st.avoid && zi.outNext < -5 && b.t-b.lastNpcDT > 4 && b.t-b.lastWeaponDT < 30 {
			// (not with a drone plinking at us, which stops the health coming back: walk on then; and
			// not for ever, in case something else does)
			// nobody on us and nobody in mind any more: stay behind what we found until the health is
			// back, instead of strolling off towards the middle (outside the next circle we do walk:
			// stopping there only to be hurried on a moment later gets us nowhere)
			rq.wantW = 0
			rq.hold = true
			b.mode = "heal"
		} else {
			setGoal(zi.goal[0], zi.goal[1], 1.5, false)
		}
		if st.stealth && tgt != nil && tgt.d > b.P.FFAReturnFire {
			holdFire = true
		}

	case wantFight:
		b.mode = "fight" + fightWhy
		if dr := b.dropRun(st, hpFrac); dr != nil && !onRoof {
			// practice stance supply drop break-off (full HP + full energy + 3 cubes)
			b.mode = "fight+drop"
			d := setGoal(dr.Pos[0], dr.Pos[1], 2.4, b.energy > 40 && dr.LandsIn < 6)
			if d < 3 {
				rq.wantW = 0
				rq.hold = true
			}
		} else if w := b.weaponRun(32); w != nil && !onRoof {
			// a better weapon lying right here beats another second of shooting
			b.mode = "fight+arm"
			dx, dy, _, _ := b.navDir(w.Pos[0], w.Pos[1])
			rq.wantX, rq.wantY, rq.wantW = dx, dy, 2.4
		} else if c := b.cubeRun(tgt, st, hpFrac); c != nil {
			// cubes from a kill are health, damage and upgrades: collect them while we keep shooting
			b.mode = "fight+cube"
			dx, dy, _, _ := b.navDir(c.Pos[0], c.Pos[1])
			rq.wantX, rq.wantY, rq.wantW = dx, dy, 2.2
		} else if near := b.kiteFrom(ths, nEngaged); near != nil {
			// several on us: open the range. Their gunnery falls off much faster with distance than
			// ours does, and the torso keeps shooting while the legs back away.
			b.mode = "kite"
			b.strafe(&rq, near.tr.Pos, b.P.KiteMin, b.P.KiteMax, 2.0)
			if near.d < 70 && b.energy > 45 {
				rq.boost = true
			}
		} else {
			b.fightMove(&rq, tgt)
		}
		rq.fenceRoof = onRoof && roofWanted

	case shy:
		b.mode = "shy"
		if b.t-b.evadeT > 0.6 {
			b.evadeCell = b.findCover(ths, 110)
			b.evadeT = b.t
		}
		switch {
		case b.evadeCell >= 0:
			cx, cy := b.Nav.Pos(b.evadeCell)
			if d := setGoal(cx, cy, 2.2, b.P.ShyBoost > 0 && b.energy > b.P.ShyBoost); d < 0.9 || (d < 3.0 && b.hiddenHere(ths)) {
				rq.wantW = 0
				rq.hold = true
			}
		case shyLoot != nil:
			// nothing to get behind in reach: the errand out of their sight, far as it is
			setGoal(shyLoot.pos[0], shyLoot.pos[1], 2.0, false)
		default:
			// nothing at all: open the distance to the nearest of them
			var nearest *threat
			for i := range ths {
				if tr := ths[i].tr; b.t-tr.LastSeen <= b.P.ShyMem && (nearest == nil || ths[i].d < nearest.d) {
					nearest = &ths[i]
				}
			}
			if nearest != nil {
				dx, dy := norm2(b.pos[0]-nearest.tr.Pos[0], b.pos[1]-nearest.tr.Pos[1])
				setGoal(b.pos[0]+dx*40, b.pos[1]+dy*40, 2.0, false)
			}
		}
		holdFire = true

	case b.pest() != nil && (!st.stealth || len(watchers) == 0):
		// a drone is plinking at us: it blocks our regen and it is worth two cubes, so kill it
		tr := b.pest()
		b.mode = "pest"
		at := b.targetOf(tr)
		d := dist2(b.pos, tr.Pos)
		if tr.LOS && d < 120 {
			shootItem = &at
			shootHP = tr.HP
			b.strafe(&rq, tr.Pos, 35, 60, 1.0)
		} else {
			setGoal(tr.Pos[0], tr.Pos[1], 1.5, false)
		}

	case st.stealth && nEngaged == 0 && !onRoof && !roofFirst && b.weaponRun(b.P.ArmRadius) != nil:
		// get armed before anything else: go for the weapon even if someone can see us
		w := b.weaponRun(b.P.ArmRadius)
		b.mode = "arm"
		setGoal(w.Pos[0], w.Pos[1], 2.2, false)
		if !goalOK {
			b.banUntil[w.ID] = b.t + 40
		}
		holdFire = true

	case st.stealth && len(watchers) > 0 && !(onRoof && roofWanted) && b.roof.state != roofApproach:
		// someone can see us: get out of their sight without starting anything
		b.mode = "evade"
		if b.t-b.evadeT > 0.6 {
			b.evadeCell = b.findCover(ths, 110)
			b.evadeT = b.t
		}
		if b.evadeCell >= 0 {
			cx, cy := b.Nav.Pos(b.evadeCell)
			if d := setGoal(cx, cy, 2.2, false); d < 0.9 || (d < 3.0 && b.hiddenHere(ths)) {
				rq.wantW = 0
				rq.hold = true
			}
		} else {
			w := watchers[0]
			dx, dy := norm2(b.pos[0]-w.tr.Pos[0], b.pos[1]-w.tr.Pos[1])
			setGoal(b.pos[0]+dx*40, b.pos[1]+dy*40, 1.8, false)
		}
		// shoot back only at someone who is actually on us and close enough to hit
		holdFire = tgt == nil || !tgt.engaged || tgt.d > b.P.FFAReturnFire

	default:
		if b.P.RegenHoldHP > 0 && !danger && hpFrac < b.P.RegenHoldHP && b.t-b.lastWeaponDT < 16 && b.t-b.lastNpcDT > 3 && zi.outNext < -5 {
			// just survived a fight at low HP: hold briefly to collect 45+ HP/s regen before starting the next fight
			b.mode = "regen-hold"
			rq.wantW = 0
			rq.hold = true
			break
		}
		if patient {
			holdFire = true // farming while a far-off hunter plinks at us: do not kill it yet
		}
		var l *loot
		if (!st.stealth || quiet) && !roofFirst {
			l = b.chooseLoot(danger && !patient, st)
		}
		if l != nil && onRoof && roofWanted && l.kind != "drop" && !(l.kind == "pickup" && b.has("laser") < 0 && dist2(b.pos, l.pos) < 90) {
			// hidden on a roof: only a weapon we badly want is worth coming down for
			l = nil
		}
		if l != nil {
			b.mode = "loot:" + l.kind
			b.lootID = l.id
			d := dist2(b.pos, l.pos)
			switch l.kind {
			case "crate":
				it := b.crates[l.id]
				at := aimTarget{pos: l.pos, radius: stat(b.C.Crates, "radius", 1.4), centerZ: stat(b.C.Crates, "height", 2.4) * 0.5}
				vis := d < 70 && b.W.Visible(b.pos[0], b.pos[1], b.pos[2]+b.muzzleH, at.pos[0], at.pos[1], at.pos[2]+at.centerZ)
				if vis {
					shootItem = &at
					if it != nil {
						shootHP = it.HP
					}
				}
				if d > 9 || !vis {
					setGoal(l.pos[0], l.pos[1], 1.5, false)
				} else {
					rq.hold = true
				}
			case "npc":
				tr := b.tracks[l.id]
				if tr != nil {
					at := b.targetOf(tr)
					if tr.LOS && d < 110 {
						shootItem = &at
						shootHP = tr.HP
					}
					// fight drones from outside their favourite range, strafing a little
					if d > 60 || !tr.LOS {
						setGoal(l.pos[0], l.pos[1], 1.5, false)
					} else {
						b.strafe(&rq, tr.Pos, 50, 70, 1.0)
					}
				}
			case "drop":
				if d > 3 {
					setGoal(l.pos[0], l.pos[1], 2.0, false)
				} else {
					rq.hold = true
				}
			default:
				// a laser is the one thing worth sprinting for: most opponents only take one that
				// lies within 60 m of their spawn, and there is one within 120 m of 60% of spawns
				sprint := false
				if l.kind == "pickup" && b.P.LootBoost > 0 && b.energy > b.P.LootBoost && d > 25 {
					if it := b.pickups[l.id]; it != nil && it.Kind == "laser" && b.has("laser") < 0 {
						sprint = true
					}
				}
				setGoal(l.pos[0], l.pos[1], 2.0, sprint)
			}
			if navigating && !goalOK {
				// cannot get there from here: stop wanting it for a while
				b.banUntil[l.id] = b.t + 40
			}
		} else {
			b.lootID = -1
			b.mode = "hold"
			if b.shyPhase(st) && !onRoof && b.mechSeenWithin(b.P.ShyMem) {
				// opening, out of their sight, and nothing safe to fetch: stay out of it for now
				b.mode = "lie-low"
				rq.wantW = 0
				rq.hold = true
			} else if zi.final {
				// last circle: stand on the centre (FinalHold shrinks tolerance as radius collapses to 0)
				tol := 2.0
				boostFinal := false
				if b.P.FinalHold {
					tol = clamp(b.o.Zone.Radius*0.06, 0.35, 2.0)
					boostFinal = b.o.Zone.Radius < 18 && b.energy > 15
				}
				if d := setGoal(b.o.Zone.NextCenter[0], b.o.Zone.NextCenter[1], 1.8, boostFinal); d < tol {
					rq.wantW = 0
					rq.hold = true
				}
			} else if sx, sy, sid, ok := b.seekTarget(st, hpFrac); ok && !roofFirst {
				b.mode = "seek"
				setGoal(sx, sy, 1.8, false)
				if !goalOK && sid >= 0 {
					if b.seekBan == nil {
						b.seekBan = map[int]float64{}
					}
					b.seekBan[sid] = b.t + 30
				}
			} else if rx, ry, ok := b.roamTarget(st, hpFrac); ok && !roofFirst && (!roofWanted || (!b.armed() && quiet)) {
				// nothing known to pick up: go and look for crates and weapons
				b.mode = "roam"
				setGoal(rx, ry, 1.5, false)
				if !goalOK && b.roamCell >= 0 {
					b.banUntil[-(b.roamCell + 1)] = b.t + 120
					b.roamCell = -1
				}
			} else if ho, ok := b.hideoutFor(st, zi, roofWanted); ok && ho.class != hmRoof {
				// a ground hideout picked from the visibility map
				b.mode = "hide"
				b.roof.state, b.roof.want, b.roof.spot = roofNone, -1, -1
				gx, gy := b.Nav.Pos(ho.cell)
				if d := setGoal(gx, gy, 1.5, false); d < 2.5 {
					rq.wantW = 0
					rq.hold = true
				}
			} else if gx, gy, settled, ok2 := b.roofPlanStep(roofWanted, act); ok2 {
				// a roof: either the one the visibility map picked (ok above) or by the older rule
				b.mode = "roof"
				if settled {
					rq.hold = true
				} else {
					setGoal(gx, gy, 1.6, false)
				}
				// stay on the roof only once we are on the one we want
				rq.fenceRoof = onRoof && b.roof.state == roofOn
			} else {
				if b.holdCell < 0 || b.t-b.hideT > 2.0 {
					b.holdCell = b.chooseHide(zi)
					b.hideT = b.t
				}
				gx, gy := zi.goal[0], zi.goal[1]
				if b.holdCell >= 0 {
					gx, gy = b.Nav.Pos(b.holdCell)
				}
				if d := setGoal(gx, gy, 1.4, false); d < 2.5 {
					rq.wantW = 0
					rq.hold = true
				}
			}
		}
	}
	// Off the walk grid (a roof we are leaving, a walled courtyard, a ledge) or heading for a place
	// the grid cannot reach: the ground planner is blind there, so go straight and jet over walls.
	if !direct && navigating && !rq.hold && !rq.fenceRoof && (!b.onGroundLevel() || !goalOK) {
		gd := hyp(lastGoal[0]-b.pos[0], lastGoal[1]-b.pos[1])
		if gd > 3 {
			b.overland(act, lastGoal[0], lastGoal[1])
			direct = true
		}
	}
	// Walking somewhere under gun fire from range: zig-zag. Every opponent leads with our velocity of
	// the moment, and in the real matches a mech walking a straight line away was hit half the time
	// at 120-180 m against under a third when it kept changing direction.
	if b.P.Jink > 0 && !direct && navigating && !rq.hold && !rq.fenceRoof && underFire && (rq.wantX != 0 || rq.wantY != 0) {
		far := false
		for i := range ths {
			if ths[i].engaged && !ths[i].phantom && ths[i].d > 55 && b.t-ths[i].tr.HurtMeT < 3 {
				far = true
			}
		}
		gd := hyp(lastGoal[0]-b.pos[0], lastGoal[1]-b.pos[1])
		if far && gd > 12 {
			if b.t >= b.jinkT {
				if b.rng.Float64() < 0.85 {
					b.jinkSign = -b.jinkSign
				}
				b.jinkT = b.t + 0.3 + b.rng.Float64()*0.4
			}
			px, py := -rq.wantY, rq.wantX
			rq.wantX, rq.wantY = norm2(rq.wantX+px*b.P.Jink*b.jinkSign, rq.wantY+py*b.P.Jink*b.jinkSign)
		}
	}
	if !direct {
		b.steer(act, rq)
		b.unstick(act, navigating && !rq.hold && !rq.fenceRoof)
	}

	// --- aim and triggers
	// someone we cannot see is hitting us: unless we are busy with a visible attacker, look that way
	// first (seeing it lets us dodge its shots and decide what to do about it)
	unseen := false
	for i := range ths {
		if ths[i].phantom && b.t-ths[i].tr.HurtMeT < 1.5 {
			unseen = true
		}
	}
	switch {
	case unseen && (tgt == nil || !tgt.engaged || !tgt.tr.LOS):
		act.Aim = [2]float64{b.lastDmgBear, 0}
		act.HasAim = true
		if li := b.has("laser"); li >= 0 && b.slots[li].charge > 0 {
			act.Fire[li] = true
		}
	case plink && shootItem != nil:
		// farming under light fire from range: the crate (a cube, maybe a weapon) comes first
		b.engage(act, *shootItem, true, true, shootHP, true)
	case tgt != nil && !holdFire && !unprovoked && (wantFight || tgt.engaged || b.retreating || peek):
		at := b.targetOf(tgt.tr)
		b.engage(act, at, tgt.tr.LOS, b.t-tgt.tr.LOST < 0.8, tgt.tr.HP, true)
		if peekCharge && peekLi >= 0 {
			act.Fire[peekLi] = true // charging needs no line of sight
		}
	case shootItem != nil:
		b.engage(act, *shootItem, true, true, shootHP, true)
	case tgt != nil && (tgt.tr.LOS || b.t-tgt.tr.LastSeen < 3):
		// watch it without provoking
		at := b.targetOf(tgt.tr)
		b.engage(act, at, tgt.tr.LOS, false, tgt.tr.HP, false)
		// ohio-ironclad: an idle mech in view is the next fight; go into it with a full magazine
		if b.P.HuntAmmo > 0 && st.name == "squad" && !act.Reload && b.t-tgt.tr.HurtMeT > 3 {
			if gi := b.has("gun"); gi >= 0 && !act.Fire[gi] {
				sl := &b.slots[gi]
				if sl.reload <= 0 && sl.ammo < stat(sl.stats, "mag", 12)*b.P.HuntAmmo {
					act.Reload = true
				}
			}
		}
	default:
		b.idleAim(act, heading, haveHeading)
		// top up the magazine while nothing is happening
		if gi := b.has("gun"); gi >= 0 {
			sl := &b.slots[gi]
			if sl.reload <= 0 && sl.ammo < stat(sl.stats, "mag", 12)*0.67 {
				act.Reload = true
			}
		}
		// a laser still charging with nobody listed: keep it for a moment in case the target steps
		// back out, rather than letting go into the scenery straight away
		if li := b.has("laser"); li >= 0 && b.slots[li].charge > 0 {
			for _, tr := range b.tracks {
				if !tr.NPC && b.t-tr.LastSeen < 1.5 {
					act.Fire[li] = true
					break
				}
			}
		}
	}
	// Apex-Go v3: CoverReload (MarwanMoh11 v5.15) — top off gun mag behind cover while no enemy has LOS >= 1.1s
	if b.P.CoverReload && !act.Reload && shootItem == nil {
		if gi := b.has("gun"); gi >= 0 && !act.Fire[gi] {
			sl := &b.slots[gi]
			mag := stat(sl.stats, "mag", 12)
			frac := 0.60
			if b.ffa {
				frac = 0.25
			}
			if sl.reload <= 0 && sl.ammo <= mag*frac && sl.ammo < mag && b.t-b.lastEnemyLOST >= 1.1 {
				act.Reload = true
			}
		}
	}
	b.modeLabel = st.name + ":" + b.mode
	// The debug label doubles as telemetry (it is recorded per decision in a file only our team can
	// get): stance:mode, health, mechs alive, attackers on us, our kills, cubes, weapons, target range.
	wl := ""
	for i := 0; i < 2; i++ {
		if b.slots[i].ok {
			wl += b.slots[i].kind[:2]
		}
	}
	td := -1.0
	if tgt != nil {
		td = tgt.d
	}
	act.Label = fmt.Sprintf("%s h%.0f a%d e%d k%d c%d %s d%.0f", b.modeLabel, b.hp, b.o.Alive, nEngaged, b.kills, b.o.Self.Cubes, wl, td)
}

// weaponRun returns a weapon on the ground that is worth going for right now even with enemies about:
// a second weapon (above all a laser) decides the early duels.
func (b *Bot) weaponRun(maxD float64) *Item {
	var best *Item
	bv := 0.0
	for _, it := range b.pickups {
		if !it.Ground || !b.sameComp(it.Cell) {
			continue
		}
		if until, banned := b.banUntil[it.ID]; banned && b.t < until {
			continue
		}
		ok, rep := b.wantPickup(it.Kind)
		if !ok {
			continue
		}
		v := b.weaponValue(it.Kind)
		if rep >= 0 {
			v -= b.weaponValue(b.slots[rep].kind)
			if v < b.P.ArmSwapMin {
				continue // only a clear upgrade is worth a detour under watch
			}
		}
		d := dist2(b.pos, it.Pos)
		if d > maxD {
			continue
		}
		if !b.inFutureZone(it.Pos[0], it.Pos[1], d/b.walk+2, 5) {
			continue
		}
		// not if somebody else is closer to it than we are
		contested := false
		for _, tr := range b.tracks {
			if tr.NPC || b.t-tr.LastSeen > 5 {
				continue
			}
			if de := dist2(tr.Pos, it.Pos); de < 30 || de < d*0.8 {
				contested = true
				break
			}
		}
		if contested {
			continue
		}
		if s := v / (d + 15); s > bv {
			best, bv = it, s
		}
	}
	return best
}

// slipFrom decides whether to decline an exchange at range: true when the attacker is beyond AvoidRange,
// we have no clear edge over it, and there is cover close by (left in b.slipCell).
func (b *Bot) slipFrom(tgt *threat, ths []threat) bool {
	return b.slipFromX(tgt, ths, false)
}

// laserEdge reports whether we hold a laser and this mech does not: the one situation in which, in
// the real matches, standing to fight (or starting the fight) paid.
func (b *Bot) laserEdge(tr *Track) bool {
	return b.has("laser") >= 0 && !tr.Laser
}

// slipFromX is slipFrom; with anyEdge it goes for cover whatever our estimated advantage (the caller
// has already decided this is not a fight to stand in).
func (b *Bot) slipFromX(tgt *threat, ths []threat, anyEdge bool) bool {
	minR, maxAdv := b.P.AvoidRange, b.P.AvoidAdv
	if b.P.CoverFirst && b.P.CoverMinRange < minR {
		minR = b.P.CoverMinRange
	}
	if b.t-b.slipOnT < 1.0 {
		// already slipping: do not flip back and forth on the threshold
		minR, maxAdv = minR*0.85, maxAdv*1.15
	}
	if anyEdge {
		maxAdv = 1e9
	}
	if b.P.AvoidAdv <= 0 || tgt.d < minR || tgt.adv >= maxAdv {
		return false
	}
	// a target one burst from dead is worth the exposure
	if tgt.tr.LOS && tgt.tr.HP > 0 && tgt.tr.HP < b.myDPS(tgt.d)*1.5+60 {
		return false
	}
	if b.t-b.slipT > 0.5 {
		reach := b.P.AvoidCover
		if tgt.tr.Laser && b.P.AvoidCoverLaser > reach {
			reach = b.P.AvoidCoverLaser // against a laser, cover is worth a longer run
		}
		c := b.findCover(ths, reach)
		if c >= 0 || b.P.EdgeHold <= 0 || b.t-b.slipOnT > 1.5 {
			// (a search that comes up empty for a moment does not cancel a run to cover already begun)
			b.slipCell = c
		}
		b.slipT = b.t
	}
	if b.slipCell < 0 {
		return false
	}
	b.slipOnT = b.t
	return true
}

// kiteFrom returns the nearest attacker to back away from when two or more are on us (nil: do not kite).
func (b *Bot) kiteFrom(ths []threat, nEngaged int) *threat {
	if nEngaged < 2 || b.P.KiteMin <= 0 {
		return nil
	}
	// opening the range only pays with a weapon that still hits out there
	if b.P.KiteNeedsLaser && b.realModel() && b.has("laser") < 0 {
		return nil
	}
	// lightning is a short-range weapon: with it and energy, stay in and brawl one at a time
	if b.has("lightning") >= 0 && b.energy > 40 && b.has("laser") < 0 {
		return nil
	}
	var near *threat
	for i := range ths {
		if ths[i].engaged && !ths[i].phantom && ths[i].tr.Listed && (near == nil || ths[i].d < near.d) {
			near = &ths[i]
		}
	}
	if near == nil || near.d > b.P.KiteMax+20 {
		return nil
	}
	return near
}

// armed reports whether we hold anything besides the starting gun.
func (b *Bot) armed() bool {
	n := 0
	for i := 0; i < 2; i++ {
		if b.slots[i].ok {
			n++
		}
	}
	return n >= 2
}

// fleeFrom is the threat to move away from: our target, else whoever is hurting us (seen or not).
func (b *Bot) fleeFrom(tgt *threat, ths []threat) *threat {
	if tgt != nil {
		return tgt
	}
	var best *threat
	for i := range ths {
		if ths[i].engaged && (best == nil || ths[i].tr.HurtMeT > best.tr.HurtMeT) {
			best = &ths[i]
		}
	}
	return best
}

// cubeRun picks a cube (or pile of cubes) worth walking to in the middle of a fight.
func (b *Bot) cubeRun(tgt *threat, st stance, hpFrac float64) *Item {
	if b.o.Self.Cubes >= st.wantCubes+6 {
		return nil
	}
	var best *Item
	bv := 0.0
	for _, c := range b.cubes {
		if !c.Ground || b.t-c.LastSeen > 20 || !b.sameComp(c.Cell) {
			continue
		}
		d := dist2(b.pos, c.Pos)
		n := 0
		for _, o := range b.cubes {
			if dist2(o.Pos, c.Pos) < 7 {
				n++
			}
		}
		maxD := 40.0
		if n >= 2 {
			maxD = 110
		}
		if st.stealth {
			maxD *= 0.5
		}
		if d > maxD || (d > 30 && hpFrac < 0.3) {
			continue
		}
		// do not walk into any lightning carrier's arc for it
		if b.has("lightning") < 0 {
			if b.P.AntiLightningW > 0 {
				lightR := stat(b.C.Weapons["lightning"], "range", 45) + 8
				dangerLight := false
				for _, tr := range b.tracks {
					if !tr.NPC && tr.Light && b.t-tr.LastSeen < 4 && dist2(c.Pos, tr.Pos) < lightR {
						dangerLight = true
						break
					}
				}
				if dangerLight {
					continue
				}
			} else if tgt != nil && tgt.tr.Light && dist2(c.Pos, tgt.tr.Pos) < 50 {
				continue
			}
		}
		if !b.inFutureZone(c.Pos[0], c.Pos[1], d/b.walk+1, 3) {
			continue
		}
		v := float64(n) / (d + 12)
		if v > bv {
			best, bv = c, v
		}
	}
	return best
}

// roamTarget returns where to go exploring, when exploring is what this stance does.
func (b *Bot) roamTarget(st stance, hpFrac float64) (float64, float64, bool) {
	if hpFrac < 0.7 {
		return 0, 0, false
	}
	if st.stealth {
		// hiding with only the starting gun loses the endgame: look for a weapon while it is quiet
		if b.armed() || b.t < 12 {
			return 0, 0, false
		}
		return b.roamGoal()
	}
	if b.o.Self.Cubes >= st.wantCubes {
		return 0, 0, false
	}
	return b.roamGoal()
}

// pest returns the drone that is shooting at us (the closest one that hurt us in the last few seconds).
func (b *Bot) pest() *Track {
	if b.t-b.lastNpcDT > 4 {
		return nil
	}
	var best *Track
	bd := 1e18
	for _, tr := range b.tracks {
		if !tr.NPC || b.t-tr.HurtMeT > 4 || b.t-tr.LastSeen > 3 {
			continue
		}
		if d := dist2(b.pos, tr.Pos); d < bd {
			best, bd = tr, d
		}
	}
	return best
}

// strafe circles a point while keeping the distance inside [rmin, rmax].
func (b *Bot) strafe(rq *steerReq, at [3]float64, rmin, rmax, w float64) {
	if b.t >= b.strafeT {
		b.strafeSign = -b.strafeSign
		if b.rng.Float64() < 0.25 {
			b.strafeSign = -b.strafeSign
		}
		b.strafeT = b.t + b.P.StrafeMin + b.rng.Float64()*(b.P.StrafeMax-b.P.StrafeMin)
	}
	rx, ry := norm2(at[0]-b.pos[0], at[1]-b.pos[1])
	d := dist2(b.pos, at)
	radial := 0.0
	if d > rmax {
		radial = clamp((d-rmax)/15, 0, 1)
	} else if d < rmin {
		radial = -clamp((rmin-d)/15, 0, 1)
	}
	tx, ty := -ry*b.strafeSign, rx*b.strafeSign
	wx := tx*(1-0.6*math.Abs(radial)) + rx*radial
	wy := ty*(1-0.6*math.Abs(radial)) + ry*radial
	rq.wantX, rq.wantY = norm2(wx, wy)
	rq.wantW = w
}

// fightMove picks the range we want against a target and strafes there.
func (b *Bot) fightMove(rq *steerReq, th *threat) {
	tr := th.tr
	haveLight := b.has("lightning") >= 0 && b.energy > 18
	rmin, rmax := th.rmin, th.rmax
	if rmax <= 0 {
		rmin, rmax = 30, 55
	}
	if !tr.LOS {
		// go find it (carefully): walk toward where it is
		dx, dy, _, _ := b.navDir(tr.Pos[0], tr.Pos[1])
		rq.wantX, rq.wantY, rq.wantW = dx, dy, 1.2
		return
	}
	b.strafe(rq, tr.Pos, rmin, rmax, 1.6)
	if haveLight && th.d > rmax+10 && b.energy > 55 {
		rq.boost = true
	}
	if b.P.LightKiteBoost && !haveLight && tr.Light && b.energy > 28 {
		lr := stat(b.C.Weapons["lightning"], "range", 45)
		if th.d < lr+12 {
			rq.boost = true
		}
	}
}

// dropRun (DropRun > 0, practice squad stance): break off for a supply drop in reach for full HP + 3 cubes.
func (b *Bot) dropRun(st stance, hpFrac float64) *proto.DropObs {
	if b.P.DropRun <= 0 || st.name != "squad" {
		return nil
	}
	if hpFrac > 0.85 && b.o.Self.Cubes >= 10 {
		return nil
	}
	var best *proto.DropObs
	bd := 1e18
	for i := range b.o.Drops {
		dr := &b.o.Drops[i]
		if until, banned := b.banUntil[dr.ID]; banned && b.t < until {
			continue
		}
		d := dist2(b.pos, dr.Pos)
		if d > b.P.DropRun {
			continue
		}
		if dr.LandsIn > d/b.walk*1.25+4 {
			continue
		}
		if !b.inFutureZone(dr.Pos[0], dr.Pos[1], d/b.walk+6, 4) {
			continue
		}
		if d < bd {
			best, bd = dr, d
		}
	}
	return best
}

// seekTarget (Seek > 0, practice squad stance): actively pursue the nearest favourable mech when healthy.
func (b *Bot) seekTarget(st stance, hpFrac float64) (x, y float64, id int, ok bool) {
	seekHP := b.P.SeekHP
	if seekHP <= 0 {
		seekHP = 0.65
	}
	seekR := b.P.SeekRange
	if seekR <= 0 {
		seekR = 320
	}
	if b.P.Seek <= 0 || st.name != "squad" || !st.hunt || hpFrac < seekHP {
		return 0, 0, -1, false
	}
	if !b.paceOK(st, hpFrac) {
		return 0, 0, -1, false
	}
	if b.seekBan == nil {
		b.seekBan = map[int]float64{}
	}
	var best *Track
	bd := 1e18
	for _, tr := range b.tracks {
		if tr.NPC || b.t-tr.LastSeen > b.P.Seek {
			continue
		}
		if until, ok := b.seekBan[tr.ID]; ok && b.t < until {
			continue
		}
		d := dist2(b.pos, tr.Pos)
		if d > seekR || !b.inFutureZone(tr.Pos[0], tr.Pos[1], 8, 6) {
			continue
		}
		if d < 22 && !tr.Listed {
			b.seekBan[tr.ID] = b.t + 20
			continue
		}
		if adv, _, _ := b.fightPlan(tr, d); adv < st.engageRatio {
			continue
		}
		if b.P.SeekHurtW > 0 && tr.MaxHP > 0 {
			d -= b.P.SeekHurtW * 100 * clamp(1-tr.HP/tr.MaxHP, 0, 1)
		}
		if d < bd || (d == bd && best != nil && tr.ID < best.ID) {
			best, bd = tr, d
		}
	}
	if best != nil {
		return best.Pos[0], best.Pos[1], best.ID, true
	}
	x, y, ok = b.roamGoal()
	return x, y, -1, ok
}

// ohio-ironclad: paceOK says whether we may open a new fight of our own (hunt or seek) now. Against the
// practice squad every kill brings another hunter, and health comes back by itself once nothing hits us
// for a few seconds, so right after a kill we only go looking for the next one at near-full health.
func (b *Bot) paceOK(st stance, hpFrac float64) bool {
	if b.P.PaceSecs <= 0 || st.name != "squad" || b.kills == 0 {
		return true
	}
	return b.t-b.lastKillT > b.P.PaceSecs || hpFrac >= b.P.PaceHP
}

// ohio-ironclad: huntAmmoOK says whether the gun has enough rounds to open a hunt (a 2.4 s reload in
// the first exchange hands the other mech free shots).
func (b *Bot) huntAmmoOK(st stance) bool {
	if b.P.HuntAmmo <= 0 || st.name != "squad" {
		return true
	}
	gi := b.has("gun")
	if gi < 0 || b.has("laser") >= 0 || b.has("lightning") >= 0 {
		return true // the second weapon opens the fight
	}
	sl := &b.slots[gi]
	return sl.reload <= 0 && sl.ammo >= stat(sl.stats, "mag", 12)*b.P.HuntAmmo
}
