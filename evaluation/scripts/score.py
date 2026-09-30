#!/usr/bin/env python3
"""Scores a snapshot of Smells against the scenario ground truth (RQ2).

Ground-truth rows carry two labels:
  truth            what a correct analysis should report (smell | clean)
  engine_expected  what the verified engine semantics predict (differs from truth only for probes)

Classes positive / negative / near-miss are "in scope" and give precision and recall.
probe-G* rows exercise a documented DSL limitation: they are scored separately (limitation
FN/FP) and must match the predicted engine output. blocked-upstream rows are not scored.

Exit code 1 when an in-scope row is misclassified, a probe prediction fails, or a Smell of an
evaluated pattern hits an object that is not in the ground truth (unless --allow-unlisted).
"""
import argparse
import csv
import glob
import json
import os
import sys
from collections import defaultdict

IN_SCOPE = {"positive", "negative", "near-miss"}


def load_truth(paths):
    rows = []
    for path in paths:
        platform = os.path.basename(os.path.dirname(path))
        with open(path, newline="") as f:
            for row in csv.DictReader(f):
                row["platform"] = platform
                rows.append(row)
    return rows


def load_smells(path):
    with open(path) as f:
        items = json.load(f)["items"]
    smells = set()
    for s in items:
        spec = s["spec"]
        t = spec.get("target", {})
        smells.add((spec["pattern"]["name"], t.get("kind", ""), t.get("namespace", "") or "", t.get("name", "")))
    return smells


def ratio(a, b):
    return a / b if b else None


def fmt(x):
    return "--" if x is None else f"{x:.2f}"


