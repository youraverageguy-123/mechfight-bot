// Package world loads the static arena (terrain heightfield, building prisms, trees) and answers the
// geometric questions both the bot and the local simulator need: ground height, wall collision,
// ray casts and line of sight.
package world

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
)

const (
	KindBuilding = 0
	KindStadium  = 1
	KindBridge   = 2
)

// Prism is an extruded polygon (with holes) that is solid between Base and Top.
type Prism struct {
	Kind                   uint8
	Base, Top              float64
	Ring                   []float64
	Holes                  [][]float64
	MinX, MinY, MaxX, MaxY float64
}

// Edge is one vertical wall of a prism.
type Edge struct {
	X0, Y0, X1, Y1 float64
	Base, Top      float64
	Prism          int32
}

type Tree struct {
	X, Y, Z, Height, CrownR, CrownBase, TrunkR, CrownX, CrownY float64
}

const gridCell = 8.0

type World struct {
	MinX, MinY, MaxX, MaxY float64 // playable bounds

	// terrain vertex grid
	TMinX, TMinY, TStep float64
	NX, NY              int
	H                   []float32
	TerrainMax          float64

	Prisms []Prism
	Edges  []Edge
	Trees  []Tree

	CanopyDensity, CanopyBlockT float64

	// uniform grid index
	gMinX, gMinY float64
	gnx, gny     int
	edgeOff      []int32
	edgeIdx      []int32
	prismOff     []int32
	prismIdx     []int32
	treeOff      []int32
	treeIdx      []int32

	// coarse terrain max (per 16 m block) for fast ray rejection
	tbN, tbNX, tbNY int
	tbMax           []float32
}

type worldJSON struct {
	Bounds struct {
		MinX, MaxX, MinY, MaxY float64
	} `json:"bounds"`
	Rules struct {
		CanopyDensity float64 `json:"canopyDensity"`
		CanopyBlockT  float64 `json:"canopyBlockT"`
	} `json:"rules"`
	Terrain struct {
		MinX  float64 `json:"minX"`
		MinY  float64 `json:"minY"`
		Step  float64 `json:"step"`
		NX    int     `json:"nx"`
		NY    int     `json:"ny"`
		Scale float64 `json:"scale"`
	} `json:"terrain"`
	Prisms []struct {
		Kind  string      `json:"kind"`
		Base  float64     `json:"base"`
		Top   float64     `json:"top"`
		Ring  []float64   `json:"ring"`
		Holes [][]float64 `json:"holes"`
	} `json:"prisms"`
	Trees struct {
		Fields []string  `json:"fields"`
		Data   []float64 `json:"data"`
	} `json:"trees"`
}

