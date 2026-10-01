# Positioning KubePattern

How to position KubePattern in the paper, using only the evidence of RQ1–RQ6 and RQ8 ([`README.md`](README.md)) and the code (`dev` @ `69d4ffd`, fix `a814e4a`, engine update `54dc498`). For each claim, this file gives what supports it and where the numbers come from. Claims that the evidence does not support are listed separately, with the wording to use instead.

## Thesis
> On relational smells across CRD ecosystems, KubePattern matches the detection of a general-purpose policy engine, as verified by that independent engine. It does so at a fraction of the operating cost, and its output is made of findings rather than per-resource reports. Because those findings are stateful, a minimum duration (`spec.for`) removes the snapshot false positives that a stateless background scan reports (RQ8). It is a purpose-built periodic analyser that complements admission control. It does not replace a policy engine.

The question "why not a policy engine?" must be answered with:
- the problem class;
- the operating cost;
- the output model, including the stateful duration (RQ8);
- the verified semantics.

It cannot be answered with expressive power or readability. On both, Kyverno is at least equal (§2). **Update (limitations update, §1.8):** on the catalogued limitations the DSL now reaches parity with the best-effort CEL policies (every probe resolved) and goes beyond them on kind sets resolved by discovery; CEL stays the more general language. Parity on the catalogue is a claim the evidence supports; superiority in power is not.

## 1. Claims supported by the evidence

### 1.1 A niche the platforms leave uncovered
| Evidence | Value | Source |
|---|---|---|
| Smelly objects reported by the platforms, right after seeding | **3/50**, only as Warning Events (CNPG `NameClash`) | RQ3, `results/oracles/oracles-fresh.csv` |
| Same, after the 1 h Event TTL | **0/50** | RQ3, `results/oracles/oracles-after-event-ttl.csv` |
| Unused definitions shown as healthy | Functions, Warehouses, ClusterIssuers and stores are `Ready`/`Healthy`. ApplicationSets that generate nothing report `ErrorOccurred=False` | RQ3 |
| Natural state | 40/45 built-in KubeVela definitions are unused, with 45/45 agreement with an independent `jq` oracle | RQ2, `results/natural/kubevela.csv` |

### 1.2 A different problem class: periodic hygiene, not admission
- An orphan is a property of the cluster state, not of an admission request. A ClusterIssuer created before its Certificates is legitimate when it is admitted, so admission control cannot judge it.
- Kyverno covers these smells through its background scan, which is an add-on to an admission engine. KubePattern is built for this task: it runs as a CronJob and is not in the admission path.
- **Latency is not an argument either way** (RQ6d, H5):
  - detection latency is bounded by the period for both tools;
  - at a 5-minute period, the median time for a smell to appear is 149 s for KubePattern and 106 s for Kyverno, and the maximum is about one period for both (1.07 vs 0.97 periods);
  - neither tool reacts to dependency-side changes.
- Recommended framing: **complementary**. Kyverno checks per-object rules at admission, and KubePattern periodically checks the relations between resources.

### 1.3 Operating cost
The numbers below compare both tools at the same 5-minute period (`results/comparison/cost.csv`, RQ6c). The KubePattern side is the engine with the per-pattern prune (`a814e4a`).

| Per hour | KubePattern (Job) | Kyverno reports controller | Kyverno, default install (4 controllers) |
|---|---|---|---|
| Memory when idle | **0** (31 MB peak for about 26 s per run) | 92 MiB | 477 MiB |
| CPU | 4.8 s | 186 s (**about 39×**) | 248 s (about 52×) |
| API requests | 1,716 (143 per run) | 15,447 (1,138 per cycle, **about 8× per cycle**) | 39,155 (about 23×) |
| In the API request path | no | – | yes: admission webhooks, about 750 webhook-configuration updates/h |

- **Use the reports-controller column in the paper** (about 8× the requests per cycle, about 39× the CPU). It is the fair comparison of the function under test. The whole-install figures include the cleanup and admission controllers, which these policies do not need, and a reviewer will attack them.
- Informers remove Kyverno's per-cycle LISTs of the analysed kinds. The extra traffic comes from the reporting pipeline, which alone issues about 8× the requests of a KubePattern run:
  - EphemeralReports are created and then aggregated into PolicyReports;
  - one PolicyViolation Event is emitted per failure.
