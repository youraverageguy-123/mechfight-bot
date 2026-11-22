// Package proto holds the mechfight/1 message types (see protocol.md and mechfight.schema.json).
package proto

import (
	"strconv"
)

type Hello struct {
	Type     string `json:"type"`
	Protocol string `json:"protocol"`
	Slot     int    `json:"slot"`
	Seed     int64  `json:"seed"`
	Map      struct {
		Dir   string   `json:"dir"`
		Files []string `json:"files"`
	} `json:"map"`
	Config Config `json:"config"`
}

type ZonePhase struct {
	HoldS   float64 `json:"hold_s"`
	ShrinkS float64 `json:"shrink_s"`
	Radius  float64 `json:"radius"`
	Dps     float64 `json:"dps"`
}

type Effect struct {
	Path string   `json:"path"`
	Mul  *float64 `json:"mul,omitempty"`
	Add  *float64 `json:"add,omitempty"`
}

type UpgradeDef struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Desc     string   `json:"desc"`
	Requires string   `json:"requires,omitempty"`
	Max      int      `json:"max"`
	Effects  []Effect `json:"effects"`
}

type Config struct {
	Match  map[string]float64 `json:"match"`
	Bot    map[string]float64 `json:"bot"`
	Mech   map[string]float64 `json:"mech"`
	Vision struct {
		Range        float64 `json:"range"`
		FovDeg       float64 `json:"fov_deg"`
		NearRange    float64 `json:"near_range"`
		CanopyBlocks bool    `json:"canopy_blocks"`
		ShowEnemyHP  bool    `json:"show_enemy_hp"`
		MemoryS      float64 `json:"memory_s"`
	} `json:"vision"`
	Sound struct {
		BearingNoiseDeg float64            `json:"bearing_noise_deg"`
		Near            float64            `json:"near"`
		Mid             float64            `json:"mid"`
		StepSpeed       float64            `json:"step_speed"`
		StepEvery       float64            `json:"step_every"`
		Range           map[string]float64 `json:"range"`
	} `json:"sound"`
	Weapons    map[string]map[string]float64 `json:"weapons"`
	Projectile map[string]float64            `json:"projectile"`
	Cubes      struct {
		PickupRadius      float64   `json:"pickup_radius"`
		HPPerCube         float64   `json:"hp_per_cube"`
		DamagePerCube     float64   `json:"damage_per_cube"`
		StatCap           float64   `json:"stat_cap"`
		DeathDropExtra    float64   `json:"death_drop_extra"`
		SizePerCube       float64   `json:"size_per_cube"`
		UpgradeThresholds []float64 `json:"upgrade_thresholds"`
	} `json:"cubes"`
	Crates map[string]float64 `json:"crates"`
	Loot   struct {
		PickupRadius      float64            `json:"pickup_radius"`
		EquipS            float64            `json:"equip_s"`
		FloorCount        float64            `json:"floor_count"`
		FloorRoofFraction float64            `json:"floor_roof_fraction"`
		CrateWeaponChance float64            `json:"crate_weapon_chance"`
		DeathDrop         float64            `json:"death_drop"`
		Weights           map[string]float64 `json:"weights"`
	} `json:"loot"`
	Npc   map[string]float64 `json:"npc"`
	Drops *struct {
		Times    []float64 `json:"times"`
		FallS    float64   `json:"fall_s"`
		Radius   float64   `json:"radius"`
		CaptureS float64   `json:"capture_s"`
		Cubes    float64   `json:"cubes"`
		Repair   float64   `json:"repair"`
		Spread   float64   `json:"spread"`
	} `json:"drops,omitempty"`
	Zone struct {
		StartCenter []float64   `json:"start_center"`
		StartRadius float64     `json:"start_radius"`
		Phases      []ZonePhase `json:"phases"`
	} `json:"zone"`
	Replay   map[string]float64 `json:"replay"`
	Debug    map[string]float64 `json:"debug"`
	Upgrades []UpgradeDef       `json:"upgrades"`
}

