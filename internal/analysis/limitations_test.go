package analysis

// One test per DSL limitation of the evaluation's catalogue (G1-G10): each reproduces the
// probe in memory and checks that the new construct gives the true verdict.

import (
	"context"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	"kubepattern-go/internal/cluster"
	"kubepattern-go/internal/linter"
)

// run lints a pattern, builds a graph from YAML objects (uids are generated), applies an
// optional run-time setup (what main resolves through discovery) and returns the names of
// the targets that get a Smell, sorted.
func run(t *testing.T, patternYAML, objectsYAML string, setup func(*linter.PatternAsCode)) []string {
	t.Helper()
	p, err := linter.Lint([]byte(patternYAML))
	if err != nil {
		t.Fatalf("lint: %v", err)
	}
	p.Metadata.UID = "pattern-uid"
	if setup != nil {
		setup(p)
	}
	var objs []unstructured.Unstructured
	dec := yaml.NewDecoder(strings.NewReader(objectsYAML))
	for i := 0; ; i++ {
		var m map[string]any
		if err := dec.Decode(&m); err != nil {
			break
		}
		if m == nil {
			continue
		}
		u := unstructured.Unstructured{Object: m}
		if u.GetUID() == "" {
			u.SetUID(types.UID("uid-" + u.GetKind() + "-" + u.GetNamespace() + "-" + u.GetName()))
		}
		objs = append(objs, u)
	}
	g := cluster.NewGraph()
	g.Build(objs)
	w := &fakeWriter{}
	if err := NewEngine(g, w).Run(context.Background(), p); err != nil {
		t.Fatalf("run: %v", err)
	}
	var names []string
	for _, s := range w.written {
		names = append(names, s.Target.Name)
	}
	sort.Strings(names)
	return names
}

func expectSmells(t *testing.T, got []string, want ...string) {
	t.Helper()
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("smells = %v, want %v", got, want)
	}
}

const header = `apiVersion: kubepattern.dev/v1
kind: Pattern
metadata: {name: p}
spec:
  displayName: P
  category: Test
  severity: LOW
  message: "{{target.metadata.name}}"
`

// G1: criteria over the same array are correlated per element with [@].
func TestG1ElementScopedCriteria(t *testing.T) {
	objects := `
apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
kind: DockerMachineTemplate
metadata: {name: t1, namespace: a}
---
apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
kind: DockerMachineTemplate
metadata: {name: t2, namespace: a}
---
apiVersion: cluster.x-k8s.io/v1beta2
kind: ClusterClass
metadata: {name: cc1, namespace: a}
spec:
  workers:
    machineDeployments:
      - infrastructure: {templateRef: {name: t1, kind: DevMachineTemplate}}
      - infrastructure: {templateRef: {name: t2, kind: DockerMachineTemplate}}
`
	pattern := func(anchor string) string {
		return header + `  target: {kind: DockerMachineTemplate, apiVersion: infrastructure.cluster.x-k8s.io/v1beta2, plural: dockermachinetemplates}
  dependencies:
    - {id: cc, kind: ClusterClass, apiVersion: cluster.x-k8s.io/v1beta2, plural: clusterclasses}
  relationships:
    matchNone:
      - with: cc
        type: custom
        criteria:
          - {targetPath: metadata.name, dependencyPath: "spec.workers.machineDeployments` + anchor + `.infrastructure.templateRef.name", operator: EQUALS}
          - {targetPath: kind, dependencyPath: "spec.workers.machineDeployments` + anchor + `.infrastructure.templateRef.kind", operator: EQUALS}
`
	}
	// [*]: name of entry 0 and kind of entry 1 combine, t1 looks used (the documented FN).
	expectSmells(t, run(t, pattern("[*]"), objects, nil))
	// [@]: name and kind come from the same entry, t1 is an orphan.
	expectSmells(t, run(t, pattern("[@]"), objects, nil), "t1")
}

