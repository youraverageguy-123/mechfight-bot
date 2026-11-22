package bot

import "math"

// How to lead a mech with the gun.
//
// Measured on 11,000 real gun shots at mechs that were watching the shooter (Sat 21 Nov): about half of
// the top teams' bots jitter on the spot under fire (aiming where they stand beats any lead by 10-25
// points against them), the other half keep walking (a lead beats no lead by as much). Which kind a
// bot is does not change during a match, so we learn it per opponent: every decision we score one
// "virtual shot" fired one flight time ago both ways against where the mech really is now, and aim the
// way that has been missing by less. Until there is enough evidence the lead is scaled by how
// persistent its recent motion has been (a straight walker gets the full lead, a jitterer none).

// leadRecord stores the current observation of a listed mech and scores one virtual shot.
func (b *Bot) leadRecord(tr *Track, facing bool) {
	if tr.hn > 0 {
		last := &tr.hist[(tr.hi+trkHist-1)%trkHist]
		if b.t-last.t > 0.25 || b.t <= last.t {
			if b.t <= last.t {
				return
			}
			tr.hn, tr.hi = 0, 0 // lost sight of it for a while: the old samples do not connect
		}
	}
	tr.hist[tr.hi] = trkSample{t: b.t, x: tr.Pos[0], y: tr.Pos[1], vx: tr.Vel[0], vy: tr.Vel[1], facing: facing}
	tr.hi = (tr.hi + 1) % trkHist
	if tr.hn < trkHist {
		tr.hn++
	}
	d := hyp(tr.Pos[0]-b.pos[0], tr.Pos[1]-b.pos[1])
	if d < 55 || d > 240 {
		return
	}
	tf := math.Max(0, d-3) / 252
	s := b.leadSample(tr, b.t-(fireLatency+tf))
	if s == nil || !s.facing {
		return // only its behaviour while it is fighting us counts
	}
	// both aim points as they would have been chosen then, against where it is now, measured across
	// our line of fire
	side := func(px, py float64) float64 {
		// aim point (px,py) vs actual position: sideways distance at the target
		ax, ay := norm2(px-b.pos[0], py-b.pos[1])
		return math.Abs(-(tr.Pos[0]-b.pos[0])*ay + (tr.Pos[1]-b.pos[1])*ax)
	}
	k := b.leadPersistAt(tr, s.t)
	lead := (fireLatency + tf*b.leadDamp(tf)) * k
	mNone := math.Min(side(s.x, s.y), 6)
	mLead := math.Min(side(s.x+s.vx*lead, s.y+s.vy*lead), 6)
	const keep = 0.997 // about a quarter of a minute of memory
	tr.leadN = tr.leadN*keep + 1
	tr.missNone = tr.missNone*keep + mNone
	tr.missLead = tr.missLead*keep + mLead
}

// leadSample returns the stored sample closest to time t (nil if none within 0.07 s).
func (b *Bot) leadSample(tr *Track, t float64) *trkSample {
	var best *trkSample
	bd := 0.07
	for i := 0; i < tr.hn; i++ {
		s := &tr.hist[(tr.hi+trkHist-1-i)%trkHist]
		if dt := math.Abs(s.t - t); dt < bd {
			best, bd = s, dt
		}
		if s.t < t-0.1 {
			break
		}
	}
	return best
}

// leadPersistAt is how straight the mech had been moving over the 1.2 s before time t: 1 for a steady
// walk (or when we cannot tell), 0 for jitter on the spot.
func (b *Bot) leadPersistAt(tr *Track, t float64) float64 {
	if !b.P.LeadPersist {
		return 1
	}
	var first, last *trkSample
	n := 0
	sp := 0.0
	for i := 0; i < tr.hn; i++ {
		s := &tr.hist[(tr.hi+trkHist-1-i)%trkHist]
		if s.t > t+0.01 {
			continue
		}
		if s.t < t-1.25 {
			break
		}
		if last == nil {
			last = s
		}
		first = s
		sp += hyp(s.vx, s.vy)
		n++
	}
	if n < 8 || last == nil || last.t-first.t < 0.7 {
		return 1
	}
	mean := hyp(last.x-first.x, last.y-first.y) / (last.t - first.t)
	pers := mean / math.Max(sp/float64(n), 0.5)
	return clamp((pers-0.2)/0.5, 0, 1)
}

// gunLead returns the multiplier to apply to the velocity lead when shooting the gun at this mech.
func (b *Bot) gunLead(tr *Track) float64 {
	if tr.NPC {
		return 1
	}
	if b.P.LeadCum && tr.leadN >= b.P.LeadCumMin {
		margin := 0.2 * tr.leadN
		if tr.missNone < tr.missLead-margin {
			return 0 // it jitters: shoot where it stands
		}
		if tr.missLead < tr.missNone-margin {
			return 1 // it walks: lead it fully whatever its last second looked like
		}
	}
	return b.leadPersistAt(tr, b.t)
}
