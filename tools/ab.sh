#!/bin/bash
# Paired A/B over N seeds: tools/ab.sh N '<json A>' '<json B>' ...  (prints mean pts and paired diff vs the first)
N=$1; shift
i=0
for o in "$@"; do node tools/sim.ts $N 0.6 none 15 1 "$o" 2>/dev/null | grep '^{' > .ab_$i.jsonl; i=$((i+1)); done
python - "$@" <<'PY'
import json,sys,statistics as st
cfgs=sys.argv[1:]
R=[[json.loads(l) for l in open(f'.ab_{i}.jsonl')] for i in range(len(cfgs))]
base=[r['pts'] for r in R[0]]
for i,c in enumerate(cfgs):
    p=[r['pts'] for r in R[i]]; d=[a-b for a,b in zip(p,base)]
    print(f"{c[-75:]:77s} pts {st.mean(p):.2f}  diff {st.mean(d):+.2f} +-{(st.stdev(d)/len(d)**0.5 if len(d)>1 and i else 0):.2f}  kills {st.mean(r['kills'] for r in R[i]):.2f} cubes {st.mean(r['cubes'] for r in R[i]):.1f}")
PY
