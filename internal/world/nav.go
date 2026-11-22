package world

import (
	"math"
	"sync"
)

// Nav is a 2 m walk grid over the ground level of the arena (terrain, low plinths, under bridges).
// Roofs are recorded (Roof) but not part of the walk graph: reaching them takes jets.
type Nav struct {
	W          *World
	Cell       float64
	NX, NY     int
	MinX, MinY float64
	Z          []float32 // walk surface height; valid where Solid is false
	Clear      []float32 // horizontal distance to the nearest wall that blocks a body standing here (capped)
	Head       []float32 // headroom above the walk surface (capped)
	Solid      []bool    // inside a building / out of bounds
	Roof       []float32 // top of the solid at this cell (for Solid cells inside prisms), else NaN

	shared *navShared // lazily built data shared by all clones

	// A* scratch (one Nav per goroutine: not safe for concurrent searches)
	g      []float32
	parent []int32
	stamp  []uint32
	closed []uint32
	gen    uint32
	open   navHeap
}

const (
	NavClearCap = 8.0
	NavHeadCap  = 40.0
)

type navShared struct {
	once     sync.Once
	exp      *Exposure
	compOnce sync.Once
	comp     []int32
	roofOnce sync.Once
	roofs    []Roof
	roofOf   []int32
	roofDist []int16
	topOnce  sync.Once
	tops     []float32
}

// Components labels the cells a base-size mech can walk between (0 = not walkable). Built once.
func (n *Nav) Components() []int32 {
	n.shared.compOnce.Do(func() {
		const r, h = 2.9, 9.0
		comp := make([]int32, n.NX*n.NY)
		var stack []int32
		next := int32(0)
		for s := range comp {
			if comp[s] != 0 || !n.Walkable(s, r, h) {
				continue
			}
			next++
			comp[s] = next
			stack = append(stack[:0], int32(s))
			for len(stack) > 0 {
				k := int(stack[len(stack)-1])
				stack = stack[:len(stack)-1]
				ci, cj := k%n.NX, k/n.NX
				for d := 0; d < 4; d++ {
					i, j := ci+navDI[d], cj+navDJ[d]
					if i < 0 || j < 0 || i >= n.NX || j >= n.NY {
						continue
					}
					nk := j*n.NX + i
					if comp[nk] != 0 || !n.Walkable(nk, r, h) {
						continue
					}
					if !n.canStep(k, nk, n.Cell) && !n.canStep(nk, k, n.Cell) {
						continue
					}
					comp[nk] = next
					stack = append(stack, int32(nk))
				}
			}
		}
		n.shared.comp = comp
	})
	return n.shared.comp
}

// Exposure returns the shared openness map (built on first use, safe across clones and goroutines).
func (n *Nav) Exposure() *Exposure {
	n.shared.once.Do(func() {
		n.shared.exp = NewExposure(n, 3, 24, 8, 200)
	})
	return n.shared.exp
}

// NewNav builds the walk grid. stepUp and bodyHeight come from the mech config.
func NewNav(w *World, stepUp, bodyHeight float64) *Nav {
	n := &Nav{W: w, Cell: 2.0, shared: &navShared{}}
	n.MinX = math.Floor(w.MinX)
	n.MinY = math.Floor(w.MinY)
	n.NX = int(math.Ceil((w.MaxX - n.MinX) / n.Cell))
	n.NY = int(math.Ceil((w.MaxY - n.MinY) / n.Cell))
	N := n.NX * n.NY
	n.Z = make([]float32, N)
	n.Clear = make([]float32, N)
	n.Head = make([]float32, N)
	n.Solid = make([]bool, N)
	n.Roof = make([]float32, N)
	nan := float32(math.NaN())
	for j := 0; j < n.NY; j++ {
		for i := 0; i < n.NX; i++ {
			k := j*n.NX + i
			x, y := n.Center(i, j)
			n.Roof[k] = nan
			if !w.InBounds(x, y) {
				n.Solid[k] = true
				continue
			}
			g := w.TerrainZ(x, y)
			solid := false
			c := w.cellOf(x, y)
			cand := w.prismIdx[w.prismOff[c]:w.prismOff[c+1]]
			for iter := 0; iter < 4; iter++ {
				changed := false
				for _, pi := range cand {
					p := &w.Prisms[pi]
					if p.Top <= g || p.Base > g+0.3 || !p.Contains(x, y) {
						continue
					}
					if p.Top-g <= stepUp {
						g = p.Top
						changed = true
					} else {
						solid = true
						if math.IsNaN(float64(n.Roof[k])) || float32(p.Top) > n.Roof[k] {
							n.Roof[k] = float32(p.Top)
						}
					}
				}
				if !changed {
					break
				}
			}
			if solid {
				// stacked prisms: the roof is the top of the highest one reachable by stacking
				top := float64(n.Roof[k])
				for iter := 0; iter < 4; iter++ {
					changed := false
					for _, pi := range cand {
						p := &w.Prisms[pi]
						if p.Top > top && p.Base <= top+0.3 && p.Contains(x, y) {
							top = p.Top
							changed = true
						}
					}
					if !changed {
						break
					}
				}
				n.Roof[k] = float32(top)
				n.Solid[k] = true
				continue
			}
			n.Z[k] = float32(g)
			head := w.Ceiling(x, y, g+0.3) - g
			if head > NavHeadCap {
				head = NavHeadCap
			}
			n.Head[k] = float32(head)
			n.Clear[k] = float32(w.WallDist(x, y, g+stepUp+0.05, g+bodyHeight, NavClearCap))
		}
	}
	n.g = make([]float32, N)
	n.parent = make([]int32, N)
	n.stamp = make([]uint32, N)
	n.closed = make([]uint32, N)
	return n
}