// Load reads <dir>/v2/world.json and <dir>/v2/terrain.i16.
func Load(dir string) (*World, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "v2", "world.json"))
	if err != nil {
		return nil, err
	}
	var wj worldJSON
	if err := json.Unmarshal(raw, &wj); err != nil {
		return nil, err
	}
	w := &World{
		MinX: wj.Bounds.MinX, MaxX: wj.Bounds.MaxX, MinY: wj.Bounds.MinY, MaxY: wj.Bounds.MaxY,
		TMinX: wj.Terrain.MinX, TMinY: wj.Terrain.MinY, TStep: wj.Terrain.Step,
		NX: wj.Terrain.NX, NY: wj.Terrain.NY,
		CanopyDensity: wj.Rules.CanopyDensity, CanopyBlockT: wj.Rules.CanopyBlockT,
	}
	if w.TStep <= 0 {
		w.TStep = 1
	}
	scale := wj.Terrain.Scale
	if scale == 0 {
		scale = 0.01
	}
	tr, err := os.ReadFile(filepath.Join(dir, "v2", "terrain.i16"))
	if err != nil {
		return nil, err
	}
	if len(tr) < w.NX*w.NY*2 {
		return nil, fmt.Errorf("terrain.i16 too short: %d < %d", len(tr), w.NX*w.NY*2)
	}
	w.H = make([]float32, w.NX*w.NY)
	w.TerrainMax = -1e9
	for i := range w.H {
		v := float64(int16(binary.LittleEndian.Uint16(tr[2*i:]))) * scale
		w.H[i] = float32(v)
		if v > w.TerrainMax {
			w.TerrainMax = v
		}
	}
	for _, p := range wj.Prisms {
		pr := Prism{Base: p.Base, Top: p.Top, Ring: p.Ring, Holes: p.Holes}
		switch p.Kind {
		case "stadium":
			pr.Kind = KindStadium
		case "bridge":
			pr.Kind = KindBridge
		}
		if len(pr.Ring) < 6 {
			continue
		}
		pr.MinX, pr.MinY, pr.MaxX, pr.MaxY = 1e18, 1e18, -1e18, -1e18
		for i := 0; i+1 < len(pr.Ring); i += 2 {
			pr.MinX = math.Min(pr.MinX, pr.Ring[i])
			pr.MaxX = math.Max(pr.MaxX, pr.Ring[i])
			pr.MinY = math.Min(pr.MinY, pr.Ring[i+1])
			pr.MaxY = math.Max(pr.MaxY, pr.Ring[i+1])
		}
		w.Prisms = append(w.Prisms, pr)
	}
	// trees
	f := wj.Trees.Fields
	nf := len(f)
	col := map[string]int{}
	for i, name := range f {
		col[name] = i
	}
	if nf > 0 {
		get := func(row int, name string) float64 {
			c, ok := col[name]
			if !ok {
				return 0
			}
			return wj.Trees.Data[row*nf+c]
		}
		n := len(wj.Trees.Data) / nf
		for i := 0; i < n; i++ {
			t := Tree{X: get(i, "x"), Y: get(i, "y"), Z: get(i, "z"), Height: get(i, "height"), CrownR: get(i, "crownR"),
				CrownBase: get(i, "crownBase"), TrunkR: get(i, "trunkR"), CrownX: get(i, "crownX"), CrownY: get(i, "crownY")}
			if _, ok := col["crownX"]; !ok {
				t.CrownX, t.CrownY = t.X, t.Y
			}
			w.Trees = append(w.Trees, t)
		}
	}
	w.buildIndex()
	return w, nil
}

func (w *World) buildIndex() {
	// edges
	for pi := range w.Prisms {
		p := &w.Prisms[pi]
		add := func(r []float64) {
			n := len(r)
			if n < 6 {
				return
			}
			j := n - 2
			for i := 0; i+1 < n; i += 2 {
				if r[j] != r[i] || r[j+1] != r[i+1] {
					w.Edges = append(w.Edges, Edge{X0: r[j], Y0: r[j+1], X1: r[i], Y1: r[i+1], Base: p.Base, Top: p.Top, Prism: int32(pi)})
				}
				j = i
			}
		}
		add(p.Ring)
		for _, h := range p.Holes {
			add(h)
		}
	}
	w.gMinX = math.Floor(w.MinX/gridCell)*gridCell - gridCell
	w.gMinY = math.Floor(w.MinY/gridCell)*gridCell - gridCell
	w.gnx = int(math.Ceil((w.MaxX-w.gMinX)/gridCell)) + 2
	w.gny = int(math.Ceil((w.MaxY-w.gMinY)/gridCell)) + 2
	nc := w.gnx * w.gny
	cellRange := func(minX, minY, maxX, maxY float64) (int, int, int, int) {
		i0 := clampi(int(math.Floor((minX-w.gMinX)/gridCell)), 0, w.gnx-1)
		i1 := clampi(int(math.Floor((maxX-w.gMinX)/gridCell)), 0, w.gnx-1)
		j0 := clampi(int(math.Floor((minY-w.gMinY)/gridCell)), 0, w.gny-1)
		j1 := clampi(int(math.Floor((maxY-w.gMinY)/gridCell)), 0, w.gny-1)
		return i0, j0, i1, j1
	}
	build := func(n int, bbox func(i int) (float64, float64, float64, float64)) ([]int32, []int32) {
		cnt := make([]int32, nc+1)
		for k := 0; k < n; k++ {
			i0, j0, i1, j1 := cellRange(bbox(k))
			for j := j0; j <= j1; j++ {
				for i := i0; i <= i1; i++ {
					cnt[j*w.gnx+i+1]++
				}
			}
		}
		for c := 0; c < nc; c++ {
			cnt[c+1] += cnt[c]
		}
		idx := make([]int32, cnt[nc])
		fill := make([]int32, nc)
		for k := 0; k < n; k++ {
			i0, j0, i1, j1 := cellRange(bbox(k))
			for j := j0; j <= j1; j++ {
				for i := i0; i <= i1; i++ {
					c := j*w.gnx + i
					idx[cnt[c]+fill[c]] = int32(k)
					fill[c]++
				}
			}
		}
		return cnt, idx
	}
	const pad = 0.01
	w.edgeOff, w.edgeIdx = build(len(w.Edges), func(k int) (float64, float64, float64, float64) {
		e := &w.Edges[k]
		return math.Min(e.X0, e.X1) - pad, math.Min(e.Y0, e.Y1) - pad, math.Max(e.X0, e.X1) + pad, math.Max(e.Y0, e.Y1) + pad
	})
	w.prismOff, w.prismIdx = build(len(w.Prisms), func(k int) (float64, float64, float64, float64) {
		p := &w.Prisms[k]
		return p.MinX, p.MinY, p.MaxX, p.MaxY
	})
	w.treeOff, w.treeIdx = build(len(w.Trees), func(k int) (float64, float64, float64, float64) {
		t := &w.Trees[k]
		r := math.Max(t.CrownR, t.TrunkR)
		return math.Min(t.X, t.CrownX) - r, math.Min(t.Y, t.CrownY) - r, math.Max(t.X, t.CrownX) + r, math.Max(t.Y, t.CrownY) + r
	})
	// coarse terrain max
	w.tbN = 16
	w.tbNX = (w.NX + w.tbN - 1) / w.tbN
	w.tbNY = (w.NY + w.tbN - 1) / w.tbN
	w.tbMax = make([]float32, w.tbNX*w.tbNY)
	for i := range w.tbMax {
		w.tbMax[i] = -1e9
	}
	for j := 0; j < w.NY; j++ {
		for i := 0; i < w.NX; i++ {
			h := w.H[j*w.NX+i]
			// a vertex influences the blocks on both sides of it
			for _, bj := range [2]int{j / w.tbN, (j - 1) / w.tbN} {
				for _, bi := range [2]int{i / w.tbN, (i - 1) / w.tbN} {
					if bi >= 0 && bj >= 0 && bi < w.tbNX && bj < w.tbNY {
						b := bj*w.tbNX + bi
						if h > w.tbMax[b] {
							w.tbMax[b] = h
						}
					}
				}
			}
		}
	}
}