- Caveats:
  - Everything was measured on 215 analysed objects. The Kyverno S1 sweep was not run, so the relative cost at 10k+ objects (informer memory vs unpaginated LISTs) is still open.
  - client-go's 5 QPS default capped KubePattern at about 750 Smells per run (RQ4). **Fixed by the engine update** (`54dc498`, 50/100 QPS): 805 Smells in 30.5 s, the whole cluster in 0.85 s, with the same 143 requests. The quadratic matching remains: the run is CPU-bound at about 20k objects. **Index the dependencies before claiming large clusters.**
  - The cost table was measured with `a814e4a`. With `54dc498` a run lasts about 1 s instead of about 26 s and uses about 0.2 s of CPU instead of about 0.3 s, with identical requests and writes (`results/regression/54dc498/`). The comparison with Kyverno was **not re-measured**: use the table above in the paper, and mention the shorter run only qualitatively (a shorter window in which the Job exists and holds credentials).

### 1.4 Finding-oriented output
| | Kyverno 1.19 | KubePattern |
|---|---|---|
| What is written | one report per evaluated resource, with pass/fail/error/skip per policy | one Smell per finding, and nothing for clean resources |
| On the RQ2 scenario | 105 reports (74 `PolicyReport` in 13 namespaces, 31 `ClusterPolicyReport`) holding 46 `fail` and 59 `pass` | 46 Smells |
| Writes per cycle | about 349 report writes and about 92 Events | 46 Smell updates |
| Identity across runs | one report per resource | `<pattern>-<targetUID>`: 40 of 51 identities were a single object across all 9 RQ5 runs; `spec.since` keeps the first observation (engine update) |
| History of a finding | none: a report records when the resource was evaluated | `spec.since` ("unused since") and `spec.phase` (Pending/Active) |
| Metadata | `severity` and `category` are optional (not set by these policies); `messageExpression` is full CEL | severity, category, message and reference; only 5 metadata placeholders |
| Resource a relational smell is attributed to | the resource the policy matches | the target, by construction of the DSL |

- `kubectl get smells -A` lists the problems to fix. With Kyverno, the failures must be filtered out of the reports.
- Self-healing: a fixed smell disappears at the next run (RQ5).
- **Some of these claims depend on the fix.** "The Smells of a skipped pattern are kept" and "no flapping" hold only with the per-pattern prune (`a814e4a`), which is now **merged into `dev`**. Release it before submission, so that the released tool matches the paper.
- **Suppression works now.** Before the engine update, every run reset `spec.suppress` to false (D3), so a suppressed false positive came back at the next run. `54dc498` keeps it, which makes "suppress a finding" a real feature of the output model.
- Fairness: Kyverno's message model is richer (full CEL vs 5 placeholders).

Where the report numbers come from: `results/raw/kyverno/equivalent-r1/*/reports.json`, in the git-ignored archive. Count them with `jq '[.items[] | {kind, ns: .metadata.namespace}]'`.

### 1.5 Small, verified semantics
| Evidence | Value | Source |
|---|---|---|
| Effectiveness on seeded smells | P = R = 1.00 on 43 positives and 52 negatives (29 near-misses), over 3 runs | RQ2 |
| Limitation probes | 10/10 behave as the semantics predict (7 FN, 3 FP) | RQ2 |
| Mutation testing | 15/15 mutants detected and restored | RQ2 |
| Independent cross-validation | the Kyverno 1:1 translation gives the same verdict on 105/105 objects, probes included; mutation 15/15 | RQ6b |
| Limitations | catalogued as G1–G10, each with a probe or an RQ1 case | [`TEST-PLAN.md`](TEST-PLAN.md) §9 |

This is the core of the "solidity" claim. The DSL can only do what [`TEST-PLAN.md`](TEST-PLAN.md) §2.1 lists, so its behaviour is predictable, and so are its failures. This matters more than P = R = 1.00 on seeded data, which a reviewer may discount as expected by construction.

### 1.6 Generality
- 14 Patterns cover 9 ecosystems and 33 GVRs with **0 engine changes** (RQ1).
- 13 of the 14 Patterns use `custom` and 1 uses `owns`. On CRD platforms, resources reference each other through plain spec fields, and naming those fields is what makes unknown kinds analysable.
- `owns` is transitive over the fetched graph. CEL has no recursion, so the Kyverno translation checks direct ownership only (RQ6a). The two are equivalent on this scenario, because ApplicationSets own their Applications directly.

