package mech

import "encoding/json"

type Vec3 [3]float64

type MechStats struct {
	Radius           float64 `json:"radius"`
	Height           float64 `json:"height"`
	EyeHeight        float64 `json:"eye_height"`
	MuzzleHeight     float64 `json:"muzzle_height"`
	HP               float64 `json:"hp"`
	WalkSpeed        float64 `json:"walk_speed"`
	BoostSpeed       float64 `json:"boost_speed"`
	GroundAccel      float64 `json:"ground_accel"`
	AirAccel         float64 `json:"air_accel"`
	Gravity          float64 `json:"gravity"`
	JumpSpeed        float64 `json:"jump_speed"`
	JetAccel         float64 `json:"jet_accel"`
	MaxRiseSpeed     float64 `json:"max_rise_speed"`
	StepUp           float64 `json:"step_up"`
	MaxSlopeDeg      float64 `json:"max_slope_deg"`
	Energy           float64 `json:"energy"`
	EnergyRegen      float64 `json:"energy_regen"`
	EnergyRegenDelay float64 `json:"energy_regen_delay"`
	BoostDrain       float64 `json:"boost_drain"`
	JetDrain         float64 `json:"jet_drain"`
	TurnRate         float64 `json:"turn_rate"`
	PitchRate        float64 `json:"pitch_rate"`
	MinPitch         float64 `json:"min_pitch"`
	MaxPitch         float64 `json:"max_pitch"`
	RegenDelay       float64 `json:"regen_delay"`
	RegenPerS        float64 `json:"regen_per_s"`
	ChargeMoveMult   float64 `json:"charge_move_mult"`
	SelfDamageMult   float64 `json:"self_damage_mult"`
	Armor            float64 `json:"armor"`
	RearMult         float64 `json:"rear_mult"`
	FrontArcDeg      float64 `json:"front_arc_deg"`
	NearRangeBonus   float64 `json:"near_range_bonus"`
	VisionBonus      float64 `json:"vision_bonus"`
	Hearing          float64 `json:"hearing"`
}

type WeaponStats struct {
	Damage       float64 `json:"damage"`
	Cooldown     float64 `json:"cooldown"`
	Speed        float64 `json:"speed,omitempty"`
	Range        float64 `json:"range,omitempty"`
	SpreadDeg    float64 `json:"spread_deg,omitempty"`
	Mag          float64 `json:"mag,omitempty"`
	Reload       float64 `json:"reload,omitempty"`
	Splash       float64 `json:"splash,omitempty"`
	SplashRadius float64 `json:"splash_radius,omitempty"`
	SplashEdge   float64 `json:"splash_edge,omitempty"`
	Gravity      float64 `json:"gravity,omitempty"`
	TTL          float64 `json:"ttl,omitempty"`
	Knock        float64 `json:"knock,omitempty"`
	KnockUp      float64 `json:"knock_up,omitempty"`
	Drain        float64 `json:"drain,omitempty"`
	Charge       float64 `json:"charge,omitempty"`
	MinCharge    float64 `json:"min_charge,omitempty"`
	MinFrac      float64 `json:"min_frac,omitempty"`
	HoldS        float64 `json:"hold_s,omitempty"`
	Pierce       float64 `json:"pierce,omitempty"`
	Cookoff      float64 `json:"cookoff,omitempty"`
	ConeDeg      float64 `json:"cone_deg,omitempty"`
	Chain        float64 `json:"chain,omitempty"`
	ChainRange   float64 `json:"chain_range,omitempty"`
	ChainFalloff float64 `json:"chain_falloff,omitempty"`
	Energy       float64 `json:"energy,omitempty"`
	ShortS       float64 `json:"short_s,omitempty"`
}

type ZonePhase struct {
	HoldS   float64 `json:"hold_s"`
	ShrinkS float64 `json:"shrink_s"`
	Radius  float64 `json:"radius"`
	Dps     float64 `json:"dps"`
}

type UpgradeEffect struct {
	Path string   `json:"path"`
	Mul  *float64 `json:"mul,omitempty"`
	Add  *float64 `json:"add,omitempty"`
}

