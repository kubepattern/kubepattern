# Proposal: `for`, a stateful duration for Patterns

Status: **implemented and evaluated** (2026-10-01).
- **Engine:** branch `feat/for` @ `54dc498`, P1. One deviation: `T` is the process start time, captured in `main.go` and passed to the writer.
- **Evaluation:** RQ8 (P2) in `evaluation/results/rq8/README.md`; regression with `for` unset in `evaluation/results/regression/54dc498/README.md`.
- **Durations** for the evaluated Patterns: `evaluation/for/durations.csv` (P4 for the evaluation; the registry is still to do).
- **Still open:** P3 (operator mode, webhook warning).
Related: `evaluation/README.md` → *Discussion and next steps*, extension (a); RQ5 (transient finding during the template rotation); RQ6d (staleness); [`intent.md`](intent.md).

## 1. Motivation
KubePattern evaluates a snapshot. A relationship that does not hold *at the moment of the run* produces a Smell, even when it holds again a few minutes later. The evaluation recorded this:
- **RQ5, run 7.** During the two-step CAPI template rotation, the new template is flagged for exactly one run; the Smell is then garbage-collected.
- **GitOps moves.** A ClusterIssuer created a year ago is unreferenced for a few minutes while its only Certificate is deleted and re-created by Flux or Argo CD. An age filter on `creationTimestamp` does not help: the object is old, only the reference is new.
- **Krateo widgets.** The registry documentation of `widget-not-referenced` lists "widgets temporarily unreferenced during active development" as an accepted transient state. The Pattern cannot express it today.

Snapshot false positives are the practical objection to orphan detection ("it is just not wired yet") and the first source of alert fatigue. Alerting systems solve it with a minimum duration: a condition is reported only after it has held for a while. Prometheus calls it `for`, with the states `pending` and `firing`. This proposal uses the same word and the same semantics, with one naming difference: the second phase is `Active`, not `firing`, because a Smell is a finding that stays until it is fixed, not an alert that fires.

**Why it is a differentiator.** Kyverno's background scan is stateless: every result is re-evaluated from scratch at each interval (RQ6d), a scan inside the window reports `fail` and emits an Event, and a report records when a resource was evaluated, not since when it has failed. A CEL age filter hides only *new* objects. KubePattern already holds what state needs: one Smell per finding with a stable identity (`<pattern>-<targetUID>`), updated in place across runs, and a per-pattern prune that deletes it when the condition stops holding. `for` adds two fields to that object and **no API request**.

## 2. Semantics
`spec.for` is a duration (`30m`, `1h`, `24h`; default `0s`). A target that satisfies the relationships produces a Smell in one of two phases:

| Phase | Meaning |
|---|---|
| `Pending` | the condition has been observed, but for less than `for` |
| `Active` | the condition has held for at least `for` |

Rules, applied once per run with the run's start time `T`:
1. **First observation.** No Smell exists for the target: create it with `since = T` and `phase = Pending` (`Active` when `for` is `0s`).
2. **Later observations.** The Smell exists: keep its `since`; set `phase = Active` if `T − since ≥ for`, else `Pending`; update in place.
3. **Condition no longer holds.** The target does not satisfy the relationships: the per-pattern prune deletes the Smell, Pending or Active, as it does today. The next observation starts a new clock. There is no `Active → Pending` transition and no retained history.
4. **No observation.** A pattern skipped (missing CRD, RBAC) or a run cut by the deadline leaves its Smells untouched, as the per-pattern prune already guarantees, and the clock keeps running: an unobserved target is not a resolved target. A long gap can therefore turn a Pending Smell Active at the first successful run. This is how Prometheus behaves after an outage, and it is preferred to "N consecutive observations", which one skipped run would reset.
5. **Resolution.** The effective delay is `for` rounded up to the next run. A `for` shorter than the run period is accepted but has no effect. In CronJob mode the engine does not know the schedule, so the linter validates only the format; in operator mode `requeueInterval` is known and the webhook can warn.
6. **Clock.** `T` is the start of the run, not the wall time of each write. At 5 QPS a run of 750 Smells spans five minutes (RQ4); per-write timestamps would skew `since` by that much.

Changing `for` on a Pattern needs no migration: phases are recomputed from `since` at the next run. A Smell written by an engine without `since` uses its `metadata.creationTimestamp`.

## 3. DSL shape
The evaluated cert-manager Pattern (`evaluation/patterns/cert-manager/clusterissuer-not-used.yaml`) with the new field. Only `for` and the `{{smell.since}}` placeholder are new.

