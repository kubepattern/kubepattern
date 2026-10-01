# Validation: configurable client QPS/Burst (WP0)

Engine change on branch `chore/engine-cleanup`, on top of `dev` @ `b9f6494` (per-pattern prune merged). The binary is `bin/kubepattern-wp0`. The change:
- client-side rate limits are configurable (`analysis.client.qps` / `burst`, default **50 / 100**, previously client-go's 5 / 10);
- the linter rejects the unimplemented primitives (`selects`, `selectedBy`, `CONTAINS`, `LABEL_SELECTOR`).

The comparison isolates the rate limit:
- "before" is the per-pattern prune engine `a814e4a` (client-go defaults);
- "after" is the same engine plus the change.

The main results (RQ1–RQ6, Krateo) are unchanged: they use the as-is engine.

## Krateo, whole cluster (`kp-krateo`, 2026-10-01)
Setup:
- cluster state checked against the committed ground truth: `krateo_oracle.py check` 201/201;
- in-cluster baseline with the installed `69d4ffd`: 61 Smells, TP 55, FP 0, FN 0, TN 121, 25/25 probes;
- then 3 alternating out-of-cluster runs per engine (`perf-local.sh`), each scored against `krateo/scenario/ground-truth.csv`;
- Kyverno 1.19.1 stayed installed and running, identical for both engines.

| Engine | Wall (median of 3) | CPU | Peak RSS | API requests | Smells | Score (TP/FP/FN/TN, probes) |
|---|---|---|---|---|---|---|
| `a814e4a`, 5 QPS | 31.85 s | 0.68–0.76 s | 39–44 MB | 171 | 61 | 55/0/0/121, 25/25 |
| wp0, 50 QPS | **1.42 s** | 0.59–0.63 s | 41–43 MB | 171 | 61 | 55/0/0/121, 25/25 |

- **22× faster** with the same requests, the same Smells and the same verdicts. Throttling was about 95% of the run.
- The fetch phase drops from 5.0 s to 0.44 s; writes and the prune from about 26 s to 0.9 s.
- The 171 requests are 160 of the as-is engine plus one Smell LIST per pattern (11) for the per-pattern prune.

Re-run: `krateo/scripts/qps-krateo.sh`. Raw runs: `results/raw/perf/krateo/qps-{a814e4a,wp0}-r{1,2,3}/` (git-ignored); summary in `krateo/runs.csv`.

## kp-eval: the RQ4 ceiling (pending, `scripts/qps-eval.sh`)
To do:
- `scale.py s2-subset` with the wp0 binary (S = 400, 800 orphans; cold, warm twice, stale twice), compared with `results/gcfix/scale-after` (`a814e4a`, which writes at most 752–804 of 805 Smells and hits the 5-minute deadline);
- whole-cluster runs scored against the RQ2 ground truth.
