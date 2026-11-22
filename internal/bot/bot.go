// Package bot is our Mech Royale agent: memory of what it has seen, a nav/dodge movement planner,
// ballistics for every weapon, and a survival-first strategy layer on top.
package bot

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"strings"

	"apexbot/internal/proto"
	"apexbot/internal/world"
)

// Params are the tunables; variants of the bot differ only in these.
type Params struct {
	Name string

	EngageRatio   float64 // start a fight when our advantage ratio is at least this
	DisengageRat  float64 // retreat when the ratio falls below this
	RetreatHP     float64 // retreat to cover below this HP fraction when someone can hurt us
	ReturnHP      float64 // come back out above this HP fraction
	LeadDamp      float64 // how much of a target's velocity to trust for long flight times
	GunMaxRange   float64 // never fire the gun at mechs beyond this
	StrafeMin     float64 // seconds between strafe direction changes (min)
	StrafeMax     float64
	LootRadius    float64 // how far we go for loot
	WantCubes     int     // stop seeking cubes beyond this
	DropGreed     float64 // 0..1 willingness to contest supply drops
	Hunt          bool    // chase weaker enemies that are not looking at us
	PracticeAware bool    // adapt when the lobby looks like the cooperating house squad

	// free-for-all stance (used once we see mechs dying that we did not kill)
	FFAEngage     float64
	FFADisengage  float64
	FFALootRadius float64
	FFAWantCubes  int
	FFAReturnFire float64 // only shoot back at attackers closer than this
	Salt          int     // changes the dice only (see New)
	ForceFFA      bool    // testing: assume a free-for-all from the start
	SlotRule      bool    // WRONG, never enable: the track page says the practice slot varies each match
	CloseDisengage float64 // disengage threshold against an attacker within ~60 m (stealth stances)
	DuelDisengage  float64 // leave a one-on-one only below this advantage

	UseRoofs      bool    // hide on rooftops when we have nothing better to do (stealth stances)
	RoofInSquad   bool    // ... and in the squad stance too
	RoofMaxClimb  float64 // tallest wall we try
	RoofMinHeight float64 // lowest roof worth standing on
	RoofMinZone   float64 // stop using roofs when the next circle is smaller than this
	ArmRadius     float64 // how far we go for a second weapon even while being watched
	OpenSecs      float64 // how long we stay careful before assuming the cooperating squad
	CreditOpenS   float64 // ... and for how long again after a death we credited to ourselves on a guess (0 = never)
	DropRadius    float64 // how far we travel for a supply drop
	LeaveTTD      float64 // with several attackers, leave when we would be dead in this many seconds
	PatientSecs   float64 // squad stance: until this time (and below PatientCubes) farm instead of killing hunters (0 = off)
	PatientCubes  int
	PatientRange  float64 // ... unless the hunter is closer than this
	// hideouts from the embedded visibility map (stealth stances)
	UseHideMap  bool
	HideSeenW   float64 // weight of the share of ground that can see a place
	HideHitW    float64 // weight of the share that has a line of fire to our centre there
	HideDistW   float64 // points per metre of travel to get there
	HideClimbW  float64 // points per metre of wall to climb
	HideStick   float64 // bonus for the place we already chose
	StealthFightRange float64 // stealth stances: stand and fight an attacker only inside this range (else slip away)
	QuietHoldS        float64 // seconds without anyone near or watching before we call it quiet
	LaserValue        float64 // pickup value of a laser (the gun is 2.0, a mortar 1.2)
	LightningValue    float64
	ArmSwapMin        float64 // go out of our way for a weapon that replaces one we hold only if it is worth this much more
	HitModel          int     // 0: gun hit rates as believed before the real matches, 1: as measured in them
	PlanV2            bool    // fight plan counts the damage we deal while changing range, not only what we take
	KiteNeedsLaser    bool    // back away from several attackers only when we hold a laser
	ExchangeW         float64 // how much the running exchange with a target (dealt vs taken) moves our estimate of the fight (0 = off)
	// "brawl": the free-for-all stance built after the first real matches (replaces stealth when on)
	Brawl           bool
	BrawlEngage     float64 // start a fight with someone who is not on us above this advantage
	BrawlDisengage  float64
	BrawlHunt       bool
	BrawlLootRadius float64
	BrawlWantCubes  int
	AvoidAdv        float64 // attacked from beyond AvoidRange with less advantage than this: slip behind cover, shooting back (0 = always stand and fight)
	AvoidRange      float64
	AvoidCover      float64 // ... if cover is within this path length
	CoverFirst      bool    // brawl stance: hit by someone we have no clear edge over, go for cover first (any range beyond CoverMinRange, light fire included)
	CoverMinRange   float64
	EdgeHold        float64 // seconds a fight-or-cover decision is given the benefit of the doubt before it may flip (0 = none)
	RunOnRange      float64 // hurt and behind cover with the attacker last seen inside this range: keep going, deeper into cover (0 = stop at the first corner)
	RunOnGain       float64 // ... each leg must put at least this much more distance between us
	CoverMargin     float64 // cover must stay cover when the threat steps this far to either side (0 = the nearest hidden spot will do)
	ShySecs         float64 // opening rule: for this long, position before loot while a mech is in view (0 = off)
	ShyRange        float64 // ... a mech in view inside this range counts
	ShyLaser        float64 // ... a laser this close is taken whatever the view
	ShyMem          float64 // ... mechs seen this long ago still count for what we may fetch
	ShyBoost        float64 // ... sprint to cover while energy is above this (0 = walk)
	ShyLootRadius   float64 // ... nothing but a laser from farther than this (0 = no limit)
	ShyExposure     float64 // ... loot in open ground is worth this much less (0..1)
	RetreatRunMax   float64 // brawl stance: in an attacker's sights, only run for cover within this many metres of path; otherwise fight on (0 = always run)
	RetreatRunLaser float64 // ... against a laser we cannot answer
	LootHiddenBias  float64 // brawl stance: loot in plain sight of a mech we know about is worth this much less (0..1)
	CoverEdgeAdv    float64 // ... unless our estimated advantage is at least this (a laser against none needs only AvoidAdv)
	HuntLaserEdge   bool    // brawl stance: start a fight only with a laser against a mech without one (or at CoverEdgeAdv)
	Peek            bool   // brawl stance with a laser: fight from behind cover (charge hidden, step out to fire)
	PeekCover       float64 // ... if a hidden cell is within this path length
	PeekReach       float64 // ... and a firing cell within this many metres of it
	PeekMinRange    float64 // ... and the target is farther than this (closer than that it is a brawl)
	OverlandEarly   float64 // when the next circle cannot be walked to, allow this many times the usual travel time plus 25 s (0 = the old rule)
	RoofCoverHold   float64 // brawl stance: stay behind the roof edge this long after the last watcher (0 = step back out at once)
	RoofStandCheck  bool    // stay on a roof only if the spot we would stand at is inside the next circle
	DropStick       bool    // supply drops: stay on one once chosen; skip them when at the cube cap and healthy (brawl stance)
	ClimbClearRange float64 // brawl stance: no climb while any mech is in plain sight inside this range
	RoofCenterW     float64 // roof choice: penalty per metre from the middle of the next circle (0 = off)
	RoofLeaveQuiet  float64 // leave a doomed roof only when unwatched, until the zone edge is this many seconds away (0 = leave at once)
	ZoneSlack       float64 // loot must stay inside the zone this long after we would reach it (0 = the old rule)
	BrawlRoofs      bool    // brawl stance, free-for-all known: wait on a roof once armed for it
	BrawlRoofCubes  int     // ... which means a laser, or this many cubes,
	BrawlRoofAfter  float64 // ... or this much match time
	Jink            float64 // zig-zag (sideways share of the heading) while walking somewhere under gun fire from range (0 = off)
	LeadPersist     bool    // scale the gun lead by how straight the target has been moving (jitterers get none)
	LeadCum         bool    // per opponent, aim the way (lead / no lead) that has been missing by less while it fights us
	LeadCumMin      float64 // ... once this many virtual shots have been scored
	LaserPreempt    bool    // a laser carrier looking at us counts as an attack already (take cover before the first beam)
	AvoidCoverLaser float64 // path length we accept to cover when the attacker holds a laser
	HoldHideMap     bool    // brawl stance: with nothing to do, wait at a ground hideout from the visibility map
	LaserAimRange   float64 // a laser carrier looking at us counts as on us out to this range (others: 200 m)
	PathFix         bool    // a goal is only called unreachable after a path search for that very goal failed
	CreditFix       bool    // a death is not credited to our own shooting while our target is in plain sight alive
	LootBoost       float64 // sprint to a laser on the ground while energy is above this (0 = walk)
	NoMortar        bool    // do not pick up mortars (holding one adds 60% to every laser hit we take)
	ProvokeRule     bool    // being looked at from beyond AvoidRange is not an attack: hold fire until touched
	PlinkMaxDPS     float64 // under lighter fire than this from range (no laser on us), keep farming and shoot back in between (0 = off)
	HuntRange       float64 // start a fight with a favourable target inside this range
	HuntRangeLaser  float64 // ... when we hold a laser
	KiteMin       float64 // against two or more attackers hold this range band from the nearest (0 = off)
	KiteMax       float64
	RoofFirstSecs float64 // opening: for this long, getting onto a nearby roof beats arming (0 = off)
	RoofFirstDist float64 // ... if an access point is within this distance

	// Apex-Go v2 breakthrough upgrades (all config-driven, individually ablatable via BOT_PARAMS)
	AllNoMortar      bool    // refuse mortars in ALL stances whenever weapons.laser.cookoff > 0
	CookoffAware     bool    // include weapons.laser.cookoff in DPS, lethal laser release, and target priority
	AntiLightningW   float64 // steer posCost repulsion weight against any enemy lightning carrier (0 = off)
	AntiLightningPad float64 // metres outside lightning.range to keep clear
	LaserDodgeV2     bool    // wider charging-laser detection + open-ground beam-ray perpendicular sidestep
	LaserBeamW       float64 // steer posCost weight for perpendicular beam-line avoidance in the open
	SquadLaserCover  bool    // in squad stance, duck behind nearby cover (<24m) while an enemy laser charges at us
	SmartFireV2      bool    // cooldown-aware gun/laser latency, dynamic gun speed in lead, rear_mult & cookoff lethal
	LaserEarlyRel    bool    // release laser at full - dt so odd-tick charges do not stall an extra decision
	RearBonus        float64 // target priority bonus fraction when enemy's back is exposed (rear_mult)
	CookoffBonus     float64 // target priority bonus fraction for mortar carriers when we hold a laser
	CautiousT        float64 // practice late-game switch to cautious brawl stance at t >= CautiousT (0 = off)
	BeamFFA          bool    // detect FFA immediately from repeated enemy-on-enemy laser beams
	FinalHold        bool    // shrink final-circle hold tolerance linearly with Zone.Radius
	SquadPeek        bool    // allow laser corner-peeking in squad stance too
	NanoBoost        float64 // additive rank boost for nanites upgrade
	RegenHoldHP      float64 // when no enemy is engaged and hpFrac < RegenHoldHP, briefly hold cover to trigger regen (0 = off)
	ThreatFocus      bool    // focus-fire bonus for laser/lightning carriers and low-HP enemies (<35%)
	LightKiteBoost   bool    // boost-kite away when an enemy lightning carrier closes inside range+12

	// Apex-Go v3 upgrades (from 402-repo GitHub re-probe, all config-driven & independently ablatable)
	CoverReload      bool    // MarwanMoh11 v5.15: top off gun mag (ammo<=0.6*mag) behind cover when no enemy LOS >= 1.1s
	UpV2             bool    // lukejwaszyn f58c537: exact sustained-DPS upgrade order (gun_mag +31% above gun_rate +8%)
	CubeSmart        bool    // lukejwaszyn a37a738: price cube piles by cluster size & 1.75x bonus when crossing upgrade threshold
	LaserMechOnly    bool    // andrew-zhouu v0.134: never start a laser charge on crates/NPCs (keep ready for mechs)
	RearAvoidW       float64 // SandeepMadhavarapu 4f3c7d9: penalize steer positions putting extra attackers in our 1.5x rear arc
	LaserCDTrack     bool    // MarwanMoh11 v5.11: track obs.Beams laser shots per enemy -> punish during 2s cooldown window
	KillFocus        float64 // andrew-zhouu v0.134: weight of enemy missing HP in target scoring (0 = default 1.0)
	Seek             float64 // andrew-zhouu v0.134: in squad stance, seek last-known enemy seen within Seek seconds (0 = off)
	SeekRange        float64 // max range for Seek (default 320)
	SeekHP           float64 // min hpFrac for Seek (default 0.65)
	DropRun          float64 // andrew-zhouu v0.134: in squad stance, break off for supply drop within DropRun m (0 = off)
	EdgeBias         float64 // andrew-zhouu v0.119: in FFA stance, prefer outer rim of next circle (0 = off)
	EdgeBand         float64 // in FFA stance, penalize hiding closer than EdgeBand*NextRadius to center (0 = off)
	PracticeRim      float64 // practice rim-riding: prefer outer 85-90% of next circle while NextRadius >= 180 (0 = off)
	RetreatPest      bool    // shoot plinking NPC drone while retreating if no enemy mech is engaged

	// ohio-ironclad additions (our own code, practice squad only, 0 = off)
	PaceSecs  float64 // after each of our kills the squad sends another hunter: for this long, start no hunt or seek ...
	PaceHP    float64 // ... unless our health is back above this fraction (regeneration does the rest)
	HuntAmmo  float64 // start a hunt only with at least this share of the gun magazine; top up while watching an idle mech
	SeekHurtW float64 // seek: prefer a wounded mech, counting each missing 1% of its health as this many metres closer
}

