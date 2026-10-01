#!/usr/bin/env python3
"""RQ4 scalability sweep (out-of-cluster measured runs, see perf-local.sh).

Workload: Flux objects only, all `suspend: true`, with the Flux controllers scaled to 0
(Flux has no admission webhooks, so nothing reconciles or churns). Object i of the sweep is an
OCIRepository `src-i` in namespace kp-scale-(i%10); HelmRelease `hr-i` consumes it. Removing
HelmReleases turns sources into orphans, i.e. Smells of flux-ocirepository-not-used.
During the sweep only that pattern (plus clones in S3) is installed.

  S2 smells   T=1000 sources, S in {0,100,200,400,800} orphans; cold run (CREATE) then warm run (UPDATE)
  S3 patterns S=0, K in {1,5,10,20} copies of the pattern over the same kinds
  S1 objects  100 orphans, T in {1000,2500,5000,10000} sources (N ~ 2T objects)
Writes results/scale/{runs.csv,summary.csv}.
`scale.py cleanup` restores the evaluation state (scale namespaces removed, Flux back, all Patterns).
`scale.py s2-subset` re-measures only S in {400, 800} (cold, warm twice, stale): the cells where
Smell GC behaviour differs between engine versions. Set SCALE_OUT and RUN_PREFIX to keep the
results of another engine version (KP_COMMIT/KP_BIN) apart, e.g. SCALE_OUT=results/scale-fix.
"""
import csv
import json
import os
import statistics
import subprocess
import sys

EVAL = os.path.abspath(os.path.join(os.path.dirname(os.path.abspath(__file__)), ".."))
OUT = os.path.join(EVAL, os.environ.get("SCALE_OUT", os.path.join("results", "scale")))
RUN_PREFIX = os.environ.get("RUN_PREFIX", "")
NS = [f"kp-scale-{i}" for i in range(10)]
PATTERN = "flux-ocirepository-not-used"
REPS = int(os.environ.get("REPS", "5"))
rows = []


def sh(cmd, input=None, check=True):
    r = subprocess.run(cmd, shell=True, input=input, capture_output=True, text=True)
    if check and r.returncode != 0:
        raise RuntimeError(f"{cmd}\n{r.stderr}")
    return r.stdout


def kubectl_create(docs):
    for i in range(0, len(docs), 500):
        sh("kubectl create -f - >/dev/null", input="\n---\n".join(docs[i:i + 500]))


def oci(i):
    return (f"apiVersion: source.toolkit.fluxcd.io/v1\nkind: OCIRepository\nmetadata:\n  name: src-{i}\n  namespace: {NS[i % 10]}\n"
            f"spec:\n  suspend: true\n  interval: 24h\n  url: oci://ghcr.io/stefanprodan/charts/podinfo\n  ref: {{semver: \">=0.0.0\"}}")


def hr(i):
    return (f"apiVersion: helm.toolkit.fluxcd.io/v2\nkind: HelmRelease\nmetadata:\n  name: hr-{i}\n  namespace: {NS[i % 10]}\n"
            f"spec:\n  suspend: true\n  interval: 24h\n  chartRef: {{kind: OCIRepository, name: src-{i}}}")


def delete_hrs(indices):
    by_ns = {}
    for i in indices:
        by_ns.setdefault(NS[i % 10], []).append(f"hr-{i}")
    for ns, names in by_ns.items():
        for j in range(0, len(names), 200):
            sh(f"kubectl -n {ns} delete helmrelease --wait=false {' '.join(names[j:j + 200])} >/dev/null")


def count_objects():
    return int(sh("kubectl get ocirepositories,helmreleases -A --no-headers | wc -l"))


def measure(experiment, cell, reps=REPS, cold=False):
    for r in range(reps):
        if cold:
            sh("kubectl delete smells.kubepattern.dev --all -A --wait=true >/dev/null")
        out = json.loads(sh(f"{EVAL}/scripts/perf-local.sh {RUN_PREFIX}{experiment}-{cell}").strip().splitlines()[-1])
        out.update({"experiment": experiment, "cell": cell, "rep": r, "objects": count_objects()})
        rows.append(out)
        print(f"  {experiment} {cell} rep{r}: wall={out['wall_s']}s rss={out['max_rss_mb']}MB smells={out['smells']} "
              f"api={out['api_requests']} timeout={out['timeout']}", flush=True)
        save()


