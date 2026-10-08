#!/usr/bin/env python3
"""Report of the composition lifecycle experiment (scripts/lifecycle.sh).

For each phase (baseline, mutated, reverted) and each tool (KubePattern, Kyverno equivalent):
  - the Smell set must equal what the independent oracle predicts for the engine semantics on the
    snapshot taken in that phase (krateo_oracle.py, engine_expected);
  - the baseline -> mutated delta must contain every change in scenario/mutations.csv;
  - the reverted set must equal the baseline set.
Writes krateo/measurements/lifecycle.csv (one row per expected change) and prints a summary; exit 1 on any failure.
"""
import csv
import glob
import importlib.util
import json
import os
import sys

KRATEO = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..")
EVAL = os.path.abspath(os.path.join(KRATEO, ".."))
RAW = os.path.join(EVAL, "measurements", "raw")
LABEL = os.environ.get("LIFECYCLE_LABEL", "lifecycle")

spec = importlib.util.spec_from_file_location("oracle", os.path.join(KRATEO, "scripts", "krateo_oracle.py"))
oracle = importlib.util.module_from_spec(spec)
spec.loader.exec_module(oracle)

PATTERNS = {p for p, *_ in csv.reader(open(os.path.join(KRATEO, "scenario", "ground-truth.csv")))} - {"pattern"}


def smells(path):
    out = set()
    for s in json.load(open(path))["items"]:
        t = s["spec"].get("target", {})
        key = (s["spec"]["pattern"]["name"], t.get("kind", ""), t.get("namespace", "") or "", t.get("name", ""))
        if key[0] in PATTERNS:
            out.add(key)
    return out


def latest(pattern):
    found = sorted(glob.glob(os.path.join(RAW, pattern, "*", "smells.json")))
    if not found:
        sys.exit(f"missing run: {pattern}")
    return found[-1]


def main():
    phases = ["baseline", "mutated", "reverted"]
    tools = {"KubePattern": "krateo/{l}-{p}", "Kyverno equivalent": "kyverno/krateo/{l}-{p}"}
    got = {(t, p): smells(latest(pat.format(l=LABEL, p=p))) for t, pat in tools.items() for p in phases}
    ok = True
    for p in phases:
        rows, _ = oracle.rows_for(oracle.load(os.path.join(RAW, "krateo", f"{LABEL}-{p}", "oracle", "objects.json")))
        expected = {(r["pattern"], r["kind"], r["namespace"], r["name"]) for r in rows if r["engine_expected"] == "smell"}
        for t in tools:
            miss, extra = expected - got[(t, p)], got[(t, p)] - expected
            ok &= not miss and not extra
            print(f"{p:9} {t:19} smells={len(got[(t, p)]):>3} oracle={len(expected):>3} missing={len(miss)} extra={len(extra)}")
            for x in sorted(miss):
                print("    missing", x)
            for x in sorted(extra):
                print("    extra  ", x)

    out = []
    for m in csv.DictReader(open(os.path.join(KRATEO, "scenario", "mutations.csv"))):
        key = (m["pattern"], m["kind"], m["namespace"], m["name"])
        row = {**m}
        for t in tools:
            before, after = key in got[(t, "baseline")], key in got[(t, "mutated")]
            seen = "+" if (not before and after) else "-" if (before and not after) else "="
            row[t] = seen
            ok &= seen == m["change"]
        out.append(row)
    for t in tools:
        same = got[(t, "baseline")] == got[(t, "reverted")]
        ok &= same
        delta = len(got[(t, "mutated")] ^ got[(t, "baseline")])
        print(f"{t:19} expected changes seen: {sum(r[t] == r['change'] for r in out)}/{len(out)}; "
              f"total delta {delta}; reverted == baseline: {same}")
    with open(os.path.join(KRATEO, "measurements", "lifecycle.csv"), "w", newline="") as f:
        w = csv.DictWriter(f, fieldnames=list(out[0]), lineterminator="\n")
        w.writeheader()
        w.writerows(out)
    sys.exit(0 if ok else 1)


if __name__ == "__main__":
    main()
