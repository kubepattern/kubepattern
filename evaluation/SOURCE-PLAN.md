# Testing plan extracted from the source note

This file extracts the testing plan from the research note *"KubePattern Beyond Krateo: Verified Use Cases for Section 4 (CronJob Mode)"*. The note was written without access to the repository. Its content is reproduced faithfully and without corrections. [`TEST-PLAN.md`](TEST-PLAN.md) is the version rewritten against the verified engine semantics, and [`README.md`](README.md) has the results.

## 1. Goal
Evaluate KubePattern as a CronJob-mode analyzer on CRD-based platforms that resolve references between custom resources **by name in spec fields**, where the controllers never report *unused* definitions.

## 2. Subject systems (all on local kind, no cloud accounts)
| Id | System | Content |
|---|---|---|
| S1 | Krateo | the existing experiments |
| S2 | Crossplane v2 | Functions, XRDs and Compositions from a public configuration package (definitions only) |
| S3 | KubeVela | default install plus sample Applications; built-in definitions give a realistic "unused catalogue" population |
| S4 | Cluster API + CAPD | one ClusterClass-based cluster, then a template rotation to produce real orphans |
| S5 | GitOps stack | Flux, cert-manager, CloudNativePG, ESO (fake/kubernetes provider); optionally a public home-ops repository |
| S6 | Argo CD + ApplicationSets | for `owns` |
| baseline | standard kinds | Online Boutique or Bookinfo |

## 3. Candidate patterns (shortlist, section C/D of the note)
| # | Platform | Smell | Primitives |
|---|---|---|---|
| 1 | Crossplane v2 | unused Function; Composition whose XRD is missing | custom, matchNone |
| 2 | KubeVela | unused Component/TraitDefinition | custom, nested `[*]` |
| 3 | Kargo | Warehouse not requested by any Stage | custom (+ namespace) |
| 4 | Cluster API | orphan DockerMachineTemplate (MD, KCP, ClusterClass) | custom, 3 dependencies |
| 5 | Flux | unused OCIRepository (HelmRelease `chartRef`, Kustomization `sourceRef`) | custom |
| 6 | cert-manager | unused ClusterIssuer (Certificate, CertificateRequest) | custom, dependency filters |
| 7 | CloudNativePG | Cluster without ScheduledBackup; Pooler named like a Cluster | custom, matchNone / matchAll |
| 8 | Argo CD | ApplicationSet owning no Applications; AppProject without Applications | owns + matchNone; custom |

## 4. Seeding
- For each pattern, inject *k* instances of the smell and *k* near-miss negatives. The note's examples:
  - a Function referenced only by an Operation;
  - a Flux source referenced cross-namespace;
  - a Krateo NavMenuItem matching `items[0]` by name and `items[1]` by namespace (correlation false negative);
  - an issuer referenced only via ingress-shim.
- Mutation-style seeding: delete a referrer, rename a reference, delete the referenced object.

## 5. Oracles
- **Dangling references:** controller status conditions (Gateway `ResolvedRefs=False`, cert-manager, Flux, Crossplane).
- **Standard kinds:** kor and Popeye output.
- **Orphans:** manual inspection by two raters with Cohen's κ, since no tool reports unused definitions.
- **Cross-check:** Kyverno background-scan policies with `context.apiCall`; compare rule size (policy lines vs Pattern lines) as an expressiveness metric.

## 6. Metrics
- Precision and recall per pattern, with separate counts for false negatives caused by limitations G1–G3.
- CronJob wall-clock time and peak memory.
- API requests per run (audit logs or client metrics) against the number of kinds and objects, with synthetic scaling (1k/5k/10k Applications or Certificates).
- Stale-Smell behaviour across consecutive runs, after fixes and after target deletion.

## 7. CronJob-specific protocol
- Run every 5 minutes during seeded churn (Flux reconcile, CAPI rollout, cert issuance).
- Count transient findings that disappear within one or two runs. This quantifies snapshot false positives and motivates a minimum-age filter.
- Recommended schedule: hourly default in production, daily for CAPI.

## 8. Limitations to report (section G of the note)
| Id | Limitation |
|---|---|
| G1 | Correlated wildcard criteria: criteria over the same array are evaluated independently |
| G2 | Per-element universal quantification: "some reference resolves to nothing" is not expressible |
| G3 | Implicit defaults (namespace/kind) |
| G4 | String-encoded references (`group/version`, `ns/name`, FQDN, `name@provider`) |
| G5 | Key escaping for labels and annotations with `.` or `/` |
| G6 | Kind wildcards and categories |
| G7 | Selectors (`selects`/`selectedBy` not implemented) |
| G8 | Snapshot hygiene: minimum age, status-condition filter, stale-Smell garbage collection |
| G9 | Content inside templates and remote refs (CUE, Go templates, Helm values, OCI bundles) |

## 9. Threats to validity (as listed in the note)
- Seeded smells are biased toward detectable forms.
- Name-only matching under-reports.
- RBAC-restricted kinds are skipped silently.
- Results depend on the CRD versions used.
- Schema and semantics are taken from the paper, not from the code.

## 10. Open items the note asked to verify in the code
- The Pattern schema, path syntax and operators of the Go engine.
- Per-element correlation of `custom` criteria.
- Stale-Smell cleanup semantics.
- Support for multiple dependencies under `matchNone`.
- The RBAC scope of the chart.
- Every field marked "K" (from API knowledge).
