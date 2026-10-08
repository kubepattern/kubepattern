# KubePattern evaluation plan (Section 4, CronJob mode)

This is the rewritten evaluation plan. It starts from the testing plan extracted in [`SOURCE-PLAN.md`](SOURCE-PLAN.md), but every assumption about the engine was checked against the code (`dev` @ `69d4ffd`) and every CRD field against the live API server. The results are in [`README.md`](README.md).

## 1. Thesis
KubePattern finds **orphan and unused-definition smells** across heterogeneous CRD ecosystems. The platforms themselves do not report these smells: controllers surface *dangling* references in status conditions, but never *unused* definitions.

It does so with:
- **zero engine changes**: one declarative, target-centric DSL;
- **Patterns and Smells as native CRs**;
- **periodic, self-healing analysis** in CronJob mode.

The evaluation must show this with reproducible numbers, and must also be explicit about the DSL's limits.

## 2. System under test
| Item | Value |
|---|---|
| Engine | `dev` @ `69d4ffd`, evaluated **as-is** (no code changes) |
| Image | `localhost/kubepattern:eval-69d4ffd`, built with podman from `git archive 69d4ffd` (`scripts/build-image.sh`) |
| Install | local Helm chart, namespace `kubepattern`, CronJob `suspend: true` (runs are triggered explicitly), `saveInNamespace: true` |
| RBAC | chart default (wildcard `get/list/watch`), narrowed only in RQ5 |

### 2.1 Engine semantics (verified in code)
| Aspect | Behaviour | Where |
|---|---|---|
| Emission | a Smell is written when the relationships evaluate **true**: all `matchAll`, at least one `matchAny`, no `matchNone` | `internal/analysis/resolver.go:20-58`, `engine.go:78-94` |
| Relationship entry | satisfied if **any** candidate of `with` satisfies all its criteria; zero candidates means false | `resolver.go:61-75` |
| Multiple dependencies | supported; the same `with` may appear in several entries | `resolver.go:49` |
| Custom criteria | only `EQUALS` is implemented, as a non-empty intersection of independently extracted value sets. **Not correlated per array element** | `resolver.go:121-156` |
| `owns` / `ownedBy` | transitive ownerReference UID walk over the fetched graph | `internal/cluster/graph.go:75-93` |
| Scope | dependencies are cluster-wide; namespace scoping is an explicit criterion | `filter.go:18-39` |
| Paths | dot-split, `[*]` wildcards (nested supported), no index, no key escaping | `filter.go:215-293` |
| Filters | `EQUALS` (string values only), `EXISTS`, `IS_EMPTY`, integer comparisons, `ARRAY_SIZE_*` | `filter.go:86-166` |
| Messages | placeholders for `metadata.{name,namespace,uid}`, `kind`, `apiVersion` only | `engine.go:188-202` |
| Fetch | lazy: one unpaginated LIST per distinct GVR (target, dependencies, owner kinds) | `internal/kube/client.go:198-267` |
| Missing CRD / RBAC | the pattern is skipped and logged; the run continues | `cmd/kubepattern/main.go:117-133` |
| Smell identity | `<pattern>-<targetUID>`, upsert with label `lastScan=<runUUID>` | `internal/kube/report.go:45-99` |
| Stale-Smell GC | at the end of the run every Smell whose `lastScan` differs from the current run is deleted | `report.go:101-124` |
| Client | client-go defaults (5 QPS, burst 10), 5-minute context timeout | `main.go:22`, `client.go:28` |

### 2.2 Known defects (documented, not fixed, not triggered by the scenarios)
- D1 missing `config.yaml` falls back to a zero-value config instead of `NewDefaultConfig()` (`main.go:57-60`).
- D2 a GVR is cached before its LIST, so a failed LIST hides data from later patterns (`client.go:225`).
- D3 `suppress` is reset to `false` on every update (`report.go:142`).
- D4 the linter unit tests lack `plural` and are stale.
- D5 `nodeSelector`, `affinity` and `tolerations` break the CronJob template (`nindent`).
- D6 patterns run in Go-map order (random).
- D7 the same kind read at two apiVersions shares UIDs in the graph. Mitigation: one apiVersion per kind (XRDs are read only as v2).
- D8 two overlapping runs (e.g. a manual Job and a scheduled one) garbage-collect each other's Smells, because each run has its own `lastScan` id. `concurrencyPolicy: Forbid` only serialises scheduled Jobs. Measured: only 12/46 Smells survive. Mitigation: the evaluation never overlaps runs.
- D9 (found by RQ4/RQ5) the global GC is fail-open. Smells of skipped patterns and Smells not refreshed before the deadline are deleted, so they flap above the throttling ceiling.

