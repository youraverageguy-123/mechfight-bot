package bot

import "math"

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func hyp(x, y float64) float64 { return math.Sqrt(x*x + y*y) }

func dist2(a, b [3]float64) float64 { return hyp(a[0]-b[0], a[1]-b[1]) }

func dist3(a, b [3]float64) float64 {
	dx, dy, dz := a[0]-b[0], a[1]-b[1], a[2]-b[2]
	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}

// angDiff returns a-b wrapped to (-pi, pi].
func angDiff(a, b float64) float64 {
	d := math.Mod(a-b, 2*math.Pi)
	if d > math.Pi {
		d -= 2 * math.Pi
	} else if d <= -math.Pi {
		d += 2 * math.Pi
	}
	return d
}

func norm2(x, y float64) (float64, float64) {
	l := hyp(x, y)
	if l < 1e-9 {
		return 0, 0
	}
	return x / l, y / l
}

func stat(m map[string]float64, key string, def float64) float64 {
	if m == nil {
		return def
	}
	if v, ok := m[key]; ok {
		return v
	}
	return def
}

// approach moves cur toward target by at most maxDelta (vector form).
func approach2(cx, cy, tx, ty, maxDelta float64) (float64, float64) {
	dx, dy := tx-cx, ty-cy
	d := hyp(dx, dy)
	if d <= maxDelta || d < 1e-12 {
		return tx, ty
	}
	return cx + dx/d*maxDelta, cy + dy/d*maxDelta
}

// segPointDist2D returns the distance from point p to segment a-b and the parameter of the closest point.
func segPointDist2D(px, py, ax, ay, bx, by float64) (float64, float64) {
	ex, ey := bx-ax, by-ay
	l2 := ex*ex + ey*ey
	t := 0.0
	if l2 > 1e-12 {
		t = clamp(((px-ax)*ex+(py-ay)*ey)/l2, 0, 1)
	}
	cx, cy := ax+ex*t, ay+ey*t
	return hyp(px-cx, py-cy), t
}
