package bot

import (
	"math"

	"mechroyale/internal/proto"
)

type cand struct {
	dx, dy float64
	boost  bool
	score  float64
}

// incoming is an enemy projectile we may have to dodge.
type incoming struct {
	mortar   bool
	pos, vel [3]float64
	maxT     float64 // seconds until it stops (wall, ground, or explosion)
	impact   [3]float64
	dmg      float64
	splash   float64
}

// gatherIncoming lists the visible projectiles that could matter to us in the next couple of seconds.
func (b *Bot) gatherIncoming() []incoming {
	var out []incoming
	g := stat(b.C.Weapons["mortar"], "gravity", 22)
	for i := range b.o.Projectiles {
		p := &b.o.Projectiles[i]
		sp := math.Sqrt(p.Vel[0]*p.Vel[0] + p.Vel[1]*p.Vel[1] + p.Vel[2]*p.Vel[2])
		if sp < 1 {
			continue
		}
		if p.Kind == "mortar" {
			// integrate the arc to its impact
			x, y, z := p.Pos[0], p.Pos[1], p.Pos[2]
			vx, vy, vz := p.Vel[0], p.Vel[1], p.Vel[2]
			tt := 0.0
			const h = 0.05
			hit := false
			for tt < 7 {
				nx, ny, nz := x+vx*h, y+vy*h, z+vz*h-0.5*g*h*h
				if !b.W.Visible(x, y, z, nx, ny, nz) {
					hit = true
					break
				}
				x, y, z = nx, ny, nz
				vz -= g * h
				tt += h
			}
			if !hit {
				continue
			}
			splash := stat(b.C.Weapons["mortar"], "splash_radius", 7) * 1.3
			if dist3([3]float64{x, y, z}, b.pos) > splash+b.radius+45 {
				continue
			}
			out = append(out, incoming{mortar: true, pos: p.Pos, vel: p.Vel, maxT: tt + h, impact: [3]float64{x, y, z},
				dmg: stat(b.C.Weapons["mortar"], "splash", 160), splash: splash})
			continue
		}
		// straight shots: ignore those not coming our way
		rx, ry := b.pos[0]-p.Pos[0], b.pos[1]-p.Pos[1]
		along := (rx*p.Vel[0] + ry*p.Vel[1]) / sp
		if along < -3 {
			continue
		}
		d := hyp(rx, ry)
		if d > 330 {
			continue
		}
		// lateral miss distance if nobody moves
		lat := math.Abs(rx*p.Vel[1]-ry*p.Vel[0]) / math.Max(hyp(p.Vel[0], p.Vel[1]), 1)
		if lat > 14+b.radius {
			continue
		}
		reach := b.W.Raycast(p.Pos[0], p.Pos[1], p.Pos[2], p.Vel[0]/sp, p.Vel[1]/sp, p.Vel[2]/sp, d+20)
		if reach < d-b.radius-3 {
			continue // hits a wall first
		}
		dmg := stat(b.C.Weapons["gun"], "damage", 45)
		if p.Kind == "npc" {
			dmg = stat(b.C.Npc, "damage", 25)
		}
		out = append(out, incoming{pos: p.Pos, vel: p.Vel, maxT: reach / sp, dmg: dmg})
	}
	return out
}

// steerReq is what the strategy layer asks the movement planner for.
type steerReq struct {
	wantX, wantY float64                         // desired direction (unit or zero)
	wantW        float64                         // value of one metre of progress along it
	boost        bool                            // boosting is desired (not just allowed for dodging)
	slow         bool                            // we move at charge speed (laser charging)
	posCost      func(x, y float64) float64      // extra cost of ending up at a position (may be nil)
	hold         bool                            // prefer standing still when nothing threatens
	threats      []incoming
	lookTicks    int  // evaluate position costs this many ticks ahead (0: default)
	dodgeBoost   bool // boosting is worth considering even without projectiles
	fenceRoof    bool // we are on a roof and mean to stay on it: do not step off the edge
	zoneFree     bool // we are following a path that takes care of the zone itself
}

var dirs16 = func() [16][2]float64 {
	var d [16][2]float64
	for i := 0; i < 16; i++ {
		a := float64(i) * math.Pi / 8
		d[i] = [2]float64{math.Cos(a), math.Sin(a)}
	}
	return d
}()

