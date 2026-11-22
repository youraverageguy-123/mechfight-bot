package mech

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
)

type Slab struct{ Base, Top float32 }

type World struct {
	MinX, MinY float64
	NX, NY     int
	Ground     []float32
	Top        []float32
	Walk       []float32
	Obst       []bool
	Low        []bool
	HasSlab    []bool
	Slabs      map[int32][]Slab
	BX0, BX1   float64
	BY0, BY1   float64

	NS        float64
	NNX, NNY  int
	Blocked   []bool
	Surf      []float32
	Clearance []float32
	Comp      []int32
	MainComp  int32
	mainNear  []int32

	geo

	g      []float32
	came   []int32
	stamp  []uint32
	closed []uint32
	gen    uint32
	heap   []heapItem
}

type heapItem struct {
	f   float32
	idx int32
}

type interval struct {
	base, top float32
	bridge    bool
}

func LoadWorld(dir string) (*World, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "v2", "world.json"))
	if err != nil {
		return nil, err
	}
	var wj struct {
		Bounds struct {
			MinX, MaxX, MinY, MaxY float64
		} `json:"bounds"`
		Terrain struct {
			MinX float64 `json:"minX"`
			MinY float64 `json:"minY"`
			Step float64 `json:"step"`
			NX   int     `json:"nx"`
			NY   int     `json:"ny"`
		} `json:"terrain"`
		Prisms []struct {
			Kind  string      `json:"kind"`
			Base  float64     `json:"base"`
			Top   float64     `json:"top"`
			Ring  []float64   `json:"ring"`
			Holes [][]float64 `json:"holes"`
		} `json:"prisms"`
		Rules Rules `json:"rules"`
		Trees struct {
			Fields []string  `json:"fields"`
			Data   []float64 `json:"data"`
		} `json:"trees"`
	}
	if err := json.Unmarshal(raw, &wj); err != nil {
		return nil, err
	}
	tb, err := os.ReadFile(filepath.Join(dir, "v2", "terrain.i16"))
	if err != nil {
		return nil, err
	}
	w := &World{MinX: wj.Terrain.MinX, MinY: wj.Terrain.MinY, NX: wj.Terrain.NX, NY: wj.Terrain.NY}
	n := w.NX * w.NY
	if len(tb) < n*2 || n == 0 {
		return nil, fmt.Errorf("terrain size mismatch")
	}
	w.BX0, w.BX1, w.BY0, w.BY1 = wj.Bounds.MinX, wj.Bounds.MaxX, wj.Bounds.MinY, wj.Bounds.MaxY
	w.Ground = make([]float32, n)
	for i := 0; i < n; i++ {
		w.Ground[i] = float32(int16(binary.LittleEndian.Uint16(tb[i*2:]))) * 0.01
	}
	var prisms []Prism
	for _, p := range wj.Prisms {
		q := Prism{Kind: p.Kind, Base: p.Base, Top: p.Top, MinX: math.Inf(1), MinY: math.Inf(1), MaxX: math.Inf(-1), MaxY: math.Inf(-1)}
		for _, r := range append([][]float64{p.Ring}, p.Holes...) {
			m := len(r) / 2
			for a := 0; a < m; a++ {
				b := (a + 1) % m
				q.Edges = append(q.Edges, r[a*2], r[a*2+1], r[b*2], r[b*2+1])
				q.MinX, q.MaxX = math.Min(q.MinX, r[a*2]), math.Max(q.MaxX, r[a*2])
				q.MinY, q.MaxY = math.Min(q.MinY, r[a*2+1]), math.Max(q.MaxY, r[a*2+1])
			}
		}
		prisms = append(prisms, q)
	}
	var trees []Tree
	if nf := len(wj.Trees.Fields); nf > 0 {
		col := map[string]int{}
		for i, f := range wj.Trees.Fields {
			col[f] = i
		}
		get := func(row []float64, name string, def float64) float64 {
			if i, ok := col[name]; ok {
				return row[i]
			}
			return def
		}
		for k := 0; k+nf <= len(wj.Trees.Data); k += nf {
			row := wj.Trees.Data[k : k+nf]
			t := Tree{X: get(row, "x", 0), Y: get(row, "y", 0), Z: get(row, "z", 0), Height: get(row, "height", 0),
				CrownR: get(row, "crownR", 1), CrownBase: get(row, "crownBase", 0), TrunkR: get(row, "trunkR", 0)}
			t.CX, t.CY = get(row, "crownX", t.X), get(row, "crownY", t.Y)
			cb, top := t.Z+t.CrownBase, t.Z+t.Height
			t.TrunkTop = t.Z + math.Max(t.CrownBase+0.5*(t.Height-t.CrownBase), 3)
			t.CZ = (cb + top) / 2
			t.RZ = math.Max((top-cb)/2, 0.5)
			trees = append(trees, t)
		}
	}
	if wj.Rules.MaxStep == 0 {
		wj.Rules.MaxStep = 1.5
	}
	w.initGeo(prisms, trees, wj.Rules)

	cells := make(map[int32][]interval)
	for _, p := range wj.Prisms {
		rings := append([][]float64{p.Ring}, p.Holes...)
		minY, maxY := math.Inf(1), math.Inf(-1)
		for k := 1; k < len(p.Ring); k += 2 {
			minY = math.Min(minY, p.Ring[k])
			maxY = math.Max(maxY, p.Ring[k])
		}
		j0 := int(math.Ceil(minY - w.MinY))
		j1 := int(math.Floor(maxY - w.MinY))
		iv := interval{float32(p.Base), float32(p.Top), p.Kind == "bridge"}
		var xs []float64
		for j := max(j0, 0); j <= j1 && j < w.NY; j++ {
			y := w.MinY + float64(j) + 1e-4
			xs = xs[:0]
			for _, r := range rings {
				m := len(r) / 2
				for a := 0; a < m; a++ {
					b := (a + 1) % m
					x0, y0, x1, y1 := r[a*2], r[a*2+1], r[b*2], r[b*2+1]
					if (y0 <= y) != (y1 <= y) {
						xs = append(xs, x0+(y-y0)/(y1-y0)*(x1-x0))
					}
				}
			}
			sort.Float64s(xs)
			for k := 0; k+1 < len(xs); k += 2 {
				i0 := int(math.Ceil(xs[k] - w.MinX))
				i1 := int(math.Floor(xs[k+1] - w.MinX))
				for i := max(i0, 0); i <= i1 && i < w.NX; i++ {
					idx := int32(j*w.NX + i)
					cells[idx] = append(cells[idx], iv)
				}
			}
		}
	}
	w.Top = make([]float32, n)
	w.Walk = make([]float32, n)
	w.Obst = make([]bool, n)
	w.Low = make([]bool, n)
	w.HasSlab = make([]bool, n)
	copy(w.Top, w.Ground)
	copy(w.Walk, w.Ground)
	w.Slabs = make(map[int32][]Slab)
	for idx, ivs := range cells {
		sort.Slice(ivs, func(a, b int) bool { return ivs[a].base < ivs[b].base })
		g := w.Ground[idx]
		top := g
		deck := float32(-1e9)
		low := false
		for _, iv := range ivs {
			if iv.top <= top {
				continue
			}
			if iv.base <= top+1.5 {
				top = iv.top
				continue
			}
			w.Slabs[idx] = append(w.Slabs[idx], Slab{iv.base, iv.top})
			w.HasSlab[idx] = true
			if iv.base-top < 10.5 {
				if iv.bridge && iv.top-g < 6.5 {
					deck = iv.top
				} else {
					low = true
				}
			}
		}
		w.Top[idx] = top
		w.Walk[idx] = top
		if deck > top {
			w.Walk[idx] = deck
		}
		w.Low[idx] = low
		w.Obst[idx] = top-g > 1.2 || low
	}
	w.buildNav()
	return w, nil
}