### 1.7 A stateful duration that a stateless scan cannot replicate (RQ8)
| Evidence | Value | Source |
|---|---|---|
| Transient findings with `spec.for` | **0**, against 2 (kp-eval: CAPI rotation, GitOps delete-and-recreate) and 3 (Krateo: widgets wired during development) without it | `results/rq8/` |
| Persistent smells with `spec.for` | all reported: 43/43 seeded, plus the new orphans | `results/rq8/eval/scores.csv` |
| Latency cost | exactly `for` rounded up to the next run (2 periods in both settings); disappear latency unchanged | `results/rq8/` |
| Kyverno at the same moment | `fail` on every transient, plus a PolicyViolation Event | `results/rq8/*/kyverno.csv` |
| Request cost | none: a Pending Smell rides the same writes | engine design, `results/rq8/README.md` |

- **Why Kyverno cannot replicate it.** The background scan re-evaluates each resource from scratch and keeps no "failing since". A CEL age filter on `creationTimestamp` hides new objects only. In the GitOps case the object is old and only its referrer is new.
- **How to word it:** "a minimum duration trades a declared detection latency for the removal of snapshot false positives."
- **Not:** "KubePattern has no false positives".

### 1.8 Limitations resolved within the declarative DSL (engine update)
| Evidence | Value | Source |
|---|---|---|
| Constructs added | `[@]` element anchors (G1, G2), defaults (G3), transforms (G4), quoted keys (G5), `kind: "*"`, `category`, `kinds`, `fromTarget` (G6), `selects`/`selectedBy` (G7), typed `EQUALS` and `valuesFrom` (G10) | [`../docs/dsl.md`](../docs/dsl.md) |
| Backward compatibility | identical Smells with the original Patterns (46/46 RQ2, 39/39 Krateo controlled cases); mutation 15/15 | `results/limits/` |
| Probes, v2 Patterns | 10/10 RQ2 and 11/11 Krateo limitation probes yield the true verdict; P = R = 1.00 | `results/limits/` |
| Kyverno-only smell | `krateo-restaction-manager-missing` now has a Pattern (G5) | `krateo/patterns-v2/` |
| Beyond the Kyverno translation | a kind set resolved by discovery at each run (`kind: "*"` over the Krateo widgets, `category: fluxcd-appliers`); Kyverno needs one GlobalContextEntry per kind (24 for the widgets) | `results/limits/`, `krateo/README.md` |
| RQ1, re-classified | 44 expressible, 5 partial, 0 not (was 27/10/12); 4 rows measured, the rest analytic | `results/limits/expressiveness.csv` |

- **Caveat: measured on the offline replay** (real API server and CRDs, no controllers), whose baseline reproduces the minikube results exactly. Confirm on `kp-eval` and `kp-krateo` before quoting it as a minikube result.
- **How to word it:** "each catalogued limitation is resolved by a construct of the declarative structure, with no expression language; the original Patterns keep their verdicts".
- **Not:** "the DSL is as expressive as CEL". A conditional default (Istio short hosts) or a join between two dependencies of arbitrary shape still need CEL.

