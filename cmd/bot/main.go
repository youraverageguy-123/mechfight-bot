// Command bot speaks mechfight/1 on stdin/stdout.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"runtime/debug"

	"mechroyale/internal/bot"
	"mechroyale/internal/proto"
	"mechroyale/internal/world"
)

type agent interface {
	Act(o *proto.Obs) proto.Action
}

// blind is the fallback when the map cannot be loaded: walk to the zone, shoot what we see.
type blind struct{}

func (blind) Act(o *proto.Obs) proto.Action {
	a := proto.NewAction(o.Tick)
	me := o.Self
	gx, gy := o.Zone.NextCenter[0], o.Zone.NextCenter[1]
	yaw := math.Atan2(gy-me.Pos[1], gx-me.Pos[0])
	if math.Hypot(gx-me.Pos[0], gy-me.Pos[1]) > 10 {
		a.Move = [2]float64{math.Cos(yaw), math.Sin(yaw)}
	}
	a.Aim = [2]float64{yaw, 0}
	a.HasAim = true
	a.Boost = !me.InZone
	best := -1.0
	for i := range o.Mechs {
		m := &o.Mechs[i]
		if m.Kind != "mech" {
			continue
		}
		d := math.Hypot(m.Pos[0]-me.Pos[0], m.Pos[1]-me.Pos[1])
		if best < 0 || d < best {
			best = d
			tf := d / 252
			a.Aim = [2]float64{math.Atan2(m.Pos[1]+m.Vel[1]*tf-me.Pos[1], m.Pos[0]+m.Vel[0]*tf-me.Pos[0]), math.Atan2(m.Pos[2]+4.5-me.Pos[2]-7, math.Max(d, 1))}
			a.Fire = [2]bool{true, true}
		}
	}
	if o.Offer != nil {
		a.Pick = 0
	}
	return a
}

func logf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

func loadWorld(dirs ...string) *world.World {
	for _, d := range dirs {
		if d == "" {
			continue
		}
		w, err := world.Load(d)
		if err == nil {
			return w
		}
		logf("world load from %q failed: %v", d, err)
	}
	return nil
}

func safeAct(ag agent, o *proto.Obs, last *proto.Action) (act proto.Action) {
	defer func() {
		if r := recover(); r != nil {
			logf("panic at tick %d: %v\n%s", o.Tick, r, debug.Stack())
			act = proto.NewAction(o.Tick)
			act.Move = last.Move
			act.Boost = last.Boost
			act.Aim = last.Aim
			act.HasAim = last.HasAim
		}
	}()
	return ag.Act(o)
}

