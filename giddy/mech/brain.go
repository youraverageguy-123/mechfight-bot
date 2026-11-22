package mech

import (
	"math"
	"os"
	"strconv"
	"math/rand"
	"sort"
)

const lat = 0.075

type Track struct {
	MechSeen
	Seen    float64
	HitMe   float64
	Dist    float64
	LOS     bool
	Closing float64
	Engage  bool
}

type known struct {
	ID   int
	Kind string
	Pos  Vec3
	HP   float64
	Seen float64
}

type circle struct{ X, Y, R float64 }

type Brain struct {
	W    *World
	Cfg  Config
	Slot int
	rng  *rand.Rand
	P    Params

	tracks  map[int]*Track
	npcs    map[int]*Track
	crates  map[int]*known
	cubes   map[int]*known
	pickups map[int]*known
	bad     map[int]float64

	now float64

	path     []int32
	pathGoal [2]float64
	pathAt   float64
	pathI    int

	strafe      float64
	strafeUntil float64

	retreat   bool
	coverAt   float64
	coverGoal [2]float64
	coverOK   bool

	lastHurt    float64
	hurtBearing float64
	hurtSeenBy  bool

	lootID    int
	lootSince float64

	hold      circle
	holdStart float64

	homeXY       [2]float64
	homeAt       float64
	homeSet      bool
	explore      [2]float64
	exploreUntil float64

	stuckSince   float64
	unstuckUntil float64
	unstuckDir   [2]float64
	lastThreatAt float64
	lastAim      [2]float64
	laserHeld    float64
	Mode         string
	Panics       int

	escLaunch, escLand [2]float64
	escAt              float64
	escOK, escFlying   bool
	oddsFleeUntil      float64
	base               Params
	baseSet, pracOn    bool
	endGhostOn         bool
	openOn             bool
	npcHurt            float64
	npcBearing         float64
	notPractice        bool
	startAlive, kills  int
	otherDead          int
	stickID            int
	stickAt, loseAt    float64
}

type Params struct {
	RetreatHP   float64
	RecoverHP   float64
	GunRange    float64
	LightRange  float64
	LaserRange  float64
	EngageDist  float64
	FarmNPC     bool
	Explore     bool
	Ghost       bool
	HomeExpo    float64
	LootRadius  float64
	CrateUntil  float64
	UseDrops    bool
	EndGhost    float64
	Dodge       bool
	NoMortar    bool
	Peek        bool
	LaserFirst  bool
	CrowdHide   bool
	Vulture     float64
	Ambush      float64
	LaserSeek   float64
	Lethal      bool
	Alert       float64
	Shy         float64
	EarlyHide   float64
	Evade       bool
	Odds        float64
	OddsFlee    float64
	Practice    bool
	Stick       float64
	HPw         float64
	LowBonus    float64
	MaxTarget   float64
	LoseRule    bool
	FightLoot   float64
	FarmHP      float64
	AllFoes     bool
	CubeGoal    float64
	Econ        bool
	LaserUp     bool
	EconSafe    float64
	Opening     bool
	NanoUp      bool
	Finish      float64
	FinishHP    float64
	FinishRush  bool
	Pursue      float64
	FightBoost  float64
	DodgeBoost  float64
	ThreatBoost float64
	SpeedUp     bool
	Backstab    float64
	LethalK     float64
}

func DefaultParams() Params {
	p := Params{RetreatHP: 0.38, RecoverHP: 0.8, GunRange: 85, LightRange: 28, LaserRange: 85, EngageDist: 60, FarmNPC: true, Explore: false,
		Ghost: true, HomeExpo: 120, LootRadius: 90, CrateUntil: 150, UseDrops: false, Dodge: true, NoMortar: true,
		Peek: true, LaserFirst: false, CrowdHide: true, Practice: true, EndGhost: 140}
	if v, err := strconv.ParseFloat(os.Getenv("GIDDY_ENDGHOST"), 64); err == nil { // per-slot table from bot.py (unset = 140)
		p.EndGhost = v
	}

	p.LaserFirst = true

	return p
}

func NewBrain(h *Hello, w *World) *Brain {
	seed := int64(h.Seed)
	if seed == 0 {
		seed = int64(h.Slot) + 1
	}
	return &Brain{
		W: w, Cfg: h.Config, Slot: h.Slot, rng: rand.New(rand.NewSource(seed)), P: DefaultParams(),
		tracks: map[int]*Track{}, npcs: map[int]*Track{}, crates: map[int]*known{}, cubes: map[int]*known{},
		pickups: map[int]*known{}, bad: map[int]float64{}, strafe: 1, lootID: -1, lastHurt: -100, lastThreatAt: -100,
		holdStart: -1,
	}
}

func hyp(x, y float64) float64 { return math.Hypot(x, y) }

func angDiff(a, b float64) float64 {
	d := math.Mod(a-b, 2*math.Pi)
	if d > math.Pi {
		d -= 2 * math.Pi
	} else if d < -math.Pi {
		d += 2 * math.Pi
	}
	return d
}

func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }

func (b *Brain) Act(o *Obs) (act Action) {
	act.Tick = o.Tick
	defer func() {
		if r := recover(); r != nil {
			b.Panics++
			a := b.lastAim
			act = Action{Tick: o.Tick, Aim: &a}
		}
	}()
	b.step(o, &act)
	if act.Aim != nil {
		a := act.Aim
		if math.IsNaN(a[0]) || math.IsInf(a[0], 0) || math.IsNaN(a[1]) || math.IsInf(a[1], 0) {
			act.Aim = nil
		} else {
			b.lastAim = *a
		}
	}
	if math.IsNaN(act.Move[0]) || math.IsNaN(act.Move[1]) || math.IsInf(act.Move[0], 0) || math.IsInf(act.Move[1], 0) {
		act.Move = [2]float64{}
	}
	return
}

func (b *Brain) scaleOf(cubes int) float64 { return 1 + b.Cfg.Cubes.SizePerCube*float64(cubes) }

func (b *Brain) center(t *Track) Vec3 {
	h := b.Cfg.Mech.Height * b.scaleOf(t.Cubes)
	if t.Kind == "npc" {
		h = b.Cfg.Npc.Height
	}
	return Vec3{t.Pos[0], t.Pos[1], t.Pos[2] + h*0.5}
}

func (b *Brain) eye(t *Track) Vec3 {
	return Vec3{t.Pos[0], t.Pos[1], t.Pos[2] + b.Cfg.Mech.EyeHeight*b.scaleOf(t.Cubes)}
}

func (b *Brain) observe(o *Obs) {
	me := &o.Self
	now := o.T
	if me.Stats.Radius == 0 {
		me.Stats = b.Cfg.Mech
	}
	eye := Vec3{me.Pos[0], me.Pos[1], me.Pos[2] + me.Stats.EyeHeight}
	muzzle := Vec3{me.Pos[0], me.Pos[1], me.Pos[2] + me.Stats.MuzzleHeight}
	for i := range o.Mechs {
		m := &o.Mechs[i]
		tab := b.tracks
		if m.Kind == "npc" {
			tab = b.npcs
		}
		t := tab[m.ID]
		if t == nil {
			t = &Track{HitMe: -100}
			tab[m.ID] = t
		}
		t.MechSeen = *m
		t.Seen = now
		dx, dy := m.Pos[0]-me.Pos[0], m.Pos[1]-me.Pos[1]
		t.Dist = hyp(dx, dy)
		c := b.center(t)
		t.LOS = b.W.Clear(muzzle, c)
		if !t.LOS && m.Kind == "mech" {
			h := b.Cfg.Mech.Height * b.scaleOf(m.Cubes)
			t.LOS = b.W.Clear(muzzle, Vec3{m.Pos[0], m.Pos[1], m.Pos[2] + h*0.85})
		}
		if t.Dist > 1 {
			t.Closing = -((m.Vel[0]-me.Vel[0])*dx + (m.Vel[1]-me.Vel[1])*dy) / t.Dist
		}
	}
	for id, t := range b.tracks {
		if now-t.Seen > 20 {
			delete(b.tracks, id)
		}
	}
	for id, t := range b.npcs {
		if now-t.Seen > 120 {
			delete(b.npcs, id)
		}
	}
	upd := func(tab map[int]*known, items []Item) {
		for i := range items {
			it := &items[i]
			k := tab[it.ID]
			if k == nil {
				k = &known{ID: it.ID}
				tab[it.ID] = k
			}
			k.Kind, k.Pos, k.HP, k.Seen = it.Kind, it.Pos, it.HP, now
		}
		for id, k := range tab {
			if k.Seen == now {
				continue
			}
			d := hyp(k.Pos[0]-me.Pos[0], k.Pos[1]-me.Pos[1])
			if d < 40 && now-k.Seen > 0.35 && b.W.Clear(eye, Vec3{k.Pos[0], k.Pos[1], k.Pos[2] + 1.5}) {
				delete(tab, id)
			}
		}
	}
	upd(b.crates, o.Crates)
	upd(b.cubes, o.Cubes)
	upd(b.pickups, o.Pickups)
	for i := range o.Events {
		e := &o.Events[i]
		switch e.Type {
		case "damaged":
			if e.Weapon == "npc" && e.Bearing != nil {
				b.npcHurt, b.npcBearing = now, *e.Bearing
			}
			if e.By != nil {
				b.lastHurt = now
				if e.Bearing != nil {
					b.hurtBearing = *e.Bearing
				}
				if t := b.tracks[*e.By]; t != nil {
					t.HitMe = now
				}
				if n := b.npcs[*e.By]; n != nil {
					n.HitMe = now
				}
			}
		case "kill":
			b.kills++
			delete(b.tracks, e.Target)
		}
	}

	if !b.baseSet {
		b.base, b.baseSet = b.P, true
		b.startAlive = o.Alive
	}
	if b.startAlive-o.Alive-b.kills > 0 {
		b.otherDead++
	} else {
		b.otherDead = 0
	}
	if b.otherDead >= 5 {
		b.notPractice = true
	}
	if on := b.base.Practice && !b.notPractice && now > 20; on != b.pracOn {
		b.pracOn = on
		b.P = b.base
		if on {
			b.P = squadParams(b.base)
		}
	}
	if open := b.base.Opening && !b.notPractice && !b.pracOn && b.startAlive-o.Alive-b.kills <= 0; open != b.openOn {
		b.openOn = open
		b.P = b.base
		if open {
			b.P.LaserFirst, b.P.LaserSeek, b.P.LootRadius = true, 240, 150
		} else if b.pracOn {
			b.P = squadParams(b.base)
		}
	}
	// End game in practice: when the circle gets small, stop fighting the whole squad and hide
	// (rule from SandeepMadhavarapu/BattleRoyale b532564, MIT).
	if b.pracOn && !b.endGhostOn && b.base.EndGhost > 0 && o.Zone.NextRadius <= b.base.EndGhost {
		b.endGhostOn = true
		b.P = b.base
		b.P.Practice = false
	}

	z := &o.Zone
	if b.holdStart != z.ShrinkStart {
		b.holdStart = z.ShrinkStart
		b.hold = circle{z.Center[0], z.Center[1], z.Radius}
	} else if now <= z.ShrinkStart {
		b.hold = circle{z.Center[0], z.Center[1], z.Radius}
	}
}

