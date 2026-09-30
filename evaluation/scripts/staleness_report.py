#!/usr/bin/env python3
"""RQ6d / H5: summarises a staleness.sh run into results/comparison/staleness.csv (every observation)
and results/comparison/staleness-summary.csv (median/min/max latency per tool and direction).

Usage: staleness_report.py <results/raw/kyverno/staleness/<run>> [--period 300]
"""
import argparse
import csv
import os
import statistics
from collections import defaultdict

EVAL = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("run")
    ap.add_argument("--period", type=float, default=300)
    args = ap.parse_args()

    events = list(csv.DictReader(open(os.path.join(args.run, "events.csv"))))
    out = os.path.join(EVAL, "results", "comparison")
    with open(os.path.join(out, "staleness.csv"), "w", newline="") as f:
        w = csv.DictWriter(f, fieldnames=list(events[0]))
        w.writeheader()
        w.writerows(events)

    lat = defaultdict(list)
    for e in events:
        lat[(e["tool"], e["direction"])].append(float(e["latency_s"]))
    rows = []
    for (tool, direction), v in sorted(lat.items()):
        rows.append({"tool": tool, "direction": direction, "n": len(v),
                     "median_s": round(statistics.median(v), 1), "min_s": round(min(v), 1), "max_s": round(max(v), 1),
                     "mean_s": round(statistics.mean(v), 1), "max_over_period": round(max(v) / args.period, 2)})
    with open(os.path.join(out, "staleness-summary.csv"), "w", newline="") as f:
        w = csv.DictWriter(f, fieldnames=list(rows[0]))
        w.writeheader()
        w.writerows(rows)
    for r in rows:
        print(r)
    reps = defaultdict(dict)
    for e in events:
        reps[(e["rep"], e["direction"], e["offset_s"])][e["tool"]] = float(e["latency_s"])
    for k, v in sorted(reps.items(), key=lambda x: (int(x[0][0]), x[0][1])):
        print(f"rep {k[0]} {k[1]:9} offset {k[2]:>3}s  " + "  ".join(f"{t}={s:6.1f}s" for t, s in sorted(v.items())))


if __name__ == "__main__":
    main()
