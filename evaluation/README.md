# KubePattern evaluation: relational smells beyond Krateo

This folder is a reproducible evaluation of KubePattern (Go engine, `dev` @ `69d4ffd`, **unmodified**). It runs on 9 CRD ecosystems installed side by side in one minikube cluster and is meant as material for Section 4 (CronJob mode) of the paper.

A fix for the Smell GC defect it uncovered (`fix/per-pattern-prune` @ `a814e4a`) is validated in a separate before/after section.

- [`SOURCE-PLAN.md`](SOURCE-PLAN.md): the testing plan extracted from the research note.
- [`TEST-PLAN.md`](TEST-PLAN.md): the rewritten plan, grounded in the verified engine semantics. It covers the research questions, subjects, seeding, oracles, protocols, the limitation catalogue and threats to validity.
- This file: how to reproduce the evaluation, and the results.

## Headline results
| RQ | Result |
|---|---|
| RQ1 Expressiveness | 14 Patterns (26–96 lines of YAML) cover **9 CRD ecosystems with zero engine changes**. Of 49 relational smells from the literature note, 27 are expressible, 10 partially, 12 not; the blocking limitations fall into a catalogue G1–G10. |
| RQ2 Effectiveness | On 43 seeded smells and 52 negatives (29 near-misses): **precision 1.00, recall 1.00**, identical over 3 runs. **10/10** limitation probes behave as the semantics predict. Mutation testing: **15/15** mutants detected and restored. Natural state: 40/45 built-in KubeVela definitions are unused, with 45/45 agreement with an independent oracle. |
| RQ3 Complementarity | The platforms report only **3/50** of these smells, and only as transient Warning Events (CNPG `NameClash`). After the 1 h Event TTL they report **0/50**. |
| RQ4 Efficiency | Whole cluster: 23 s, 30 MB, 0.26 s CPU, 129 API requests. **About 97% of the time is client-side throttling** (5 QPS, 200 ms gaps), which caps a run at about 750 Smells (5-minute deadline). Memory is linear (about 9 KB/object) and matching CPU is quadratic, hidden by the throttling up to about 20k objects. |
| RQ5 CronJob mode | 9 scheduled runs with scripted fixes, injections, a missing CRD, narrowed RBAC and a template rotation: **9/9 snapshots match the prediction**. Smell identity is stable (40 Smells updated in place across all runs), fixes self-heal in the next run, and failures are isolated per pattern. Weak spot found: the GC is fail-open (skipped patterns, deadline overruns and overlapping runs delete valid Smells). |
| RQ6 Comparison with Kyverno | Kyverno 1.19 (CEL `ValidatingPolicy`) translated 1:1 reproduces all **105/105** verdicts: P = R = 1.00, 10/10 probes as KubePattern predicts, mutation 15/15. This cross-validates the semantics. CEL is **more expressive**: best-effort variants resolve **10/10** probes, and the policies are shorter (417 vs 657 lines). At the same 5-minute period Kyverno costs **477 MiB always-on, about 50× the CPU and about 23× the API requests** (39,155/h vs 1,716/h); KubePattern has zero idle footprint. Detection latency after a dependency change is bounded by the period for both. |
| Fix validation | Porting the per-pattern prune from the `operator` branch (`fix/per-pattern-prune` @ `a814e4a`) removes all three failure modes, with no regression (RQ2 43/52/10, mutation 15/15). Overlapping runs keep **46/46** Smells instead of 12/46; a skipped pattern keeps its Smells (**0** instead of 3 GC'd); above the ceiling **0** valid Smells are deleted (instead of 47–51 per run) and coverage converges (752 → 797 → 804 of 805). Cost: 14 extra LISTs per run (+2.8 s). |


## Layout
```
env/            versions.env (pins), minikube-up.sh, audit-policy.yaml, install/NN-*.sh, values/ (RBAC variants)
patterns/       14 evaluated Patterns (one folder per platform), kubevela/natural/ (catalogue variant), _robustness/
scenarios/      <platform>/{base.yaml,seeds.yaml,[pre|post].sh,ground-truth.csv}, mutations.csv
scripts/        build-image.sh, apply-scenario.sh, apply-patterns.sh, run-once.sh, score.py, verify_fields.py,
                natural-findings.sh, mutate.sh, collect_oracles.py, audit_summary.py, perf-local.sh, scale.py,
                cronjob-timeline.sh, timeline_report.py, pattern_metrics.py, make_tables.py,
                concurrency_check.sh, gcfix-remeasure.sh, gcfix_compare.py (GC fix validation),
                kyverno-*.sh, kyverno_reports_to_smells.py, policy_metrics.py, comparison_*.py, audit-harvest.sh,
                footprint-sample.sh, audit_by_user.py, cost_report.py, staleness.sh, staleness_report.py (RQ6)
comparison/     kyverno/: RBAC, GlobalContextEntries and ValidatingPolicies of the RQ6 comparison
results/        CSV used in the paper, tables/*.tex (booktabs); results/raw/ holds the per-run archives (git-ignored)
```

## Reproduce
Requirements: minikube (kvm2), kubectl, helm, podman, jq, python3 with PyYAML, and `htpasswd` (httpd-tools). The profile needs 8 vCPU and 16 GiB; about 4 GiB are actually used.

```bash
cd evaluation
./env/minikube-up.sh                                    # profile kp-eval, k8s v1.34.4, audit log on
./scripts/build-image.sh build && ./scripts/build-image.sh extract
for s in env/install/[0-9]*.sh; do [[ $s == *95-kyverno* ]] || "$s"; done   # KubePattern + 9 platforms (~15 min); Kyverno is RQ6 only
./scripts/apply-scenario.sh && ./scripts/apply-patterns.sh
./scripts/verify_fields.py                              # every Pattern path exists in the live CRD schemas

# RQ2 effectiveness (3 repetitions)
for i in 1 2 3; do out=$(./scripts/run-once.sh full-$i | tail -1); ./scripts/score.py "$out/smells.json"; done
./scripts/score.py results/raw/full-3/*/smells.json --out results/effectiveness
./scripts/mutate.sh                                     # RQ2b mutation testing
./scripts/natural-findings.sh                           # RQ2 natural state (KubeVela catalogue)
./scripts/collect_oracles.py                            # RQ3 complementarity
./scripts/pattern_metrics.py                            # RQ1 pattern size
./scripts/cronjob-timeline.sh && ./scripts/timeline_report.py   # RQ5 (~20 min)
./scripts/scale.py && ./scripts/scale.py cleanup       # RQ4 (~1 h), then restore the evaluation state
./scripts/make_tables.py                                # LaTeX tables in results/tables/

# Fix validation (per-pattern prune): build the fixed engine, then re-measure the GC scenarios
KP_COMMIT=a814e4a ./scripts/build-image.sh all          # image eval-a814e4a + bin/kubepattern-a814e4a
./scripts/gcfix-remeasure.sh a814e4a                    # before/after (~1.5 h), leaves the fix installed
./scripts/gcfix_compare.py                              # results/gcfix/compare.csv, tables/gcfix.tex

# RQ6 comparison with Kyverno (the audit policy must include the kyverno ServiceAccounts: restart the profile)
KYVERNO_SCAN_INTERVAL=5m ./env/install/95-kyverno.sh
kubectl apply -f comparison/kyverno/rbac.yaml -f comparison/kyverno/globalcontext/
kubectl apply -f comparison/kyverno/equivalent/
for i in 1 2 3; do ./scripts/kyverno-run.sh equivalent-r$i; done  # forced full re-scans, archived
kubectl apply -f comparison/kyverno/best-effort/
for i in 1 2 3; do ./scripts/kyverno-run.sh best-effort-r$i; done
kubectl apply -f comparison/kyverno/equivalent/
./scripts/comparison_effectiveness.py && ./scripts/policy_metrics.py
MUTATION_RUNNER=./scripts/kyverno-run.sh MUTATION_LABEL=kyverno-mutation MUTATION_OUT=results/comparison/mutation ./scripts/mutate.sh
# cost: 1 h idle window, then scripts/cost_report.py <window> --kp-run <run-once archive> --kp-perf <perf-local line>
./scripts/footprint-sample.sh <dir>/footprint.tsv 3600 10 & ./scripts/audit-harvest.sh <dir>/audit.jsonl 3600 120
# staleness: CronJob */5 unsuspended, then REPS=5 PERIOD=300 ./scripts/staleness.sh && ./scripts/staleness_report.py <run>
./scripts/kyverno-robustness.sh
./scripts/comparison_tables.py                          # results/tables/comparison.tex
```

## Results

All numbers below come from `results/` and were produced on the `kp-eval` profile with the pinned versions in `env/versions.env`.

### RQ1: Expressiveness
The evaluation covers 14 Patterns on 9 platforms with **no change to the engine**. The platforms span 33 distinct GVRs, all read through the dynamic client after discovery. No Pattern needs platform-specific code.

| Metric | Value |
|---|---|
| Pattern size (non-comment YAML lines) | min 26, median 38, max 96 (total 657) |
| Relationship types | 13 `custom`, 1 `owns` |
| Relationship groups | 13 `matchNone`, 1 `matchAll` |
| Patterns with >1 dependency (several `matchNone` entries) | 6 (up to 4 dependencies and 5 entries: CAPI) |
| Patterns with `[*]` wildcards / nested wildcards | 7 / 1 |
| Patterns with target or dependency filters | 6 |
| Field paths verified against live CRD schemas (`kubectl explain`) | 41 / 41 |

The source note's long list has 49 relational smells, classified against the verified semantics in `results/expressiveness/expressiveness.csv`:

| Verdict | Smells |
|---|---|
| expressible | 27 (55%) |
| partial | 10 (20%) |
| not expressible | 12 (25%) |

Blocking limitations for partial or not-expressible smells (a smell may have several):

| Limitation | Count |
|---|---|
| G3 implicit defaults | 6 |
| G6 kind wildcards | 4 |
| G2 per-element quantification | 3 |
| G4 string-encoded refs | 3 |
| G7 selectors | 3 |
| G1 correlation | 2 |
| G5 key escaping | 2 |
| G8 / G9 | 1 each |

### RQ2: Effectiveness
The seeded scenarios contain 43 positives and 52 negatives, 29 of them near-misses. Results were identical over 3 repetitions: run 1 goes through the CREATE path, runs 2 and 3 through the UPDATE path.

| Platform | Pattern | Pos | Neg | TP | FP | FN | TN | Prec. | Rec. | Probes (as predicted) |
|---|---|---|---|---|---|---|---|---|---|---|
| Argo CD | `argocd-applicationset-generates-nothing` | 3 | 3 | 3 | 0 | 0 | 3 | 1.00 | 1.00 | 0 |
| Argo CD | `argocd-appproject-empty` | 3 | 3 | 3 | 0 | 0 | 3 | 1.00 | 1.00 | 0 |
| Cluster API | `capi-machinetemplate-orphan` | 3 | 6 | 3 | 0 | 0 | 6 | 1.00 | 1.00 | 1 (1) |
| cert-manager | `certmanager-clusterissuer-not-used` | 3 | 4 | 3 | 0 | 0 | 4 | 1.00 | 1.00 | 2 (2) |
| CloudNativePG | `cnpg-cluster-without-scheduledbackup` | 3 | 3 | 3 | 0 | 0 | 3 | 1.00 | 1.00 | 1 (1) |
| CloudNativePG | `cnpg-pooler-name-clashes-with-cluster` | 3 | 3 | 3 | 0 | 0 | 3 | 1.00 | 1.00 | 0 |
| Crossplane | `crossplane-composition-without-xrd` | 3 | 3 | 3 | 0 | 0 | 3 | 1.00 | 1.00 | 1 (1) |
| Crossplane | `crossplane-function-not-used` | 3 | 4 | 3 | 0 | 0 | 4 | 1.00 | 1.00 | 0 |
| External Secrets | `eso-clustersecretstore-not-used` | 3 | 4 | 3 | 0 | 0 | 4 | 1.00 | 1.00 | 1 (1) |
| External Secrets | `eso-secretstore-not-used` | 3 | 5 | 3 | 0 | 0 | 5 | 1.00 | 1.00 | 2 (2) |
| Flux | `flux-ocirepository-not-used` | 4 | 4 | 4 | 0 | 0 | 4 | 1.00 | 1.00 | 1 (1) |
| Kargo | `kargo-warehouse-not-requested` | 3 | 4 | 3 | 0 | 0 | 4 | 1.00 | 1.00 | 0 |
| KubeVela | `kubevela-componentdefinition-not-used` | 3 | 3 | 3 | 0 | 0 | 3 | 1.00 | 1.00 | 1 (1) |
| KubeVela | `kubevela-traitdefinition-not-used` | 3 | 3 | 3 | 0 | 0 | 3 | 1.00 | 1.00 | 0 |
| **Total** | | **43** | **52** | **43** | **0** | **0** | **52** | **1.00** | **1.00** | **10 (10)** |

- **Limitation probes** are scored separately. All 10 behaved exactly as the verified semantics predict: 7 limitation false negatives and 3 limitation false positives.

  | Kind | Count | Probes |
  |---|---|---|
  | FN | 7 | G1: CAPI `t1`, ESO `ss-g1`, ESO `css-g1b`; G4: Crossplane foreign group, cert-manager foreign group; G8: cert-manager retained CertificateRequest; G10: CNPG suspended ScheduledBackup |
  | FP | 3 | G3: Flux explicit `chartRef.namespace`; G4: KubeVela `name@v1`; G7: ESO label-selected store |

  So the limitations are **predictable from the DSL semantics**; they are not random errors.
- **Mutation testing** (`results/mutation/mutation.csv`) used 14 mutants, one per pattern, with the operators delete-referrer, rename-reference, delete-referenced and introduce-clash. It also recorded one side effect: the Cluster introduced by M03 has no ScheduledBackup, so a second pattern rightly flags it.
  - All **15/15 were detected**, with no unexpected Smell.
  - After the revert, the Smell set was **identical to the baseline**, because the stale Smells were garbage-collected.
- **Natural state** (`results/natural/kubevela.csv`): KubeVela ships 45 built-in definitions, and **40 of them (89%) are unused** by the Applications in the cluster (8/10 ComponentDefinitions, 32/35 TraitDefinitions). KubePattern agrees with an independent `jq` oracle on **45/45**.

### RQ3: Complementarity with the platforms
`results/oracles/` records, for each of the 50 smelly objects (positives and probes), whether the platform itself exposes a *discriminative* signal: an unhealthy condition or a Warning Event that does not also appear on clean objects of the same pattern.

| When collected | Smells reported by the platform |
|---|---|
| right after seeding (`oracles-fresh.csv`) | **3 / 50** (6%): only the CNPG `NameClash` Warning Events on the clashing Poolers |
| after the Event TTL (1 h, `oracles-after-event-ttl.csv`) | **0 / 50**: the Events expired and the Poolers carry no status |

- None of the 9 platforms reports an unused definition. For example, the Functions, Warehouses, ClusterIssuers and stores all show `Ready`/`Healthy`, and the ApplicationSets that generate nothing report `ErrorOccurred=False`.
- Crossplane accepts a Composition for a kind no XRD defines: 2.4 has no Composition webhook and no status for it.
- Only one misconfiguration is prevented upstream: CNPG's webhook rejects a Pooler that has the same name as the Cluster it points to. It still accepts a Pooler named like *another* Cluster, which KubePattern flags.
- KubePattern turns transient or missing signals into **persistent, queryable CRs** (`kubectl get smells -A`).

### RQ4: Efficiency and scalability
The whole scenario (14 Patterns, 9 platforms, 215 objects fetched) gives these per-run numbers (`results/effectiveness/runs.csv`, `results/scale/full-cluster.csv`):

| Metric | Value |
|---|---|
| In-cluster Job duration | 25–27 s |
| Out-of-cluster wall time (3 runs) | 23.4 s |
| Peak RSS | 30 MB |
| CPU (user + sys) | 0.26 s |
| API requests | 129: 2 discovery, 35 LIST (33 GVRs + Patterns + Smells), 46 GET, 46 CREATE/UPDATE |
| Phases (from µs audit timestamps) | fetch 4.8 s, Smell writes 18.2 s, GC < 0.01 s |
| Gap between consecutive requests | median **200.0 ms**, against a median server latency of 2–4 ms |

About 97% of the run is spent waiting on client-go's default rate limiter (5 QPS, burst 10). The engine computes almost nothing: the Pattern evaluation itself is sub-second.

The scale sweep (`results/scale/summary.csv`, runs in `runs.csv`) uses suspended Flux objects with the controllers at 0. Each run is the out-of-cluster binary under `/usr/bin/time -v`, authenticated as the ServiceAccount.

**S2, Smell count** (1,000 sources, 1 run per cell; the rate limiter makes runs deterministic):

| Orphans | Smells written | Wall (cold / warm) | API requests | Note |
|---|---|---|---|---|
| 0 | 5 | 1.0 / 1.0 s | 17 | |
| 100 | 105 | 41.0 / 41.0 s | 217 | |
| 200 | 205 | 81.1 / 81.1 s | 417 | |
| 400 | 405 | 161.1 / 161.1 s | 817 | |
| 800 | **752 of 805** | 300.1 / 309.8 s | 1,512 / 1,561 | **5-minute deadline hit** |
| 800 → all fixed (stale) | 5 | 150.4 s | 764 | 747 DELETEs by GC (GC has no deadline) |

- Wall time is **0.4 s per Smell** (GET + CREATE/UPDATE at 5 QPS). The ceiling is therefore about 750 Smells per run.
- Above the ceiling, the warm run of this sweep issued 703 UPDATE + **49 CREATE + 49 DELETE**. The same engine gave 47 and 51 on two consecutive warm runs in the re-measurement (see *Fix validation*), hence the "47–51 per run" used elsewhere. Smells that were still valid but not refreshed before the deadline were garbage-collected, and others were re-created, because patterns and targets are evaluated in random map order. Smells **flap** from run to run on an unchanged cluster.

**S3, Pattern count** (K copies of the Flux pattern over the same kinds, 5 runs each, IQR ≤ 0.01 s):

| K | Smells | Wall | CPU | API requests |
|---|---|---|---|---|
| 1 | 5 | 1.0 s | 0.5 s | 17 |
| 5 | 25 | 9.0 s | 1.8 s | 57 |
| 10 | 50 | 19.0 s | 3.6 s | 107 |
| 20 | 100 | 39.0 s | 7.2 s | 207 |

The number of LISTs is constant: each GVR is listed once, whatever the number of patterns. Requests grow only with the Smells written (7 + 10K). CPU grows linearly, because every pattern re-evaluates its own targets × dependencies.

**S1, object count** (100 orphans, i.e. 105 Smells; 5 runs each):

| Objects (sources + HelmReleases) | Wall (median) | Peak RSS | CPU | Fetch phase |
|---|---|---|---|---|
| 1,914 | 41.0 s | 44 MB | 0.9 s | 0.11 s |
| 4,914 | 41.0 s | 71 MB | 2.9 s | 0.28 s |
| 9,914 | 41.0 s | 116 MB | 9.0 s | 0.55 s |
| 19,914 | 41.6 s | 204 MB | **38.5 s** | 1.13 s |

- Memory grows linearly, about 9 KB per object, because LISTs are unpaginated and the whole graph is held in memory. At 20k objects it is still well under the chart's 1 GiB limit.
- CPU grows **quadratically** (×3.1 then ×4.3 per doubling). The engine matches every target against every dependency candidate (O(T·D), with no index).
- Up to about 20k objects this cost is hidden behind the throttled writes, because matching overlaps the rate-limiter waits. At 20k objects CPU ≈ wall time, so beyond that the engine itself becomes the bottleneck.
- The T=1,000 IQR of 9.5 s comes from its first run, which also garbage-collected the 95 Smells of the S3 clones.

**Implications.** All measurements above use the engine as-is.
- *Done:* pattern-scoped GC that never prunes skipped or deadline-cut patterns. See *Fix validation*: it removes the flapping but not the ceiling.
- *Future work:*
  - raise client QPS/Burst, since server latency is 2–4 ms;
  - index dependencies by the values of the first criterion (hash join instead of nested loops);
  - paginate LISTs;
  - share a single Smell LIST across patterns in the per-pattern prune.


### RQ5: CronJob mode (snapshot semantics)
The CronJob ran every 2 minutes for 9 runs, with a scripted event between runs (`results/cronjob/timeline.csv`, `lifecycle.csv`). **All 9 snapshots equal the predicted Smell set.**

| Run | Event before the run | Smells | Created | Kept (updated in place) | Garbage-collected | Skipped patterns |
|---|---|---|---|---|---|---|
| 1 | baseline | 46 | – | – | – | – |
| 2 | E1 fix 4 smells (delete 3 orphans, add a Stage for a 4th Warehouse) | 42 | 0 | 42 | 4 | – |
| 3 | E2 inject 3 new unused objects | 45 | 3 | 42 | 0 | – |
| 4 | E3 add a Pattern whose CRD is not installed | 45 | 0 | 45 | 0 | kratix (logged, run completes) |
| 5 | E4 narrow RBAC (no `kargo.akuity.io`) | 42 | 0 | 42 | **3** | kargo, kratix |
| 6 | E5 restore RBAC | 45 | 3 | 42 | 0 | kratix |
| 7 | E6 rotation step 1: new template created, not yet used | 46 | **1 (transient)** | 45 | 0 | kratix |
| 8 | E6 rotation step 2: MachineDeployment switched | 46 | 1 | 45 | 1 | kratix |
| 9 | E7 revert to the seeded scenario | 46 | 4 | 42 | 4 | – |

- **Stable identity.** 40 of the 51 Smell identities were present in all 9 runs as **one single Smell object**: same UID, updated in place by the deterministic name `<pattern>-<targetUID>`.
- **Self-healing.** A fixed smell disappears in the next run and no manual clean-up is needed.
- **Robustness.** A missing CRD and a narrowed RBAC only skip the affected pattern; every Job completed (25–27 s).
- **Fail-open GC (as-is engine; fixed, see *Fix validation*).** When a pattern is skipped (RBAC), `CleanOldScans` deletes its Smells. An unreadable resource type is therefore reported as "resolved" (run 5), and its Smells come back as *new* objects once access returns (run 6).
- **Transient (G8).** During a two-step template rotation, the new template is flagged for exactly one run (run 7). With a longer CronJob period or a minimum-age filter this disappears.
- **Timeout (observed incidentally).** In the first attempt of this experiment the host suspended during run 5. The Job resumed after its 5-minute deadline, every remaining write failed, and GC then deleted the 11 Smells that had not been refreshed (42 → 31). The archive is kept in `results/raw/cronjob-aborted-host-suspend/`. Same mechanism as above: **a run that exceeds the deadline silently resolves the Smells it could not refresh.** RQ4 reproduces this deliberately.

### Fix validation: per-pattern prune
The fail-open GC found by RQ4 and RQ5 was fixed on branch `fix/per-pattern-prune` (commit `a814e4a`) by porting the design already used on the `operator` branch:
- Every Smell is labeled `kubepattern.dev/pattern-uid`.
- Each pattern computes its *live* set from the evaluation (a failed write stays live) and deletes only its own Smells that are no longer live.
- Deletions are bounded by the run context, and skipped patterns are never pruned.
- `PruneOrphans` removes the Smells of uninstalled Patterns and unlabeled Smells left by older versions.

The change adds 9 unit tests (`internal/analysis/engine_test.go`, `internal/kube/report_test.go`). `go vet` is clean; the only failures are the pre-existing linter tests (D4).

`scripts/gcfix-remeasure.sh` repeated the affected scenarios with both engines (`results/gcfix/compare.csv`, `results/tables/gcfix.tex`):

| Scenario | Metric | As-is `69d4ffd` | Per-pattern prune `a814e4a` |
|---|---|---|---|
| D8: two overlapping runs | Smells left after both runs (of 46) | **12** | **46** |
| RQ5 run 5: RBAC narrowed | Smells of the skipped pattern garbage-collected | 3 | **0** |
| RQ5 run 6: RBAC restored | Smells re-created as new objects | 3 | **0** (same objects kept) |
| RQ4 S=800, warm run | valid Smells deleted / Smells after the run | 47 / 752 | **0** / 797 |
| RQ4 S=800, 2nd warm run | valid Smells deleted / Smells after the run | 51 / 752 | **0** / 804 of 805 |
| RQ4 S=800, all orphans fixed | DELETEs, wall time, Smells left (expected 5) | 747, 150 s, 5 | 799, 161 s, 5 |

Regression and migration checks:
- RQ2 still gives 43/52/10 with no unlisted Smell, across 3 runs with the fixed engine.
- Mutation testing still gives 15/15 killed and restored, with reverted == baseline (`results/gcfix/mutation-after/`).
- The RQ5 timeline matches the per-pattern expectations **9/9** (`results/cronjob-fix/`).
- The first run of the fixed engine relabeled every existing Smell, leaving 0 unlabeled.
- **Cost:** one label-selected LIST of Smells per pattern, plus one for orphans. That is 143 instead of 129 requests on the whole cluster, i.e. +2.8 s at 5 QPS. A single LIST shared by all patterns would remove this overhead (future work).
- **What the fix does not change:** the throttling ceiling. A run still writes at most about 750 Smells. The difference is that the Smell set now converges monotonically over consecutive runs instead of flapping.

### RQ6: Comparison with Kyverno
> How does KubePattern compare with a general-purpose policy engine on the same smells, cluster and ground truth? The protocol and hypotheses H1–H5 are in [`COMPARISON-PLAN.md`](COMPARISON-PLAN.md).

**Set-up.**
- Kyverno **v1.19.1** (chart 3.9.1): default install, Audit only, background scan on, `env/install/95-kyverno.sh`.
- KubePattern side: the engine with the per-pattern prune (`a814e4a`). RQ2 and mutation testing give the same results with both engines (see *Fix validation*).
- **API choice.** In 1.19 the JMESPath `ClusterPolicy` is deprecated. The comparison therefore uses the GA CEL `ValidatingPolicy` (`policies.kyverno.io/v1`), which supports background scans with cross-resource lookups.
  - Every dependency kind is read through a **`GlobalContextEntry`** (`kyverno.io/v2`, an informer-cached list of one GVR, all namespaces). There are 21 entries, shared by all policies.
  - `apicall/` holds the same policies with live `resource.List` calls. It was generated but not measured.
- Controller read access comes from an aggregated ClusterRole covering the same 15 API groups as the Patterns (`comparison/kyverno/rbac.yaml`).
- Reports are converted to the smells.json shape (`scripts/kyverno_reports_to_smells.py`) and scored with the **unchanged** ground truth and `score.py`.
- Between repetitions a full re-scan is forced by restarting the reports controller, which re-evaluates all 105 results in 62–103 s.

```
comparison/kyverno/
  rbac.yaml              aggregated read access (admission, background, reports controllers)
  globalcontext/         21 GlobalContextEntries, one per dependency GVR
  equivalent/            14 ValidatingPolicies: literal 1:1 translations, same names as the Patterns
  best-effort/           8 variants of the probed policies that use CEL's extra power
  apicall/               equivalent/ with live resource.List calls (cost variant, not measured)
  robustness/            R1a/R1b missing CRDs, R2 RBAC without kargo.akuity.io
```

**RQ6a: Expressiveness** (`results/comparison/expressiveness.csv`, `scripts/policy_metrics.py`).
- All 14 smells are expressible in CEL.
- `owns` is the only KubePattern primitive without a direct counterpart. KubePattern walks ownerReferences transitively over the fetched graph; CEL has no recursion and checks direct ownership only. That is equivalent on this scenario, because ApplicationSets own their Applications directly.

| Metric (14 smells) | KubePattern Patterns | Kyverno policies (`equivalent/`) |
|---|---|---|
| Lines (non-comment YAML), min/median/max (total) | 26/38/96 (657) | 24/28/46 (417), plus 189 lines of shared GlobalContextEntries |
| Logic lines only (no metadata, display fields or message) | 13/25/83 (475) | 12/16/34 (249) |
| Matching primitives (criteria + filters) / CEL comparisons | 58 | 59 |
| Other constructs | declarative paths with `[*]` | 43 `exists`/`all` comprehensions, 67 optional hops (`.?`, `orValue`), 5,129 characters of CEL |

- **H1 is not confirmed on size.** The CEL policies are *shorter*, with the same number of atomic comparisons (59 vs 58). The complexity moves into dense expression strings:
  - `dyn()` casts are required on the global-context values, otherwise the policy is rejected at admission;
  - optional chaining turns a mistyped field name into a silent default, the same failure mode as a mistyped KubePattern path (empty value set);
  - neither tool validates paths against the CRD schema when a rule is authored (the evaluation did it for the Patterns with `scripts/verify_fields.py`).
- **H3 is refuted in part.** The `best-effort/` variants add 0–13 lines (median +1) and resolve **all 10 probes**, including the two that H3 expected Kyverno not to overcome:

| Limitation | Probe(s) | CEL construct that resolves it |
|---|---|---|
| G1 per-element correlation | CAPI `t1`, ESO `ss-g1`, `css-g1b` | name and kind tested inside the same `exists(m, …)` |
| G3 overridden namespace | Flux `cross-ns` | `d.spec.?chartRef.?namespace.orValue(d.metadata.namespace)` |
| G4 string-encoded references | Crossplane `app-foreign-group`, cert-manager `ci-foreign-group`, KubeVela `kp-versioned` | `apiVersion.split('/')[0] == d.spec.group`, `issuerRef.?group.orValue('cert-manager.io')`, `type.split('@')[0]` |
| G7 selectors | ESO `ss-selected` | PushSecret `labelSelector` evaluated by hand (`matchLabels` + `matchExpressions`, 10 lines) |
| G8 retained history objects | cert-manager `ci-old-ca` | join between two dependencies: a CertificateRequest does not count when its revision annotation is older than its owning Certificate's `status.revision` |
| G10 booleans | CNPG `db-suspended` | `!d.spec.?suspend.orValue(false)` |

- **G8 correction.** This particular probe can also be fixed *inside* the KubePattern DSL: a dependency filter `metadata.ownerReferences IS_EMPTY` keeps only hand-made CertificateRequests. On the live cluster the only owner-less CertificateRequest naming a ClusterIssuer is the one for `ci-manual`, which stays clean. So `ci-old-ca` is a pattern-authoring choice rather than a hard limit. G8 still stands for transients (RQ5 rotation).
- **Analytic, not measured:**
  - G2 (universal quantification) and G5 (key escaping) have direct CEL constructs (`all()`, `labels['a.b/c']`);
  - G6 (kind wildcards) is only partly covered: `matchConstraints` accepts wildcards on the target, but a dependency list needs one GlobalContextEntry or `resource.List` per GVR;
  - G9 (template or remote content) was not attempted.

**RQ6b: Effectiveness** (`results/comparison/effectiveness.csv`, per-run scores in `results/comparison/effectiveness/`, `scripts/comparison_effectiveness.py`).

| Policy set | Runs | TP | FP | FN | TN | Precision | Recall | Probes as the KubePattern semantics predict | Probes resolved (truth) | Error results |
|---|---|---|---|---|---|---|---|---|---|---|
| `equivalent/` (1:1) | 3 | 43 | 0 | 0 | 52 | 1.00 | 1.00 | **10/10** | 0/10 | 0 |
| `best-effort/` | 3 | 43 | 0 | 0 | 52 | 1.00 | 1.00 | 0/10 | **10/10** | 0 |

- **H2 is confirmed.** The 1:1 translation agrees with KubePattern on all 105 verdicts, including the 10 probes. An independent engine that implements the verified semantics reproduces them exactly, which **cross-validates the semantics in §2.1 of `TEST-PLAN.md`**.
- **Mutation testing** (`MUTATION_RUNNER=scripts/kyverno-run.sh scripts/mutate.sh`, `results/comparison/mutation/`): **15/15** mutants killed and restored, with no unexpected or lost finding and reverted == baseline. This is the same as KubePattern.

**RQ6c: Cost** (`results/comparison/cost.csv`, `scripts/cost_report.py`).
- Kyverno: one idle hour (08:03–09:03 UTC), background scan every 5 min (12 scans). The inputs are:
  - per-container CPU counters and working-set memory from the kubelet every 10 s (`scripts/footprint-sample.sh`);
  - the audit log of every ServiceAccount in namespace `kyverno`, harvested every 2 min because the kubelet rotates it (`scripts/audit-harvest.sh`, `scripts/audit_by_user.py`).
- KubePattern: one in-cluster run (audit) and one out-of-cluster run (`perf-local.sh`, RSS and CPU), scaled to one run per 5 min.

| Per hour, one cycle every 5 min | KubePattern (Job) | Kyverno reports controller | Kyverno, whole install (4 controllers) |
|---|---|---|---|
| Memory when idle | **0** (31 MB peak for ~26 s per run) | 92 MiB | **477 MiB** (admission 164, cleanup 115, background 106, reports 92) |
| CPU | 4.8 s (0.40 s per run) | 186 s | **248 s** (0.07 cores on average) |
| API requests | 1,716 (**143 per run**) | 15,447 (1,138 per cycle without leases) | **39,155** (3,263 per cycle) |
| Reads of the analysed kinds | 49 LISTs per run | 0 LISTs (informers; ~18 WATCH reconnections/h per kind) | – |
| Output writes | 46 Smell updates per run | ~349 report writes + ~92 Events per cycle | – |

- **H4 is confirmed in direction but not in mechanism.** Informers do remove the per-cycle LISTs of the analysed kinds: in the hour, the only LISTs were of (Cluster)EphemeralReports. However, the reporting pipeline alone issues about 8× the requests of a KubePattern run per cycle:
  - EphemeralReports are created, listed, fetched and aggregated into PolicyReports;
  - a PolicyViolation Event is emitted for each failing result.
- The rest of the default install adds background traffic unrelated to these policies:
  - 14,040 SelfSubjectAccessReviews/h from the cleanup controller;
  - about 750 webhook-configuration updates/h from the admission controller;
  - 7,519 lease renewals/h.
- CPU is about 39× (reports controller only) to 52× (whole install) that of KubePattern.

**RQ6d: Operational behaviour.**
- **Staleness (H5)** (`scripts/staleness.sh`, `results/comparison/staleness*.csv`).
  - Protocol: CronJob `*/5` and `backgroundScanInterval=5m`, both observing the same change at the same time. The change is M02: the ScheduledBackup of `db-live` is deleted (the smell appears) or re-created (the smell disappears). 5 repetitions per direction, at random offsets, polled every 2 s.

  | Latency (s) | KubePattern appear | KubePattern disappear | Kyverno appear | Kyverno disappear |
  |---|---|---|---|---|
  | median (min–max) | 149 (62–320) | 138 (108–313) | 106 (56–290) | 162 (47–179) |

  - **H5 is confirmed.** Neither tool reacts to a dependency-side change. KubePattern sees it 8–27 s after its next CronJob tick. Kyverno sees it 61–63 s after a wall-clock 5-minute boundary in all 10 observations, i.e. always at the same phase of its own scan: the background scan re-evaluates *all* 105 results in one burst per interval. It does not re-queue per resource, and it does not track GlobalContextEntry changes.
  - The maximum latency is about one period for both (1.07 vs 0.97 periods). Which tool wins a given repetition depends only on the phase of the change relative to the two ticks.
- **Robustness** (`scripts/kyverno-robustness.sh`, `results/comparison/robustness.csv`):

  | Case | KubePattern | Kyverno 1.19 |
  |---|---|---|
  | R1a target CRD not installed | pattern skipped and logged; run completes | policy accepted; status `RBACPermissionsGranted=False` with a misleading "missing permissions" message; no results; other policies unaffected |
  | R1b dependency CRD not installed | pattern skipped and logged | policy Ready; **every target reported as `error`** (9/9) in the PolicyReports; other policies unaffected |
  | R2 RBAC without `kargo.akuity.io` | as-is: Smells garbage-collected (fail-open); fix: Smells kept; skip logged per run | watcher cannot start (forbidden): **the last results are kept unchanged, with no error result and the policy still Ready**; the problem appears only in the controller log |
  | R2 restored | fix: same Smell objects updated in place | watchers restart; results identical to the baseline |

  Kyverno is more visible than KubePattern on a missing dependency (per-resource `error` results). On lost read access to the target it is **silently stale**, which is the same outcome as the fixed KubePattern (results kept, the problem only in the logs). The as-is KubePattern instead deletes them (fail-open).
- **Output model.**
  - Kyverno reports per resource (one PolicyReport per object, pass/fail/error/skip per policy) and emits a PolicyViolation Event per failure. Report results have optional `severity` and `category` fields, which these policies do not set.
  - KubePattern writes one Smell per finding, with severity, category, message, reference and a stable identity `<pattern>-<targetUID>`, and nothing for clean objects.
  - Kyverno's `messageExpression` is full CEL, whereas KubePattern has only 5 placeholders.

**Summary.** On relational orphan smells, Kyverno 1.19 with CEL matches KubePattern's effectiveness exactly and is strictly more expressive. KubePattern delivers the same detection with:
- **zero idle footprint**, against about 480 MiB of always-on controllers;
- **about 23× fewer API requests** and **about 50× less CPU** at the same 5-minute period;
- a declarative, schema-checkable specification instead of CEL code.

Detection latency is bounded by the period in both. The KubePattern limitations G1, G3, G4, G7 and G10 are the concrete gap to close, and CEL shows constructs that close them.

**Threats to validity (RQ6).**
- *Author bias.* The policies and the Patterns come from the same evaluation authors, and the best-effort variants were written knowing the probes. Mitigations:
  - literal 1:1 translation, checked against every near-miss;
  - best-effort reported separately;
  - all policies are in `comparison/kyverno/`.
- *Version and API.* The measurements hold for Kyverno 1.19.1 with CEL policies and GlobalContextEntries. JMESPath `ClusterPolicy` (deprecated) and live `resource.List` calls (`apicall/`) were not measured; the latter would trade informer memory for per-evaluation LISTs.
- *Cost attribution.* Part of Kyverno's traffic comes from components that these policies do not need (cleanup and admission controllers). It is reported separately from the reports controller, which is the fair per-function comparison (about 8× requests and 39× CPU).
- *Scan scheduling.* The latencies depend on the phase of the change relative to each tool's tick. They are reported as distributions with their bound (about one period), not as a winner.
- *Scale.* Everything was measured on the evaluation scenario (215 analysed objects). The optional S1 sweep was not run, so the relative cost at 10k+ objects (informer memory vs unpaginated LISTs) is open.

## Findings to report in the paper
1. **Unused definitions are invisible to the platforms.** Controllers expose dangling references, while orphan and unused definitions go unreported (RQ3). KubePattern covers this niche on 9 ecosystems with 14 declarative Patterns and no engine changes (RQ1, RQ2).
2. **The limitations are principled.** Every seeded case that the DSL cannot handle fails exactly as its semantics predict (10/10 probes). They map to a small catalogue (G1–G10) that gives concrete future work: element-scoped criteria, defaulted paths, string operators, selectors, kind wildcards, typed comparisons and a minimum-age filter.
3. **Run time is dominated by client-side throttling.** client-go's default 5 QPS spaces requests 200 ms apart, against 2–4 ms of server latency. That is about 97% of a run, and it caps one run at about 750 Smells before the 5-minute deadline (RQ4).
   Memory is linear (about 9 KB/object) and matching is quadratic, O(targets × dependencies). CPU reaches wall-clock time at about 20k objects, which motivates indexing the dependencies. Still open: raising QPS/Burst, indexing and paginating LISTs.
4. **The GC is fail-open (fixed).** In the as-is engine, a skipped pattern (RBAC), a run that hits the deadline, or two overlapping runs delete Smells that are still valid. Above the ceiling, Smells flap between runs on an unchanged cluster (47–51 deleted and re-created per run at 805 Smells), and overlapping runs keep only 12/46 Smells (RQ4, RQ5, D8).
   The per-pattern prune (`fix/per-pattern-prune` @ `a814e4a`) removes all three failure modes with no regression, at the cost of one extra LIST per pattern (see *Fix validation*).
5. New limitation **G10**: filter `EQUALS` compares strings only, so boolean fields such as `spec.suspend` cannot be filtered.
6. **Same effectiveness as a general-purpose policy engine, at a fraction of the operating cost (RQ6).**
   - A 1:1 Kyverno 1.19 CEL translation reproduces every KubePattern verdict (105/105, probes included) and the mutation score. This independently confirms the verified semantics.
   - CEL is more expressive: it resolves all 10 limitation probes, including selectors (G7) and retained history (G8), with shorter policies.
   - At the same 5-minute period Kyverno keeps 477 MiB of controllers always on and uses about 50× the CPU and 23× the API requests. Its reporting pipeline alone (EphemeralReports, PolicyReports, Events) issues 8× the requests of a KubePattern run, even though it reads the analysed kinds from informers.
   - Neither tool reacts to dependency-side changes: detection latency is bounded by the period in both (H5).
   - The G8 probe (`ci-old-ca`) can also be fixed within the DSL, with `metadata.ownerReferences IS_EMPTY` on the CertificateRequest dependency. It is a pattern-authoring choice, not a hard limit.

## Discussion and next steps
An overall assessment after RQ1–RQ6, to guide the paper's discussion and the engine's roadmap.

**Strengths.**
- *Verified, predictable semantics.* All 10 probes behave as the semantics predict and 15/15 mutants are killed. An independent engine (Kyverno, translated 1:1) agrees on 105/105 verdicts. This matters more than P = R = 1.00 on seeded scenarios, which a reviewer may discount as expected by construction.
- *A real niche.* The platforms report 3/50 of these smells, and only as transient Events (RQ3).
- *Low operating cost.* Zero idle footprint, and about 23× fewer API requests and about 50× less CPU than Kyverno at the same 5-minute period (RQ6). The CronJob model fits periodic hygiene checks.
- *Generality.* 9 ecosystems with 0 engine changes (RQ1).

**Weaknesses.**
- *Expressiveness.* CEL resolves all 10 probes with shorter policies (RQ6a). The question "why not a policy engine?" must be answered with cost, structure and the smell-oriented output model, not with power.
- *G1 is a correctness risk.* Uncorrelated criteria give silent false negatives (CAPI `t1`, ESO `ss-g1`), not just a missing feature.
- *Scalability limits of the implementation.* The 5 QPS client limit takes about 97% of a run and caps it at about 750 Smells; matching is quadratic (RQ4). Both are cheap to fix.
- *Maturity.* The fail-open GC (fixed on this branch), the stale linter tests (D4) and the random pattern order (D6) show a prototype.
- *External validity (main threat).*
  - Synthetic scenarios on a single-node minikube.
  - The same authors wrote the Patterns, the ground truth and the Kyverno policies.
  - The only natural state is the KubeVela catalogue.

**Positioning.** KubePattern is a declarative, purpose-built analyser for relational smells across CRD ecosystems:
- its semantics are verified and cross-validated by an independent engine;
- it matches a general-purpose policy engine's effectiveness at a fraction of the operating cost;
- its expressiveness limits are catalogued (G1–G10) as a roadmap.

**Where an extended DSL can go beyond Kyverno.** Kyverno evaluates one object at a time, statelessly, with non-recursive CEL, over a list of kinds declared in advance. Three extensions exploit that. None of them works on today's engine either.

| Extension | Concrete example | Kyverno 1.19 | KubePattern with the extension |
|---|---|---|---|
| (a) Stateful duration, `for: 1h` | A ClusterIssuer created a year ago is unreferenced for 7 minutes while its only Certificate is moved between namespaces by GitOps | the scan inside the window reports `fail` and emits an Event; an age filter (`time.now()` − `creationTimestamp`) does not help because the object is old; no "unused since" | the candidate is recorded at the first run and dropped when the Certificate returns, so nothing is reported. A really unused issuer becomes a Smell after 1 h, with "unused since". The Smell's stable identity already records the first sighting |
| (b) Graph properties: `cycle` / reachability | Flux Kustomizations infra → monitoring → apps → db → infra (`dependsOn` cycle of 4): nothing ever reconciles | CEL has no recursion. A policy unrolled for cycles up to length 3 passes all 4 (miss), and each extra length needs one more nested `exists`. When a cycle is found, it gives one `fail` per member | a graph traversal in linear time, any length, on the graph the engine already holds (`owns` is already transitive); **one** Smell naming the whole cycle. The same machinery gives "transitively blocked by a suspended Kustomization" |
| (c) Discovery-based kind sets + recursive paths | Secret `pg-bootstrap` referenced only by a CNPG Cluster (`spec.bootstrap.initdb.secret.name`); CNPG installed after the rule was written | the policy must enumerate the referrer kinds and fields; the CNPG Cluster is invisible, so the Secret is reported unused (false positive, risky deletion) | dependency = every kind found by discovery at each run, with paths such as `**.secretRef.name` / `**.secret.name`; new operators are covered automatically. Heuristic on field names, and costly (one LIST per kind) |

A Crossplane ProviderConfig is **not** a good example for (c): Crossplane tracks its usage natively (`ProviderConfigUsage`, `status.users`). A dependency kind that can be *derived from the target* (e.g. a Gatekeeper ConstraintTemplate and its Constraint kind) is also within Kyverno's reach, through `resource.List` with computed strings.

**Priorities.**
1. Element-scoped criteria (G1): cheap, and it removes silent false negatives.
2. Raise client QPS/Burst and index dependencies: removes the ~750-Smell ceiling and the quadratic matching.
3. Merge the per-pattern prune (already validated).
4. Prototype `for:` (extension (a)): the differentiator most aligned with the CronJob thesis.
5. One real-world dataset (a staging cluster or the platforms' public demo repositories): the step that most increases external validity.
6. AI-assisted authoring through a natural-language `intent`: see [`../docs/proposals/intent.md`](../docs/proposals/intent.md).

## Deviations from the plan
- The first RQ5 attempt was invalidated by a host suspend. It was re-run from the verified baseline under `systemd-inhibit`, and the aborted run is kept as evidence of the timeout behaviour.
- The RQ3 oracle excludes signals that are not discriminative. For example, ESO's `StoreUnmaintained` warning appears on every fake-provider store, used or not.
- `scenarios/cert-manager/post.sh` was made idempotent (the rotated Certificate is created only once), so re-applying the scenarios does not trigger new rotations.
- Krateo and kor were not run (see `TEST-PLAN.md` §11). Kyverno was added later as RQ6 ([`COMPARISON-PLAN.md`](COMPARISON-PLAN.md)), with these deviations:
  - CEL `ValidatingPolicy` instead of the JMESPath `ClusterPolicy` sketched in the plan, because the latter is deprecated in 1.19;
  - KubePattern side measured with the fix `a814e4a`, which is the installed engine; RQ2 and mutation results are identical with both engines;
  - re-scans forced by restarting the reports controller;
  - the `apicall/` cost variant and the optional S1 scale sweep were not measured;
  - installing Kyverno and extending the audit policy required a restart of `kp-eval`; the RQ2 baseline (43/52/10) was re-checked afterwards for both tools.
- The fix validation was not in the original plan. It was added after RQ4/RQ5 exposed the fail-open GC. Only the affected scenarios were re-measured, with both engines; all other results refer to the as-is engine.