func (b *Brain) zoneAt(z *Zone, t float64) circle {
	if t <= z.ShrinkStart || z.ShrinkEnd <= z.ShrinkStart {
		return b.hold
	}
	f := clamp((t-z.ShrinkStart)/(z.ShrinkEnd-z.ShrinkStart), 0, 1)
	return circle{b.hold.X + (z.NextCenter[0]-b.hold.X)*f, b.hold.Y + (z.NextCenter[1]-b.hold.Y)*f, b.hold.R + (z.NextRadius-b.hold.R)*f}
}

func (b *Brain) curZone(z *Zone) circle {
	live := circle{z.Center[0], z.Center[1], z.Radius}
	if b.now <= z.ShrinkStart {
		return live
	}
	if math.Abs(live.R-b.hold.R) > 0.01 || math.Abs(live.X-b.hold.X) > 0.01 {
		return live
	}
	return b.zoneAt(z, b.now)
}

func (b *Brain) zoneDeadline(z *Zone, x, y, pad float64) float64 {
	if hyp(x-z.NextCenter[0], y-z.NextCenter[1]) <= z.NextRadius-pad {
		return math.Inf(1)
	}
	for k := 0; k <= 24; k++ {
		t := z.ShrinkStart + (z.ShrinkEnd-z.ShrinkStart)*float64(k)/24
		c := b.zoneAt(z, t)
		if hyp(x-c.X, y-c.Y) > c.R-pad {
			return t
		}
	}
	return z.ShrinkEnd
}

func (b *Brain) zonePenalty(z *Zone, x, y float64) float64 {
	c := b.curZone(z)
	p := 0.0
	if d := hyp(x-c.X, y-c.Y) - c.R; d > -5 {
		p += 150 + 5*(d+5)
	}
	if dn := hyp(x-z.NextCenter[0], y-z.NextCenter[1]) - z.NextRadius; dn > -5 {
		soon := clamp(1-(z.ShrinkStart-b.now)/25, 0.1, 1)
		p += soon * (25 + 0.8*(dn+5))
	}
	return p
}

func (b *Brain) zoneGoal(z *Zone, x, y float64) (float64, float64) {
	margin := clamp(0.3*z.NextRadius, 3, 30)
	if z.NextRadius <= 30 {
		return z.NextCenter[0], z.NextCenter[1]
	}
	dx, dy := x-z.NextCenter[0], y-z.NextCenter[1]
	d := hyp(dx, dy)
	if d < 1 {
		return x, y
	}
	r := math.Max(0, z.NextRadius-margin-5)
	if d <= r {
		return x, y
	}
	return z.NextCenter[0] + dx/d*r, z.NextCenter[1] + dy/d*r
}

func hasWeapon(ws []*WeaponState, kind string) (int, *WeaponState) {
	for i, w := range ws {
		if w != nil && w.Kind == kind {
			return i, w
		}
	}
	return -1, nil
}

func holds(list []string, kind string) bool {
	for _, k := range list {
		if k == kind {
			return true
		}
	}
	return false
}

var upgradeOrder = []string{
	"plating", "armor", "gun_damage", "nanites", "gun_rate", "lightning_efficiency", "lightning_range", "laser_damage",
	"servos", "laser_charge", "gun_mag", "gun_velocity", "gyros", "lightning_chain", "reactor", "capacitor",
	"mortar_reload", "mortar_splash", "mortar_velocity", "sensors", "jets", "laser_pierce",
}

var upgradeOrderLaser = []string{
	"plating", "armor", "laser_damage", "laser_charge", "nanites", "gun_damage", "gun_rate", "lightning_efficiency", "lightning_range",
	"servos", "gun_mag", "gun_velocity", "gyros", "lightning_chain", "reactor", "capacitor",
	"mortar_reload", "mortar_splash", "mortar_velocity", "sensors", "jets", "laser_pierce",
}

var upgradeOrderNano = []string{
	"plating", "armor", "nanites", "laser_damage", "laser_charge", "gun_damage", "gun_rate", "lightning_efficiency", "lightning_range",
	"servos", "gun_mag", "gun_velocity", "gyros", "lightning_chain", "reactor", "capacitor",
	"mortar_reload", "mortar_splash", "mortar_velocity", "sensors", "jets", "laser_pierce",
}

var upgradeOrderSpeed = []string{
	"plating", "armor", "servos", "jets", "reactor", "laser_damage", "laser_charge", "nanites", "gun_damage", "gun_rate", "lightning_efficiency", "lightning_range",
	"gun_mag", "gun_velocity", "gyros", "lightning_chain", "capacitor",
	"mortar_reload", "mortar_splash", "mortar_velocity", "sensors", "laser_pierce",
}

func pickUpgrade(of *Offer, laserUp, nanoUp, speedUp bool) int {
	order := upgradeOrder
	if laserUp {
		order = upgradeOrderLaser
	}
	if nanoUp {
		order = upgradeOrderNano
	}
	if speedUp {
		order = upgradeOrderSpeed
	}
	best, bestRank := 0, 1<<30
	for i, c := range of.Choices {
		rank := len(order)
		for r, id := range order {
			if id == c.ID {
				rank = r
				break
			}
		}
		if rank < bestRank {
			best, bestRank = i, rank
		}
	}
	return best
}

var weaponRank = map[string]int{"lightning": 3, "laser": 2, "mortar": 1}

