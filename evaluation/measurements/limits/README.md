# Engine update: the DSL limitations G1–G10

The engine of branch `ccr-02fe82f9-6bq28v` (from `dev` @ `a848c98`) extends the Pattern DSL to resolve the limitation catalogue of the evaluation (`../../TEST-PLAN.md` §9). The constructs are documented in [`../../../docs/dsl.md`](../../../docs/dsl.md). Every number below was measured on the **offline replay** ([`../../replay/`](../../replay/README.md)): a real `kube-apiserver` v1.34.4 with the pinned CRDs of the nine platforms and no controllers. minikube was not available in the environment of this update, so the replay was checked first against the original results (row "baseline" below). `summary.txt` is the full log of `replay/regression.sh`; per-run scores and rows are in the sub-folders.

## What changed in the engine
| Limitation | Construct | Code |
|---|---|---|
| G1 correlation per element | `[@]` element anchors on the dependency side | `internal/fieldpath`, `resolver.go` |
| G2 "some reference resolves to nothing" | `[@]` on the target side: target filters and relationships share the binding; defective if one element is | `engine.go` (`SelectTargets`) |
| G3 implicit defaults | `targetDefault` / `dependencyDefault` (`{path}` or `{value}`) | `resolver.go` |
| G4 string-encoded references | `targetTransform` / `dependencyTransform`: `apiGroup`, `split`, `regex`, `lowercase` | `linter.ApplyTransform` |
| G5 key escaping | quoted keys `labels['krateo.io/managed-by']` | `internal/fieldpath` |
| G6 kind wildcards, categories | `kind: "*"`, `category`, `kinds`, dependency `fromTarget` | `kube.ExpandRefs`, `main.go` (`fetchForPattern`) |
| G7 selectors | `selects` / `selectedBy` with `selectorPath` and optional criteria | `resolver.go` (`selectorMatches`) |
| G8 history objects | already expressible with a filter (`metadata.ownerReferences IS_EMPTY`); transients by `spec.for` | – |
| G10 typed filters | `EQUALS` compares booleans and numbers by their string form; `valuesFrom` | `filter.go` |
| (support) | literal criterion sides `targetValue` / `dependencyValue` | `resolver.go` |

G9 (content inside templates and remote references) stays out of scope. The linter validates every new field, the path syntax and the anchor rules. Tests: `internal/fieldpath/fieldpath_test.go`, `internal/analysis/limitations_test.go` (one test per limitation, reproducing its probe), `internal/linter/limits_test.go`.

## Results
**Backward compatibility.** With the original Patterns, the new engine writes exactly the same Smells as the baseline engine, on both suites. Patterns that do not use the new constructs are unaffected.

| Suite (replay) | Engine, Patterns | Smells | TP / FP / FN / TN | P / R | Probes | Mutation |
|---|---|---|---|---|---|---|
| RQ2, 9 platforms | baseline `a848c98`, original | 46 | 43 / 0 / 0 / 52 | 1.00 / 1.00 | 10/10 as predicted (7 FN, 3 FP) | 15/15, reverted = baseline |
| RQ2, 9 platforms | new, original | 46 (identical) | 43 / 0 / 0 / 52 | 1.00 / 1.00 | 10/10 as predicted | 15/15, reverted = baseline |
| RQ2, 9 platforms | new, **v2** (`../../patterns-v2/`) | 50 | 43 / 0 / 0 / 52 | 1.00 / 1.00 | **10/10 resolved** (verdict = truth) | 15/15, reverted = baseline |
| Krateo controlled cases | baseline, given | 39 | 29 / 0 / 0 / 39 | 1.00 / 1.00 | 14/14 as predicted | – |
| Krateo controlled cases | new, given | 39 (identical) | 29 / 0 / 0 / 39 | 1.00 / 1.00 | 14/14 as predicted | – |
| Krateo controlled cases | new, **v2** (`../../krateo/patterns-v2/`) | 45 | 30 / 0 / 0 / 40 | 1.00 / 1.00 | **11/11 limitation probes resolved**; 3 scope probes kept | – |

- The baseline rows equal the minikube results (RQ2: `../effectiveness/`, `../mutation/`; Krateo: `../../krateo/results/` restricted to the seeded rows), which validates the replay.
- The 4 extra Smells of RQ2 v2 are the probe balance: the 7 limitation false negatives are now found and the 3 limitation false positives are gone (46 + 7 − 3 = 50).
- Krateo v2 includes `krateo-restaction-manager-missing` (G5), the smell that only Kyverno could express before (`../../krateo/scenario/ground-truth-kyverno-only.csv`, seeded rows: 1 positive, 1 negative).
- The **scope probes** (`page-route`, `panel-kind`, `panel-links`) are gaps of the given Patterns' scope (Routes not considered, Panels matched by name only), not of the engine. They are kept as given, exactly as the Kyverno best-effort set did, so the two remain comparable.

