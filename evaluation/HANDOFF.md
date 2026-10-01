# Handoff: KubePattern evaluation (RQ1–RQ6, Krateo case study, engine update and RQ8 done)

Read this first when you continue the work in a new session. RQ1–RQ5, the fix validation and the Kyverno comparison (RQ6, [`COMPARISON-PLAN.md`](COMPARISON-PLAN.md)) are done; results are in [`README.md`](README.md).

## 1. What this is
The user is writing a paper on **KubePattern**, a Go engine that runs as a CronJob. It evaluates declarative `Pattern` CRs (`kubepattern.dev/v1`) against the live cluster and writes `Smell` CRs.

`evaluation/` is the paper's Section 4 material:
- [`SOURCE-PLAN.md`](SOURCE-PLAN.md): the testing plan extracted from the user's research note.
- [`TEST-PLAN.md`](TEST-PLAN.md): the rewritten plan, with RQ1–RQ5, verified engine semantics, the limitation catalogue G1–G10, and defects D1–D9.
- [`README.md`](README.md): reproduction steps and **all results**, with headline table, per-RQ sections, fix validation, findings and deviations.
- `results/`: CSV files and `tables/*.tex` (booktabs), which go into the paper. Per-run archives are in `results/raw/` (git-ignored).

**Status: done.**

| Item | Result |
|---|---|
| RQ1 Expressiveness | 14 Patterns, 9 platforms, 0 engine changes; 49 candidate smells: 27 expressible, 10 partial, 12 not |
| RQ2 Effectiveness | P = R = 1.00 on 43 positives and 52 negatives (29 near-misses); 10/10 limitation probes as predicted; mutation 15/15; KubeVela catalogue 40/45 unused, 45/45 oracle agreement |
| RQ3 Complementarity | The platforms report 3/50 smells, and only as transient Events (0/50 after the 1 h TTL) |
| RQ4 Efficiency | Whole cluster: 23 s, 30 MB, 0.26 s CPU, 129 requests. About 97% client-side throttling (5 QPS), ceiling about 750 Smells per run, quadratic matching CPU |
| RQ5 CronJob mode | 9/9 scheduled snapshots as predicted; found a fail-open GC |
| Fix validation | Per-pattern prune on `fix/per-pattern-prune` @ `a814e4a`. Overlapping runs 12/46 → 46/46 Smells kept; the skipped pattern keeps its Smells; no flapping above the ceiling; no regression |
| RQ6 Kyverno 1.19.1 | CEL `ValidatingPolicy` 1:1: 105/105 verdicts equal (P = R = 1.00, probes 10/10 as predicted), mutation 15/15; best-effort resolves 10/10 probes; cost at 5-min period: 477 MiB always-on, ~50× CPU, ~23× API requests; staleness bounded by the period for both; robustness R1/R2 recorded |
| Engine update `54dc498` | QPS 50/100 + linter + `spec.for`: full regression unchanged (RQ2, mutation, D8, RQ5, Krateo); whole cluster 26.2 s → 0.85 s; 805 Smells in 30.5 s (ceiling gone) |
| RQ8 `spec.for` | 0 transient findings with `for` (vs 2 on kp-eval, 3 on Krateo); every persistent smell reported 2 periods later; Kyverno reports the transients |

All main results use the engine **as-is** (`dev` @ `69d4ffd`). The fix is reported as a separate before/after validation.

## 2. Repository state
- **Branch layout since 2026-10-01: everything is on `dev`** (pushed). The per-pattern prune, the engine update (`chore/engine-cleanup`, `feat/for`) and the evaluation material (`paper/evaluation`) were merged into `dev`, which now holds the engine, `evaluation/` and `docs/proposals/`. New work goes on feature branches from `dev`; merge them back into `dev`. The merged branches and `fix/per-pattern-prune` are kept on origin for history.
- **`operator` is set aside** (user decision, 2026-10-01): do not port or develop on it for now.
- `evaluation/` is committed (git-ignored runtime artefacts excluded). Commit further changes only when the user asks.
- Git-ignored: `evaluation/bin/` (binaries `kubepattern-69d4ffd`, `kubepattern-a814e4a`), `results/raw/`, `env/.kargo-admin`, `env/sa.kubeconfig`, `__pycache__/`.
- Engine tests: `internal/analysis` and `internal/kube` pass. `internal/linter` tests were already broken on `dev` (D4: fixtures lack `plural`); leave them alone unless asked.
- Go is **not installed** on the host. Use the container:
  ```bash
  podman run --rm -v "$PWD":/src:Z -v kp-gomod:/go/pkg/mod -w /src docker.io/library/golang:1.25.8-alpine go test ./internal/analysis/... ./internal/kube/...
  ```

