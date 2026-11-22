package bot

import (
	"math"

	"mechroyale/internal/proto"
)

type zoneInfo struct {
	outNext  float64 // metres outside the next circle (negative: inside)
	outCur   float64 // metres outside the current circle
	tPass    float64 // seconds until the shrinking edge reaches where we stand
	goal     [2]float64
	urgent   bool
	critical bool
	final    bool // the next circle is (almost) a point
	walkable bool // goal can be reached on foot (otherwise we have to go over things)
}

func (b *Bot) zoneInfo() zoneInfo {
	z := &b.o.Zone
	var zi zoneInfo
	dn := hyp(b.pos[0]-z.NextCenter[0], b.pos[1]-z.NextCenter[1])
	dc := hyp(b.pos[0]-z.Center[0], b.pos[1]-z.Center[1])
	zi.outNext = dn - z.NextRadius
	zi.outCur = dc - z.Radius
	zi.tPass = math.Inf(1)
	zi.final = z.NextRadius < 12
	t0 := math.Max(b.t, z.ShrinkStart)
	if zi.outCur > 0 {
		zi.tPass = 0
	} else if zi.outNext > 0 && z.ShrinkEnd > t0 {
		for i := 1; i <= 50; i++ {
			f := float64(i) / 50
			cx := z.Center[0] + (z.NextCenter[0]-z.Center[0])*f
			cy := z.Center[1] + (z.NextCenter[1]-z.Center[1])*f
			r := z.Radius + (z.NextRadius-z.Radius)*f
			if hyp(b.pos[0]-cx, b.pos[1]-cy) > r {
				zi.tPass = (t0 - b.t) + f*(z.ShrinkEnd-t0)
				break
			}
		}
	} else if zi.outNext > 0 && z.ShrinkEnd <= t0 {
		zi.tPass = 0
	}
	// where to stand: inside the next circle, on our side of it
	dx, dy := norm2(b.pos[0]-z.NextCenter[0], b.pos[1]-z.NextCenter[1])
	gr := math.Min(z.NextRadius*0.6, z.NextRadius-10)
	if gr < 0 {
		gr = 0
	}
	zi.goal = [2]float64{z.NextCenter[0] + dx*gr, z.NextCenter[1] + dy*gr}
	zi.walkable = true
	// The goal must be a cell we can stand on that is really inside the next circle (snapping to the
	// nearest walkable cell can land outside it when the ideal point is in a building) and, if at
	// all possible, one we can walk to. Recomputed about once a second.
	key := [3]float64{z.NextCenter[0], z.NextCenter[1], z.NextRadius}
	if key != b.zgKey || b.t-b.zgT > 1.0 || b.zgComp != b.myComp {
		b.zgKey, b.zgT, b.zgComp = key, b.t, b.myComp
		margin := math.Min(5, z.NextRadius*0.3)
		base := math.Atan2(dy, dx)
		found, foundAny := false, false
		var anyGoal [2]float64
	search:
		for _, fr := range [...]float64{0.6, 0.45, 0.75, 0.3, 0.15, 0.9, 0.0} {
			rr := math.Max(0, math.Min(z.NextRadius*fr, z.NextRadius-10))
			for _, off := range [...]float64{0, 0.4, -0.4, 0.8, -0.8, 1.4, -1.4, 2.2, -2.2, math.Pi} {
				x, y := z.NextCenter[0]+math.Cos(base+off)*rr, z.NextCenter[1]+math.Sin(base+off)*rr
				k := b.Nav.Nearest(x, y, b.radius+0.3, b.height, 4)
				if k < 0 {
					continue
				}
				px, py := b.Nav.Pos(k)
				if z.NextRadius > 12 && hyp(px-z.NextCenter[0], py-z.NextCenter[1]) > z.NextRadius-margin {
					continue
				}
				if !b.sameComp(k) {
					if !foundAny {
						foundAny, anyGoal = true, [2]float64{px, py}
					}
					continue
				}
				b.zgGoal, found = [2]float64{px, py}, true
				break search
			}
		}
		b.zgWalk = found
		if !found {
			if foundAny {
				b.zgGoal = anyGoal // standable but not on foot from here: we will go over things
			} else {
				b.zgGoal = [2]float64{z.NextCenter[0], z.NextCenter[1]}
			}
		}
	}
	zi.goal, zi.walkable = b.zgGoal, b.zgWalk
	// time we need to get inside, with slack for detours
	need := 0.0
	if zi.outNext > 0 {
		need = (zi.outNext+8)/b.walk*1.35 + 4
		if !zi.walkable && b.P.OverlandEarly > 0 {
			// the way in is over walls (we are in the stadium bowl, or on a roof with no walk down):
			// that is slow and can fail, so start long before the edge arrives
			need = need*b.P.OverlandEarly + 25
		}
	}
	zi.critical = zi.outCur > -2 || zi.tPass < 2.5
	zi.urgent = zi.critical || zi.tPass < 9 || (zi.outNext > 0 && z.ShrinkEnd-b.t < need) || (zi.outNext > 0 && zi.tPass-need < 3)
	return zi
}

