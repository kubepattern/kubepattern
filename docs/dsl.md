# Pattern DSL reference

A `Pattern` (`kubepattern.dev/v1`) declares a smell: the **target** it is about, the **dependencies** read to judge it, and the **relationships** the target is expected to have with them. A target that satisfies the relationships gets a `Smell`. This page describes every construct. The constructs marked *new* were added to resolve the limitation catalogue G1–G10 of the evaluation (`evaluation/TEST-PLAN.md` §9); Patterns that do not use them behave exactly as before (`evaluation/results/limits/`).

```yaml
apiVersion: kubepattern.dev/v1
kind: Pattern
metadata: {name: eso-secretstore-not-used}
spec:
  displayName: SecretStore Not Used
  category: Security            # free text
  severity: MEDIUM              # LOW | MEDIUM | HIGH | CRITICAL
  for: 1h                       # optional minimum duration (Pending -> Active)
  message: "SecretStore {{target.metadata.name}} is not used."
  reference: https://...        # optional
  target: {...}
  dependencies: [...]
  relationships: {matchAll: [...], matchAny: [...], matchNone: [...]}
```

## Paths
Paths address fields of a resource. Segments are separated by dots.

| Syntax | Meaning |
|---|---|
| `spec.secretStoreRef.name` | a nested field |
| `spec.data[*].name` | every element of an array; the values of different elements are **not** correlated |
| `spec.data[@].name` | *new*: the element **bound** by the evaluation (element scope, below) |
| `spec.data[0].name` | one element |
| `metadata.labels['krateo.io/managed-by']` | *new*: a key with dots or slashes (also `["..."]`) |

A path that reaches nothing yields no value: a criterion with no value on one side fails, a filter `EXISTS` is false and `IS_EMPTY` is true. The linter rejects malformed paths.

### Element scope `[@]` (new; G1, G2)
All paths of the same resource that share the prefix up to a `[@]` refer to the **same** array element. The engine enumerates the elements and evaluates one binding at a time.

- **Dependency side (G1).** In a relationship, a dependency matches when one of its elements satisfies all the criteria. With `[*]`, `spec.data[*].sourceRef.storeRef.name` and `...kind` could come from two different entries; with `[@]` they come from the same entry.
- **Target side (G2).** Target filters and all relationships share the target's bindings. The target is defective when the relationships hold for **at least one** element that passes the target filters. Under `matchNone` this reads "some reference resolves to nothing". A target whose anchored array is missing or empty is not a candidate.

```yaml
target:
  kind: RoleBinding
  apiVersion: rbac.authorization.k8s.io/v1
  plural: rolebindings
  filters:
    matchAll:                                    # examine ServiceAccount subjects only
      - {path: "subjects[@].kind", operator: EQUALS, values: [ServiceAccount]}
relationships:
  matchNone:                                     # defective if SOME of them does not exist
    - with: serviceaccount
      type: custom
      criteria:
        - {targetPath: "subjects[@].name", dependencyPath: metadata.name, operator: EQUALS}
        - {targetPath: "subjects[@].namespace", targetDefault: {path: metadata.namespace},
           dependencyPath: metadata.namespace, operator: EQUALS}
```

Rules: `[@]` is not allowed in dependency filters (dependencies are filtered before any binding) nor after a `[*]` in the same path. Nested anchors (`a[@].b[@]`) and anchors on two different arrays (cartesian product) are supported.

## Target and dependency resource types
| Form | Example | Meaning |
|---|---|---|
| one kind | `{kind: Secret, apiVersion: v1, plural: secrets}` | as before |
| *new* every kind of a group version (G6) | `{kind: "*", apiVersion: widgets.templates.krateo.io/v1beta1}` | the listable kinds found by discovery at run time |
| *new* a category (G6) | `{category: fluxcd-appliers}` | every kind whose discovery categories include it |
| *new* a list (G6) | `kinds: [{kind: Markdown, ...}, {kind: "*", ...}, {category: ...}]` | the union |
| *new* derived from the targets (G6, dependencies only) | `fromTarget: {apiVersionPath: status.apiVersion, kindPath: status.kind, pluralPath: status.resource}` | the kinds named by the targets' fields (`apiVersion:` instead of `apiVersionPath:` for a fixed group version). A kind the API server does not serve has no instances |

`{{target.kind}}` in the message names the kind of a multi-kind target. An unknown group version in a `kind: "*"` makes the Pattern skipped, as a missing CRD does; a category that matches nothing gives no candidates.