## 2. Claims to avoid
| Claim | Why not | Say instead |
|---|---|---|
| "The DSL is more readable or simpler than CEL" | Readability was not measured (no user study). By size, CEL wins: 417 vs 657 lines (249 vs 475 logic lines), with 59 vs 58 atomic comparisons. H1 is not confirmed (RQ6a) | "A pattern contains no expressions: every check is a (path, operator, values) triple within a fixed structure of target, dependencies and relationships. The Kyverno translations are shorter, but they move the logic into 5,129 characters of CEL, with 43 `exists`/`all` comprehensions, 67 optional hops and `dyn()` casts." |
| "Policy engines cannot express relational smells" or "existing analysers are blind to relational smells" | CEL expresses all 14 smells, and the best-effort variants resolve 10/10 probes (RQ6a) | "Policy engines express them through explicit lookups, evaluated by always-on controllers." |
| "Policy languages have a learning curve that suits developers rather than architects and operations staff" | there is no user study | Drop it, or state it as a design goal. |
| "Patterns are validated, so they cannot silently produce wrong findings" | The linter checks structure only. A mistyped path yields an empty value set, so inside `matchNone` every target becomes a finding. (Since the engine update, the linter at least rejects the unimplemented `selects`, `selectedBy`, `CONTAINS` and `LABEL_SELECTOR`, which used to evaluate to false silently.) | "A structurally malformed pattern is logged and skipped without affecting the others." Note that a mistyped path fails the same way as CEL optional chaining. |
| "Schema-checkable", as a current feature | The linter does not check paths against the CRD schemas. The evaluation checked them externally (`scripts/verify_fields.py`, 41/41) | Present it as a potential advantage of declarative paths, until it is implemented (§4). |
| "Findings are visible only to the users entitled to see them (RBAC)", as a differentiator | Kyverno also writes namespaced reports, with the same RBAC granularity (§3) | Keep it as a design note, not a contribution. |
| "Detects smells faster than Kyverno" | both tools are bounded by the period (RQ6d) | "Same detection latency, bounded by the period." |
| "Scales to large clusters" | the 5 QPS ceiling is fixed (805 Smells in 30.5 s), but matching is quadratic (CPU-bound at about 20k objects) and LISTs are unpaginated; the S1 sweep was not run for Kyverno | "No throttling ceiling: 805 Smells in 30.5 s; run time is CPU-bound above about 20k objects (quadratic matching, future work)." |
| "Checkers evaluate one resource at a time" | kube-score has a few hard-coded cross-resource checks (e.g. a Service that selects no Pod), and kor and Popeye report unused native resources (to verify and cite) | "Where they consider relations, these are hard-coded for native kinds." |

## 3. RBAC
**Output side.** RBAC is not a differentiator here.

| | Kyverno 1.19 | KubePattern |
|---|---|---|
| Where findings live | `PolicyReport` in the resource's namespace (one per resource, named by its UID); `ClusterPolicyReport` for cluster-scoped resources | `Smell` in the target's namespace (`saveInNamespace: true`, the chart default); for cluster-scoped targets, in the configured namespace (by default, the release namespace) |
| RBAC granularity | resource type × namespace | resource type × namespace |
| Visibility per policy or per pattern | no: RBAC does not filter by label | no: same reason; Smell names contain the target UID, so `resourceNames` is impractical |

There is one small difference. In Kyverno's design, a PolicyReport aggregates every policy that matches a resource, so security and hygiene results share the same object. (In this scenario each resource is matched by one policy only.) Smells are a separate resource type, so `get smells` can be granted without `get policyreports`. A reviewer would still call it "just another CRD".

**Analyser side.**
- Both tools need cluster-wide read access on the analysed API groups. The Kyverno comparison used an aggregated ClusterRole over the same 15 API groups as the Patterns (`comparison/kyverno/rbac.yaml`).
- KubePattern holds its credentials only while the Job runs, whereas Kyverno's controllers hold them permanently. This difference is real but only qualitative.

**Failure visibility** (`results/comparison/robustness.csv`).

| Case | Kyverno 1.19 | KubePattern (fix) |
|---|---|---|
| R1a: target CRD missing | policy accepted; `RBACPermissionsGranted=False` with a misleading "missing permissions" message; no results | pattern skipped and logged |
| R1b: dependency CRD missing | policy Ready; every target reported as `error` (9/9) | pattern skipped and logged |
| R2: read access to the target lost | silently stale: last results kept, policy still Ready, the error only in the controller log | silently stale: pattern skipped, Smells kept, the skip only in the Job log |

Kyverno is more visible on a missing dependency. Both tools go silently stale when read access is lost.

**How RBAC could become a differentiator** (not implemented):
- **Pattern status conditions.** Write a `Skipped` condition on the Pattern CR, with reason `Forbidden` or `CRDNotFound` and the time of the last successful evaluation (the CRD has no status today). Lost access would then show up in `kubectl get patterns`, which Kyverno does not offer in R2. It is cheap to build and measurable with the existing R1/R2 scripts.
- **Per-pattern output namespace.** Route Smells by audience, for example security smells to a restricted namespace. Plain namespace RBAC would then give per-pattern visibility.

## 4. Differentiators to build
These items would turn the positioning into evidence. They are ordered by their impact on the "serious alternative" claim.

