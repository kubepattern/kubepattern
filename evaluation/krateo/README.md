# Krateo case study: KubePattern vs Kyverno on Krateo PlatformOps

A separate case study, next to RQ1–RQ6 (whose numbers it does not change). It answers one question: **on a real Krateo platform, do KubePattern and Kyverno address the same types of smells?**

It runs on its own cluster (minikube profile `kp-krateo`):
- **Krateo PlatformOps 3.0.2**, installed as the official docs describe;
- **a blueprint (`springboot-app`) and two compositions** of it. The blueprint is enriched with the objects of a real Krateo installation: the anonymised dump in [`fixtures/files-clean.yaml`](fixtures/files-clean.yaml);
- the **3 Patterns given with the dump** plus **8 new Krateo Patterns**, run by KubePattern (as-is engine `69d4ffd`);
- the same smells as **Kyverno 1.19.1** CEL `ValidatingPolicy` sets: a 1:1 translation (`equivalent/`) and a `best-effort/` one that uses CEL's extra power.

Results are in [Results](#results).

## Contents
| Path | What |
|---|---|
| `env/krateo.env` | cluster size and version pins; honoured by the shared scripts (`../env/versions.env`) |
| `env/cluster-up.sh` | stops kp-eval (state kept) and creates `kp-krateo` with `../env/minikube-up.sh` |
| `env/install/10-krateo.sh` | pinned `krateoctl` 1.2.1, the three install Secrets, `krateoctl install apply --version 3.0.2 --type nodeport --profile no-finops` |
| `env/install/20-chart-repo.sh` | packages the blueprint and serves it from an in-cluster Helm repository (nginx, `http://charts.kp-charts.svc.cluster.local`) |
| `env/install/30-blueprint.sh` | CompositionDefinition of the official `portal-blueprint-page` 1.0.9, a `PortalBlueprintPage` for `springboot-app`, the two compositions |
| `env/install/40-kubepattern.sh`, `50-kyverno.sh`, `60-seeds.sh` | KubePattern and Kyverno (shared RQ6 scripts), the controlled cases |
| `blueprint/springboot-app/` | the blueprint (Helm chart + `values.schema.json`) |
| `scenario/` | `compositions.yaml`, `seeds*.yaml`, `ground-truth.csv`, `ground-truth-kyverno-only.csv`, `mutations.csv`, `lifecycle/` |
| `patterns/` | one Pattern per file; `verbatim/` holds the given `panel-not-referenced` as it was written |
| `kyverno/` | `rbac.yaml`, `globalcontext/` (30 entries), `equivalent/` (11), `best-effort/` (10), `verbatim/` |
| `scripts/` | `krateo_oracle.py` (independent oracle, ground truth), `final-runs.sh` (all scored runs), `lifecycle.sh` + `lifecycle_report.py`, `krateo_compare.py` |
| `measurements/` | `effectiveness.csv`, `expressiveness.csv`, `coverage.csv`, `lifecycle.csv`, `tables/krateo.tex`; raw runs in `../measurements/raw/{krateo,kyverno/krateo}/` (git-ignored) |

## Setup
```bash
cd evaluation/krateo
./env/cluster-up.sh                 # kp-krateo: kvm2, 12 vCPU, 16 GiB, Kubernetes v1.34.4 (kp-eval is stopped)
./env/install/10-krateo.sh          # Krateo 3.0.2 (~10 min)
./env/install/20-chart-repo.sh      # in-cluster Helm repository with springboot-app 1.0.0
./env/install/30-blueprint.sh       # PortalBlueprintPage -> CompositionDefinition -> 2 SpringbootApp compositions
./env/install/40-kubepattern.sh     # KubePattern 69d4ffd, CronJob suspended
./env/install/50-kyverno.sh         # Kyverno 1.19.1 (chart 3.9.1), background scan every 5 min, read RBAC
./env/install/60-seeds.sh           # controlled cases + 2 seeded CompositionDefinitions
```
Every script sources `env/krateo.env`, so the shared scripts in `../scripts/` target `kp-krateo`. From `evaluation/`, run `source krateo/env/krateo.env` first.

| Component | Version |
|---|---|
| Kubernetes (minikube, kvm2, docker) | v1.34.4 |
| Krateo PlatformOps (`krateoctl` 1.2.1, profile `no-finops`) | 3.0.2: core-provider 1.0.0, snowplow 0.20.6, frontend 1.0.27, authn 0.22.2, portal 1.3.4, resources-presenter 0.10.3 |
| Blueprints (marketplace.krateo.io) | portal-blueprint-page 1.0.9, portal-composition-page-generic 1.4.3, github-scaffolding-with-composition-page 1.2.5 |
| KubePattern | `69d4ffd` (as-is engine of the main results), chart from the working tree |
| Kyverno | 1.19.1 (chart 3.9.1), `backgroundScanInterval=5m` |

## The blueprint
`blueprint/springboot-app` (chart 1.0.0) generates kind `SpringbootApp` at `composition.krateo.io/v1-0-0`.

**Base.** It follows the official app blueprints (`github-scaffolding-with-composition-page`):
- a Helm chart with a `values.schema.json`, from which core-provider generates the CRD;
- the composition page from the Helm dependency `portal-composition-page-generic` 1.4.3: composition Panel, TabList, events/status/values Panels, YamlViewer, Buttons, Form and RESTActions.

The GitHub and Argo CD parts are dropped, because they need external credentials.

**Enrichment from the dump.** Every object of `files-clean.yaml` is rendered from the values, with its name, labels and references:

| Template | Objects (demo-app) | Dangling references kept from the dump |
|---|---|---|
| `namespace.yaml`, `configmap.documentation.yaml` | Namespace `<composition name>`, documentation ConfigMap | – |
| `rbac.yaml` | 5 Roles, 6 RoleBindings (AI CVE agents, platform developers) | agent ServiceAccounts in the tenant namespace; ClusterRole `platform-devs-vulnerabilityreport-clusterrole` |
| `cve.yaml` | RESTAction `cve-table`, one Markdown per CVE (label `managed-by: cve-table`) | – |
| `metrics.yaml` | one Prometheus RESTAction per pod (label `krateo.io/managed-by`) | endpoint Secret `krateo-system/prometheus-endpoint`; the generator named in the label |
| `sbom.yaml` | SBOM RESTAction + YamlViewer (`spec.apiRef`) per ReplicaSet (label `managed-by: sbom-table`) | the `sbom-table` manager |

**The layout the dump lacks.** `portal-page.yaml` adds NavMenuItem → Page → Row → 2 Columns → Panels (overview, CVE, SBOM) → Paragraph / Markdown / YamlViewer, so that the given Patterns (Page, Panel, Paragraph) have real targets. The portal lists every NavMenuItem in its sidebar, so the page is reachable at `/apps/<name>`.

**Template defects.** They are realistic ones, and every composition inherits them:
- the release-notes Paragraph is rendered and listed under two separate switches;
- an SBOM viewer can be rendered but not listed (`displayed: false`) or listed without its RESTAction (`scanned: false`);
- `cve.tableEnabled: false` removes the CVE Markdowns' manager.

**Deviation from the dump.** In the dump, the RESTActions and widgets were created by in-cluster generators (their labels name them). The blueprint renders them statically, so they also carry the composition labels.

**Compositions.**
- `SpringbootApp tenant-a/demo-app` mirrors the dump.
- `tenant-b/demo-app-2` has other pods and CVEs, `cve.tableEnabled: false`, one unscanned SBOM, and the release notes listed.

Both are installed through the portal's own mechanism: a `PortalBlueprintPage` (official `portal-blueprint-page` 1.0.9) templates the CompositionDefinition and the blueprint's card on the portal's Blueprints page.

## Scenario and ground truth
- **Natural state:** Krateo itself, the portal, the two compositions (57 and 50 managed objects) and the Kyverno/KubePattern installs.
- **Controlled cases** in `kp-krateo-a/b` ([`scenario/seeds.yaml`](scenario/seeds.yaml), 64 objects). For each Pattern:
  - positives, negatives, near-misses (same name in another namespace, a reference only through another layout, cross-namespace references);
  - probes of the engine limitations: G1, G2, G4, G6, G7;
  - `probe-scope`: gaps of the given Patterns' scope, which can be fixed in the DSL.
- **Seeded CompositionDefinitions** (`seeds-definitions.yaml`):
  - `github-scaffolding-with-composition-page`, with no composition;
  - `portal-composition-page-generic`, with one composition (`page-generic-a`).
- **Oracle.** [`scripts/krateo_oracle.py`](scripts/krateo_oracle.py) generates the ground truth from a cluster snapshot, independently of the engine. For every target of every Pattern it computes two verdicts:
  - `truth`: the operational meaning in Krateo 3.0.2. References are correlated per item, every reference must resolve, the portal's label discovery and `/call` paths count as uses, and generated kinds are listed from the definition's status;
  - `engine_expected`: the Pattern re-implemented from the verified engine semantics.

  A difference between the two is a probe. Near-misses are the seeded ones listed in the script.
  - **Ground truth:** 201 rows over 11 Patterns, plus 7 rows for the Kyverno-only policy (`ground-truth-kyverno-only.csv`).
  - **Re-check:** `krateo_oracle.py check <snapshot>` compares a new snapshot with the committed ground truth.

## Patterns
**Given** (from the dump; spec verbatim, one per file, suite labels added). Their names have no `krateo-` prefix.

| Pattern | Target ← referrers | Notes |
|---|---|---|
| `page-not-referenced` | Page ← NavMenuItem, Panel (`resourcesRefs.items[*]` name + namespace) | G1: name and namespace not correlated per item; Pages reached only through a Route are out of scope |
| `panel-not-referenced` | Panel ← Panel, Column, Row, TabList, Page, DataGrid (name only), PortalBlueprintPage (`status.managed[*].name`) | pins PortalBlueprintPage **v1-0-10**, which no published portal-blueprint-page provides (latest 1.0.9): as given, the whole Pattern is skipped. The scored copy reads v1-0-9 (`patterns/panel-not-referenced.yaml`, a documented deviation). Names only (no namespace, no resource); Panels listed by label are G7 |
| `paragraph-not-referenced` | Paragraph ← Panel, Column, Row (name + namespace) | G1 |

**New** (proposed for this case study):

| Pattern | Smell | Family | Why it matters in Krateo |
|---|---|---|---|
| `krateo-yamlviewer-apiref-dangling` | `spec.apiRef` names a missing RESTAction | dangling | snowplow fails to resolve the widget (hard error) |
| `krateo-restaction-not-used` | no widget reads it through `spec.apiRef` (24 widget kinds enumerated) | orphan | API calls defined but never rendered; the portal's own RESTActions (krateo-system) are excluded |
| `krateo-restaction-endpoint-missing` | an `api[*].endpointRef` Secret is missing | dangling | **silent**: snowplow stops resolving and the widget renders empty, without an error |
| `krateo-widget-manager-missing` | Markdown `managed-by` label names a missing RESTAction/Table | dangling (label) | generated widgets that nothing keeps up to date |
| `krateo-navmenuitem-page-missing` | the Page a sidebar entry opens does not exist | dangling | broken navigation |
| `krateo-rolebinding-clusterrole-missing` | RoleBinding to a missing ClusterRole | dangling (RBAC) | composition RBAC that grants nothing today and whatever a future ClusterRole grants |
| `krateo-rolebinding-subject-sa-missing` | RoleBinding to a missing ServiceAccount | dangling (RBAC) | dormant grants to agents that were never deployed |
| `krateo-compositiondefinition-unused` | CompositionDefinition with no composition | unused definition | a CRD and a composition-dynamic-controller kept running for nothing |

## Kyverno translation
- **`kyverno/equivalent/`:** a 1:1 CEL `ValidatingPolicy` per Pattern, with the RQ6 translation rules:
  - `fail` means smell; each policy has the same name as its Pattern;
  - one GlobalContextEntry per dependency kind;
  - uncorrelated `[*]` criteria become separate `exists`;
  - target filters become `matchConditions`.

  Where several dependencies share the same criteria, CEL folds them into one list (`[a, b, ...].exists(l, ...)`). It is a CEL abstraction that the DSL does not have.
- **`kyverno/best-effort/`:** fixes the engine limitations and nothing else. The given Patterns' scope (Route, names only) is kept, because it can also be fixed in the DSL.

  | Limitation | CEL fix | Policies |
  |---|---|---|
  | G1 | correlated `exists(i, name && namespace && resource)` | E1, E3, N5 (`resourceRefId` ↔ `items[].id`), N7 |
  | G2 | `all(...)` over endpoints and subjects | N3, N7 |
  | G4 | `/call` path strings | N2 |
  | G5 | `labels['krateo.io/...']` | E2, and the Kyverno-only `krateo-restaction-manager-missing` |
  | G6 | several target kinds in one policy | N4 |
  | G6 | instances listed live with `resource.List(object.status.apiVersion, object.status.resource, "")` | N8 |
  | G7 | label-discovered Panels | E2 |
- **`kyverno/verbatim/`:** the given `panel-not-referenced` with its v1-0-10 GlobalContextEntry, for the robustness comparison.
- **RBAC:** `kyverno/rbac.yaml` gives aggregated read access to the Krateo groups, RBAC, Secrets and ServiceAccounts.
- **Secrets GlobalContextEntry:** it is projected to name and namespace, so policies see only the references (the informer still watches whole Secrets).

## Protocol
From `evaluation/`, after `source krateo/env/krateo.env`:
```bash
kubectl apply -f krateo/patterns/                          # 11 Patterns (panel-not-referenced adapted to v1-0-9)
./scripts/run-once.sh krateo/kp-r{1,2,3}                   # KubePattern, 3 repetitions
kubectl apply -f krateo/kyverno/globalcontext/ -f krateo/kyverno/equivalent/
./scripts/kyverno-run.sh krateo/equivalent-r{1,2,3}        # forced re-scan, archived when the reports settle
kubectl apply -f krateo/kyverno/best-effort/
./scripts/kyverno-run.sh krateo/best-effort-r{1,2,3}
./krateo/scripts/lifecycle.sh && ./krateo/scripts/lifecycle_report.py
./krateo/scripts/krateo_compare.py                         # measurements/*.csv and tables/krateo.tex
```
- **Scoring:** each run is scored with `../scripts/score.py --truth krateo/scenario/ground-truth.csv`.
  - KubePattern and the 1:1 policies are scored against `engine_expected` (the probes must behave as the engine semantics predict).
  - The best-effort set is scored against `truth`, with `ground-truth-kyverno-only.csv` added.
- **Verbatim run:** `patterns/verbatim/` and `kyverno/verbatim/`, applied once in place of the adapted `panel-not-referenced`.

## Results
All scored runs are on the same cluster state (`scripts/final-runs.sh`), which is the state of `scenario/ground-truth.csv` (201 rows; `krateo_oracle.py check` 201/201).

**Answer.** On Krateo, both tools address the same smell types, with the same limits:
- Where a smell reads **static reference fields**, the two agree verdict for verdict. That covers orphan widgets, orphan RESTActions, dangling apiRefs, endpoints and navigation, dangling RBAC and unused definitions.
- **Kyverno's CEL goes further on 6 of the 7 engine limitations that Krateo exercises:** G1, G2, G4, G5, G6 (target kinds, generated kinds) and G7 (label discovery).
- **Neither tool models snowplow's dynamic references (G9).** Neither can avoid enumerating the 24 widget kinds for "RESTAction not used" (G6 on the dependency side).

### Effectiveness (3 repetitions each, identical)
| Tool | Smells | TP | FP | FN | TN | P / R | G-probes (22) | Scope probes (3) |
|---|---|---|---|---|---|---|---|---|
| KubePattern `69d4ffd` | 61 | 55 | 0 | 0 | 121 | 1.00 / 1.00 | 22/22 as the engine semantics predict | as predicted |
| Kyverno 1:1 (`equivalent/`) | 61 `fail` | 55 | 0 | 0 | 121 | 1.00 / 1.00 | 22/22 as predicted (same verdicts as KubePattern on all 201 rows) | as predicted |
| Kyverno best-effort | 79 `fail` | 61 | 0 | 0 | 122 | 1.00 / 1.00 | **22/22 resolved** | kept (0/3, by design) |

- **In-scope rows:** the best-effort line also counts the Kyverno-only G5 policy (6 positives, 1 negative).
- **Scope probes:** they are gaps of the given Patterns (a Page reached through a Route; Panels matched by name only). Both tools can fix them, so best-effort keeps them to stay comparable.
- **Probes per limitation** (seed and natural):

  | Probe | Rows | Examples |
  |---|---|---|
  | G1 | 5 | `page-g1`, `para-g1`, `nav-two`, `ra-ep-g1`, `rb-sa-g1` |
  | G2 | 2 | `ra-ep-mixed`, `rb-sa-mixed` |
  | G4 | 1 | `ra-called` |
  | G6 | 11 | 10 YamlViewers/RESTActions with a missing `managed-by` manager, and the `portal-composition-page-generic` definition |
  | G7 | 3 | the composition Panels that the portal lists by label, including both compositions' |
- **Engine cost, descriptive only:** one KubePattern run takes 33 s and 160 API requests. Kyverno reports settle 63–102 s after a forced re-scan. Cost was not in scope; RQ6 has the comparison.

### Coverage by smell type (`measurements/coverage.csv`, `measurements/tables/krateo.tex`)
| Smell | Family | KubePattern | Kyverno 1:1 | Kyverno best-effort | Limitation | Rows agreeing with the truth (KP / 1:1 / best) |
|---|---|---|---|---|---|---|
| Page not referenced | orphan widget | partial | partial | expressible | G1; scope | 6/8, 6/8, 7/8 |
| Panel not in any layout | orphan widget | partial | partial | expressible | G7/G5; scope; version pin | 20/25, 20/25, 23/25 |
| Paragraph not referenced | orphan widget | partial | partial | expressible | G1 | 8/9, 8/9, 9/9 |
| YamlViewer apiRef dangling | dangling | expressible | expressible | expressible | – | 13/13 in all three |
| RESTAction not used | orphan | partial | partial | partial | G6 (both), G4 | 36/37, 36/37, 37/37 |
| RESTAction endpoint Secret missing | dangling (silent) | partial | partial | expressible | G1; G2 | 23/25, 23/25, 25/25 |
| Widget manager missing (`managed-by`) | dangling label | partial | partial | expressible | G6 | 11/21, 11/21, 21/21 |
| RESTAction generator missing (`krateo.io/managed-by`) | dangling label | **not expressible** | n/a | expressible | G5 | –, –, 7/7 |
| NavMenuItem Page missing | dangling | partial | partial | expressible | G1 | 9/10, 9/10, 10/10 |
| RoleBinding to missing ClusterRole | dangling (RBAC) | expressible | expressible | expressible | (G5: no composition scoping) | 10/10 in all three |
| RoleBinding to missing ServiceAccount | dangling (RBAC) | partial | partial | expressible | G1; G2 | 37/39, 37/39, 39/39 |
| CompositionDefinition unused | unused definition | partial | partial | expressible | G6 (`resource.List` on the definition's status) | 3/4, 3/4, 4/4 |
| References computed by snowplow | orphan widget | not expressible | not expressible | not expressible | G9 | – |

**Totals:**
- KubePattern: 2 smell types expressible, 9 partial, 2 not.
- Kyverno best-effort: 11 expressible, 1 partial (the widget-kind enumeration), 1 not (G9).

**The G6 shape differs between the tools:**
- For a *generated* kind (N8), CEL resolves the kind at run time from the definition's status.
- For "any widget", neither tool can discover the kinds: KubePattern writes 24 relationship entries (a 337-line Pattern); Kyverno needs 24 GlobalContextEntries (216 lines) under a 55-line policy.

### Expressiveness (`measurements/expressiveness.csv`)
- **Size:** the 11 Patterns total 826 LOC (489 without the enumerating `krateo-restaction-not-used`). The 11 policies total 367 LOC (312 without it), plus 30 shared GlobalContextEntries (273 LOC).
- **Decision logic:** 91 Pattern primitives against 34 CEL predicates.
- **CEL list abstraction:** `[a, b, ...].exists(...)` folds dependencies that share a criterion; the DSL repeats one relationship entry per kind.
- **Best-effort cost:** fixing the limitations changes the policy size by −5 to +5 lines. In most cases the fix is a tighter predicate, not more code.

### Composition lifecycle (`measurements/lifecycle.csv`)
Six mutations were done through Krateo (`scenario/mutations.csv`):
- composition values (release notes listed, CVE table off, an SBOM unscanned);
- a deleted referrer;
- a created endpoint Secret;
- a deleted composition;
- a deleted definition.

| Tool | Baseline / mutated / reverted Smells | Equal to the oracle | Expected changes seen | Reverted == baseline |
|---|---|---|---|---|
| KubePattern | 61 / 51 / 61 | 3/3 phases | 26/26 (+8, −18) | yes |
| Kyverno 1:1 | 61 / 51 / 61 | 3/3 phases | 26/26 | yes |

Both tools converge after Krateo re-renders a composition, deletes it or recreates it; the CronJob-mode GC and the report cleanup both drop the Smells of deleted objects.

### Robustness: the given `panel-not-referenced` as written (v1-0-10)
| Tool | Behaviour |
|---|---|
| KubePattern | the whole Pattern is skipped (one `WARN ... the server could not find the requested resource`), and the other 10 are unaffected: its 2 true positives are lost silently, except for the log line |
| Kyverno | per object: 19 `pass` (Panels already referenced by a layout short-circuit the `||`) and 6 `error` ("got 'types.Null', expected iterable type"), including the 2 true positives. The policy status stays `ready` |

## Engine update and RQ8 (2026-10-01)
These are separate validations; the results above stay on `69d4ffd`.
- **Regression of `54dc498`** (client QPS, linter, `spec.for` unset), with `krateo/scripts/regression.sh`:
  - 3 in-cluster runs: 55/0/0/121, 25/25 probes;
  - lifecycle 61/51/61, 26/26, reverted == baseline, for both tools;
  - a whole-cluster run takes 1.42 s instead of 31.85 s, with the same 171 requests (`krateo/scripts/qps-krateo.sh`, `../measurements/qps/README.md`).

  The lifecycle re-created `demo-app-2`, which renamed two RoleBindings in `scenario/ground-truth.csv`.
- **RQ8** (`krateo/scripts/rq8-dev-wiring.sh`, `scenario/rq8/`, `../measurements/rq8/README.md`). `paragraph-not-referenced` was evaluated next to a `for: 5m` copy in the same runs:
  - widgets wired during development, or briefly unreferenced during a re-sync, give 3 transient findings without `for` and none with it;
  - Kyverno reports them as `fail`;
  - the never-wired widget is reported 2 periods later.

  This is the trade-off that the registry documentation of the widget Patterns describes.
- **The current portal has moved on.** `krateo-platformops/portal` `main` (chart 1.5.93, 2026-10-01) renders 577 objects with a new widget set: there is no `Page`, `Panel`, `Column` or `NavMenuItem`. The widget Patterns of this case study therefore apply to Krateo 3.0.x only (`../POSITIONING.md` §6). This is a threat to external validity, and evidence for G6: enumerated kind lists go stale.

## Findings about Krateo (observed while building the case study)
These are what a platform team hits while doing what this case study did. None of them is reported by the platform as a failure of the affected object.

1. **`portal-blueprint-page` 1.0.9 cannot publish a chart without credentials.**
   - It renders `credentials: {passwordRef: null}`, which the CompositionDefinition CRD rejects.
   - The Helm release fails, yet the `PortalBlueprintPage` reports `Ready=True` ("Composition created").
   - The blueprint's card (Panel, Markdown, Form) is created anyway, so the portal offers a blueprint whose CompositionDefinition does not exist.
   - Worked around with dummy credentials (`scenario/blueprint-page.yaml`).
2. **The `krateo.io/release-name` annotation is not honoured** by the composition-dynamic-controller (core-provider 1.0.0).
   - Release names get a random 8-character suffix (e.g. `springboot-app-4zdz9fdz`, not the UID prefix the docs describe).
   - The CDC's per-release RoleBindings are renamed whenever a composition is recreated, so the ground truth has to be regenerated then.
3. **core-provider 1.0.0 turns array-of-object `default`s in `values.schema.json` into strings.**
   - The generated CRD is rejected; this one is reported (`Synced=False`, `ReconcileError`).
   - Fixed by moving the default to `values.yaml`.
4. **The official portal 1.3.4 ships a dangling reference:** Route `demo-system/demo-system-compositions-route` targets DataGrid `demo-system/demo-system-compositions-page-datagrid`, which the chart does not create. No Pattern covers Route targets: the target kind varies (G6) and other Routes use placeholders (G9).
5. **The widget CRDs make `spec.apiRef.namespace` required.** snowplow would silently ignore an `apiRef` without a namespace, so the "no namespace" case cannot arise.
6. **A `PortalBlueprintPage` version pin (`v1-0-10`) in the given Pattern is not satisfiable** with any published `portal-blueprint-page` (latest 1.0.9). Generated kinds change with the chart version, which is also why N8 needs G6.

**Kyverno.** When a policy's match is narrowed (best-effort N4 covers 3 kinds, the 1:1 one only Markdown), the old results of the no-longer-matched resources stay in the PolicyReports: 21 results where 11 are current. The snapshot never settles until the reports are deleted. `lifecycle.sh` and `final-runs.sh` reset the reports whenever the policy set changes.

## Deviations from the plan
- **Krateo 3.0.2, installed with `krateoctl`.** krateoctl 1.2.1 has no `--init-secrets`, so `10-krateo.sh` creates the three Secrets as the docs specify.
- **`panel-not-referenced` is scored at v1-0-9.** The as-given version is measured only as a robustness run.
- **Metrics scripts unchanged.** `pattern_metrics.py` and `policy_metrics.py` were not modified: `krateo_compare.py` imports the RQ6 policy metrics instead. The only shared file changed is `env/versions.env`: the profile and cluster size are now overridable, with defaults unchanged.
- **Mutations.** The lifecycle experiment replaces `mutate.sh`, whose mutations are hard-coded for the 9-platform scenario.
- **Discarded runs.**
  - One best-effort run used a policy file whose E2 fix had not been applied: a script error, archived as `best-effort-bug-e2`, not scored.
  - The runs before the lifecycle experiment are kept as `*-prelifecycle-r*` (same results), and the KubePattern runs before the Kyverno install as `kp-prekyverno-r*` (same results; the install only adds 4 negative RoleBindings).
- **Cost and staleness were not measured** (out of scope).

## Threats to validity
- **Same authors.** The same authors wrote the blueprint, the new Patterns, the policies and the oracle.
  - The oracle is an independent re-implementation, not the engine: it agrees with the engine on 201/201 rows, and on 51 and 61 Smells in the lifecycle phases.
  - `truth` encodes one reading of Krateo's semantics (label discovery and `/call` paths count as uses).
- **Synthetic enrichment.** The blueprint renders statically what the dump's generators created at run time. The natural state is one Krateo installation with two compositions.
- **Scope probes** are properties of the given Patterns, not of the engine. They are counted separately.