type threat struct {
	tr         *Track
	d          float64
	adv        float64 // best advantage ratio over the ranges we could fight at
	rmin, rmax float64 // the range band that gives it
	engaged    bool
	dps        float64
	phantom    bool // unseen attacker placed along a damage bearing
}

func (b *Bot) assess() []threat {
	var out []threat
	for _, tr := range b.tracks {
		if tr.NPC {
			continue
		}
		phantom := !tr.Listed && b.t-tr.PhantomT < 4 && b.t-tr.LastSeen > 1
		if b.t-tr.LastSeen > 8 && !phantom {
			continue
		}
		d := dist3(b.pos, tr.Pos)
		th := threat{tr: tr, d: d, phantom: phantom}
		th.adv, th.rmin, th.rmax = b.fightPlan(tr, d)
		if b.P.ExchangeW > 0 && !tr.NPC && (tr.HurtMe > 1 || tr.IHurt > 1) {
			// how the exchange with this one is actually going says more than any table
			ex := (tr.IHurt + 150) / (tr.HurtMe + 150)
			th.adv *= math.Pow(clamp(ex, 0.4, 1.8), b.P.ExchangeW)
		}
		th.dps = b.theirDPS(tr, d)
		aimR := 200.0
		if tr.Laser && b.P.LaserAimRange > aimR && b.realModel() {
			aimR = b.P.LaserAimRange // a laser reaches 280 m and hits there as well as anywhere
		}
		th.engaged = b.t-tr.HurtMeT < 4 || (tr.LOS && b.t-tr.AimAtMeT < 0.6 && d < aimR) || (tr.LOS && d < 40)
		if !th.engaged && tr.LOS && d < 90 {
			// walking at us with its torso turned our way: it is about to open up, do not wait for it
			bear := math.Atan2(b.pos[1]-tr.Pos[1], b.pos[0]-tr.Pos[0])
			closing := (tr.Vel[0]*(b.pos[0]-tr.Pos[0]) + tr.Vel[1]*(b.pos[1]-tr.Pos[1])) / math.Max(d, 1)
			if math.Abs(angDiff(tr.Aim[0], bear)) < 0.5 && closing > 4 {
				th.engaged = true
			}
		}
		if phantom {
			// we only know it is hurting us: assume the worst until we can see it
			th.engaged = true
			th.dps = math.Max(th.dps, tr.HurtMe/3)
			th.adv = math.Min(th.adv, 0.5)
		}
		// a little persistence, so a flag that flickers does not flip our whole stance every tick
		if th.engaged {
			tr.EngT = b.t
		} else if b.t-tr.EngT < 0.8 && tr.Listed {
			th.engaged = true
		}
		out = append(out, th)
	}
	return out
}

// sameComp reports whether a nav cell is in the walkable region we are standing in.
func (b *Bot) sameComp(cell int) bool {
	if b.myComp == 0 || cell < 0 {
		return true
	}
	c := b.Nav.Components()[cell]
	return c == b.myComp
}

// reachable reports whether a ground point can be walked to from here (same walkable region).
func (b *Bot) reachable(x, y float64) bool {
	if b.myComp == 0 {
		return true
	}
	c := b.Nav.Nearest(x, y, 2.9, 9, 3)
	return c >= 0 && b.Nav.Components()[c] == b.myComp
}