func (n *Nav) Center(i, j int) (float64, float64) {
	return n.MinX + (float64(i)+0.5)*n.Cell, n.MinY + (float64(j)+0.5)*n.Cell
}

func (n *Nav) CellAt(x, y float64) (int, int) {
	i := clampi(int((x-n.MinX)/n.Cell), 0, n.NX-1)
	j := clampi(int((y-n.MinY)/n.Cell), 0, n.NY-1)
	return i, j
}

func (n *Nav) Index(x, y float64) int {
	i, j := n.CellAt(x, y)
	return j*n.NX + i
}

func (n *Nav) Pos(k int) (float64, float64) {
	return n.Center(k%n.NX, k/n.NX)
}

// Walkable reports whether a body of radius r and height h can stand in cell k.
func (n *Nav) Walkable(k int, r, h float64) bool {
	return !n.Solid[k] && float64(n.Clear[k]) >= r && float64(n.Head[k]) >= h
}

var navDI = [8]int{1, -1, 0, 0, 1, 1, -1, -1}
var navDJ = [8]int{0, 0, 1, -1, 1, -1, 1, -1}

const tanMaxSlope = 1.19 // tan(50 deg)

func (n *Nav) canStep(from, to int, dist float64) bool {
	rise := float64(n.Z[to] - n.Z[from])
	lim := tanMaxSlope * dist * 0.85
	if lim < 1.2 {
		lim = 1.2
	}
	return rise <= lim
}

// Nearest returns the closest walkable cell to (x,y) within maxR cells (ring search), or -1.
func (n *Nav) Nearest(x, y, r, h float64, maxR int) int {
	ci, cj := n.CellAt(x, y)
	best, bestD := -1, 1e18
	for rad := 0; rad <= maxR; rad++ {
		for dj := -rad; dj <= rad; dj++ {
			for di := -rad; di <= rad; di++ {
				if absi(di) != rad && absi(dj) != rad {
					continue
				}
				i, j := ci+di, cj+dj
				if i < 0 || j < 0 || i >= n.NX || j >= n.NY {
					continue
				}
				k := j*n.NX + i
				if !n.Walkable(k, r, h) {
					continue
				}
				px, py := n.Center(i, j)
				d := (px-x)*(px-x) + (py-y)*(py-y)
				if d < bestD {
					best, bestD = k, d
				}
			}
		}
		if best >= 0 && rad >= 1 {
			return best
		}
		if best >= 0 && rad == 0 {
			return best
		}
	}
	return best
}

func absi(a int) int {
	if a < 0 {
		return -a
	}
	return a
}

type navItem struct {
	k int32
	f float32
}
type navHeap []navItem

// push and pop are a plain binary min-heap on f (no interface boxing: path search is our hot loop).
func (h *navHeap) push(it navItem) {
	*h = append(*h, it)
	a := *h
	i := len(a) - 1
	for i > 0 {
		p := (i - 1) / 2
		if a[p].f <= a[i].f {
			break
		}
		a[p], a[i] = a[i], a[p]
		i = p
	}
}

func (h *navHeap) pop() navItem {
	a := *h
	top := a[0]
	n := len(a) - 1
	a[0] = a[n]
	a = a[:n]
	i := 0
	for {
		l := 2*i + 1
		if l >= n {
			break
		}
		m := l
		if r := l + 1; r < n && a[r].f < a[l].f {
			m = r
		}
		if a[i].f <= a[m].f {
			break
		}
		a[i], a[m] = a[m], a[i]
		i = m
	}
	*h = a
	return top
}

// CostFn adds an extra per-cell cost (danger, zone, ...) on top of distance. nil = none.
type CostFn func(k int) float32

