package bot

import "math"

// fightPlan evaluates fighting a mech at several ranges and returns the best advantage ratio together
// with the range band to hold. The ratio is (time they need to kill us) / (time we need to kill
// them), after paying for the damage we would take while moving to that range.
func (b *Bot) fightPlan(tr *Track, d float64) (adv, rmin, rmax float64) {
	hp := tr.HP
	if hp <= 0 {
		hp = math.Max(tr.MaxHP, 1)
	}
	type band struct{ r float64 }
	cands := [...]float64{22, 45, 75, 115, 160, 200}
	bestAdv, bestR := -1.0, d
	eval := func(r float64, here bool) {
		mine := b.myDPS(r)
		theirs := b.theirDPS(tr, r) * (1 - b.armor) * b.dodgeFactor(r, tr)
		if mine < 1 {
			return
		}
		// damage taken (and, in the newer accounting, dealt) while changing range
		move := math.Abs(d - r)
		cost, gain := 0.0, 0.0
		if !here && move > 8 {
			mid := (d + r) / 2
			// closing is slower than their backing away is easy: assume 9 m/s net
			cost = b.theirDPS(tr, mid) * (1 - b.armor) * b.dodgeFactor(mid, tr) * (move / 9.0) * 0.8
			if b.P.PlanV2 && b.realModel() {
				// the torso keeps shooting while the legs change the range
				gain = b.myDPS(mid) * (move / 9.0) * 0.8
			}
			if r > d && !tr.NPC {
				// we cannot hold a longer range against something that wants to close; discount it
				cost += 60
			}
		}
		left := b.hp - cost
		if left <= 0 {
			return
		}
		hpT := hp - gain
		if hpT < 20 {
			hpT = 20
		}
		a := 10.0
		if theirs >= 1 {
			a = (left / theirs) / (hpT / mine)
		}
		if here {
			a *= 1.05 // staying put is free
		}
		if a > bestAdv {
			bestAdv, bestR = a, r
		}
	}
	eval(d, true)
	for _, r := range cands {
		eval(r, false)
	}
	if bestAdv < 0 {
		return 0, 30, 55
	}
	w := 10 + bestR*0.15
	return bestAdv, math.Max(8, bestR-w), bestR + w
}

// dodgeFactor scales an enemy's expected damage on us at a range: far-off gunfire mostly misses a
// target that side-steps, lasers and lightning do not care.
func (b *Bot) dodgeFactor(r float64, tr *Track) float64 {
	if tr.NPC {
		return 0.6
	}
	// opponents' range falloff is already in theirGunHitProb
	return 1
}

// --- exploration ---------------------------------------------------------------------------------

const seenCell = 32.0

// markSeen stamps the coarse cells we can currently see.
func (b *Bot) markSeen() {
	if b.seenT == nil {
		b.seenNX = int(math.Ceil((b.W.MaxX-b.W.MinX)/seenCell)) + 1
		b.seenNY = int(math.Ceil((b.W.MaxY-b.W.MinY)/seenCell)) + 1
		b.seenT = make([]float32, b.seenNX*b.seenNY)
		for i := range b.seenT {
			b.seenT[i] = -1e6
		}
	}
	const R = 150.0
	half := b.C.Vision.FovDeg * math.Pi / 360
	if half <= 0 {
		half = math.Pi / 3
	}
	ex, ey, ez := b.pos[0], b.pos[1], b.pos[2]+b.eyeH
	i0 := int((ex - R - b.W.MinX) / seenCell)
	i1 := int((ex + R - b.W.MinX) / seenCell)
	j0 := int((ey - R - b.W.MinY) / seenCell)
	j1 := int((ey + R - b.W.MinY) / seenCell)
	for j := j0; j <= j1; j++ {
		if j < 0 || j >= b.seenNY {
			continue
		}
		for i := i0; i <= i1; i++ {
			if i < 0 || i >= b.seenNX {
				continue
			}
			k := j*b.seenNX + i
			if float64(b.seenT[k]) > b.t-2.0 {
				continue
			}
			cx := b.W.MinX + (float64(i)+0.5)*seenCell
			cy := b.W.MinY + (float64(j)+0.5)*seenCell
			d := hyp(cx-ex, cy-ey)
			if d > R {
				continue
			}
			if d > 40 && math.Abs(angDiff(math.Atan2(cy-ey, cx-ex), b.aim[0])) > half {
				continue
			}
			gz := b.W.TerrainZ(cx, cy) + 2
			if d < 20 || b.W.Visible(ex, ey, ez, cx, cy, gz) {
				b.seenT[k] = float32(b.t)
			}
		}
	}
}

// roamGoal picks a ground position worth looking at: not seen for a while, inside the circle, close.
func (b *Bot) roamGoal() (float64, float64, bool) {
	if b.seenT == nil {
		return 0, 0, false
	}
	if b.roamCell >= 0 && b.t-b.roamT < 12 && float64(b.seenT[b.roamCell]) < b.t-20 {
		i, j := b.roamCell%b.seenNX, b.roamCell/b.seenNX
		x, y := b.W.MinX+(float64(i)+0.5)*seenCell, b.W.MinY+(float64(j)+0.5)*seenCell
		if c := b.Nav.Nearest(x, y, b.radius+0.3, b.height, 8); c >= 0 {
			px, py := b.Nav.Pos(c)
			return px, py, true
		}
	}
	z := &b.o.Zone
	best, bs := -1, 1e18
	for j := 0; j < b.seenNY; j++ {
		for i := 0; i < b.seenNX; i++ {
			k := j*b.seenNX + i
			age := b.t - float64(b.seenT[k])
			if age < 45 {
				continue
			}
			x := b.W.MinX + (float64(i)+0.5)*seenCell
			y := b.W.MinY + (float64(j)+0.5)*seenCell
			if !b.W.InBounds(x, y) {
				continue
			}
			if hyp(x-z.NextCenter[0], y-z.NextCenter[1]) > z.NextRadius-15 {
				continue
			}
			nk := b.Nav.Index(x, y)
			if b.Nav.Solid[nk] {
				continue // inside a building: nothing to pick up on the ground there
			}
			if until, banned := b.banUntil[-(k + 1)]; banned && b.t < until {
				continue
			}
			if !b.reachable(x, y) {
				b.seenT[k] = float32(b.t) // cut off from us: stop considering it
				continue
			}
			d := hyp(x-b.pos[0], y-b.pos[1])
			s := d - math.Min(age, 300)*0.15
			if s < bs {
				best, bs = k, s
			}
		}
	}
	if best < 0 {
		return 0, 0, false
	}
	b.roamCell, b.roamT = best, b.t
	i, j := best%b.seenNX, best/b.seenNX
	x, y := b.W.MinX+(float64(i)+0.5)*seenCell, b.W.MinY+(float64(j)+0.5)*seenCell
	if c := b.Nav.Nearest(x, y, b.radius+0.3, b.height, 8); c >= 0 {
		px, py := b.Nav.Pos(c)
		return px, py, true
	}
	// unreachable cell: do not pick it again for a while
	b.seenT[best] = float32(b.t)
	return 0, 0, false
}