D8 and D9 are **fixed on branch `fix/per-pattern-prune` @ `a814e4a`**, a port of the `operator` branch's per-pattern prune. The main results in this plan use the as-is engine. The fix is validated separately by re-running only the affected scenarios with both engines (`scripts/gcfix-remeasure.sh`):
- regression: RQ2 and mutation testing;
- D8: overlapping runs;
- RQ5: timeline with per-pattern expectations;
- RQ4: S2 at S ∈ {400, 800}, cold, two warm runs and two stale runs.

D3 and D4 are **fixed by the engine update** (`feat/for` @ `54dc498`): the Smell update keeps `suppress` (and the new `since`), and the linter fixtures declare `plural`. The same update makes the client rate limits configurable (default 50/100), so the 5 QPS throttling is gone, and makes the linter reject the unimplemented primitives (`selects`, `selectedBy`, `CONTAINS`, `LABEL_SELECTOR`). Its regression and RQ8 are in `README.md` (*Engine update*, *RQ8*). The transient half of **G8** is addressed by `spec.for` (RQ8). The history-object half (`ci-old-ca`) is unchanged: it is a filter choice on the dependency.

Results are in `measurements/gcfix/` and `measurements/cronjob-fix/`.

## 3. Research questions and metrics
| RQ | Question | Metrics |
|---|---|---|
| RQ1 Expressiveness | Which relational smells across CRD ecosystems can the current DSL express, and at what cost? | verdict (expressible / partial / not) for 49 smells with the blocking limitation; Pattern LOC and primitives; engine lines changed (0) |
| RQ2 Effectiveness | Does KubePattern find the seeded smells without false alarms? | per-pattern TP/FP/FN/TN, precision and recall on positives + near-misses; **limitation probes scored separately** (prediction accuracy); mutation score; agreement with an independent oracle on the natural state |
| RQ3 Complementarity | Do the platforms already report these smells? | fraction of smelly objects with a *discriminative* unhealthy condition or Warning event |
| RQ4 Efficiency | How do run time, memory and API load scale? | wall time, peak RSS, API requests by verb/resource, per-phase timing from µs audit timestamps; object count, smell count and pattern count sweeps |
| RQ5 CronJob mode | How do Smells behave across consecutive scheduled runs? | Smell lifecycle (create / update-in-place / GC), convergence after fixes, robustness to a missing CRD and narrowed RBAC, transient findings during a rotation |
| RQ6 Comparison with Kyverno | How does KubePattern compare with a general-purpose policy engine (Kyverno) on the same smells, cluster and ground truth? | policy size and constructs vs Pattern size and primitives; P/R and probe outcomes of a 1:1 (`equivalent/`) and a `best-effort/` policy set; mutation score; idle footprint, API requests by category and report writes per cycle; detection latency after a dependency-side change at equal periods; robustness to a missing CRD and narrowed RBAC. Specified in [`COMPARISON-PLAN.md`](COMPARISON-PLAN.md) |

## 4. Environment and subject systems
One minikube profile hosts every platform, so a single KubePattern run analyses 9 ecosystems at once:
- profile `kp-eval`: kvm2 driver, docker runtime, 8 vCPU, 16 GiB, Kubernetes v1.34.4;
- API-server audit log restricted to the KubePattern ServiceAccount (`env/audit-policy.yaml`).

Each install script in `env/install/` uses only Helm or release manifests (no vendor CLIs).

