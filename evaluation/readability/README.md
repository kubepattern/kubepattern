# Readability survey: KubePattern vs Kyverno

This folder holds the material for a readability survey. Each of the 5 examples is one smell written twice: as a KubePattern `Pattern` and as a Kyverno 1.19 `ValidatingPolicy`. The questions measure comprehension (right or wrong answers, confidence, time) and then stated preference.

[`../POSITIONING.md`](../POSITIONING.md) §2 explains why the paper cannot claim today that the DSL is "more readable than CEL": readability was never measured, and by size CEL wins. This survey is one way to measure it (§4 there).

## Layout
```
README.md          this file (English)
primer.md          one-page introduction to both syntaxes, given to participants (Italian)
form-A.md          participant form A (Italian), generated
form-B.md          participant form B (Italian), generated; mirror of A
answer-key.md      answers, scoring rubric and ground-truth provenance (English), generated; not for participants
build_forms.py     regenerates the two forms and the answer key, and checks every answer against the ground truth
items.csv          the 19 objects asked about, with the correct answer
rubric.csv         expected answer and key elements for the open question of each example
examples/E<n>-<slug>/
  pattern.yaml, policy.yaml                  realistic versions, deployable (Part 2)
  pattern.neutral.yaml, policy.neutral.yaml  neutral names and messages (Part 1)
  globalcontext.yaml                         the GlobalContextEntries the policy needs
  state.yaml                                 the cluster state shown to participants
  context.it.md                              two-line domain context (Italian)
```

## Examples
The pairs were not written for the survey. They are the Patterns evaluated in RQ1–RQ2 (`../patterns/`) and their hand-written 1:1 Kyverno translations from RQ6 (`../comparison/kyverno/equivalent/`). The two versions agree on all 105 verdicts of the evaluation (RQ6b), so their equivalence was measured on a live cluster. It is not just asserted.

| Id | Smell | What it exercises | Lines: Pattern / policy (+ GCE) |
|---|---|---|---|
| E1 | CNPG Cluster without a ScheduledBackup | 1 dependency, 2 criteria, `matchNone` | 30 / 25 (+9) |
| E2 | CNPG Pooler named like a Cluster | `matchAll`: the smell is a presence; CEL inverts the polarity (`!exists`) | 30 / 25 (+9) |
| E3 | Argo CD ApplicationSet that owns no Application | `owns` vs an `ownerReferences` comprehension | 23 / 25 (+9) |
| E4 | cert-manager ClusterIssuer not used | 2 dependencies; dependency filters vs optional chaining | 47 / 29 (+18) |
| E5 | Crossplane Function not used | 4 dependencies, `[*]` wildcards, nested paths | 57 / 37 (+36) |

The line counts are non-blank YAML lines of the realistic versions. In the evaluation, each GlobalContextEntry (GCE) is shared by every policy that reads that kind.

**Normalization** (applied to both sides):
- Realistic versions are the evaluated files, with the eval-only labels and all comments removed.
- Neutral versions also replace every text that gives the answer away:
  - Pattern: `metadata.name: rule-e<n>`, `displayName: Rule E<n>`, `category: General`, `severity: MEDIUM`, `message: "Finding on {{target.metadata.name}}"`, and `reference` removed. The first four fields and `message` cannot be dropped, because the linter requires them (`internal/linter/pattern.go`, `lintSpec`).
  - Policy: `metadata.name: rule-e<n>` and `messageExpression: "'Finding on ' + object.metadata.name"`.
- The rule logic is never changed.

**Cluster states.** `state.yaml` holds objects from `../scenarios/<platform>/{base,seeds}.yaml`, trimmed to their identifying fields and to the fields the rules read. The form tells participants that these are the only objects of the kinds involved. In E3, the Applications carry the `ownerReferences` that the Argo CD controller sets, with illustrative UIDs.

## Design
- **Part 0, background:** experience with Kubernetes, Kyverno/CEL and KubePattern, and familiarity with the four platforms.
- **Part 1, comprehension.** For each of E1→E5, the participant sees the neutral rule in **one** syntax and the cluster state. They then answer:
  - an open question: what does the rule report?
  - "reported: yes/no" for each target object (3–5 per example, 19 in total, 10 yes and 9 no);
  - their confidence (1–5) and the minutes spent.
