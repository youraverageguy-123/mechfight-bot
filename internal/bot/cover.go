package bot

// Cover that lasts.
//
// findCover returns the nearest cell out of a threat's sight, which by construction lies on the very edge
// of the shadow: in the real matches our mech, having got out of an attacker's sight, was seen again
// within 10 s in 79% of cases (the most of any team), typically after a second or three in which the
// attacker had moved two or three metres and we had not moved at all. The best survivor on the board
// was seen again half as often: it keeps walking. deeperCover is the second step: from the edge of the
// shadow to a place that stays hidden when the attacker shifts sideways, and, with a hurt mech and the
// attacker close, a place clearly farther from it as well (health comes back just as fast on the move).

// stuckOn reports whether we have been working on the same crate or drone for more than limit seconds
// without taking 20 health off it. It keeps the bookkeeping itself: call it every decision spent on
// the thing, with its current health.
func (b *Bot) stuckOn(id int, hp, limit float64) bool {
	if id != b.crateTryID || hp < b.crateTryHP-20 || b.t-b.crateSeenT > 1.5 {
		b.crateTryID, b.crateTryT, b.crateTryHP = id, b.t, hp
	}
	b.crateSeenT = b.t
	if b.t-b.crateTryT > limit {
		b.crateTryID = -1
		return true
	}
	return false
}

// shyPhase reports whether the opening rule applies: the first ShySecs of a match in a free-for-all
// stance (see think for what it does and why).
func (b *Bot) shyPhase(st stance) bool {
	// (only once we know it is a free-for-all: against the practice squad it costs kills; ShyOpen
	// lets an opening set try it while the lobby is not known yet)
	return b.P.ShySecs > 0 && (b.ffa || (b.P.ShyOpen && st.name == "open")) && st.avoid && b.t < b.P.ShySecs
}

// exposedTo reports whether a mech we saw in the last secs seconds, no farther than maxRange from us,
// has a clear line from where we last saw it to where we stand. (We only see 120 degrees ahead: a mech
// that has dropped out of our view has not lost sight of us.)
func (b *Bot) exposedTo(secs, maxRange float64) bool {
	for _, tr := range b.tracks {
		if tr.NPC || b.t-tr.LastSeen > secs {
			continue
		}
		if hyp(tr.Pos[0]-b.pos[0], tr.Pos[1]-b.pos[1]) > maxRange {
			continue
		}
		sc := tr.scale(b.sizePerCube)
		ex, ey, ez := tr.Pos[0], tr.Pos[1], tr.Pos[2]+8*sc
		if b.W.Visible(ex, ey, ez, b.pos[0], b.pos[1], b.pos[2]+b.height*0.5) || b.W.Visible(ex, ey, ez, b.pos[0], b.pos[1], b.pos[2]+b.height*0.95) {
			return true
		}
	}
	return false
}

// mechSeenWithin reports whether we have seen another mech in the last secs seconds.
func (b *Bot) mechSeenWithin(secs float64) bool {
	for _, tr := range b.tracks {
		if !tr.NPC && b.t-tr.LastSeen <= secs {
			return true
		}
	}
	return false
}

// coverClose reports whether there is somewhere out of every threat's sight within a few seconds' run:
// RetreatRunMax metres of path, or RetreatRunLaser when a laser we cannot answer is on us (against that,
// in the open, nothing else works). The answer is kept for half a second.
func (b *Bot) coverClose(ths []threat, tgt *threat) bool {
	if b.t-b.nearCoverT < 0.5 {
		return b.nearCoverOK
	}
	reach := b.P.RetreatRunMax
	if tgt != nil && tgt.tr.Laser && b.has("laser") < 0 && b.P.RetreatRunLaser > reach {
		reach = b.P.RetreatRunLaser
	}
	b.nearCoverT = b.t
	b.nearCoverOK = b.findCover(ths, reach) >= 0
	return b.nearCoverOK
}

// deeperCover returns the nearest cell that is hidden from every recent threat's last known position and
// from CoverMargin metres to either side of it, with the straight way there out of sight too. With
// farther set and the nearest threat inside RunOnRange, the cell must also be RunOnGain metres farther
// from that threat than we are. It returns -1 when where we stand already qualifies, or nothing does.
func (b *Bot) deeperCover(ths []threat, maxCost float64, farther bool) int {
	type eyeT struct{ x, y, z float64 }
	var eyes []eyeT
	d0 := 1e9
	for i := range ths {
		tr := ths[i].tr
		if (b.t-tr.LastSeen > 6 && !ths[i].phantom) || ths[i].d > 260 {
			continue
		}
		sc := tr.scale(b.sizePerCube)
		eyes = append(eyes, eyeT{tr.Pos[0], tr.Pos[1], tr.Pos[2] + 8*sc})
		if d := hyp(tr.Pos[0]-b.pos[0], tr.Pos[1]-b.pos[1]); d < d0 {
			d0 = d
		}
	}
	if len(eyes) == 0 {
		return -1
	}
	needFar := farther && b.P.RunOnRange > 0 && d0 <= b.P.RunOnRange
	margin := b.P.CoverMargin
	if margin <= 0 && !needFar {
		return -1
	}
	seen := func(ex, ey, ez, x, y, gz float64) bool {
		return b.W.Visible(ex, ey, ez, x, y, gz+b.height*0.5) || b.W.Visible(ex, ey, ez, x, y, gz+b.height*0.95)
	}
	hidden := func(x, y, gz float64) bool {
		for _, e := range eyes {
			if seen(e.x, e.y, e.z, x, y, gz) {
				return false
			}
		}
		return true
	}
	// hidden with room to spare: also from where the threat would be after a step to either side
	robust := func(x, y, gz float64) bool {
		for _, e := range eyes {
			if seen(e.x, e.y, e.z, x, y, gz) {
				return false
			}
			if margin <= 0 {
				continue
			}
			dx, dy := x-e.x, y-e.y
			l := hyp(dx, dy)
			if l < 1 {
				return false
			}
			px, py := -dy/l*margin, dx/l*margin
			if seen(e.x+px, e.y+py, e.z, x, y, gz) || seen(e.x-px, e.y-py, e.z, x, y, gz) {
				return false
			}
		}
		return true
	}
	if !needFar && robust(b.pos[0], b.pos[1], b.pos[2]) {
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
		if needFar {
			de := 1e9
			for _, e := range eyes {
				if d := hyp(x-e.x, y-e.y); d < de {
					de = d
				}
			}
			if de < d0+b.P.RunOnGain {
				return true
			}
		}
		checked++
		if checked > 300 {
			return false
		}
		if !robust(x, y, float64(b.Nav.Z[k])) {
			return true
		}
		// the straight way there: walkable points a quarter, half and three quarters along must be hidden too
		for _, f := range [3]float64{0.25, 0.5, 0.75} {
			mx, my := b.pos[0]+(x-b.pos[0])*f, b.pos[1]+(y-b.pos[1])*f
			mk := b.Nav.Index(mx, my)
			if b.Nav.Solid[mk] {
				continue // the walk goes round this
			}
			if !hidden(mx, my, float64(b.Nav.Z[mk])) {
				return true
			}
		}
		found = k
		return false
	})
	return found
}