func (w *World) vidx(x, y float64) int {
	i := int(math.Round(x - w.MinX))
	j := int(math.Round(y - w.MinY))
	if i < 0 {
		i = 0
	} else if i >= w.NX {
		i = w.NX - 1
	}
	if j < 0 {
		j = 0
	} else if j >= w.NY {
		j = w.NY - 1
	}
	return j*w.NX + i
}

func (w *World) InBounds(x, y float64) bool {
	return x > w.BX0 && x < w.BX1 && y > w.BY0 && y < w.BY1
}

func (w *World) GroundAt(x, y float64) float64 {
	fx := x - w.MinX
	fy := y - w.MinY
	fx = math.Max(0, math.Min(fx, float64(w.NX-1)-1e-6))
	fy = math.Max(0, math.Min(fy, float64(w.NY-1)-1e-6))
	i, j := int(fx), int(fy)
	tx, ty := fx-float64(i), fy-float64(j)
	a := float64(w.Ground[j*w.NX+i])
	b := float64(w.Ground[j*w.NX+i+1])
	c := float64(w.Ground[(j+1)*w.NX+i])
	d := float64(w.Ground[(j+1)*w.NX+i+1])
	return (a*(1-tx)+b*tx)*(1-ty) + (c*(1-tx)+d*tx)*ty
}

func (w *World) stack(x, y float64, deck bool) float64 {
	z, _ := w.stackKind(x, y, deck)
	return z
}

func (w *World) stackKind(x, y float64, deck bool) (float64, int) {
	g := w.GroundAt(x, y)
	z := g
	id := -1
	c := w.cellOf(x, y)
	if c < 0 {
		return z, id
	}
	ps := w.geo.gridP[c]
	for pass := 0; pass < 8; pass++ {
		moved := false
		for _, pi := range ps {
			p := &w.geo.Prisms[pi]
			if p.Top <= z || x < p.MinX || x > p.MaxX || y < p.MinY || y > p.MaxY {
				continue
			}
			if !(p.Base <= z+1.5 || deck && p.Kind == "bridge" && p.Top-g < 6.5) {
				continue
			}
			if pointInPoly(p.Edges, x, y) {
				z, id = p.Top, int(pi)
				moved = true
			}
		}
		if !moved {
			break
		}
	}
	return z, id
}

func (w *World) WalkAt(x, y float64) float64 { return w.stack(x, y, true) }

func (w *World) TopAt(x, y float64) float64 { return w.stack(x, y, false) }

func (w *World) Solid(x, y, z float64) bool {
	if z < w.GroundAt(x, y) {
		return true
	}
	c := w.cellOf(x, y)
	if c < 0 {
		return false
	}
	for _, pi := range w.geo.gridP[c] {
		p := &w.geo.Prisms[pi]
		if z > p.Base && z < p.Top && x >= p.MinX && x <= p.MaxX && y >= p.MinY && y <= p.MaxY && pointInPoly(p.Edges, x, y) {
			return true
		}
	}
	return false
}

func (w *World) solidRaster(x, y, z float64) bool {
	idx := w.vidx(x, y)
	if z < float64(w.Top[idx])-0.05 {
		return true
	}
	if w.HasSlab[idx] {
		for _, s := range w.Slabs[int32(idx)] {
			if z > float64(s.Base) && z < float64(s.Top) {
				return true
			}
		}
	}
	return false
}

func (w *World) Clear(a, b Vec3) bool { return w.ShotClear(a, b) }

func (w *World) clearRaster(a, b Vec3) bool {
	dx, dy, dz := b[0]-a[0], b[1]-a[1], b[2]-a[2]
	d := math.Sqrt(dx*dx + dy*dy + dz*dz)
	n := int(d/1.0) + 1
	inv := 1 / float64(n)
	for k := 1; k < n; k++ {
		t := float64(k) * inv
		if w.solidRaster(a[0]+dx*t, a[1]+dy*t, a[2]+dz*t) {
			return false
		}
	}
	return true
}

var NavRadius = 2.6

func (w *World) standable(x, y, z, r float64) bool {
	if x < w.BX0+r || x > w.BX1-r || y < w.BY0+r || y > w.BY1-r {
		return false
	}
	return !w.wallWithin(x, y, z, r, 9)
}

func (w *World) wallWithin(x, y, z, r, h float64) bool {
	lo, hi := z+StepUp, z+h
	r2 := r * r
	found := false
	w.forCells(x-r, y-r, x+r, y+r, func(c int) {
		if found {
			return
		}
		for _, pi := range w.geo.gridP[c] {
			p := &w.geo.Prisms[pi]
			if p.Top <= lo || p.Base >= hi || x+r < p.MinX || x-r > p.MaxX || y+r < p.MinY || y-r > p.MaxY {
				continue
			}
			e := p.Edges
			for i := 0; i < len(e); i += 4 {
				x0, y0 := e[i], e[i+1]
				ux, uy := e[i+2]-x0, e[i+3]-y0
				l2 := ux*ux + uy*uy
				t := 0.0
				if l2 > 1e-9 {
					t = math.Max(0, math.Min(1, ((x-x0)*ux+(y-y0)*uy)/l2))
				}
				if mx, my := x0+ux*t-x, y0+uy*t-y; mx*mx+my*my < r2 {
					found = true
					return
				}
			}
			if pointInPoly(e, x, y) {
				found = true
				return
			}
		}
	})
	return found
}

