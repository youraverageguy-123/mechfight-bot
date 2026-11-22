package bot

import (
	"math"
	"sort"
)

// stance is how we behave given what kind of lobby this looks like.
type stance struct {
	name        string
	engageRatio float64 // start fights above this advantage
	disengage   float64 // evade below this
	hunt        bool    // chase favourable targets that are not on us
	stealth     bool    // hide, stay out of sight, loot only when quiet
	avoid       bool    // decline even exchanges at range: slip behind cover (shooting back) instead
	lootRadius  float64
	wantCubes   int
}

// brawlStance is the free-for-all stance adopted after the first real matches: armed, farming, fighting
// with an edge and declining coin flips. Also used while we do not know the lobby yet.
func (b *Bot) brawlStance(name string) stance {
	return stance{name: name, engageRatio: b.P.BrawlEngage, disengage: b.P.BrawlDisengage, hunt: b.P.BrawlHunt,
		avoid: b.P.AvoidAdv > 0, lootRadius: b.P.BrawlLootRadius, wantCubes: b.P.BrawlWantCubes}
}

// realModel reports whether the measured (team bot) hit rates apply: in a free-for-all, and until we
// know better. Against the house squad the older numbers stay (they are what scored in practice).
func (b *Bot) realModel() bool {
	if b.P.HitModel < 1 {
		return false
	}
	return !b.P.PracticeAware || b.ffa || b.openPhase()
}

// openPhase reports whether we still do not know what kind of lobby this is: the first OpenSecs, and
// for CreditOpenS after a death that we put down to our own shooting without having been given the kill
// (in a free-for-all that is usually somebody else's kill, and one such guess must not send us into the
// practice stance in the middle of a real fight).
func (b *Bot) openPhase() bool {
	return b.t < b.P.OpenSecs || (b.P.CreditOpenS > 0 && b.t-b.creditAbsorbT < b.P.CreditOpenS)
}

// belief updates what we think the lobby is and returns the matching stance.
func (b *Bot) belief() stance {
	// Deaths we did not cause mean the others fight each other. A mech we were just shooting that
	// then dies to something else (a drone, the zone, a stray shell) is credited to us, and one odd
	// death is not enough: the cooperating house squad can lose a member to the zone.
	total := int(stat(b.C.Match, "mechs", 16))
	dead := total - b.o.Alive
	if dead > b.deadSeen {
		n := dead - b.deadSeen
		b.deadSeen = dead
		if b.kills > b.killsSeen {
			n -= b.kills - b.killsSeen
			b.killsSeen = b.kills
		} else if b.t-b.creditT < 6 {
			// we were shooting someone: the death may be that one finished off by something else.
			// Not if we can see that one alive this very moment (then somebody else died, and a
			// fight of our own must not keep hiding a free-for-all from us).
			mine := true
			if b.P.CreditFix {
				if tr := b.tracks[b.creditID]; tr != nil && tr.Listed && tr.HP > 0 {
					mine = false
				}
			}
			if mine {
				n--
				b.creditAbsorbT = b.t
			}
		}
		if n > 0 {
			b.unexplained += n
		}
		if (b.unexplained >= 1 && b.t < 35) || (b.unexplained >= 2 && b.t < 110) || b.unexplained >= 3 {
			b.ffa = true
		}
	}
	alive := b.o.Alive
	if b.P.ForceFFA || b.soundFFA {
		b.ffa = true
	}
	// The seat tells: in every public lobby with house bots (60 recorded) the players sit in the lowest
	// seats and the house bots fill the rest, so in practice (us and 15 house bots) we are in seat 0;
	// where all 16 are team bots the seats are shuffled. Any other seat is a free-for-all from the
	// first tick. (Should practice ever seat us elsewhere, its score will say so at once.)
	if b.P.SlotRule && b.H.Slot != 0 {
		b.ffa = true
	}
	switch {
	case alive <= 2:
		// second place is banked: nothing to lose by fighting for first
		return stance{name: "duel", engageRatio: 0.0, disengage: 0.45, hunt: true, lootRadius: 150, wantCubes: 40}
	case b.ffa && b.P.PracticeAware:
		if b.P.Brawl {
			return b.brawlStance("ffa")
		}
		return stance{name: "ffa", engageRatio: b.P.FFAEngage, disengage: b.P.FFADisengage, hunt: false, stealth: true, lootRadius: b.P.FFALootRadius, wantCubes: b.P.FFAWantCubes}
	case b.openPhase() && b.P.PracticeAware:
		if b.P.Brawl {
			return b.brawlStance("open")
		}
		// not known yet: careful, but not shy about loot close by
		return stance{name: "open", engageRatio: 1.5, disengage: 0.9, hunt: false, stealth: true, lootRadius: 110, wantCubes: b.P.WantCubes}
	}
	return stance{name: "squad", engageRatio: b.P.EngageRatio, disengage: b.P.DisengageRat, hunt: b.P.Hunt, lootRadius: b.P.LootRadius, wantCubes: b.P.WantCubes}
}