func (b *Brain) step(o *Obs, act *Action) {
	me := &o.Self
	b.now = o.T
	now := o.T
	b.observe(o)
	st := &me.Stats
	x, y, zf := me.Pos[0], me.Pos[1], me.Pos[2]
	z := &o.Zone
	hpFrac := me.HP / math.Max(1, me.MaxHP)

	if o.Offer != nil && len(o.Offer.Choices) > 0 {
		p := pickUpgrade(o.Offer, b.P.LaserUp, b.P.NanoUp, b.P.SpeedUp)
		act.Pick = &p
	}

	_, gun := hasWeapon(me.Weapons, "gun")
	_, light := hasWeapon(me.Weapons, "lightning")
	_, laser := hasWeapon(me.Weapons, "laser")
	late := (z.NextRadius <= 70 && !b.endGhostOn) || o.Alive <= 3

	var seen []*Track
	var target, hidden, shy *Track
	bestScore := math.Inf(1)
	maxT, hpw := 215.0, 60.0/1400
	if b.P.MaxTarget > 0 {
		maxT = b.P.MaxTarget
	}
	if b.P.HPw > 0 {
		hpw = b.P.HPw
	}
	for _, t := range b.tracks {
		if now-t.Seen > 2.5 {
			continue
		}
		t.Engage = false
		if t.Seen == now {
			seen = append(seen, t)
		} else {
			seen = append(seen, t)
			t.LOS = false
			continue
		}
		toMe := math.Atan2(y-t.Pos[1], x-t.Pos[0])
		aimed := math.Abs(angDiff(t.Aim[0], toMe)) < 0.22
		switch {
		case b.P.Ghost:

			t.Engage = t.Dist < b.P.EngageDist || now-t.HitMe < 4 || (aimed && t.Charging && t.Dist < 120) ||
				(t.HP < 350 && t.Dist < 120) || o.Alive <= 2
			off := math.Abs(angDiff(t.Aim[0], toMe))

			if b.P.Vulture > 0 && hpFrac > 0.75 && t.HP < b.P.Vulture*t.MaxHP && off > 0.6 && t.Dist < 165 {
				t.Engage = true
			}
			if b.P.Alert > 0 && t.LOS && t.Dist < b.P.Alert && (t.Closing > 3 || aimed) {
				t.Engage = true
			}
			if b.P.Shy > 0 && t.LOS && t.Dist < b.P.Shy && t.Dist >= b.P.EngageDist && (t.Closing > 3 || aimed) && !t.Engage &&
				(shy == nil || t.Dist < shy.Dist) {
				shy = t
			}

			if b.P.Ambush > 0 && hpFrac > 0.85 && off > 1.75 && t.Dist < b.P.Ambush {
				t.Engage = true
			}
			if b.P.Odds > 0 && !t.Engage && t.LOS && hpFrac > 0.5 && t.Dist < b.reach(me) && b.odds(me, t, now, true) >= b.P.Odds {
				t.Engage = true
			}
		case b.P.AllFoes && t.Dist < maxT:
			t.Engage = true
		case t.Dist < b.P.EngageDist, now-t.HitMe < 6, late:
			t.Engage = true
		case t.Closing > 3 && t.Dist < 180:
			t.Engage = true
		case t.HP < 0.4*t.MaxHP && t.Dist < 160:
			t.Engage = true
		case aimed && t.Dist < 190:
			t.Engage = true
		}
		if t.Engage && !t.LOS && t.Dist < maxT && (hidden == nil || t.Dist < hidden.Dist) {
			hidden = t
		}
		if !t.Engage || !t.LOS || t.Dist > maxT {
			continue
		}
		s := t.Dist + hpw*t.HP
		if b.P.LowBonus > 0 && t.HP <= 0.3*t.MaxHP {
			s -= b.P.LowBonus
		}
		if b.P.Stick > 0 && t.ID == b.stickID && now-b.stickAt < 2 {
			s -= b.P.Stick
		}
		if b.P.Backstab > 0 && b.showsBack(t, x, y) {
			s -= b.P.Backstab
		}
		if now-t.HitMe < 4 {
			s -= 40
		}
		if t.Closing > 3 {
			s -= 15
		}
		if s < bestScore {
			bestScore, target = s, t
		}
	}
	if target != nil {
		b.stickID, b.stickAt = target.ID, now
	}
	foe := target
	if foe == nil && b.P.Peek {
		foe = hidden
	}
	if foe == nil && b.P.Pursue > 0 && hidden != nil && hidden.HP < b.P.Pursue*hidden.MaxHP {
		foe = hidden
	}
	danger := false
	crowd := 0
	for _, t := range seen {
		if t.Engage || now-t.HitMe < 4 {
			danger = true
		}
		if t.Seen == now && (t.Engage || (t.LOS && t.Dist < 150)) {
			crowd++
		}
	}
	if danger || now-b.lastHurt < 3 {
		b.lastThreatAt = now
	}

	finishing := target != nil && target.Dist < 120 && (target.HP < 200 || target.HP < me.HP*0.5)
	_, myLaser := hasWeapon(me.Weapons, "laser")
	outgunned := target != nil && holds(target.Weapons, "laser") && myLaser == nil && target.Dist > 55 && hpFrac < 0.8
	swamped := b.P.CrowdHide && (crowd >= 3 || (crowd == 2 && hpFrac < 0.65) || outgunned)
	if b.P.OddsFlee > 0 && target != nil && !finishing && b.odds(me, target, now, false) < b.P.OddsFlee {
		b.oddsFleeUntil = now + 5
	}
	if now < b.oddsFleeUntil && !finishing {
		swamped = true
	}
	if !b.retreat && ((hpFrac < b.P.RetreatHP && now-b.lastThreatAt < 2) || swamped) {
		b.retreat = true
		b.coverAt = -1
	}
	if b.retreat && !swamped && (hpFrac > b.P.RecoverHP || (now-b.lastThreatAt > 7 && hpFrac > 0.55)) {
		b.retreat = false
	}
	if b.P.LoseRule {
		near := 0
		for _, t := range seen {
			if t.Seen == now && t.Dist < 160 {
				near++
			}
		}
		losing := (near >= 2 && hpFrac < 0.55) || (near >= 3 && hpFrac < 0.8)
		if target != nil {
			tf := target.HP / math.Max(1, target.MaxHP)
			if hpFrac < 0.35 && tf > hpFrac+0.1 {
				losing = true
			}
			if tf < 0.25 && hpFrac > 0.2 && near <= 2 {
				losing = false
			}
		}
		if losing {
			b.loseAt = now
		}
		b.retreat = now-b.loseAt < 1.5 && b.loseAt > 0
	}

	exposed := false
	for _, t := range seen {
		if t.Seen == now && t.LOS && t.Dist < 230 {
			exposed = true
		}
	}
	for _, t := range seen {
		if t.Seen == now && t.Dist < 45 {
			exposed = false
		}
	}

	var mx, my float64
	boost := false
	mode := "idle"
	var farm *Track
	var farmPos *Vec3

	zgx, zgy := b.zoneGoal(z, x, y)
	zdist := hyp(zgx-x, zgy-y)
	tNeed := zdist*1.45/st.WalkSpeed + 4
	deadline := b.zoneDeadline(z, x, y, 8)
	zoneMust := !me.InZone || now+tNeed+3 >= deadline
	zoneShould := now+tNeed+14 >= deadline

	escJump := false
	switch {
	case b.escape(me, &mx, &my, &boost, &escJump):
		mode = "escape"
	case now < b.unstuckUntil:
		mx, my = b.unstuckDir[0], b.unstuckDir[1]
		mode = "unstuck"
	case b.dodgeShell(o, &mx, &my):
		boost = me.Energy > 15
		mode = "dodge"
	case zoneMust && zdist > 2:
		mx, my = b.travel(me, zgx, zgy)
		boost = (!me.InZone || now+tNeed >= deadline) && me.Energy > 5
		mode = "zone"
	case b.retreat && !finishing && !late:
		if b.coverMove(o, seen, &mx, &my) {
			boost = me.Energy > 10 && (mx != 0 || my != 0)
			mode = "cover"
		} else if target != nil {
			mx, my = b.fightMove(o, target, seen, 1)
			mode = "cornered"
		} else {
			mx, my = b.travel(me, zgx, zgy)
			if b.npcHurt > 0 && now-b.npcHurt < 3 && mx == 0 && my == 0 {

				side := 1.0
				if math.Mod(now, 3) > 1.5 {
					side = -1
				}
				mx, my = -math.Sin(b.npcBearing)*side, math.Cos(b.npcBearing)*side
			}
			mode = "flee"
		}
	case target == nil && now-b.lastHurt < 2.5 && !late && b.P.Ghost && b.hideFromBearing(o, seen, &mx, &my):
		boost = me.Energy > 30 && (mx != 0 || my != 0)
		mode = "cover"
	case now < b.P.EarlyHide && exposed && !finishing && b.coverMove(o, seen, &mx, &my):
		boost = me.Energy > 20 && (mx != 0 || my != 0)
		mode = "earlyhide"
	case foe == nil && shy != nil && !late && b.coverMove(o, seen, &mx, &my):
		boost = me.Energy > 40 && (mx != 0 || my != 0)
		mode = "shy"
	case b.P.Evade && foe != nil && !late && !finishing && foe.Dist > b.P.EngageDist*1.2 && foe.HP > 0.45*foe.MaxHP &&
		b.coverMove(o, seen, &mx, &my):
		boost = me.Energy > 30 && (mx != 0 || my != 0)
		mode = "evade"
	case foe != nil:

		ready := gun != nil && gun.Ammo > 0 && gun.Reload <= 0.25 && foe.Dist <= gun.Stats.Range-10
		hot := laser != nil && laser.Charge >= laser.Stats.Charge-0.3 && foe.Dist <= laser.Stats.Range-5
		if hot || (light != nil && foe.Dist <= light.Stats.Range+6 && me.Energy >= light.Stats.Energy*3) {
			ready = true
		}
		if !b.P.Peek && (light != nil || laser != nil) {
			ready = true
		}
		wantLos := -1.0
		if ready {
			wantLos = 1
		}
		if b.P.Peek && !hot {
			for _, t := range seen {
				if t.Seen == now && t.Charging && math.Abs(angDiff(t.Aim[0], math.Atan2(y-t.Pos[1], x-t.Pos[0]))) < 0.3 {
					wantLos = -1.6
				}
			}
		}
		finHP := 0.38
		if b.P.FinishHP > 0 {
			finHP = b.P.FinishHP
		}
		if keep := b.P; b.P.Finish > 0 && foe.HP < finHP*foe.MaxHP && hpFrac > 0.42 && crowd <= 2 {
			b.P.GunRange, b.P.LaserRange, b.P.LightRange = b.P.Finish, b.P.Finish, b.P.Finish
			mx, my = b.fightMove(o, foe, seen, wantLos)
			b.P = keep
			if b.P.FinishRush && foe.Dist > 2*b.P.Finish && me.Energy > 30 {
				boost = true
			}
		} else {
			mx, my = b.fightMove(o, foe, seen, wantLos)
		}
		if light != nil && foe.Dist > 50 && me.Energy > 55 && wantLos > 0 {
			boost = true
		}
		mode = "fight"
		if b.P.FightBoost > 0 && me.Energy > b.P.FightBoost && (laser == nil || laser.Charge <= 0) && (light == nil || me.Energy > 60) {
			boost = true
		}
		if b.P.FightLoot > 0 {
			r := b.P.FightLoot
			if hpFrac <= 0.5 {
				r *= 0.57
			}
			var bc *Vec3
			for _, c := range b.cubes {
				if d := hyp(c.Pos[0]-x, c.Pos[1]-y); d < r {
					r = d
					cp := c.Pos
					bc = &cp
				}
			}
			if bc != nil {
				mx, my = b.travel(me, bc[0], bc[1])
				mode = "fight:cube"
			}
		}
	case zoneShould && zdist > 2:
		mx, my = b.travel(me, zgx, zgy)
		mode = "zone"
	default:
		lc := b.chooseLoot(o, hpFrac, danger)
		if lc != nil {
			mode = "loot:" + lc.kind
			switch lc.kind {
			case "npc":
				t := b.npcs[lc.id]
				if t != nil && now-t.Seen < 1 && t.LOS && t.Dist < 75 {
					farm = t
					mx, my = b.fightMove(o, t, seen, 1)
				} else {
					mx, my = b.travel(me, lc.pos[0], lc.pos[1])
				}
			case "crate":
				d := hyp(lc.pos[0]-x, lc.pos[1]-y)
				cp := Vec3{lc.pos[0], lc.pos[1], lc.pos[2] + b.Cfg.Crates.Height*0.5}
				if d < 50 && b.W.Clear(Vec3{x, y, zf + st.MuzzleHeight}, cp) {
					farmPos = &cp
					if d < 12 {
						mx, my = (x-lc.pos[0])/math.Max(d, 0.1), (y-lc.pos[1])/math.Max(d, 0.1)
					}
				} else {
					mx, my = b.travel(me, lc.pos[0], lc.pos[1])
				}
			default:
				mx, my = b.travel(me, lc.pos[0], lc.pos[1])
				if lc.kind == "drop" && hyp(lc.pos[0]-x, lc.pos[1]-y) < 2.5 {
					mx, my = 0, 0
				}
			}
		} else if zdist > 3 && (!b.P.Ghost || z.NextRadius <= 30) {
			mx, my = b.travel(me, zgx, zgy)
			mode = "zone-walk"
		} else if b.P.Ghost {

			if hx, hy, ok := b.home(o); ok && hyp(hx-x, hy-y) > 3 {
				mx, my = b.travel(me, hx, hy)
				if !danger && now-b.lastHurt > 6 && !b.openOn {
					q := math.Min(1, 4.6/st.WalkSpeed)
					mx, my = mx*q, my*q
				}
				mode = "home"
			}
		} else if b.P.Explore && !late && me.Cubes < b.cubeCap() && hpFrac > 0.7 && !danger {
			mx, my = b.exploreMove(o)
			mode = "explore"
		}
	}

	if escJump {
		act.Jump = true
	}
	if b.P.Dodge && mode != "unstuck" && mode != "escape" {
		if dx, dy, lz := b.dodge(o); dx != 0 || dy != 0 {
			mx, my = mx*0.4+dx*0.9, my*0.4+dy*0.9
			if l := hyp(mx, my); l > 1 {
				mx, my = mx/l, my/l
			}
			if lz && me.Energy > 25 {
				boost = true
			}
			if b.P.DodgeBoost > 0 && me.Energy > b.P.DodgeBoost {
				boost = true
			}
		}
	}
	if b.P.ThreatBoost > 0 && me.Energy > b.P.ThreatBoost && (foe != nil || now-b.lastHurt < 2.5) && (light == nil || me.Energy > 60) {
		boost = true
	}

	if mode == "escape" {
		b.stuckSince = -1
	} else if (mx != 0 || my != 0) && hyp(me.Vel[0], me.Vel[1]) < 1.2 && me.Grounded {
		if b.stuckSince < 0 {
			b.stuckSince = now
		} else if now-b.stuckSince > 0.7 {
			a := b.rng.Float64() * 2 * math.Pi
			b.unstuckDir = [2]float64{math.Cos(a), math.Sin(a)}
			b.unstuckUntil = now + 0.5
			b.stuckSince = -1
			b.path = nil
			act.Jump = true
		}
	} else {
		b.stuckSince = -1
	}

	muzzle := Vec3{x + me.Vel[0]*lat, y + me.Vel[1]*lat, zf + st.MuzzleHeight}
	aim := [2]float64{me.Aim[0], 0}
	haveAim := false
	var tp, tv Vec3
	var tr, th float64
	shootable := false
	isMech := false
	defend := false
	if target == nil && b.npcHurt > 0 && now-b.npcHurt < 3 {

		var worst *Track
		bestOff := 0.7
		for _, n := range b.npcs {
			if now-n.Seen > 0.5 || !n.LOS || n.Dist > 150 {
				continue
			}
			off := math.Abs(angDiff(math.Atan2(n.Pos[1]-y, n.Pos[0]-x), b.npcBearing))
			if now-n.HitMe < 3 {
				off = 0
			}
			if off < bestOff {
				bestOff, worst = off, n
			}
		}
		if worst != nil {
			farm, farmPos, defend = worst, nil, true
		}
	}
	if b.P.Econ && target == nil && farm == nil && farmPos == nil && gun != nil && !b.retreat {

		reach := 110.0
		for _, t := range b.tracks {
			if now-t.Seen < 4 {
				reach = 50
			}
		}
		for _, c := range b.crates {
			if until, ok := b.bad[c.ID]; ok && until > now {
				continue
			}
			d := hyp(c.Pos[0]-x, c.Pos[1]-y)
			cp := Vec3{c.Pos[0], c.Pos[1], c.Pos[2] + b.Cfg.Crates.Height*0.5}
			if d < reach && d > 6 && b.W.Clear(muzzle, cp) {
				reach = d
				farmPos = &cp
			}
		}
	}
	switch {
	case target != nil:
		sc := b.scaleOf(target.Cubes)
		tr, th = b.Cfg.Mech.Radius*sc, b.Cfg.Mech.Height*sc
		tp, tv = target.Pos, target.Vel
		tp[2] += th * 0.5
		if !b.W.Clear(muzzle, tp) {
			tp[2] = target.Pos[2] + th*0.85
		}
		shootable, isMech = true, true
	case farm != nil:
		tr, th = b.Cfg.Npc.Radius, b.Cfg.Npc.Height
		tp, tv = farm.Pos, farm.Vel
		tp[2] += th * 0.5
		shootable = true
	case farmPos != nil:
		tr, th = b.Cfg.Crates.Radius, b.Cfg.Crates.Height
		tp = *farmPos
		shootable = true
	}
	if !isMech && target == nil && (b.retreat || hpFrac < 0.5) && !b.P.Econ && !defend {
		shootable = false
	}

	if shootable {
		dist := hyp(tp[0]-x, tp[1]-y)
		grounded := math.Abs(tv[2]) < 0.5

		var gy, gp float64
		gunOK := false
		if gun != nil {
			gy, gp = b.lead(muzzle, tp, tv, gun.Stats.Speed, grounded)
			gunOK = dist <= gun.Stats.Range*0.96
		}

		dp := Vec3{tp[0] + tv[0]*(lat+0.05), tp[1] + tv[1]*(lat+0.05), tp[2]}
		hy := math.Atan2(dp[1]-muzzle[1], dp[0]-muzzle[0])
		hp := math.Atan2(dp[2]-muzzle[2], hyp(dp[0]-muzzle[0], dp[1]-muzzle[1]))
		aim = [2]float64{gy, gp}
		if gun == nil {
			aim = [2]float64{hy, hp}
		}
		haveAim = true
		tolY := math.Atan2(tr*0.75, math.Max(dist, 1))
		tolP := math.Atan2(th*0.4, math.Max(dist, 1))
		slew := func(wantY, wantP float64) bool {
			ey := math.Max(0, math.Abs(angDiff(wantY, me.Aim[0]))-st.TurnRate*0.05)
			ep := math.Max(0, math.Abs(wantP-me.Aim[1])-st.PitchRate*0.05)
			return ey < tolY && ep < tolP
		}
		for i, w := range me.Weapons {
			if w == nil || i > 1 {
				continue
			}
			switch w.Kind {
			case "gun":
				if gunOK && w.Ammo > 0 && w.Reload <= 0 && slew(gy, gp) {
					act.Fire[i] = true
				}
			case "lightning":
				cone := w.Stats.ConeDeg * math.Pi / 180
				if dist < w.Stats.Range-1.5 && math.Abs(angDiff(hy, me.Aim[0])) < cone*0.5-0.05 && me.Energy >= w.Stats.Energy {
					act.Fire[i] = true
				}
			case "laser":
				if !isMech || dist > w.Stats.Range*0.97 {
					break
				}
				full := w.Stats.Charge
				if b.P.Lethal && w.Charge >= w.Stats.MinCharge && w.Charge < full-0.051 {
					dmg := w.Stats.Damage * (w.Stats.MinFrac + (1-w.Stats.MinFrac)*w.Charge/full) *
						(1 + b.Cfg.Cubes.DamagePerCube*float64(min(me.Cubes, b.Cfg.Cubes.StatCap)))
					if holds(target.Weapons, "mortar") {
						dmg *= 1 + w.Stats.Cookoff
					}
					lk := 0.8
					if b.P.LethalK > 0 {
						lk = b.P.LethalK
					}
					if b.P.Backstab > 0 && b.showsBack(target, x, y) {
						dmg *= b.rearMult()
					}
					if dmg*lk >= target.HP && math.Abs(angDiff(hy, me.Aim[0])) < tolY*0.9 && math.Abs(hp-me.Aim[1]) < tolP {
						aim = [2]float64{hy, hp}
						act.Fire[i] = false
						break
					}
				}
				if w.Charge < full-0.051 {
					act.Fire[i] = true
					if w.Charge > full*0.45 {
						aim = [2]float64{hy, hp}
					}
				} else {
					aim = [2]float64{hy, hp}
					ey := math.Abs(angDiff(hy, me.Aim[0]))
					ep := math.Abs(hp - me.Aim[1])
					if ey < tolY*0.9 && ep < tolP {
						act.Fire[i] = false
					} else {
						act.Fire[i] = true
					}
				}
			case "mortar":
				if !isMech || w.Cooldown > 0.25 || dist < 16 || dist > 200 {
					break
				}
				tube := Vec3{muzzle[0], muzzle[1], zf + st.Height + 0.5}
				foot := Vec3{tp[0], tp[1], tp[2] - th*0.5}
				if my2, mp, ok := b.mortar(tube, foot, tv, w.Stats.Speed, w.Stats.Gravity, st); ok {

					if gun == nil || gun.Reload > 0 || gun.Ammo == 0 || !gunOK {
						aim = [2]float64{my2, mp}
					}
					ey := math.Max(0, math.Abs(angDiff(my2, me.Aim[0]))-st.TurnRate*0.05)
					ep := math.Max(0, math.Abs(mp-me.Aim[1])-st.PitchRate*0.05)
					if ey < 0.04 && ep < 0.03 {
						act.Fire[i] = true
					}
				}
			}
		}
	} else {

	}

	if target == nil && foe != nil {

		for i, w := range me.Weapons {
			if w != nil && i < 2 && w.Kind == "laser" && w.Cooldown <= 0.05 && foe.Dist < w.Stats.Range && w.Charge < w.Stats.Charge+w.Stats.HoldS-0.35 {
				act.Fire[i] = true
			}
		}
	}

	if !haveAim {

		var watch *Track
		for _, t := range seen {
			if t.Engage || now-t.HitMe < 6 {
				if watch == nil || t.Dist < watch.Dist {
					watch = t
				}
			}
		}
		switch {
		case watch != nil:
			aim[0] = math.Atan2(watch.Pos[1]-y, watch.Pos[0]-x)
		case b.npcHurt > 0 && now-b.npcHurt < 2.5:
			aim[0] = b.npcBearing
		case now-b.lastHurt < 2.5:
			aim[0] = b.hurtBearing
		case mx != 0 || my != 0:
			aim[0] = math.Atan2(my, mx) + 1.15*math.Sin(now*1.3)
		default:
			aim[0] = me.Aim[0] + 0.35
		}
		aim[1] = 0
	}
	aim[1] = clamp(aim[1], st.MinPitch, st.MaxPitch)
	act.Aim = &aim

	if gun != nil && gun.Reload <= 0 && gun.Ammo < gun.Stats.Mag {
		if target == nil && farm == nil && farmPos == nil && (gun.Ammo <= gun.Stats.Mag*0.6 || now-b.lastThreatAt > 3) {
			act.Reload = true
		}
	}

	slot1 := ""
	if len(me.Weapons) > 1 && me.Weapons[1] != nil {
		slot1 = me.Weapons[1].Kind
	}
	for i := range o.Pickups {
		p := &o.Pickups[i]
		if hyp(p.Pos[0]-x, p.Pos[1]-y) > b.Cfg.Loot.PickupRadius-0.3 || math.Abs(p.Pos[2]-zf) > 3.6 {
			continue
		}
		if !b.wantPickup(me, p.Kind) {
			continue
		}
		id := p.ID
		act.Take = &id
		if slot1 != "" && len(me.Weapons) > 0 && me.Weapons[0] != nil {
			if slot1 == "laser" && me.Weapons[1].Charge > 0 {
				act.Take = nil
				break
			}
			one := 1
			act.Drop = &one
		}
		break
	}

	act.Move = [2]float64{mx, my}
	act.Boost = boost && (mx != 0 || my != 0)
	b.Mode = mode
}