### Probe by probe
| Class | Pattern | Object | Truth | Original Pattern | v2 Pattern |
|---|---|---|---|---|---|
| G1 | `capi-machinetemplate-orphan` | t1 | smell | clean | **smell** |
| G1 | `eso-secretstore-not-used` | ss-g1 | smell | clean | **smell** |
| G1 | `eso-clustersecretstore-not-used` | css-g1b | smell | clean | **smell** |
| G3 | `flux-ocirepository-not-used` | cross-ns | clean | smell | **clean** |
| G4 | `certmanager-clusterissuer-not-used` | ci-foreign-group | smell | clean | **smell** |
| G4 | `crossplane-composition-without-xrd` | app-foreign-group | smell | clean | **smell** |
| G4 | `kubevela-componentdefinition-not-used` | kp-versioned | clean | smell | **clean** |
| G7 | `eso-secretstore-not-used` | ss-selected | clean | smell | **clean** |
| G8 | `certmanager-clusterissuer-not-used` | ci-old-ca | smell | clean | **smell** |
| G10 | `cnpg-cluster-without-scheduledbackup` | db-suspended | smell | clean | **smell** |
| G1 | `page-not-referenced` (Krateo) | page-g1 | smell | clean | **smell** |
| G1 | `paragraph-not-referenced` (Krateo) | para-g1 | smell | clean | **smell** |
| G1 | `krateo-navmenuitem-page-missing` | nav-two | smell | clean | **smell** |
| G1 | `krateo-restaction-endpoint-missing` | ra-ep-g1 | smell | clean | **smell** |
| G2 | `krateo-restaction-endpoint-missing` | ra-ep-mixed | smell | clean | **smell** |
| G1 | `krateo-rolebinding-subject-sa-missing` | rb-sa-g1 | smell | clean | **smell** |
| G2 | `krateo-rolebinding-subject-sa-missing` | rb-sa-mixed | smell | clean | **smell** |
| G4 | `krateo-restaction-not-used` | ra-called | clean | smell | **clean** |
| G6 | `krateo-compositiondefinition-unused` | portal-composition-page-generic | clean | smell | **clean** |
| G6 | `krateo-widget-manager-missing` | yv-managed-missing | smell | clean | **smell** |
| G7 | `panel-not-referenced` (Krateo) | portalcompositionpagegeneric-…-composition-panel | clean | smell | **clean** |

Run-time features exercised on the API server, not only in unit tests: discovery of a category (`fluxcd-appliers` in the Flux v2 Pattern), of a group version (`kind: "*"` over the Krateo widgets in `krateo-restaction-not-used`), and kinds derived from the targets (`fromTarget` in `krateo-compositiondefinition-unused`).

### RQ1 re-classified (analytic)
`expressiveness.csv` re-classifies the 49 relational smells of RQ1 with the new constructs. Four rows are backed by a measurement (replay or unit test), the others are an analytic reading of the semantics, like the original RQ1.

| Verdict | Before | After |
|---|---|---|
| expressible | 27 | 44 |
| partial | 10 | 5 |
| not expressible | 12 | 0 |

The five partial smells: Kratix (the generated request kinds must be named in the Promise status, to verify), Argo Workflows (G8: Workflows are transient, history needed), Tekton (G9: remote resolvers and bundles), Istio (a short host needs a conditional default to become an FQDN), Sealed Secrets (unchanged, not blocked by a catalogued limitation).

## Not re-measured
Cost and footprint (RQ4 audit, RQ6c), the CronJob timeline (RQ5), RQ8, the Kyverno comparison, the scale sweep and the natural state of the Krateo case study need minikube. The engine changes add no API request for Patterns without `kind: "*"`, `category` or `fromTarget` (same fetch path), and the replay run times are unchanged (0.85 s for the 14 original Patterns with both engines).

**Matching cost.** RQ4 found the matching quadratic (targets × dependencies). The new engine extracts the target side of each criterion once per target instead of once per candidate, and enumerates elements only for relationships that use `[@]`. On the same workload (`internal/analysis/bench_test.go`: 1,500 targets × 1,500 dependencies, three criteria, 4 CPUs) it is faster than the baseline engine:

| Engine | Time per run | Allocations |
|---|---|---|
| baseline `a848c98` | 566 ms | 7.4 M |
| new | 189–208 ms | 2.5 M |

The complexity is still quadratic: indexing the dependencies remains the next scalability step. To re-run on minikube: build the image of this branch (`KP_COMMIT=<commit> ./scripts/build-image.sh all`) and run `./scripts/regression.sh <commit>`; for the v2 Patterns, run `./scripts/apply-patterns.sh`, then `kubectl apply -R -f patterns-v2/` (same names, so they replace the originals), and score with `score.py --probe-expect truth`.
