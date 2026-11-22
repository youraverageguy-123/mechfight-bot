package bot

import (
	"math"

	"mechroyale/internal/proto"
	"mechroyale/internal/world"
)

// Roof play: a mech standing back from the edge of a roof cannot be seen or hit from the ground
// around the building (the edge hides its body), and getting up there costs a third of an energy bar.

const (
	roofNone = iota
	roofApproach
	roofClimb
	roofOn
)

type roofPlan struct {
	state  int
	id     int
	acc    world.RoofAccess
	t0     float64 // when the current state began
	ban    map[int]float64
	chosen float64 // last time we evaluated roofs
	tries  int
	stepT  float64 // last time the roof plan was the thing we were doing
	waitT0 float64 // since when we have been standing at the wall waiting for watchers to go (0: not waiting)
	// set by the hideout chooser: the roof to use and the cell on it to stand at (-1: choose by the
	// older geometric rule)
	want int
	spot int
}

// onRoof returns the id of the roof region we are standing on, or -1.
func (b *Bot) onRoof() int {
	roofs, roofOf := b.Nav.Roofs()
	k := b.Nav.Index(b.pos[0], b.pos[1])
	id := int(roofOf[k])
	if id < 0 || id >= len(roofs) {
		return -1
	}
	if math.Abs(b.pos[2]-roofs[id].Z) < 1.6 {
		return id
	}
	return -1
}

func (b *Bot) maxClimb() float64 {
	drain := stat(b.o.Self.Stats, "jet_drain", b.mech("jet_drain", 15))
	if drain <= 0 {
		drain = 15
	}
	// rise is about 8.9 m/s; keep a reserve for getting over the edge
	return (b.o.Self.MaxEnergy - 28) / drain * 8.9
}

// roofSpotOK reports whether a roof's inner spot will stay inside the zone long enough to be worth it.
func (b *Bot) roofSpotOK(r *world.Roof) bool {
	z := &b.o.Zone
	x, y := b.Nav.Pos(r.Inner)
	if b.roof.want == r.ID && b.roof.spot >= 0 {
		x, y = b.Nav.Pos(b.roof.spot)
	}
	return hyp(x-z.NextCenter[0], y-z.NextCenter[1]) < z.NextRadius-8
}