func DefaultParams() Params {
	return Params{
		Name:          "ohio-ironclad",
		EngageRatio:   0.9,
		DisengageRat:  0.75,
		RetreatHP:     0.45,
		ReturnHP:      0.85,
		LeadDamp:      0.65,
		GunMaxRange:   224,
		StrafeMin:     0.35,
		StrafeMax:     1.1,
		LootRadius:    220,
		WantCubes:     15,
		DropGreed:     0.5,
		Hunt:          true,
		PracticeAware: true,
		Salt:          8,
		// SlotRule stays off: the track page says the practice slot varies each match
		FFAEngage:     1.8,
		FFADisengage:  0.95,
		FFALootRadius: 140,
		FFAWantCubes:  10,
		FFAReturnFire: 230,
		CloseDisengage: 0.8,
		DuelDisengage:  0.5,
		UseRoofs:       true,
		RoofMaxClimb:   40,
		RoofMinHeight:  12,
		RoofMinZone:    60,
		ArmRadius:      110,
		OpenSecs:       24,
		CreditOpenS:    25,
		DropRadius:     300,
		LeaveTTD:       10,
		UseHideMap:     false, // stealth hideouts: unused by the brawl stance (kept for the variants)
		HideSeenW:      0.6,
		HideHitW:       0.9,
		HideDistW:      0.16,
		HideClimbW:     0.5,
		HideStick:      12,
		StealthFightRange: 140,
		QuietHoldS:        3,
		LaserValue:        6.0,
		LightningValue:    2.6,
		ArmSwapMin:        1.0,
		HitModel:          1,
		PlanV2:            true,
		KiteNeedsLaser:    true,
		Brawl:             true,
		AvoidAdv:          1.25,
		BrawlEngage:       1.3,
		BrawlDisengage:    0.75,
		BrawlHunt:         true,
		BrawlLootRadius:   220,
		BrawlWantCubes:    10, // stats stop growing at 10 cubes; size (and so how easy we are to hit) does not
		AvoidRange:        85,
		AvoidCover:        55,
		ZoneSlack:         10,
		CoverFirst:        true,
		NoMortar:          true, // a laser hit on a mortar carrier does 60% more (seen: 1,024 health gone in 0.7 s)
		EdgeHold:          2.5,
		RunOnRange:        80,
		RunOnGain:         18,
		CoverMargin:       8,
		// The opening rule is off (ShySecs 25 with SlotRule: variant b-shy). It does what it says (at
		// 10 s in sight 45% against 55%, inside 100 m and seen 16% against 26%, hit inside 20 s 55%
		// against 66%) but gives up a quarter of the early lasers, and in the re-enactment the deaths
		// only move from before 30 s to after it: about +0.1 points by the real-match index. Against
		// the practice squad it costs 0.4, hence the SlotRule it would need.
		ShyRange:          220,
		ShyLaser:          60,
		ShyMem:            6,
		// RetreatRunMax 50 / RetreatRunLaser 90 (variant b-standrun) is not on: the real matches favour it
		// (one attacker inside 60 m, shot back: alive 20 s later 35% staying against 23% running),
		// every simulator here puts it 0.1-0.2 points behind, and either way it is a small thing.
		RetreatRunLaser:   90,
		HuntLaserEdge:     true,
		LootHiddenBias:    0.5,
		CoverMinRange:     40,
		CoverEdgeAdv:      3.0,
		PeekCover:         25,
		PeekReach:         9,
		PeekMinRange:      45,
		RoofCoverHold:     6,
		RoofStandCheck:    true,
		DropStick:         true,
		RoofLeaveQuiet:    25,
		ClimbClearRange:   150,
		BrawlRoofs:        true,
		LaserPreempt:      true,
		LaserAimRange:     285,
		AvoidCoverLaser:   90,
		BrawlRoofCubes:    6,
		BrawlRoofAfter:    60,
		LeadPersist:       true,
		LeadCum:           true,
		LeadCumMin:        8,
		PathFix:           true,
		CreditFix:         true,
		LootBoost:         30,
		ProvokeRule:       true,
		PlinkMaxDPS:       60,
		HuntRange:         180,
		HuntRangeLaser:    200,
		KiteMin:        130,
		KiteMax:        190,
		PatientCubes:   10,
		PatientRange:   70,
		// Apex-Go v2 verified winners (#1 6.28 official; 4.70 192m grand; 8.35 FFA)
		AntiLightningW:   180,
		AntiLightningPad: 12,
		SmartFireV2:      true,
		LaserEarlyRel:    true,
		RearBonus:        0.25,
		CookoffBonus:     0.30,
		CautiousT:        265,
		BeamFFA:          true,
		FinalHold:        true,
		// Apex-Go v3 verified winners (4.99 192m grand, 6.00 TR_HK1, 9,070 dealt, 3.87x trade, 8 alive wins)
		LaserCDTrack: true,
		RearAvoidW:   50,
		CoverReload:  true,
		RetreatPest:  true,
		PracticeRim:  0.6,
		CubeSmart: true, // ohio-ironclad practice setting
		KillFocus: 1.5, // ohio-ironclad practice setting
		Seek: 30, // ohio-ironclad practice setting
		SeekHurtW: 1.5, // ohio-ironclad practice setting
	}
}

