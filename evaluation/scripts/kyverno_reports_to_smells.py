#!/usr/bin/env python3
"""Converts Kyverno PolicyReports into a smells.json-shaped file, so score.py can score them (RQ6).

Every `fail` result of a ValidatingPolicy becomes one item:
  spec.pattern.name = policy name, spec.target = {kind, namespace, name} of the report scope.
Results other than fail/pass (error, skip, warn) are printed and kept in the summary, because an
`error` is neither a smell nor a clean verdict.

Input: `kubectl get policyreports.wgpolicyk8s.io,clusterpolicyreports.wgpolicyk8s.io -A -o json`.
"""
import argparse
import json
import sys
from collections import Counter, defaultdict


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("reports", help="PolicyReport + ClusterPolicyReport list (JSON)")
    ap.add_argument("--out", required=True, help="smells.json-shaped output")
    ap.add_argument("--summary", help="optional JSON summary: counts per policy and result")
    ap.add_argument("--policies", nargs="*", help="only these policies (default: every ValidatingPolicy result)")
    args = ap.parse_args()

    with open(args.reports) as f:
        reports = json.load(f)["items"]

    items, counts = [], defaultdict(Counter)
    non_verdicts = []
    newest = 0
    for rep in reports:
        scope = rep.get("scope") or {}
        for res in rep.get("results") or []:
            if res.get("source") != "KyvernoValidatingPolicy":
                continue
            policy = res["policy"]
            if args.policies and policy not in args.policies:
                continue
            result = res["result"]
            counts[policy][result] += 1
            newest = max(newest, int((res.get("timestamp") or {}).get("seconds", 0)))
            target = {"kind": scope.get("kind", ""), "namespace": scope.get("namespace", ""), "name": scope.get("name", "")}
            if result == "fail":
                items.append({"spec": {"pattern": {"name": policy}, "target": target, "message": res.get("message", "")}})
            elif result != "pass":
                non_verdicts.append((policy, result, target, res.get("message", "")))

    items.sort(key=lambda s: (s["spec"]["pattern"]["name"], s["spec"]["target"]["namespace"], s["spec"]["target"]["name"]))
    with open(args.out, "w") as f:
        json.dump({"items": items}, f, indent=1)

    summary = {
        "policies": {p: dict(c) for p, c in sorted(counts.items())},
        "fail": len(items),
        "nonVerdicts": len(non_verdicts),
        "newestResult": newest,
    }
    if args.summary:
        with open(args.summary, "w") as f:
            json.dump(summary, f, indent=1)
    for p, r, t, m in non_verdicts:
        print(f"{r:5} {p} {t['kind']} {t['namespace']}/{t['name']}: {m}", file=sys.stderr)
    print(f"{len(counts)} policies, {sum(sum(c.values()) for c in counts.values())} results, "
          f"{len(items)} fail, {len(non_verdicts)} error/skip/warn", file=sys.stderr)


if __name__ == "__main__":
    main()