type Upgrade struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Desc     string          `json:"desc"`
	Requires string          `json:"requires,omitempty"`
	Max      int             `json:"max"`
	Effects  []UpgradeEffect `json:"effects"`
}

type Config struct {
	Match struct {
		TickHz      float64 `json:"tick_hz"`
		DecideEvery int     `json:"decide_every"`
		DurationS   float64 `json:"duration_s"`
	} `json:"match"`
	Bot struct {
		TickBudgetMs float64 `json:"tick_budget_ms"`
	} `json:"bot"`
	Mech   MechStats `json:"mech"`
	Vision struct {
		Range     float64 `json:"range"`
		FovDeg    float64 `json:"fov_deg"`
		NearRange float64 `json:"near_range"`
		MemoryS   float64 `json:"memory_s"`
	} `json:"vision"`
	Weapons map[string]WeaponStats `json:"weapons"`
	Cubes   struct {
		PickupRadius      float64 `json:"pickup_radius"`
		HpPerCube         float64 `json:"hp_per_cube"`
		DamagePerCube     float64 `json:"damage_per_cube"`
		StatCap           int     `json:"stat_cap"`
		DeathDropExtra    int     `json:"death_drop_extra"`
		SizePerCube       float64 `json:"size_per_cube"`
		UpgradeThresholds []int   `json:"upgrade_thresholds"`
	} `json:"cubes"`
	Crates struct {
		Count  int     `json:"count"`
		HP     float64 `json:"hp"`
		Radius float64 `json:"radius"`
		Height float64 `json:"height"`
	} `json:"crates"`
	Loot struct {
		PickupRadius      float64 `json:"pickup_radius"`
		EquipS            float64 `json:"equip_s"`
		FloorCount        int     `json:"floor_count"`
		CrateWeaponChance float64 `json:"crate_weapon_chance"`
	} `json:"loot"`
	Npc struct {
		Count           int     `json:"count"`
		HP              float64 `json:"hp"`
		Radius          float64 `json:"radius"`
		Height          float64 `json:"height"`
		AggroRange      float64 `json:"aggro_range"`
		DeaggroRange    float64 `json:"deaggro_range"`
		Damage          float64 `json:"damage"`
		Cooldown        float64 `json:"cooldown"`
		ProjectileSpeed float64 `json:"projectile_speed"`
		SpreadDeg       float64 `json:"spread_deg"`
		Cubes           int     `json:"cubes"`
	} `json:"npc"`
	Drops struct {
		Times    []float64 `json:"times"`
		FallS    float64   `json:"fall_s"`
		Radius   float64   `json:"radius"`
		CaptureS float64   `json:"capture_s"`
		Cubes    int       `json:"cubes"`
	} `json:"drops"`
	Zone struct {
		StartCenter [2]float64  `json:"start_center"`
		StartRadius float64     `json:"start_radius"`
		Phases      []ZonePhase `json:"phases"`
	} `json:"zone"`
	Upgrades []Upgrade `json:"upgrades"`
}

type Hello struct {
	Type string `json:"type"`
	Slot int    `json:"slot"`
	Seed uint64 `json:"seed"`
	Map  struct {
		Dir string `json:"dir"`
	} `json:"map"`
	Config Config `json:"config"`
}

type WeaponState struct {
	Kind     string      `json:"kind"`
	Cooldown float64     `json:"cooldown"`
	Ammo     float64     `json:"ammo"`
	Reload   float64     `json:"reload"`
	Charge   float64     `json:"charge"`
	Stats    WeaponStats `json:"stats"`
}

type Self struct {
	ID        int            `json:"id"`
	Pos       Vec3           `json:"pos"`
	Vel       Vec3           `json:"vel"`
	Aim       [2]float64     `json:"aim"`
	HP        float64        `json:"hp"`
	MaxHP     float64        `json:"max_hp"`
	Energy    float64        `json:"energy"`
	MaxEnergy float64        `json:"max_energy"`
	Grounded  bool           `json:"grounded"`
	Cubes     int            `json:"cubes"`
	Upgrades  []string       `json:"upgrades"`
	InZone    bool           `json:"in_zone"`
	Weapons   []*WeaponState `json:"weapons"`
	Stats     MechStats      `json:"stats"`
}