func clampi(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clampf(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// TerrainZ samples the terrain bilinearly (clamped at the grid edge).
func (w *World) TerrainZ(x, y float64) float64 {
	fx := (x - w.TMinX) / w.TStep
	fy := (y - w.TMinY) / w.TStep
	if fx < 0 {
		fx = 0
	} else if fx > float64(w.NX-1) {
		fx = float64(w.NX - 1)
	}
	if fy < 0 {
		fy = 0
	} else if fy > float64(w.NY-1) {
		fy = float64(w.NY - 1)
	}
	i := int(fx)
	j := int(fy)
	if i >= w.NX-1 {
		i = w.NX - 2
	}
	if j >= w.NY-1 {
		j = w.NY - 2
	}
	tx := fx - float64(i)
	ty := fy - float64(j)
	k := j*w.NX + i
	h00 := float64(w.H[k])
	h10 := float64(w.H[k+1])
	h01 := float64(w.H[k+w.NX])
	h11 := float64(w.H[k+w.NX+1])
	return (h00*(1-tx)+h10*tx)*(1-ty) + (h01*(1-tx)+h11*tx)*ty
}

func pointInRing(r []float64, x, y float64) bool {
	in := false
	n := len(r)
	j := n - 2
	for i := 0; i+1 < n; i += 2 {
		yi := r[i+1]
		yj := r[j+1]
		if (yi > y) != (yj > y) {
			xi := r[i]
			xj := r[j]
			if x < (xj-xi)*(y-yi)/(yj-yi)+xi {
				in = !in
			}
		}
		j = i
	}
	return in
}

// Contains reports whether (x,y) is inside the prism's footprint.
func (p *Prism) Contains(x, y float64) bool {
	if x < p.MinX || x > p.MaxX || y < p.MinY || y > p.MaxY {
		return false
	}
	if !pointInRing(p.Ring, x, y) {
		return false
	}
	for _, h := range p.Holes {
		if pointInRing(h, x, y) {
			return false
		}
	}
	return true
}

func (w *World) cellOf(x, y float64) int {
	i := clampi(int((x-w.gMinX)/gridCell), 0, w.gnx-1)
	j := clampi(int((y-w.gMinY)/gridCell), 0, w.gny-1)
	return j*w.gnx + i
}

// InBounds reports whether a point is inside the playable rectangle.
func (w *World) InBounds(x, y float64) bool {
	return x >= w.MinX && x <= w.MaxX && y >= w.MinY && y <= w.MaxY
}

// Floor returns the highest solid surface at (x,y) that is at or below z: the terrain or the top of a
// prism under the point. A mech with feet at fz stands on Floor(x, y, fz+step_up).
// blocked is true when the terrain itself is above z (the point is underground).
func (w *World) Floor(x, y, z float64) (floor float64, onPrism int32) {
	floor = w.TerrainZ(x, y)
	onPrism = -1
	c := w.cellOf(x, y)
	for _, pi := range w.prismIdx[w.prismOff[c]:w.prismOff[c+1]] {
		p := &w.Prisms[pi]
		if p.Top > floor && p.Top <= z && p.Contains(x, y) {
			floor = p.Top
			onPrism = pi
		}
	}
	return
}

// Ceiling returns the lowest prism underside above z at (x,y), or +Inf.
func (w *World) Ceiling(x, y, z float64) float64 {
	ceil := math.Inf(1)
	c := w.cellOf(x, y)
	for _, pi := range w.prismIdx[w.prismOff[c]:w.prismOff[c+1]] {
		p := &w.Prisms[pi]
		if p.Base >= z && p.Base < ceil && p.Contains(x, y) {
			ceil = p.Base
		}
	}
	return ceil
}

// SolidAt reports whether the point is inside terrain or a prism.
func (w *World) SolidAt(x, y, z float64) bool {
	if z < w.TerrainZ(x, y) {
		return true
	}
	c := w.cellOf(x, y)
	for _, pi := range w.prismIdx[w.prismOff[c]:w.prismOff[c+1]] {
		p := &w.Prisms[pi]
		if z > p.Base && z < p.Top && p.Contains(x, y) {
			return true
		}
	}
	return false
}

// ColumnBlocked reports whether a vertical span [z0,z1] at (x,y) intersects any prism solid.
func (w *World) ColumnBlocked(x, y, z0, z1 float64) bool {
	c := w.cellOf(x, y)
	for _, pi := range w.prismIdx[w.prismOff[c]:w.prismOff[c+1]] {
		p := &w.Prisms[pi]
		if z1 > p.Base && z0 < p.Top && p.Contains(x, y) {
			return true
		}
	}
	return false
}

// PushOut moves a vertical cylinder (center x,y, radius r, vertical span z0..z1) out of any prism wall
// it overlaps and returns the corrected center plus whether a wall was touched.
func (w *World) PushOut(x, y, z0, z1, r float64) (float64, float64, bool) {
	hit := false
	for pass := 0; pass < 3; pass++ {
		moved := false
		i0 := clampi(int((x-r-w.gMinX)/gridCell), 0, w.gnx-1)
		i1 := clampi(int((x+r-w.gMinX)/gridCell), 0, w.gnx-1)
		j0 := clampi(int((y-r-w.gMinY)/gridCell), 0, w.gny-1)
		j1 := clampi(int((y+r-w.gMinY)/gridCell), 0, w.gny-1)
		for j := j0; j <= j1; j++ {
			for i := i0; i <= i1; i++ {
				c := j*w.gnx + i
				for _, ei := range w.edgeIdx[w.edgeOff[c]:w.edgeOff[c+1]] {
					e := &w.Edges[ei]
					if z1 <= e.Base || z0 >= e.Top {
						continue
					}
					cx, cy := closestOnSeg(x, y, e.X0, e.Y0, e.X1, e.Y1)
					dx, dy := x-cx, y-cy
					d2 := dx*dx + dy*dy
					if d2 >= r*r {
						continue
					}
					d := math.Sqrt(d2)
					if d < 1e-9 {
						// on the wall line: push along the edge normal, away from the prism interior
						nx, ny := e.Y1-e.Y0, -(e.X1 - e.X0)
						nl := math.Hypot(nx, ny)
						if nl < 1e-12 {
							continue
						}
						nx, ny = nx/nl, ny/nl
						if w.Prisms[e.Prism].Contains(x+nx*0.05, y+ny*0.05) {
							nx, ny = -nx, -ny
						}
						x += nx * r
						y += ny * r
					} else {
						push := r - d
						x += dx / d * push
						y += dy / d * push
					}
					moved = true
					hit = true
				}
			}
		}
		if !moved {
			break
		}
	}
	return x, y, hit
}

func closestOnSeg(px, py, ax, ay, bx, by float64) (float64, float64) {
	ex, ey := bx-ax, by-ay
	l2 := ex*ex + ey*ey
	if l2 < 1e-18 {
		return ax, ay
	}
	t := ((px-ax)*ex + (py-ay)*ey) / l2
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	return ax + ex*t, ay + ey*t
}

// WallDist returns the horizontal distance from (x,y) to the nearest prism wall whose vertical span
// overlaps (z0,z1), capped at maxR.
func (w *World) WallDist(x, y, z0, z1, maxR float64) float64 {
	best := maxR * maxR
	i0 := clampi(int((x-maxR-w.gMinX)/gridCell), 0, w.gnx-1)
	i1 := clampi(int((x+maxR-w.gMinX)/gridCell), 0, w.gnx-1)
	j0 := clampi(int((y-maxR-w.gMinY)/gridCell), 0, w.gny-1)
	j1 := clampi(int((y+maxR-w.gMinY)/gridCell), 0, w.gny-1)
	for j := j0; j <= j1; j++ {
		for i := i0; i <= i1; i++ {
			c := j*w.gnx + i
			for _, ei := range w.edgeIdx[w.edgeOff[c]:w.edgeOff[c+1]] {
				e := &w.Edges[ei]
				if z1 <= e.Base || z0 >= e.Top {
					continue
				}
				cx, cy := closestOnSeg(x, y, e.X0, e.Y0, e.X1, e.Y1)
				d2 := (x-cx)*(x-cx) + (y-cy)*(y-cy)
				if d2 < best {
					best = d2
				}
			}
		}
	}
	return math.Sqrt(best)
}

// Raycast returns the distance along the unit direction (dx,dy,dz) from (ox,oy,oz) to the first solid
// (terrain or prism), or maxT when nothing is hit within maxT.
func (w *World) Raycast(ox, oy, oz, dx, dy, dz, maxT float64) float64 {
	// A NaN anywhere would make the grid walk below never terminate: treat it as "nothing hit".
	if ox != ox || oy != oy || oz != oz || dx != dx || dy != dy || dz != dz {
		return maxT
	}
	if maxT != maxT || maxT <= 0 {
		return 0
	}
	best := w.raycastPrisms(ox, oy, oz, dx, dy, dz, maxT)
	return w.raycastTerrain(ox, oy, oz, dx, dy, dz, best)
}

func (w *World) raycastPrisms(ox, oy, oz, dx, dy, dz, maxT float64) float64 {
	best := maxT
	ci := int(math.Floor((ox - w.gMinX) / gridCell))
	cj := int(math.Floor((oy - w.gMinY) / gridCell))
	if ci < 0 || cj < 0 || ci >= w.gnx || cj >= w.gny {
		return best
	}
	var stepI, stepJ int
	tMaxX, tMaxY := math.Inf(1), math.Inf(1)
	tDX, tDY := math.Inf(1), math.Inf(1)
	if dx > 1e-12 {
		stepI = 1
		tMaxX = (w.gMinX + float64(ci+1)*gridCell - ox) / dx
		tDX = gridCell / dx
	} else if dx < -1e-12 {
		stepI = -1
		tMaxX = (w.gMinX + float64(ci)*gridCell - ox) / dx
		tDX = -gridCell / dx
	}
	if dy > 1e-12 {
		stepJ = 1
		tMaxY = (w.gMinY + float64(cj+1)*gridCell - oy) / dy
		tDY = gridCell / dy
	} else if dy < -1e-12 {
		stepJ = -1
		tMaxY = (w.gMinY + float64(cj)*gridCell - oy) / dy
		tDY = -gridCell / dy
	}
	tEnter := 0.0
	// the walk visits at most one cell per row plus one per column; the cap is a hard stop that
	// only matters if the arithmetic ever goes wrong
	for guard := 0; guard < 2*(w.gnx+w.gny)+8; guard++ {
		c := cj*w.gnx + ci
		tExit := math.Min(tMaxX, tMaxY)
		// walls
		for _, ei := range w.edgeIdx[w.edgeOff[c]:w.edgeOff[c+1]] {
			e := &w.Edges[ei]
			ex, ey := e.X1-e.X0, e.Y1-e.Y0
			den := dx*ey - dy*ex
			if den > -1e-12 && den < 1e-12 {
				continue
			}
			ax, ay := e.X0-ox, e.Y0-oy
			t := (ax*ey - ay*ex) / den
			if t <= 1e-9 || t >= best {
				continue
			}
			s := (ax*dy - ay*dx) / den
			if s < 0 || s > 1 {
				continue
			}
			z := oz + t*dz
			if z >= e.Base && z <= e.Top {
				best = t
			}
		}
		// caps (roof from above, underside from below)
		if dz < -1e-9 || dz > 1e-9 {
			for _, pi := range w.prismIdx[w.prismOff[c]:w.prismOff[c+1]] {
				p := &w.Prisms[pi]
				var t float64
				if dz < 0 {
					if oz < p.Top {
						continue
					}
					t = (p.Top - oz) / dz
				} else {
					if oz > p.Base {
						continue
					}
					t = (p.Base - oz) / dz
				}
				if t < tEnter-1e-6 || t > tExit+1e-6 || t >= best || t < 0 {
					continue
				}
				if p.Contains(ox+t*dx, oy+t*dy) {
					best = t
				}
			}
		}
		if tExit >= best {
			break
		}
		tEnter = tExit
		if tMaxX < tMaxY {
			ci += stepI
			tMaxX += tDX
		} else {
			cj += stepJ
			tMaxY += tDY
		}
		if ci < 0 || cj < 0 || ci >= w.gnx || cj >= w.gny {
			break
		}
	}
	return best
}

func (w *World) raycastTerrain(ox, oy, oz, dx, dy, dz, maxT float64) float64 {
	// quick reject: the whole segment is above the highest terrain
	zEnd := oz + dz*maxT
	if oz > w.TerrainMax && zEnd > w.TerrainMax {
		return maxT
	}
	t := 0.0
	prevT := 0.0
	prevH := oz - w.TerrainZ(ox, oy)
	if prevH < 0 {
		// starting underground: let the shot go until it clears, treat as immediate hit
		return 0
	}
	const coarse = 8.0
	for t < maxT {
		// fast path: while the ray is above the highest terrain of the 16 m blocks holding this
		// sample and the next one, nothing in between can touch it
		if !noFastTerrain && t+coarse < maxT {
			x0, y0, z0 := ox+dx*t, oy+dy*t, oz+dz*t
			x1, y1, z1 := x0+dx*coarse, y0+dy*coarse, z0+dz*coarse
			m0, m1 := w.blockMax(x0, y0), w.blockMax(x1, y1)
			zl := z0
			if z1 < zl {
				zl = z1
			}
			if zl > m0+0.3 && zl > m1+0.3 {
				t += coarse
				prevT = t
				prevH = z1 - m1
				continue
			}
		}
		// adaptive step: farther above ground => bigger steps
		step := prevH * 0.5
		if step < 1.0 {
			step = 1.0
		} else if step > 12 {
			step = 12
		}
		t += step
		if t > maxT {
			t = maxT
		}
		x, y, z := ox+dx*t, oy+dy*t, oz+dz*t
		h := z - w.TerrainZ(x, y)
		if h < 0 {
			lo, hi := prevT, t
			for k := 0; k < 12; k++ {
				mid := (lo + hi) * 0.5
				if oz+dz*mid-w.TerrainZ(ox+dx*mid, oy+dy*mid) < 0 {
					hi = mid
				} else {
					lo = mid
				}
			}
			return lo
		}
		prevT, prevH = t, h
	}
	return maxT
}

// noFastTerrain disables the block-skipping fast path (tests compare both against brute force).
var noFastTerrain = false

// blockMax returns the highest terrain in the 16 m block containing (x,y).
func (w *World) blockMax(x, y float64) float64 {
	i := int((x - w.TMinX) / w.TStep)
	j := int((y - w.TMinY) / w.TStep)
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
	return float64(w.tbMax[(j/w.tbN)*w.tbNX+i/w.tbN])
}

// Visible reports whether the straight segment a→b is free of terrain and prisms.
func (w *World) Visible(ax, ay, az, bx, by, bz float64) bool {
	dx, dy, dz := bx-ax, by-ay, bz-az
	d := math.Sqrt(dx*dx + dy*dy + dz*dz)
	if d < 1e-6 {
		return true
	}
	inv := 1 / d
	return w.Raycast(ax, ay, az, dx*inv, dy*inv, dz*inv, d-0.05) >= d-0.05
}