// steer evaluates candidate moves by simulating our motion against incoming fire and picks the best.
func (b *Bot) steer(act *proto.Action, rq steerReq) {
	grounded := b.o.Self.Grounded
	accel := b.mech("ground_accel", 58.5)
	if !grounded {
		accel = b.mech("air_accel", 12)
	}
	walk := b.walk
	if rq.slow {
		walk *= b.mech("charge_move_mult", 0.5)
	}
	boostV := b.boostV
	if rq.slow {
		boostV *= b.mech("charge_move_mult", 0.5)
	}
	canBoost := b.energy > 12 && grounded
	horizon := 8
	for i := range rq.threats {
		k := int(rq.threats[i].maxT/b.dt) + 2
		if k > horizon {
			horizon = k
		}
	}
	if horizon > 44 {
		horizon = 44
	}
	progTick := 6
	if rq.lookTicks > progTick {
		progTick = rq.lookTicks
		if horizon < progTick {
			horizon = progTick
		}
	}

	cands := b.scratch[:0]
	cands = append(cands, cand{0, 0, false, 0})
	for i := 0; i < 16; i++ {
		cands = append(cands, cand{dirs16[i][0], dirs16[i][1], false, 0})
	}
	if rq.wantX != 0 || rq.wantY != 0 {
		cands = append(cands, cand{rq.wantX, rq.wantY, false, 0})
	}
	if canBoost && (rq.boost || rq.dodgeBoost || len(rq.threats) > 0) {
		n := len(cands)
		for i := 1; i < n; i++ {
			cands = append(cands, cand{cands[i].dx, cands[i].dy, true, 0})
		}
	}
	zc, zr := b.o.Zone.Center, b.o.Zone.Radius
	fence := -1
	var roofOf []int32
	var roofDist []int16
	// A cell is only a wall for us if stepping into it makes things worse than where we stand now:
	// pressed against a wall (or on a roof's outer cells) our own cell already fails the test, and
	// treating it as blocked would freeze every candidate.
	startCell := b.Nav.Index(b.pos[0], b.pos[1])
	startClear := b.Nav.Clear[startCell]
	groundLevel := b.onGroundLevel()
	startRD := int16(0)
	if rq.fenceRoof {
		fence = b.onRoof()
		_, roofOf = b.Nav.Roofs()
		roofDist = b.Nav.RoofDist()
		startRD = roofDist[startCell]
	}
	minClear := float32(b.radius * 0.8)
	best := -1
	for ci := range cands {
		c := &cands[ci]
		sp := walk
		if c.boost {
			sp = boostV
		}
		tvx, tvy := c.dx*sp, c.dy*sp
		x, y := b.pos[0], b.pos[1]
		vx, vy := b.vel[0], b.vel[1]
		score := 0.0
		blocked := false
		var px, py float64 // position at the progress tick
		for k := 1; k <= horizon; k++ {
			vx, vy = approach2(vx, vy, tvx, tvy, accel*b.dt)
			nx, ny := x+vx*b.dt, y+vy*b.dt
			cell := b.Nav.Index(nx, ny)
			if cell != startCell {
				if fence >= 0 {
					// stay on our roof, and off its outermost cells (unless we already stand on one:
					// then anything that is not further out is fine)
					if int(roofOf[cell]) != fence || (roofDist[cell] < 2 && roofDist[cell] < startRD) {
						blocked = true
						vx, vy = 0, 0
						nx, ny = x, y
					}
				} else if groundLevel && (b.Nav.Solid[cell] || !b.W.InBounds(nx, ny) || (b.Nav.Clear[cell] < minClear && b.Nav.Clear[cell] < startClear)) {
					blocked = true
					vx, vy = 0, 0
					nx, ny = x, y
				}
			}
			x, y = nx, ny
			if k == progTick {
				px, py = x, y
			}
			t0 := float64(k-1) * b.dt
			t1 := float64(k) * b.dt
			for ti := range rq.threats {
				th := &rq.threats[ti]
				if th.mortar {
					if th.maxT > t0 && th.maxT <= t1+1e-9 || (k == horizon && th.maxT > t1) {
						d := math.Sqrt((x-th.impact[0])*(x-th.impact[0]) + (y-th.impact[1])*(y-th.impact[1]) + (b.pos[2]+b.height*0.5-th.impact[2])*(b.pos[2]+b.height*0.5-th.impact[2]))
						reach := th.splash + b.radius
						if d < reach {
							score -= th.dmg*(1-0.7*d/reach) + 40
						}
					}
					continue
				}
				if t0 >= th.maxT {
					continue
				}
				te := math.Min(t1, th.maxT)
				ax, ay := th.pos[0]+th.vel[0]*t0, th.pos[1]+th.vel[1]*t0
				bx, by := th.pos[0]+th.vel[0]*te, th.pos[1]+th.vel[1]*te
				d, s := segPointDist2D(x, y, ax, ay, bx, by)
				if d < b.radius+0.3+0.55 {
					tz := th.pos[2] + th.vel[2]*(t0+(te-t0)*s)
					if tz > b.pos[2]-0.6 && tz < b.pos[2]+b.height+0.6 {
						score -= th.dmg
					}
				}
			}
		}
		if horizon < progTick {
			px, py = x, y
		}
		// progress toward the goal
		score += ((px-b.pos[0])*rq.wantX + (py-b.pos[1])*rq.wantY) * rq.wantW
		if blocked {
			score -= 3
		}
		// do not side-step out of the zone. Only while we are inside it: once outside, the way back
		// in may well start by going around something, and a penalty here would pin us in place.
		if zr > 0 && !rq.zoneFree {
			dOut := hyp(px-zc[0], py-zc[1]) - zr
			dNow := hyp(b.pos[0]-zc[0], b.pos[1]-zc[1]) - zr
			if dNow < -0.5 && dOut > -4 && dOut > dNow {
				score -= (dOut - dNow) * 8
			}
		}
		if rq.posCost != nil {
			score -= rq.posCost(px, py)
		}
		if c.boost && !rq.boost {
			score -= 1.5 // energy is not free
		}
		if c.dx == 0 && c.dy == 0 {
			if rq.hold {
				score += 0.6
			}
		} else {
			// mild preference for keeping our heading (less jitter)
			score += 0.25 * (c.dx*b.lastMove[0] + c.dy*b.lastMove[1])
		}
		c.score = score
		if best < 0 || score > cands[best].score {
			best = ci
		}
	}
	b.scratch = cands
	if best >= 0 {
		act.Move = [2]float64{cands[best].dx, cands[best].dy}
		act.Boost = cands[best].boost
	}
}