// G2: a target-side [@] makes the pattern hold when SOME element is defective
// ("some reference resolves to nothing"); target filters select the elements.
func TestG2SomeElementIsDangling(t *testing.T) {
	pattern := header + `  target:
    kind: RoleBinding
    apiVersion: rbac.authorization.k8s.io/v1
    plural: rolebindings
    filters:
      matchAll:
        - {path: "subjects[@].kind", operator: EQUALS, values: [ServiceAccount]}
  dependencies:
    - {id: sa, kind: ServiceAccount, apiVersion: v1, plural: serviceaccounts}
  relationships:
    matchNone:
      - with: sa
        type: custom
        criteria:
          - {targetPath: "subjects[@].name", dependencyPath: metadata.name, operator: EQUALS}
          - targetPath: "subjects[@].namespace"
            targetDefault: {path: metadata.namespace}
            dependencyPath: metadata.namespace
            operator: EQUALS
`
	objects := `
apiVersion: v1
kind: ServiceAccount
metadata: {name: agent, namespace: a}
---
apiVersion: v1
kind: ServiceAccount
metadata: {name: builder, namespace: a}
---
# every subject exists (one without namespace: it defaults to the binding's namespace)
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata: {name: rb-ok, namespace: a}
subjects:
  - {kind: ServiceAccount, name: agent, namespace: a}
  - {kind: ServiceAccount, name: builder}
---
# one of several ServiceAccounts is missing
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata: {name: rb-mixed, namespace: a}
subjects:
  - {kind: ServiceAccount, name: agent, namespace: a}
  - {kind: ServiceAccount, name: ghost, namespace: a}
---
# a Group named like a missing ServiceAccount is not a ServiceAccount subject
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata: {name: rb-group, namespace: a}
subjects:
  - {kind: ServiceAccount, name: agent, namespace: a}
  - {kind: Group, name: ghost}
---
# no ServiceAccount subject at all: not a target
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata: {name: rb-users, namespace: a}
subjects:
  - {kind: User, name: alice}
`
	expectSmells(t, run(t, pattern, objects, nil), "rb-mixed")
}

// G3: a default (path or literal) applies when the reference does not set the field.
func TestG3Defaults(t *testing.T) {
	pattern := header + `  target: {kind: OCIRepository, apiVersion: source.toolkit.fluxcd.io/v1, plural: ocirepositories}
  dependencies:
    - {id: hr, kind: HelmRelease, apiVersion: helm.toolkit.fluxcd.io/v2, plural: helmreleases}
  relationships:
    matchNone:
      - with: hr
        type: custom
        criteria:
          - {targetPath: metadata.name, dependencyPath: spec.chartRef.name, operator: EQUALS}
          - {targetPath: kind, dependencyPath: spec.chartRef.kind, dependencyDefault: {value: OCIRepository}, operator: EQUALS}
          - targetPath: metadata.namespace
            dependencyPath: spec.chartRef.namespace
            dependencyDefault: {path: metadata.namespace}
            operator: EQUALS
`
	objects := `
apiVersion: source.toolkit.fluxcd.io/v1
kind: OCIRepository
metadata: {name: local, namespace: a}
---
apiVersion: source.toolkit.fluxcd.io/v1
kind: OCIRepository
metadata: {name: cross-ns, namespace: a}
---
apiVersion: source.toolkit.fluxcd.io/v1
kind: OCIRepository
metadata: {name: unused, namespace: a}
---
apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata: {name: hr1, namespace: a}
spec: {chartRef: {name: local}}
---
apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata: {name: hr2, namespace: b}
spec: {chartRef: {name: cross-ns, kind: OCIRepository, namespace: a}}
---
apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata: {name: hr3, namespace: b}
spec: {chartRef: {name: unused, kind: OCIRepository}}
`
	expectSmells(t, run(t, pattern, objects, nil), "unused")
}

