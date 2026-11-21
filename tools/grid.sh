#!/bin/bash
# Parallel sim grid: tools/grid.sh N "ENV1;ENV2..." '<json A>' '<json B>' ...
# ENV entries look like "NH=aggressive AIMN=0.25 SKILLS=0,0.35,0.65,0.9" (separated by ';').
N=$1; ENVS=$2; shift 2
mkdir -p .grid; rm -f .grid/*.jsonl
IFS=';' read -ra EL <<< "$ENVS"
e=0
for en in "${EL[@]}"; do
  i=0
  for o in "$@"; do
    ( /usr/bin/env TIERS=1 $en node tools/sim.ts $N 0.6 none 15 1 "$o" 2>/dev/null | grep '^{' > .grid/r_${e}_${i}.jsonl ) &
    i=$((i+1))
  done
  e=$((e+1))
done
wait
python - "$ENVS" "$@" <<'PY'
import json,sys,statistics as st
envs=sys.argv[1].split(';'); cfgs=sys.argv[2:]
for e,en in enumerate(envs):
    print('==',en)
    R=[[json.loads(l) for l in open(f'.grid/r_{e}_{i}.jsonl')] for i in range(len(cfgs))]
    base=[r['pts'] for r in R[0]]
    for i,c in enumerate(cfgs):
        p=[r['pts'] for r in R[i]]; d=[a-b for a,b in zip(p,base)]
        se=(st.stdev(d)/len(d)**0.5) if len(d)>1 and i else 0
        print(f"  {c[-70:]:72s} pts {st.mean(p):5.2f} diff {st.mean(d):+.2f}+-{se:.2f} surv {st.mean(r['surv'] for r in R[i]):5.0f} cubes {st.mean(r['cubes'] for r in R[i]):4.1f}")
PY