// hiddenHere reports whether none of the threats can see us where we stand right now.
func (b *Bot) hiddenHere(ths []threat) bool {
	for i := range ths {
		tr := ths[i].tr
		if (b.t-tr.LastSeen > 6 && !ths[i].phantom) || ths[i].d > 260 {
			continue
		}
		sc := tr.scale(b.sizePerCube)
		ex, ey, ez := tr.Pos[0], tr.Pos[1], tr.Pos[2]+8*sc
		if b.W.Visible(ex, ey, ez, b.pos[0], b.pos[1], b.pos[2]+b.height*0.5) || b.W.Visible(ex, ey, ez, b.pos[0], b.pos[1], b.pos[2]+b.height*0.95) {
			return false
		}
	}
	return true
}

// findCover looks for the nearest walkable cell hidden from every listed threat.
func (b *Bot) findCover(ths []threat, maxCost float64) int {
	type eyeT struct{ x, y, z float64 }
	var eyes []eyeT
	for i := range ths {
		tr := ths[i].tr
		if (b.t-tr.LastSeen > 6 && !ths[i].phantom) || ths[i].d > 260 {
			continue
		}
		sc := tr.scale(b.sizePerCube)
		eyes = append(eyes, eyeT{tr.Pos[0], tr.Pos[1], tr.Pos[2] + 8*sc})
	}
	if len(eyes) == 0 {
		return -1
	}
	z := &b.o.Zone
	found := -1
	checked := 0
	b.Nav.Flood(b.pos[0], b.pos[1], b.radius+0.15, b.height, maxCost, nil, func(k int, cost float32) bool {
		if (k%b.Nav.NX+k/b.Nav.NX)%2 != 0 {
			return true
		}
		if float64(b.Nav.Clear[k]) < b.radius+0.6 {
			return true
		}
		x, y := b.Nav.Pos(k)
		if hyp(x-z.Center[0], y-z.Center[1]) > z.Radius-6 {
			return true
		}
		if z.ShrinkEnd-b.t < 25 && hyp(x-z.NextCenter[0], y-z.NextCenter[1]) > z.NextRadius-4 {
			return true
		}
		checked++
		if checked > 700 {
			return false
		}
		gz := float64(b.Nav.Z[k])
		for _, e := range eyes {
			// hidden means neither our middle nor our head can be seen
			if b.W.Visible(e.x, e.y, e.z, x, y, gz+b.height*0.5) || b.W.Visible(e.x, e.y, e.z, x, y, gz+b.height*0.95) {
				return true
			}
		}
		found = k
		return false
	})
	return found
}

// cellHidden reports whether a mech standing in the cell is out of sight of every recent threat's
// last known position.
func (b *Bot) cellHidden(k int, ths []threat) bool {
	x, y := b.Nav.Pos(k)
	gz := float64(b.Nav.Z[k])
	for i := range ths {
		tr := ths[i].tr
		if (b.t-tr.LastSeen > 6 && !ths[i].phantom) || ths[i].d > 260 {
			continue
		}
		sc := tr.scale(b.sizePerCube)
		ex, ey, ez := tr.Pos[0], tr.Pos[1], tr.Pos[2]+8*sc
		if b.W.Visible(ex, ey, ez, x, y, gz+b.height*0.5) || b.W.Visible(ex, ey, ez, x, y, gz+b.height*0.95) {
			return false
		}
	}
	return true
}

// nearestEye is the distance to the nearest recent threat's last known position (1e9: none).
func (b *Bot) nearestEye(ths []threat) float64 {
	d0 := 1e9
	for i := range ths {
		tr := ths[i].tr
		if (b.t-tr.LastSeen > 6 && !ths[i].phantom) || ths[i].d > 260 {
			continue
		}
		if d := hyp(tr.Pos[0]-b.pos[0], tr.Pos[1]-b.pos[1]); d < d0 {
			d0 = d
		}
	}
	return d0
}

type loot struct {
	kind  string // "pickup", "cube", "crate", "npc", "drop"
	id    int
	pos   [3]float64
	score float64
	shoot bool
}

// weaponValue ranks the weapons for pickup and swap decisions.
func (b *Bot) weaponValue(kind string) float64 {
	switch kind {
	case "laser":
		return b.P.LaserValue
	case "lightning":
		return b.P.LightningValue
	case "mortar":
		return 1.2
	case "gun":
		return 2.0
	}
	return 0
}