// G4: transforms decode string-encoded references.
func TestG4Transforms(t *testing.T) {
	issuers := header + `  target: {kind: ClusterIssuer, apiVersion: cert-manager.io/v1, plural: clusterissuers}
  dependencies:
    - {id: cert, kind: Certificate, apiVersion: cert-manager.io/v1, plural: certificates}
  relationships:
    matchNone:
      - with: cert
        type: custom
        criteria:
          - {targetPath: metadata.name, dependencyPath: spec.issuerRef.name, operator: EQUALS}
          - targetPath: apiVersion
            targetTransform: {apiGroup: true}
            dependencyPath: spec.issuerRef.group
            dependencyDefault: {value: cert-manager.io}
            operator: EQUALS
`
	objects := `
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata: {name: ci-default}
---
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata: {name: ci-grouped}
---
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata: {name: ci-foreign}
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata: {name: c1, namespace: a}
spec: {issuerRef: {name: ci-default}}
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata: {name: c2, namespace: a}
spec: {issuerRef: {name: ci-grouped, group: cert-manager.io}}
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata: {name: c3, namespace: a}
spec: {issuerRef: {name: ci-foreign, group: example.com}}
`
	expectSmells(t, run(t, issuers, objects, nil), "ci-foreign")

	definitions := header + `  target: {kind: ComponentDefinition, apiVersion: core.oam.dev/v1beta1, plural: componentdefinitions}
  dependencies:
    - {id: app, kind: Application, apiVersion: core.oam.dev/v1beta1, plural: applications}
  relationships:
    matchNone:
      - with: app
        type: custom
        criteria:
          - targetPath: metadata.name
            dependencyPath: "spec.components[*].type"
            dependencyTransform: {split: {separator: "@", index: 0}}
            operator: EQUALS
`
	objects = `
apiVersion: core.oam.dev/v1beta1
kind: ComponentDefinition
metadata: {name: versioned, namespace: a}
---
apiVersion: core.oam.dev/v1beta1
kind: ComponentDefinition
metadata: {name: unused, namespace: a}
---
apiVersion: core.oam.dev/v1beta1
kind: Application
metadata: {name: app, namespace: a}
spec: {components: [{name: x, type: versioned@v1}]}
`
	expectSmells(t, run(t, definitions, objects, nil), "unused")

	calls := header + `  target: {kind: RESTAction, apiVersion: templates.krateo.io/v1, plural: restactions}
  dependencies:
    - {id: caller, kind: RESTAction, apiVersion: templates.krateo.io/v1, plural: restactions}
  relationships:
    matchNone:
      - with: caller
        type: custom
        criteria:
          - targetPath: metadata.name
            dependencyPath: "spec.api[@].path"
            dependencyTransform: {regex: "resource=restactions&name=([^&]+)"}
            operator: EQUALS
          - targetPath: metadata.namespace
            dependencyPath: "spec.api[@].path"
            dependencyTransform: {regex: "&namespace=([^&]+)"}
            operator: EQUALS
`
	objects = `
apiVersion: templates.krateo.io/v1
kind: RESTAction
metadata: {name: called, namespace: a}
---
apiVersion: templates.krateo.io/v1
kind: RESTAction
metadata: {name: caller, namespace: a}
spec:
  api:
    - {name: x, path: "/call?resource=restactions&name=called&namespace=a"}
    - {name: y, path: "/call?resource=restactions&name=caller&namespace=b"}
`
	// caller is named by entry y, but for namespace b: correlated, so not a use.
	expectSmells(t, run(t, calls, objects, nil), "caller")
}

// G5: label and annotation keys with dots and slashes are quoted in paths.
func TestG5QuotedKeys(t *testing.T) {
	pattern := header + `  target:
    kind: RESTAction
    apiVersion: templates.krateo.io/v1
    plural: restactions
    filters:
      matchAll:
        - {path: "metadata.labels['krateo.io/managed-by']", operator: EXISTS}
  dependencies:
    - {id: ra, kind: RESTAction, apiVersion: templates.krateo.io/v1, plural: restactions}
  relationships:
    matchNone:
      - with: ra
        type: custom
        criteria:
          - {targetPath: "metadata.labels['krateo.io/managed-by']", dependencyPath: metadata.name, operator: EQUALS}
`
	objects := `
apiVersion: templates.krateo.io/v1
kind: RESTAction
metadata: {name: generator, namespace: a}
---
apiVersion: templates.krateo.io/v1
kind: RESTAction
metadata: {name: managed, namespace: a, labels: {krateo.io/managed-by: generator}}
---
apiVersion: templates.krateo.io/v1
kind: RESTAction
metadata: {name: orphaned, namespace: a, labels: {krateo.io/managed-by: gone}}
`
	expectSmells(t, run(t, pattern, objects, nil), "orphaned")
}