type MechSeen struct {
	ID       int        `json:"id"`
	Kind     string     `json:"kind"`
	Pos      Vec3       `json:"pos"`
	Vel      Vec3       `json:"vel"`
	Aim      [2]float64 `json:"aim"`
	HP       float64    `json:"hp"`
	MaxHP    float64    `json:"max_hp"`
	Cubes    int        `json:"cubes"`
	Weapons  []string   `json:"weapons"`
	Charging bool       `json:"charging"`
}

type Proj struct {
	ID   int    `json:"id"`
	Kind string `json:"kind"`
	Pos  Vec3   `json:"pos"`
	Vel  Vec3   `json:"vel"`
}

type Item struct {
	ID   int     `json:"id"`
	Kind string  `json:"kind,omitempty"`
	Pos  Vec3    `json:"pos"`
	HP   float64 `json:"hp,omitempty"`
}

type Sound struct {
	Kind    string  `json:"kind"`
	Bearing float64 `json:"bearing"`
	Dist    string  `json:"dist"`
	Age     float64 `json:"age"`
}

type Event struct {
	Type    string          `json:"type"`
	Amount  float64         `json:"amount,omitempty"`
	Weapon  string          `json:"weapon,omitempty"`
	Bearing *float64        `json:"bearing,omitempty"`
	By      *int            `json:"by,omitempty"`
	Rear    bool            `json:"rear,omitempty"`
	Target  int             `json:"target,omitempty"`
	Effect  string          `json:"effect,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"`
	Kind    string          `json:"kind,omitempty"`
	Reason  string          `json:"reason,omitempty"`
	Cubes   int             `json:"cubes,omitempty"`
}

type Drop struct {
	ID       int     `json:"id"`
	Pos      Vec3    `json:"pos"`
	Radius   float64 `json:"radius"`
	LandsIn  float64 `json:"lands_in"`
	CaptureS float64 `json:"capture_s"`
	Progress float64 `json:"progress"`
	Holder   *int    `json:"holder"`
}

type Zone struct {
	Center      [2]float64 `json:"center"`
	Radius      float64    `json:"radius"`
	NextCenter  [2]float64 `json:"next_center"`
	NextRadius  float64    `json:"next_radius"`
	ShrinkStart float64    `json:"shrink_start"`
	ShrinkEnd   float64    `json:"shrink_end"`
	Dps         float64    `json:"dps"`
}

type OfferChoice struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
	Desc string `json:"desc,omitempty"`
}

type Offer struct {
	ID      int           `json:"id"`
	Expires float64       `json:"expires"`
	Choices []OfferChoice `json:"choices"`
}

type Obs struct {
	Type        string     `json:"type"`
	Tick        int        `json:"tick"`
	T           float64    `json:"t"`
	Alive       int        `json:"alive"`
	Self        Self       `json:"self"`
	Mechs       []MechSeen `json:"mechs"`
	Projectiles []Proj     `json:"projectiles"`
	Crates      []Item     `json:"crates"`
	Cubes       []Item     `json:"cubes"`
	Pickups     []Item     `json:"pickups"`
	Sounds      []Sound    `json:"sounds"`
	Events      []Event    `json:"events"`
	Drops       []Drop     `json:"drops"`
	Zone        Zone       `json:"zone"`
	Offer       *Offer     `json:"offer"`
}

type Action struct {
	Tick   int         `json:"tick"`
	Move   [2]float64  `json:"move"`
	Boost  bool        `json:"boost"`
	Jump   bool        `json:"jump"`
	Aim    *[2]float64 `json:"aim,omitempty"`
	Fire   [2]bool     `json:"fire"`
	Reload bool        `json:"reload"`
	Pick   *int        `json:"pick,omitempty"`
	Take   *int        `json:"take,omitempty"`
	Drop   *int        `json:"drop,omitempty"`
}

type Pilot interface {
	Act(o *Obs) Action
}