func (w *World) buildNav() {
	w.NS = 2
	w.NNX = (w.NX + 1) / 2
	w.NNY = (w.NY + 1) / 2
	n := w.NNX * w.NNY
	w.Blocked = make([]bool, n)
	w.Surf = make([]float32, n)
	for c := 0; c < n; c++ {
		x, y := w.CellPos(c)
		z, id := w.stackKind(x, y, true)
		w.Surf[c] = float32(z)

		roof := id >= 0 && w.geo.Prisms[id].Kind != "bridge" && z-w.GroundAt(x, y) > 1.5
		w.Blocked[c] = roof || !w.standable(x, y, z, NavRadius)
	}

	w.Clearance = make([]float32, n)
	q := make([]int32, 0, n)
	for c := 0; c < n; c++ {
		if w.Blocked[c] {
			w.Clearance[c] = 0
			q = append(q, int32(c))
		} else {
			w.Clearance[c] = 1e9
		}
	}
	for h := 0; h < len(q); h++ {
		c := int(q[h])
		d := w.Clearance[c] + float32(w.NS)
		if d > 20 {
			continue
		}
		ci, cj := c%w.NNX, c/w.NNX
		for _, o := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			i, j := ci+o[0], cj+o[1]
			if i < 0 || j < 0 || i >= w.NNX || j >= w.NNY {
				continue
			}
			nb := j*w.NNX + i
			if w.Clearance[nb] > d {
				w.Clearance[nb] = d
				q = append(q, int32(nb))
			}
		}
	}
	for c := range w.Clearance {
		if w.Clearance[c] > 20 {
			w.Clearance[c] = 20
		}
	}

	w.Comp = make([]int32, n)
	var label, bestN int32
	bestSize := 0
	for c0 := 0; c0 < n; c0++ {
		if w.Blocked[c0] || w.Comp[c0] != 0 {
			continue
		}
		label++
		w.Comp[c0] = label
		q = append(q[:0], int32(c0))
		for h := 0; h < len(q); h++ {
			c := int(q[h])
			ci, cj := c%w.NNX, c/w.NNX
			for _, o := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
				i, j := ci+o[0], cj+o[1]
				if i < 0 || j < 0 || i >= w.NNX || j >= w.NNY {
					continue
				}
				nb := j*w.NNX + i
				if w.Comp[nb] == 0 && w.canStep(c, nb, false) && w.canStep(nb, c, false) {
					w.Comp[nb] = label
					q = append(q, int32(nb))
				}
			}
		}
		if len(q) > bestSize {
			bestSize, bestN = len(q), label
		}
	}
	w.MainComp = bestN
	w.g = make([]float32, n)
	w.came = make([]int32, n)
	w.stamp = make([]uint32, n)
	w.closed = make([]uint32, n)
}

func (w *World) Cell(x, y float64) int {
	ci := int(math.Round((x - w.MinX) / w.NS))
	cj := int(math.Round((y - w.MinY) / w.NS))
	if ci < 0 {
		ci = 0
	} else if ci >= w.NNX {
		ci = w.NNX - 1
	}
	if cj < 0 {
		cj = 0
	} else if cj >= w.NNY {
		cj = w.NNY - 1
	}
	return cj*w.NNX + ci
}

func (w *World) CellPos(c int) (float64, float64) {
	return w.MinX + float64(c%w.NNX)*w.NS, w.MinY + float64(c/w.NNX)*w.NS
}

func (w *World) Free(x, y float64) bool { return !w.Blocked[w.Cell(x, y)] }

func (w *World) NearestFree(c int) int {
	if !w.Blocked[c] {
		return c
	}
	ci, cj := c%w.NNX, c/w.NNX
	best, bestD := -1, 1<<30
	for r := 1; r <= 20; r++ {
		for dj := -r; dj <= r; dj++ {
			for di := -r; di <= r; di++ {
				if max(abs(di), abs(dj)) != r {
					continue
				}
				i, j := ci+di, cj+dj
				if i < 0 || j < 0 || i >= w.NNX || j >= w.NNY {
					continue
				}
				if !w.Blocked[j*w.NNX+i] {
					if d := di*di + dj*dj; d < bestD {
						best, bestD = j*w.NNX+i, d
					}
				}
			}
		}
		if best >= 0 {
			return best
		}
	}
	return -1
}

func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}

var nbrs = [8][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}, {1, 1}, {1, -1}, {-1, 1}, {-1, -1}}

func (w *World) canStep(a, b int, diag bool) bool {
	if w.Blocked[b] {
		return false
	}
	rise := w.Surf[b] - w.Surf[a]
	lim := float32(2.2)
	if diag {
		lim = 3.1
	}
	return rise <= lim
}

func (w *World) push(it heapItem) {
	w.heap = append(w.heap, it)
	i := len(w.heap) - 1
	for i > 0 {
		p := (i - 1) / 2
		if w.heap[p].f <= w.heap[i].f {
			break
		}
		w.heap[p], w.heap[i] = w.heap[i], w.heap[p]
		i = p
	}
}

func (w *World) pop() heapItem {
	top := w.heap[0]
	last := len(w.heap) - 1
	w.heap[0] = w.heap[last]
	w.heap = w.heap[:last]
	i := 0
	for {
		l, r := 2*i+1, 2*i+2
		m := i
		if l < last && w.heap[l].f < w.heap[m].f {
			m = l
		}
		if r < last && w.heap[r].f < w.heap[m].f {
			m = r
		}
		if m == i {
			break
		}
		w.heap[m], w.heap[i] = w.heap[i], w.heap[m]
		i = m
	}
	return top
}

func (w *World) Path(sx, sy, gx, gy float64, maxExpand int) (path []int32, reached bool) {
	s := w.NearestFree(w.Cell(sx, sy))
	g := w.NearestFree(w.Cell(gx, gy))
	if s < 0 || g < 0 {
		return nil, false
	}
	if s == g {
		return []int32{int32(s)}, true
	}
	w.gen++
	gen := w.gen
	w.heap = w.heap[:0]
	gi, gj := g%w.NNX, g/w.NNX
	h := func(c int) float32 {
		dx := float64(abs(c%w.NNX - gi))
		dy := float64(abs(c/w.NNX - gj))
		mn, mx := math.Min(dx, dy), math.Max(dx, dy)
		return float32((mx + 0.4142*mn) * w.NS * 1.15)
	}
	w.g[s] = 0
	w.came[s] = -1
	w.stamp[s] = gen
	w.push(heapItem{h(s), int32(s)})
	best, bestH := s, h(s)
	found := false
	for n := 0; len(w.heap) > 0 && n < maxExpand; n++ {
		it := w.pop()
		c := int(it.idx)
		if w.closed[c] == gen {
			continue
		}
		w.closed[c] = gen
		if c == g {
			found = true
			best = c
			break
		}
		if hc := h(c); hc < bestH {
			best, bestH = c, hc
		}
		ci, cj := c%w.NNX, c/w.NNX
		for k, o := range nbrs {
			i, j := ci+o[0], cj+o[1]
			if i < 0 || j < 0 || i >= w.NNX || j >= w.NNY {
				continue
			}
			nb := j*w.NNX + i
			diag := k >= 4
			if w.closed[nb] == gen || !w.canStep(c, nb, diag) {
				continue
			}
			step := float32(w.NS)
			if diag {
				if w.Blocked[cj*w.NNX+i] || w.Blocked[j*w.NNX+ci] {
					continue
				}
				step *= 1.4142
			}

			if cl := w.Clearance[nb]; cl < 4 {
				step *= 1 + (4-cl)*0.15
			}
			ng := w.g[c] + step
			if w.stamp[nb] != gen || ng < w.g[nb] {
				w.g[nb] = ng
				w.came[nb] = int32(c)
				w.stamp[nb] = gen
				w.push(heapItem{ng + h(nb), int32(nb)})
			}
		}
	}
	for c := best; c >= 0; c = int(w.came[c]) {
		path = append(path, int32(c))
		if c == s {
			break
		}
	}
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return path, found
}

