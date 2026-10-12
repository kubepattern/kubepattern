# Pattern DSL reference

A `Pattern` (`kubepattern.dev/v1`) declares a smell: the **target** it is about, the **dependencies** read to judge it, and the **relationships** the target is expected to have with them. A target that satisfies the relationships gets a `Smell`. This page describes every construct.

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
| `spec.data[@].name` | the element **bound** by the evaluation (element scope, below) |
| `spec.data[0].name` | one element |
| `metadata.labels['krateo.io/managed-by']` | a key with dots or slashes (also `["..."]`) |

A path that reaches nothing yields no value: a criterion with no value on one side fails, a filter `EXISTS` is false and `IS_EMPTY` is true. The linter rejects malformed paths.

### Element scope `[@]`
All paths of the same resource that share the prefix up to a `[@]` refer to the **same** array element. The engine enumerates the elements and evaluates one binding at a time.

- **Dependency side.** In a relationship, a dependency matches when one of its elements satisfies all the criteria. With `[*]`, `spec.data[*].sourceRef.storeRef.name` and `...kind` could come from two different entries; with `[@]` they come from the same entry.
- **Target side.** Target filters and all relationships share the target's bindings. The target is defective when the relationships hold for **at least one** element that passes the target filters. Under `matchNone` this reads "some reference resolves to nothing". A target whose anchored array is missing or empty is not a candidate.

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

Rules, enforced by the linter:
- `[@]` is not allowed in dependency filters (dependencies are filtered before any binding) nor after a `[*]` in the same path.
- The anchors of one side must form **a single nested chain**: `spec.rules[@]` and `spec.rules[@].refs[@]` together are fine, `spec.data[@]` and `spec.dataFrom[@]` together are rejected. Independent arrays would bind their cartesian product, which is empty as soon as one of them is: the target would never be examined, or the dependency would never match. The target side covers the target filters and every relationship; the dependency side is checked per relationship, so two relationships may anchor different arrays of the same dependency (e.g. `data[@]` and `dataFrom[@]` of an ExternalSecret).
- A target whose anchored array is missing or empty has no binding and is not a candidate. Under `matchNone` this reads "some reference resolves to nothing": a RoleBinding without subjects has no dangling subject.
- When the target side has anchors, **every relationship must read the innermost anchored element** (through a target path or a target default). A relationship that ignores it (another field of the target, or `owns` / `ownedBy`) would never be evaluated for a target without the element; put it in its own Pattern.

## Target and dependency resource types
| Form | Example | Meaning |
|---|---|---|
| one kind | `{kind: Secret, apiVersion: v1, plural: secrets}` | that kind |
| every kind of a group version | `{kind: "*", apiVersion: widgets.templates.krateo.io/v1beta1}` | the listable kinds found by discovery at run time |
| a category | `{category: fluxcd-appliers}` | every kind whose discovery categories include it |
| a list | `kinds: [{kind: Markdown, ...}, {kind: "*", ...}, {category: ...}]` | the union |
| derived from the targets (dependencies only) | `fromTarget: {apiVersionPath: status.apiVersion, kindPath: status.kind, pluralPath: status.resource}` | the kinds named by the target candidates' fields (the target filters apply; `apiVersion:` instead of `apiVersionPath:` for a fixed group version). With `[@]` in the paths (`spec.generates[@].kind`) each element names one kind, read from that element only; `[*]` is rejected. A kind the API server does not serve has no instances |

`{{target.kind}}` in the message names the kind of a multi-kind target. Categories resolve to the server's preferred version of each kind: as for any kind read at two versions by two Patterns, read a kind at one version only. An unknown group version in a `kind: "*"` makes the Pattern skipped, as a missing CRD does; a category that matches nothing gives no candidates.

## Filters
A filter condition is `{path, operator, values}` on a single resource, grouped by `matchAll`, `matchAny` and `matchNone`.

| Operator | Values |
|---|---|
| `EXISTS`, `IS_EMPTY` | none |
| `EQUALS` | strings; booleans and numbers compare by their string form (e.g. `{path: spec.suspend, operator: EQUALS, values: ["true"]}`) |
| `GREATER_THAN`, `GREATER_OR_EQUAL`, `LESS_THAN`, `LESS_OR_EQUAL` | one integer |
| `ARRAY_SIZE_EQUALS` and the other `ARRAY_SIZE_*` | one integer |

`valuesFrom: <path>` replaces `values` for `EQUALS` and compares with another field of the same resource, under the same binding (e.g. the menu entry whose `items[@].id` equals `widgetData.resourceRefId`).

## Relationships
Each entry names a dependency (`with`) and a `type`. It holds when **at least one** candidate of the dependency satisfies it. `matchAll`, `matchAny` and `matchNone` combine the entries.

| Type | Holds when |
|---|---|
| `custom` | all `criteria` hold |
| `owns` / `ownedBy` | the target transitively owns / is owned by the dependency (`ownerReferences`) |
| `selects` | the target's label selector at `selectorPath` (default `spec.selector`) matches the dependency's labels, and the optional `criteria` hold |
| `selectedBy` | the dependency's label selector at `selectorPath` matches the target's labels, and the optional `criteria` hold |

Selectors may be a `metav1.LabelSelector` (`matchLabels`, `matchExpressions`) or a plain label map as in `Service.spec.selector`. A missing or invalid selector selects nothing (an invalid one is logged). Namespaces are not implied: add a namespace criterion when the selection is namespaced.

A selector without requirements selects what `emptySelector` says (`all` or `none`). When it is unset, the API conventions apply: a LabelSelector with empty `matchLabels`/`matchExpressions`, or an empty `{}` in a field named `*Selector` (`podSelector: {}` in a NetworkPolicy, `labelSelector: {}`), selects everything; an empty `{}` in any other field selects nothing (an empty Service selector). Set `emptySelector: all` where the API differs, e.g. a PodDisruptionBudget, whose `spec.selector: {}` selects every Pod of its namespace.

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
| `targetValue`, `dependencyValue` | a literal instead of a path (e.g. the bound entry must have `resource: pages`) |
| `targetDefault`, `dependencyDefault` | `{path: ...}` or `{value: ...}`, used when the path yields nothing (e.g. `chartRef.namespace` defaults to the referrer's `metadata.namespace`; an omitted `kind` is `SecretStore`). Not allowed with a `[*]` path, where it would apply only when no element has a value: use `[@]` to default each element. The default path itself cannot contain `[*]` |
| `targetTransform`, `dependencyTransform` | one of `{apiGroup: true}` (`cert-manager.io/v1` → `cert-manager.io`), `{split: {separator: "@", index: 0}}` (negative index from the end), `{regex: "name=([^&]+)"}` (first capture group; no match drops the value), `{lowercase: true}`. Applied after the default |

## Not implemented
The criteria operators `CONTAINS` and `LABEL_SELECTOR` are not implemented, and the linter rejects them: use transforms and `selects`/`selectedBy` instead. Paths are checked for syntax, not against the CRD schemas.