## Filters
A filter condition is `{path, operator, values}` on a single resource, grouped by `matchAll`, `matchAny` and `matchNone`.

| Operator | Values |
|---|---|
| `EXISTS`, `IS_EMPTY` | none |
| `EQUALS` | strings; *new*: booleans and numbers compare by their string form (G10: `{path: spec.suspend, operator: EQUALS, values: ["true"]}`) |
| `GREATER_THAN`, `GREATER_OR_EQUAL`, `LESS_THAN`, `LESS_OR_EQUAL` | one integer |
| `ARRAY_SIZE_EQUALS` and the other `ARRAY_SIZE_*` | one integer |

*New*: `valuesFrom: <path>` replaces `values` for `EQUALS` and compares with another field of the same resource, under the same binding (e.g. the menu entry whose `items[@].id` equals `widgetData.resourceRefId`).

## Relationships
Each entry names a dependency (`with`) and a `type`. It holds when **at least one** candidate of the dependency satisfies it. `matchAll`, `matchAny` and `matchNone` combine the entries.

| Type | Holds when |
|---|---|
| `custom` | all `criteria` hold |
| `owns` / `ownedBy` | the target transitively owns / is owned by the dependency (`ownerReferences`) |
| *new* `selects` (G7) | the target's label selector at `selectorPath` (default `spec.selector`) matches the dependency's labels, and the optional `criteria` hold |
| *new* `selectedBy` (G7) | the dependency's label selector at `selectorPath` matches the target's labels, and the optional `criteria` hold |

Selectors may be a `metav1.LabelSelector` (`matchLabels`, `matchExpressions`) or a plain label map as in `Service.spec.selector`. A missing selector selects nothing. An empty `{}` selects nothing for a field named `selector` (an empty Service selector) and everything for fields named `*Selector` (`podSelector: {}` in a NetworkPolicy, `labelSelector: {}`). Namespaces are not implied: add a namespace criterion when the selection is namespaced.

```yaml
- with: push                                   # a PushSecret selects stores by label
  type: selectedBy
  selectorPath: "spec.secretStoreRefs[@].labelSelector"
  criteria:
    - {targetPath: kind, dependencyPath: "spec.secretStoreRefs[@].kind",
       dependencyDefault: {value: SecretStore}, operator: EQUALS}
```

### Criteria
A criterion compares the target side with the dependency side with `EQUALS`: it holds when the two value sets intersect.

| Field | Meaning |
|---|---|
| `targetPath`, `dependencyPath` | the paths of each side |
| *new* `targetValue`, `dependencyValue` | a literal instead of a path (e.g. the bound entry must have `resource: pages`) |
| *new* `targetDefault`, `dependencyDefault` (G3) | `{path: ...}` or `{value: ...}`, used when the path yields nothing (e.g. `chartRef.namespace` defaults to the referrer's `metadata.namespace`; an omitted `kind` is `SecretStore`) |
| *new* `targetTransform`, `dependencyTransform` (G4) | one of `{apiGroup: true}` (`cert-manager.io/v1` → `cert-manager.io`), `{split: {separator: "@", index: 0}}` (negative index from the end), `{regex: "name=([^&]+)"}` (first capture group; no match drops the value), `{lowercase: true}`. Applied after the default |

## Limitations: status
| Id | Limitation | Status | Construct |
|---|---|---|---|
| G1 | criteria not correlated per array element | resolved | `[@]` on the dependency side |
| G2 | no per-element universal quantification | resolved | `[@]` on the target side |
| G3 | implicit defaults | resolved | `targetDefault` / `dependencyDefault` |
| G4 | string-encoded references | resolved | transforms (a short host to FQDN mapping that needs a conditional default is still partial) |
| G5 | key escaping | resolved | quoted keys |
| G6 | kind wildcards and categories | resolved | `kind: "*"`, `category`, `kinds`, `fromTarget` |
| G7 | selectors | resolved | `selects`, `selectedBy` |
| G8 | snapshot hygiene | transients: `spec.for`; history objects: a dependency filter (e.g. `metadata.ownerReferences IS_EMPTY`) | |
| G9 | content inside templates and remote references | open, out of scope | rendering CUE or Go templates, or fetching OCI bundles, is not an analysis of the cluster state |
| G10 | filters compare strings only | resolved | typed `EQUALS` |

Not implemented: the criteria operators `CONTAINS` and `LABEL_SELECTOR` (use transforms and `selects`/`selectedBy`); the linter rejects them. Paths are checked for syntax, not against the CRD schemas.