## 3. Cluster
- **2026-10-01: `kp-eval` was re-created empty** by `minikube start` (profile config and audit policy kept, new IP). It was rebuilt with `scripts/rebuild-eval.sh` (about 13 min), and the as-is baseline reproduced exactly. The installed KubePattern is now **`eval-54dc498`** (QPS + linter + `spec.for`), CronJob suspended. Claude cannot run `minikube` (see the memory note): the user starts profiles and loads images (`minikube -p <profile> image load evaluation/bin/kubepattern-eval-<commit>.tar`); check with `kubectl get nodes -o json | jq '.items[].status.images[].names[]'`.
- **minikube profile `kp-eval`**:
  - kvm2 driver, docker runtime, 8 vCPU / 16 GiB, Kubernetes v1.34.4;
  - about 4 GiB used;
  - kubectl context `kp-eval`.
- The old profile `minikube` is stopped; do not delete it.
- **API-server audit log** (`env/audit-policy.yaml`, copied to `~/.minikube/files/etc/ssl/certs/`) logs **only** the KubePattern SA and, since RQ6, every ServiceAccount in namespace `kyverno`, to the apiserver's stdout. To change it, edit the policy, copy it again and restart the profile (`minikube stop -p kp-eval && minikube start -p kp-eval`).
- Installed, pinned in `env/versions.env`, one script each in `env/install/`:

  | Platform | Version |
  |---|---|
  | cert-manager | v1.21.2 |
  | CloudNativePG | 1.30.1 |
  | External Secrets | chart 2.11.0 |
  | Flux | v2.9.5 |
  | Argo CD | chart 10.9.2 |
  | Kargo | 1.11.4 |
  | Crossplane | 2.4.2 (`--enable-operations`) |
  | KubeVela | 1.11.0 |
  | Cluster API + CAPD | v1.14.2 |
  | Kyverno (RQ6 only, `95-kyverno.sh`) | chart 3.9.1 / v1.19.1, `backgroundScanInterval=5m` |

- Scenarios are applied (`scenarios/<platform>/`), together with the 14 Patterns (`patterns/<platform>/`).
- Kyverno state: `comparison/kyverno/rbac.yaml`, the 21 GlobalContextEntries and the 14 `equivalent/` policies are applied (background scan every 5 min, reports in `wgpolicyk8s.io`). To remove it: `helm -n kyverno uninstall kyverno` and delete `clusterrole kp-eval:kyverno-read-subjects`.
- KubePattern: Helm release `kubepattern`, namespace `kubepattern`. The installed image is currently **`localhost/kubepattern:eval-a814e4a` (the fix)**, CronJob **suspended**.
  - Reinstall the as-is engine with `KP_COMMIT=69d4ffd ./env/install/00-kubepattern.sh`, the fix with `KP_COMMIT=a814e4a ...`.
- After a host reboot, `minikube start -p kp-eval` may fail with a libvirt socket timeout. Wait a minute and retry: `virsh -c qemu:///system list` must answer first.

