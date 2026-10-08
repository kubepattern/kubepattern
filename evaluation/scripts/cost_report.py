#!/usr/bin/env python3
"""RQ6c: footprint and API load of Kyverno over an idle window, next to KubePattern per run.

Inputs (one window directory from measurements/raw/kyverno/cost-idle/<run>/):
  footprint.tsv  epoch, pod, container, cumulative CPU seconds, memory working set (scripts/footprint-sample.sh)
  audit.jsonl    audit events of the kyverno ServiceAccounts (scripts/audit-harvest.sh)
  start, end     window bounds (ISO)
KubePattern per-run numbers come from a run-once.sh archive (audit) and a perf-local.sh line (RSS, CPU).

Writes measurements/comparison/cost.csv (one row per component) and prints it.
"""
import argparse
import csv
import json
import os
import statistics
import subprocess
import sys
import tempfile
from collections import defaultdict
from datetime import datetime

EVAL = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..")
CATS = ["watch", "list", "get", "report-write", "smell-write", "lease", "event", "other-write", "discovery"]


def iso(s):
    return datetime.fromisoformat(s.strip().replace("Z", "+00:00"))


def controller(pod):
    for c in ("admission", "background", "cleanup", "reports"):
        if f"kyverno-{c}-controller" in pod:
            return f"{c}-controller"
    return pod


def audit_counts(path, t_from=None, t_to=None, users=None):
    tmp = tempfile.NamedTemporaryFile(suffix=".json", delete=False).name
    cmd = [os.path.join(EVAL, "scripts", "audit_by_user.py"), path, "--json", tmp]
    if t_from:
        cmd += ["--from", t_from]
    if t_to:
        cmd += ["--to", t_to]
    if users:
        cmd += ["--user", *users]
    subprocess.run(cmd, capture_output=True, text=True, check=True)
    with open(tmp) as f:
        data = json.load(f)
    os.remove(tmp)
    return data


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("window", help="measurements/raw/kyverno/cost-idle/<run>")
    ap.add_argument("--scan-interval", type=float, default=300, help="Kyverno backgroundScanInterval in seconds")
    ap.add_argument("--kp-run", help="run-once.sh archive of a KubePattern run (audit.jsonl, summary.json)")
    ap.add_argument("--kp-perf", help="perf-local.sh JSON line (file) of a KubePattern run")
    ap.add_argument("--kp-period", type=float, default=300, help="KubePattern CronJob period in seconds")
    args = ap.parse_args()

    w = args.window
    t0, t1 = open(os.path.join(w, "start")).read().strip(), open(os.path.join(w, "end")).read().strip()
    hours = (iso(t1) - iso(t0)).total_seconds() / 3600
    cycles = hours * 3600 / args.scan_interval

    samples = defaultdict(list)
    with open(os.path.join(w, "footprint.tsv")) as f:
        for r in csv.DictReader(f, delimiter="\t"):
            samples[controller(r["pod"])].append((int(r["epoch"]), float(r["cpu_seconds_total"]), int(r["memory_working_set_bytes"])))

    audit = audit_counts(os.path.join(w, "audit.jsonl"), t0, t1)
    by_user = {k.replace("kyverno:kyverno-", ""): v for k, v in audit["by_user"].items()}

    rows = []
    tot = defaultdict(float)
    for comp in sorted(samples):
        s = sorted(samples[comp])
        mem = [m / 2**20 for _, _, m in s]
        cpu_s = s[-1][1] - s[0][1]
        span = s[-1][0] - s[0][0]
        req = by_user.get(comp, {})
        row = {"tool": "kyverno", "component": comp,
               "mem_median_mib": round(statistics.median(mem), 1), "mem_peak_mib": round(max(mem), 1),
               "cpu_cores_avg": round(cpu_s / span, 4), "cpu_s_per_hour": round(cpu_s / span * 3600, 1),
               "requests_per_hour": round(sum(req.values()) / hours),
               **{f"{c}_per_hour": round(req.get(c, 0) / hours) for c in CATS},
               "requests_per_cycle": round(sum(req.values()) / cycles, 1),
               "requests_per_cycle_no_lease": round((sum(req.values()) - req.get("lease", 0)) / cycles, 1)}
        rows.append(row)
        for k, v in row.items():
            if isinstance(v, (int, float)) and k != "cpu_cores_avg":
                tot[k] += v
        tot["cpu_cores_avg"] += row["cpu_cores_avg"]
    rows.append({"tool": "kyverno", "component": "TOTAL (4 controllers)",
                 **{k: round(v, 4 if k == "cpu_cores_avg" else 1) for k, v in tot.items()}})

    if args.kp_run:
        s = json.load(open(os.path.join(args.kp_run, "summary.json")))
        kp = audit_counts(os.path.join(args.kp_run, "audit.jsonl"))["by_user"].get("ALL", {})
        per_h = 3600 / args.kp_period
        perf = json.loads(open(args.kp_perf).read().strip().splitlines()[-1]) if args.kp_perf else {}
        cpu = (perf.get("user_s", 0) + perf.get("sys_s", 0)) if perf else None
        n = sum(kp.values())
        rows.append({"tool": "kubepattern", "component": f"kubepattern-job (1 run per {int(args.kp_period)} s; idle 0)",
                     "mem_median_mib": 0, "mem_peak_mib": perf.get("max_rss_mb", ""),
                     "cpu_cores_avg": round(cpu / args.kp_period, 4) if cpu is not None else "",
                     "cpu_s_per_hour": round(cpu * per_h, 1) if cpu is not None else "",
                     "requests_per_hour": round(n * per_h),
                     **{f"{c}_per_hour": round(kp.get(c, 0) * per_h) for c in CATS},
                     "requests_per_cycle": n, "requests_per_cycle_no_lease": n - kp.get("lease", 0),
                     "smell_writes_per_cycle": kp.get("smell-write", 0), "run_s": s.get("completion") and
                     (iso(s["completion"]) - iso(s["start"])).total_seconds()})

    out = os.path.join(EVAL, "measurements", "comparison", "cost.csv")
    fields = list(dict.fromkeys(k for r in rows for k in r))
    with open(out, "w", newline="") as fh:
        wr = csv.DictWriter(fh, fieldnames=fields)
        wr.writeheader()
        wr.writerows(rows)
    print(f"window {t0} .. {t1} ({hours:.2f} h, {cycles:.1f} scan cycles)")
    for r in rows:
        print(json.dumps(r))
    print("by resource (top 8 per user):")
    for u, c in audit["by_resource"].items():
        print(f"  {u}: " + ", ".join(f"{k}={v}" for k, v in list(c.items())[:8]))


if __name__ == "__main__":
    sys.exit(main())
