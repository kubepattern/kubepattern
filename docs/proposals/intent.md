# Proposal: natural-language `intent` for Patterns (AI-assisted authoring)

Status: **proposal** (2026-09-30). No code yet.
Related: `evaluation/README.md` → *Discussion and next steps*, and the RQ6 comparison with Kyverno.

## 1. Motivation
The evaluation (RQ1–RQ6) shows that KubePattern detects exactly what its Patterns say, and that most risk sits in **writing** the Patterns:
- **G1 correlation.** Criteria on the same array are not correlated per element. A Pattern that looks right can silently miss smells (CAPI `t1`, ESO `ss-g1`).
- **Silent path errors.** A mistyped path yields an empty value set, not an error. The evaluation needed an external script (`evaluation/scripts/verify_fields.py`) to check 41 paths against the live CRD schemas.
- **Knowing where references live.** Each ecosystem hides its references in different fields (`spec.chartRef`, `spec.data[*].sourceRef.storeRef`, `spec.workers.machineDeployments[*].infrastructure.templateRef`, …).
- **The Kyverno comparison.** CEL is more expressive than the DSL, but harder to validate: optional chaining turns a typo into a silent default.

An `intent` field lets the author state *what* should be detected in plain language. A tool then drafts the *how*, as the usual declarative spec, and checks it against the cluster.

## 2. What `intent` solves, and what it does not
It solves:
- **Authoring cost and discoverability.** The generator finds kinds, plurals, versions and reference fields from discovery and the CRD schemas.
- **Documentation.** The intent stays in the Pattern as the *why*. `displayName` and `message` describe the finding; `intent` describes the detection logic.
- **Review.** A reviewer compares the intent, the generated spec, a plain-language explanation of the spec, and the objects it matches today.

It does **not** add expressiveness. A generator can only emit what the DSL can express. What it *can* do is **diagnose the gap**: map the intent to the limitation catalogue (G1–G10) and report it, instead of approximating silently. For example:

> Intent: "SecretStores that no ExternalSecret entry of kind SecretStore references."
> Draft: per-entry name match. **Gap G1**: name and kind cannot be correlated on the same `data[]` entry, so an entry that names this store with kind ClusterSecretStore also counts as a use (possible false negative).

## 3. Operating modes
**Invariant for every mode:**
- the engine always evaluates a declarative, validated spec;
- the LLM never decides a single Smell;
- the analysis run stays deterministic between spec changes.

| Mode | When the AI acts | Trade-offs |
|---|---|---|
| **A. Authoring** | `kubepattern draft` generates a draft; a person reviews it and applies it (commit or `kubectl apply`) | Deterministic. No LLM at run time and zero idle cost, as today. The review is a manual step |
| **B. Runtime regeneration** (experimental, opt-in) | The engine or a controller re-drafts when the intent or the CRD schemas the spec uses change (a hash of intent + schemas). The new spec replaces the old one only if every deterministic gate passes: lint, paths, examples, bounded dry-run diff. Otherwise it falls back to mode C | Fully automatic. Not deterministic across changes. An LLM call and cluster metadata leave the cluster at each change, and the cluster needs API credentials |
| **C. Authoring + change proposals** (target) | A controller watches intents, CRD schema changes and Stale specs. It opens a pull request with the new draft on the GitOps repository that holds the Pattern, and never mutates live Patterns | Automatic *and* reviewed; the most credible mode. The hardest to build: Git provider integration, credentials, PR de-duplication, mapping a Pattern to its file |

**Positioning against Kyverno.** A small, schema-checkable DSL is an artefact that an AI can generate *and* a person can review line by line. The plain-language explanation and the gap diagnosis rely on the DSL's verified semantics (10/10 probes predicted, 105/105 verdicts cross-validated). Before any claim about Kyverno's own AI tooling goes into the paper, it must be verified.

## 4. DSL shape
```yaml
apiVersion: kubepattern.dev/v1
kind: Pattern
metadata:
  name: certmanager-clusterissuer-not-used
  annotations:
    kubepattern.dev/intent-hash: "sha256:…"          # intent the approved spec was drafted from
    kubepattern.dev/approved-spec-hash: "sha256:…"   # hash of target + dependencies + relationships
    kubepattern.dev/drafted-by: "kubepattern-draft/v0.1 claude-opus-5-5"
spec:
  displayName: ClusterIssuer Not Used
  category: Security
  severity: LOW
  message: "ClusterIssuer {{target.metadata.name}} is not referenced by any Certificate or CertificateRequest."
  intent: |
    Report ClusterIssuers that no Certificate or CertificateRequest references by name
    with kind ClusterIssuer. A reference without kind means a namespaced Issuer.
  examples:                     # optional; they ground the generation and become a regression check
    smell: [{kind: ClusterIssuer, name: ci-retired}]
    clean: [{kind: ClusterIssuer, name: ci-manual}]
  target: …                     # generated
  dependencies: …               # generated
  relationships: …              # generated
```

State, derived from the annotations:
- **Draft.** No spec, or a spec whose hash differs from `approved-spec-hash`. The engine skips it and logs the skip, as it does for a missing CRD.
- **Approved.** The spec hash matches. The engine evaluates it.
- **Stale.** The intent hash differs from `intent-hash`. The engine keeps evaluating the last approved spec and logs a warning; mode C opens a PR.

## 5. The iterative loop ("man mano")
`kubepattern draft --intent <file|text> [--examples …] [--context <kubeconfig>]` runs a tool-use loop. Claude proposes; deterministic Go tools check.