func (b *Brain) escape(me *Self, mx, my *float64, boost, jump *bool) bool {
	x, y, zf := me.Pos[0], me.Pos[1], me.Pos[2]
	w := b.W
	if me.Grounded {
		if zf-w.GroundAt(x, y) > 3 && w.Obst[w.vidx(x, y)] {
			b.escOK, b.escFlying = false, false
			return false
		}
		if !w.OffMain(x, y) {
			b.escOK, b.escFlying = false, false
			return false
		}
		b.escFlying = false
		if !b.escOK || b.now-b.escAt > 6 {
			b.escLaunch, b.escLand, b.escOK = w.EscapePlan(x, y)
			b.escAt = b.now
			if !b.escOK {
				return false
			}
		}
		if hyp(b.escLaunch[0]-x, b.escLaunch[1]-y) > 2.5 {
			*mx, *my = b.travel(me, b.escLaunch[0], b.escLaunch[1])
			return true
		}
		if me.Energy < math.Min(70, me.MaxEnergy*0.7) {
			*mx, *my = 0, 0
			return true
		}
		b.escFlying = true
		*jump = true
	} else if !b.escFlying {
		return false
	}
	dx, dy := b.escLand[0]-x, b.escLand[1]-y
	d := hyp(dx, dy)
	if d < 1.5 {
		return true
	}
	*mx, *my = dx/d, dy/d
	need := -1e9
	for t := 0.0; t <= math.Min(d, 30); t += 1 {
		need = math.Max(need, w.TopAt(x+*mx*t, y+*my*t))
	}
	*boost = zf < need+2.5 && me.Energy > 1
	return true
}

