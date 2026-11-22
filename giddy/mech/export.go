package mech

func (b *Brain) Travel(me *Self, gx, gy float64) (float64, float64) { return b.travel(me, gx, gy) }
func (b *Brain) SetNow(t float64)                                   { b.now = t }