func (w *World) LineWalkable(x0, y0, x1, y1 float64) bool {
	dx, dy := x1-x0, y1-y0
	d := math.Hypot(dx, dy)
	n := int(d/1.0) + 1
	prev := w.Cell(x0, y0)
	for k := 1; k <= n; k++ {
		t := float64(k) / float64(n)
		c := w.Cell(x0+dx*t, y0+dy*t)
		if c == prev {
			continue
		}
		if w.Blocked[c] || w.Surf[c]-w.Surf[prev] > 3.1 {
			return false
		}
		prev = c
	}
	return true
}

func (w *World) CanStand(x, y, z, r float64) bool {
	if x < w.BX0+r || x > w.BX1-r || y < w.BY0+r || y > w.BY1-r {
		return false
	}
	return !w.wallWithin(x, y, z, r, 9)
}

func FlatWorld() *World {
	w := &World{MinX: -513, MinY: -334, NX: 1070, NY: 724, BX0: -512, BX1: 555, BY0: -333, BY1: 388}
	n := w.NX * w.NY
	w.Ground = make([]float32, n)
	w.Top = make([]float32, n)
	w.Walk = make([]float32, n)
	w.Obst = make([]bool, n)
	w.Low = make([]bool, n)
	w.HasSlab = make([]bool, n)
	w.Slabs = map[int32][]Slab{}
	w.initGeo(nil, nil, Rules{MaxStep: 1.5})
	w.buildNav()
	return w
}

func (w *World) MainNear(c int) int {
	if w.mainNear == nil {
		n := w.NNX * w.NNY
		near := make([]int32, n)
		depth := make([]uint8, n)
		q := make([]int32, 0, n)
		for i := range near {
			near[i] = -1
			if !w.Blocked[i] && w.Comp[i] == w.MainComp {
				near[i] = int32(i)
				q = append(q, int32(i))
			}
		}
		for h := 0; h < len(q); h++ {
			c := int(q[h])
			if depth[c] >= 20 {
				continue
			}
			ci, cj := c%w.NNX, c/w.NNX
			for _, o := range nbrs {
				i, j := ci+o[0], cj+o[1]
				if i < 0 || j < 0 || i >= w.NNX || j >= w.NNY {
					continue
				}
				nb := j*w.NNX + i
				if near[nb] < 0 {
					near[nb] = near[c]
					depth[nb] = depth[c] + 1
					q = append(q, int32(nb))
				}
			}
		}
		w.mainNear = near
	}
	return int(w.mainNear[c])
}

func (w *World) OffMain(x, y float64) bool {
	c := w.NearestFree(w.Cell(x, y))
	return c < 0 || w.Comp[c] != w.MainComp
}

func (w *World) EscapePlan(x, y float64) (launch, land [2]float64, ok bool) {
	s := w.NearestFree(w.Cell(x, y))
	if s < 0 {
		return
	}
	w.gen++
	gen := w.gen
	queue := []int32{int32(s)}
	w.stamp[s] = gen
	w.g[s] = 0
	best, bestS := -1, math.Inf(1)
	for h := 0; h < len(queue) && h < 60000; h++ {
		c := int(queue[h])
		ci, cj := c%w.NNX, c/w.NNX
		if m := w.MainNear(c); m >= 0 {
			cx, cy := w.CellPos(c)
			mx, my := w.CellPos(m)
			if sc := float64(w.g[c])*w.NS + 4*math.Hypot(mx-cx, my-cy); sc < bestS {
				best, bestS = c, sc
			}
		}
		for k, o := range nbrs {
			i, j := ci+o[0], cj+o[1]
			if i < 0 || j < 0 || i >= w.NNX || j >= w.NNY {
				continue
			}
			nb := j*w.NNX + i
			if w.stamp[nb] == gen || !w.canStep(c, nb, k >= 4) {
				continue
			}
			if k >= 4 && (w.Blocked[cj*w.NNX+i] || w.Blocked[j*w.NNX+ci]) {
				continue
			}
			w.stamp[nb] = gen
			w.g[nb] = w.g[c] + 1
			queue = append(queue, int32(nb))
		}
	}
	if best < 0 {
		return
	}
	lx, ly := w.CellPos(best)
	mx, my := w.CellPos(w.MainNear(best))

	if d := math.Hypot(mx-lx, my-ly); d > 0.1 {
		ex, ey := mx+(mx-lx)/d*5, my+(my-ly)/d*5
		if w.InBounds(ex, ey) && w.Reachable(ex, ey) {
			mx, my = ex, ey
		}
	}
	return [2]float64{lx, ly}, [2]float64{mx, my}, true
}

func (w *World) Reachable(x, y float64) bool {
	c := w.Cell(x, y)
	return !w.Blocked[c] && w.Comp[c] == w.MainComp
}

func (w *World) Exposure(x, y float64) float64 {
	z := w.WalkAt(x, y) + 7
	total := 0.0
	for k := 0; k < 24; k++ {
		a := float64(k) * math.Pi / 12
		c, s := math.Cos(a), math.Sin(a)
		d := 2.5
		for ; d < 200; d += 2.5 {
			px, py := x+c*d, y+s*d
			if !w.InBounds(px, py) || w.solidRaster(px, py, z) {
				break
			}
		}
		total += d
	}
	return total / (24 * 200)
}

const geoCell = 16.0

type Prism struct {
	Kind                   string
	Base, Top              float64
	Edges                  []float64
	MinX, MinY, MaxX, MaxY float64
}

type Tree struct {
	X, Y, Z, Height, CrownR, CrownBase, TrunkR float64
	CX, CY                                     float64
	TrunkTop, CZ, RZ                           float64
}

type Rules struct {
	MaxStep       float64 `json:"maxStep"`
	MaxSlopeDeg   float64 `json:"maxSlopeDeg"`
	CanopyDensity float64 `json:"canopyDensity"`
	CanopyBlockT  float64 `json:"canopyBlockT"`
}

