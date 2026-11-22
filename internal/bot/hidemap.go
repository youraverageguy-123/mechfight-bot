package bot

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/binary"
	"io"
	"math"
	"sync"

	"mechroyale/internal/world"
)

// The hide map is computed offline from the arena (the map is the same every match): for every place
// a mech can stand, on a 4 m lattice, what share of the ground within vision range can see it (its
// centre or its top) and what share has a clear line from a muzzle to its centre. Nowhere on open
// ground is hidden from everything, so we pick hideouts by these numbers rather than by guessing.
//
//go:embed hidemap.gz
var hideMapGz []byte

const (
	hmGround   = 0 // open ground of the main walkable region
	hmEnclosed = 1 // ground that cannot be walked to from the main region
	hmRoof     = 2
	hmNone     = 255
)

type hideMap struct {
	ok           bool
	nx, ny       int // nav grid the map was built on
	step, lw, lh int
	class        []uint8
	seen, hit    []float32
	cells        []int32 // nav cell of every standable lattice cell
}

var (
	hideMapOnce sync.Once
	hideMapData *hideMap
)

func parseHideMap(gz []byte) *hideMap {
	h := &hideMap{}
	zr, err := gzip.NewReader(bytes.NewReader(gz))
	if err != nil {
		return h
	}
	raw, err := io.ReadAll(zr)
	if err != nil || len(raw) < 24 || string(raw[:4]) != "HM3\n" {
		return h
	}
	u := func(i int) int { return int(int32(binary.LittleEndian.Uint32(raw[4+4*i:]))) }
	h.nx, h.ny, h.step, h.lw, h.lh = u(0), u(1), u(2), u(3), u(4)
	body := raw[24:]
	n := h.lw * h.lh
	if h.step <= 0 || n <= 0 || len(body) < n*3 {
		return h
	}
	h.class = make([]uint8, n)
	h.seen = make([]float32, n)
	h.hit = make([]float32, n)
	for i := 0; i < n; i++ {
		h.class[i] = body[i*3]
		s, t := float32(body[i*3+1])/255, float32(body[i*3+2])/255
		h.seen[i], h.hit[i] = s*s, t*t
		if h.class[i] != hmNone {
			li, lj := i%h.lw, i/h.lw
			ci, cj := li*h.step+h.step/2, lj*h.step+h.step/2
			if ci < h.nx && cj < h.ny {
				h.cells = append(h.cells, int32(cj*h.nx+ci))
			}
		}
	}
	h.ok = true
	return h
}

// getHideMap returns the embedded map if it fits this arena (nil otherwise: callers fall back to the
// run-time exposure estimate).
func getHideMap(nav *world.Nav) *hideMap {
	hideMapOnce.Do(func() { hideMapData = parseHideMap(hideMapGz) })
	h := hideMapData
	if h == nil || !h.ok || h.nx != nav.NX || h.ny != nav.NY {
		return nil
	}
	// spot check against the map we actually loaded: a standable ground cell must not be solid, and
	// a roof cell must be
	bad, n := 0, 0
	for i := 0; i < len(h.cells); i += 97 {
		k := int(h.cells[i])
		cl := h.class[h.lat(k)]
		n++
		if (cl == hmRoof) != nav.Solid[k] {
			bad++
		}
	}
	if n == 0 || bad*20 > n {
		return nil
	}
	return h
}

func (h *hideMap) lat(k int) int {
	return (k/h.nx/h.step)*h.lw + (k%h.nx)/h.step
}

// at returns the class and the seen-from / hittable-from fractions for the lattice cell holding nav cell k.
func (h *hideMap) at(k int) (uint8, float64, float64) {
	i := h.lat(k)
	if i < 0 || i >= len(h.class) {
		return hmNone, 1, 1
	}
	return h.class[i], float64(h.seen[i]), float64(h.hit[i])
}

// hideout is a place to wait out the match.
type hideout struct {
	cell  int
	class uint8
	z     float64
	roof  int // roof region for class hmRoof (-1 otherwise)
	score float64
}

func (b *Bot) hideDanger(seen, hit float64) float64 {
	return 100 * (b.P.HideSeenW*seen + b.P.HideHitW*hit)
}