// Track is what we remember about another mech or drone.
type Track struct {
	ID       int
	NPC      bool
	Pos, Vel [3]float64
	Aim      [2]float64
	HP       float64
	MaxHP    float64
	Cubes    int
	Laser    bool
	Mortar   bool
	Light    bool
	Gun      bool
	Charging bool
	LastSeen float64 // match time of the last observation that listed it
	Listed   bool    // listed in the current observation
	LOS      bool    // we have a clear line from our eye to it right now
	LOST     float64 // last time LOS was true
	HurtMeT  float64 // last time it damaged us
	HurtMe   float64 // decayed damage it dealt to us
	IHurtT   float64 // last time we damaged it
	IHurt    float64 // decayed damage we dealt to it
	AimAtMeT float64 // last time its aim pointed at us
	SeenT0   float64
	PhantomT float64 // last time it hit us while unseen (its Pos is then a guess along the bearing)
	WasListed bool   // listed in the previous observation
	ViewT    float64 // last time it was inside our own field of view (not just remembered)
	EngT     float64 // last time it counted as engaged with us
	LaserFiredT float64 // last time we saw this mech fire a laser beam (for cooldown punish window)

	// recent motion, for choosing how to lead it (see lead.go)
	hist            [trkHist]trkSample
	hn, hi          int     // samples held, index of the next write
	leadN           float64 // (decayed) number of virtual shots scored
	missNone        float64 // (decayed) sum of what aiming at where it stood would have missed by
	missLead        float64 // ... and what leading it would have missed by
}

