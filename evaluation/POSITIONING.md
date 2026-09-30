# Positioning KubePattern

How to position KubePattern in the paper, using only the evidence of RQ1–RQ6 ([`README.md`](README.md)) and the code (`dev` @ `69d4ffd`, fix `a814e4a`). For each claim, this file gives what supports it and where the numbers come from. Claims that the evidence does not support are listed separately, with the wording to use instead.

## Thesis
> On relational smells across CRD ecosystems, KubePattern matches the detection of a general-purpose policy engine, as verified by that independent engine. It does so at a fraction of the operating cost, and its output is made of findings rather than per-resource reports. It is a purpose-built periodic analyser that complements admission control. It does not replace a policy engine.

The question "why not a policy engine?" must be answered with:
- the problem class;
- the operating cost;
- the output model;
- the verified semantics.

It cannot be answered with expressive power or readability. On both, Kyverno is at least equal (§2).

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
  - client-go's 5 QPS default caps KubePattern at about 750 Smells per run, and its matching is quadratic (RQ4). **Fix both before calling KubePattern a serious alternative on large clusters.**

### 1.4 Finding-oriented output
| | Kyverno 1.19 | KubePattern |
|---|---|---|
| What is written | one report per evaluated resource, with pass/fail/error/skip per policy | one Smell per finding, and nothing for clean resources |
| On the RQ2 scenario | 105 reports (74 `PolicyReport` in 13 namespaces, 31 `ClusterPolicyReport`) holding 46 `fail` and 59 `pass` | 46 Smells |
| Writes per cycle | about 349 report writes and about 92 Events | 46 Smell updates |
| Identity across runs | one report per resource | `<pattern>-<targetUID>`: 40 of 51 identities were a single object across all 9 RQ5 runs |
| Metadata | `severity` and `category` are optional (not set by these policies); `messageExpression` is full CEL | severity, category, message and reference; only 5 metadata placeholders |
| Resource a relational smell is attributed to | the resource the policy matches | the target, by construction of the DSL |

- `kubectl get smells -A` lists the problems to fix. With Kyverno, the failures must be filtered out of the reports.
- Self-healing: a fixed smell disappears at the next run (RQ5).
- **Some of these claims depend on the fix.** "The Smells of a skipped pattern are kept" and "no flapping" hold only with the per-pattern prune (`a814e4a`). Merge it before submission, so that the released tool matches the paper.
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

## 2. Claims to avoid
| Claim | Why not | Say instead |
|---|---|---|
| "The DSL is more readable or simpler than CEL" | Readability was not measured (no user study). By size, CEL wins: 417 vs 657 lines (249 vs 475 logic lines), with 59 vs 58 atomic comparisons. H1 is not confirmed (RQ6a) | "A pattern contains no expressions: every check is a (path, operator, values) triple within a fixed structure of target, dependencies and relationships. The Kyverno translations are shorter, but they move the logic into 5,129 characters of CEL, with 43 `exists`/`all` comprehensions, 67 optional hops and `dyn()` casts." |
| "Policy engines cannot express relational smells" or "existing analysers are blind to relational smells" | CEL expresses all 14 smells, and the best-effort variants resolve 10/10 probes (RQ6a) | "Policy engines express them through explicit lookups, evaluated by always-on controllers." |
| "Policy languages have a learning curve that suits developers rather than architects and operations staff" | there is no user study | Drop it, or state it as a design goal. |
| "Patterns are validated, so they cannot silently produce wrong findings" | The linter checks structure only. A mistyped path yields an empty value set, so inside `matchNone` every target becomes a finding. `selects` and `selectedBy` pass the linter (`internal/linter/pattern.go:458`) but evaluate to false (`internal/analysis/resolver.go:90`) | "A structurally malformed pattern is logged and skipped without affecting the others." Note that a mistyped path fails the same way as CEL optional chaining. |
| "Schema-checkable", as a current feature | The linter does not check paths against the CRD schemas. The evaluation checked them externally (`scripts/verify_fields.py`, 41/41) | Present it as a potential advantage of declarative paths, until it is implemented (§4). |
| "Findings are visible only to the users entitled to see them (RBAC)", as a differentiator | Kyverno also writes namespaced reports, with the same RBAC granularity (§3) | Keep it as a design note, not a contribution. |
| "Detects smells faster than Kyverno" | both tools are bounded by the period (RQ6d) | "Same detection latency, bounded by the period." |
| "Scales to large clusters" | the 5 QPS ceiling (about 750 Smells per run), quadratic matching and unpaginated LISTs (RQ4); the S1 sweep was not run for Kyverno | Re-measure after raising QPS and indexing. |
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
| `for:` (stateful duration) | "unused for at least 1 h", with an "unused since" timestamp; removes transients (G8) | not replicable: each evaluation is per object and stateless | README *Discussion*, extension (a) |
| Element-scoped criteria (G1) | removes silent false negatives, including in the paper's Krateo `Page` example | already possible (`exists(m, …)`) | roadmap item 1 |
| Higher client QPS/Burst and indexed dependencies | removes the ~750-Smell ceiling and the quadratic matching; a prerequisite for any scale claim | – | roadmap item 2 |
| Pattern status conditions | failures become visible (§3) | misleading (R1a) or absent (R2) | new |
| Checking paths against the CRD schemas | makes "schema-checkable" true | CEL strings are not checked (RQ6a) | new |
| Cycle / reachability primitives | detects cycles of any length, with one Smell per cycle | no recursion; policies unrolled to a fixed length miss longer cycles | extension (b) |
| Kind sets resolved by discovery | covers new operators automatically | kinds must be enumerated in advance | extension (c) |

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
| Listing 1 (the Krateo `Page`), described as "lists its name and namespace among the entries" | classified *partial, G1* in `results/expressiveness/expressiveness.csv`: `items[*].name` and `items[*].namespace` are not correlated per element | implement G1, or state the limitation |
| Contribution (ii): "orphan and zombie resources, misplaced resources, misconfigurations" | the catalogue families are orphan 36, dangling 11, missing-companion 1, misconfiguration 1 | use the catalogue families |
| "Smells are written in a dedicated namespace or, optionally, in the namespace of the resource" | the chart default is `saveInNamespace: true` | invert the two |
| "validated … instead of silently producing wrong findings" | see §2 | use the §2 wording |
| "blind to relational smells"; "learning curve" | see §2 | use the §2 wording |
| CRDs "which the platform then reconciles exactly as the native ones" | the API server stores and serves them uniformly; the operator's controllers reconcile them | reword |
| Contribution (iii): "five relational primitives" | 3 are implemented; the selector primitives pass the linter but evaluate to false | write "five primitives, three of which are implemented", and make the linter reject the other two |

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
instead of one report per evaluated resource.
```
