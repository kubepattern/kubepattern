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

## kp-eval (pending)
`scripts/rq8.sh` runs 5 runs at 150 s, with `for: 4m` copies of the 14 Patterns. Events:
- CAPI rotation (transient new template, then the old template left behind);
- GitOps delete-and-recreate of Certificate `web` (transient `ci-web`);
- a new never-referenced ClusterIssuer (latency).

Scoring: the RQ2 ground truth (copies scored on Active Smells only).