func (b *Brain) wantPickup(me *Self, kind string) bool {
	if kind == "gun" {
		return len(me.Weapons) > 0 && me.Weapons[0] == nil
	}
	if i, _ := hasWeapon(me.Weapons, kind); i >= 0 {
		return false
	}
	if kind == "mortar" && b.P.NoMortar {
		return false
	}
	rank := func(k string) int {
		if b.P.LaserFirst && k == "laser" {
			return 4
		}
		return weaponRank[k]
	}
	cur := 0
	if len(me.Weapons) > 1 && me.Weapons[1] != nil {
		cur = rank(me.Weapons[1].Kind)
	}
	return rank(kind) > cur
}

func (b *Brain) lead(muzzle, tp, tv Vec3, speed float64, grounded bool) (float64, float64) {
	if speed <= 0 {
		speed = 252
	}
	t := 0.0
	var px, py, pz float64
	for k := 0; k < 4; k++ {
		tt := lat + t
		px, py, pz = tp[0]+tv[0]*tt, tp[1]+tv[1]*tt, tp[2]
		if !grounded {
			pz += tv[2] * tt * 0.5
		}
		d := math.Sqrt((px-muzzle[0])*(px-muzzle[0]) + (py-muzzle[1])*(py-muzzle[1]) + (pz-muzzle[2])*(pz-muzzle[2]))
		t = math.Max(0, d-3) / speed
	}
	return math.Atan2(py-muzzle[1], px-muzzle[0]), math.Atan2(pz-muzzle[2], hyp(px-muzzle[0], py-muzzle[1]))
}

