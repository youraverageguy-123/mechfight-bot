package world

import (
	"math"
	"sort"
)

// Roof is a flat rooftop region a mech can stand on (connected nav cells with the same roof height).
type Roof struct {
	ID     int
	Z      float64 // roof surface height
	Cells  []int32 // nav cell indices
	Area   float64 // m^2
	Height float64 // above the lowest adjacent ground (what we have to climb)
	// Inner is the cell deepest inside the region and InnerD its distance to the region edge (m).
	Inner  int
	InnerD float64
	// Access points: (ground cell, roof cell) pairs where the wall can be climbed.
	Access []RoofAccess
	CX, CY float64 // centroid
}

type RoofAccess struct {
	Ground int // walkable ground cell next to the wall
	Edge   int // roof cell just inside the edge
	Climb  float64
}

// Roofs finds rooftop regions usable as standing places. Built once and shared.
// The second result maps every nav cell to its roof id (-1: none).
func (n *Nav) Roofs() ([]Roof, []int32) {
	n.shared.roofOnce.Do(func() {
		n.shared.roofs, n.shared.roofOf, n.shared.roofDist = buildRoofs(n)
	})
	return n.shared.roofs, n.shared.roofOf
}

// RoofDist returns, for every roof cell, its distance to the region edge in cells (1 = edge cell).
func (n *Nav) RoofDist() []int16 {
	n.Roofs()
	return n.shared.roofDist
}

func buildRoofs(n *Nav) ([]Roof, []int32, []int16) {
	N := n.NX * n.NY
	roofOf := make([]int32, N)
	for i := range roofOf {
		roofOf[i] = -1
	}
	var roofs []Roof
	var stack []int32
	for s := 0; s < N; s++ {
		if roofOf[s] >= 0 || !n.Solid[s] || math.IsNaN(float64(n.Roof[s])) {
			continue
		}
		id := int32(len(roofs))
		z0 := float64(n.Roof[s])
		roofOf[s] = id
		stack = append(stack[:0], int32(s))
		var cells []int32
		sumZ := 0.0
		for len(stack) > 0 {
			k := int(stack[len(stack)-1])
			stack = stack[:len(stack)-1]
			cells = append(cells, int32(k))
			sumZ += float64(n.Roof[k])
			ci, cj := k%n.NX, k/n.NX
			for d := 0; d < 4; d++ {
				i, j := ci+navDI[d], cj+navDJ[d]
				if i < 0 || j < 0 || i >= n.NX || j >= n.NY {
					continue
				}
				nk := j*n.NX + i
				if roofOf[nk] >= 0 || !n.Solid[nk] || math.IsNaN(float64(n.Roof[nk])) {
					continue
				}
				// same flat level as the seed and as the neighbour
				if math.Abs(float64(n.Roof[nk])-z0) > 0.8 || math.Abs(float64(n.Roof[nk]-n.Roof[k])) > 0.6 {
					continue
				}
				roofOf[nk] = id
				stack = append(stack, int32(nk))
			}
		}
		r := Roof{ID: int(id), Z: sumZ / float64(len(cells)), Cells: cells, Area: float64(len(cells)) * n.Cell * n.Cell}
		roofs = append(roofs, r)
	}
	// distance to the region edge (multi-source BFS from boundary cells), access points, heights
	dist := make([]int16, N)
	for ri := range roofs {
		r := &roofs[ri]
		var q []int32
		minGround := math.Inf(1)
		sx, sy := 0.0, 0.0
		for _, c := range r.Cells {
			k := int(c)
			x, y := n.Pos(k)
			sx += x
			sy += y
			ci, cj := k%n.NX, k/n.NX
			edge := false
			for d := 0; d < 8; d++ {
				i, j := ci+navDI[d], cj+navDJ[d]
				if i < 0 || j < 0 || i >= n.NX || j >= n.NY {
					edge = true
					continue
				}
				nk := j*n.NX + i
				if roofOf[nk] != int32(ri) {
					edge = true
					// a taller neighbour roof is a wall, not an edge we can fall off; still an edge
					if !n.Solid[nk] {
						g := float64(n.Z[nk])
						if g < minGround {
							minGround = g
						}
					}
				}
			}
			if edge {
				dist[k] = 1
				q = append(q, c)
			} else {
				dist[k] = 0
			}
		}
		r.CX, r.CY = sx/float64(len(r.Cells)), sy/float64(len(r.Cells))
		for h := 0; h < len(q); h++ {
			k := int(q[h])
			ci, cj := k%n.NX, k/n.NX
			for d := 0; d < 4; d++ {
				i, j := ci+navDI[d], cj+navDJ[d]
				if i < 0 || j < 0 || i >= n.NX || j >= n.NY {
					continue
				}
				nk := j*n.NX + i
				if roofOf[nk] == int32(ri) && dist[nk] == 0 {
					dist[nk] = dist[k] + 1
					q = append(q, int32(nk))
				}
			}
		}
		best, bd := int(r.Cells[0]), int16(0)
		for _, c := range r.Cells {
			if dist[c] > bd {
				best, bd = int(c), dist[c]
			}
		}
		r.Inner = best
		r.InnerD = (float64(bd) - 0.5) * n.Cell
		if math.IsInf(minGround, 1) {
			r.Height = 0
		} else {
			r.Height = r.Z - minGround
		}
		// access: roof edge cells with a walkable ground cell two or three cells straight out
		for _, c := range r.Cells {
			k := int(c)
			if dist[k] != 1 {
				continue
			}
			ci, cj := k%n.NX, k/n.NX
			for d := 0; d < 4; d++ {
				// the cell right outside must be open air (not another roof), and a body must be able
				// to stand a little further out
				i1, j1 := ci+navDI[d], cj+navDJ[d]
				if i1 < 0 || j1 < 0 || i1 >= n.NX || j1 >= n.NY || n.Solid[j1*n.NX+i1] {
					continue
				}
				for step := 2; step <= 3; step++ {
					i, j := ci+navDI[d]*step, cj+navDJ[d]*step
					if i < 0 || j < 0 || i >= n.NX || j >= n.NY {
						break
					}
					g := j*n.NX + i
					if n.Solid[g] {
						break
					}
					if n.Walkable(g, 2.7, 9) {
						// need a roof cell a few metres in to land on
						i2, j2 := ci-navDI[d]*2, cj-navDJ[d]*2
						if i2 < 0 || j2 < 0 || i2 >= n.NX || j2 >= n.NY || roofOf[j2*n.NX+i2] != int32(ri) {
							break
						}
						r.Access = append(r.Access, RoofAccess{Ground: g, Edge: j2*n.NX + i2, Climb: r.Z - float64(n.Z[g])})
						break
					}
				}
			}
		}
	}
	// keep only roofs worth standing on; re-number
	keep := roofs[:0]
	remap := make([]int32, len(roofs))
	for i := range remap {
		remap[i] = -1
	}
	sort.SliceStable(roofs, func(a, b int) bool { return roofs[a].ID < roofs[b].ID })
	for i := range roofs {
		r := roofs[i]
		if r.Area < 250 || r.InnerD < 5 || len(r.Access) == 0 {
			continue
		}
		remap[r.ID] = int32(len(keep))
		r.ID = len(keep)
		keep = append(keep, r)
	}
	for i := range roofOf {
		if roofOf[i] >= 0 {
			roofOf[i] = remap[roofOf[i]]
		}
	}
	out := make([]Roof, len(keep))
	copy(out, keep)
	for i := range dist {
		if roofOf[i] < 0 {
			dist[i] = 0
		}
	}
	return out, roofOf, dist
}