def main():
    here = os.path.dirname(os.path.abspath(__file__))
    ap = argparse.ArgumentParser()
    ap.add_argument("smells", help="smells.json produced by run-once.sh")
    ap.add_argument("--truth", nargs="*", default=sorted(glob.glob(os.path.join(here, "..", "scenarios", "*", "ground-truth.csv"))))
    ap.add_argument("--patterns", nargs="*", help="only score these patterns (default: every pattern in the ground truth)")
    ap.add_argument("--allow-unlisted", nargs="*", default=[], help="patterns whose extra findings are expected (natural findings)")
    ap.add_argument("--out", help="directory for scores.csv, rows.csv and table.tex")
    ap.add_argument("--probe-expect", choices=["engine", "truth"], default="engine",
                    help="probe oracle: the predicted engine output (default) or the truth (RQ6 best-effort policies)")
    args = ap.parse_args()

    rows = load_truth(args.truth)
    if args.patterns:
        rows = [r for r in rows if r["pattern"] in args.patterns]
    smells = load_smells(args.smells)
    patterns = sorted({r["pattern"] for r in rows})
    platform_of = {r["pattern"]: r["platform"] for r in rows}

    per = defaultdict(lambda: defaultdict(int))
    detail, failures = [], []
    listed = set()
    for r in rows:
        key = (r["pattern"], r["kind"], r["namespace"], r["name"])
        listed.add(key)
        if r["class"] == "blocked-upstream":
            continue
        observed = "smell" if key in smells else "clean"
        p = per[r["pattern"]]
        outcome = {("smell", "smell"): "TP", ("smell", "clean"): "FN",
                   ("clean", "smell"): "FP", ("clean", "clean"): "TN"}[(r["truth"], observed)]
        if r["class"] in IN_SCOPE:
            p[outcome] += 1
            p["positives" if r["truth"] == "smell" else "negatives"] += 1
            ok = outcome in ("TP", "TN")
        else:  # probe
            p["probes"] += 1
            p[f"probe_{outcome}"] += 1
            ok = observed == (r["engine_expected"] if args.probe_expect == "engine" else r["truth"])
            p["probes_predicted"] += int(ok)
        if not ok:
            failures.append(f"{r['class']:>10} {'/'.join(key)}: truth={r['truth']} expected={r['engine_expected']} observed={observed}")
        detail.append({**{k: r[k] for k in ("platform", "pattern", "kind", "namespace", "name", "class", "truth", "engine_expected")},
                       "observed": observed, "outcome": outcome})

    unlisted = sorted(k for k in smells if k[0] in patterns and k not in listed)
    for k in unlisted:
        per[k[0]]["unlisted"] += 1
        if k[0] not in args.allow_unlisted:
            failures.append(f"  unlisted {'/'.join(k)}")

    cols = ["platform", "pattern", "positives", "negatives", "TP", "FP", "FN", "TN", "precision", "recall",
            "probes", "probes_predicted", "probe_FN", "probe_FP", "unlisted"]
    table = []
    tot = defaultdict(int)
    for name in patterns:
        p = per[name]
        for c in cols[2:]:
            if c not in ("precision", "recall"):
                tot[c] += p[c]
        table.append({"platform": platform_of[name], "pattern": name, **{c: p[c] for c in cols[2:] if c not in ("precision", "recall")},
                      "precision": ratio(p["TP"], p["TP"] + p["FP"]), "recall": ratio(p["TP"], p["TP"] + p["FN"])})
    total = {"platform": "all", "pattern": "TOTAL", **dict(tot),
             "precision": ratio(tot["TP"], tot["TP"] + tot["FP"]), "recall": ratio(tot["TP"], tot["TP"] + tot["FN"])}

    print(f"{'pattern':45} {'P':>3} {'N':>3} {'TP':>3} {'FP':>3} {'FN':>3} {'TN':>3} {'prec':>5} {'rec':>5} {'probes(ok)':>11} {'unl':>4}")
    for t in table + [total]:
        print(f"{t['pattern']:45} {t['positives']:>3} {t['negatives']:>3} {t['TP']:>3} {t['FP']:>3} {t['FN']:>3} {t['TN']:>3} "
              f"{fmt(t['precision']):>5} {fmt(t['recall']):>5} {t['probes']:>5}({t['probes_predicted']:>3}) {t['unlisted']:>4}")
    if failures:
        print("\nMISMATCHES:")
        print("\n".join(failures))

    if args.out:
        os.makedirs(args.out, exist_ok=True)
        with open(os.path.join(args.out, "scores.csv"), "w", newline="") as f:
            w = csv.DictWriter(f, fieldnames=cols)
            w.writeheader()
            for t in table + [total]:
                w.writerow({c: (fmt(t[c]) if c in ("precision", "recall") else t.get(c, 0)) for c in cols})
        with open(os.path.join(args.out, "rows.csv"), "w", newline="") as f:
            w = csv.DictWriter(f, fieldnames=list(detail[0].keys()))
            w.writeheader()
            w.writerows(detail)
        with open(os.path.join(args.out, "table.tex"), "w") as f:
            f.write("% Generated by scripts/score.py\n\\begin{tabular}{llrrrrrrrrr}\n\\toprule\n")
            f.write("Platform & Pattern & Pos & Neg & TP & FP & FN & TN & Prec. & Rec. & Probes (pred.) \\\\\n\\midrule\n")
            for t in table:
                f.write(f"{t['platform']} & \\texttt{{{t['pattern']}}} & {t['positives']} & {t['negatives']} & {t['TP']} & {t['FP']} & "
                        f"{t['FN']} & {t['TN']} & {fmt(t['precision'])} & {fmt(t['recall'])} & {t['probes']} ({t['probes_predicted']}) \\\\\n")
            f.write(f"\\midrule\n\\multicolumn{{2}}{{l}}{{Total}} & {total['positives']} & {total['negatives']} & {total['TP']} & {total['FP']} & "
                    f"{total['FN']} & {total['TN']} & {fmt(total['precision'])} & {fmt(total['recall'])} & {total['probes']} ({total['probes_predicted']}) \\\\\n")
            f.write("\\bottomrule\n\\end{tabular}\n")
    sys.exit(1 if failures else 0)


if __name__ == "__main__":
    main()