type geo struct {
	Prisms      []Prism
	Trees       []Tree
	Rules       Rules
	maxSlopeTan float64
	gx0, gy0    float64
	gnx, gny    int
	gridP       [][]int32
	gridT       [][]int32
	bnx, bny    int
	blockMax    []float32
}

const StepUp = 1.2

func pointInPoly(e []float64, x, y float64) bool {
	in := false
	for i := 0; i < len(e); i += 4 {
		x0, y0, x1, y1 := e[i], e[i+1], e[i+2], e[i+3]
		if (y0 > y) != (y1 > y) && x < (x1-x0)*(y-y0)/(y1-y0)+x0 {
			in = !in
		}
	}
	return in
}

func closestOnEdges(e []float64, x, y float64) (cx, cy, d float64) {
	best := math.Inf(1)
	for i := 0; i < len(e); i += 4 {
		x0, y0 := e[i], e[i+1]
		ux, uy := e[i+2]-x0, e[i+3]-y0
		l2 := ux*ux + uy*uy
		p := 0.0
		if l2 > 1e-9 {
			p = ((x-x0)*ux + (y-y0)*uy) / l2
		}
		p = math.Max(0, math.Min(1, p))
		mx, my := x0+ux*p, y0+uy*p
		if g := (mx-x)*(mx-x) + (my-y)*(my-y); g < best {
			best, cx, cy = g, mx, my
		}
	}
	return cx, cy, math.Sqrt(best)
}

func (w *World) initGeo(prisms []Prism, trees []Tree, rules Rules) {
	g := &w.geo
	g.Prisms, g.Trees, g.Rules = prisms, trees, rules
	if g.Rules.MaxSlopeDeg == 0 {
		g.Rules.MaxSlopeDeg = 50
	}
	if g.Rules.CanopyDensity == 0 {
		g.Rules.CanopyDensity, g.Rules.CanopyBlockT = 0.35, 0.25
	}
	g.maxSlopeTan = math.Tan(g.Rules.MaxSlopeDeg * math.Pi / 180)
	g.gx0, g.gy0 = w.BX0-geoCell, w.BY0-geoCell
	g.gnx = int(math.Ceil((w.BX1-w.BX0)/geoCell)) + 3
	g.gny = int(math.Ceil((w.BY1-w.BY0)/geoCell)) + 3
	g.gridP = make([][]int32, g.gnx*g.gny)
	g.gridT = make([][]int32, g.gnx*g.gny)
	for i := range g.Prisms {
		p := &g.Prisms[i]
		w.forCells(p.MinX, p.MinY, p.MaxX, p.MaxY, func(c int) { g.gridP[c] = append(g.gridP[c], int32(i)) })
	}
	for i := range g.Trees {
		t := &g.Trees[i]
		r := t.CrownR
		w.forCells(math.Min(t.CX-r, t.X-t.TrunkR), math.Min(t.CY-r, t.Y-t.TrunkR),
			math.Max(t.CX+r, t.X+t.TrunkR), math.Max(t.CY+r, t.Y+t.TrunkR),
			func(c int) { g.gridT[c] = append(g.gridT[c], int32(i)) })
	}
	g.bnx, g.bny = (w.NX+7)/8, (w.NY+7)/8
	bm := make([]float32, g.bnx*g.bny)
	for i := range bm {
		bm[i] = float32(math.Inf(-1))
	}
	for j := 0; j < w.NY; j++ {
		for i := 0; i < w.NX; i++ {
			b := (j/8)*g.bnx + i/8
			if h := w.Ground[j*w.NX+i]; h > bm[b] {
				bm[b] = h
			}
		}
	}
	g.blockMax = make([]float32, len(bm))
	for j := 0; j < g.bny; j++ {
		for i := 0; i < g.bnx; i++ {
			v := bm[j*g.bnx+i]
			if i+1 < g.bnx {
				v = max(v, bm[j*g.bnx+i+1])
			}
			if j+1 < g.bny {
				v = max(v, bm[(j+1)*g.bnx+i])
			}
			if i+1 < g.bnx && j+1 < g.bny {
				v = max(v, bm[(j+1)*g.bnx+i+1])
			}
			g.blockMax[j*g.bnx+i] = v
		}
	}
}

func (w *World) cellOf(x, y float64) int {
	g := &w.geo
	i := int(math.Floor((x - g.gx0) / geoCell))
	j := int(math.Floor((y - g.gy0) / geoCell))
	if i < 0 || j < 0 || i >= g.gnx || j >= g.gny {
		return -1
	}
	return j*g.gnx + i
}

func (w *World) forCells(x0, y0, x1, y1 float64, f func(int)) {
	g := &w.geo
	i0 := max(0, int(math.Floor((x0-g.gx0)/geoCell)))
	i1 := min(g.gnx-1, int(math.Floor((x1-g.gx0)/geoCell)))
	j0 := max(0, int(math.Floor((y0-g.gy0)/geoCell)))
	j1 := min(g.gny-1, int(math.Floor((y1-g.gy0)/geoCell)))
	for j := j0; j <= j1; j++ {
		for i := i0; i <= i1; i++ {
			f(j*g.gnx + i)
		}
	}
}

func (w *World) walkCells(x0, y0, x1, y1 float64, f func(int) float64) {
	w.walkCellsT(x0, y0, x1, y1, func(c int, _, _ float64) float64 { return f(c) })
}

func (w *World) walkCellsT(x0, y0, x1, y1 float64, f func(c int, t0, t1 float64) float64) {
	g := &w.geo
	dx, dy := x1-x0, y1-y0
	ci := int(math.Floor((x0 - g.gx0) / geoCell))
	cj := int(math.Floor((y0 - g.gy0) / geoCell))
	ei := int(math.Floor((x1 - g.gx0) / geoCell))
	ej := int(math.Floor((y1 - g.gy0) / geoCell))
	si, sj := -1, -1
	if dx > 0 {
		si = 1
	}
	if dy > 0 {
		sj = 1
	}
	inf := math.Inf(1)
	tdx, tdy, tmx, tmy := inf, inf, inf, inf
	if math.Abs(dx) > 1e-9 {
		tdx = geoCell / math.Abs(dx)
		if dx > 0 {
			tmx = (float64(ci+1)*geoCell + g.gx0 - x0) / math.Abs(dx)
		} else {
			tmx = (x0 - (float64(ci)*geoCell + g.gx0)) / math.Abs(dx)
		}
	}
	if math.Abs(dy) > 1e-9 {
		tdy = geoCell / math.Abs(dy)
		if dy > 0 {
			tmy = (float64(cj+1)*geoCell + g.gy0 - y0) / math.Abs(dy)
		} else {
			tmy = (y0 - (float64(cj)*geoCell + g.gy0)) / math.Abs(dy)
		}
	}
	best := inf
	tin := 0.0
	for k := 0; k < 4096; k++ {
		if ci >= 0 && cj >= 0 && ci < g.gnx && cj < g.gny {
			best = f(cj*g.gnx+ci, tin, math.Min(1, math.Min(tmx, tmy)))
		}
		if ci == ei && cj == ej {
			return
		}
		var t float64
		if tmx < tmy {
			t = tmx
			tmx += tdx
			ci += si
		} else {
			t = tmy
			tmy += tdy
			cj += sj
		}
		if t > 1 || best < t {
			return
		}
		tin = t
	}
}