| Item | What it gives | Kyverno 1.19 | Where |
|---|---|---|---|
| `for:` (stateful duration) | "unused for at least 1 h", with an "unused since" timestamp; removes transients (G8) | not replicable: each evaluation is per object and stateless | **done** (`54dc498`), measured by RQ8 (§1.7) |
| Element-scoped criteria (G1) | removes silent false negatives, including in the paper's Krateo `Page` example | already possible (`exists(m, …)`) | **done** (limitations update, §1.8), with G2–G7 and G10 |
| Higher client QPS/Burst and indexed dependencies | removes the ~750-Smell ceiling and the quadratic matching; a prerequisite for any scale claim | – | QPS **done** (`54dc498`); indexing open |
| Pattern status conditions | failures become visible (§3) | misleading (R1a) or absent (R2) | new |
| Checking paths against the CRD schemas | makes "schema-checkable" true | CEL strings are not checked (RQ6a) | new |
| Cycle / reachability primitives | detects cycles of any length, with one Smell per cycle | no recursion; policies unrolled to a fixed length miss longer cycles | extension (b) |
| Kind sets resolved by discovery | covers new operators automatically | kinds must be enumerated in advance | **done** for group versions and categories (`kind: "*"`, `category`) and generated kinds (`fromTarget`); recursive paths (`**.secretRef.name`) open |

**Making readability measurable.** There are two options:
- a small user study: 10–15 participants, comprehension tasks on pattern/policy pairs, measuring time and correctness;
- the candidate RQ7 in [`../docs/proposals/intent.md`](../docs/proposals/intent.md): DSL vs CEL as the target language for LLM generation, scored against the 105 verdicts.

The material for the user study (5 verified pairs, two counterbalanced forms, answer key) is in [`readability/`](readability/README.md).

## 5. Consequences for the paper draft
**Title.** Two terms are weak:
- "architectural smells" is the least supported term: 36 of the 49 catalogued smells are orphans and 11 are dangling references;
- "interactions" suggests runtime communication.

Recommended titles:
- for a general venue: *KubePattern: Declarative Detection of Relational Smells across Kubernetes Custom Resources*;
- for an architecture venue: *KubePattern: Detecting Architectural Smells across Kubernetes Custom Resources*. In that case §2 must argue why orphans and dangling references are architectural smells.

**Draft statements that conflict with the evidence.**

| Draft | Evidence | Fix |
|---|---|---|
| Abstract: evaluation on "a mock application" and "third-party applications" | the evaluation covers 9 CRD ecosystems, with seeded ground truth, probes, mutation testing and the Kyverno comparison | report the RQ2, RQ3 and RQ6 numbers |
| Listing 1 (the Krateo `Page`), described as "lists its name and namespace among the entries" | classified *partial, G1* in `results/expressiveness/expressiveness.csv`: `items[*].name` and `items[*].namespace` are not correlated per element | **G1 implemented**: write the listing with `items[@]` (`krateo/patterns-v2/page-not-referenced.yaml`), and the sentence becomes true |
| Contribution (ii): "orphan and zombie resources, misplaced resources, misconfigurations" | the catalogue families are orphan 36, dangling 11, missing-companion 1, misconfiguration 1 | use the catalogue families |
| "Smells are written in a dedicated namespace or, optionally, in the namespace of the resource" | the chart default is `saveInNamespace: true` | invert the two |
| "validated … instead of silently producing wrong findings" | see §2 | use the §2 wording |
| "blind to relational smells"; "learning curve" | see §2 | use the §2 wording |
| CRDs "which the platform then reconciles exactly as the native ones" | the API server stores and serves them uniformly; the operator's controllers reconcile them | reword |
| Contribution (iii): "five relational primitives" | 3 were implemented; **the limitations update implements `selects` and `selectedBy`** | "five relational primitives" is true with the updated engine; release it before submission |

**Positioning paragraph (draft).**
```latex
\tool is not meant to replace a policy engine. Orphan and unused
resources are properties of the cluster state rather than of an admission
request, and policy engines detect them only through a background scan
evaluated by always-on controllers. On the same smells, a 1:1 Kyverno
translation reproduces all 105 verdicts of \tool, which cross-validates
its semantics, and CEL is strictly more expressive. \tool delivers the
same detection as a scheduled job with no idle footprint, about
8$\times$ fewer API requests and 39$\times$ less CPU than Kyverno's
reports controller at the same period, and with one finding per smell
instead of one report per evaluated resource. Because each finding keeps
its first observation, a Pattern can require a condition to hold for a
minimum duration: on scripted GitOps re-syncs, template rotations and
widgets wired during development, this removes every transient finding
that Kyverno reports, at a declared latency of one duration rounded up
to the next run.
```

