#!/usr/bin/env python3
"""Checks every field path used by the evaluated Patterns against the live CRD schemas.

For each filter path, targetPath and dependencyPath (metadata.* and kind are universal and
skipped) it runs `kubectl explain <plural>.<path without [*]> --api-version=<apiVersion>`.
Writes measurements/field-verification.csv; exit code 1 if any path is unknown to the API server.
"""
import csv
import glob
import os
import subprocess
import sys

import yaml

EVAL = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..")
UNIVERSAL = ("metadata.", "kind")


def explain(plural, api_version, path):
    field = path.replace("[*]", "")
    r = subprocess.run(["kubectl", "explain", f"{plural}.{field}", f"--api-version={api_version}"],
                       capture_output=True, text=True)
    return r.returncode == 0


def main():
    rows = []
    files = sorted(glob.glob(os.path.join(EVAL, "patterns", "**", "*.yaml"), recursive=True))
    for f in files:
        p = yaml.safe_load(open(f))
        spec = p["spec"]
        kinds = {"target": spec["target"], **{d["id"]: d for d in spec.get("dependencies", [])}}
        checks = []
        for owner, res in kinds.items():
            for group in ("matchAll", "matchAny", "matchNone"):
                for c in (res.get("filters") or {}).get(group, []):
                    checks.append((owner, c["path"]))
        for group in ("matchAll", "matchAny", "matchNone"):
            for rel in (spec.get("relationships") or {}).get(group, []):
                for c in rel.get("criteria", []):
                    checks.append(("target", c["targetPath"]))
                    checks.append((rel["with"], c["dependencyPath"]))
        seen = set()
        for owner, path in checks:
            res = kinds[owner]
            key = (res["apiVersion"], res["plural"], path)
            if path.startswith(UNIVERSAL) or key in seen:
                continue
            seen.add(key)
            ok = explain(res["plural"], res["apiVersion"], path)
            rows.append({"pattern": p["metadata"]["name"], "kind": res["kind"], "apiVersion": res["apiVersion"],
                         "path": path, "status": "V" if ok else "UNKNOWN"})
    out = os.path.join(EVAL, "measurements", "field-verification.csv")
    os.makedirs(os.path.dirname(out), exist_ok=True)
    with open(out, "w", newline="") as fh:
        w = csv.DictWriter(fh, fieldnames=["pattern", "kind", "apiVersion", "path", "status"])
        w.writeheader()
        w.writerows(rows)
    bad = [r for r in rows if r["status"] != "V"]
    print(f"{len(rows)} paths checked, {len(bad)} unknown -> {out}")
    for r in bad:
        print("UNKNOWN", r)
    sys.exit(1 if bad else 0)


if __name__ == "__main__":
    main()