func main() {
	// few, late collections: the live heap is small and we have 512 MB
	debug.SetGCPercent(800)
	debug.SetMemoryLimit(380 << 20)

	lines := make(chan []byte, 256)
	go func() {
		rd := bufio.NewReaderSize(os.Stdin, 1<<20)
		for {
			line, err := rd.ReadBytes('\n')
			if len(bytes.TrimSpace(line)) > 0 {
				lines <- line
			}
			if err != nil {
				close(lines)
				return
			}
		}
	}()

	out := bufio.NewWriterSize(os.Stdout, 1<<16)
	var ag agent
	var last proto.Action
	buf := make([]byte, 0, 512)
	panics := 0
	badObs := 0

	handle := func(line []byte) (obsLine []byte, done bool) {
		// cheap type sniff: every server message starts with {"type":"..."
		var head struct {
			Type string `json:"type"`
		}
		if bytes.Contains(line[:min(len(line), 40)], []byte(`"obs"`)) {
			return line, false
		}
		if err := json.Unmarshal(line, &head); err != nil {
			return nil, false
		}
		switch head.Type {
		case "obs":
			return line, false
		case "end":
			return nil, true
		case "hello":
			var h proto.Hello
			if err := json.Unmarshal(line, &h); err != nil {
				logf("bad hello: %v (retrying sanitised)", err)
				h = proto.Hello{}
				if err2 := json.Unmarshal(stripZeroFractions(line), &h); err2 != nil {
					logf("bad hello again: %v", err2)
				}
			}
			name := "ironclad"
			func() {
				defer func() {
					if r := recover(); r != nil {
						logf("init panic: %v\n%s", r, debug.Stack())
						ag = blind{}
					}
				}()
				w := loadWorld(h.Map.Dir, "/arena", "arena")
				if w == nil {
					ag = blind{}
					return
				}
				nav := world.NewNav(w, stat(h.Config.Mech, "step_up", 1.2), stat(h.Config.Mech, "height", 9))
				b := bot.Factory(&h, w, nav)
				name = "ohio-ironclad"
				ag = b
			}()
			fmt.Fprintf(out, "{\"type\":\"ready\",\"name\":%q}\n", name)
			out.Flush()
		}
		return nil, false
	}

	for line := range lines {
		latest, done := handle(line)
		if done {
			return
		}
		// drain whatever else is queued: only the newest observation matters
	drain:
		for {
			select {
			case more, ok := <-lines:
				if !ok {
					break drain
				}
				l2, d2 := handle(more)
				if d2 {
					return
				}
				if l2 != nil {
					latest = l2
				}
			default:
				break drain
			}
		}
		if latest == nil {
			continue
		}
		var o proto.Obs
		if err := json.Unmarshal(latest, &o); err != nil {
			// Never go silent over a formatting surprise: retry with integer-valued floats
			// ("5.0") rewritten as integers, and if that fails too, coast on the last action.
			o = proto.Obs{}
			if err2 := json.Unmarshal(stripZeroFractions(latest), &o); err2 != nil {
				badObs++
				if badObs <= 5 || badObs%200 == 0 {
					logf("bad obs (%d so far): %v", badObs, err)
				}
				if tick, ok := sniffTick(latest); ok {
					act := proto.NewAction(tick)
					act.Move, act.Boost, act.Aim, act.HasAim = last.Move, last.Boost, last.Aim, last.HasAim
					buf = act.AppendJSON(buf[:0])
					buf = append(buf, '\n')
					out.Write(buf)
					out.Flush()
				}
				continue
			}
		}
		if ag == nil {
			ag = blind{}
		}
		act := safeAct(ag, &o, &last)
		if act.Tick != o.Tick {
			act.Tick = o.Tick
			panics++
		}
		last = act
		buf = act.AppendJSON(buf[:0])
		buf = append(buf, '\n')
		out.Write(buf)
		out.Flush()
	}
}

// stripZeroFractions rewrites JSON numbers like 12.0 or 7.000 (outside strings) as 12 and 7, so a
// server that prints whole numbers as floats cannot break decoding into integer fields.
func stripZeroFractions(in []byte) []byte {
	out := make([]byte, 0, len(in))
	inStr, esc := false, false
	for i := 0; i < len(in); i++ {
		c := in[i]
		if inStr {
			out = append(out, c)
			if esc {
				esc = false
			} else if c == '\\' {
				esc = true
			} else if c == '"' {
				inStr = false
			}
			continue
		}
		if c == '"' {
			inStr = true
			out = append(out, c)
			continue
		}
		if c == '.' && i > 0 && in[i-1] >= '0' && in[i-1] <= '9' {
			j := i + 1
			for j < len(in) && in[j] == '0' {
				j++
			}
			// only zeros after the point, and the number ends there
			if j > i+1 && j < len(in) && (in[j] == ',' || in[j] == ']' || in[j] == '}' || in[j] == ' ' || in[j] == '\n') {
				i = j - 1
				continue
			}
		}
		out = append(out, c)
	}
	return out
}

// sniffTick pulls "tick":N out of a line we could not decode.
func sniffTick(line []byte) (int, bool) {
	k := bytes.Index(line, []byte(`"tick":`))
	if k < 0 {
		return 0, false
	}
	n, seen := 0, false
	for i := k + 7; i < len(line); i++ {
		c := line[i]
		if c == ' ' && !seen {
			continue
		}
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
		seen = true
	}
	return n, seen
}

func stat(m map[string]float64, key string, def float64) float64 {
	if v, ok := m[key]; ok {
		return v
	}
	return def
}