// chooseRoof picks the best roof to hide on, with the access point to climb at.
func (b *Bot) chooseRoof() (int, world.RoofAccess, bool) {
	roofs, _ := b.Nav.Roofs()
	if w := b.roof.want; w >= 0 && w < len(roofs) {
		// the hideout chooser already picked the roof: find the way up
		acc, _, ok := b.roofAccessFor(&roofs[w], math.Min(b.maxClimb(), b.P.RoofMaxClimb))
		return w, acc, ok
	}
	comp := b.Nav.Components()
	myComp := int32(0)
	if c := b.Nav.Nearest(b.pos[0], b.pos[1], 2.9, 9, 6); c >= 0 {
		myComp = comp[c]
	}
	limit := math.Min(b.maxClimb(), b.P.RoofMaxClimb)
	bestID, bestS := -1, 1e18
	var bestAcc world.RoofAccess
	for i := range roofs {
		r := &roofs[i]
		if until, ok := b.roof.ban[i]; ok && b.t < until {
			continue
		}
		if r.Height < b.P.RoofMinHeight || r.InnerD < 6+b.radius || !b.roofSpotOK(r) {
			continue
		}
		if hyp(r.CX-b.pos[0], r.CY-b.pos[1]) > 260+math.Sqrt(r.Area) {
			continue
		}
		// nearest usable access point
		var acc world.RoofAccess
		ad := 1e18
		for _, a := range r.Access {
			if a.Climb > limit || a.Climb < 6 {
				continue
			}
			if myComp != 0 && comp[a.Ground] != myComp {
				continue
			}
			if !b.Nav.Walkable(a.Ground, b.radius+0.2, b.height) {
				continue
			}
			gx, gy := b.Nav.Pos(a.Ground)
			if d := hyp(gx-b.pos[0], gy-b.pos[1]); d < ad {
				acc, ad = a, d
			}
		}
		if ad > 1e17 {
			continue
		}
		s := ad*1.2 - math.Min(r.InnerD, 25)*2.5 - math.Min(r.Height, 30)*1.5
		ix, iy := b.Nav.Pos(r.Inner)
		if b.P.RoofCenterW > 0 {
			// a roof nearer the middle of the next circle is likelier to survive the shrink after it:
			// every move between roofs is a walk in the open, and that is where the roof-sitter died
			z := &b.o.Zone
			s += hyp(ix-z.NextCenter[0], iy-z.NextCenter[1]) * b.P.RoofCenterW
		}
		if b.P.EdgeBias > 0 && b.o.Zone.NextRadius > 60 {
			// EdgeBias: the outer roofs of the next circle first (see the loot chooser)
			z := &b.o.Zone
			dn := hyp(ix-z.NextCenter[0], iy-z.NextCenter[1])
			s -= b.P.EdgeBias * 0.25 * math.Min(dn, z.NextRadius)
			if b.P.EdgeBand > 0 && dn < z.NextRadius*b.P.EdgeBand {
				s += 120
			}
		}
		for _, tr := range b.tracks {
			if tr.NPC || b.t-tr.LastSeen > 20 {
				continue
			}
			if d := hyp(tr.Pos[0]-ix, tr.Pos[1]-iy); d < 120 {
				s += (120 - d) * 0.6
			}
		}
		if i == b.roof.id && b.roof.state != roofNone {
			s -= 40
		}
		if s < bestS {
			bestID, bestS, bestAcc = i, s, acc
		}
	}
	return bestID, bestAcc, bestID >= 0
}

// roofSpot returns where to stand on roof id: climb the distance-to-edge field from where we are.
func (b *Bot) roofSpot(id int) (float64, float64) {
	roofs, roofOf := b.Nav.Roofs()
	dist := b.Nav.RoofDist()
	r := &roofs[id]
	if s := b.roof.spot; b.roof.want == id && s >= 0 && int(roofOf[s]) == id {
		return b.Nav.Pos(s) // the cell the hideout chooser picked on this roof
	}
	k := b.Nav.Index(b.pos[0], b.pos[1])
	if int(roofOf[k]) != id {
		return b.Nav.Pos(r.Inner)
	}
	// straight to the deepest cell when the way there stays on the roof
	ix, iy := b.Nav.Pos(r.Inner)
	d := hyp(ix-b.pos[0], iy-b.pos[1])
	steps := int(d/2) + 1
	clear := true
	for s := 1; s <= steps; s++ {
		t := float64(s) / float64(steps)
		c := b.Nav.Index(b.pos[0]+(ix-b.pos[0])*t, b.pos[1]+(iy-b.pos[1])*t)
		if int(roofOf[c]) != id || dist[c] < 2 {
			clear = false
			break
		}
	}
	if clear {
		return ix, iy
	}
	// otherwise walk uphill on the edge-distance field
	ci, cj := k%b.Nav.NX, k/b.Nav.NX
	best, bd := k, dist[k]
	for dj := -3; dj <= 3; dj++ {
		for di := -3; di <= 3; di++ {
			i, j := ci+di, cj+dj
			if i < 0 || j < 0 || i >= b.Nav.NX || j >= b.Nav.NY {
				continue
			}
			c := j*b.Nav.NX + i
			if int(roofOf[c]) == id && dist[c] > bd {
				best, bd = c, dist[c]
			}
		}
	}
	return b.Nav.Pos(best)
}