## 6. External references (checked on 2026-10-01)
These are the related-work and context claims that come from outside the evaluation. Each was checked against its primary source, and the table gives the wording to use.

| Claim | Source | Verdict | Wording to use |
|---|---|---|---|
| Upstream ValidatingAdmissionPolicy cannot look up other resources | [kubernetes#133631](https://github.com/kubernetes/kubernetes/issues/133631), open since 2025-08-20 | **confirmed**: VAP sees only the object, the old object and an optional param | "ValidatingAdmissionPolicy has no cross-resource lookup; an enhancement request is open (#133631)." |
| Kyverno has an open request for time-based background re-evaluation | [kyverno#16214](https://github.com/kyverno/kyverno/issues/16214), open since 2026-06-02 | **partial**: it concerns `MutatingPolicy` `matchConditions`, with timestamps read from the object's status. It says nothing about `ValidatingPolicy` results or a "failing since" | Do not cite it as evidence for `for`. The evidence is RQ6d: reports are stateless and re-evaluated at every scan. At most: "background re-evaluation of time-based conditions is an open request even for mutation (#16214)." |
| Kyverno 1.19 | GitHub releases | **confirmed**: v1.19.0 on 2026-08-20; the evaluated v1.19.1 on 2026-09-10 | cite v1.19.1 |
| kor reports unused resources | [yonahd/kor](https://github.com/yonahd/kor) README and `pkg/kor/crds.go` | **confirmed**: 24 native kinds plus CRDs, where a CRD is unused when it has no instances ("CRD has no instances") | "kor reports unused native resources and CRDs without instances; it does not follow references between custom resources." |
| Popeye reports unused resources | [derailed/popeye](https://github.com/derailed/popeye) README | **confirmed**: potentially unused ServiceAccounts, Secrets and ConfigMaps (and their keys), plus checks on native workloads and volumes | "Popeye flags potentially unused native resources." |
| kube-score has hard-coded cross-resource checks | [zegl/kube-score](https://github.com/zegl/kube-score) `README_CHECKS.md` | **confirmed**: `service-targets-pod`, `ingress-targets-service`, `networkpolicy-targets-pod`, PDB and HPA checks, on manifests | §2 wording stands: "where they consider relations, these are hard-coded for native kinds." |
| Argo CD orphaned resources | [Argo CD docs](https://argo-cd.readthedocs.io/en/stable/user-guide/orphaned-resources/) | **confirmed**: "a top-level namespaced resource that does not belong to any Argo CD Application", within the project's destination namespaces; no reference semantics | "Argo CD's orphan monitoring measures membership in an Application, not references between resources." |
| Steampipe queries custom resources with SQL | [turbot/steampipe-plugin-kubernetes](https://github.com/turbot/steampipe-plugin-kubernetes) docs | **confirmed**: custom resources become dynamic tables (`custom_resource_tables`, all by default); it is a CLI that queries the API, and it writes no findings to the cluster | "Steampipe can join custom resources with ad-hoc SQL from outside the cluster; it keeps no findings in the cluster." |
| The Krateo portal renders about 530 custom resources and 45 RESTActions | [krateo-platformops/portal](https://github.com/krateo-platformops/portal) README, `main` @ `a1b4c7b` (2026-10-01, chart 1.5.93) | **stale**: rendering `helm/portal` with default values gives **577 objects**, among them 482 widgets (`widgets.templates.krateo.io/v1beta1`, 32 kinds), 63 RESTActions and 21 RBAC objects | Cite the rendered count with the commit, not the README. |

**Note for the Krateo case study.** The current portal (`main`, chart 1.5.93) uses a new widget set:
- it includes `Card`, `Flex`, `Listy`, `PageHeader`, `Statistic`, `Descriptions`, `Layout` and `Menu`;
- it has **no** `Page`, `Panel`, `Column` or `NavMenuItem`, which are the target kinds of the three given Patterns and of `krateo-navmenuitem-page-missing`;
- only `YamlViewer`, `Paragraph`, `Row`, `Markdown` and `RESTAction` remain.

The case study (Krateo 3.0.2, portal 1.3.4) is still valid for that release, but its widget Patterns do not carry over to the next portal. This supports the G6 argument: the kind set changes between portal versions, so an enumerated kind list goes stale. State it as a threat to the case study's external validity.