const trkHist = 40 // four seconds at ten observations a second

type trkSample struct {
	t, x, y, vx, vy float64
	facing          bool // its torso pointed at us
}

func (t *Track) scale(sizePerCube float64) float64 { return 1 + sizePerCube*float64(t.Cubes) }

type modeRec struct {
	t    float64
	mode string
}

type hitRec struct {
	t   float64
	amt float64
}

type dmgRec struct {
	t      float64
	by     int
	weapon string
	amt    float64
	d      float64
	seen   bool
}

type Item struct {
	ID       int
	Pos      [3]float64
	HP       float64
	Kind     string
	LastSeen float64
	Ground   bool // reachable on the ground nav grid
	Cell     int
}

type slot struct {
	ok       bool
	kind     string
	cooldown float64
	ammo     float64
	reload   float64
	charge   float64
	stats    map[string]float64
}

type Bot struct {
	P   Params
	H   *proto.Hello
	C   *proto.Config
	W   *world.World
	Nav *world.Nav
	rng *rand.Rand

	// config shortcuts
	dt          float64 // seconds per sim tick
	decide      float64 // seconds per decision
	sizePerCube float64

	// current observation
	o       *proto.Obs
	t       float64
	pos     [3]float64
	vel     [3]float64
	aim     [2]float64
	hp      float64
	maxHP   float64
	energy  float64
	scale   float64
	radius  float64
	height  float64
	eyeH    float64
	muzzleH float64
	walk    float64
	boostV  float64
	turn    float64
	pitchR  float64
	armor   float64
	slots   [2]slot

	// memory
	tracks  map[int]*Track
	crates  map[int]*Item
	cubes   map[int]*Item
	pickups map[int]*Item

	lastDamageT  float64 // last time we took damage from anything
	lastWeaponDT float64 // last time a weapon (not zone) hurt us
	lastEnemyLOST float64 // last time any enemy mech had LOS to us (for cover-pause top-off reload)
	lastDealtT   float64
	recentDmg    float64 // decayed damage taken
	lastDmgBear  float64
	lastDmgBy    int
	lastSoundT   float64
	lastSoundB   float64

	// movement state
	path       []int32
	pathBad    bool // the last search failed or stopped short of the goal
	pathSet    bool // a search has been made (pathGoal and pathT mean something)
	pathGoal   [2]float64
	pathT      float64
	pathIdx    int
	strafeSign float64
	strafeT    float64
	stuckT     float64
	stuckPos   [3]float64
	stuckFor   float64
	unstickT   float64
	unstickDir [2]float64
	lastMove   [2]float64

	// strategy state
	mode       string
	targetID   int
	retreating bool
	coverCell  int
	coverT     float64
	deepCell   int     // retreat: the next place deeper into cover we are walking to (-1 none)
	deepT      float64 // ... when we chose it
	deepTryT   float64 // ... when we last looked for one
	standT      float64 // last time we stayed in a fight for want of cover close by
	creditAbsorbT float64 // last death we put down to our own shooting without being given the kill
	nearCoverT  float64 // coverClose: when we last looked
	nearCoverOK bool    // ... and what we found
	lootID     int
	lootT      float64
	holdCell   int
	holdT      float64
	lastAimCmd [2]float64
	scanPhase  float64
	laserHold  bool
	takeT      float64
	ffaSeenT    float64 // last time we saw evidence other mechs fight each other
	ffaBeams    int     // count of distinct observations with distant enemy-on-enemy beams
	kills       int
	lastKillT   float64 // ohio-ironclad: match time of our latest kill
	killsSeen   int
	deadSeen    int
	unexplained int  // deaths we did not cause
	ffa         bool // the lobby is a free-for-all (others kill each other)
	st          stance
	evadeCell   int
	evadeT      float64
	hideT       float64
	lastLabel   string
	modeLabel   string // stance:mode of this decision
	lastTraceT  float64
	lastNpcDT   float64
	seenT       []float32 // coarse grid: when we last looked at each cell
	seenNX      int
	seenNY      int
	roamCell    int
	roamT       float64
	dmgLog      []dmgRec
	modeLog     []modeRec
	hitLog      []hitRec
	lowT        float64
	retreatT    float64
	corneredT   float64
	zgKey       [3]float64 // zone-goal cache
	zgT         float64
	zgComp      int32
	zgGoal      [2]float64
	zgWalk      bool
	myComp      int32           // walkable region we are in (0 unknown)
	mainCompID  int32           // the largest walkable region (cached)
	surf        []int32         // route over the top surface (overland travel)
	surfGoal    [2]float64
	surfT       float64
	surfIdx     int
	surfBan     map[int]float64 // surface cells that turned out to be impassable
	surfWatchT  float64
	surfWatchPos [3]float64
	surfPushing bool
	banUntil    map[int]float64 // loot ids / roam cells we could not reach
	seekBan     map[int]float64 // mech ids we sought and didn't find there
	roof        roofPlan
	roofCoverC  int
	roofCoverT  float64
	unquietT    float64  // last time someone was near or watching
	peek        peekPlan // laser fight from behind cover: the current hide and fire cells
	roofCoverHoldT float64 // last time something made us duck behind the roof edge
	holdClimb   bool     // do not start jetting up a wall right now (someone is watching or shooting)
	holdRoofMove bool    // do not leave the roof we are on right now (same reason; the zone is not close yet)
	jinkSign    float64  // which side the current zig of a zig-zag goes
	jinkT       float64  // when it flips next
	edgeOnT     float64  // last time we stood to fight on an edge (brawl stance)
	slipCell    int      // cover we are slipping to (brawl stance)
	slipT       float64  // when it was last looked for
	slipOnT     float64  // last time we decided to slip
	hm          *hideMap // embedded visibility map (nil if it does not fit this arena)
	hideCell    int      // hideout we are heading for or sitting in (-1 none)
	hideOut     hideout
	hideOutOK   bool
	hideOutT    float64
	roofFirstOK bool
	roofFirstChecked bool
	// debug accounting (Stats)
	roofSecs      float64
	dmgOnRoof     float64
	dmgOffRoof    float64
	dmgClimb      float64
	climbs        int
	prevRoofState int
	creditT     float64 // last time we damaged a mech (deaths right after count as ours)
	creditID    int     // ... and which one

	scratch []cand
}

