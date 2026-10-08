#!/usr/bin/env python3
"""RQ6a size/constructs of the Kyverno comparison policies, next to the KubePattern Patterns.

Writes measurements/comparison/expressiveness.csv (one row per smell) and prints a summary.
Metrics (non-comment, non-blank lines for LOC):
  kp_loc, kp_primitives       Pattern LOC; criteria + filters (from measurements/expressiveness/patterns.csv)
  kp_logic_loc                Pattern LOC from spec.target on (no metadata, display fields or message)
  kyv_loc                     ValidatingPolicy LOC (equivalent/)
  kyv_logic_loc               policy LOC from spec.matchConstraints on, without messageExpression
  kyv_loc_with_gce            kyv_loc + LOC of the GlobalContextEntries it reads (shared infrastructure)
  lookups                     globalContext.Get calls (one per dependency kind)
  cel_expressions             CEL expression fields (variables, matchConditions, validations, messages)
  cel_predicates              comparisons in the decision expressions (== != < <= > >= in), the counterpart of kp_primitives
  comprehensions              exists()/all() macros
  optional_hops               optional field/index selections (.? and [?)
  cel_chars                   characters of the decision expressions, whitespace-collapsed
  best_effort_loc / delta     same policy in best-effort/ (empty when there is no probe)
"""
import csv
import glob
import os
import re

import yaml

EVAL = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..")
KYV = os.path.join(EVAL, "comparison", "kyverno")
PRED = re.compile(r"==|!=|<=|>=|(?<![-=])<(?!=)|(?<![-=])>(?!=)|\bin\b")


def loc(text):
    return sum(1 for l in text.splitlines() if l.strip() and not l.strip().startswith("#"))


def logic_loc(text, start):
    """LOC from the line `start` on, skipping message/messageExpression blocks (key line + continuation)."""
    lines = [l for l in text.splitlines() if l.strip() and not l.strip().startswith("#")]
    lines = lines[next(i for i, l in enumerate(lines) if l.rstrip() == start):]
    n, skip = 0, None
    for l in lines:
        indent = len(l) - len(l.lstrip())
        if skip is not None and indent > skip:
            continue
        skip = indent if re.match(r"\s*(message|messageExpression):", l) else None
        n += skip is None
    return n


def cel_parts(policy):
    spec = policy["spec"]
    decision = [v["expression"] for v in spec.get("validations", [])] + [m["expression"] for m in spec.get("matchConditions", [])]
    fields = ([v["expression"] for v in spec.get("variables", [])] + decision
              + [v["messageExpression"] for v in spec.get("validations", []) if "messageExpression" in v])
    return decision, fields, [v["expression"] for v in spec.get("variables", [])]


def measure(path, gce_loc):
    text = open(path).read()
    pol = yaml.safe_load(text)
    decision, fields, variables = cel_parts(pol)
    gces = sorted({g for v in variables for g in re.findall(r'globalContext\.Get\("([^"]+)"', v)})
    dec = " ".join(decision)
    return {
        "loc": loc(text),
        "logic_loc": logic_loc(text, "  matchConstraints:"),
        "loc_with_gce": loc(text) + sum(gce_loc[g] for g in gces),
        "lookups": len(gces),
        "gce": "+".join(gces),
        "cel_expressions": len(fields),
        "cel_predicates": len(PRED.findall(re.sub(r"'[^']*'", "''", dec))),
        "comprehensions": len(re.findall(r"\.(?:exists|all)\(", dec)),
        "optional_hops": dec.count(".?") + dec.count("[?"),
        "cel_chars": len(re.sub(r"\s+", " ", dec).strip()),
    }


def main():
    gce_loc = {}
    for f in glob.glob(os.path.join(KYV, "globalcontext", "*.yaml")):
        gce_loc[yaml.safe_load(open(f))["metadata"]["name"]] = loc(open(f).read())
    kp = {r["pattern"]: r for r in csv.DictReader(open(os.path.join(EVAL, "measurements", "expressiveness", "patterns.csv")))}
    kp_file = {yaml.safe_load(open(f))["metadata"]["name"]: f
               for f in glob.glob(os.path.join(EVAL, "patterns", "*", "*.yaml")) if "/_" not in f}
    probes = {}
    for f in glob.glob(os.path.join(EVAL, "scenarios", "*", "ground-truth.csv")):
        for r in csv.DictReader(open(f)):
            if r["class"].startswith("probe-"):
                probes.setdefault(r["pattern"], []).append(r["class"][len("probe-"):])

    rows = []
    for f in sorted(glob.glob(os.path.join(KYV, "equivalent", "*.yaml"))):
        name = os.path.basename(f)[:-5]
        m = measure(f, gce_loc)
        be = os.path.join(KYV, "best-effort", name + ".yaml")
        b = measure(be, gce_loc) if os.path.exists(be) else None
        k = kp[name]
        rows.append({
            "platform": k["platform"], "pattern": name,
            "kyverno_verdict": "partial" if name == "argocd-applicationset-generates-nothing" else "expressible",
            "kp_loc": int(k["loc"]), "kp_logic_loc": logic_loc(open(kp_file[name]).read(), "  target:"),
            "kp_primitives": int(k["criteria"]) + int(k["filters"]),
            "kyv_loc": m["loc"], "kyv_logic_loc": m["logic_loc"], "kyv_loc_with_gce": m["loc_with_gce"], "loc_ratio": round(m["loc"] / int(k["loc"]), 2),
            "lookups": m["lookups"], "cel_expressions": m["cel_expressions"], "cel_predicates": m["cel_predicates"],
            "comprehensions": m["comprehensions"], "optional_hops": m["optional_hops"], "cel_chars": m["cel_chars"],
            "probes": "+".join(sorted(probes.get(name, []))),
            "best_effort_loc": b["loc"] if b else "", "best_effort_delta": b["loc"] - m["loc"] if b else "",
            "best_effort_predicates": b["cel_predicates"] if b else "",
            "gce": m["gce"],
        })

    out = os.path.join(EVAL, "measurements", "comparison", "expressiveness.csv")
    os.makedirs(os.path.dirname(out), exist_ok=True)
    with open(out, "w", newline="") as fh:
        w = csv.DictWriter(fh, fieldnames=list(rows[0]))
        w.writeheader()
        w.writerows(rows)

    def stats(key):
        v = sorted(r[key] for r in rows)
        return f"{v[0]}/{v[len(v) // 2]}/{v[-1]} (total {sum(v)})"

    print(f"{len(rows)} policies; min/median/max (total)")
    for key in ("kp_loc", "kyv_loc", "kyv_loc_with_gce", "kp_logic_loc", "kyv_logic_loc", "kp_primitives", "cel_predicates", "comprehensions", "optional_hops", "cel_chars"):
        print(f"  {key:18} {stats(key)}")
    print(f"  GlobalContextEntries: {len(gce_loc)} ({sum(gce_loc.values())} LOC, shared)")
    for r in rows:
        print(f"  {r['pattern']:42} kp={r['kp_loc']:>3} kyv={r['kyv_loc']:>3} (+gce {r['kyv_loc_with_gce']:>3}) "
              f"prim={r['kp_primitives']:>2} pred={r['cel_predicates']:>2} comp={r['comprehensions']:>2} "
              f"be={r['best_effort_loc'] or '-':>3} {r['probes']}")


if __name__ == "__main__":
    main()