| Platform | Version | Install | What the scenario contains |
|---|---|---|---|
| cert-manager | v1.21.2 | Helm | 9 ClusterIssuers, Certificates (one rotated), a hand-made CertificateRequest |
| CloudNativePG | 1.30.1 | manifest | 1 live PostgreSQL cluster + 6 object-only Clusters, ScheduledBackups, Poolers |
| External Secrets | chart 2.11.0 | Helm | fake-provider SecretStores/ClusterSecretStores, ExternalSecrets (top-level and per-entry refs), PushSecrets, ClusterExternalSecret |
| Flux | v2.9.5 | install.yaml | real OCI sources (podinfo) consumed by suspended HelmReleases/Kustomizations |
| Argo CD | chart 10.9.2 | Helm | AppProjects, Applications, ApplicationSets (list, cluster and git generators) |
| Kargo | 1.11.4 | Helm (OCI) | 2 Projects, Warehouses (manual Freight, 24h discovery), Stages |
| Crossplane | 2.4.2 (`--enable-operations`) | Helm | 7 Functions (one Pod each), XRDs of the three scopes, Compositions, Cron/WatchOperations |
| KubeVela | 1.11.0 | Helm | team definitions in namespaces, Applications (custom and built-in types); 45 built-in definitions |
| Cluster API + CAPD | v1.14.2 | release manifests | paused Cluster, KubeadmControlPlane, MachineDeployments, MachineSet, ClusterClass, Docker/Dev machine templates |
| Kyverno (RQ6 baseline only) | v1.19.1 (chart 3.9.1) | Helm (`env/install/95-kyverno.sh`) | 14 `ValidatingPolicy` (CEL, `policies.kyverno.io/v1`) + 21 `GlobalContextEntry` in `comparison/kyverno/`; Audit only, background scan |

## 5. Patterns (14, in `patterns/`)
| # | Pattern | Relationships | Why it matters |
|---|---|---|---|
| 1a | `crossplane-function-not-used` | 4 × `matchNone` custom (Composition + Operation/Cron/Watch pipelines, nested `[*]`) | each Function is a running Pod and a supply-chain surface |
| 1b | `crossplane-composition-without-xrd` | `matchNone` custom | the Composition can never be selected |
| 2a/2b | `kubevela-{component,trait}definition-not-used` | `matchNone` custom, nested `[*]`, namespace filter | stale CUE templates in the catalogue |
| 3 | `kargo-warehouse-not-requested` | `matchNone` custom + namespace | the Warehouse keeps polling registries |
| 4 | `capi-machinetemplate-orphan` | 5 × `matchNone` over MD, MS, KCP and ClusterClass (2 entries) | leftovers of template rotations |
| 5 | `flux-ocirepository-not-used` | 2 × `matchNone`, name + kind + namespace | source-controller pulls artefacts nobody uses |
| 6 | `certmanager-clusterissuer-not-used` | 2 × `matchNone`, dependency filters on `issuerRef.kind` | signing material with no consumer |
| 7a | `cnpg-cluster-without-scheduledbackup` | `matchNone` + namespace | database without backup schedule |
| 7b | `cnpg-pooler-name-clashes-with-cluster` | **`matchAll`** name + namespace | violates the Pooler API contract |
| 8a | `argocd-applicationset-generates-nothing` | **`owns`** + `matchNone` | zombie generator |
| 8b | `argocd-appproject-empty` | `matchNone` + target filter | permissions granted to nobody |
| 9a | `eso-secretstore-not-used` | 4 × `matchNone` (top-level, `data[*]`, `dataFrom[*]`, PushSecret) | external credentials with no consumer |
| 9b | `eso-clustersecretstore-not-used` | 3 × `matchNone` (ExternalSecret, per-entry, ClusterExternalSecret) | cluster-wide credentials with no consumer |

There are also two catalogue variants (`patterns/kubevela/natural/`) for the natural-state experiment, and one robustness pattern (`patterns/_robustness/`, target CRD not installed).

## 6. Seeding design (ground truth by construction)
Every scenario (`scenarios/<platform>/`) has three parts:
- `base.yaml`: the platform in normal use;
- `seeds.yaml`: the seeded cases;
- optional `pre.sh`/`post.sh`: steps that need the controllers, such as a certificate rotation or a pinned definition revision.