## 4. How to run things (from `evaluation/`)
| Task | Command |
|---|---|
| Baseline check (must print `43 52 43 0 0 52 1.00 1.00 10( 10) 0`, exit 0) | `out=$(./scripts/run-once.sh check \| tail -1); ./scripts/score.py "$out/smells.json"` |
| Re-apply scenarios / Patterns (idempotent) | `./scripts/apply-scenario.sh [platform]`, `./scripts/apply-patterns.sh [platform]` |
| One in-cluster run with archive (job, pod log, smells.json, audit.jsonl) | `./scripts/run-once.sh <label>` |
| One out-of-cluster measured run (wall, RSS, CPU, requests) | `./scripts/perf-local.sh <label>` |
| API request phases from an audit archive | `./scripts/audit_summary.py <audit.jsonl>` |
| Mutation testing (15 mutants) | `MUTATION_LABEL=... MUTATION_OUT=... ./scripts/mutate.sh` |
| CronJob timeline (RQ5) | `TIMELINE_LABEL=... ./scripts/cronjob-timeline.sh`, then `./scripts/timeline_report.py --label ... --gc global\|per-pattern` |
| Scale sweep / GC subset / restore | `./scripts/scale.py`, `./scripts/scale.py s2-subset`, `./scripts/scale.py cleanup` |
| LaTeX tables | `./scripts/make_tables.py`, `./scripts/gcfix_compare.py`, `./scripts/comparison_tables.py` (RQ6) |
| RQ6: one forced Kyverno scan, archived and converted to smells.json | `./scripts/kyverno-run.sh <label>` (restart of the reports controller + `kyverno-snapshot.sh`) |
| RQ6: effectiveness and expressiveness | `./scripts/comparison_effectiveness.py`, `./scripts/policy_metrics.py` |
| RQ6: mutation testing with Kyverno | `MUTATION_RUNNER=./scripts/kyverno-run.sh MUTATION_LABEL=... MUTATION_OUT=... ./scripts/mutate.sh` |
| RQ6: cost window and report | `footprint-sample.sh <dir>/footprint.tsv 3600 10` + `audit-harvest.sh <dir>/audit.jsonl 3600 120`, then `cost_report.py <dir> --kp-run <run-once dir> --kp-perf <perf-local line>` |
| RQ6: staleness (CronJob `*/5` unsuspended first) and robustness | `REPS=5 PERIOD=300 ./scripts/staleness.sh`, `./scripts/staleness_report.py <run>`, `./scripts/kyverno-robustness.sh` |

Ground truth: `scenarios/*/ground-truth.csv`, with columns `pattern,kind,namespace,name,truth,engine_expected,class,note`.
- Classes: `positive`, `negative`, `near-miss`, `probe-G*` (the documented limitation; `engine_expected` differs from `truth`), `blocked-upstream`.
- `score.py` takes any smells.json-shaped file, using `spec.pattern.name` and `spec.target.{kind,namespace,name}`. **Reuse it for Kyverno** by converting PolicyReports to that shape.

## 5. Pitfalls learned (important)
- **Never overlap two analysis runs** on the as-is engine. They garbage-collect each other's Smells (D8: 12/46 left). Keep the CronJob suspended and trigger runs one at a time.
- **Run long experiments under** `systemd-inhibit --what=sleep:idle:handle-lid-switch --mode=block <cmd>`. The laptop suspended once in the middle of an experiment.
- `pkill -f <script-name>` also kills the calling shell, because the pattern matches its own command line. Stop background jobs another way.
- `kubectl wait cluster/...` resolves to the CAPI `Cluster`. Use fully qualified names (`clusters.postgresql.cnpg.io/...`).
- The apiserver audit log goes to container stdout and the kubelet rotates it. **Harvest it right after every run** (`run-once.sh` and `perf-local.sh` already do).
- The `kubepattern` binary has **no flags**: running it (even with `--help`) performs a real analysis against the current kubeconfig.
- `scenarios/*/post.sh` and `pre.sh` must stay idempotent. `apply-scenario.sh` is re-run by the revert steps.
- Client-go throttling (5 QPS) dominates KubePattern's run time: about 0.4 s per Smell written.

## 6. User preferences
- Chat in **Italian**; documents, code and comments in **English**.
- The user wants a **plan to review before execution** for non-trivial work.
- **No commits or pushes** unless explicitly asked.
- Krateo is out of scope for this cluster (the user has separate Krateo experiments).
- The main results stay on the as-is engine; improvements are reported as separate validations.

## 7. Next steps
**Plan before submission (about one month, agreed on 2026-10-01):**
- **WP0:** cleanup and client QPS;
- **WP1:** element-scoped criteria (G1, `[@]`), validated with v2 Patterns on `kp-eval` and `kp-krateo`;
- **WP2:** `for` (P1 of `docs/proposals/for.md`) and RQ8;
- **paper track:** POSITIONING §5 and §6, Krateo section, threats.

Index, G2, discovery, readability study and the operator port come after the paper.

**Status (2026-10-01, end of day):**
- **WP0 done.** On `chore/engine-cleanup`:
  - `d4274e8`: client QPS/Burst, default 50/100;
  - `f5ab2be`: the linter rejects the unimplemented primitives; D4 fixed.
- **`spec.for` done** (WP2 brought forward). On `feat/for`:
  - `54dc498`: Smell `phase`/`since`, D3 fixed;
  - `2981a4a`: chart default for missing `analysis.client`.