// wantPickup reports whether we would take this weapon and which slot it would replace (-1: empty slot).
func (b *Bot) wantPickup(kind string) (bool, int) {
	if b.has(kind) >= 0 {
		return false, -1
	}
	if kind == "mortar" && (b.P.NoMortarHard || (b.P.NoMortar && b.realModel())) {
		return false, -1
	}
	for i := 0; i < 2; i++ {
		if !b.slots[i].ok {
			return true, -1
		}
	}
	// both full: replace the weaker non-gun weapon if the new one is clearly better
	worst, wv := -1, 1e9
	for i := 0; i < 2; i++ {
		if b.slots[i].kind == "gun" {
			continue
		}
		if v := b.weaponValue(b.slots[i].kind); v < wv {
			worst, wv = i, v
		}
	}
	if worst >= 0 && b.weaponValue(kind) > wv+0.3 && b.slots[worst].charge <= 0 {
		return true, worst
	}
	return false, -1
}

// inFutureZone reports whether a point will still be inside the zone in `secs` seconds (roughly).
func (b *Bot) inFutureZone(x, y, secs, margin float64) bool {
	z := &b.o.Zone
	if hyp(x-z.Center[0], y-z.Center[1]) > z.Radius-margin {
		return false
	}
	if b.t+secs > z.ShrinkStart {
		f := 1.0
		if z.ShrinkEnd > z.ShrinkStart {
			f = clamp((b.t+secs-z.ShrinkStart)/(z.ShrinkEnd-z.ShrinkStart), 0, 1)
		}
		// conservative: interpolate from the current circle toward the next one
		cx := z.Center[0] + (z.NextCenter[0]-z.Center[0])*f
		cy := z.Center[1] + (z.NextCenter[1]-z.Center[1])*f
		r := z.Radius + (z.NextRadius-z.Radius)*f
		if hyp(x-cx, y-cy) > r-margin {
			return false
		}
	}
	return true
}

func (b *Bot) enemyNear(pos [3]float64, r float64) bool {
	for _, tr := range b.tracks {
		if tr.NPC || b.t-tr.LastSeen > 6 {
			continue
		}
		if dist2(tr.Pos, pos) < r {
			return true
		}
	}
	return false
}