type WeaponObs struct {
	Kind     string             `json:"kind"`
	Cooldown float64            `json:"cooldown"`
	Ammo     float64            `json:"ammo"`
	Reload   float64            `json:"reload"`
	Charge   float64            `json:"charge"`
	Stats    map[string]float64 `json:"stats"`
}

type SelfObs struct {
	ID        int                `json:"id"`
	Pos       [3]float64         `json:"pos"`
	Vel       [3]float64         `json:"vel"`
	Aim       [2]float64         `json:"aim"`
	HP        float64            `json:"hp"`
	MaxHP     float64            `json:"max_hp"`
	Energy    float64            `json:"energy"`
	MaxEnergy float64            `json:"max_energy"`
	Grounded  bool               `json:"grounded"`
	Cubes     int                `json:"cubes"`
	Upgrades  []string           `json:"upgrades"`
	InZone    bool               `json:"in_zone"`
	Weapons   []*WeaponObs       `json:"weapons"`
	Stats     map[string]float64 `json:"stats"`
}

type ActorObs struct {
	ID       int        `json:"id"`
	Kind     string     `json:"kind"`
	Pos      [3]float64 `json:"pos"`
	Vel      [3]float64 `json:"vel"`
	Aim      [2]float64 `json:"aim"`
	HP       float64    `json:"hp"`
	MaxHP    float64    `json:"max_hp"`
	Cubes    int        `json:"cubes"`
	Weapons  []string   `json:"weapons,omitempty"`
	Charging bool       `json:"charging,omitempty"`
}

type ProjectileObs struct {
	ID   int        `json:"id"`
	Kind string     `json:"kind"`
	Pos  [3]float64 `json:"pos"`
	Vel  [3]float64 `json:"vel"`
}

type ItemObs struct {
	ID  int        `json:"id"`
	Pos [3]float64 `json:"pos"`
	HP  float64    `json:"hp,omitempty"`
}

type PickupObs struct {
	ID   int        `json:"id"`
	Kind string     `json:"kind"`
	Pos  [3]float64 `json:"pos"`
}

type SoundObs struct {
	Kind    string  `json:"kind"`
	Bearing float64 `json:"bearing"`
	Dist    string  `json:"dist"`
	Age     float64 `json:"age"`
}

type BeamObs struct {
	By   int          `json:"by"`
	Kind string       `json:"kind"`
	Path [][3]float64 `json:"path"`
	Age  float64      `json:"age"`
}

// Event is the union of every event type; unused fields stay zero / nil.
type Event struct {
	Type    string   `json:"type"`
	Amount  float64  `json:"amount,omitempty"`
	Weapon  string   `json:"weapon,omitempty"`
	Bearing *float64 `json:"bearing,omitempty"`
	By      *int     `json:"by,omitempty"`
	Rear    bool     `json:"rear,omitempty"`
	Target  int      `json:"target,omitempty"`
	Effect  string   `json:"effect,omitempty"`
	ID      IDValue  `json:"id,omitempty"`
	Cubes   int      `json:"cubes,omitempty"`
	Kind    string   `json:"kind,omitempty"`
	Slot    int      `json:"slot,omitempty"`
	Reason  string   `json:"reason,omitempty"`
}

// IDValue is an event id that is a number for most events and a string for "upgrade".
type IDValue struct {
	Num int
	Str string
}

func (v *IDValue) UnmarshalJSON(b []byte) error {
	if len(b) > 0 && b[0] == '"' {
		s, err := strconv.Unquote(string(b))
		if err != nil {
			return nil
		}
		v.Str = s
		return nil
	}
	f, err := strconv.ParseFloat(string(b), 64)
	if err == nil {
		v.Num = int(f)
	}
	return nil
}

func (v IDValue) MarshalJSON() ([]byte, error) {
	if v.Str != "" {
		return []byte(strconv.Quote(v.Str)), nil
	}
	return []byte(strconv.Itoa(v.Num)), nil
}