`ground-truth.csv` labels each object twice:
- `truth`: what a correct analysis must report (`smell` | `clean`);
- `engine_expected`: what the verified semantics predict. It differs from `truth` only for probes.

| Class | Meaning | Scoring |
|---|---|---|
| `positive` | a real smell (never used; used only in another namespace; same name but different kind; reference removed) | in scope |
| `negative` | the platform in normal use | in scope |
| `near-miss` | a tricky negative (second array element, second trait of the second component, only an Operation, only a PushSecret, cross-kind same name…) | in scope |
| `probe-G*` | a case where a documented DSL limitation predicts a FN or FP | separate: prediction accuracy + limitation FN/FP |
| `blocked-upstream` | the misconfiguration cannot be created (admission webhook) | reported only |

Totals: 43 positives, 52 negatives/near-misses, 10 probes, 1 blocked-upstream.

**Mutation seeding** (`scenarios/mutations.csv`, `scripts/mutate.sh`). One operator per pattern (14 mutants plus one side effect), using four operators:
- *delete-referrer*;
- *rename-reference*;
- *delete-referenced*;
- *introduce-clash*.

Protocol: baseline run → apply all mutants → run → revert → run. A mutant is **killed** if exactly its target gains a Smell; it is **restored** if the Smell disappears after the revert.

## 7. Oracles
| Evidence | Oracle |
|---|---|
| seeded scenarios | ground truth by construction (`ground-truth.csv`) |
| natural state (KubeVela built-in catalogue) | an independent `jq` computation over the live objects (`scripts/natural-findings.sh`) |
| complementarity | platform status conditions and Warning Events. A signal counts only if it is **discriminative**: it must not also appear on clean objects of the same pattern (`scripts/collect_oracles.py`) |
| field paths | `kubectl explain` on every path used by the Patterns (`scripts/verify_fields.py`) |

## 8. Protocols
- **RQ1:** classify the source note's long list against the verified semantics (`measurements/expressiveness/expressiveness.csv`), and measure Pattern size and primitives (`scripts/pattern_metrics.py`).
- **RQ2:**
  1. `apply-scenario.sh` then `apply-patterns.sh`;
  2. 3 repetitions of `run-once.sh`: run 1 exercises the CREATE path, runs 2–3 the UPDATE path;
  3. `score.py` exits non-zero on any in-scope error, failed probe prediction or unlisted Smell.
  4. Then run `mutate.sh`, and `natural-findings.sh` for the KubeVela catalogue.
- **RQ3:** `collect_oracles.py` over every smelly object (positives and probes).
- **RQ4:**
  - Per-run audit summary (`audit_summary.py`), with a phase breakdown and the median gap between requests (a throttling signature).
  - **Scale sweep (`scale.py`):**
    - Flux objects only, all `suspend: true`, with the Flux controllers scaled to 0 (no webhooks, no churn); only the Flux pattern (and its clones) is installed.
    - S2 Smells: 1,000 sources, S ∈ {0, 100, 200, 400, 800} orphans. For each S there is a cold run (Smells deleted first: CREATE path) and a warm run (UPDATE path); after S=800 all orphans are fixed and a stale run measures the DELETE path. S=800 is chosen to exceed the ceiling predicted by 5 QPS × the 5-minute timeout.
    - S3 patterns: K ∈ {1, 5, 10, 20} copies of the pattern over the same kinds, 5 repetitions each.
    - S1 objects: 100 orphans, T ∈ {1,000; 2,500; 5,000; 10,000} sources plus T−100 HelmReleases (N ≈ 2T objects), 5 repetitions each.
    - `scale.py cleanup` restores the evaluation state afterwards.
  - **Out-of-cluster measurements (`perf-local.sh`):** the binary extracted from the image, run with `/usr/bin/time -v` for wall time, peak RSS and CPU, authenticated as the SA with a token so the audit filter still applies.
