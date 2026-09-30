#!/usr/bin/env python3
"""RQ5 report: Smell lifecycle across the scheduled runs archived by cronjob-timeline.sh.

For each run it compares the Smell set (pattern, kind, namespace, name) with the previous run
and classifies every Smell object as created, kept (same object updated in place: same UID),
recreated (same identity, new object) or garbage-collected. It also checks each run against the
set predicted from the scripted events, and extracts run duration and skip messages.
Writes results/<label>/{timeline.csv,lifecycle.csv}; exit code 1 if a run deviates.

  --label  archive to read (results/raw/<label>) and output folder (default: cronjob)
  --gc     expected GC semantics: "global" (engine as-is: a skipped pattern loses its Smells)
           or "per-pattern" (fixed engine: a skipped pattern keeps its Smells)
"""
import argparse
import csv
import glob
import json
import os
import re
import sys
from datetime import datetime

EVAL = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..")

FIXED = {("certmanager-clusterissuer-not-used", "ClusterIssuer", "", "ci-unused"),
         ("flux-ocirepository-not-used", "OCIRepository", "kp-flux-a", "orphan-1"),
         ("capi-machinetemplate-orphan", "DockerMachineTemplate", "kp-capi-a", "dmt-orphan"),
         ("kargo-warehouse-not-requested", "Warehouse", "kp-kargo-1", "orphan")}
INJECTED = {("certmanager-clusterissuer-not-used", "ClusterIssuer", "", "ci-new-unused"),
            ("kargo-warehouse-not-requested", "Warehouse", "kp-kargo-1", "new-unused"),
            ("eso-secretstore-not-used", "SecretStore", "kp-secrets-a", "ss-new-unused")}
V3 = ("capi-machinetemplate-orphan", "DockerMachineTemplate", "kp-capi-a", "dmt-rotated-v3")
V2 = ("capi-machinetemplate-orphan", "DockerMachineTemplate", "kp-capi-a", "dmt-rotated-v2")
EVENTS = {1: "baseline", 2: "after E1 fix (4 smells resolved)", 3: "after E2 inject (3 new smells)",
          4: "after E3 add pattern with missing CRD", 5: "after E4 RBAC without kargo.akuity.io",
          6: "after E5 RBAC restored", 7: "after E6 rotation step 1 (new template not yet used)",
          8: "after E6 rotation step 2 (switch)", 9: "after E7 revert"}


def load(run_dir):
    items = json.load(open(os.path.join(run_dir, "smells.json")))["items"]
    return {(s["spec"]["pattern"]["name"], s["spec"]["target"]["kind"], s["spec"]["target"].get("namespace") or "",
             s["spec"]["target"]["name"]): (s["metadata"]["uid"], s["metadata"]["creationTimestamp"]) for s in items}


def expected(n, base, gc):
    s = set(base)
    if n >= 2: s -= FIXED
    if n >= 3: s |= INJECTED
    if n == 5 and gc == "global":  # the skipped Kargo pattern loses its Smells
        s = {k for k in s if k[0] != "kargo-warehouse-not-requested"}
    if n >= 7: s.add(V3)
    if n >= 8: s = (s - {V3}) | {V2}
    if n >= 9: s = set(base)
    return s


def duration(run_dir):
    j = json.load(open(os.path.join(run_dir, "job.json")))["status"]
    f = lambda x: datetime.fromisoformat(x.replace("Z", "+00:00"))
    return (f(j["completionTime"]) - f(j["startTime"])).total_seconds()


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--label", default="cronjob")
    ap.add_argument("--gc", choices=("global", "per-pattern"), default="global")
    args = ap.parse_args()
    raw = os.path.join(EVAL, "results", "raw", args.label)
    out = os.path.join(EVAL, "results", args.label)

    runs = sorted(glob.glob(os.path.join(raw, "run-*")), key=lambda p: int(p.rsplit("-", 1)[1]))
    snaps = [load(r) for r in runs]
    base = set(snaps[0])
    os.makedirs(out, exist_ok=True)
    timeline, ok = [], True
    for i, (r, cur) in enumerate(zip(runs, snaps), start=1):
        prev = snaps[i - 2] if i > 1 else {}
        created = [k for k in cur if k not in prev]
        gc = [k for k in prev if k not in cur]
        kept = [k for k in cur if k in prev and cur[k][0] == prev[k][0]]
        recreated = [k for k in cur if k in prev and cur[k][0] != prev[k][0]]
        exp = expected(i, base, args.gc)
        match = set(cur) == exp
        ok &= match
        log = open(os.path.join(r, "pod.log")).read()
        skipped = sorted(set(re.findall(r'skipping pattern due to resource fetch failure.*?pattern=(\S+)', log)))
        timeline.append({"run": i, "event_before_run": EVENTS.get(i, ""), "smells": len(cur), "expected": len(exp),
                         "matches_expected": match, "created": len(created), "kept_updated_in_place": len(kept),
                         "recreated": len(recreated), "garbage_collected": len(gc), "skipped_patterns": ";".join(skipped),
                         "job_duration_s": duration(r),
                         "diff_missing": ";".join("/".join(x for x in k if x) for k in sorted(exp - set(cur))),
                         "diff_extra": ";".join("/".join(x for x in k if x) for k in sorted(set(cur) - exp))})
    with open(os.path.join(out, "timeline.csv"), "w", newline="") as f:
        w = csv.DictWriter(f, fieldnames=list(timeline[0]))
        w.writeheader()
        w.writerows(timeline)
    # lifespan of every Smell identity (number of consecutive runs it was present)
    keys = sorted(set().union(*map(set, snaps)))
    with open(os.path.join(out, "lifecycle.csv"), "w", newline="") as f:
        w = csv.writer(f)
        w.writerow(["pattern", "kind", "namespace", "name", "present_in_runs", "distinct_smell_objects"])
        for k in keys:
            present = [i + 1 for i, s in enumerate(snaps) if k in s]
            w.writerow([*k, " ".join(map(str, present)), len({s[k][0] for s in snaps if k in s})])
    for t in timeline:
        print(f"run {t['run']}: {t['smells']:>3} smells (expected {t['expected']:>3}) {'OK ' if t['matches_expected'] else 'DIFF'} "
              f"+{t['created']} ={t['kept_updated_in_place']} ~{t['recreated']} -{t['garbage_collected']} "
              f"{t['job_duration_s']:.0f}s skipped=[{t['skipped_patterns']}]  <- {t['event_before_run']}"
              + (f"\n    missing: {t['diff_missing']}\n    extra: {t['diff_extra']}" if not t['matches_expected'] else ""))
    sys.exit(0 if ok else 1)


if __name__ == "__main__":
    main()