// Factory builds a bot with default parameters.
func Factory(h *proto.Hello, w *world.World, nav *world.Nav) *Bot {
	return New(h, w, nav, DefaultParams())
}

func New(h *proto.Hello, w *world.World, nav *world.Nav, p Params) *Bot {
	b := &Bot{P: p, H: h, C: &h.Config, W: w, Nav: nav}
	// (Salt: builds that differ only in it play the same game with different dice. Practice is 48 fixed
	// hidden matches, so each such build is a fresh draw of the same strength.)
	b.rng = rand.New(rand.NewSource(h.Seed*7919 + int64(h.Slot) + 1 + int64(p.Salt)*104729))
	hz := stat(h.Config.Match, "tick_hz", 20)
	if hz <= 0 {
		hz = 20
	}
	b.dt = 1 / hz
	b.decide = stat(h.Config.Match, "decide_every", 2) * b.dt
	b.sizePerCube = h.Config.Cubes.SizePerCube
	b.tracks = map[int]*Track{}
	b.crates = map[int]*Item{}
	b.cubes = map[int]*Item{}
	b.pickups = map[int]*Item{}
	b.strafeSign = 1
	b.targetID = -1
	b.lootID = -1
	b.coverCell = -1
	b.deepCell, b.deepT, b.deepTryT = -1, -100, -100
	b.nearCoverT = -100
	b.standT = -100
	b.creditAbsorbT = -100
	b.holdCell = -1
	b.evadeCell = -1
	b.lastDamageT = -100
	b.lastWeaponDT = -100
	b.lastEnemyLOST = -100
	b.lastDealtT = -100
	b.lastSoundT = -100
	b.ffaSeenT = -100
	b.lastDmgBy = -1
	b.lastNpcDT = -100
	b.roamCell = -1
	b.creditT = -100
	b.lowT = -100
	b.corneredT = -100
	b.mode = "init"
	b.roof.id = -1
	b.roof.want, b.roof.spot = -1, -1
	b.roofCoverC = -1
	b.hideCell = -1
	b.hideOutT = -100
	b.unquietT = -100
	b.slipCell, b.slipT, b.slipOnT = -1, -100, -100
	b.edgeOnT = -100
	b.jinkSign, b.jinkT = 1, -100
	b.roofCoverHoldT = -100
	b.peek = peekPlan{t: -100, tgt: -1, roof: -1, duck: -1, fire: -1}
	if p.UseHideMap || p.HoldHideMap {
		b.hm = getHideMap(nav)
	}
	b.roofFirstOK = true
	b.banUntil = map[int]float64{}
	b.coverT = -100
	b.evadeT = -100
	b.roofCoverT = -100
	// build the shared maps now (inside the hello budget), never during a decision
	nav.Exposure()
	nav.Components()
	nav.Roofs()
	nav.Tops()
	b.surfT = -100
	return b
}

// Trace, when set (MECH_TRACE=1 in the local arena), logs our mode changes to stderr.
var Trace = os.Getenv("MECH_TRACE") != ""

// Stats (MECH_STATS=1) logs a one-line snapshot every 30 s of match time, for aggregate analysis.
var Stats = os.Getenv("MECH_STATS") != ""

func (b *Bot) trace(act *proto.Action) {
	if b.modeLabel == b.lastLabel && b.t-b.lastTraceT < 5 {
		return
	}
	b.lastLabel, b.lastTraceT = b.modeLabel, b.t
	tg := ""
	if tr := b.tracks[b.targetID]; tr != nil {
		tg = fmt.Sprintf(" tgt=%d d=%.0f los=%v hp=%.0f", tr.ID, dist3(b.pos, tr.Pos), tr.LOS, tr.HP)
	}
	w := ""
	for i := 0; i < 2; i++ {
		if b.slots[i].ok {
			w += b.slots[i].kind[:3] + " "
		}
	}
	fmt.Fprintf(os.Stderr, "[%d] t=%6.1f alive=%2d hp=%4.0f/%4.0f en=%3.0f cubes=%2d pos=(%.0f,%.0f,%.0f) %-18s w=%s proj=%d mv=(%+.1f,%+.1f)%s%s\n",
		b.H.Slot, b.t, b.o.Alive, b.hp, b.maxHP, b.energy, b.o.Self.Cubes, b.pos[0], b.pos[1], b.pos[2], b.modeLabel, w, len(b.o.Projectiles), act.Move[0], act.Move[1], map[bool]string{true: " BOOST", false: ""}[act.Boost], tg)
	if strings.Contains(b.modeLabel, "zone") {
		z := &b.o.Zone
		fmt.Fprintf(os.Stderr, "      zone cur=(%.0f,%.0f r%.0f) dcur=%.0f next=(%.0f,%.0f r%.0f) dnext=%.0f shrink=%.0f..%.0f inzone=%v goal=(%.0f,%.0f) walk=%v comp=%d ground=%v jump=%v\n",
			z.Center[0], z.Center[1], z.Radius, hyp(b.pos[0]-z.Center[0], b.pos[1]-z.Center[1]), z.NextCenter[0], z.NextCenter[1], z.NextRadius,
			hyp(b.pos[0]-z.NextCenter[0], b.pos[1]-z.NextCenter[1]), z.ShrinkStart, z.ShrinkEnd, b.o.Self.InZone, b.zgGoal[0], b.zgGoal[1], b.zgWalk, b.myComp, b.onGroundLevel(), act.Jump)
	}
}