// roofCover finds a spot on our roof that none of the given threats can see (body or head), nearest
// first. It returns -1 when the roof cannot hide us from them.
func (b *Bot) roofCover(id int, ths []threat) int {
	roofs, roofOf := b.Nav.Roofs()
	dist := b.Nav.RoofDist()
	r := &roofs[id]
	type eyeT struct{ x, y, z float64 }
	var eyes []eyeT
	for i := range ths {
		tr := ths[i].tr
		if ths[i].d > 280 || (b.t-tr.LastSeen > 8 && !ths[i].phantom) {
			continue
		}
		sc := tr.scale(b.sizePerCube)
		eyes = append(eyes, eyeT{tr.Pos[0], tr.Pos[1], tr.Pos[2] + 8*sc})
	}
	if len(eyes) == 0 {
		return -1
	}
	best, bs := -1, 1e18
	for _, c := range r.Cells {
		k := int(c)
		ci, cj := k%b.Nav.NX, k/b.Nav.NX
		if (ci+cj)%2 != 0 || dist[k] < 3 || int(roofOf[k]) != id {
			continue
		}
		x, y := b.Nav.Pos(k)
		d := hyp(x-b.pos[0], y-b.pos[1])
		if d > 50 || d >= bs {
			continue
		}
		hidden := true
		for _, e := range eyes {
			if b.W.Visible(e.x, e.y, e.z, x, y, r.Z+b.height*0.5) || b.W.Visible(e.x, e.y, e.z, x, y, r.Z+b.height*0.97) {
				hidden = false
				break
			}
		}
		if hidden {
			best, bs = k, d
		}
	}
	return best
}

// enemyOnRoof reports whether a mech is up here with us (same level, close).
func (b *Bot) enemyOnRoof(ths []threat) bool {
	for i := range ths {
		tr := ths[i].tr
		if tr.Listed && ths[i].d < 70 && math.Abs(tr.Pos[2]-b.pos[2]) < 5 {
			return true
		}
	}
	return false
}

// roofClimb drives the jet climb. It writes the action directly (the ground planner does not apply).
func (b *Bot) roofClimbStep(act *proto.Action) {
	roofs, roofOf := b.Nav.Roofs()
	r := &roofs[b.roof.id]
	ex, ey := b.Nav.Pos(b.roof.acc.Edge)
	dx, dy := norm2(ex-b.pos[0], ey-b.pos[1])
	act.Move = [2]float64{dx, dy}
	feet := b.pos[2]
	cell := b.Nav.Index(b.pos[0], b.pos[1])
	over := int(roofOf[cell]) == b.roof.id
	grounded := b.o.Self.Grounded
	switch {
	case grounded && over && math.Abs(feet-r.Z) < 1.6:
		b.roof.state = roofOn
		b.roof.t0 = b.t
		b.roof.tries = 0
		return
	case grounded && feet < r.Z-1.0:
		act.Jump = true
	case feet < r.Z+0.7:
		act.Boost = true
	case !over:
		// above the edge: hold height while we drift over the roof
		act.Boost = b.vel[2] < 0.5
	}
	// give up when it is not working
	budget := b.roof.acc.Climb/8.0 + 5
	if b.t-b.roof.t0 > budget || (b.energy < 2 && !grounded && feet < r.Z-0.5) {
		b.roof.tries++
		if b.roof.ban == nil {
			b.roof.ban = map[int]float64{}
		}
		b.roof.ban[b.roof.id] = b.t + 90
		b.roof.state = roofNone
	}
}