func (b *Brain) mortar(tube, foot, tv Vec3, v, g float64, st *MechStats) (yaw, pitch float64, ok bool) {
	if v <= 0 || g <= 0 {
		return 0, 0, false
	}
	t := 0.0
	for k := 0; k < 3; k++ {
		px, py := foot[0]+tv[0]*(lat+t), foot[1]+tv[1]*(lat+t)
		dx := hyp(px-tube[0], py-tube[1])
		dz := foot[2] - tube[2]
		disc := v*v*v*v - g*(g*dx*dx+2*dz*v*v)
		if disc < 0 || dx < 1 {
			return 0, 0, false
		}
		pitch = math.Atan((v*v - math.Sqrt(disc)) / (g * dx))
		yaw = math.Atan2(py-tube[1], px-tube[0])
		t = dx / (v * math.Cos(pitch))
	}
	if pitch < st.MinPitch || pitch > st.MaxPitch {
		return 0, 0, false
	}

	c, s := math.Cos(pitch), math.Sin(pitch)
	for tt := 0.1; tt < t*0.9; tt += 0.1 {
		d := v * c * tt
		px, py := tube[0]+math.Cos(yaw)*d, tube[1]+math.Sin(yaw)*d
		pz := tube[2] + v*s*tt - 0.5*g*tt*tt
		if b.W.Solid(px, py, pz) {
			return 0, 0, false
		}
	}
	return yaw, pitch, true
}

func (b *Brain) dodgeShell(o *Obs, mx, my *float64) bool {
	me := &o.Self
	g := b.Cfg.Weapons["mortar"].Gravity
	if g <= 0 {
		g = 22
	}
	rad := b.Cfg.Weapons["mortar"].SplashRadius + me.Stats.Radius + 2
	for i := range o.Projectiles {
		p := &o.Projectiles[i]
		if p.Kind != "mortar" {
			continue
		}
		px, py, pz := p.Pos[0], p.Pos[1], p.Pos[2]
		vz := p.Vel[2]
		for t := 0.0; t < 4; t += 0.05 {
			px += p.Vel[0] * 0.05
			py += p.Vel[1] * 0.05
			pz += vz * 0.05
			vz -= g * 0.05
			if b.W.Solid(px, py, pz) {
				d := hyp(px-me.Pos[0], py-me.Pos[1])
				if d < rad && t < 2.5 {
					dx, dy := me.Pos[0]-px, me.Pos[1]-py
					if d < 0.5 {
						dx, dy = -p.Vel[1], p.Vel[0]
						d = hyp(dx, dy)
					}
					dx, dy = dx/d, dy/d

					for _, rot := range []float64{0, 0.7, -0.7, 1.4, -1.4} {
						c, s := math.Cos(rot), math.Sin(rot)
						ex, ey := dx*c-dy*s, dx*s+dy*c
						if b.W.LineWalkable(me.Pos[0], me.Pos[1], me.Pos[0]+ex*6, me.Pos[1]+ey*6) {
							*mx, *my = ex, ey
							return true
						}
					}
				}
				break
			}
		}
	}
	return false
}

func (b *Brain) travel(me *Self, gx, gy float64) (float64, float64) {
	x, y := me.Pos[0], me.Pos[1]
	d := hyp(gx-x, gy-y)
	if d < 0.8 {
		return 0, 0
	}
	unit := func(tx, ty float64) (float64, float64) {
		l := hyp(tx-x, ty-y)
		if l < 1e-6 {
			return 0, 0
		}
		return (tx - x) / l, (ty - y) / l
	}

	if me.Pos[2]-b.W.GroundAt(x, y) > 3 && b.W.Obst[b.W.vidx(x, y)] {
		return unit(gx, gy)
	}
	if d < 45 && b.W.LineWalkable(x, y, gx, gy) {
		return unit(gx, gy)
	}
	if b.path == nil || hyp(gx-b.pathGoal[0], gy-b.pathGoal[1]) > 5 || b.now-b.pathAt > 2.5 {
		b.path, _ = b.W.Path(x, y, gx, gy, 22000)
		b.pathGoal = [2]float64{gx, gy}
		b.pathAt = b.now
		b.pathI = 0
	}
	if len(b.path) == 0 {
		return unit(gx, gy)
	}

	bi, bd := b.pathI, math.Inf(1)
	for i := b.pathI; i < len(b.path) && i < b.pathI+10; i++ {
		px, py := b.W.CellPos(int(b.path[i]))
		if dd := hyp(px-x, py-y); dd < bd {
			bi, bd = i, dd
		}
	}
	b.pathI = bi
	if bd > 12 {
		b.path = nil
		return unit(gx, gy)
	}
	for i := min(bi+14, len(b.path)-1); i > bi; i-- {
		px, py := b.W.CellPos(int(b.path[i]))
		if b.W.LineWalkable(x, y, px, py) {
			if i == len(b.path)-1 && hyp(px-gx, py-gy) < 4 && b.W.LineWalkable(x, y, gx, gy) {
				return unit(gx, gy)
			}
			return unit(px, py)
		}
	}
	if bi+1 < len(b.path) {
		px, py := b.W.CellPos(int(b.path[bi+1]))
		return unit(px, py)
	}
	return unit(gx, gy)
}

func (b *Brain) prefRange(me *Self, t *Track) float64 {
	_, light := hasWeapon(me.Weapons, "lightning")
	_, laser := hasWeapon(me.Weapons, "laser")
	r := b.P.GunRange
	switch {
	case light != nil && me.Energy > 12:
		r = b.P.LightRange
	case laser != nil:
		r = b.P.LaserRange
	}
	if t.Kind == "npc" {
		return 58
	}
	if light == nil {
		if holds(t.Weapons, "lightning") {
			r = math.Max(r, 64)
		}
		if holds(t.Weapons, "mortar") {
			r = math.Max(r, 80)
		}
	}
	return r
}

func (b *Brain) fightMove(o *Obs, tg *Track, seen []*Track, wantLos float64) (float64, float64) {
	me := &o.Self
	x, y := me.Pos[0], me.Pos[1]
	rp := b.prefRange(me, tg)
	bx, by := tg.Pos[0]-x, tg.Pos[1]-y
	bd := hyp(bx, by) + 1e-9
	ux, uy := bx/bd, by/bd
	if b.now >= b.strafeUntil {
		if b.rng.Float64() < 0.6 {
			b.strafe = -b.strafe
		}
		b.strafeUntil = b.now + 0.3 + b.rng.Float64()*0.7
	}
	sx, sy := -uy*b.strafe, ux*b.strafe
	tc := b.center(tg)
	mh := me.Stats.MuzzleHeight
	var cube *known
	for _, c := range b.cubes {
		if d := hyp(c.Pos[0]-x, c.Pos[1]-y); d < 40 && (cube == nil || d < hyp(cube.Pos[0]-x, cube.Pos[1]-y)) && math.Abs(c.Pos[2]-me.Pos[2]) < 4 {
			cube = c
		}
	}
	best := math.Inf(-1)
	var mx, my float64
	blockedSide := false
	rings := 1
	if b.P.Peek {
		rings = 2
	}
	for k := -1; k < 16*rings; k++ {
		cx, cy, dx, dy, r := x, y, 0.0, 0.0, 0.0
		if k >= 0 {
			a := float64(k%16) * math.Pi / 8
			r = 6 * float64(1+k/16)
			dx, dy = math.Cos(a), math.Sin(a)
			cx, cy = x+dx*r, y+dy*r
			if !b.W.LineWalkable(x, y, cx, cy) || !b.W.Free(cx, cy) {
				if k < 16 && dx*sx+dy*sy > 0.9 {
					blockedSide = true
				}
				continue
			}
		}
		cz := b.W.WalkAt(cx, cy)
		d := hyp(tg.Pos[0]-cx, tg.Pos[1]-cy)
		s := -1.1*math.Abs(d-rp) - r*0.4
		los := b.W.Clear(Vec3{cx, cy, cz + mh}, tc)
		if los {
			s += 45 * wantLos
		}
		if b.P.Peek && k >= 0 && k < 16 && (los == (wantLos > 0)) {

			for q := 0; q < 16; q += 2 {
				qx, qy := cx+math.Cos(float64(q)*math.Pi/8)*7, cy+math.Sin(float64(q)*math.Pi/8)*7
				if b.W.Free(qx, qy) && b.W.Clear(Vec3{qx, qy, b.W.WalkAt(qx, qy) + mh}, tc) != los {
					s += 8
					break
				}
			}
		}
		body := Vec3{cx, cy, cz + me.Stats.Height*0.6}
		for _, e := range seen {
			if e == tg || hyp(e.Pos[0]-cx, e.Pos[1]-cy) > 170 {
				continue
			}
			if b.W.Clear(b.eye(e), body) {
				s -= 30
			}
		}
		s -= b.zonePenalty(&o.Zone, cx, cy)
		if cube != nil {
			s += 1.5 * (hyp(cube.Pos[0]-x, cube.Pos[1]-y) - hyp(cube.Pos[0]-cx, cube.Pos[1]-cy))
		}
		if cl := float64(b.W.Clearance[b.W.Cell(cx, cy)]); cl < 4 {
			s -= (4 - cl) * 2.5
		}
		if wantLos > 0 {
			s += 9 * (dx*sx + dy*sy)
		}
		if k < 0 {
			s -= 8
			if wantLos < 0 && !los {
				s += 14
			}
		}
		if s > best {
			best, mx, my = s, dx, dy
		}
	}
	if blockedSide {
		b.strafe = -b.strafe
		b.strafeUntil = b.now + 0.5
	}
	return mx, my
}

