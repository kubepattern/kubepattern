# RQ6 plan: KubePattern vs Kyverno on relational orphan smells

Status: **done** (2026-09-29). Results, deviations and threats are in [`README.md`](README.md), section *RQ6: Comparison with Kyverno*.

**Scope decided by the user:** Kyverno only. kor, Popeye, Gatekeeper and Kubevious are out of scope.

## Question
> **RQ6.** How does KubePattern compare with a general-purpose policy engine (Kyverno) for relational orphan smells, on the same cluster and the same ground truth, in expressiveness, effectiveness, cost and operational behaviour?

Hypotheses. Each one is to be *measured*, not assumed; report whichever way it turns out.

| Id | Hypothesis |
|---|---|
| H1 | Kyverno can express the 14 smells through context lookups + JMESPath, but more verbosely and imperatively (more lines and expressions per smell) |
| H2 | With 1:1-equivalent semantics, Kyverno reaches the same precision/recall on the seeded scenarios |
| H3 | Kyverno's richer expression language overcomes some KubePattern limitation probes: G1 per-element filters, G3 `a \|\| b` defaults, G4 group checks, G10 booleans. It does not overcome G7 (selectors) or G8 (retained history objects) |
| H4 | Kyverno has a permanent footprint (always-on controllers) and informer-based reads (fewer API requests per cycle). KubePattern has zero idle footprint but LISTs everything every run |
| H5 | Neither tool re-evaluates a target when only its *dependencies* change. Detection latency after a dependency-side change is therefore bounded by the scan period in both |

## 0. Preparation
1. Start from the baseline:
   - `minikube start -p kp-eval`, then run the baseline check in `HANDOFF.md` §4;
   - keep the KubePattern CronJob **suspended** during the Kyverno measurements;
   - decide which engine to compare against: the fix `a814e4a` is installed; use it, and state it in the results.
2. Pin the latest stable Kyverno Helm chart in `env/versions.env` (`KYVERNO_CHART_VERSION`), and add `env/install/95-kyverno.sh`:
   - namespace `kyverno`;
   - admission webhooks are not needed (Audit only), but keep the default install for realism;
   - reports controller and background controller enabled.
3. **RBAC:** Kyverno's background and reports controllers must list the 15 CRD groups used by the Patterns (the list is in `env/values/rbac-without-kargo.yaml`, plus `kargo.akuity.io`). Add `comparison/kyverno/rbac.yaml` with aggregated ClusterRoles:
   - labels `rbac.kyverno.io/aggregate-to-background-controller: "true"` and `rbac.kyverno.io/aggregate-to-reports-controller: "true"`;
   - verbs `get/list/watch`;
   - verify the label names against the installed chart.
4. **Verify the APIs of the pinned version before writing policies** (`kubectl api-resources | grep -i kyverno`, `kubectl explain`):
   - `ClusterPolicy` (`kyverno.io/v1`, JMESPath) vs the newer CEL `ValidatingPolicy`. Primary: whichever is GA and supports background scans with cross-resource lookups;
   - `GlobalContextEntry` (informer-cached lists of a GVR);
   - the PolicyReport group (`wgpolicyk8s.io/v1alpha2` or `openreports.io/v1alpha1`);
   - the background-scan interval flag of the reports controller.

## 1. Policies
Layout:

```
comparison/kyverno/
  rbac.yaml
  globalcontext/*.yaml   # one GlobalContextEntry per dependency GVR (certificates, stages, helmreleases, ...)
  equivalent/*.yaml      # 14 policies, same names and 1:1 semantics as the KubePattern Patterns
  best-effort/*.yaml     # variants using Kyverno's extra power on the probe cases
  apicall/*.yaml         # optional: equivalent/ with context.apiCall instead of GlobalContextEntry (cost only)
```

- **Equivalent set.** Translate each `patterns/<platform>/*.yaml` literally:
  - same target kind and filters;
  - same dependency filters;
  - same criteria, including namespace criteria and the *uncorrelated* name/kind matching where KubePattern has it;
  - a `fail` result means "smell". Policy name = pattern name, so `score.py` can be reused.
- **Best-effort set.** Only for patterns with probes. Use per-element JMESPath filters (`[?name=='x' && kind=='y']`), `spec.chartRef.namespace || metadata.namespace`, group checks and boolean comparisons to fix the probe cases where possible.
- Validate every policy on the near-misses before scoring. If a policy cannot express a smell, record why instead of approximating it silently.

Sketch, to adapt to the verified API:

```yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: certmanager-clusterissuer-not-used
spec:
  background: true
  admission: false
  validationFailureAction: Audit
  rules:
    - name: clusterissuer-not-used
      match:
        any:
          - resources:
              kinds: ["cert-manager.io/v1/ClusterIssuer"]
      context:
        - name: certRefs
          globalReference:
            name: certificates           # GlobalContextEntry listing cert-manager.io/v1 certificates
            jmesPath: "[?spec.issuerRef.kind=='ClusterIssuer'].spec.issuerRef.name"
        - name: crRefs
          globalReference:
            name: certificaterequests
            jmesPath: "[?spec.issuerRef.kind=='ClusterIssuer'].spec.issuerRef.name"
      validate:
        message: "ClusterIssuer {{request.object.metadata.name}} is not used"
        deny:
          conditions:
            all:
              - key: "{{ request.object.metadata.name }}"
                operator: AnyNotIn
                value: "{{ certRefs }}"
              - key: "{{ request.object.metadata.name }}"
                operator: AnyNotIn
                value: "{{ crRefs }}"
```

Special cases:
- **`owns`** (`argocd-applicationset-generates-nothing`): select Applications whose `metadata.ownerReferences[?uid=='{{request.object.metadata.uid}}']` is non-empty.
- **`matchAll`** (`cnpg-pooler-name-clashes-with-cluster`): fail when *some* Cluster in the same namespace has the Pooler's name.

## 2. RQ6a Expressiveness
New script `scripts/policy_metrics.py`, mirroring `pattern_metrics.py`, writes `results/comparison/expressiveness.csv`. For each of the 14 smells and 10 probes:
- verdict (expressible / partial / not);
- non-comment lines of code, for Kyverno vs Pattern;
- number of context entries;
- number of JMESPath expressions;
- whether a GlobalContextEntry was needed.

Report which G-limitations Kyverno overcomes (H3), and which KubePattern features have no direct equivalent (e.g. `owns` transitivity, `emitOnEmpty`).

## 3. RQ6b Effectiveness
1. Apply the GlobalContextEntries and the `equivalent/` policies, then wait for a completed background scan. Make the reports settle before reading them, with a stable count across two polls.
2. New script `scripts/kyverno_reports_to_smells.py`: converts every `fail` result into a smells.json-shaped item (`spec.pattern.name` = policy, `spec.target` = kind/namespace/name) and writes it into `results/raw/kyverno/<run>/smells.json`.
3. Score it with `./scripts/score.py <file>` against the unchanged ground truth: precision/recall, unlisted findings, and probe outcomes.
4. Repeat 3 times. Force a re-scan between repetitions (reports-controller restart or policy re-apply; document which).
5. Do the same with `best-effort/`, which gives the probe outcomes for H3.
6. **Mutation testing:** apply the mutations of `scripts/mutate.sh` (factor out its apply/revert blocks, or copy them), wait for the next scan, and score the Kyverno reports: killed and restored.

## 4. RQ6c Cost
1. **Footprint:** sample `kubectl top pods -n kyverno` every 10 s for 30 min idle and across a background scan, giving median/peak CPU and memory per controller. For KubePattern, use the Job pod during a run (25–30 s) and **0 when idle**.
2. **API load:** extend `env/audit-policy.yaml` to the Kyverno ServiceAccounts in namespace `kyverno` (keep the KubePattern SA) and restart `kp-eval`. For one scan cycle and one hour idle, count requests by verb and resource (`audit_summary.py` needs a user filter parameter). Separate WATCH, which is long-lived, from LIST, GET and report writes.
3. **Report churn:** count the PolicyReport writes per cycle, against the Smell writes per KubePattern run.
4. **Optional scale:** the S1 subset (T ∈ {1000, 5000} Flux objects via `scale.py` helpers), measuring Kyverno scan time and memory against `results/scale/summary.csv`.

## 5. RQ6d Operational behaviour
- **Staleness / detection latency (H5).** Set the same period for both tools (e.g. KubePattern CronJob `*/5`, Kyverno background scan interval 5m). Apply a dependency-side change (delete a referrer, e.g. M02 or M08) at a random offset, and measure the time until the Smell appears and until the PolicyReport shows `fail`. Do 5 repetitions per tool, then the reverse: time until it disappears.
- **Robustness:**
  - policy on a CRD that is not installed: is it rejected at admission, or reported?
  - narrowed RBAC (remove `kargo.akuity.io` from the aggregated role): what happens to existing report results (kept, or turned into errors)?
- **Output model:** a short qualitative comparison, PolicyReport entries (per rule/resource, pass/fail/error) vs Smell CRs (per finding, with severity, category, message and a stable identity).

## 6. Outputs
- `results/comparison/{expressiveness,effectiveness,cost,staleness}.csv` and `results/tables/comparison.tex`.
- A `README.md` section **"RQ6: Comparison with Kyverno"**, plus one row in the headline table and one finding.
- Update `TEST-PLAN.md`: add RQ6 to §3, and the Kyverno version to §4.

## 7. Threats to validity (write them down)
- Policy-author skill bias: the policies are written by the same authors as the Patterns. Mitigation: 1:1 translation, near-miss validation, and the best-effort set reported separately.
- Kyverno version, and the choice between JMESPath and CEL.
- GlobalContextEntry vs apiCall changes the cost profile: report both if time allows.
- Scan scheduling semantics differ (continuous controller vs CronJob): compare latency at equal periods.