type hideCand struct {
	cell  int
	score float64
}

// chooseHide picks a low-exposure place inside the circle we must end up in. Lower score is better.
func (b *Bot) chooseHide(zi zoneInfo) int {
	z := &b.o.Zone
	exp := b.Nav.Exposure()
	comp := b.Nav.Components()
	myComp := int32(0)
	if c := b.Nav.Nearest(b.pos[0], b.pos[1], 2.9, 9, 6); c >= 0 {
		myComp = comp[c]
	}
	// the region we may hide in: the next circle, shrunk by a margin; early in a long hold we also
	// accept anywhere in the current circle that the next shrink will not reach soon
	margin := math.Min(14, z.NextRadius*0.25)
	// KeepAway (free-for-all sets): nearness to the mechs we know of as a cost in its own right, before
	// the shortlist is drawn. In the real matches the time a mech spends near others in its first 30 s,
	// weighted by how near, sorts the field: the quarter with the least of it was hit inside 20 s 14% of
	// the time and scored 9.3, the quarter with the most 75% and 6.5; the teams that die least keep it
	// a third below ours. Mechs seen in the last KeepAwayMem seconds count in full, weapons heard in
	// that time (rough positions) half.
	type kaPt struct{ x, y, w float64 }
	var kaPts []kaPt
	if b.P.KeepAway > 0 {
		for _, tr := range b.tracks {
			if tr.NPC || b.t-tr.LastSeen > b.P.KeepAwayMem {
				continue
			}
			kaPts = append(kaPts, kaPt{tr.Pos[0], tr.Pos[1], 1})
		}
		nTr := len(kaPts)
		for i := range b.noise {
			if b.t-b.noise[i].t > b.P.KeepAwayMem {
				continue
			}
			// (not the mech we can see making it)
			dup := false
			for _, q := range kaPts[:nTr] {
				if hyp(q.x-b.noise[i].x, q.y-b.noise[i].y) < 50 {
					dup = true
					break
				}
			}
			if !dup {
				kaPts = append(kaPts, kaPt{b.noise[i].x, b.noise[i].y, 0.5})
			}
		}
	}
	keepAway := func(x, y float64) float64 {
		s := 0.0
		for _, q := range kaPts {
			s += q.w * nearW(hyp(q.x-x, q.y-y))
		}
		return b.P.KeepAway * s
	}
	var cands []hideCand
	for j := 0; j < exp.NY; j++ {
		for i := 0; i < exp.NX; i++ {
			e := exp.V[j*exp.NX+i]
			if e > 1.5 {
				continue
			}
			k := exp.Cell(i, j)
			if myComp != 0 && comp[k] != myComp {
				continue
			}
			x, y := b.Nav.Pos(k)
			dn := hyp(x-z.NextCenter[0], y-z.NextCenter[1])
			if dn > z.NextRadius-margin {
				continue
			}
			if !b.Nav.Walkable(k, b.radius+0.4, b.height) {
				continue
			}
			d := hyp(x-b.pos[0], y-b.pos[1])
			if d > 320 {
				continue
			}
			// walking is when we get seen: a far better spot has to be clearly better
			s := float64(e)*100 + d*b.hideDistW()
			// the middle of the circle is where everyone ends up walking through
			if z.NextRadius > 60 && dn < z.NextRadius*0.35 {
				s += 10
			}
			// EdgeBias: out towards the edge of the next circle (see the loot chooser); EdgeBand: nothing
			// nearer the middle than this share of its radius
			if b.P.EdgeBias > 0 && z.NextRadius > 60 {
				s -= b.P.EdgeBias * 30 * dn / z.NextRadius
				if b.P.EdgeBand > 0 && dn < z.NextRadius*b.P.EdgeBand {
					s += 60
				}
			}
			// prefer being close to a wall (something to duck behind)
			s += float64(b.Nav.Clear[k]) * 0.8
			if k == b.holdCell {
				s -= 9
				if b.P.KeepAway > 0 {
					s -= b.P.KeepAwayStay // moving is being seen and heard: only for a clearly better place
				}
			}
			if len(kaPts) > 0 {
				s += keepAway(x, y)
			}
			cands = append(cands, hideCand{k, s})
		}
	}
	if len(cands) == 0 {
		return -1
	}
	sort.Slice(cands, func(a, c int) bool { return cands[a].score < cands[c].score })
	if len(cands) > 60 {
		cands = cands[:60]
	}
	// refine the shortlist with what we know about enemies and hot spots
	best, bs := -1, 1e18
	for _, c := range cands {
		x, y := b.Nav.Pos(c.cell)
		gz := float64(b.Nav.Z[c.cell])
		s := c.score
		for _, tr := range b.tracks {
			// ThreatMem: a mech that has hurt us stays a pair of eyes at the place we last saw it for
			// that long, seen since or not. (A real match: hit twice by a laser from 234 m, out of its
			// sight for eight seconds, then back into the same line of fire and dead.)
			// (only one we have actually seen: a track made by a hit from nowhere has no place)
			hurt := b.P.ThreatMem > 0 && !tr.NPC && tr.HurtMeT > 0 && b.t-tr.HurtMeT < b.P.ThreatMem && tr.LastSeen > 0
			if tr.NPC || (b.t-tr.LastSeen > 25 && !hurt) {
				continue
			}
			d := hyp(tr.Pos[0]-x, tr.Pos[1]-y)
			if d < 90 {
				s += (90 - d) * 0.35
			}
			if d < 260 && (b.t-tr.LastSeen < 8 || hurt) {
				sc := tr.scale(b.sizePerCube)
				if b.W.Visible(tr.Pos[0], tr.Pos[1], tr.Pos[2]+8*sc, x, y, gz+b.height*0.6) {
					s += 30
					if hurt {
						s += 60
					}
				}
			}
		}
		for i := range b.o.Drops {
			if hyp(b.o.Drops[i].Pos[0]-x, b.o.Drops[i].Pos[1]-y) < 70 {
				s += 15
			}
		}
		if s < bs {
			best, bs = c.cell, s
		}
	}
	return best
}

// nearW is how much one mech at distance d weighs in the keep-away cost: 1 inside 40 m, a fifth at
// 100 m, a fourteenth at 140 m (a smooth version of the danger measure's bands).
func nearW(d float64) float64 {
	if d <= 40 {
		return 1
	}
	return math.Exp(-(d - 40) / 38)
}

func (b *Bot) hideDistW() float64 {
	if b.P.HideNearW > 0 && b.st.stealth {
		return b.P.HideNearW
	}
	if b.st.stealth {
		return 0.16
	}
	return 0.07
}

// seenBy lists the enemy mechs that currently have a line to us (and are close enough to matter).
func (b *Bot) seenBy(ths []threat, maxD float64) []*threat {
	var out []*threat
	for i := range ths {
		if ths[i].tr.LOS && ths[i].tr.Listed && ths[i].d < maxD {
			out = append(out, &ths[i])
		}
	}
	return out
}