def save():
    os.makedirs(OUT, exist_ok=True)
    cols = ["experiment", "cell", "rep", "objects", "smells", "wall_s", "max_rss_mb", "user_s", "sys_s", "api_requests", "timeout", "by_verb", "phases", "dir"]
    with open(os.path.join(OUT, "runs.csv"), "w", newline="") as f:
        w = csv.DictWriter(f, fieldnames=cols, extrasaction="ignore")
        w.writeheader()
        for r in rows:
            w.writerow({**r, "by_verb": json.dumps(r["by_verb"]), "phases": json.dumps(r["phases"])})
    groups = {}
    for r in rows:
        groups.setdefault((r["experiment"], r["cell"]), []).append(r)
    with open(os.path.join(OUT, "summary.csv"), "w", newline="") as f:
        w = csv.writer(f)
        w.writerow(["experiment", "cell", "runs", "objects", "smells", "wall_median_s", "wall_iqr_s", "rss_median_mb", "cpu_median_s",
                    "api_requests_median", "fetch_median_s", "write_median_s", "gc_median_s", "timeouts"])
        for (e, c), g in groups.items():
            walls = sorted(x["wall_s"] for x in g)
            q = statistics.quantiles(walls, n=4) if len(walls) >= 2 else [walls[0], walls[0], walls[0]]
            w.writerow([e, c, len(g), g[-1]["objects"], g[-1]["smells"], round(statistics.median(walls), 2), round(q[2] - q[0], 2),
                        round(statistics.median(x["max_rss_mb"] for x in g), 1), round(statistics.median(x["user_s"] + x["sys_s"] for x in g), 2),
                        statistics.median(x["api_requests"] for x in g),
                        *(round(statistics.median(x["phases"].get(ph, 0) for x in g), 3) for ph in ("fetch", "write", "gc")),
                        sum(bool(x["timeout"]) for x in g)])


def prepare(T):
    print("prepare: Flux controllers -> 0, only the Flux pattern installed", flush=True)
    sh("kubectl -n flux-system scale deploy --all --replicas=0 >/dev/null")
    sh("kubectl get patterns -o name | grep -v '/" + PATTERN + "$' | xargs -r kubectl delete >/dev/null")
    for ns in NS:
        sh(f"kubectl create ns {ns} >/dev/null", check=False)
    kubectl_create([oci(i) for i in range(T)] + [hr(i) for i in range(T)])


def s2_subset():
    """GC behaviour around the throttling ceiling: warm runs above it expose flapping."""
    prepare(1000)
    removed = 0
    for S in (400, 800):
        delete_hrs(range(removed, S))
        removed = S
        measure("S2cold", f"S{S}", reps=1, cold=True)
        measure("S2warm", f"S{S}", reps=1)
        measure("S2warm2", f"S{S}", reps=1)  # second warm run: does the Smell set converge?
    kubectl_create([hr(i) for i in range(removed)])
    measure("S2stale", f"S{removed}", reps=1)
    measure("S2stale2", f"S{removed}", reps=1)  # finishes any pruning cut by the deadline
    print("done", flush=True)


def main():
    T = 1000
    prepare(T)

    print("S2 smells", flush=True)
    removed = 0
    for S in (0, 100, 200, 400, 800):
        delete_hrs(range(removed, S))
        removed = S
        measure("S2cold", f"S{S}", reps=1, cold=True)
        measure("S2warm", f"S{S}", reps=1)
    kubectl_create([hr(i) for i in range(removed)])
    measure("S2stale", f"S{removed}", reps=1)  # every orphan fixed: DELETE path in CleanOldScans

    print("S3 patterns", flush=True)
    base = json.loads(sh(f"kubectl get pattern {PATTERN} -o json"))
    for k in (1, 5, 10, 20):
        for c in range(1, k):
            clone = {"apiVersion": base["apiVersion"], "kind": "Pattern", "metadata": {"name": f"{PATTERN}-clone-{c}"}, "spec": base["spec"]}
            sh("kubectl apply -f - >/dev/null", input=json.dumps(clone))
        measure("S3", f"K{k}")
    sh(f"kubectl get patterns -o name | grep -- '-clone-' | xargs -r kubectl delete >/dev/null")

    print("S1 objects", flush=True)
    delete_hrs(range(100))
    for T_next in (1000, 2500, 5000, 10000):
        if T_next > T:
            kubectl_create([oci(i) for i in range(T, T_next)] + [hr(i) for i in range(T, T_next)])
            T = T_next
        measure("S1", f"T{T}")
    print("done", flush=True)


def cleanup():
    sh(f"kubectl delete ns {' '.join(NS)} --wait=true >/dev/null", check=False)
    sh("kubectl get patterns -o name | grep -- '-clone-' | xargs -r kubectl delete >/dev/null", check=False)
    sh("kubectl -n flux-system scale deploy --all --replicas=1 >/dev/null")
    sh(f"{EVAL}/scripts/apply-patterns.sh >/dev/null")
    print("restored: scale namespaces deleted, Flux controllers back, all evaluation Patterns applied")


def rebuild_summary():
    import re
    with open(os.path.join(OUT, "runs.csv")) as f:
        for r in csv.DictReader(f):
            log = open(os.path.join(r["dir"], "time.txt")).read()
            r.update({"rep": int(r["rep"]), "objects": int(r["objects"]), "smells": int(r["smells"]), "wall_s": float(r["wall_s"]),
                      "max_rss_mb": float(r["max_rss_mb"]), "user_s": float(r["user_s"]), "sys_s": float(r["sys_s"]),
                      "api_requests": int(r["api_requests"]), "by_verb": json.loads(r["by_verb"]), "phases": json.loads(r["phases"]),
                      "timeout": bool(re.search(r"would exceed context deadline|context deadline exceeded", log))})
            rows.append(r)
    save()


if __name__ == "__main__":
    if sys.argv[1:] == ["summary"]:
        rebuild_summary()
        sys.exit(0)
    if sys.argv[1:] == ["cleanup"]:
        cleanup()
        sys.exit(0)
    try:
        s2_subset() if sys.argv[1:] == ["s2-subset"] else main()
    finally:
        if rows:
            save()