func (b *Bot) chooseLoot(danger bool, st stance) *loot {
	var best *loot
	// where the mechs we know about are looking from (for the out-of-sight preference)
	var watchEyes [][3]float64
	// (the opening is stricter: see the "shy" rule in think)
	strict := b.shyPhase(st)
	mem := 4.0
	if strict {
		mem = b.P.ShyMem
	}
	if (b.P.LootHiddenBias > 0 || strict) && st.avoid {
		for _, tr := range b.tracks {
			if tr.NPC || b.t-tr.LastSeen > mem {
				continue
			}
			sc := tr.scale(b.sizePerCube)
			watchEyes = append(watchEyes, [3]float64{tr.Pos[0], tr.Pos[1], tr.Pos[2] + 8*sc})
			if len(watchEyes) >= 6 {
				break
			}
		}
	}
	// OpenAway (opening sets): errands that take us towards a mech we know of are worth less. In the real
	// matches mechs that began with one 100-150 m off and got 15 m+ closer in the first 10 s were dead
	// inside a minute 47% of the time (6.0 points); those that kept the distance 26% (7.6); those that
	// opened it 15% (8.6). Nothing is forbidden (the stricter "shy" rule costs kills in practice).
	var awayFrom [][3]float64
	if b.P.OpenAway > 0 && st.name == "open" {
		for _, tr := range b.tracks {
			if tr.NPC || b.t-tr.LastSeen > 6 || dist2(b.pos, tr.Pos) > 220 {
				continue
			}
			awayFrom = append(awayFrom, tr.Pos)
		}
	}
	// EdgeBias (free-for-all sets): stay out towards the edge of the next circle and away from the mechs
	// we know of. In the real matches (4 PM to midnight Saturday) the two teams with the best
	// free-for-all records stood at 0.6-0.7 of the zone radius from its centre at 90-120 s and 140-200 m
	// from the nearest mech; everyone else, we included, at 0.4-0.55 and 100-135 m.
	// ThreatMem: nothing that lies in the sight of a mech that has just hurt us (from where we last saw it)
	var threatEyes [][3]float64
	if b.P.ThreatMem > 0 && (st.avoid || st.stealth) {
		for _, tr := range b.tracks {
			if tr.NPC || tr.HurtMeT <= 0 || b.t-tr.HurtMeT >= b.P.ThreatMem || tr.LastSeen <= 0 {
				continue
			}
			sc := tr.scale(b.sizePerCube)
			threatEyes = append(threatEyes, [3]float64{tr.Pos[0], tr.Pos[1], tr.Pos[2] + 8*sc})
		}
	}
	var edgeMechs [][3]float64
	if b.P.EdgeBias > 0 && b.ffa {
		for _, tr := range b.tracks {
			if tr.NPC || b.t-tr.LastSeen > 10 {
				continue
			}
			edgeMechs = append(edgeMechs, tr.Pos)
		}
	}
	consider := func(l loot, value, handling float64) {
		d := dist2(b.pos, l.pos)
		if d > st.lootRadius {
			return
		}
		if b.P.EdgeBias > 0 && b.ffa && l.kind != "drop" {
			if z := &b.o.Zone; z.NextRadius > 60 {
				f := hyp(l.pos[0]-z.NextCenter[0], l.pos[1]-z.NextCenter[1]) / z.NextRadius
				value *= math.Max(0.2, 1+b.P.EdgeBias*0.6*(clamp(f, 0, 1.1)-0.5))
				if b.P.EdgeBand > 0 && f < b.P.EdgeBand {
					value *= 0.4
				}
			}
			dm := 120.0
			for _, e := range edgeMechs {
				if de := hyp(e[0]-l.pos[0], e[1]-l.pos[1]); de < dm {
					dm = de
				}
			}
			if b.P.EdgeNoise > 0 {
				// ... and from where weapons were heard a moment ago (rough positions: half the weight)
				dn := 120.0
				for i := range b.noise {
					if b.t-b.noise[i].t > b.P.EdgeNoise {
						continue
					}
					if de := hyp(b.noise[i].x-l.pos[0], b.noise[i].y-l.pos[1]); de < dn {
						dn = de
					}
				}
				value *= math.Max(0.15, 1-b.P.EdgeBias*0.3*(1-dn/120))
			}
			if b.P.EdgeClamp {
				// (later builds: nothing at all right beside a mech, and never a negative worth)
				f := 1 - b.P.EdgeBias*0.6*(1-dm/120)
				if f < 0.05 {
					return
				}
				value *= f
			} else {
				value *= 1 - b.P.EdgeBias*0.6*(1-dm/120)
			}
		}
		if len(awayFrom) > 0 {
			laserClose := false
			if l.kind == "pickup" && b.has("laser") < 0 && d < 60 {
				if it := b.pickups[l.id]; it != nil && it.Kind == "laser" {
					laserClose = true
				}
			}
			if !laserClose {
				for _, e := range awayFrom {
					de := hyp(e[0]-l.pos[0], e[1]-l.pos[1])
					if de < 150 && de < hyp(e[0]-b.pos[0], e[1]-b.pos[1])-15 {
						value *= 1 - b.P.OpenAway
						break
					}
				}
			}
		}
		if until, banned := b.banUntil[l.id]; banned && b.t < until {
			return
		}
		if !b.reachable(l.pos[0], l.pos[1]) {
			return
		}
		for _, e := range threatEyes {
			if hyp(e[0]-l.pos[0], e[1]-l.pos[1]) < 270 && b.W.Visible(e[0], e[1], e[2], l.pos[0], l.pos[1], l.pos[2]+4.5) {
				return
			}
		}
		if st.stealth {
			// in the open is where we get shot: discount exposed loot
			if e := b.Nav.Exposure().At(b.Nav.Index(l.pos[0], l.pos[1])); e <= 1 {
				value *= 1 - 0.6*float64(e)
			}
		}
		tt := d*1.25/b.walk + handling
		// (with ZoneSlack: the edge must not arrive there for a while after we do, or the zone rule
		// turns us round the moment we get close and we shuttle between the two)
		if !b.inFutureZone(l.pos[0], l.pos[1], tt+2+b.P.ZoneSlack, 6) {
			return
		}
		if b.enemyNear(l.pos, 45) && l.kind != "cube" {
			value *= 0.35
		}
		isLaser := false
		if l.kind == "pickup" && b.has("laser") < 0 {
			if it := b.pickups[l.id]; it != nil && it.Kind == "laser" {
				isLaser = true
			}
		}
		if strict {
			// opening: short errands (in the real matches mechs that had gone 60 m+ by 10 s did a
			// point worse than those that had gone less), and sheltered ones before open ones
			if b.P.ShyLootRadius > 0 && d > b.P.ShyLootRadius && !isLaser {
				return
			}
			if b.P.ShyExposure > 0 {
				if e := b.Nav.Exposure().At(b.Nav.Index(l.pos[0], l.pos[1])); e <= 1 {
					value *= 1 - b.P.ShyExposure*float64(e)
				}
			}
		}
		if strict && len(watchEyes) > 0 {
			// opening: nothing that would have us standing in a known mech's sight, or closing on it
			// (a laser a few steps away excepted)
			laserHere := isLaser && d < b.P.ShyLaser
			if !laserHere {
				for _, e := range watchEyes {
					de := hyp(e[0]-l.pos[0], e[1]-l.pos[1])
					if de < 230 && b.W.Visible(e[0], e[1], e[2], l.pos[0], l.pos[1], l.pos[2]+4.5) {
						return
					}
					if de < 150 && de < hyp(e[0]-b.pos[0], e[1]-b.pos[1])-10 {
						return
					}
				}
			}
		} else if b.P.LootHiddenBias > 0 && st.avoid && len(watchEyes) > 0 {
			// errands out of other mechs' sight first: in the real matches everyone caught up in a
			// fight in the first half minute, whoever started it, finished a point below the rest
			for _, e := range watchEyes {
				if hyp(e[0]-l.pos[0], e[1]-l.pos[1]) < 230 && b.W.Visible(e[0], e[1], e[2], l.pos[0], l.pos[1], l.pos[2]+4.5) {
					value *= 1 - b.P.LootHiddenBias
					break
				}
			}
		}
		l.score = value / (tt + 1.5)
		if l.id == b.lootID {
			l.score *= 1.25 // stick with the current pick
		}
		if best == nil || l.score > best.score {
			c := l
			best = &c
		}
	}
	wantCubes := b.o.Self.Cubes < st.wantCubes
	// Health comes back at 45 a second once we have neither dealt nor taken damage for 5 s, and shooting
	// a crate or a drone counts as dealing damage. So when hurt and left alone: no crates, no drones,
	// until it is back (cubes on the ground and weapons cost nothing).
	if b.P.HealFirst > 0 {
		switch {
		case b.hp < b.maxHP*b.P.HealFirst:
			b.healing = true
		case b.hp >= b.maxHP*b.P.HealUntil:
			b.healing = false
		}
	}
	noShoot := b.P.HealFirst > 0 && b.healing && !danger
	for _, it := range b.pickups {
		if !it.Ground {
			continue
		}
		if ok, rep := b.wantPickup(it.Kind); ok {
			v := b.weaponValue(it.Kind) * 4
			if rep >= 0 {
				v = (b.weaponValue(it.Kind) - b.weaponValue(b.slots[rep].kind)) * 4
			}
			consider(loot{kind: "pickup", id: it.ID, pos: it.Pos}, v, 0.5)
		}
	}
	if wantCubes {
		for _, it := range b.cubes {
			if it.Ground {
				consider(loot{kind: "cube", id: it.ID, pos: it.Pos}, 11, 0)
			}
		}
		for _, it := range b.crates {
			// roof crates can still be shot from the ground, but the cube stays up there: skip them
			if it.Ground && !noShoot {
				consider(loot{kind: "crate", id: it.ID, pos: it.Pos, shoot: true}, 7, 2.5)
			}
		}
		if !danger && !noShoot {
			for _, tr := range b.tracks {
				if tr.NPC && b.t-tr.LastSeen < 20 {
					k := b.Nav.Index(tr.Pos[0], tr.Pos[1])
					if !b.Nav.Solid[k] && math.Abs(float64(b.Nav.Z[k])-tr.Pos[2]) < 4 {
						consider(loot{kind: "npc", id: tr.ID, pos: tr.Pos, shoot: true}, 11, 5)
					}
				}
			}
		}
	}
	// Supply drops: three cubes and a full repair for four seconds alone in the ring. In the top-16
	// matches almost nobody but the eventual winner collects them. Everyone knows where they are, so
	// we go only when nobody has been seen near one, and not long before it lands.
	for i := range b.o.Drops {
		dr := &b.o.Drops[i]
		d := dist2(b.pos, dr.Pos)
		if d > b.P.DropRadius {
			continue
		}
		if dr.LandsIn > d/b.walk*1.3+6 && !(b.P.DropStick && dr.ID == b.lootID) {
			// (once we are on our way to one we stay on it: the time budget shrinks as we approach,
			// and without that the "not yet" rule sent us to and fro every other decision)
			continue
		}
		if b.P.DropStick && st.avoid && b.o.Self.Cubes >= st.wantCubes && b.hp > b.maxHP*0.7 {
			// three more cubes past the stat cap only make us a bigger target: a drop is then worth
			// it for the repair alone
			continue
		}
		if dr.Holder != nil && *dr.Holder != b.o.Self.ID && st.stealth {
			continue // someone is already on it
		}
		if st.stealth && b.enemyNear(dr.Pos, 110) {
			continue
		}
		v := 30.0 * b.P.DropGreed * 2
		if b.hp < b.maxHP*0.6 {
			v *= 1.5
		}
		saved := st.lootRadius
		st.lootRadius = b.P.DropRadius
		consider(loot{kind: "drop", id: dr.ID, pos: dr.Pos}, v, 4+dr.LandsIn*0.3)
		st.lootRadius = saved
	}
	return best
}