- **RQ5 (`cronjob-timeline.sh`):** CronJob at `*/2` for 9 runs with scripted events between runs:
  1. fix 4 smells;
  2. inject 3 new ones;
  3. add a pattern whose CRD is missing;
  4. narrow RBAC (no `kargo.akuity.io`);
  5. restore RBAC;
  6. template rotation step 1;
  7. template rotation step 2;
  8. revert.

  After every Job, snapshot the Smells (name, UID, creationTimestamp) and score the timeline against the expected deltas.

## 9. DSL limitation catalogue (probes)
| Id | Limitation | Probe(s) in the scenarios | Predicted |
|---|---|---|---|
| G1 | criteria not correlated per array element | CAPI `t1`; ESO `ss-g1`, `css-g1b` | FN |
| G2 | no per-element universal quantification | (RQ1 only: dangling list references) | not expressible |
| G3 | implicit or overridden namespace/kind defaults | Flux `cross-ns` (explicit `chartRef.namespace`) | FP |
| G4 | string-encoded references (group in apiVersion, `name@v1`) | Crossplane `app-foreign-group`; cert-manager `ci-foreign-group`; KubeVela `kp-versioned` | FN, FN, FP |
| G5 | no key escaping (labels/annotations with dots) | (RQ1: Strimzi, Dapr) | not expressible |
| G6 | no kind wildcards/categories | (RQ1: Crossplane MRs, Kratix, Gatekeeper) | not expressible |
| G7 | selectors not implemented | ESO `ss-selected` (PushSecret label selector) | FP |
| G8 | snapshot hygiene (history objects, transients) | cert-manager `ci-old-ca` (retained CertificateRequest); RQ5 rotation | FN; transient |
| G9 | template / remote content | (RQ1: Tekton resolvers, CUE) | out of scope |
| G10 | *new*: filters compare strings only (no booleans) | CNPG `db-suspended` (`spec.suspend: true`) | FN |

## 10. Threats to validity
- **Construct:**
  - Seeded smells are biased toward detectable forms. Mitigations: near-misses, probes, mutation seeding, and a natural-state experiment with an independent oracle.
  - "Unused" is defined per pattern, e.g. a suspended referrer still counts as a use.
- **Internal:**
  - The known defects D1–D7 are avoided by the scenarios: every CRD exists and one apiVersion is used per kind.
  - GC also deletes the Smells of skipped patterns (observed and reported in RQ5; fixed by the per-pattern prune, see D9).
  - The random pattern order does not affect results, because all GVRs succeed.
- **External:**
  - Single-node minikube.
  - Versions are pinned (`env/versions.env`).
  - Objects are realistic but synthetic; production clusters would have more referrer kinds (e.g. ingress-shim Certificates, Operations).
- **Conclusion:**
  - RQ2 repetitions are deterministic.
  - RQ4 reports medians and IQR over 5 runs per cell.

## 11. Changes with respect to the source plan
| Source plan | This plan | Reason |
|---|---|---|
| stale-Smell cleanup "unverified" | measured as a feature (RQ5) | the code implements GC via `lastScan` |
| multiple dependencies under `matchNone` "unverified" | used by 6 of 14 patterns | verified in `resolver.go` |
| messages such as `{{target.spec.compositeTypeRef.kind}}` | metadata-only placeholders | only five placeholders exist |
| Pattern examples without a namespace criterion | explicit `metadata.namespace` criteria where needed | dependencies are cluster-wide |
| Krateo as S1 | referenced only | existing experiments; the correlation issue (G1) is reproduced on CAPI and ESO instead |
| kind cluster | minikube (kvm2) | available environment; one profile hosts all 9 platforms |
| CAPD live workload cluster | paused Cluster (object graph only) | the patterns need references, not machines |
| oracles: two raters + Cohen's κ | ground truth by construction + independent jq oracle + discriminative platform signals | deterministic and reproducible; κ is not needed for seeded data |
| kor / Kyverno baselines | kor out of scope; Kyverno run as RQ6 | standard kinds are not the claim; Kyverno was added later as a full comparison ([`COMPARISON-PLAN.md`](COMPARISON-PLAN.md)) |
| — | new limitation G10 and the throttling finding (RQ4) | discovered during verification |