// Path runs A* from (sx,sy) to (tx,ty) for a body of radius r, height h. It returns cell indices from
// start to goal (inclusive), or nil. maxExpand bounds the work. extra may be nil.
func (n *Nav) Path(sx, sy, tx, ty, r, h float64, maxExpand int, extra CostFn) []int32 {
	start := n.Nearest(sx, sy, r, h, 6)
	goal := n.Nearest(tx, ty, r, h, 10)
	if start < 0 || goal < 0 {
		return nil
	}
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
	gi, gj := goal%n.NX, goal/n.NX
	hfn := func(k int) float32 {
		di := float64(absi(k%n.NX - gi))
		dj := float64(absi(k/n.NX - gj))
		mn, mx := di, dj
		if mn > mx {
			mn, mx = mx, mn
		}
		// slightly inflated: far fewer expansions for paths a few percent longer
		return float32((mx + 0.4142*mn) * n.Cell * 1.15)
	}
	n.open = n.open[:0]
	n.g[start] = 0
	n.parent[start] = -1
	n.stamp[start] = gen
	n.open.push(navItem{int32(start), hfn(start)})
	expanded := 0
	bestK, bestH := start, hfn(start)
	found := false
	wantClear := float32(r + 2.0)
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
		hk := hfn(k)
		if hk < bestH {
			bestH, bestK = hk, k
		}
		ci, cj := k%n.NX, k/n.NX
		for d := 0; d < 8; d++ {
			i, j := ci+navDI[d], cj+navDJ[d]
			if i < 0 || j < 0 || i >= n.NX || j >= n.NY {
				continue
			}
			nk := j*n.NX + i
			if n.closed[nk] == gen || !n.Walkable(nk, r, h) {
				continue
			}
			dist := n.Cell
			if d >= 4 {
				// no corner cutting
				if !n.Walkable(cj*n.NX+i, r, h) || !n.Walkable(j*n.NX+ci, r, h) {
					continue
				}
				dist *= 1.41421356
			}
			if !n.canStep(k, nk, dist) {
				continue
			}
			cost := float32(dist)
			if n.Clear[nk] < wantClear {
				cost += (wantClear - n.Clear[nk]) * 0.6
			}
			if extra != nil {
				cost += extra(nk)
			}
			ng := n.g[k] + cost
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
		// partial path toward the closest explored cell
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

// WalkLine reports whether a body can walk the straight segment a→b on the ground grid.
func (n *Nav) WalkLine(ax, ay, bx, by, r, h float64) bool {
	d := math.Hypot(bx-ax, by-ay)
	steps := int(d/(n.Cell*0.5)) + 1
	prev := n.Index(ax, ay)
	if !n.Walkable(prev, r, h) {
		return false
	}
	for s := 1; s <= steps; s++ {
		t := float64(s) / float64(steps)
		k := n.Index(ax+(bx-ax)*t, ay+(by-ay)*t)
		if k == prev {
			continue
		}
		if !n.Walkable(k, r, h) {
			return false
		}
		if !n.canStep(prev, k, n.Cell) {
			return false
		}
		prev = k
	}
	return true
}

// Flood runs Dijkstra from (sx,sy) up to maxCost metres, calling visit for every reached cell with its
// path cost. visit returning false stops the flood. extra may be nil.
func (n *Nav) Flood(sx, sy, r, h, maxCost float64, extra CostFn, visit func(k int, cost float32) bool) {
	start := n.Nearest(sx, sy, r, h, 6)
	if start < 0 {
		return
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
	n.open = n.open[:0]
	n.g[start] = 0
	n.parent[start] = -1
	n.stamp[start] = gen
	n.open.push(navItem{int32(start), 0})
	for len(n.open) > 0 {
		it := n.open.pop()
		k := int(it.k)
		if n.closed[k] == gen {
			continue
		}
		n.closed[k] = gen
		if !visit(k, n.g[k]) {
			return
		}
		ci, cj := k%n.NX, k/n.NX
		for d := 0; d < 8; d++ {
			i, j := ci+navDI[d], cj+navDJ[d]
			if i < 0 || j < 0 || i >= n.NX || j >= n.NY {
				continue
			}
			nk := j*n.NX + i
			if n.closed[nk] == gen || !n.Walkable(nk, r, h) {
				continue
			}
			dist := n.Cell
			if d >= 4 {
				if !n.Walkable(cj*n.NX+i, r, h) || !n.Walkable(j*n.NX+ci, r, h) {
					continue
				}
				dist *= 1.41421356
			}
			if !n.canStep(k, nk, dist) {
				continue
			}
			cost := float32(dist)
			if extra != nil {
				cost += extra(nk)
			}
			ng := n.g[k] + cost
			if float64(ng) > maxCost {
				continue
			}
			if n.stamp[nk] != gen || ng < n.g[nk] {
				n.g[nk] = ng
				n.parent[nk] = int32(k)
				n.stamp[nk] = gen
				n.open.push(navItem{int32(nk), ng})
			}
		}
	}
}

// PathTo reconstructs the path to cell k after a Flood (valid until the next search).
func (n *Nav) PathTo(k int) []int32 {
	var path []int32
	for c := int32(k); c >= 0; c = n.parent[c] {
		path = append(path, c)
		if len(path) > 100000 {
			break
		}
	}
	for a, b := 0, len(path)-1; a < b; a, b = a+1, b-1 {
		path[a], path[b] = path[b], path[a]
	}
	return path
}

// Clone returns a Nav sharing the immutable grids but with its own search scratch, so several
// goroutines can search at once.
func (n *Nav) Clone() *Nav {
	c := *n
	N := n.NX * n.NY
	c.g = make([]float32, N)
	c.parent = make([]int32, N)
	c.stamp = make([]uint32, N)
	c.closed = make([]uint32, N)
	c.gen = 0
	c.open = nil
	return &c
}
