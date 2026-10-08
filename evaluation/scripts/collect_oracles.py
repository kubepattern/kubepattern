#!/usr/bin/env python3
"""RQ3 complementarity: does the platform itself report the smells KubePattern finds?

For every ground-truth row whose truth is "smell" (positives and limitation probes) it reads
the live target object and its Events, and records:
  - the status conditions that are not healthy (status False for positive-polarity types,
    or any condition whose reason/type mentions an error),
  - the number of Warning Events whose involvedObject is the target.
A signal counts as "reported upstream" only if it is discriminative: the same condition/event
reason must not appear on any clean object (negative or near-miss) of the same pattern. This
filters generic warnings (e.g. ESO's StoreUnmaintained on every fake-provider store).
Writes measurements/oracles/oracles.csv and prints a per-pattern summary.
"""
import csv
import glob
import json
import os
import re
import subprocess
from collections import defaultdict

EVAL = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..")
RESOURCE = {  # kind -> fully-qualified resource for kubectl
    "ClusterIssuer": "clusterissuers.cert-manager.io", "Cluster": "clusters.postgresql.cnpg.io",
    "Pooler": "poolers.postgresql.cnpg.io", "SecretStore": "secretstores.external-secrets.io",
    "ClusterSecretStore": "clustersecretstores.external-secrets.io", "OCIRepository": "ocirepositories.source.toolkit.fluxcd.io",
    "ApplicationSet": "applicationsets.argoproj.io", "AppProject": "appprojects.argoproj.io",
    "Warehouse": "warehouses.kargo.akuity.io", "Function": "functions.pkg.crossplane.io",
    "Composition": "compositions.apiextensions.crossplane.io", "ComponentDefinition": "componentdefinitions.core.oam.dev",
    "TraitDefinition": "traitdefinitions.core.oam.dev", "DockerMachineTemplate": "dockermachinetemplates.infrastructure.cluster.x-k8s.io",
}
NEGATIVE_TYPES = {"stalled", "degraded", "failed", "error", "reconcileerror", "erroroccurred"}  # negative-polarity condition types
BAD_REASON = re.compile(r"(error|fail|invalid|notfound|not_found|missing)", re.I)


def kubectl_json(*args):
    r = subprocess.run(["kubectl", *args, "-o", "json"], capture_output=True, text=True)
    return json.loads(r.stdout) if r.returncode == 0 else None


def unhealthy(conditions):
    bad = []
    for c in conditions or []:
        t, s, reason = c.get("type", ""), c.get("status", ""), c.get("reason", "") or ""
        negative_type = t.lower() in NEGATIVE_TYPES
        if (negative_type and s == "True") or (not negative_type and s == "False") or BAD_REASON.search(reason):
            bad.append(f"{t}={s}({reason})")
    return bad


def main():
    rows = []
    for path in sorted(glob.glob(os.path.join(EVAL, "scenarios", "*", "ground-truth.csv"))):
        for r in csv.DictReader(open(path)):
            if r["class"] == "blocked-upstream":
                continue
            ns_args = ["-n", r["namespace"]] if r["namespace"] else []
            obj = kubectl_json("get", RESOURCE[r["kind"]], r["name"], *ns_args)
            status = (obj or {}).get("status") or {}
            bad = unhealthy(status.get("conditions"))
            if status.get("phase") in ("Failed", "Error"):
                bad.append(f"phase={status['phase']}")
            ev = kubectl_json("get", "events", "-A" if not r["namespace"] else "-n", *([r["namespace"]] if r["namespace"] else []),
                              "--field-selector", f"involvedObject.name={r['name']},involvedObject.kind={r['kind']},type=Warning")
            warnings = [e.get("reason", "") for e in (ev or {}).get("items", [])]
            rows.append({"platform": os.path.basename(os.path.dirname(path)), "pattern": r["pattern"], "kind": r["kind"],
                         "namespace": r["namespace"], "name": r["name"], "class": r["class"],
                         "truth": r["truth"], "has_status": bool(status),
                         "signals": sorted(set(b.split("(")[0] for b in bad) | {f"event:{w}" for w in warnings}),
                         "unhealthy_conditions": ";".join(bad), "warning_events": ";".join(sorted(set(warnings)))})
    clean_signals = defaultdict(set)
    for r in rows:
        if r["truth"] == "clean":
            clean_signals[r["pattern"]] |= set(r["signals"])
    for r in rows:
        r["discriminative_signals"] = ";".join(x for x in r["signals"] if x not in clean_signals[r["pattern"]])
        r["reported_upstream"] = bool(r["discriminative_signals"])
        r["signals"] = ";".join(r["signals"])
    rows = [r for r in rows if r["truth"] == "smell"]
    out = os.path.join(EVAL, "measurements", "oracles")
    os.makedirs(out, exist_ok=True)
    with open(os.path.join(out, "oracles.csv"), "w", newline="") as f:
        w = csv.DictWriter(f, fieldnames=list(rows[0].keys()))
        w.writeheader()
        w.writerows(rows)
    per = defaultdict(lambda: [0, 0])
    for r in rows:
        per[r["pattern"]][0] += 1
        per[r["pattern"]][1] += int(r["reported_upstream"])
    print(f"{'pattern':45} smells  reported-by-platform")
    for p, (n, rep) in sorted(per.items()):
        print(f"{p:45} {n:>6}  {rep:>6}")
    print(f"{'TOTAL':45} {sum(v[0] for v in per.values()):>6}  {sum(v[1] for v in per.values()):>6}")
    for r in rows:
        if r["reported_upstream"]:
            print("  reported:", r["pattern"], r["namespace"], r["name"], r["discriminative_signals"])


if __name__ == "__main__":
    main()