// onGroundLevel reports whether our feet are near the walk-grid surface (not on a roof or in the air).
func (b *Bot) onGroundLevel() bool {
	k := b.Nav.Index(b.pos[0], b.pos[1])
	if b.Nav.Solid[k] {
		return false
	}
	return b.pos[2]-float64(b.Nav.Z[k]) < 3.5
}

// navDir returns the direction to walk now to reach (gx,gy) along the ground, and the remaining
// straight-line distance. ok=false when we have no idea how to get there.
func (b *Bot) navDir(gx, gy float64) (float64, float64, float64, bool) {
	d := hyp(gx-b.pos[0], gy-b.pos[1])
	if d < 1.0 {
		return 0, 0, d, true
	}
	if !b.onGroundLevel() {
		// on a roof or airborne: head straight for the goal and drop off the edge
		dx, dy := norm2(gx-b.pos[0], gy-b.pos[1])
		return dx, dy, d, true
	}
	r, h := b.radius+0.15, b.height
	if d < 14 && b.Nav.WalkLine(b.pos[0], b.pos[1], gx, gy, r, h) {
		dx, dy := norm2(gx-b.pos[0], gy-b.pos[1])
		return dx, dy, d, true
	}
	// the path we hold is for somewhere else (or there is none yet)
	other := !b.pathSet || hyp(gx-b.pathGoal[0], gy-b.pathGoal[1]) > 5
	replan := len(b.path) == 0 || other || b.t-b.pathT > 1.6
	if !replan && b.pathIdx < len(b.path) {
		px, py := b.Nav.Pos(int(b.path[b.pathIdx]))
		if hyp(px-b.pos[0], py-b.pos[1]) > 12 {
			replan = true
		}
	}
	// path searches are the most expensive thing we do: never more than about three a second, and
	// a search that failed or fell short is not repeated for a second. A new goal may search sooner
	// (but not every decision, in case two goals alternate).
	gap := 0.35
	if b.pathBad {
		gap = 1.0
	}
	if other && b.P.PathFix {
		gap = 0.15
	}
	if replan && (b.pathSet || !b.P.PathFix) && b.t-b.pathT < gap {
		replan = false
	}
	if other && !replan && b.P.PathFix {
		// No search allowed for this goal yet: head straight for it for a moment. It must not be
		// reported as unreachable on the strength of a path (or a failure) that was for another place:
		// the caller would cross it off for 40 seconds, and at the first tick of a match that used to
		// cost us the best weapon near our spawn.
		dx, dy := norm2(gx-b.pos[0], gy-b.pos[1])
		return dx, dy, d, true
	}
	if replan {
		b.pathSet = true
		b.path = b.Nav.Path(b.pos[0], b.pos[1], gx, gy, r, h, 9000, b.navCost())
		b.pathGoal = [2]float64{gx, gy}
		b.pathT = b.t
		b.pathIdx = 0
		b.pathBad = len(b.path) == 0
		if !b.pathBad {
			lx, ly := b.Nav.Pos(int(b.path[len(b.path)-1]))
			b.pathBad = hyp(lx-gx, ly-gy) > 7
		}
	}
	if len(b.path) == 0 {
		dx, dy := norm2(gx-b.pos[0], gy-b.pos[1])
		return dx, dy, d, false
	}
	if b.pathBad {
		// a partial path: once we stand at its end the goal is out of reach from here
		lx, ly := b.Nav.Pos(int(b.path[len(b.path)-1]))
		if hyp(lx-b.pos[0], ly-b.pos[1]) < 4 {
			dx, dy := norm2(gx-b.pos[0], gy-b.pos[1])
			return dx, dy, d, false
		}
	}
	// advance past nodes we have reached
	for b.pathIdx < len(b.path)-1 {
		px, py := b.Nav.Pos(int(b.path[b.pathIdx]))
		if hyp(px-b.pos[0], py-b.pos[1]) < 3.2 {
			b.pathIdx++
		} else {
			break
		}
	}
	// look ahead to the farthest node we can walk straight to
	tgt := b.pathIdx
	hi := b.pathIdx + 14
	if hi > len(b.path)-1 {
		hi = len(b.path) - 1
	}
	for j := hi; j > b.pathIdx; j -= 2 {
		px, py := b.Nav.Pos(int(b.path[j]))
		if b.Nav.WalkLine(b.pos[0], b.pos[1], px, py, r, h) {
			tgt = j
			break
		}
	}
	if tgt > b.pathIdx+1 {
		b.pathIdx = tgt - 1
	}
	px, py := b.Nav.Pos(int(b.path[tgt]))
	if tgt == len(b.path)-1 && hyp(gx-px, gy-py) < 4 {
		px, py = gx, gy
	}
	dx, dy := norm2(px-b.pos[0], py-b.pos[1])
	return dx, dy, d, true
}

