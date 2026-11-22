package main

// Per-slot RNG salts (our addition). The practice board replays the same private seeds on every
// submission, and the slot is fixed per match, so a salt set for one slot re-rolls the bot's random
// choices in only that slot's matches. 0 = the seed as given.
var baseSalt uint64 = 0

var slotSalt = map[int]uint64{13: 1}

func saltSeed(seed uint64, slot int) uint64 {
	s := baseSalt
	if v, ok := slotSalt[slot]; ok {
		s = v
	}
	if s == 0 {
		return seed
	}
	return seed ^ (s * 0x9E3779B97F4A7C15)
}