// G6: kind wildcards, kind lists, categories and kinds derived from the target.
func TestG6KindSets(t *testing.T) {
	objects := `
apiVersion: templates.krateo.io/v1
kind: RESTAction
metadata: {name: used-by-viewer, namespace: a}
---
apiVersion: templates.krateo.io/v1
kind: RESTAction
metadata: {name: used-by-table, namespace: a}
---
apiVersion: templates.krateo.io/v1
kind: RESTAction
metadata: {name: unused, namespace: a}
---
apiVersion: widgets.templates.krateo.io/v1beta1
kind: YamlViewer
metadata: {name: v, namespace: a}
spec: {apiRef: {name: used-by-viewer, namespace: a}}
---
apiVersion: widgets.templates.krateo.io/v1beta1
kind: Table
metadata: {name: t, namespace: a}
spec: {apiRef: {name: used-by-table, namespace: a}}
---
apiVersion: other.example.com/v1
kind: Thing
metadata: {name: x, namespace: a}
spec: {apiRef: {name: unused, namespace: a}}
`
	wildcard := header + `  target: {kind: RESTAction, apiVersion: templates.krateo.io/v1, plural: restactions}
  dependencies:
    - {id: widget, kind: "*", apiVersion: widgets.templates.krateo.io/v1beta1}
  relationships:
    matchNone:
      - with: widget
        type: custom
        criteria:
          - {targetPath: metadata.name, dependencyPath: spec.apiRef.name, operator: EQUALS}
          - {targetPath: metadata.namespace, dependencyPath: spec.apiRef.namespace, operator: EQUALS}
`
	// The Thing of another group is not a widget.
	expectSmells(t, run(t, wildcard, objects, nil), "unused")

	category := strings.Replace(wildcard, `{id: widget, kind: "*", apiVersion: widgets.templates.krateo.io/v1beta1}`, `{id: widget, category: krateo-widgets}`, 1)
	resolveCategory := func(p *linter.PatternAsCode) {
		// What discovery returns for the category in main.
		p.Spec.Dependencies[0].Resolved = linter.KindSet{}
		p.Spec.Dependencies[0].Resolved.Add("widgets.templates.krateo.io/v1beta1", "YamlViewer")
		p.Spec.Dependencies[0].Resolved.Add("widgets.templates.krateo.io/v1beta1", "Table")
	}
	expectSmells(t, run(t, category, objects, resolveCategory), "unused")

	// Several target kinds in one pattern, with the kind in the message.
	targets := header + `  target:
    kinds:
      - {kind: "*", apiVersion: widgets.templates.krateo.io/v1beta1}
      - {kind: Thing, apiVersion: other.example.com/v1, plural: things}
  dependencies:
    - {id: ra, kind: RESTAction, apiVersion: templates.krateo.io/v1, plural: restactions}
  relationships:
    matchAll:
      - with: ra
        type: custom
        criteria:
          - {targetPath: spec.apiRef.name, dependencyPath: metadata.name, operator: EQUALS}
`
	expectSmells(t, run(t, targets, objects, nil), "t", "v", "x")

	// Instances of the kind a definition generates (fromTarget).
	derived := header + `  target: {kind: CompositionDefinition, apiVersion: core.krateo.io/v1alpha1, plural: compositiondefinitions}
  dependencies:
    - id: composition
      fromTarget: {apiVersionPath: status.apiVersion, kindPath: status.kind}
  relationships:
    matchNone:
      - with: composition
        type: custom
        criteria:
          - {targetPath: status.kind, dependencyPath: kind, operator: EQUALS}
          - {targetPath: status.apiVersion, dependencyPath: apiVersion, operator: EQUALS}
`
	defs := `
apiVersion: core.krateo.io/v1alpha1
kind: CompositionDefinition
metadata: {name: used, namespace: a}
status: {apiVersion: composition.krateo.io/v1-0-0, kind: SpringbootApp}
---
apiVersion: core.krateo.io/v1alpha1
kind: CompositionDefinition
metadata: {name: unused, namespace: a}
status: {apiVersion: composition.krateo.io/v1-2-0, kind: PortalPage}
---
apiVersion: composition.krateo.io/v1-0-0
kind: SpringbootApp
metadata: {name: demo, namespace: t}
`
	resolveDerived := func(p *linter.PatternAsCode) {
		p.Spec.Dependencies[0].Resolved = linter.KindSet{}
		p.Spec.Dependencies[0].Resolved.Add("composition.krateo.io/v1-0-0", "SpringbootApp")
		p.Spec.Dependencies[0].Resolved.Add("composition.krateo.io/v1-2-0", "PortalPage")
	}
	expectSmells(t, run(t, derived, defs, resolveDerived), "unused")
}