// Act is one decision.
func (b *Bot) Act(o *proto.Obs) proto.Action {
	act := proto.NewAction(o.Tick)
	b.observe(o)
	b.think(&act)
	if Trace {
		b.trace(&act)
	}
	if Stats && o.Tick > 0 && o.Tick%600 == 0 {
		w := ""
		for i := 0; i < 2; i++ {
			if b.slots[i].ok {
				w += b.slots[i].kind + "+"
			}
		}
		fmt.Fprintf(os.Stderr, "[stat] t=%d stance=%s weapons=%s cubes=%d hp=%.0f alive=%d\n", int(b.t), b.st.name, w, o.Self.Cubes, b.hp, o.Alive)
		fmt.Fprintf(os.Stderr, "[roof] t=%d onroof_s=%.0f offroof_s=%.0f dmg_on=%.0f dmg_off=%.0f dmg_climb=%.0f climbs=%d\n", int(b.t), b.roofSecs, b.t-b.roofSecs, b.dmgOnRoof, b.dmgOffRoof, b.dmgClimb, b.climbs)
	}
	if Stats {
		on := b.onRoof() >= 0 && o.Self.Grounded
		if on {
			b.roofSecs += b.decide
		}
		for i := range o.Events {
			e := &o.Events[i]
			if e.Type == "damaged" && e.Weapon != "zone" {
				switch {
				case on:
					b.dmgOnRoof += e.Amount
				case b.roof.state == roofClimb:
					b.dmgClimb += e.Amount
				default:
					b.dmgOffRoof += e.Amount
				}
			}
		}
		if b.roof.state == roofClimb && b.prevRoofState != roofClimb {
			b.climbs++
		}
		b.prevRoofState = b.roof.state
	}
	if Stats && b.hp < 220 && b.t-b.lowT > 15 && len(b.dmgLog) > 0 {
		// near death: who did this, from where, could we see them
		b.lowT = b.t
		type agg struct {
			dmg, d float64
			n, seen int
			w       string
		}
		by := map[int]*agg{}
		first := b.t
		total := 0.0
		for _, r := range b.dmgLog {
			if b.t-r.t > 12 {
				continue
			}
			a := by[r.by]
			if a == nil {
				a = &agg{}
				by[r.by] = a
			}
			a.dmg += r.amt
			a.n++
			a.w = r.weapon
			if r.d >= 0 {
				a.d = r.d
			}
			if r.seen {
				a.seen++
			}
			if r.t < first {
				first = r.t
			}
			total += r.amt
		}
		s := ""
		for id, a := range by {
			s += fmt.Sprintf(" %d:%s:%.0fdmg:%.0fm:seen%d/%d", id, a.w, a.dmg, a.d, a.seen, a.n)
		}
		w := ""
		for i := 0; i < 2; i++ {
			if b.slots[i].ok {
				w += b.slots[i].kind + "+"
			}
		}
		// what were we doing while it happened
		modes := map[string]int{}
		nm := 0
		for _, mr := range b.modeLog {
			if mr.t >= first-0.5 {
				modes[mr.mode]++
				nm++
			}
		}
		top, topN := "", 0
		for m, c := range modes {
			if c > topN {
				top, topN = m, c
			}
		}
		dealt := 0.0
		for _, hr := range b.hitLog {
			if hr.t >= first-0.5 {
				dealt += hr.amt
			}
		}
		fmt.Fprintf(os.Stderr, "[low] t=%.1f stance=%s mode=%s weapons=%s cubes=%d attackers=%d over=%.1fs total=%.0f dealt=%.0f fightfrac=%.2f |%s\n", b.t, b.st.name, top, w, o.Self.Cubes, len(by), b.t-first, total, dealt, float64(modes["fight"]+modes["fight+cube"])/float64(max(nm, 1)), s)
	}
	if Stats {
		b.modeLog = append(b.modeLog, modeRec{b.t, b.mode})
		if len(b.modeLog) > 400 {
			b.modeLog = b.modeLog[200:]
		}
	}
	// never emit garbage
	if math.IsNaN(act.Move[0]) || math.IsNaN(act.Move[1]) {
		act.Move = [2]float64{0, 0}
	}
	if math.IsNaN(act.Aim[0]) || math.IsNaN(act.Aim[1]) {
		act.HasAim = false
	}
	b.lastMove = act.Move
	if act.HasAim {
		b.lastAimCmd = act.Aim
	}
	return act
}

func (b *Bot) mech(key string, def float64) float64 { return stat(b.C.Mech, key, def) }