- **Branch state:** all merged into `dev` (`c423610`).
- **Regression of `54dc498`:** `results/regression/54dc498/README.md`. Every correctness result is unchanged (RQ2, mutation, D8, RQ5, Krateo scored and lifecycle). Whole cluster 26.2 s → 0.85 s; the 805-Smell case goes from deadline hit to 30.5 s; the next limit is CPU (quadratic matching) at about 20k objects.
- **RQ8:** `results/rq8/README.md`, with hypotheses (a), (b) and (c) holding on Krateo and kp-eval. Tooling: `for/durations.csv` + `scripts/set-for.sh`; `score.py --active-only`.
- **Pitfalls found:**
  - `perf-local.sh` reused the SA kubeconfig of another profile (fixed: it now regenerates when the server changes);
  - waiting with `pgrep -f <script>` matches the waiting shell itself.
- **Next:** WP1 (G1, element-scoped criteria `[@]`), then the paper track. The registry patch for the `v1-0-10` pin is ready but not applied.

The earlier assessment (strengths, weaknesses, positioning) and the roadmap are in `README.md` → *Discussion and next steps*. Candidates, in the order recommended before the plan:
1. Element-scoped criteria (G1): removes silent false negatives.
2. Raise client QPS/Burst and index dependencies: removes the ~750-Smell ceiling and the quadratic matching.
3. Merge the per-pattern prune fix.
4. Prototype `for:` (stateful duration), the extension that Kyverno cannot replicate. Two more: `cycle`/reachability and discovery-based kind sets (examples in the README table).
5. One real-world dataset for external validity.
6. AI-assisted authoring with a natural-language `intent`: proposal in [`../docs/proposals/intent.md`](../docs/proposals/intent.md), with a candidate RQ7 (DSL vs CEL as an LLM generation target).

Any of these that changes the engine is reported as a separate validation; RQ1–RQ6 stay on the existing engines. Optional RQ6 extensions, not run: the `apicall/` cost variant and the Kyverno S1 scale sweep.

## 8. Krateo case study (2026-09-30, separate from RQ1–RQ6)
Everything is in [`krateo/README.md`](krateo/README.md): setup, blueprint, Patterns, Kyverno policies, results and findings.

**Question:** on a real Krateo platform, do KubePattern and Kyverno address the same types of smells?

**Answer:**
- **Static reference fields:** the two tools agree verdict for verdict, with P = R = 1.00 on 55 positives and 22/22 probes as predicted.
- **Kyverno best-effort:** it resolves 22/22 engine-limitation probes (G1, G2, G4, G5, G6, G7). Coverage by smell type: KubePattern has 2 types expressible, 9 partial and 2 not; Kyverno has 11 expressible, 1 partial and 1 not.
- **Shared limits:** neither tool models snowplow's dynamic references (G9), and both must enumerate the 24 widget kinds.
- **Lifecycle:** 26/26 changes seen by both tools, over 6 mutations done through Krateo.

**Cluster.**
- **Profile `kp-krateo`:** kvm2, 12 vCPU, 16 GiB, Kubernetes v1.34.4, context `kp-krateo`. Installed: Krateo 3.0.2, blueprint `springboot-app` with 2 compositions, KubePattern `69d4ffd` (CronJob suspended), Kyverno 1.19.1 with the 1:1 policies applied.
- **`kp-eval` is stopped** to free memory; its state is kept. Before using it again, run `minikube stop -p kp-krateo` and `minikube start -p kp-eval`.

**Scripts.** Every Krateo script sources `krateo/env/krateo.env`. The shared scripts follow it because `env/versions.env` now takes `KP_PROFILE` and the cluster size from the environment (defaults unchanged).

**Re-run:**
- all scored runs: `./krateo/scripts/final-runs.sh`
- report: `./krateo/scripts/krateo_compare.py`
- lifecycle experiment: `./krateo/scripts/lifecycle.sh && ./krateo/scripts/lifecycle_report.py`

**Pitfalls:**
- Composition `Ready=True` is also reported for a failed Helm release; check `helm list -a`.
- Recreating a composition renames the CDC's RoleBindings; regenerate the ground truth with `krateo_oracle.py truth <snapshot>`, then `check`.
- Kyverno keeps stale results when a policy's match is narrowed; delete the PolicyReports when switching policy sets (the scripts do).
- Do not run `helm` from `evaluation/krateo/`: `./kyverno` would be taken as a local chart.