```yaml
apiVersion: kubepattern.dev/v1
kind: Pattern
metadata:
  name: certmanager-clusterissuer-not-used
spec:
  displayName: ClusterIssuer Not Used
  category: Security
  severity: LOW
  # The condition must hold for at least 1h before the Smell becomes Active.
  # Default 0s: today's behaviour (Active at the first observation).
  for: 1h
  message: "ClusterIssuer {{target.metadata.name}} has not been referenced by any Certificate or CertificateRequest since {{smell.since}}; it still holds signing material."
  reference: "https://cert-manager.io/docs/usage/certificate/"
  target:
    kind: ClusterIssuer
    apiVersion: cert-manager.io/v1
    plural: clusterissuers
  dependencies:
    - id: certificate
      kind: Certificate
      apiVersion: cert-manager.io/v1
      plural: certificates
      filters:
        matchAll:
          - path: "spec.issuerRef.kind"
            operator: EQUALS
            values: ["ClusterIssuer"]
    - id: request
      kind: CertificateRequest
      apiVersion: cert-manager.io/v1
      plural: certificaterequests
      filters:
        matchAll:
          - path: "spec.issuerRef.kind"
            operator: EQUALS
            values: ["ClusterIssuer"]
  relationships:
    matchNone:
      - with: certificate
        type: custom
        criteria:
          - targetPath: "metadata.name"
            dependencyPath: "spec.issuerRef.name"
            operator: EQUALS
      - with: request
        type: custom
        criteria:
          - targetPath: "metadata.name"
            dependencyPath: "spec.issuerRef.name"
            operator: EQUALS
```

## 4. Smell shape
Two new spec fields, one label for `kubectl` selectors, two printer columns.

```yaml
apiVersion: kubepattern.dev/v1
kind: Smell
metadata:
  name: certmanager-clusterissuer-not-used-7c1e...   # identity unchanged: <pattern>-<targetUID>
  namespace: kubepattern
  creationTimestamp: "2026-09-30T10:00:12Z"
  labels:
    kubepattern.dev/pattern-uid: "..."
    lastScan: "..."
    kubepattern.dev/phase: Pending                    # mirrors spec.phase; -l kubepattern.dev/phase=Active
spec:
  phase: Pending                                      # Pending | Active
  since: "2026-09-30T10:00:12Z"                       # first observation (run start), preserved across updates
  name: ClusterIssuer Not Used
  severity: LOW
  category: Security
  message: "ClusterIssuer ci-retired has not been referenced by any Certificate or CertificateRequest since 2026-09-30T10:00:12Z; it still holds signing material."
  reference: "https://cert-manager.io/docs/usage/certificate/"
  suppress: false
  pattern: {name: certmanager-clusterissuer-not-used, version: v1}
  target: {apiVersion: cert-manager.io/v1, kind: ClusterIssuer, name: ci-retired, uid: 7c1e...}
```

| Item | Change |
|---|---|
| Pattern CRD and linter | `spec.for`, duration string, default `0s`; validated with `time.ParseDuration` |
| Smell CRD | `spec.phase` (enum `Pending`, `Active`; default `Active` so older Smells read correctly), `spec.since` (date-time); printer columns `PHASE` and `SINCE`; label `kubepattern.dev/phase` |
| Message placeholders | `{{smell.since}}` (RFC 3339) and `{{smell.phase}}`, next to the five existing `target.*` placeholders |
| `kubectl` | `kubectl get smells -A` shows the phase; `-l kubepattern.dev/phase=Active` keeps dashboards and alerts on confirmed findings |

Pending Smells are written, not kept in memory or in a side object: writing them reuses the identity, the update-in-place and the prune, and gives the "unused since" timestamp for free. The alternative, a candidate list in a ConfigMap, would duplicate the lifecycle and lose the stable identity.

## 5. Engine changes
Small, and confined to the run and the writer:
- `Engine.Run` receives the run start `T` and the pattern's `for`, and passes `since`/`phase` to the writer. The synthetic Smell of an absence pattern (`emitOnEmpty`) follows the same rules: it has a deterministic name too.
- `SmellWriter.Write` already does a `GET` before every `CREATE`/`UPDATE` (`internal/kube/report.go`). On update it copies `spec.since` from the existing object and computes the phase. **Cost: zero extra requests**; the fields ride the writes that happen today.
- The same "preserve from the existing object" step fixes **D3** (`suppress` reset to `false` on every update): `suppress` is copied the same way, in the same change.
- `Prune` and `PruneOrphans` are unchanged. Overlapping runs (D8) are already handled by the per-pattern prune; both runs preserve `since`.
- Unit tests in `internal/analysis/engine_test.go` and `internal/kube/report_test.go` with an injected clock: first observation, promotion at the threshold, prune on resolution, `for: 0s`, and a Smell without `since`.
- Operator mode (`operator` branch): `PatternReconciler.Reconcile` runs the same engine on a `requeueInterval` cadence; `T` is the reconcile start. The validating webhook can reject or warn when `for` is shorter than the interval.

## 6. Example, run by run
CronJob every 30 minutes, `for: 1h`. Two ClusterIssuers: `ci-retired`, really unused, and `ci-moving`, whose only Certificate is deleted and re-created by GitOps between 10:20 and 10:40.

| Run | Time | `ci-retired` | `ci-moving` |
|---|---|---|---|
| 1 | 10:00 | created, `Pending`, since 10:00 | no Smell (the Certificate exists) |
| 2 | 10:30 | `Pending`, held for 30 min | created, `Pending`, since 10:30 |
| 3 | 11:00 | **`Active`**, held for 60 min | Certificate back: Smell pruned |
| 4 | 11:30 | `Active`, updated in place | no Smell, clock reset |