// chooseHideout picks the least exposed reachable place inside the circle we must end up in, weighing
// how visible and how hittable it is against the trip there. Lower score is better.
func (b *Bot) chooseHideout(zi zoneInfo, allowRoof bool) (hideout, bool) {
	hm := b.hm
	if hm == nil {
		return hideout{}, false
	}
	z := &b.o.Zone
	nav := b.Nav
	comp := nav.Components()
	roofs, roofOf := nav.Roofs()
	roofDist := nav.RoofDist()
	margin := math.Min(14, z.NextRadius*0.25)
	limit := math.Min(b.maxClimb(), b.P.RoofMaxClimb)
	myRoof := -1
	if b.o.Self.Grounded {
		myRoof = b.onRoof()
	}
	// the ground we can walk on: the region we stand in, or (up on a roof) the one we last stood in
	walkComp := b.myComp
	if walkComp == 0 {
		walkComp = b.mainComp()
	}
	type cnd struct {
		h hideout
		d float64
	}
	var short []cnd
	worst := 1e18
	const keep = 48
	// roof access is the same for every cell of a roof: look it up once per roof
	type racc struct {
		done bool
		ok   bool
		acc  world.RoofAccess
		d    float64
	}
	var roofAcc []racc
	if allowRoof {
		roofAcc = make([]racc, len(roofs))
	}
	for _, c := range hm.cells {
		k := int(c)
		x, y := nav.Pos(k)
		if dn := hyp(x-z.NextCenter[0], y-z.NextCenter[1]); dn > z.NextRadius-margin {
			continue
		}
		d := hyp(x-b.pos[0], y-b.pos[1])
		if d > 380 {
			continue
		}
		cl, seen, hit := hm.at(k)
		s := b.hideDanger(seen, hit)
		ho := hideout{cell: k, class: cl, roof: -1}
		switch cl {
		case hmGround, hmEnclosed:
			// only ground we can walk to from where we are (a walled-in area counts when we are in it)
			if comp[k] != walkComp || !nav.Walkable(k, b.radius+0.4, b.height) {
				continue
			}
			ho.z = float64(nav.Z[k])
			if myRoof >= 0 {
				s += 6 // we would have to come down first
			}
		case hmRoof:
			if !allowRoof {
				continue
			}
			id := int(roofOf[k])
			if id < 0 || roofDist[k] < 2 {
				continue
			}
			if until, ok := b.roof.ban[id]; ok && b.t < until {
				continue
			}
			r := &roofs[id]
			ho.z, ho.roof = r.Z, id
			if id == myRoof {
				break // already up here
			}
			ra := &roofAcc[id]
			if !ra.done {
				ra.done = true
				ra.acc, ra.d, ra.ok = b.roofAccessFor(r, limit)
			}
			if !ra.ok {
				continue
			}
			gx, gy := nav.Pos(ra.acc.Ground)
			d = ra.d + hyp(x-gx, y-gy)
			s += ra.acc.Climb * b.P.HideClimbW
		default:
			continue
		}
		s += d * b.P.HideDistW
		if k == b.hideCell {
			s -= b.P.HideStick
		}
		ho.score = s
		if len(short) < keep {
			short = append(short, cnd{ho, d})
			if len(short) == keep {
				worst = -1e18
				for i := range short {
					if short[i].h.score > worst {
						worst = short[i].h.score
					}
				}
			}
			continue
		}
		if s >= worst {
			continue
		}
		// replace the worst kept candidate
		wi := 0
		for i := range short {
			if short[i].h.score > short[wi].h.score {
				wi = i
			}
		}
		short[wi] = cnd{ho, d}
		worst = -1e18
		for i := range short {
			if short[i].h.score > worst {
				worst = short[i].h.score
			}
		}
	}
	if len(short) == 0 {
		return hideout{}, false
	}
	// refine the shortlist with what we know right now: who is near, who could see or hit us there
	best := -1
	for i := range short {
		h := &short[i].h
		x, y := nav.Pos(h.cell)
		s := h.score
		for _, tr := range b.tracks {
			if tr.NPC || b.t-tr.LastSeen > 25 {
				continue
			}
			d := hyp(tr.Pos[0]-x, tr.Pos[1]-y)
			if d < 90 && math.Abs(tr.Pos[2]-h.z) < 12 {
				s += (90 - d) * 0.35
			}
			if d < 260 && b.t-tr.LastSeen < 8 {
				sc := tr.scale(b.sizePerCube)
				switch {
				case b.W.Visible(tr.Pos[0], tr.Pos[1], tr.Pos[2]+7*sc, x, y, h.z+b.height*0.5):
					s += 40 // it could shoot us there
				case b.W.Visible(tr.Pos[0], tr.Pos[1], tr.Pos[2]+8*sc, x, y, h.z+b.height):
					s += 12 // it would see our head
				}
			}
		}
		for j := range b.o.Drops {
			if hyp(b.o.Drops[j].Pos[0]-x, b.o.Drops[j].Pos[1]-y) < 70 {
				s += 15
			}
		}
		h.score = s
		if best < 0 || s < short[best].h.score {
			best = i
		}
	}
	return short[best].h, true
}

// roofAccessFor returns the nearest usable place to climb a roof from.
func (b *Bot) roofAccessFor(r *world.Roof, limit float64) (world.RoofAccess, float64, bool) {
	comp := b.Nav.Components()
	var acc world.RoofAccess
	ad := 1e18
	for _, a := range r.Access {
		if a.Climb > limit || a.Climb < 3 {
			continue
		}
		if b.myComp != 0 && comp[a.Ground] != b.myComp {
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
	return acc, ad, ad < 1e17
}