// overland travels to a goal over the top surface of the map: walking, dropping off edges and
// jetting up walls, along a route from Nav.SurfacePath. It is how we move wherever the walk grid
// does not apply (leaving a roof, out of a walled courtyard, into a circle that closes inside the
// stadium or on top of a building). It writes the action directly.
func (b *Bot) overland(act *proto.Action, gx, gy float64) {
	tops := b.Nav.Tops()
	feet := b.pos[2]
	grounded := b.o.Self.Grounded
	here := b.Nav.Index(b.pos[0], b.pos[1])
	if b.surfBan == nil {
		b.surfBan = map[int]float64{}
	}
	// watchdog: pushing along the route without getting anywhere means it runs through something
	// the surface map does not show (a gap narrower than we are, an overhang): rule those cells out
	if b.t-b.surfWatchT >= 1.5 {
		if b.surfPushing && dist3(b.pos, b.surfWatchPos) < 1.2 && len(b.surf) >= 2 {
			for j := b.surfIdx + 1; j < len(b.surf) && j <= b.surfIdx+3; j++ {
				b.surfBan[int(b.surf[j])] = b.t + 25
			}
			b.surf = nil
			b.surfT = -100
		}
		b.surfWatchT, b.surfWatchPos, b.surfPushing = b.t, b.pos, false
	}
	// (re)plan about once a second, when the goal moves, or when we are off the route
	replan := len(b.surf) < 2 || hyp(gx-b.surfGoal[0], gy-b.surfGoal[1]) > 6 || b.t-b.surfT > 1.2
	if !replan {
		px, py := b.Nav.Pos(int(b.surf[b.surfIdx]))
		if hyp(px-b.pos[0], py-b.pos[1]) > 7 {
			replan = true
		}
	}
	if replan && b.t-b.surfT >= 0.3 {
		blocked := func(k int) bool {
			until, ok := b.surfBan[k]
			return ok && b.t < until
		}
		b.surf = b.Nav.SurfacePath(b.pos[0], b.pos[1], feet, gx, gy, b.maxClimb(), b.radius, 14000, blocked)
		b.surfGoal = [2]float64{gx, gy}
		b.surfT = b.t
		b.surfIdx = 0
	}
	dx, dy := norm2(gx-b.pos[0], gy-b.pos[1])
	need := feet
	if len(b.surf) >= 2 {
		// where are we along it
		best, bd := b.surfIdx, 1e18
		for j := b.surfIdx; j < len(b.surf) && j < b.surfIdx+8; j++ {
			px, py := b.Nav.Pos(int(b.surf[j]))
			if d := hyp(px-b.pos[0], py-b.pos[1]); d < bd {
				best, bd = j, d
			}
		}
		b.surfIdx = best
		n1 := best + 1
		if n1 > len(b.surf)-1 {
			n1 = len(b.surf) - 1
		}
		tgt := n1
		// look a little further ahead while the way is level (smoother walking)
		for tgt < len(b.surf)-1 && tgt < best+3 && math.Abs(float64(tops[b.surf[tgt+1]])-float64(tops[b.surf[n1]])) < 0.6 && float64(tops[b.surf[n1]]) <= feet+1.0 {
			tgt++
		}
		tx, ty := b.Nav.Pos(int(b.surf[tgt]))
		if int(b.surf[tgt]) != here {
			dx, dy = norm2(tx-b.pos[0], ty-b.pos[1])
		}
		need = float64(tops[b.surf[n1]])
		if h := float64(tops[b.surf[tgt]]); h > need {
			need = h
		}
	} else {
		// no route: straight at it, climbing what is directly ahead
		for _, s := range [...]float64{b.radius + 1, b.radius + 3.5} {
			if h, _ := b.W.Floor(b.pos[0]+dx*s, b.pos[1]+dy*s, 1e9); h > need {
				need = h
			}
		}
	}
	act.Move = [2]float64{dx, dy}
	rise := need - feet
	drain := stat(b.o.Self.Stats, "jet_drain", b.mech("jet_drain", 15))
	switch {
	case rise < 1.0:
		// level or down: walk; run only with energy to spare (climbs come first)
		act.Boost = grounded && !b.o.Self.InZone && b.energy > 70
		b.surfPushing = true
	case grounded:
		// a wall: jump once we have the energy to clear it, otherwise stand and recharge
		if b.energy >= math.Min(rise/8.9*drain+16, b.o.Self.MaxEnergy-3) {
			act.Jump = true
			b.surfPushing = true
		} else {
			act.Move = [2]float64{0, 0}
		}
	default:
		act.Boost = feet < need+0.8 && b.energy > 1
	}
}

