package world

import "math"

// Exposure is a coarse map of how open each ground position is: the mean free sight distance over a
// ring of horizontal rays at eye height, normalised to 0..1. Low values are enclosed pockets.
type Exposure struct {
	Step   int // lattice spacing in nav cells
	NX, NY int
	V      []float32 // 2 = not standable
	nav    *Nav
}

// NewExposure samples every step-th nav cell with `rays` rays out to maxR metres.
func NewExposure(n *Nav, step, rays int, eyeH, maxR float64) *Exposure {
	e := &Exposure{Step: step, nav: n}
	e.NX = (n.NX + step - 1) / step
	e.NY = (n.NY + step - 1) / step
	e.V = make([]float32, e.NX*e.NY)
	dirs := make([][2]float64, rays)
	for i := range dirs {
		a := (float64(i) + 0.5) * 2 * math.Pi / float64(rays)
		dirs[i] = [2]float64{math.Cos(a), math.Sin(a)}
	}
	for j := 0; j < e.NY; j++ {
		for i := 0; i < e.NX; i++ {
			ci, cj := i*step+step/2, j*step+step/2
			if ci >= n.NX {
				ci = n.NX - 1
			}
			if cj >= n.NY {
				cj = n.NY - 1
			}
			k := cj*n.NX + ci
			if n.Solid[k] || n.Clear[k] < 2.6 {
				e.V[j*e.NX+i] = 2
				continue
			}
			x, y := n.Center(ci, cj)
			z := float64(n.Z[k]) + eyeH
			sum := 0.0
			for _, d := range dirs {
				free := n.W.Raycast(x, y, z, d[0], d[1], 0, maxR)
				// trees hide us from anything beyond the proximity sensors (45 m)
				if free > 45 {
					if c := n.W.CanopyReach(x, y, z, d[0], d[1], 0, free); c < free {
						free = math.Max(c, 45)
					}
				}
				sum += free
			}
			e.V[j*e.NX+i] = float32(sum / (float64(rays) * maxR))
		}
	}
	return e
}

// At returns the exposure at a nav cell (2 when the lattice point is not standable).
func (e *Exposure) At(k int) float32 {
	ci, cj := k%e.nav.NX, k/e.nav.NX
	i, j := ci/e.Step, cj/e.Step
	if i >= e.NX {
		i = e.NX - 1
	}
	if j >= e.NY {
		j = e.NY - 1
	}
	return e.V[j*e.NX+i]
}

// Cell returns the nav cell index of lattice point (i,j).
func (e *Exposure) Cell(i, j int) int {
	ci, cj := i*e.Step+e.Step/2, j*e.Step+e.Step/2
	if ci >= e.nav.NX {
		ci = e.nav.NX - 1
	}
	if cj >= e.nav.NY {
		cj = e.nav.NY - 1
	}
	return cj*e.nav.NX + ci
}