// G7: selects / selectedBy evaluate label selectors (label maps and LabelSelectors).
func TestG7Selectors(t *testing.T) {
	services := header + `  target: {kind: Service, apiVersion: v1, plural: services}
  dependencies:
    - {id: pod, kind: Pod, apiVersion: v1, plural: pods}
  relationships:
    matchNone:
      - with: pod
        type: selects
        criteria:
          - {targetPath: metadata.namespace, dependencyPath: metadata.namespace, operator: EQUALS}
`
	objects := `
apiVersion: v1
kind: Pod
metadata: {name: web-1, namespace: a, labels: {app: web, tier: front}}
---
apiVersion: v1
kind: Service
metadata: {name: web, namespace: a}
spec: {selector: {app: web}}
---
apiVersion: v1
kind: Service
metadata: {name: stale, namespace: a}
spec: {selector: {app: gone}}
---
apiVersion: v1
kind: Service
metadata: {name: other-ns, namespace: b}
spec: {selector: {app: web}}
---
# no selector: selects nothing
apiVersion: v1
kind: Service
metadata: {name: external, namespace: a}
spec: {type: ExternalName}
`
	expectSmells(t, run(t, services, objects, nil), "external", "other-ns", "stale")

	stores := header + `  target: {kind: SecretStore, apiVersion: external-secrets.io/v1, plural: secretstores}
  dependencies:
    - {id: push, kind: PushSecret, apiVersion: external-secrets.io/v1alpha1, plural: pushsecrets}
  relationships:
    matchNone:
      - with: push
        type: selectedBy
        selectorPath: "spec.secretStoreRefs[@].labelSelector"
        criteria:
          - {targetPath: kind, dependencyPath: "spec.secretStoreRefs[@].kind", dependencyDefault: {value: SecretStore}, operator: EQUALS}
`
	objects = `
apiVersion: external-secrets.io/v1
kind: SecretStore
metadata: {name: selected, namespace: a, labels: {tier: push}}
---
apiVersion: external-secrets.io/v1
kind: SecretStore
metadata: {name: expr, namespace: a, labels: {env: prod}}
---
apiVersion: external-secrets.io/v1
kind: SecretStore
metadata: {name: wrong-kind, namespace: a, labels: {tier: cluster}}
---
apiVersion: external-secrets.io/v1
kind: SecretStore
metadata: {name: unselected, namespace: a, labels: {tier: none}}
---
apiVersion: external-secrets.io/v1alpha1
kind: PushSecret
metadata: {name: p, namespace: a}
spec:
  secretStoreRefs:
    - labelSelector: {matchLabels: {tier: push}}
    - kind: SecretStore
      labelSelector: {matchExpressions: [{key: env, operator: In, values: [prod, staging]}]}
    - kind: ClusterSecretStore
      labelSelector: {matchLabels: {tier: cluster}}
`
	expectSmells(t, run(t, stores, objects, nil), "unselected", "wrong-kind")
}

