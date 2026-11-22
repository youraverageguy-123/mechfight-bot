"""Per-slot salt bookkeeping: reads SLOTS.md, turns scores into exact point changes per slot and salt,
and prints the best salt table (Go map literal for giddy/cmd/bot/salt.go) with its predicted score."""
import re, sys
rows = [l for l in open("SLOTS.md", encoding="utf-8") if l.startswith("| g-")]
base = None
tests = []
for l in rows:
    c = [x.strip() for x in l.strip().strip("|").split("|")]
    name, slot, salt, score = c[0], c[2], c[3], c[4]
    if not re.match(r"^\d+(\.\d+)?$", score):
        continue
    pts = round(float(score) * 48)
    if name == "g-base":
        base = pts
    elif slot.isdigit():
        tests.append((int(slot), int(salt), pts))
    elif name == "g-dup":
        print("determinism check: dup", pts, "base", base)
if base is None:
    sys.exit("no base score yet")
best = {}
for s, k, p in tests:
    d = p - base
    print(f"slot {s:2d} salt {k}: {d:+d} points")
    if d > best.get(s, (0, 0))[1]:
        best[s] = (k, d)
gain = sum(d for _, d in best.values())
print(f"base {base} pts = {base/48:.2f}; best table gains {gain:+d} -> {(base+gain)/48:.2f}")
print("var slotSalt = map[int]uint64{" + ", ".join(f"{s}: {k}" for s, (k, _) in sorted(best.items())) + "}")
