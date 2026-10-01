# RQ8: does `spec.for` remove snapshot false positives without hiding persistent smells?

Engine: `54dc498` (branch `feat/for`, which contains the WP0 changes). Proposal and hypotheses: [`../../../docs/proposals/for.md`](../../../docs/proposals/for.md) §8.

**Method.** Each experiment evaluates two copies of the same Pattern in the same runs:
- the original, with no `spec.for` (today's behaviour);
- a copy with `spec.for`.

Transients and persistent smells are therefore measured on identical cluster states. Runs are out-of-cluster (`perf-local.sh`) at a fixed period.

## Krateo: widgets wired during development (`kp-krateo`, 2026-10-01)
Script: `krateo/scripts/rq8-dev-wiring.sh`. Objects: `krateo/scenario/rq8/`. Pattern: `paragraph-not-referenced`, with the copy at `for: 5m`. Period 180 s.

| Paragraph | What happens |
|---|---|
| `rq8-stale` | never wired: a persistent smell |
| `rq8-wip` | created unreferenced, wired into the Column after run 2 (development in progress) |
| `rq8-moved` | wired from the start; the Column is deleted after run 1 and re-created after run 2 (a re-sync) |

| Run (t) | No `for` (Active) | `for: 5m` | Kyverno 1:1 (forced scan after run 2) |
|---|---|---|---|
| 1 (0 s) | rq8-stale, **rq8-wip** | rq8-stale, rq8-wip: Pending | |
| 2 (180 s) | rq8-stale, **rq8-wip**, **rq8-moved** | all three Pending | `fail`: rq8-stale, **rq8-wip**, **rq8-moved** |
| 3 (360 s) | rq8-stale | rq8-stale **Active** (held 360 s ≥ 5 m) | |
| 4 (540 s) | rq8-stale | rq8-stale Active | |

- **Transients:**
  - without `for`, 3 transient findings: rq8-wip in runs 1 and 2, rq8-moved in run 2;
  - with `for`, none ever becomes Active: they stay Pending and are pruned once wired;
  - Kyverno reports both transients as `fail`.
- **Persistent smell:** rq8-stale becomes Active at run 3. Its `since` stays the first observation (17:33:36Z) in every run.
- **Latency cost:** +2 periods (360 s). `for` = 300 s is rounded up to the next run, as predicted (for.md §2, rule 5).
- Cleanup verified: the namespace and the copy Pattern were removed.

Data: `krateo/timeline.csv`, `krateo/kyverno.csv`. Raw runs: `../raw/perf/krateo/rq8-run{1..4}/`.

## kp-eval: rotation, GitOps move, new orphan (2026-10-01)
Script: `scripts/rq8.sh`. All 14 evaluation Patterns, each with a copy at `for: 4m`; 5 runs at 150 s. Events:
- **After run 1:**
  - CAPI rotation step 1: `dmt-rotated-v3` is created unreferenced (**transient**);
  - Certificate `web` is deleted, so `ci-web` is unreferenced (**transient**: a GitOps delete-and-recreate);
  - ClusterIssuer `ci-rq8-new` is created and never referenced (**new persistent smell**).
- **After run 2:**
  - forced Kyverno scan;
  - rotation step 2: `md-rotated` switches to v3, so `dmt-rotated-v2` is left behind (**new persistent smell**);
  - Certificate `web` is restored.

Scored against the RQ2 ground truth, which describes the seeded state. "Unlisted" Smells hit objects created by the events. Copies are scored on Active Smells only.

| Run (t) | No `for`: TP / FP / unlisted | `for: 4m` (Active): TP / FP / unlisted | Event objects, no `for` | Event objects, `for: 4m` |
|---|---|---|---|---|
| 1 (0 s) | 43 / 0 / 0 | 0 / 0 / 0 (all 46 Pending) | – | – |
| 2 (150 s) | 43 / 1 / 2 | 0 / 0 / 0 | **v3**, **ci-web**, ci-rq8-new Active | all Pending |
| 3 (300 s) | 43 / 1 / 1 | **43 / 0 / 0** | v2, ci-rq8-new Active | v2, ci-rq8-new Pending |
| 4 (450 s) | 43 / 1 / 1 | 43 / 0 / 1 | v2, ci-rq8-new Active | ci-rq8-new Active, v2 Pending |
| 5 (600 s) | 43 / 1 / 1 | 43 / 1 / 1 | v2, ci-rq8-new Active | v2, ci-rq8-new Active |

Kyverno 1:1, forced scan after run 2: `fail` on **dmt-rotated-v3**, **ci-web** and ci-rq8-new.

Hypotheses (for.md §8):
- **(a) holds.** No transient ever becomes Active with `for`. Without `for` there are 2 transient findings (v3 and ci-web in run 2, the FP and an unlisted Smell). Every one of the 43 seeded persistent smells becomes Active at run 3 (held 300 s ≥ 240 s), and none is missed.
- **(b) holds.** Appear latency grows by exactly 2 periods, which is `for` = 240 s rounded up to the next run:
  - `ci-rq8-new`: run 2 without `for`, run 4 with it;
  - `dmt-rotated-v2`: run 3 without, run 5 with.

  Disappear latency is unchanged: transients are pruned at the first run where they are referenced again, in both sets.
- **(c) holds.** Kyverno reports both transients as `fail` (and emits PolicyViolation Events).

**Notes:**
- In runs 3–5 the FP of the "no for" column is `dmt-rotated-v2`. It is a real orphan after the rotation, but a negative in the seeded ground truth. It is not a transient.
- `since` is preserved across runs (e.g. `dmt-rotated-v1` keeps the first observation of the regression run, 19:06:12Z).
- **Cost:** 249–261 requests per run for 28 Patterns (two sets), 3.1–3.2 s per run. A Pending Smell costs the same writes as an Active one.
- **Cleanup verified:** the copies were deleted and the scenario restored. The next run gave 46 Smells and 43/52/10 again; 193 requests, because `PruneOrphans` deleted the copies' Smells.

Data: `eval/timeline.csv`, `eval/scores.csv`, `eval/kyverno.csv`. Raw runs: `../raw/perf/rq8/run{1..5}/`.
