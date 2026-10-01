#!/usr/bin/env python3
"""Summarises the API requests of one analysis run from its audit log (RQ4).

Input: audit.jsonl written by run-once.sh (Metadata-level events of the KubePattern SA).
Output: request counts by verb/resource and a phase breakdown with microsecond timestamps:
  discovery  GET /api, /apis           (RESTMapper)
  fetch      LIST of Patterns and of every target/dependency/owner GVR
  write      GET + CREATE/UPDATE of Smells
  gc         LIST of Smells + DELETE of stale ones
The median gap between consecutive write requests exposes client-side rate limiting
(client-go default: 5 QPS, burst 10 -> ~200 ms).
"""
import json
import statistics
import sys
from collections import Counter
from datetime import datetime


def ts(s):
    return datetime.fromisoformat(s.replace("Z", "+00:00"))


def phase(ev):
    res = (ev.get("objectRef") or {}).get("resource")
    verb = ev["verb"]
    if res is None:
        return "discovery"
    if res == "smells":
        return "gc" if verb in ("list", "delete") else "write"
    return "fetch"


def summarise(path):
    events = [json.loads(l) for l in open(path) if l.strip()]
    events = [e for e in events if e.get("stage") == "ResponseComplete"]
    events.sort(key=lambda e: e["requestReceivedTimestamp"])
    if not events:
        return {"requests": 0}
    out = {"requests": len(events),
           "by_verb": dict(Counter(e["verb"] for e in events)),
           "lists": sorted({(e.get("objectRef") or {}).get("resource") for e in events if e["verb"] == "list"}),
           "phases": {}}
    t0 = ts(events[0]["requestReceivedTimestamp"])
    out["span_s"] = round((ts(events[-1]["stageTimestamp"]) - t0).total_seconds(), 3)
    for name in ("discovery", "fetch", "write", "gc"):
        ev = [e for e in events if phase(e) == name]
        if not ev:
            continue
        start = ts(ev[0]["requestReceivedTimestamp"])
        end = ts(ev[-1]["stageTimestamp"])
        p = {"requests": len(ev), "start_s": round((start - t0).total_seconds(), 3),
             "duration_s": round((end - start).total_seconds(), 3)}
        starts = [ts(e["requestReceivedTimestamp"]) for e in ev]
        gaps = [(b - a).total_seconds() * 1000 for a, b in zip(starts, starts[1:])]
        if gaps:
            p["median_gap_ms"] = round(statistics.median(gaps), 1)
        lat = [(ts(e["stageTimestamp"]) - ts(e["requestReceivedTimestamp"])).total_seconds() * 1000 for e in ev]
        p["median_server_latency_ms"] = round(statistics.median(lat), 1)
        out["phases"][name] = p
    return out


if __name__ == "__main__":
    for path in sys.argv[1:]:
        print(json.dumps({"file": path, **summarise(path)}, indent=2))