// observe folds the observation into memory.
func (b *Bot) observe(o *proto.Obs) {
	b.o = o
	b.t = o.T
	s := &o.Self
	b.pos, b.vel, b.aim = s.Pos, s.Vel, s.Aim
	b.hp, b.maxHP, b.energy = s.HP, s.MaxHP, s.Energy
	if b.maxHP <= 0 {
		b.maxHP = 1
	}
	st := s.Stats
	baseMuzzle := b.mech("muzzle_height", 7)
	b.scale = 1 + b.sizePerCube*float64(s.Cubes)
	b.muzzleH = stat(st, "muzzle_height", baseMuzzle*b.scale)
	b.eyeH = stat(st, "eye_height", b.mech("eye_height", 8)*b.scale)
	// stats may or may not carry the size-scaled numbers: trust whichever is larger
	if b.muzzleH < baseMuzzle*b.scale-1e-6 {
		b.muzzleH = baseMuzzle * b.scale
	}
	if e := b.mech("eye_height", 8) * b.scale; b.eyeH < e-1e-6 {
		b.eyeH = e
	}
	b.radius = math.Max(stat(st, "radius", 0), b.mech("radius", 2.5)*b.scale)
	b.height = math.Max(stat(st, "height", 0), b.mech("height", 9)*b.scale)
	b.walk = stat(st, "walk_speed", b.mech("walk_speed", 11.7))
	b.boostV = stat(st, "boost_speed", b.mech("boost_speed", 19.5))
	b.turn = stat(st, "turn_rate", b.mech("turn_rate", 4))
	b.pitchR = stat(st, "pitch_rate", b.mech("pitch_rate", 3))
	b.armor = stat(st, "armor", 0)
	for i := 0; i < 2; i++ {
		b.slots[i] = slot{}
		if i < len(s.Weapons) && s.Weapons[i] != nil {
			w := s.Weapons[i]
			b.slots[i] = slot{ok: true, kind: w.Kind, cooldown: w.Cooldown, ammo: w.Ammo, reload: w.Reload, charge: w.Charge, stats: w.Stats}
			if b.slots[i].stats == nil {
				b.slots[i].stats = b.C.Weapons[w.Kind]
			}
		}
	}

	// decay
	b.recentDmg *= math.Exp(-b.decide / 3.0)
	for _, tr := range b.tracks {
		tr.WasListed = tr.Listed
		tr.Listed = false
		tr.LOS = false
		tr.HurtMe *= math.Exp(-b.decide / 4.0)
		tr.IHurt *= math.Exp(-b.decide / 4.0)
	}
	// which walkable region are we in (the stadium floor, for one, is cut off from the rest)
	if c := b.Nav.Nearest(b.pos[0], b.pos[1], 2.9, 9, 4); c >= 0 && b.onGroundLevel() {
		b.myComp = b.Nav.Components()[c]
	}

	eye := [3]float64{b.pos[0], b.pos[1], b.pos[2] + b.eyeH}
	fovHalf := b.C.Vision.FovDeg * math.Pi / 360
	if fovHalf <= 0 {
		fovHalf = math.Pi / 3
	}
	visRange := b.C.Vision.Range + stat(s.Stats, "vision_bonus", 0)
	nearRange := b.C.Vision.NearRange + stat(s.Stats, "near_range_bonus", 0)
	for i := range o.Mechs {
		m := &o.Mechs[i]
		tr := b.tracks[m.ID]
		if tr == nil {
			tr = &Track{ID: m.ID, NPC: m.Kind == "npc", HurtMeT: -100, IHurtT: -100, AimAtMeT: -100, LOST: -100, SeenT0: b.t, PhantomT: -100, ViewT: -100, EngT: -100}
			b.tracks[m.ID] = tr
		}
		if b.t-tr.LastSeen > 6 {
			tr.SeenT0 = b.t
		}
		tr.Pos, tr.Vel, tr.Aim = m.Pos, m.Vel, m.Aim
		if m.MaxHP > 0 {
			tr.HP, tr.MaxHP = m.HP, m.MaxHP
		} else if tr.MaxHP == 0 {
			tr.HP, tr.MaxHP = 1400, 1400
		}
		tr.Cubes = m.Cubes
		tr.Charging = m.Charging
		tr.LastSeen = b.t
		tr.Listed = true
		if !tr.NPC {
			tr.Gun, tr.Laser, tr.Mortar, tr.Light = false, false, false, false
			for _, w := range m.Weapons {
				switch w {
				case "gun":
					tr.Gun = true
				case "laser":
					tr.Laser = true
				case "mortar":
					tr.Mortar = true
				case "lightning":
					tr.Light = true
				}
			}
		}
		sc := 1.0
		if !tr.NPC {
			sc = tr.scale(b.sizePerCube)
		}
		ch := 4.5 * sc
		if tr.NPC {
			ch = stat(b.C.Npc, "height", 3.5) * 0.5
		}
		// the server's rule: we see it when our eye has a line to its centre or to its top
		tr.LOS = b.W.Visible(eye[0], eye[1], eye[2], m.Pos[0], m.Pos[1], m.Pos[2]+ch) ||
			b.W.Visible(eye[0], eye[1], eye[2], m.Pos[0], m.Pos[1], m.Pos[2]+ch*1.9)
		if tr.LOS {
			tr.LOST = b.t
			if !tr.NPC {
				b.lastEnemyLOST = b.t
			}
			// would the server say we see it right now (as opposed to remembering it)?
			d3 := dist3(eye, [3]float64{m.Pos[0], m.Pos[1], m.Pos[2] + ch})
			if d3 < nearRange-1 || (d3 < visRange-2 && math.Abs(angDiff(math.Atan2(m.Pos[1]-b.pos[1], m.Pos[0]-b.pos[0]), b.aim[0])) < fovHalf-0.05) {
				tr.ViewT = b.t
			}
		}
		if !tr.NPC && tr.LOS {
			// is its torso pointed at us?
			bear := math.Atan2(b.pos[1]-m.Pos[1], b.pos[0]-m.Pos[0])
			d := dist2(b.pos, m.Pos)
			tol := 0.12 + math.Atan2(b.radius*1.5, math.Max(d, 1))
			off := math.Abs(angDiff(m.Aim[0], bear))
			if off < tol {
				tr.AimAtMeT = b.t
			}
			if b.P.LeadPersist || b.P.LeadCum {
				b.leadRecord(tr, off < 0.35)
			}
		}
	}
	// forget stale tracks (dead or far gone)
	mem := b.C.Vision.MemoryS
	if mem <= 0 {
		mem = 5
	}
	for id, tr := range b.tracks {
		if b.t-tr.LastSeen > 45 && b.t-tr.HurtMeT > 45 {
			delete(b.tracks, id)
			continue
		}
		// The server keeps listing a mech for memory_s after we last saw it. One that drops off the
		// list while we were looking straight at it a moment ago is dead, not hidden.
		if tr.WasListed && !tr.Listed && b.t-tr.ViewT < mem-1.5 {
			delete(b.tracks, id)
			if b.targetID == id {
				b.targetID = -1
			}
		}
	}

	// events
	for i := range o.Events {
		e := &o.Events[i]
		switch e.Type {
		case "damaged":
			b.lastDamageT = b.t
			if e.Weapon == "npc" {
				// drones are a nuisance, not a fight: tracked separately (see think)
				b.lastNpcDT = b.t
				if e.Bearing != nil {
					b.lastDmgBear = *e.Bearing
				}
				if e.By != nil {
					if tr := b.tracks[*e.By]; tr != nil {
						tr.HurtMeT = b.t
					}
				}
			} else if e.Weapon != "zone" {
				b.lastWeaponDT = b.t
				b.recentDmg += e.Amount
				if e.Bearing != nil {
					b.lastDmgBear = *e.Bearing
				}
				if e.By != nil && *e.By != s.ID {
					b.lastDmgBy = *e.By
					tr := b.tracks[*e.By]
					if tr == nil {
						tr = &Track{ID: *e.By, NPC: *e.By >= 100, HurtMeT: -100, IHurtT: -100, AimAtMeT: -100, LOST: -100, SeenT0: b.t, LastSeen: -100, HP: 1400, MaxHP: 1400, PhantomT: -100, ViewT: -100, EngT: -100}
						b.tracks[*e.By] = tr
					}
					if !tr.Listed && b.t-tr.LastSeen > 2 && e.Bearing != nil {
						// shot by something we cannot see: place it along the bearing at a typical range
						// for that weapon until we lay eyes on it
						guess := 115.0
						switch e.Weapon {
						case "laser":
							guess = 150
						case "lightning":
							guess = 35
						}
						tr.Pos = [3]float64{b.pos[0] + math.Cos(*e.Bearing)*guess, b.pos[1] + math.Sin(*e.Bearing)*guess, b.pos[2]}
						tr.Vel = [3]float64{}
						tr.PhantomT = b.t
					}
					tr.HurtMeT = b.t
					tr.HurtMe += e.Amount
					if Stats {
						d := -1.0
						if tr.Listed {
							d = dist2(b.pos, tr.Pos)
						}
						b.dmgLog = append(b.dmgLog, dmgRec{b.t, *e.By, e.Weapon, e.Amount, d, tr.Listed && tr.LOS})
					}
					switch e.Weapon {
					case "laser":
						tr.Laser = true
					case "mortar":
						tr.Mortar = true
					case "lightning":
						tr.Light = true
					}
				}
			}
		case "hit":
			b.lastDealtT = b.t
			if e.Target < 100 {
				b.creditT = b.t
				b.creditID = e.Target
				if Stats {
					b.hitLog = append(b.hitLog, hitRec{b.t, e.Amount})
				}
			}
			if tr := b.tracks[e.Target]; tr != nil {
				tr.IHurtT = b.t
				tr.IHurt += e.Amount
			}
		case "kill":
			b.kills++
			b.lastKillT = b.t
			delete(b.tracks, e.Target)
			if b.targetID == e.Target {
				b.targetID = -1
			}
		case "took":
			delete(b.pickups, e.ID.Num)
		case "take_failed":
			if e.Reason == "gone" || e.Reason == "have" {
				delete(b.pickups, e.ID.Num)
			}
		}
	}

	// items
	b.foldItems(o.Crates, b.crates, "crate")
	b.foldItems(o.Cubes, b.cubes, "cube")
	seen := map[int]bool{}
	for i := range o.Pickups {
		p := &o.Pickups[i]
		seen[p.ID] = true
		it := b.pickups[p.ID]
		if it == nil {
			it = &Item{ID: p.ID}
			b.pickups[p.ID] = it
		}
		it.Pos, it.Kind, it.LastSeen = p.Pos, p.Kind, b.t
		b.groundCheck(it)
	}
	b.expireUnseen(b.pickups, seen, eye)

	// sounds: remember the freshest loud one for looking around
	for i := range o.Sounds {
		sd := &o.Sounds[i]
		switch sd.Kind {
		case "gun", "laser", "lightning", "mortar", "laser_charge", "boost", "step", "jet", "reload":
			if sd.Dist != "far" {
				b.lastSoundT = b.t - sd.Age
				b.lastSoundB = sd.Bearing
			}
		}
	}
	// evidence about the lobby: beams fired by others whose entire path stays far from us mean mechs fight each other
	sawFFABeam := false
	for i := range o.Beams {
		bm := &o.Beams[i]
		if bm.By == s.ID {
			continue
		}
		if b.P.LaserCDTrack && bm.Kind == "laser" {
			if tr := b.tracks[bm.By]; tr != nil {
				tr.LaserFiredT = b.t - bm.Age
			}
		}
		if len(bm.Path) >= 2 {
			p0 := bm.Path[0]
			end := bm.Path[len(bm.Path)-1]
			segD, _ := segPointDist2D(b.pos[0], b.pos[1], p0[0], p0[1], end[0], end[1])
			if dist2(end, b.pos) > 25 && segD > 22 {
				b.ffaSeenT = b.t
				sawFFABeam = true
			}
		}
	}
	if sawFFABeam {
		b.ffaBeams++
	}
}

