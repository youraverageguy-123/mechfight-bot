package world

import "math"

// CanopyBlocked reports whether trees hide b from a: a trunk on the line, or too much crown along it
// (transmittance exp(-density*length) below CanopyBlockT).
func (w *World) CanopyBlocked(ax, ay, az, bx, by, bz float64) bool {
	if len(w.Trees) == 0 {
		return false
	}
	dx, dy, dz := bx-ax, by-ay, bz-az
	d := math.Sqrt(dx*dx + dy*dy + dz*dz)
	if d < 1e-6 {
		return false
	}
	dx, dy, dz = dx/d, dy/d, dz/d
	limit := 1e18
	if w.CanopyDensity > 0 && w.CanopyBlockT > 0 && w.CanopyBlockT < 1 {
		limit = -math.Log(w.CanopyBlockT) / w.CanopyDensity
	}
	hl := dx*dx + dy*dy
	through := 0.0
	var seenBuf [96]int32
	seen := seenBuf[:0]
	const sub = 4.0
	steps := int(d/sub) + 1
	seg := d / float64(steps)
	lastCell := -1
	for s := 0; s <= steps; s++ {
		t := float64(s) * seg
		ci := clampi(int(math.Floor((ax+dx*t-w.gMinX)/gridCell)), 0, w.gnx-1)
		cj := clampi(int(math.Floor((ay+dy*t-w.gMinY)/gridCell)), 0, w.gny-1)
		c := cj*w.gnx + ci
		if c == lastCell {
			continue
		}
		lastCell = c
	trees:
		for _, ti := range w.treeIdx[w.treeOff[c]:w.treeOff[c+1]] {
			for _, sidx := range seen {
				if sidx == ti {
					continue trees
				}
			}
			if len(seen) < cap(seen) {
				seen = append(seen, ti)
			}
			tr := &w.Trees[ti]
			// trunk: vertical cylinder from the ground to the crown base
			if tr.TrunkR > 0 && hl > 1e-12 {
				tt := ((tr.X-ax)*dx + (tr.Y-ay)*dy) / hl
				if tt > 0 && tt < d {
					px, py := ax+dx*tt-tr.X, ay+dy*tt-tr.Y
					if px*px+py*py < tr.TrunkR*tr.TrunkR {
						z := az + dz*tt
						if z >= tr.Z && z <= tr.Z+tr.CrownBase {
							return true
						}
					}
				}
			}
			// crown: ellipsoid between crownBase and height with horizontal radius crownR
			through += ellipsoidChord(ax, ay, az, dx, dy, dz, d, tr)
			if through > limit {
				return true
			}
		}
	}
	return false
}

// CanopyReach returns how far a sight line from (ax,ay,az) along the unit direction (dx,dy,dz) gets
// before trees hide what is behind them (trunk hit, or crown path over the blocking length), capped
// at maxT.
func (w *World) CanopyReach(ax, ay, az, dx, dy, dz, maxT float64) float64 {
	if len(w.Trees) == 0 {
		return maxT
	}
	limit := 1e18
	if w.CanopyDensity > 0 && w.CanopyBlockT > 0 && w.CanopyBlockT < 1 {
		limit = -math.Log(w.CanopyBlockT) / w.CanopyDensity
	}
	hl := dx*dx + dy*dy
	through := 0.0
	var seenBuf [96]int32
	seen := seenBuf[:0]
	const sub = 4.0
	steps := int(maxT/sub) + 1
	seg := maxT / float64(steps)
	lastCell := -1
	best := maxT
	for s := 0; s <= steps; s++ {
		t := float64(s) * seg
		if t > best+gridCell {
			break
		}
		ci := clampi(int(math.Floor((ax+dx*t-w.gMinX)/gridCell)), 0, w.gnx-1)
		cj := clampi(int(math.Floor((ay+dy*t-w.gMinY)/gridCell)), 0, w.gny-1)
		c := cj*w.gnx + ci
		if c == lastCell {
			continue
		}
		lastCell = c
	trees:
		for _, ti := range w.treeIdx[w.treeOff[c]:w.treeOff[c+1]] {
			for _, sidx := range seen {
				if sidx == ti {
					continue trees
				}
			}
			if len(seen) < cap(seen) {
				seen = append(seen, ti)
			}
			tr := &w.Trees[ti]
			if tr.TrunkR > 0 && hl > 1e-12 {
				tt := ((tr.X-ax)*dx + (tr.Y-ay)*dy) / hl
				if tt > 0 && tt < best {
					px, py := ax+dx*tt-tr.X, ay+dy*tt-tr.Y
					if px*px+py*py < tr.TrunkR*tr.TrunkR {
						z := az + dz*tt
						if z >= tr.Z && z <= tr.Z+tr.CrownBase {
							best = tt
							continue
						}
					}
				}
			}
			ch := ellipsoidChord(ax, ay, az, dx, dy, dz, maxT, tr)
			if ch > 0 {
				through += ch
				if through > limit {
					// blocked roughly where this crown sits
					tc := ((tr.CrownX-ax)*dx + (tr.CrownY-ay)*dy)
					if hl > 1e-12 {
						tc /= hl
					}
					if tc < 0 {
						tc = 0
					}
					if tc < best {
						best = tc
					}
				}
			}
		}
	}
	return best
}

// ellipsoidChord returns the length of the segment (o + t d, t in [0,maxT]) inside the tree crown.
func ellipsoidChord(ox, oy, oz, dx, dy, dz, maxT float64, tr *Tree) float64 {
	rh := tr.CrownR
	rv := (tr.Height - tr.CrownBase) * 0.5
	if rh <= 0 || rv <= 0 {
		return 0
	}
	cz := tr.Z + tr.CrownBase + rv
	// scale space so the ellipsoid is a unit sphere
	px, py, pz := (ox-tr.CrownX)/rh, (oy-tr.CrownY)/rh, (oz-cz)/rv
	vx, vy, vz := dx/rh, dy/rh, dz/rv
	a := vx*vx + vy*vy + vz*vz
	b := 2 * (px*vx + py*vy + pz*vz)
	c := px*px + py*py + pz*pz - 1
	disc := b*b - 4*a*c
	if disc <= 0 || a < 1e-12 {
		return 0
	}
	sq := math.Sqrt(disc)
	t0 := (-b - sq) / (2 * a)
	t1 := (-b + sq) / (2 * a)
	if t0 < 0 {
		t0 = 0
	}
	if t1 > maxT {
		t1 = maxT
	}
	if t1 <= t0 {
		return 0
	}
	return t1 - t0
}
