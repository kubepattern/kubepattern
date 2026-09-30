#!/usr/bin/env python3
"""Counts API requests in an audit log by user, category, verb and resource (RQ6c).

Each request (auditID) is counted once. Categories:
  watch         WATCH (long-lived; counted when opened)
  list, get     reads
  report-write  create/update/patch/delete of (Cluster)PolicyReports and (Cluster)EphemeralReports
  smell-write   create/update/patch/delete of Smells
  lease         leader-election Lease traffic
  event         Event writes
  other-write   any other mutating request
  discovery     requests without an object reference (/api, /apis, /version)

Usage: audit_by_user.py <audit.jsonl> [--user SUBSTR ...] [--from ISO] [--to ISO] [--json OUT]
"""
import argparse
import json
from collections import Counter, defaultdict

REPORTS = {"policyreports", "clusterpolicyreports", "ephemeralreports", "clusterephemeralreports"}
WRITES = {"create", "update", "patch", "delete", "deletecollection"}
STAGE_RANK = {"ResponseComplete": 0, "Panic": 1, "ResponseStarted": 2}


def category(e):
    ref = e.get("objectRef") or {}
    res, verb = ref.get("resource"), e["verb"]
    if res is None:
        return "discovery"
    if verb == "watch":
        return "watch"
    if res == "leases":
        return "lease"
    if verb in ("list", "get"):
        return verb
    if res in REPORTS:
        return "report-write"
    if res == "smells":
        return "smell-write"
    if res == "events":
        return "event"
    return "other-write" if verb in WRITES else verb


def short_user(u):
    return u.replace("system:serviceaccount:", "")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("audit")
    ap.add_argument("--user", nargs="*", default=[], help="keep users containing any of these substrings")
    ap.add_argument("--from", dest="t_from", help="keep requests received at or after this ISO timestamp")
    ap.add_argument("--to", dest="t_to", help="keep requests received before this ISO timestamp")
    ap.add_argument("--json", help="write the counts as JSON")
    args = ap.parse_args()

    best = {}
    for line in open(args.audit):
        if not line.strip():
            continue
        e = json.loads(line)
        rank = STAGE_RANK.get(e.get("stage"), 9)
        if e["auditID"] not in best or rank < STAGE_RANK.get(best[e["auditID"]].get("stage"), 9):
            best[e["auditID"]] = e
    events = [e for e in best.values()
              if (not args.user or any(u in e["user"]["username"] for u in args.user))
              and (not args.t_from or e["requestReceivedTimestamp"] >= args.t_from)
              and (not args.t_to or e["requestReceivedTimestamp"] < args.t_to)]

    by_user = defaultdict(Counter)
    by_res = defaultdict(Counter)
    for e in events:
        u = short_user(e["user"]["username"])
        c = category(e)
        by_user[u][c] += 1
        by_user["ALL"][c] += 1
        ref = e.get("objectRef") or {}
        by_res[u][f"{e['verb']} {ref.get('resource', '-')}" + (f".{ref['apiGroup']}" if ref.get("apiGroup") else "")] += 1

    cats = ["watch", "list", "get", "report-write", "smell-write", "lease", "event", "other-write", "discovery"]
    span = ""
    if events:
        ts = sorted(e["requestReceivedTimestamp"] for e in events)
        span = f"  ({ts[0]} .. {ts[-1]})"
    print(f"{len(events)} requests{span}")
    print(f"{'user':52} {'total':>6} " + " ".join(f"{c:>12}" for c in cats))
    for u in sorted(by_user, key=lambda x: (x == "ALL", x)):
        c = by_user[u]
        print(f"{u:52} {sum(c.values()):>6} " + " ".join(f"{c[k]:>12}" for k in cats))
    if args.json:
        with open(args.json, "w") as f:
            json.dump({"requests": len(events), "by_user": {u: dict(c) for u, c in by_user.items()},
                       "by_resource": {u: dict(c.most_common()) for u, c in by_res.items()}}, f, indent=1)


if __name__ == "__main__":
    main()
