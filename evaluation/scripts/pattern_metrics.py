#!/usr/bin/env python3
"""RQ1 size/primitives of the evaluated Patterns -> results/expressiveness/patterns.csv."""
import csv, glob, os
import yaml

EVAL = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..")
rows = []
for f in sorted(glob.glob(os.path.join(EVAL, "patterns", "*", "*.yaml"))):
    if "/_" in f:
        continue
    text = open(f).read()
    p = yaml.safe_load(text)
    spec = p["spec"]
    rels = [r for g in ("matchAll", "matchAny", "matchNone") for r in (spec.get("relationships") or {}).get(g, [])]
    crit = [c for r in rels for c in r.get("criteria", [])]
    filt = [c for res in [spec["target"], *spec.get("dependencies", [])] for g in ("matchAll", "matchAny", "matchNone")
            for c in (res.get("filters") or {}).get(g, [])]
    paths = [c["dependencyPath"] for c in crit] + [c["targetPath"] for c in crit] + [c["path"] for c in filt]
    rows.append({
        "platform": os.path.basename(os.path.dirname(f)), "pattern": p["metadata"]["name"],
        "loc": sum(1 for l in text.splitlines() if l.strip() and not l.strip().startswith("#")),
        "dependencies": len(spec.get("dependencies", [])), "relationship_entries": len(rels),
        "criteria": len(crit), "filters": len(filt),
        "wildcard_paths": sum("[*]" in x for x in paths), "nested_wildcards": sum(x.count("[*]") > 1 for x in paths),
        "types": "+".join(sorted({r["type"] for r in rels})),
        "groups": "+".join(sorted({g for g in ("matchAll", "matchAny", "matchNone") if (spec.get("relationships") or {}).get(g)})),
    })
out = os.path.join(EVAL, "results", "expressiveness", "patterns.csv")
os.makedirs(os.path.dirname(out), exist_ok=True)
with open(out, "w", newline="") as fh:
    w = csv.DictWriter(fh, fieldnames=list(rows[0]))
    w.writeheader(); w.writerows(rows)
loc = [r["loc"] for r in rows]
print(f"{len(rows)} patterns, LOC min/median/max = {min(loc)}/{sorted(loc)[len(loc)//2]}/{max(loc)}, total {sum(loc)}")
for r in rows: print(f"  {r['pattern']:42} loc={r['loc']:>3} deps={r['dependencies']} rels={r['relationship_entries']} crit={r['criteria']} filt={r['filters']} wild={r['wildcard_paths']} {r['types']}/{r['groups']}")