type DropObs struct {
	ID       int        `json:"id"`
	Pos      [3]float64 `json:"pos"`
	Radius   float64    `json:"radius"`
	LandsIn  float64    `json:"lands_in"`
	CaptureS float64    `json:"capture_s"`
	Progress float64    `json:"progress"`
	Holder   *int       `json:"holder"`
}

type ZoneObs struct {
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
	Name string `json:"name"`
	Desc string `json:"desc"`
}

type Offer struct {
	ID      int           `json:"id"`
	Expires float64       `json:"expires"`
	Choices []OfferChoice `json:"choices"`
}

type Obs struct {
	Type        string          `json:"type"`
	Tick        int             `json:"tick"`
	T           float64         `json:"t"`
	Alive       int             `json:"alive"`
	Self        SelfObs         `json:"self"`
	Mechs       []ActorObs      `json:"mechs"`
	Projectiles []ProjectileObs `json:"projectiles"`
	Crates      []ItemObs       `json:"crates"`
	Cubes       []ItemObs       `json:"cubes"`
	Pickups     []PickupObs     `json:"pickups"`
	Sounds      []SoundObs      `json:"sounds"`
	Beams       []BeamObs       `json:"beams"`
	Events      []Event         `json:"events"`
	Drops       []DropObs       `json:"drops"`
	Zone        ZoneObs         `json:"zone"`
	Offer       *Offer          `json:"offer"`
}

// Action is one decision. Pick/Take/Drop use -1 for "not set".
type Action struct {
	Tick   int
	Move   [2]float64
	Boost  bool
	Jump   bool
	Aim    [2]float64
	HasAim bool
	Fire   [2]bool
	Reload bool
	Pick   int
	Take   int
	Drop   int
	Label  string // optional debug label
}

// NewAction returns an action with the optional fields unset.
func NewAction(tick int) Action {
	return Action{Tick: tick, Pick: -1, Take: -1, Drop: -1}
}

func appendF(b []byte, v float64, prec int) []byte {
	if v != v || v > 1e15 || v < -1e15 { // NaN/Inf guard: never emit invalid JSON
		return append(b, '0')
	}
	return strconv.AppendFloat(b, v, 'f', prec, 64)
}

func appendBool(b []byte, v bool) []byte {
	if v {
		return append(b, "true"...)
	}
	return append(b, "false"...)
}

// AppendJSON serialises the action as one JSON line (without the trailing newline).
func (a *Action) AppendJSON(b []byte) []byte {
	b = append(b, `{"tick":`...)
	b = strconv.AppendInt(b, int64(a.Tick), 10)
	b = append(b, `,"move":[`...)
	b = appendF(b, a.Move[0], 4)
	b = append(b, ',')
	b = appendF(b, a.Move[1], 4)
	b = append(b, `],"boost":`...)
	b = appendBool(b, a.Boost)
	b = append(b, `,"jump":`...)
	b = appendBool(b, a.Jump)
	if a.HasAim {
		b = append(b, `,"aim":[`...)
		b = appendF(b, a.Aim[0], 5)
		b = append(b, ',')
		b = appendF(b, a.Aim[1], 5)
		b = append(b, ']')
	}
	b = append(b, `,"fire":[`...)
	b = appendBool(b, a.Fire[0])
	b = append(b, ',')
	b = appendBool(b, a.Fire[1])
	b = append(b, ']')
	if a.Reload {
		b = append(b, `,"reload":true`...)
	}
	if a.Pick >= 0 {
		b = append(b, `,"pick":`...)
		b = strconv.AppendInt(b, int64(a.Pick), 10)
	}
	if a.Take >= 0 {
		b = append(b, `,"take":`...)
		b = strconv.AppendInt(b, int64(a.Take), 10)
	}
	if a.Drop >= 0 {
		b = append(b, `,"drop":`...)
		b = strconv.AppendInt(b, int64(a.Drop), 10)
	}
	if a.Label != "" {
		lab := a.Label
		if len(lab) > 70 {
			lab = lab[:70]
		}
		b = append(b, `,"debug":{"label":`...)
		b = strconv.AppendQuoteToASCII(b, lab)
		b = append(b, '}')
	}
	b = append(b, '}')
	return b
}