func (w *World) GroundGradient(x, y float64) (float64, float64) {
	return (w.GroundAt(x+1, y) - w.GroundAt(x-1, y)) / 2, (w.GroundAt(x, y+1) - w.GroundAt(x, y-1)) / 2
}

func (w *World) SurfaceAt(x, y, maxZ float64) float64 {
	z, _ := w.SurfaceKind(x, y, maxZ)
	return z
}

func (w *World) SurfaceKind(x, y, maxZ float64) (float64, int) {
	z := w.GroundAt(x, y)
	id := -1
	c := w.cellOf(x, y)
	if c < 0 {
		return z, id
	}
	for _, pi := range w.geo.gridP[c] {
		p := &w.geo.Prisms[pi]
		if p.Top > maxZ || p.Top <= z || x < p.MinX || x > p.MaxX || y < p.MinY || y > p.MaxY || !pointInPoly(p.Edges, x, y) {
			continue
		}
		z, id = p.Top, int(pi)
	}
	return z, id
}

func (w *World) CeilingAbove(x, y, z, r float64) float64 {
	best := math.Inf(1)
	w.forCells(x-r, y-r, x+r, y+r, func(c int) {
		for _, pi := range w.geo.gridP[c] {
			p := &w.geo.Prisms[pi]
			if p.Base < z-0.01 || p.Base >= best || x < p.MinX || x > p.MaxX || y < p.MinY || y > p.MaxY {
				continue
			}
			if pointInPoly(p.Edges, x, y) {
				best = p.Base
			}
		}
	})
	return best
}

var TrunkWalls = false

func (w *World) Penetration(x, y, z, r, h float64) (depth, nx, ny float64, kind string, id int) {
	kind, id = "terrain", -1
	lo, hi := z+StepUp, z+h
	put := func(d, ax, ay float64, k string, i int) {
		if d > depth {
			depth, nx, ny, kind, id = d, ax, ay, k, i
		}
	}
	w.forCells(x-r, y-r, x+r, y+r, func(c int) {
		for _, pi := range w.geo.gridP[c] {
			p := &w.geo.Prisms[pi]
			if p.Top <= lo || p.Base >= hi || x+r < p.MinX || x-r > p.MaxX || y+r < p.MinY || y-r > p.MaxY {
				continue
			}
			cx, cy, d := closestOnEdges(p.Edges, x, y)
			if pointInPoly(p.Edges, x, y) {
				ux, uy := cx-x, cy-y
				l := math.Hypot(ux, uy)
				if l == 0 {
					l = 1e-9
				}
				put(l+r, ux/l, uy/l, p.Kind, int(pi))
			} else if d < r {
				l := d
				if l == 0 {
					l = 1e-9
				}
				put(r-d, (x-cx)/l, (y-cy)/l, p.Kind, int(pi))
			}
		}
		for _, ti := range w.geo.gridT[c] {
			if !TrunkWalls {
				break
			}
			t := &w.geo.Trees[ti]
			if t.TrunkTop <= lo || t.Z-1 >= hi {
				continue
			}
			ux, uy := x-t.X, y-t.Y
			d := math.Hypot(ux, uy)
			if f := r + t.TrunkR; d < f {
				if d > 1e-9 {
					put(f-d, ux/d, uy/d, "trunk", int(ti))
				} else {
					put(f-d, 1, 0, "trunk", int(ti))
				}
			}
		}
	})
	if w.GroundAt(x, y) > lo {
		gx, gy := w.GroundGradient(x, y)
		if n := math.Hypot(gx, gy); n > 0.001 {
			ux, uy := -gx/n, -gy/n
			l := 0.0
			for l < 2*r && w.GroundAt(x+ux*l, y+uy*l) > lo {
				l += 0.25
			}
			put(math.Max(l, 0.05), ux, uy, "terrain", -1)
		}
	}
	put(w.BX0+r-x, 1, 0, "terrain", -2)
	put(x+r-w.BX1, -1, 0, "terrain", -2)
	put(w.BY0+r-y, 0, 1, "terrain", -2)
	put(y+r-w.BY1, 0, -1, "terrain", -2)
	return
}

func (w *World) IsWalkable(x, y, r, maxZ, h float64) bool {
	if x-r < w.BX0 || x+r > w.BX1 || y-r < w.BY0 || y+r > w.BY1 {
		return false
	}
	if math.IsNaN(maxZ) {
		maxZ = w.GroundAt(x, y) + StepUp
	}
	z := w.SurfaceAt(x, y, maxZ)
	if z <= w.GroundAt(x, y)+0.01 {
		if gx, gy := w.GroundGradient(x, y); math.Hypot(gx, gy) > w.geo.maxSlopeTan {
			return false
		}
	}
	d, _, _, _, _ := w.Penetration(x, y, z, r, h)
	return d <= 0.001
}

type MoveResult struct {
	X, Y, Z  float64
	Grounded bool
	StoppedZ bool
	Walls    int
}

func (w *World) MoveAndSlide(x, y, z, r, h, dx, dy, dz float64) MoveResult {
	ox, oy, oz := x, y, z
	n := max(1, int(math.Ceil(math.Hypot(dx, dy)/(r*0.4))))
	sx, sy := dx/float64(n), dy/float64(n)
	ms := StepUp
	walls := 0
	var seen [8]int
	for k := 0; k < n; k++ {
		x += sx
		y += sy
		for it := 0; it < 4; it++ {
			d, nx, ny, _, id := w.Penetration(x, y, z, r, h)
			if d <= 1e-4 {
				break
			}
			x += nx * (d + 0.001)
			y += ny * (d + 0.001)
			dup := false
			for i := 0; i < walls && i < len(seen); i++ {
				if seen[i] == id {
					dup = true
				}
			}
			if !dup && walls < len(seen) {
				seen[walls] = id
				walls++
			}
		}
		if s := w.SurfaceAt(x, y, z+ms); s > z && dz <= 0 {
			z = s
		}
	}
	res := MoveResult{X: x, Y: y, Walls: walls}
	surf := w.SurfaceAt(x, y, z+ms)
	nz := z + dz
	if dz > 0 {
		if c := w.CeilingAbove(x, y, z+h, r); nz+h > c {
			nz = c - h
			res.StoppedZ = true
		}
	}
	if nz <= surf+1e-4 {
		nz = surf
		res.Grounded = true
		res.StoppedZ = res.StoppedZ || dz < 0
	} else if dz <= 0 && nz-surf <= 0.75 && oz-w.SurfaceAt(ox, oy, oz+0.01) < 0.05 {
		nz = surf
		res.Grounded = true
	}
	res.Z = nz
	return res
}

