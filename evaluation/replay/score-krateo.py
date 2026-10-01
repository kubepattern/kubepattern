#!/usr/bin/env python3
"""Scores a replay run of the Krateo Patterns on the controlled cases only.

Keeps the ground-truth rows and the Smells of the seeded namespaces (kp-krateo-a/b), then
calls ../scripts/score.py. Extra arguments are passed to score.py (e.g. --probe-expect truth).
"""
import csv, json, os, subprocess, sys, tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
SEEDED = {"kp-krateo-a", "kp-krateo-b"}


def main():
    smells_path, extra = sys.argv[1], sys.argv[2:]
    truth_files = [os.path.join(HERE, "..", "krateo", "scenario", "ground-truth.csv")]
    if "--with-kyverno-only" in extra:
        extra.remove("--with-kyverno-only")
        truth_files.append(os.path.join(HERE, "..", "krateo", "scenario", "ground-truth-kyverno-only.csv"))
    tmp = tempfile.mkdtemp()
    os.makedirs(os.path.join(tmp, "krateo"))
    truth = os.path.join(tmp, "krateo", "ground-truth.csv")
    with open(truth, "w", newline="") as out:
        w = None
        for f in truth_files:
            for row in csv.DictReader(open(f)):
                if row["namespace"] not in SEEDED:
                    continue
                if w is None:
                    w = csv.DictWriter(out, fieldnames=list(row.keys()))
                    w.writeheader()
                w.writerow(row)
    items = json.load(open(smells_path))["items"]
    kept = [s for s in items if (s["spec"]["target"].get("namespace") or "") in SEEDED]
    smells = os.path.join(tmp, "smells.json")
    json.dump({"items": kept}, open(smells, "w"))
    sys.exit(subprocess.call([sys.executable, os.path.join(HERE, "..", "scripts", "score.py"), smells, "--truth", truth] + extra))


if __name__ == "__main__":
    main()