- **Counterbalancing.** Form A shows E1, E3 and E5 as KubePattern and E2 and E4 as Kyverno. Form B is the mirror. So every example is read in both syntaxes across the two forms, and nobody reads the same rule twice before answering.
- **Part 2, side by side.** For each example, the participant sees the realistic pair and rates, for each syntax, ease of understanding and ease of modification (1–5). They then give a preference and optional comments. Form A shows KubePattern first; Form B shows Kyverno first. Part 2 comes after Part 1, so that seeing both versions cannot help the comprehension answers.
- **Part 3:** overall preference, with free comments.
- Items were chosen among the scenario's positives, negatives and near-misses. Several test one detail: a backup in another namespace (E1), a mistyped name (E1), a same-name Cluster in another namespace (E2), a reference without `kind` (E4), and a Function used only in the second pipeline step or only by a CronOperation (E5). Limitation probes (`probe-G*`) are excluded.

## Running it
1. Alternate the forms between participants (A, B, A, B, …). Give each participant `primer.md` and one form. Never give out `answer-key.md`.
2. The participant reads the primer, then fills in the form in order, without other tools (no web, no AI assistants, no cluster). Expect 30–40 minutes.
3. To move the forms into a survey tool, copy the YAML blocks as code or screenshots. Question ids (B, E, C, G) are stable across both forms.

To change a text or an item, edit `examples/*/context.it.md`, `items.csv` or `rubric.csv`, then run `python3 build_forms.py`. The script refuses to build if an answer disagrees with `engine_expected` in `../scenarios/*/ground-truth.csv`, or if an item is a limitation probe.

## Scoring
- **Accuracy:** for each item, correct or incorrect. Compare accuracy per syntax within each example (across forms) and per participant (across examples).
- **Open question:** score it against `rubric.csv`. It counts as correct when it names the target kind, the dependency kinds, the matching condition (e.g. name *and* namespace) and the polarity (reported when none matches, or when one matches).
- **Effort:** confidence and minutes per example, by syntax.
- **Preference:** Part 2 ratings are paired within participant, so compare them with a paired test (e.g. Wilcoxon signed-rank); use Part 3 for the overall preference.
- With one or two participants the results are a **pilot**: report them as anecdotal, and use them to fix the material. A claim in the paper needs about 10 or more participants per form, and the analysis should account for experience (Part 0).

## Verification (done when the material was created)
- `build_forms.py` checks all 19 answers against the ground truth at every build.
- A one-off script (not kept in the repo) checked:
  - every realistic file equals its evaluated source once the eval labels are removed, and every neutral file differs from it only in the neutralized fields;
  - each `globalcontext.yaml` equals the source GlobalContextEntries and covers exactly the kinds its policy reads;
  - an independent Python evaluator of the Pattern semantics, run on each `state.yaml`, reports exactly the items marked "yes" in `items.csv`. Every target object in a snapshot is an item;
  - both forms contain every item, no answers, the neutral rules in Part 1, and each example once per syntax across A and B.
- Not done: `kubectl apply --dry-run=server` on the files, because the `kp-eval` cluster was stopped. The logic is unchanged from the files that ran in RQ2 and RQ6.

## Threats to validity
- **Author bias:** the same authors wrote both versions, the scenarios and the questions. It is mitigated because the pairs come from the evaluation unchanged and are equivalent on 105/105 verdicts.
- **Participants:** friends of the author, and a small sample. Their prior experience with CEL or Kubernetes can dominate the results, which is why Part 0 records it.
- **Primer:** it is equally long for both syntaxes, but its wording can favour one of them.
- **Ownership in E3:** KubePattern's `owns` is transitive, while the CEL translation checks direct ownership only. They are equivalent here, because ApplicationSets own their Applications directly.
- **Typing effort not measured:** the GlobalContextEntries are shown in the primer and not in every pair. Adding a dependency in Kyverno also requires a new entry, and Part 2's "ease of modification" only partly captures this.