// unstick notices when we push against something without moving and hops out of it.
func (b *Bot) unstick(act *proto.Action, navigating bool) {
	if !navigating {
		// strafing, holding or fighting: short moves back and forth are not "stuck"
		b.stuckFor = 0
		b.stuckT = b.t
		b.stuckPos = b.pos
		b.unstickT = 0
		return
	}
	moving := hyp(act.Move[0], act.Move[1]) > 0.3
	if b.t-b.stuckT >= 0.5 {
		moved := dist2(b.pos, b.stuckPos)
		if moving && moved < 1.2 {
			b.stuckFor += b.t - b.stuckT
		} else {
			b.stuckFor = 0
		}
		b.stuckT = b.t
		b.stuckPos = b.pos
	}
	if b.stuckFor >= 1.0 && b.t > b.unstickT {
		// hop and slide sideways for a moment
		a := b.rng.Float64() * 2 * math.Pi
		b.unstickDir = [2]float64{math.Cos(a), math.Sin(a)}
		b.unstickT = b.t + 0.7
		b.stuckFor = 0
		b.path = nil
	}
	if b.t < b.unstickT {
		act.Move = [2]float64{act.Move[0]*0.4 + b.unstickDir[0], act.Move[1]*0.4 + b.unstickDir[1]}
		l := hyp(act.Move[0], act.Move[1])
		if l > 1 {
			act.Move[0] /= l
			act.Move[1] /= l
		}
		if b.o.Self.Grounded {
			act.Jump = true
		} else if b.energy > 30 {
			act.Boost = true
		}
	}
}
