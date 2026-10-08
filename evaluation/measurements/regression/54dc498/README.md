# Regression: engine `54dc498` (client QPS + linter + `spec.for`)

The engine of `feat/for` (commit `54dc498`; chart fix in `2981a4a`) was re-run through the whole evaluation with `spec.for` unset. Every result must equal the per-pattern prune engine (`a814e4a`, `measurements/gcfix/`, `measurements/cronjob-fix/`), except run time.

Scripts:
- `scripts/regression.sh 54dc498 [incluster|outcluster]` (kp-eval);
- `krateo/scripts/regression.sh 54dc498` (kp-krateo).

**Cluster note.** `minikube start -p kp-eval` re-created the profile empty on 2026-10-01; the profile configuration and the audit policy were kept. `scripts/rebuild-eval.sh` rebuilt it from the pinned install scripts. The as-is engine `69d4ffd` then reproduced the original RQ2/RQ4 baseline exactly: 46 Smells, 43/52/10, 23.4 s, 129 requests. During the measurements, Kyverno's reports and background controllers were scaled to 0 (they were not installed for the original RQ4) and restored afterwards.

## Correctness: unchanged
| Check | Reference | `54dc498` |
|---|---|---|
| RQ2, in-cluster, first run + 3 repetitions | 43/52, P = R = 1.00, 10/10 probes, 0 unlisted | identical (4/4); all 46 Smells `Active`, with `since` set |
| RQ2b mutation testing | 15/15 killed, reverted == baseline | 15/15, reverted == baseline |
| D8, Job + local run | 46/46 | 46/46 (requests overlapped for 0.24 s: a run now takes under 1 s) |
| D8, two runs started together, 3 repetitions | – | 46/46 each |
| RQ5 CronJob timeline | 9/9 per-pattern expectations | 9/9; runs take 3–4 s |
| Final baseline | 43/52/10 | identical |
| Krateo, 3 in-cluster runs | 55/0/0/121, 25/25 probes | identical |
| Krateo, composition lifecycle | 61/51/61; 26/26 changes; reverted == baseline | identical, for KubePattern and Kyverno |

## Run time: the throttling ceiling is gone (`whole-cluster.jsonl`, `scale/`, `scale-subset/`)
| Measurement | `a814e4a`, 5 QPS | `54dc498`, 50 QPS |
|---|---|---|
| Whole cluster (46 Smells, 143 requests), median of 3 | 26.2 s | **0.85 s** |
| Krateo whole cluster (61 Smells, 171 requests) | 31.9 s | 1.42 s |
| S2, 800 orphans (805 Smells) | 752 written, 5-min deadline hit; 797 and 804 on the next runs | **805 in 30.5 s**, at the first run |
| S2, all 800 orphans fixed (DELETE path) | 150–161 s | 14.5 s |
| Seconds per Smell written | about 0.4 | about 0.04 |
| S3, 20 pattern copies (100 Smells) | 39.0 s | 5.6 s |
| S1, 10,000 sources (about 20k objects, 105 Smells) | 41.6 s, CPU 38.5 s | 33.0 s, CPU 39.3 s |

- **Requests are unchanged**, except one Smell LIST per pattern for the per-pattern prune (already measured in the fix validation).
- **The next limit is CPU:** at 20k objects the run is CPU-bound by the quadratic matching (O(targets × dependencies)). Indexing the dependencies (WP3, after the paper) removes it.
