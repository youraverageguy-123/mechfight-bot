package world

import "math"

// Tops returns, for every nav cell, the height of the top surface there: the terrain, or the top of
// the highest prism over the cell centre. Built once and shared.
func (n *Nav) Tops() []float32 {
	n.shared.topOnce.Do(func() {
		tops := make([]float32, n.NX*n.NY)
		for j := 0; j < n.NY; j++ {
			for i := 0; i < n.NX; i++ {
				x, y := n.Center(i, j)
				h, _ := n.W.Floor(x, y, 1e9)
				tops[j*n.NX+i] = float32(h)
			}
		}
		n.shared.tops = tops
	})
	return n.shared.tops
}

// SurfacePath plans a route over the top surface of everything (ground, roofs, courtyards) for a
// mech that can walk small steps, drop any height, and jet up walls no taller than maxClimb. It is
// the planner for places the walk grid does not cover: leaving a roof, getting out of a walled
// courtyard, reaching a circle that closes inside the stadium. sz is our current feet height.
// Returns cell indices from start to goal, or nil.
func (n *Nav) SurfacePath(sx, sy, sz, tx, ty, maxClimb, bodyR float64, maxExpand int, blocked func(k int) bool) []int32 {
	tops := n.Tops()
	si, sj := n.CellAt(sx, sy)
	gi, gj := n.CellAt(tx, ty)
	start, goal := sj*n.NX+si, gj*n.NX+gi
	if start == goal {
		return []int32{int32(start)}
	}
	n.gen++
	if n.gen == 0 {
		for i := range n.stamp {
			n.stamp[i] = 0
			n.closed[i] = 0
		}
		n.gen = 1
	}
	gen := n.gen
	hfn := func(k int) float32 {
		di := float64(absi(k%n.NX - gi))
		dj := float64(absi(k/n.NX - gj))
		mn, mx := di, dj
		if mn > mx {
			mn, mx = mx, mn
		}
		return float32((mx + 0.4142*mn) * n.Cell * 1.1)
	}
	n.open = n.open[:0]
	n.g[start] = 0
	n.parent[start] = -1
	n.stamp[start] = gen
	n.open.push(navItem{int32(start), hfn(start)})
	expanded := 0
	bestK, bestH := start, hfn(start)
	found := false
	for len(n.open) > 0 {
		it := n.open.pop()
		k := int(it.k)
		if n.closed[k] == gen {
			continue
		}
		n.closed[k] = gen
		if k == goal {
			found = true
			break
		}
		expanded++
		if expanded > maxExpand {
			break
		}
		if hk := hfn(k); hk < bestH {
			bestH, bestK = hk, k
		}
		hcur := float64(tops[k])
		if k == start {
			hcur = sz
		}
		ci, cj := k%n.NX, k/n.NX
		for d := 0; d < 8; d++ {
			i, j := ci+navDI[d], cj+navDJ[d]
			if i < 0 || j < 0 || i >= n.NX || j >= n.NY {
				continue
			}
			nk := j*n.NX + i
			if n.closed[nk] == gen {
				continue
			}
			x, y := n.Center(i, j)
			if !n.W.InBounds(x, y) {
				continue
			}
			dist := n.Cell
			if d >= 4 {
				dist *= 1.41421356
			}
			if blocked != nil && blocked(nk) {
				continue
			}
			dh := float64(tops[nk]) - hcur
			cost := dist
			switch {
			case dh > maxClimb:
				continue
			case dh > 1.2:
				// a jet climb: the rise itself plus getting lined up and over the edge
				cost += 10 + dh*2.2
			case dh < -1.2:
				cost += 1.5
			}
			// ground cells too close to a wall for our body: we can stand against the wall there
			// (that is how a climb starts) but a gap made of such cells may be too narrow to pass,
			// so squeezing through costs about as much as going over a low roof instead
			if !n.Solid[nk] && float64(n.Clear[nk]) < bodyR-0.3 {
				cost += 14
			}
			ng := n.g[k] + float32(cost)
			if n.stamp[nk] != gen || ng < n.g[nk] {
				n.g[nk] = ng
				n.parent[nk] = int32(k)
				n.stamp[nk] = gen
				n.open.push(navItem{int32(nk), ng + hfn(nk)})
			}
		}
	}
	end := goal
	if !found {
		end = bestK
		if end == start {
			return nil
		}
	}
	var path []int32
	for k := int32(end); k >= 0; k = n.parent[k] {
		path = append(path, k)
		if len(path) > 100000 {
			break
		}
	}
	for a, b := 0, len(path)-1; a < b; a, b = a+1, b-1 {
		path[a], path[b] = path[b], path[a]
	}
	return path
}

// TopAt returns the top-surface height at a nav cell.
func (n *Nav) TopAt(k int) float64 { return float64(n.Tops()[k]) }

var _ = math.Abs
