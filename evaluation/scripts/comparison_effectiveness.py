#!/usr/bin/env python3
"""RQ6b: scores every archived Kyverno run (measurements/raw/kyverno/{equivalent,best-effort}-r*/<run>/smells.json)
with score.py and writes measurements/comparison/effectiveness.csv, plus per-run score.py outputs in
measurements/comparison/effectiveness/<label>/.

Probes are reported twice: as predicted by the KubePattern engine semantics (engine_expected) and
against the truth. A 1:1 translation should reproduce the engine column; a best-effort policy the truth column.
"""
import csv
import glob
import json
import os
import subprocess

EVAL = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..")
OUT = os.path.join(EVAL, "measurements", "comparison")


def score(smells, oracle, out=None):
    tmp = out or os.path.join(OUT, ".tmp-score")
    cmd = [os.path.join(EVAL, "scripts", "score.py"), smells, "--probe-expect", oracle, "--out", tmp]
    rc = subprocess.run(cmd, capture_output=True, text=True).returncode
    total = [r for r in csv.DictReader(open(os.path.join(tmp, "scores.csv"))) if r["pattern"] == "TOTAL"][0]
    return rc, total


def main():
    rows = []
    for d in sorted(glob.glob(os.path.join(EVAL, "measurements", "raw", "kyverno", "*-r[0-9]*", "*"))):
        label = os.path.basename(os.path.dirname(d))
        policy_set, rep = label.rsplit("-r", 1)
        smells = os.path.join(d, "smells.json")
        summary = json.load(open(os.path.join(d, "summary.json")))
        own = "engine" if policy_set == "equivalent" else "truth"
        rc_e, e = score(smells, "engine", os.path.join(OUT, "effectiveness", label) if own == "engine" else None)
        rc_t, t = score(smells, "truth", os.path.join(OUT, "effectiveness", label) if own == "truth" else None)
        rows.append({
            "set": policy_set, "rep": int(rep), "run": os.path.basename(d),
            "results": sum(sum(c.values()) for c in summary["policies"].values()), "fail": summary["fail"],
            "errors": summary["nonVerdicts"],
            **{k: e[k] for k in ("positives", "negatives", "TP", "FP", "FN", "TN", "precision", "recall", "probes")},
            "probes_as_engine": e["probes_predicted"], "probes_as_truth": t["probes_predicted"],
            "probe_FN": e["probe_FN"], "probe_FP": e["probe_FP"],
            "score_exit": rc_e if own == "engine" else rc_t,
        })
    tmp = os.path.join(OUT, ".tmp-score")
    if os.path.isdir(tmp):
        for f in os.listdir(tmp):
            os.remove(os.path.join(tmp, f))
        os.rmdir(tmp)
    with open(os.path.join(OUT, "effectiveness.csv"), "w", newline="") as fh:
        w = csv.DictWriter(fh, fieldnames=list(rows[0]))
        w.writeheader()
        w.writerows(rows)
    for r in rows:
        print(f"{r['set']:12} r{r['rep']} {r['TP']:>3}TP {r['FP']}FP {r['FN']}FN {r['TN']:>3}TN P={r['precision']} R={r['recall']} "
              f"probes engine={r['probes_as_engine']}/{r['probes']} truth={r['probes_as_truth']}/{r['probes']} "
              f"errors={r['errors']} exit={r['score_exit']}")


if __name__ == "__main__":
    main()