func (b *Brain) coverMove(o *Obs, seen []*Track, mx, my *float64) bool {
	me := &o.Self
	x, y := me.Pos[0], me.Pos[1]
	var threats []*Track
	for _, t := range seen {
		if hyp(t.Pos[0]-x, t.Pos[1]-y) < 230 {
			threats = append(threats, t)
		}
	}
	if len(threats) == 0 {
		return false
	}
	if len(threats) > 4 {
		sort.Slice(threats, func(i, j int) bool {
			return hyp(threats[i].Pos[0]-x, threats[i].Pos[1]-y) < hyp(threats[j].Pos[0]-x, threats[j].Pos[1]-y)
		})
		threats = threats[:4]
	}
	hidden := func(cx, cy float64) bool {
		cz := b.W.WalkAt(cx, cy)
		for _, t := range threats {
			e := b.eye(t)
			if b.W.Clear(e, Vec3{cx, cy, cz + me.Stats.Height}) || b.W.Clear(e, Vec3{cx, cy, cz + me.Stats.Height*0.5}) {
				return false
			}
		}
		return true
	}
	if b.coverOK && b.now-b.coverAt < 0.4 {
		*mx, *my = b.travelCover(me)
		return true
	}
	b.coverAt = b.now
	if hidden(x, y) && b.zonePenalty(&o.Zone, x, y) == 0 {
		b.coverOK = true
		b.coverGoal = [2]float64{x, y}
		*mx, *my = 0, 0
		return true
	}

	w := b.W
	s := w.NearestFree(w.Cell(x, y))
	if s < 0 {
		return false
	}
	w.gen++
	gen := w.gen
	queue := []int32{int32(s)}
	w.stamp[s] = gen
	w.g[s] = 0
	bestC, bestS := -1, math.Inf(1)
	checked := 0
	for h := 0; h < len(queue) && checked < 350; h++ {
		c := int(queue[h])
		ci, cj := c%w.NNX, c/w.NNX
		steps := float64(w.g[c])
		if steps > 50 || (bestC >= 0 && steps*2 > bestS+30) {
			break
		}
		if (ci+cj)%2 == 0 && steps > 0 {
			cx, cy := w.CellPos(c)
			checked++
			if hidden(cx, cy) {
				sc := steps*2 + b.zonePenalty(&o.Zone, cx, cy)
				near := math.Inf(1)
				for _, t := range threats {
					near = math.Min(near, hyp(t.Pos[0]-cx, t.Pos[1]-cy))
				}
				sc -= math.Min(near, 80) * 0.25
				if sc < bestS {
					bestC, bestS = c, sc
				}
			}
		}
		for k, o2 := range nbrs {
			i, j := ci+o2[0], cj+o2[1]
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
	if bestC < 0 {
		b.coverOK = false
		return false
	}
	gx, gy := w.CellPos(bestC)
	b.coverOK = true
	b.coverGoal = [2]float64{gx, gy}
	*mx, *my = b.travelCover(me)
	return true
}

func (b *Brain) travelCover(me *Self) (float64, float64) {
	if hyp(b.coverGoal[0]-me.Pos[0], b.coverGoal[1]-me.Pos[1]) < 1.5 {
		return 0, 0
	}
	return b.travel(me, b.coverGoal[0], b.coverGoal[1])
}

type lootChoice struct {
	kind  string
	id    int
	pos   Vec3
	value float64
}

func (b *Brain) chooseLoot(o *Obs, hpFrac float64, danger bool) *lootChoice {
	me := &o.Self
	x, y := me.Pos[0], me.Pos[1]
	z := &o.Zone
	now := b.now
	cur := b.curZone(z)
	var best *lootChoice
	bestU := 0.0
	econ := b.P.Econ
	consider := func(kind string, id int, pos Vec3, value float64) {
		if until, ok := b.bad[id]; ok && until > now {
			return
		}
		d := hyp(pos[0]-x, pos[1]-y)
		if d > 260 {
			return
		}
		if hyp(pos[0]-cur.X, pos[1]-cur.Y) > cur.R-6 {
			return
		}

		if dl := b.zoneDeadline(z, pos[0], pos[1], 6); now+d*1.4/me.Stats.WalkSpeed+5 > dl {
			return
		}

		if pos[2]-b.W.GroundAt(pos[0], pos[1]) > 3.2 {
			return
		}
		if nf := b.W.NearestFree(b.W.Cell(pos[0], pos[1])); nf < 0 || b.W.Comp[nf] != b.W.MainComp {
			return
		}
		u := value / (d + 18)
		if id == b.lootID {
			u *= 1.35
		}
		if econ {
			u = value - 0.6*d
			if id == b.lootID {
				u += 30
			}
		}
		if u > bestU {
			bestU = u
			best = &lootChoice{kind, id, pos, value}
		}
	}
	if econ {
		mechNear := false
		for _, t := range b.tracks {
			if t.Seen == now {
				mechNear = true
			}
		}
		hurt := hpFrac <= 0.45
		nearOnly := false
		for _, t := range b.tracks {
			if b.P.EconSafe > 0 && now-t.Seen < b.P.EconSafe {
				nearOnly = true
			}
		}
		inner := consider
		consider = func(kind string, id int, pos Vec3, value float64) {
			if nearOnly && hyp(pos[0]-x, pos[1]-y) > 60 {
				return
			}
			inner(kind, id, pos, value)
		}
		for _, p := range b.pickups {
			if !b.wantPickup(me, p.Kind) || hurt {
				continue
			}
			v := 150.0
			if len(me.Weapons) > 1 && me.Weapons[1] == nil {
				v = 260
			}
			if p.Kind == "laser" {
				v = math.Max(v, 270) + 20
			}
			consider("pickup", p.ID, p.Pos, v)
		}
		for _, c := range b.cubes {
			if !hurt || hyp(c.Pos[0]-x, c.Pos[1]-y) < 35 {
				consider("cube", c.ID, c.Pos, 110)
			}
		}
		for _, c := range b.crates {
			if !hurt || hyp(c.Pos[0]-x, c.Pos[1]-y) < 35 {
				consider("crate", c.ID, c.Pos, 105)
			}
		}
		if b.P.FarmNPC && hpFrac > 0.6 && !mechNear {
			for _, n := range b.npcs {
				v := 170.0
				if now-n.Seen > 12 {
					v = 155
				}
				consider("npc", n.ID, n.Pos, v)
			}
		}
		for i := range o.Drops {
			d := &o.Drops[i]
			if b.P.UseDrops && !hurt && hyp(d.Pos[0]-x, d.Pos[1]-y)/me.Stats.WalkSpeed < d.LandsIn+25 {
				consider("drop", d.ID, d.Pos, 170)
			}
		}
		if best == nil {
			b.lootID = -1
			return nil
		}
		if best.id != b.lootID {
			b.lootID = best.id
			b.lootSince = now
		} else if lim := 30.0; now-b.lootSince > lim || (best.kind == "crate" && now-b.lootSince > lim-6 && hyp(best.pos[0]-x, best.pos[1]-y) < 50) {
			b.bad[best.id] = now + 60
			b.lootID = -1
			return nil
		}
		return best
	}
	for _, p := range b.pickups {
		rad := b.P.LootRadius
		seek := p.Kind == "laser" && b.P.LaserSeek > rad
		if seek {
			rad = b.P.LaserSeek
		}
		if b.wantPickup(me, p.Kind) && hyp(p.Pos[0]-x, p.Pos[1]-y) < rad {
			v := 60.0
			if len(me.Weapons) > 1 && me.Weapons[1] == nil {
				v = 130
			}
			if seek {
				v = math.Max(v, 150)
			}
			consider("pickup", p.ID, p.Pos, v)
		}
	}
	cubeV := 75.0
	if me.Cubes >= b.cubeCap() {
		cubeV = 25
	}
	for _, c := range b.cubes {
		if hyp(c.Pos[0]-x, c.Pos[1]-y) < math.Max(60, b.P.LootRadius*0.7) {
			consider("cube", c.ID, c.Pos, cubeV)
		}
	}
	farmHP := 0.7
	if b.P.FarmHP > 0 {
		farmHP = b.P.FarmHP
	}
	if hpFrac > farmHP && !danger && now < b.P.CrateUntil {
		for _, c := range b.crates {
			if hyp(c.Pos[0]-x, c.Pos[1]-y) < b.P.LootRadius {
				consider("crate", c.ID, c.Pos, cubeV*0.6)
			}
		}
		if b.P.FarmNPC && me.Cubes < b.cubeCap() {
			for _, n := range b.npcs {
				if now-n.Seen < 12 && hyp(n.Pos[0]-x, n.Pos[1]-y) < b.P.LootRadius {
					consider("npc", n.ID, n.Pos, 85)
				}
			}
		}
	}
	for i := range o.Drops {
		d := &o.Drops[i]
		dist := hyp(d.Pos[0]-x, d.Pos[1]-y)
		if b.P.UseDrops && dist/me.Stats.WalkSpeed < d.LandsIn+25 && !danger {
			consider("drop", d.ID, d.Pos, 260)
		}
	}
	if best == nil {
		b.lootID = -1
		return nil
	}
	if best.id != b.lootID {
		b.lootID = best.id
		b.lootSince = now
	} else if now-b.lootSince > 30 {
		b.bad[best.id] = now + 60
		b.lootID = -1
		return nil
	}
	return best
}

func (b *Brain) exploreMove(o *Obs) (float64, float64) {
	me := &o.Self
	x, y := me.Pos[0], me.Pos[1]
	z := &o.Zone
	if b.now > b.exploreUntil || hyp(b.explore[0]-x, b.explore[1]-y) < 6 {
		for try := 0; try < 20; try++ {
			a := b.rng.Float64() * 2 * math.Pi
			r := math.Sqrt(b.rng.Float64()) * z.NextRadius * 0.8
			px, py := z.NextCenter[0]+math.Cos(a)*r, z.NextCenter[1]+math.Sin(a)*r
			if !b.W.InBounds(px, py) || !b.W.Free(px, py) {
				continue
			}
			if d := hyp(px-x, py-y); d < 50 || d > 220 {
				continue
			}
			b.explore = [2]float64{px, py}
			b.exploreUntil = b.now + 30
			break
		}
	}
	if b.now > b.exploreUntil {
		return 0, 0
	}
	return b.travel(me, b.explore[0], b.explore[1])
}

func (b *Brain) homeCost(o *Obs, px, py float64) float64 {
	z := &o.Zone
	crowd := 0.0
	for _, t := range b.tracks {
		if b.now-t.Seen < 20 {
			crowd += math.Max(0, 80-hyp(t.Pos[0]-px, t.Pos[1]-py)) / 80
		}
	}
	for i := range o.Drops {
		crowd += math.Max(0, 60-hyp(o.Drops[i].Pos[0]-px, o.Drops[i].Pos[1]-py)) / 60
	}
	edge := hyp(px-z.NextCenter[0], py-z.NextCenter[1]) / math.Max(30, z.NextRadius)
	return b.P.HomeExpo*b.W.Exposure(px, py) + 0.25*hyp(px-o.Self.Pos[0], py-o.Self.Pos[1]) + 40*crowd + 10*edge
}

func (b *Brain) home(o *Obs) (float64, float64, bool) {
	z := &o.Zone
	out := !b.homeSet || hyp(b.homeXY[0]-z.NextCenter[0], b.homeXY[1]-z.NextCenter[1]) > z.NextRadius*0.8
	if out || b.now-b.homeAt > 12 {
		b.homeAt = b.now
		best, bestC := [2]float64{}, math.Inf(1)
		for k := 0; k < 40; k++ {
			a := b.rng.Float64() * 2 * math.Pi
			r := math.Sqrt(b.rng.Float64()) * z.NextRadius * 0.7
			px, py := z.NextCenter[0]+math.Cos(a)*r, z.NextCenter[1]+math.Sin(a)*r
			if !b.W.InBounds(px, py) || !b.W.Reachable(px, py) {
				continue
			}
			if c := b.homeCost(o, px, py); c < bestC {
				best, bestC = [2]float64{px, py}, c
			}
		}
		if !math.IsInf(bestC, 1) && (out || bestC < b.homeCost(o, b.homeXY[0], b.homeXY[1])-15) {
			b.homeXY, b.homeSet = best, true
		}
	}
	return b.homeXY[0], b.homeXY[1], b.homeSet
}

func (b *Brain) hideFromBearing(o *Obs, seen []*Track, mx, my *float64) bool {
	me := &o.Self
	ghost := &Track{}
	ghost.Pos = Vec3{me.Pos[0] + math.Cos(b.hurtBearing)*80, me.Pos[1] + math.Sin(b.hurtBearing)*80, me.Pos[2]}
	return b.coverMove(o, append([]*Track{ghost}, seen...), mx, my)
}

func (b *Brain) dodge(o *Obs) (dx, dy float64, laser bool) {
	me := &o.Self
	cx, cy, cz := me.Pos[0], me.Pos[1], me.Pos[2]+me.Stats.Height*0.56
	reach := me.Stats.Radius + 1.1
	for i := range o.Projectiles {
		p := &o.Projectiles[i]
		if p.Kind == "mortar" {
			continue
		}
		v2 := p.Vel[0]*p.Vel[0] + p.Vel[1]*p.Vel[1] + p.Vel[2]*p.Vel[2]
		if v2 < 1 {
			continue
		}
		tc := ((cx-p.Pos[0])*p.Vel[0] + (cy-p.Pos[1])*p.Vel[1] + (cz-p.Pos[2])*p.Vel[2]) / v2
		if tc <= 0 || tc > 0.7 {
			continue
		}
		mxx, myy := p.Pos[0]+p.Vel[0]*tc-cx, p.Pos[1]+p.Vel[1]*tc-cy
		if hyp(mxx, myy) < reach {
			vh := hyp(p.Vel[0], p.Vel[1])
			if vh < 1 {
				continue
			}
			px, py := -p.Vel[1]/vh, p.Vel[0]/vh
			if px*mxx+py*myy > 0 {
				px, py = -px, -py
			}
			dx += px
			dy += py
		}
	}
	for i := range o.Mechs {
		m := &o.Mechs[i]
		if m.Kind != "mech" || !m.Charging {
			continue
		}
		toMe := math.Atan2(cy-m.Pos[1], cx-m.Pos[0])
		if math.Abs(angDiff(m.Aim[0], toMe)) > 0.15 {
			continue
		}
		dx += math.Cos(toMe+math.Pi/2*b.strafe) * 1.5
		dy += math.Sin(toMe+math.Pi/2*b.strafe) * 1.5
		laser = true
	}
	if l := hyp(dx, dy); l > 0.01 {
		return dx / l, dy / l, laser
	}
	return 0, 0, false
}

func (b *Brain) reach(me *Self) float64 {
	r := 0.0
	for _, w := range me.Weapons {
		if w == nil {
			continue
		}
		switch w.Kind {
		case "gun":
			r = math.Max(r, math.Min(w.Stats.Range-10, 130))
		case "laser":
			r = math.Max(r, w.Stats.Range-5)
		case "lightning":
			r = math.Max(r, w.Stats.Range)
		}
	}
	return r
}

func (b *Brain) odds(me *Self, t *Track, now float64, opening bool) float64 {
	myW, thW := 0, len(t.Weapons)
	for _, w := range me.Weapons {
		if w != nil {
			myW++
		}
	}
	if thW == 0 {
		thW = 1
	}
	capc := func(c int) float64 { return math.Min(float64(c), 10) }
	busyMe, busyThem := 0.0, 0.0
	for _, u := range b.tracks {
		if u.ID == t.ID || now-u.Seen > 2.5 {
			continue
		}
		if now-u.HitMe < 6 {
			busyMe = 1
		}
		br := math.Atan2(u.Pos[1]-t.Pos[1], u.Pos[0]-t.Pos[0])
		if math.Abs(angDiff(t.Aim[0], br)) < 0.2 && hyp(u.Pos[0]-t.Pos[0], u.Pos[1]-t.Pos[1]) < 160 {
			busyThem = 1
		}
	}
	z := 1.5093*math.Log(math.Max(me.HP, 1)/math.Max(t.HP, 1)) + 0.7049*(capc(me.Cubes)-capc(t.Cubes))/10 +
		0.9583*float64(myW-thW) - 0.5592*(busyMe-busyThem)
	if opening {
		z += 0.0562 + 0.7475*math.Min(t.Dist, 200)/100
		toMe := math.Atan2(me.Pos[1]-t.Pos[1], me.Pos[0]-t.Pos[0])
		if math.Abs(angDiff(t.Aim[0], toMe)) > 1.75 {
			z += 0.8235
		}
	}
	return 1 / (1 + math.Exp(-z))
}

func squadParams(p Params) Params {
	p.Ghost = false
	p.Peek = false
	p.LaserFirst = true
	p.LaserRange = 170
	p.LootRadius = 150
	p.CrateUntil = 1e9
	p.Stick = 120
	p.HPw = 0.2
	p.LowBonus = 100
	p.MaxTarget = 240
	p.LoseRule = true
	p.FightLoot = 70
	p.FarmHP = 0.5

	p.LaserSeek = 240
	p.LaserUp = true
	p.Lethal = true
	p.Finish = 20
	p.Stick = 200
	p.LowBonus = 200
	p.LethalK = 0.7
	p.HPw = 0.15
	p.HPw = 0.18

	return p
}

func (b *Brain) cubeCap() int {
	c := b.Cfg.Cubes.StatCap
	if g := int(b.P.CubeGoal); g > c {
		c = g
	}
	return c
}

func (b *Brain) rearMult() float64 {
	if m := b.Cfg.Mech.RearMult; m > 1 {
		return m
	}
	return 1
}

func (b *Brain) showsBack(t *Track, x, y float64) bool {
	arc := b.Cfg.Mech.FrontArcDeg
	if arc <= 0 {
		arc = 180
	}
	return math.Abs(angDiff(t.Aim[0], math.Atan2(y-t.Pos[1], x-t.Pos[0]))) > arc*math.Pi/360+0.1
}