var upgradeRank = map[string]float64{
	"plating": 1.00, "armor": 0.96, "gun_rate": 0.90, "gun_damage": 0.88, "gun_velocity": 0.80,
	"laser_damage": 0.91, "laser_charge": 0.86, "lightning_efficiency": 0.92, "lightning_range": 0.87,
	"nanites": 0.72, "servos": 0.76, "reactor": 0.50, "capacitor": 0.45, "gun_mag": 0.60,
	"sensors": 0.35, "gyros": 0.30, "jets": 0.30, "mortar_reload": 0.42, "mortar_splash": 0.45,
	"mortar_velocity": 0.40, "lightning_chain": 0.40, "laser_pierce": 0.20,
}

func (b *Bot) pickUpgrade(act *proto.Action) {
	of := b.o.Offer
	if of == nil || len(of.Choices) == 0 {
		return
	}
	best, bv := 0, -1.0
	for i, c := range of.Choices {
		v, ok := upgradeRank[c.ID]
		if !ok {
			v = 0.25
		}
		if b.has("lightning") >= 0 && (c.ID == "reactor" || c.ID == "capacitor") {
			v += 0.3
		}
		if b.P.UpOffense > 0 {
			switch c.ID {
			case "gun_rate", "gun_damage", "laser_damage", "laser_charge":
				v += b.P.UpOffense
			}
		}
		if v > bv {
			best, bv = i, v
		}
	}
	act.Pick = best
}

// manageWeapons takes a wanted weapon when we stand on it.
func (b *Bot) manageWeapons(act *proto.Action) {
	if b.t < b.takeT {
		return
	}
	reach := b.C.Loot.PickupRadius
	if reach <= 0 {
		reach = 4
	}
	bestID, bestV, bestRep := -1, 0.0, -1
	for _, it := range b.pickups {
		if b.t-it.LastSeen > 0.5 {
			continue
		}
		if dist2(b.pos, it.Pos) > reach-0.3 || math.Abs(it.Pos[2]-b.pos[2]) > 3.7 {
			continue
		}
		if ok, rep := b.wantPickup(it.Kind); ok {
			if v := b.weaponValue(it.Kind); v > bestV {
				bestID, bestV, bestRep = it.ID, v, rep
			}
		}
	}
	if bestID >= 0 {
		act.Take = bestID
		if bestRep >= 0 {
			act.Drop = bestRep
		}
		b.takeT = b.t + 0.25
	}
}