func (b *Bot) foldItems(list []proto.ItemObs, m map[int]*Item, kind string) {
	eye := [3]float64{b.pos[0], b.pos[1], b.pos[2] + b.eyeH}
	seen := map[int]bool{}
	for i := range list {
		p := &list[i]
		seen[p.ID] = true
		it := m[p.ID]
		if it == nil {
			it = &Item{ID: p.ID, Kind: kind}
			m[p.ID] = it
		}
		it.Pos, it.HP, it.LastSeen = p.Pos, p.HP, b.t
		b.groundCheck(it)
	}
	b.expireUnseen(m, seen, eye)
}

// expireUnseen drops remembered items that we should be seeing right now but are not listed.
func (b *Bot) expireUnseen(m map[int]*Item, seen map[int]bool, eye [3]float64) {
	near := b.C.Vision.NearRange * 0.85
	for id, it := range m {
		if seen[id] {
			continue
		}
		if b.t-it.LastSeen > 240 {
			delete(m, id)
			continue
		}
		d := dist3(it.Pos, eye)
		if d < near && b.W.Visible(eye[0], eye[1], eye[2], it.Pos[0], it.Pos[1], it.Pos[2]+0.8) {
			delete(m, id)
		}
	}
}

// groundCheck marks whether an item can be reached by walking.
func (b *Bot) groundCheck(it *Item) {
	k := b.Nav.Index(it.Pos[0], it.Pos[1])
	it.Cell = k
	it.Ground = false
	if !b.Nav.Solid[k] && math.Abs(float64(b.Nav.Z[k])-it.Pos[2]) < 3.0 {
		it.Ground = true
		return
	}
	// just inside a wall cell: look for an adjacent walkable cell at the same height
	if c := b.Nav.Nearest(it.Pos[0], it.Pos[1], b.radius, b.height, 2); c >= 0 && math.Abs(float64(b.Nav.Z[c])-it.Pos[2]) < 3.0 {
		it.Cell = c
		it.Ground = true
	}
}

func (b *Bot) has(kind string) int {
	for i := 0; i < 2; i++ {
		if b.slots[i].ok && b.slots[i].kind == kind {
			return i
		}
	}
	return -1
}

func (b *Bot) cubeDamageMul(cubes int) float64 {
	capc := b.C.Cubes.StatCap
	c := float64(cubes)
	if c > capc {
		c = capc
	}
	return 1 + b.C.Cubes.DamagePerCube*c
}