// mainComp returns the id of the largest walkable region (the open map).
func (b *Bot) mainComp() int32 {
	if b.mainCompID != 0 {
		return b.mainCompID
	}
	comp := b.Nav.Components()
	counts := map[int32]int{}
	for _, c := range comp {
		if c != 0 {
			counts[c]++
		}
	}
	best, bn := int32(0), 0
	for c, n := range counts {
		if n > bn {
			best, bn = c, n
		}
	}
	b.mainCompID = best
	return best
}

// roofWanted reports whether hiding on a roof is what we want right now.
func (b *Bot) roofWanted(st stance, zi zoneInfo) bool {
	if !b.P.UseRoofs || zi.final || b.o.Zone.NextRadius < b.P.RoofMinZone {
		return false
	}
	if st.name == "duel" {
		return false
	}
	if !st.stealth && !b.P.RoofInSquad {
		// the brawl stance in a known free-for-all: once armed for it (a laser, or some cubes, or
		// simply later in the match) wait on a roof instead of on the ground. In the real matches our
		// mech took 9 damage a second on roofs against 32 anywhere else.
		if !(b.P.BrawlRoofs && st.avoid && st.name == "ffa") {
			return false
		}
		if b.has("laser") < 0 && b.o.Self.Cubes < b.P.BrawlRoofCubes && b.t < b.P.BrawlRoofAfter {
			return false
		}
	}
	return true
}

// roofPlanStep runs the roof plan when roofs are wanted; ok=false means "use the ground hide instead".
func (b *Bot) roofPlanStep(wanted bool, act *proto.Action) (gx, gy float64, settled, ok bool) {
	if !wanted {
		b.roof.state = roofNone
		return 0, 0, false, false
	}
	return b.roofHold(act)
}

// hideoutFor is the policy's entry to the hideout chooser: stealth stances only, and it leaves the
// roof plan pointed at the chosen roof (or free to choose by the older rule when the map is not used).
func (b *Bot) hideoutFor(st stance, zi zoneInfo, allowRoof bool) (hideout, bool) {
	if b.hm == nil || !(st.stealth || (b.P.HoldHideMap && st.avoid)) {
		b.roof.want, b.roof.spot = -1, -1
		return hideout{}, false
	}
	if !st.stealth {
		allowRoof = false // the brawl stance waits on the ground
	}
	ho, ok := b.hideoutStep(zi, allowRoof)
	if !ok {
		b.roof.want, b.roof.spot = -1, -1
		return ho, false
	}
	if ho.class == hmRoof {
		b.roof.want, b.roof.spot = ho.roof, ho.cell
	}
	return ho, true
}

// hideoutStep returns the hideout to use right now (re-evaluated every couple of seconds, at once
// when the current one falls outside the circle we must end up in).
func (b *Bot) hideoutStep(zi zoneInfo, allowRoof bool) (hideout, bool) {
	if b.hm == nil {
		return hideout{}, false
	}
	stale := b.t-b.hideOutT > 2.0 || !b.hideOutOK
	if b.hideOutOK && !stale {
		z := &b.o.Zone
		x, y := b.Nav.Pos(b.hideOut.cell)
		if hyp(x-z.NextCenter[0], y-z.NextCenter[1]) > z.NextRadius-4 || (b.hideOut.class == hmRoof && !allowRoof) {
			stale = true
		}
	}
	if stale {
		b.hideOut, b.hideOutOK = b.chooseHideout(zi, allowRoof)
		b.hideOutT = b.t
		if b.hideOutOK {
			b.hideCell = b.hideOut.cell
		} else {
			b.hideCell = -1
		}
	}
	return b.hideOut, b.hideOutOK
}