type RayOpts struct {
	NoTrunks, NoTerrain, Canopy bool
}

type Hit struct {
	T          float64
	X, Y, Z    float64
	NX, NY, NZ float64
	Kind       string
	ID         int
}

func (w *World) rayPrism(pi int, x, y, z, dx, dy, dz, tmax float64) (Hit, bool) {
	p := &w.geo.Prisms[pi]
	t0, t1 := 0.0, tmax
	slab := func(o, d, lo, hi float64) bool {
		if math.Abs(d) < 1e-9 {
			return o >= lo && o <= hi
		}
		a, b := (lo-o)/d, (hi-o)/d
		if a > b {
			a, b = b, a
		}
		t0, t1 = math.Max(t0, a), math.Min(t1, b)
		return t0 <= t1
	}
	if !slab(x, dx, p.MinX, p.MaxX) || !slab(y, dy, p.MinY, p.MaxY) || !slab(z, dz, p.Base, p.Top) {
		return Hit{}, false
	}
	if z >= p.Base && z <= p.Top && pointInPoly(p.Edges, x, y) {
		l := math.Sqrt(dx*dx+dy*dy+dz*dz) + 1e-300
		return Hit{T: 0, X: x, Y: y, Z: z, NX: -dx / l, NY: -dy / l, NZ: -dz / l, Kind: p.Kind, ID: pi}, true
	}
	var h Hit
	ok := false
	best := tmax
	e := p.Edges
	for i := 0; i < len(e); i += 4 {
		ex, ey := e[i+2]-e[i], e[i+3]-e[i+1]
		den := dx*ey - dy*ex
		if math.Abs(den) < 1e-9 {
			continue
		}
		px, py := e[i]-x, e[i+1]-y
		t := (px*ey - py*ex) / den
		s := (px*dy - py*dx) / den
		if t < 0 || t >= best || s < 0 || s > 1 {
			continue
		}
		hz := z + dz*t
		if hz < p.Base || hz > p.Top {
			continue
		}
		l := math.Hypot(ex, ey)
		best = t
		h = Hit{T: t, X: x + dx*t, Y: y + dy*t, Z: hz, NX: ey / l, NY: -ex / l, Kind: p.Kind, ID: pi}
		ok = true
	}
	if math.Abs(dz) > 1e-9 {
		for _, cap := range [2]struct{ z, n float64 }{{p.Top, 1}, {p.Base, -1}} {
			t := (cap.z - z) / dz
			if t < 0 || t >= best || (cap.n > 0 && dz > 0) || (cap.n < 0 && dz < 0) {
				continue
			}
			hx, hy := x+dx*t, y+dy*t
			if pointInPoly(e, hx, hy) {
				best = t
				h = Hit{T: t, X: hx, Y: hy, Z: cap.z, NZ: cap.n, Kind: p.Kind, ID: pi}
				ok = true
			}
		}
	}
	return h, ok
}

func rayTrunk(cx, cy, r, z0, z1, x, y, z, dx, dy, dz, tmax float64) (float64, bool) {
	px, py := x-cx, y-cy
	a := dx*dx + dy*dy
	if a < 1e-9 {
		return 0, false
	}
	b := 2 * (px*dx + py*dy)
	c := px*px + py*py - r*r
	disc := b*b - 4*a*c
	if disc < 0 {
		return 0, false
	}
	t := (-b - math.Sqrt(disc)) / (2 * a)
	if t < 0 || t >= tmax {
		return 0, false
	}
	if hz := z + dz*t; hz < z0 || hz > z1 {
		return 0, false
	}
	return t, true
}

func rayCrown(tr *Tree, x, y, z, dx, dy, dz float64) (float64, float64, bool) {
	ox, oy, oz := (x-tr.CX)/tr.CrownR, (y-tr.CY)/tr.CrownR, (z-tr.CZ)/tr.RZ
	ux, uy, uz := dx/tr.CrownR, dy/tr.CrownR, dz/tr.RZ
	a := ux*ux + uy*uy + uz*uz
	if a < 1e-9 {
		return 0, 0, false
	}
	b := 2 * (ox*ux + oy*uy + oz*uz)
	c := ox*ox + oy*oy + oz*oz - 1
	disc := b*b - 4*a*c
	if disc <= 0 {
		return 0, 0, false
	}
	s := math.Sqrt(disc)
	t0, t1 := (-b-s)/(2*a), (-b+s)/(2*a)
	if t1 < 0 || t0 > 1 {
		return 0, 0, false
	}
	return t0, t1, true
}

func (w *World) rayTerrain(x, y, z, x1, y1, z1, tmax float64) (Hit, bool) {
	dx, dy, dz := x1-x, y1-y, z1-z
	hl := math.Hypot(dx, dy)
	const blk = 8.0
	above := func(t float64) float64 { return z + dz*t - w.GroundAt(x+dx*t, y+dy*t) }
	if above(0) < 0 {
		return Hit{T: 0, X: x, Y: y, Z: z, NZ: 1, Kind: "terrain", ID: -1}, true
	}
	step := 1.0
	if hl > 1e-9 {
		step = 0.5 / hl
	}
	g := &w.geo
	t := 0.0
	for it := 0; t < tmax; it++ {
		if it > 100000 {
			return Hit{}, false
		}
		px, py := x+dx*t, y+dy*t
		bi := int(math.Floor((px - w.MinX) / blk))
		bj := int(math.Floor((py - w.MinY) / blk))
		inb := bi >= 0 && bj >= 0 && bi < g.bnx && bj < g.bny
		tb := tmax
		if hl > 1e-9 {
			ex, ey := w.MinX+float64(bi)*blk, w.MinY+float64(bj)*blk
			nx, ny := math.Inf(1), math.Inf(1)
			if dx > 1e-9 {
				nx = (ex + blk - px) / dx
			} else if dx < -1e-9 {
				nx = (ex - px) / dx
			}
			if dy > 1e-9 {
				ny = (ey + blk - py) / dy
			} else if dy < -1e-9 {
				ny = (ey - py) / dy
			}
			tb = math.Min(t+math.Max(0, math.Min(nx, ny))+1e-6, tmax)
		}
		if inb && math.Min(z+dz*t, z+dz*tb) > float64(g.blockMax[bj*g.bnx+bi]) {
			t = tb
			continue
		}
		s := math.Min(t+step, tmax)
		if above(s) < 0 {
			lo, hi := t, s
			for k := 0; k < 20; k++ {
				m := (lo + hi) / 2
				if above(m) < 0 {
					hi = m
				} else {
					lo = m
				}
			}
			hx, hy := x+dx*hi, y+dy*hi
			gx, gy := w.GroundGradient(hx, hy)
			l := math.Sqrt(gx*gx + gy*gy + 1)
			return Hit{T: hi, X: hx, Y: hy, Z: w.GroundAt(hx, hy), NX: -gx / l, NY: -gy / l, NZ: 1 / l, Kind: "terrain", ID: -1}, true
		}
		if s >= tmax {
			break
		}
		t = s
	}
	return Hit{}, false
}