// G8 (history objects): a dependency filter keeps hand-made objects only.
func TestG8OwnerlessDependencies(t *testing.T) {
	pattern := header + `  target: {kind: ClusterIssuer, apiVersion: cert-manager.io/v1, plural: clusterissuers}
  dependencies:
    - id: request
      kind: CertificateRequest
      apiVersion: cert-manager.io/v1
      plural: certificaterequests
      filters:
        matchAll:
          - {path: metadata.ownerReferences, operator: IS_EMPTY}
  relationships:
    matchNone:
      - with: request
        type: custom
        criteria:
          - {targetPath: metadata.name, dependencyPath: spec.issuerRef.name, operator: EQUALS}
`
	objects := `
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata: {name: ci-manual}
---
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata: {name: ci-old-ca}
---
apiVersion: cert-manager.io/v1
kind: CertificateRequest
metadata: {name: manual, namespace: a}
spec: {issuerRef: {name: ci-manual}}
---
apiVersion: cert-manager.io/v1
kind: CertificateRequest
metadata:
  name: api-1
  namespace: a
  ownerReferences: [{apiVersion: cert-manager.io/v1, kind: Certificate, name: api, uid: cert-uid}]
spec: {issuerRef: {name: ci-old-ca}}
`
	expectSmells(t, run(t, pattern, objects, nil), "ci-old-ca")
}

// G10: filters compare booleans and numbers by their string form; valuesFrom compares two
// fields of the same resource.
func TestG10TypedFiltersAndValuesFrom(t *testing.T) {
	pattern := header + `  target: {kind: Cluster, apiVersion: postgresql.cnpg.io/v1, plural: clusters}
  dependencies:
    - id: schedule
      kind: ScheduledBackup
      apiVersion: postgresql.cnpg.io/v1
      plural: scheduledbackups
      filters:
        matchNone:
          - {path: spec.suspend, operator: EQUALS, values: ["true"]}
  relationships:
    matchNone:
      - with: schedule
        type: custom
        criteria:
          - {targetPath: metadata.name, dependencyPath: spec.cluster.name, operator: EQUALS}
`
	objects := `
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata: {name: db-live, namespace: a}
---
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata: {name: db-suspended, namespace: a}
---
apiVersion: postgresql.cnpg.io/v1
kind: ScheduledBackup
metadata: {name: s1, namespace: a}
spec: {cluster: {name: db-live}, suspend: false}
---
apiVersion: postgresql.cnpg.io/v1
kind: ScheduledBackup
metadata: {name: s2, namespace: a}
spec: {cluster: {name: db-suspended}, suspend: true}
`
	expectSmells(t, run(t, pattern, objects, nil), "db-suspended")

	// The menu entry that the item opens (items[@].id == widgetData.resourceRefId) must
	// name an existing Page.
	nav := header + `  target:
    kind: NavMenuItem
    apiVersion: widgets.templates.krateo.io/v1beta1
    plural: navmenuitems
    filters:
      matchAll:
        - {path: "spec.resourcesRefs.items[@].id", operator: EQUALS, valuesFrom: spec.widgetData.resourceRefId}
  dependencies:
    - {id: page, kind: Page, apiVersion: widgets.templates.krateo.io/v1beta1, plural: pages}
  relationships:
    matchNone:
      - with: page
        type: custom
        criteria:
          - {targetPath: "spec.resourcesRefs.items[@].name", dependencyPath: metadata.name, operator: EQUALS}
          - {targetPath: "spec.resourcesRefs.items[@].namespace", dependencyPath: metadata.namespace, operator: EQUALS}
`
	objects = `
apiVersion: widgets.templates.krateo.io/v1beta1
kind: Page
metadata: {name: home, namespace: a}
---
apiVersion: widgets.templates.krateo.io/v1beta1
kind: NavMenuItem
metadata: {name: nav-ok, namespace: a}
spec:
  widgetData: {resourceRefId: open}
  resourcesRefs: {items: [{id: open, name: home, namespace: a}]}
---
apiVersion: widgets.templates.krateo.io/v1beta1
kind: NavMenuItem
metadata: {name: nav-two, namespace: a}
spec:
  widgetData: {resourceRefId: open}
  resourcesRefs: {items: [{id: open, name: missing, namespace: a}, {id: other, name: home, namespace: a}]}
`
	expectSmells(t, run(t, nav, objects, nil), "nav-two")
}