// roofHold is the "nothing else to do" behaviour when roofs are in play. It returns the goal to walk
// to and whether we are settled (stand still). ok=false means no roof plan: fall back to ground hiding.
func (b *Bot) roofHold(act *proto.Action) (gx, gy float64, settled, ok bool) {
	roofs, _ := b.Nav.Roofs()
	if b.t-b.roof.stepT > 1.5 {
		b.roof.t0 = b.t // resuming after doing something else: the approach clock starts again
	}
	b.roof.stepT = b.t
	if id := b.onRoof(); id >= 0 && b.o.Self.Grounded {
		r := &roofs[id]
		stay := b.roofSpotOK(r) && r.Height >= b.P.RoofMinHeight-3
		if b.roof.want >= 0 {
			// the hideout chooser decides which roof: stay only if this is the one it wants
			stay = id == b.roof.want && b.roofSpotOK(r)
		}
		if stay && b.P.RoofStandCheck {
			// the place we would actually stand at (not just the roof's deepest cell) must be inside
			// the next circle, or the zone rule and this one pull us to and fro along the roof
			z := &b.o.Zone
			sx, sy := b.roofSpot(id)
			if hyp(sx-z.NextCenter[0], sy-z.NextCenter[1]) > z.NextRadius-4 {
				stay = false
				if b.roof.ban == nil {
					b.roof.ban = map[int]float64{}
				}
				b.roof.ban[id] = b.t + 60
			}
		}
		if !stay && b.holdRoofMove {
			stay = true // someone is watching and the zone is not here yet: do not climb down in front of them
		}
		if stay {
			b.roof.state, b.roof.id = roofOn, id
			x, y := b.roofSpot(id)
			return x, y, hyp(x-b.pos[0], y-b.pos[1]) < 2.0, true
		}
		// this roof is about to be outside the zone (or is not the one we want): plan the next one
		b.roof.state = roofNone
	}
	if b.roof.state == roofOn {
		b.roof.state = roofNone // we are not on it any more (fell or walked off)
	}
	if b.roof.state == roofNone || b.t-b.roof.chosen > 3 {
		id, acc, found := b.chooseRoof()
		b.roof.chosen = b.t
		if !found {
			b.roof.state = roofNone
			return 0, 0, false, false
		}
		if b.roof.state == roofNone || id != b.roof.id {
			b.roof.state, b.roof.id, b.roof.acc, b.roof.t0 = roofApproach, id, acc, b.t
		}
	}
	gx, gy = b.Nav.Pos(b.roof.acc.Ground)
	d := hyp(gx-b.pos[0], gy-b.pos[1])
	if d < 1.6 && b.onGroundLevel() {
		need := b.roof.acc.Climb/8.9*stat(b.o.Self.Stats, "jet_drain", 15) + 24
		if b.holdClimb {
			// somebody is about: no jetting up a wall in front of them. If that lasts, this roof is
			// the wrong one for now (the ground rules take over until another is chosen).
			if b.roof.waitT0 <= 0 {
				b.roof.waitT0 = b.t
			}
			if b.t-b.roof.waitT0 > 4 {
				if b.roof.ban == nil {
					b.roof.ban = map[int]float64{}
				}
				b.roof.ban[b.roof.id] = b.t + 20
				b.roof.state = roofNone
				b.roof.waitT0 = 0
				return 0, 0, false, false
			}
			return gx, gy, true, true
		}
		b.roof.waitT0 = 0
		if b.energy >= need {
			b.roof.state = roofClimb
			b.roof.t0 = b.t
		}
		return gx, gy, true, true
	}
	if b.t-b.roof.t0 > 60 {
		// could not get there
		if b.roof.ban == nil {
			b.roof.ban = map[int]float64{}
		}
		b.roof.ban[b.roof.id] = b.t + 60
		b.roof.state = roofNone
		return 0, 0, false, false
	}
	return gx, gy, false, true
}