Kyverno at the same period writes `fail` and emits a PolicyViolation Event for `ci-moving` at 10:30, and has no "since" to show for `ci-retired`.

## 7. Which Patterns get a `for`
`for` belongs to Patterns whose target can legitimately be unreferenced for a while. Suggested defaults for the evaluated and registry Patterns; they are authoring choices, to be stated in each Pattern's documentation.

| Pattern | Suggested `for` | Why |
|---|---|---|
| Krateo `page-`, `paragraph-`, `panel-not-referenced` (registry) | `1h` | widgets wired after creation during development |
| `capi-machinetemplate-orphan` | `24h` | template rotations (RQ5, run 7) |
| `flux-ocirepository-not-used`, `eso-*store-not-used`, `certmanager-clusterissuer-not-used` | `1h` | GitOps delete-and-recreate of the referrer |
| `kubevela-*definition-not-used`, `crossplane-function-not-used` | `24h` | catalogue entries are published before their first consumer |
| `cnpg-pooler-name-clashes-with-cluster`, `argocd-appproject-empty` | `0s` | a misconfiguration is a smell at the first observation |

## 8. Research angle: candidate RQ8
> **RQ8.** Does a minimum duration remove snapshot false positives without hiding persistent smells, and at what cost in detection latency?

The existing assets make this cheap to measure (`evaluation/scripts/`):
- **Transients.** Re-run the RQ5 timeline (`cronjob-timeline.sh`, period 2 min) with `for: 5m` on the CAPI Pattern. Expected: the rotation step 1 (run 7) produces a Pending Smell and never an Active one; the Active set after run 8 equals the set after run 6.
- **GitOps move.** New scenario step: delete and re-create the Certificate of a live ClusterIssuer with a gap of one run. Expected: Pending for one run, then pruned; zero Active Smells.
- **Latency.** The staleness protocol (`staleness.sh`, `REPS=5 PERIOD=300`) with `for: 5m` and `for: 10m`. Expected: appear latency grows by exactly `for` rounded up to the next run; disappear latency unchanged.
- **Baseline.** Kyverno at the same period (`kyverno-run.sh`, `kyverno-snapshot.sh`): every transient is reported as `fail` with an Event.
- **Cost.** Requests and writes per run from the audit log (`audit_summary.py`): expected unchanged, since a Pending Smell costs the same writes as a Smell today.

Metrics: transient Active Smells per run, Pending Smells per run, appear and disappear latency, writes per run.

Hypotheses:
- (a) with `for ≥ 2 × period`, no transient of the RQ5 timeline and of the GitOps-move scenario becomes Active, and every seeded persistent smell (RQ2) still does;
- (b) appear latency increases by `for` plus at most one period; disappear latency is unchanged;
- (c) Kyverno reports every transient at the same period, and the only suppression it offers, an age filter on the object, does not cover the old-object case.

If (a) and (b) hold, "beyond Kyverno" is a measured claim: the same precision and recall on persistent smells, zero transient findings, at a declared latency cost. It also answers the RQ6d trade-off explicitly: `for` trades latency for precision.

## 9. Risks and open questions
| Risk | Mitigation |
|---|---|
| A `for` longer than the operator's lifetime of a legitimate transient hides nothing; a `for` too long delays real findings | Suggested per-Pattern defaults (§7); `kubectl get smells` shows Pending Smells, so nothing is invisible |
| Clock gaps (rule 4) promote a Pending Smell without an observation in between | Documented; the Smell carries `since`, and the alternative resets on every skipped run |
| Clock skew between runs on different nodes | `T` is the process start time of the run; a skew of seconds is irrelevant at periods of minutes |
| Users filter only Active Smells and forget Pending ones | printer column and label; a Pending Smell is still a Smell in the list |

Open questions:
- Should a per-target override exist (an annotation on the target such as `kubepattern.dev/for: 0s`)? Not in the first version.
- Should the Smell keep an `activeSince` next to `since`? It is derivable (`since + for`), so no.
- Should `for` be honoured by the `suppress` logic (a suppressed Smell never becomes Active)? Suppression already hides the finding; the phase stays informative.

## 10. Phases
1. **P1: engine and CRDs.** `spec.for`, `spec.phase`, `spec.since`, placeholders, D3 fix, unit tests. CronJob mode.
2. **P2: evaluation.** RQ8 with the assets above; one row in the headline table and one finding in `evaluation/README.md`.
3. **P3: operator mode.** `T` from the reconcile start; webhook warning when `for` is shorter than `requeueInterval`.
4. **P4: registry.** `for` on the Krateo widget Patterns and on the evaluated Patterns, with the rationale of §7 in their documentation.

**Non-goals:** a `Resolved` phase or any history after the condition clears (the prune deletes); per-target overrides; any change to the prune; any LLM involvement.