| Tool | What it does | Reuses |
|---|---|---|
| `list_kinds(filter)` | kinds, groups, versions and plurals from discovery | engine RESTMapper / discovery |
| `explain(kind, path)` | the CRD OpenAPI schema of a path (type, description, required) | — (like `kubectl explain`) |
| `sample_objects(kind, n)` | a few objects, **redacted** to metadata and reference-like fields; never Secret data; can be disabled | dynamic client |
| `lint(pattern)` | static validation | `internal/linter.Lint` |
| `check_paths(pattern)` | every path exists in the live CRD schemas | port of `evaluation/scripts/verify_fields.py` |
| `dry_run(pattern)` | runs the engine in memory on a snapshot and returns the matches; writes no Smells | `internal/analysis` |
| `check_examples(pattern)` | the `smell`/`clean` examples are classified as stated | like `evaluation/scripts/score.py` |

The loop:
1. Claude reads the intent and examples, explores kinds and schemas, and emits a draft spec.
2. The tools report lint errors, unknown paths, dry-run matches and failed examples.
3. Claude repairs the draft.
4. The loop stops when lint, paths and examples all pass, or when the intent needs a missing feature, in which case the gap is reported (§2).

Output: the Pattern YAML, a plain-language explanation of the spec, the list of limitations used or hit, and the dry-run matches. The person then reviews and applies it (mode A) or merges the PR (mode C).

**Model and API settings** (Anthropic Go SDK `anthropic-sdk-go`, which matches the engine's language):
- `claude-opus-5-5` with adaptive thinking.
- Tools defined with `strict: true` (schema-valid inputs) and driven by the SDK tool runner.
- The final spec returned through structured outputs (`output_config.format`) against the Pattern JSON schema.
- `tool_choice` stays `auto`; this model rejects forced tool choice, so the prompt names the tools to use.

## 6. Risks and mitigations
| Risk | Mitigation |
|---|---|
| Hallucinated kinds or paths | `list_kinds`, `explain`, `check_paths`, `lint`; a draft with unknown paths never leaves the loop |
| Plausible but wrong semantics (e.g. uncorrelated criteria) | Examples + dry-run matches + a plain-language explanation; the G1–G10 catalogue in the system prompt so gaps are named |
| Non-determinism | The artefact is the spec, not the prompt; approval is by spec hash; the engine never calls the LLM (modes A and C) |
| Cluster data leaving the cluster | Schemas and discovery first; object samples are redacted, opt-in, and exclude Secrets; mode B is opt-in |
| Prompt injection through cluster content (annotations, descriptions) | Tool results are treated as data; no auto-apply outside the deterministic gates (mode B) or review (modes A, C) |
| Cost and latency | Only at authoring or change time, never per analysis run; to be measured in RQ7 |

## 7. Research angle: candidate RQ7
> **RQ7.** Is a constrained declarative DSL a more reliable target for LLM generation than a general policy language (CEL)?

The existing evaluation assets make this cheap to measure:
- **Inputs.** The 14 smells as natural-language intents, written from `displayName`, `message` and the reference, ideally by someone who did not write the Patterns. Optionally one `smell` and one `clean` example each.
- **Arms.**
  - KubePattern draft loop (§5).
  - The same loop generating Kyverno `ValidatingPolicy` + GlobalContextEntries, with equivalent tools (admission dry-run, background scan, `kyverno-snapshot.sh`).
  - An ablation: single-shot generation without tools, for both targets.
  - N = 5 samples per intent and arm.
- **Scoring.** The unchanged ground truth, with `score.py` (both targets produce smells.json-shaped output).
- **Metrics.**
  - valid on first attempt;
  - iterations to converge;
  - precision and recall;
  - **silent errors** (drafts that pass their own checks but are wrong on the ground truth);
  - handling of the 10 probes (gap reported, correct approximation, or silent approximation);
  - size of the human review;
  - tokens and cost.
- **Hypotheses.**
  - (a) The DSL arm has fewer silent errors and higher first-attempt validity.
  - (b) The CEL arm resolves more probes (expressiveness, as in RQ6).
  - (c) The validation tools matter more than the target language.

If (a) holds, the DSL's smaller expressiveness becomes a **trust** argument: it is the safer target for generated rules.

## 8. Phases
Each phase reuses the previous one:
1. **P1: mode A.** Offline `kubepattern draft` CLI and the RQ7 harness. No CRD change; the output is a normal Pattern.
2. **P2.** `spec.intent` and `spec.examples` in the CRD and linter, provenance annotations, and the Draft/Approved/Stale handling in the engine.
3. **P3: mode C.** A controller that opens PRs on the GitOps repository, with de-duplication by Pattern and intent hash, and mapping from a Pattern to its file (source annotation or GitOps tracking labels).
4. **P4: mode B.** An opt-in experiment behind the same gates, falling back to mode C when a gate fails.

**Non-goals:** the LLM deciding Smells; LLM calls inside the analysis run in modes A and C; applying a spec without deterministic gates or review.

## 9. Open questions
- Where drafts live in mode C (same repo as the Patterns, a dedicated branch naming scheme) and which Git providers to support first.
- Whether `examples` should become part of the regular engine run, as a self-check that raises a warning when an approved Pattern stops classifying its own examples.
- Air-gapped clusters: authoring on a workstation with schemas exported from the cluster, and no object samples.