func (w *World) Raycast(a, b Vec3, o RayOpts) (Hit, bool) {
	x, y, z := a[0], a[1], a[2]
	dx, dy, dz := b[0]-x, b[1]-y, b[2]-z
	var best Hit
	found := false
	tmax := func() float64 {
		if found {
			return best.T
		}
		return 1
	}
	var seenP, seenT []int32
	var bufP, bufT [64]int32
	seenP, seenT = bufP[:0], bufT[:0]
	has := func(s []int32, v int32) bool {
		for _, u := range s {
			if u == v {
				return true
			}
		}
		return false
	}
	w.walkCells(x, y, b[0], b[1], func(c int) float64 {
		for _, pi := range w.geo.gridP[c] {
			if has(seenP, pi) {
				continue
			}
			seenP = append(seenP, pi)
			if h, ok := w.rayPrism(int(pi), x, y, z, dx, dy, dz, tmax()); ok {
				best, found = h, true
			}
		}
		for _, ti := range w.geo.gridT[c] {
			if has(seenT, ti) {
				continue
			}
			seenT = append(seenT, ti)
			tr := &w.geo.Trees[ti]
			if !o.NoTrunks {
				if t, ok := rayTrunk(tr.X, tr.Y, tr.TrunkR, tr.Z-1, tr.TrunkTop, x, y, z, dx, dy, dz, tmax()); ok {
					best = Hit{T: t, X: x + dx*t, Y: y + dy*t, Z: z + dz*t, Kind: "trunk", ID: int(ti)}
					found = true
				}
			}
			if o.Canopy {
				if t0, _, ok := rayCrown(tr, x, y, z, dx, dy, dz); ok && t0 < tmax() {
					t := math.Max(t0, 0)
					best = Hit{T: t, X: x + dx*t, Y: y + dy*t, Z: z + dz*t, Kind: "canopy", ID: int(ti)}
					found = true
				}
			}
		}
		return tmax()
	})
	if !o.NoTerrain {
		if h, ok := w.rayTerrain(x, y, z, b[0], b[1], b[2], tmax()); ok {
			best, found = h, true
		}
	}
	return best, found
}

func (w *World) CanopyTransmittance(a, b Vec3) float64 {
	return math.Exp(-w.geo.Rules.CanopyDensity * w.crownMeters(a, b, math.Inf(1)))
}

func (w *World) crownMeters(a, b Vec3, limit float64) float64 {
	x, y, z := a[0], a[1], a[2]
	dx, dy, dz := b[0]-x, b[1]-y, b[2]-z
	l := math.Sqrt(dx*dx + dy*dy + dz*dz)
	sum := 0.0
	w.walkCellsT(x, y, b[0], b[1], func(c int, c0, c1 float64) float64 {
		for _, ti := range w.geo.gridT[c] {
			if t0, t1, ok := rayCrown(&w.geo.Trees[ti], x, y, z, dx, dy, dz); ok {
				if t0, t1 = math.Max(t0, c0), math.Min(t1, c1); t1 > t0 {
					sum += (t1 - t0) * l
				}
			}
		}
		if sum > limit {
			return -1
		}
		return 1
	})
	return sum
}

func (w *World) LineOfSight(a, b Vec3) bool {
	if w.anyHit(a, b, true) {
		return false
	}

	lim := -math.Log(w.geo.Rules.CanopyBlockT) / w.geo.Rules.CanopyDensity
	return w.crownMeters(a, b, lim) <= lim
}

func (w *World) ShotClear(a, b Vec3) bool { return !w.anyHit(a, b, false) }

func (w *World) anyHit(a, b Vec3, trunks bool) bool {
	x, y, z := a[0], a[1], a[2]
	dx, dy, dz := b[0]-x, b[1]-y, b[2]-z
	var bufP, bufT [64]int32
	seenP, seenT := bufP[:0], bufT[:0]
	hit := false
	w.walkCells(x, y, b[0], b[1], func(c int) float64 {
	nextP:
		for _, pi := range w.geo.gridP[c] {
			for _, u := range seenP {
				if u == pi {
					continue nextP
				}
			}
			if len(seenP) < cap(seenP) {
				seenP = append(seenP, pi)
			}
			if _, ok := w.rayPrism(int(pi), x, y, z, dx, dy, dz, 1); ok {
				hit = true
				return -1
			}
		}
		if trunks {
		nextT:
			for _, ti := range w.geo.gridT[c] {
				for _, u := range seenT {
					if u == ti {
						continue nextT
					}
				}
				if len(seenT) < cap(seenT) {
					seenT = append(seenT, ti)
				}
				tr := &w.geo.Trees[ti]
				if _, ok := rayTrunk(tr.X, tr.Y, tr.TrunkR, tr.Z-1, tr.TrunkTop, x, y, z, dx, dy, dz, 1); ok {
					hit = true
					return -1
				}
			}
		}
		return 1
	})
	if hit {
		return true
	}
	_, ok := w.rayTerrain(x, y, z, b[0], b[1], b[2], 1)
	return ok
}

func (w *World) InCanopy(x, y, z float64) int {
	c := w.cellOf(x, y)
	if c < 0 {
		return -1
	}
	for _, ti := range w.geo.gridT[c] {
		t := &w.geo.Trees[ti]
		a, b, d := (x-t.CX)/t.CrownR, (y-t.CY)/t.CrownR, (z-t.CZ)/t.RZ
		if a*a+b*b+d*d <= 1 {
			return int(ti)
		}
	}
	return -1
}

func (w *World) SupportAt(x, y, feetZ, r float64) float64 {
	z0 := w.SurfaceAt(x, y, feetZ+StepUp)
	z := z0
	lim := feetZ + 0.05
	re := r + 0.02
	w.forCells(x-re, y-re, x+re, y+re, func(c int) {
		for _, pi := range w.geo.gridP[c] {
			p := &w.geo.Prisms[pi]

			if p.Top <= z || p.Top > lim || p.Top <= z0+StepUp || x+re < p.MinX || x-re > p.MaxX || y+re < p.MinY || y-re > p.MaxY {
				continue
			}
			if _, _, d := closestOnEdges(p.Edges, x, y); d <= re || pointInPoly(p.Edges, x, y) {
				z = p.Top
			}
		}
	})
	return z
}
