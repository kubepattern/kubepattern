package linter

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"kubepattern-go/internal/fieldpath"
)

// -------------------------
// Types
// -------------------------

type RelationshipType string

const (
	RelationshipCustom     RelationshipType = "custom"
	RelationshipOwns       RelationshipType = "owns"
	RelationshipOwnedBy    RelationshipType = "ownedBy"
	RelationshipSelects    RelationshipType = "selects"
	RelationshipSelectedBy RelationshipType = "selectedBy"
)

type Severity string

const (
	SeverityLow      Severity = "LOW"
	SeverityMedium   Severity = "MEDIUM"
	SeverityHigh     Severity = "HIGH"
	SeverityCritical Severity = "CRITICAL"
)

type FilterOperator string

const (
	FilterEquals                  FilterOperator = "EQUALS"
	FilterIsEmpty                 FilterOperator = "IS_EMPTY"
	FilterExists                  FilterOperator = "EXISTS"
	FilterGreaterThan             FilterOperator = "GREATER_THAN"
	FilterGreaterOrEqual          FilterOperator = "GREATER_OR_EQUAL"
	FilterLessThan                FilterOperator = "LESS_THAN"
	FilterLessOrEqual             FilterOperator = "LESS_OR_EQUAL"
	FilterArraySizeEquals         FilterOperator = "ARRAY_SIZE_EQUALS"
	FilterArraySizeGreaterThan    FilterOperator = "ARRAY_SIZE_GREATER_THAN"
	FilterArraySizeGreaterOrEqual FilterOperator = "ARRAY_SIZE_GREATER_OR_EQUAL"
	FilterArraySizeLessThan       FilterOperator = "ARRAY_SIZE_LESS_THAN"
	FilterArraySizeLessOrEqual    FilterOperator = "ARRAY_SIZE_LESS_OR_EQUAL"
)

type CriteriaOperator string

const (
	CriteriaEquals        CriteriaOperator = "EQUALS"
	CriteriaContains      CriteriaOperator = "CONTAINS"
	CriteriaLabelSelector CriteriaOperator = "LABEL_SELECTOR"
)

// -------------------------
// Schema structs
// -------------------------

type PatternAsCode struct {
	APIVersion string   `yaml:"apiVersion"`
	Kind       string   `yaml:"kind"`
	Metadata   Metadata `yaml:"metadata"`
	Spec       Spec     `yaml:"spec"`
}

type Metadata struct {
	Name string `yaml:"name"`
	// UID is set by the API server; it links every Smell to the Pattern that produced it.
	UID string `yaml:"uid,omitempty"`
}

type Spec struct {
	DisplayName   string        `yaml:"displayName"`
	Category      string        `yaml:"category"`
	Severity      Severity      `yaml:"severity"`
	Reference     string        `yaml:"reference,omitempty"`
	Message       string        `yaml:"message"`
	Target        Target        `yaml:"target"`
	Dependencies  []Dependency  `yaml:"dependencies,omitempty"`
	Relationships Relationships `yaml:"relationships,omitempty"`

	// For is the minimum duration (e.g. "30m", "1h") a target must keep satisfying the
	// relationships before its Smell becomes Active; until then the Smell is Pending.
	// Empty means 0s: the Smell is Active at the first observation.
	For string `yaml:"for,omitempty"`
}

// ResourceRef identifies a set of resource types: one kind (kind + apiVersion + plural),
// every kind of an API group version (kind "*" + apiVersion), or every kind that discovery
// lists under a category (category).
type ResourceRef struct {
	Kind       string `yaml:"kind,omitempty"`
	APIVersion string `yaml:"apiVersion,omitempty"`
	PluralName string `yaml:"plural,omitempty"`
	Category   string `yaml:"category,omitempty"`
}

// KindWildcard as a kind selects every kind of the apiVersion's group version.
const KindWildcard = "*"

// KindSet is a set of resource types, keyed by GVKKey.
type KindSet map[string]struct{}

// GVKKey is the key of an apiVersion and kind in a KindSet.
func GVKKey(apiVersion, kind string) string { return apiVersion + "|" + kind }

// Add adds an apiVersion and kind to the set.
func (k KindSet) Add(apiVersion, kind string) { k[GVKKey(apiVersion, kind)] = struct{}{} }

type Target struct {
	Kind       string `yaml:"kind,omitempty"`
	APIVersion string `yaml:"apiVersion,omitempty"`
	PluralName string `yaml:"plural,omitempty"`
	// Category and Kinds select several resource types (see ResourceRef); they are mutually
	// exclusive with Kind.
	Category    string        `yaml:"category,omitempty"`
	Kinds       []ResourceRef `yaml:"kinds,omitempty"`
	Filters     Filters       `yaml:"filters,omitempty"`
	EmitOnEmpty bool          `yaml:"emitOnEmpty"`

	// Resolved holds the resource types found by discovery for categories; it is filled at
	// run time and never read from the Pattern.
	Resolved KindSet `yaml:"-"`
}

// Refs returns the resource types the target selects.
func (t Target) Refs() []ResourceRef {
	return refsOf(t.Kind, t.APIVersion, t.PluralName, t.Category, t.Kinds)
}

// IsSingleKind reports whether the target is the classic single, concrete kind.
func (t Target) IsSingleKind() bool {
	return len(t.Kinds) == 0 && t.Category == "" && t.Kind != KindWildcard
}

type Dependency struct {
	ID         string        `yaml:"id"`
	Kind       string        `yaml:"kind,omitempty"`
	APIVersion string        `yaml:"apiVersion,omitempty"`
	PluralName string        `yaml:"plural,omitempty"`
	Category   string        `yaml:"category,omitempty"`
	Kinds      []ResourceRef `yaml:"kinds,omitempty"`
	// FromTarget derives the dependency's resource types from fields of the targets
	// (e.g. the kind a definition generates). Mutually exclusive with the fields above.
	FromTarget *FromTarget `yaml:"fromTarget,omitempty"`
	Filters    Filters     `yaml:"filters,omitempty"`

	// Resolved holds the resource types found at run time for categories and fromTarget.
	Resolved KindSet `yaml:"-"`
}

// Refs returns the resource types the dependency selects (none for fromTarget).
func (d Dependency) Refs() []ResourceRef {
	if d.FromTarget != nil {
		return nil
	}
	return refsOf(d.Kind, d.APIVersion, d.PluralName, d.Category, d.Kinds)
}

// IsSingleKind reports whether the dependency is the classic single, concrete kind.
func (d Dependency) IsSingleKind() bool {
	return d.FromTarget == nil && len(d.Kinds) == 0 && d.Category == "" && d.Kind != KindWildcard
}

// FromTarget names the target fields that hold the apiVersion and kind (and optionally the
// plural resource name) of the dependency's resource types. A fixed apiVersion can be given
// instead of a path (e.g. Gatekeeper constraints: constraints.gatekeeper.sh/v1beta1).
type FromTarget struct {
	APIVersion     string `yaml:"apiVersion,omitempty"`
	APIVersionPath string `yaml:"apiVersionPath,omitempty"`
	KindPath       string `yaml:"kindPath"`
	PluralPath     string `yaml:"pluralPath,omitempty"`
}

// Paths returns the non-empty paths of a FromTarget.
func (ft FromTarget) Paths() []string {
	var out []string
	for _, p := range []string{ft.APIVersionPath, ft.KindPath, ft.PluralPath} {
		out = appendPath(out, p)
	}
	return out
}

func refsOf(kind, apiVersion, plural, category string, kinds []ResourceRef) []ResourceRef {
	if len(kinds) > 0 {
		return kinds
	}
	return []ResourceRef{{Kind: kind, APIVersion: apiVersion, PluralName: plural, Category: category}}
}

type Filters struct {
	MatchAll  []FilterCondition `yaml:"matchAll,omitempty"`
	MatchAny  []FilterCondition `yaml:"matchAny,omitempty"`
	MatchNone []FilterCondition `yaml:"matchNone,omitempty"`
}

type FilterCondition struct {
	Path     string         `yaml:"path"`
	Operator FilterOperator `yaml:"operator"`
	Values   []string       `yaml:"values,omitempty"`
	// ValuesFrom compares with the values of another path of the same resource (EQUALS only).
	ValuesFrom string `yaml:"valuesFrom,omitempty"`
}

type Relationships struct {
	MatchAll  []Relationship `yaml:"matchAll,omitempty"`
	MatchAny  []Relationship `yaml:"matchAny,omitempty"`
	MatchNone []Relationship `yaml:"matchNone,omitempty"`
}

type Relationship struct {
	With     string           `yaml:"with"`
	Type     RelationshipType `yaml:"type"`
	Criteria []Criteria       `yaml:"criteria,omitempty"`
	// SelectorPath locates the label selector for selects (on the target) and selectedBy
	// (on the dependency). Default: spec.selector.
	SelectorPath string `yaml:"selectorPath,omitempty"`
	// EmptySelector decides what a selector without requirements selects: "all" or "none".
	// When it is unset, a LabelSelector ({matchLabels: {}}) or an empty map in a field named
	// "*Selector" selects all, and an empty map in a field named "selector" selects none.
	EmptySelector EmptySelector `yaml:"emptySelector,omitempty"`
}

// EmptySelector is what a selector without requirements selects.
type EmptySelector string

const (
	EmptySelectorAll  EmptySelector = "all"
	EmptySelectorNone EmptySelector = "none"
)

// DefaultSelectorPath is where selects and selectedBy read the selector by default.
const DefaultSelectorPath = "spec.selector"

// Selector returns the selector path of a selects or selectedBy relationship.
func (r Relationship) Selector() string {
	if r.SelectorPath == "" {
		return DefaultSelectorPath
	}
	return r.SelectorPath
}

// TargetPaths returns every path read from the target by the relationship: its criteria's
// target paths and defaults, and the selector of a selects relationship.
func (r Relationship) TargetPaths() []string {
	var out []string
	for _, c := range r.Criteria {
		out = appendPath(out, c.TargetPath)
		if c.TargetDefault != nil {
			out = appendPath(out, c.TargetDefault.Path)
		}
	}
	if r.Type == RelationshipSelects {
		out = append(out, r.Selector())
	}
	return out
}

// DependencyPaths returns every path read from the dependency by the relationship: its
// criteria's dependency paths and defaults, and the selector of a selectedBy relationship.
// Their "[@]" anchors are enumerated per dependency.
func (r Relationship) DependencyPaths() []string {
	var out []string
	for _, c := range r.Criteria {
		out = appendPath(out, c.DependencyPath)
		if c.DependencyDefault != nil {
			out = appendPath(out, c.DependencyDefault.Path)
		}
	}
	if r.Type == RelationshipSelectedBy {
		out = append(out, r.Selector())
	}
	return out
}

// TargetPaths returns every path read from the target by the pattern: target filters and
// all relationships. They share the target's "[@]" anchors.
func (s Spec) TargetPaths() []string {
	var out []string
	f := s.Target.Filters
	for _, group := range [][]FilterCondition{f.MatchAll, f.MatchAny, f.MatchNone} {
		for _, c := range group {
			out = appendPath(out, c.Path)
			out = appendPath(out, c.ValuesFrom)
		}
	}
	r := s.Relationships
	for _, group := range [][]Relationship{r.MatchAll, r.MatchAny, r.MatchNone} {
		for _, rel := range group {
			out = append(out, rel.TargetPaths()...)
		}
	}
	return out
}

func appendPath(paths []string, p string) []string {
	if p == "" {
		return paths
	}
	return append(paths, p)
}

type Criteria struct {
	TargetPath     string           `yaml:"targetPath,omitempty"`
	DependencyPath string           `yaml:"dependencyPath,omitempty"`
	Operator       CriteriaOperator `yaml:"operator"`
	// TargetValue / DependencyValue replace a path with a literal, e.g. to require that the
	// bound element of a dependency has resource "pages" (element-scoped constants).
	TargetValue     *string `yaml:"targetValue,omitempty"`
	DependencyValue *string `yaml:"dependencyValue,omitempty"`
	// Defaults apply when the path yields no value (implicit defaults of an API).
	TargetDefault     *ValueDefault `yaml:"targetDefault,omitempty"`
	DependencyDefault *ValueDefault `yaml:"dependencyDefault,omitempty"`
	// Transforms turn each value before the comparison (string-encoded references).
	TargetTransform     *Transform `yaml:"targetTransform,omitempty"`
	DependencyTransform *Transform `yaml:"dependencyTransform,omitempty"`
}

// ValueDefault is either another path of the same resource or a literal value.
type ValueDefault struct {
	Path  string  `yaml:"path,omitempty"`
	Value *string `yaml:"value,omitempty"`
}

// Transform rewrites a string value. Exactly one field is set.
type Transform struct {
	// APIGroup keeps the group of an apiVersion ("cert-manager.io/v1" -> "cert-manager.io",
	// "v1" -> "").
	APIGroup bool `yaml:"apiGroup,omitempty"`
	// Split keeps one part of the value split by a separator (negative index: from the end).
	Split *SplitTransform `yaml:"split,omitempty"`
	// Regex keeps the first capture group of a regular expression; no match drops the value.
	Regex string `yaml:"regex,omitempty"`
	// Lowercase lowercases the value.
	Lowercase bool `yaml:"lowercase,omitempty"`
}

type SplitTransform struct {
	Separator string `yaml:"separator"`
	Index     int    `yaml:"index"`
}

// -------------------------
// LintError
// -------------------------

type LintError struct {
	Message string
}

func (e *LintError) Error() string {
	return fmt.Sprintf("malformed pattern: %s", e.Message)
}

func lintErr(msg string, args ...any) error {
	return &LintError{Message: fmt.Sprintf(msg, args...)}
}

// -------------------------
// Regex
// -------------------------

var (
	reAPIVersion = regexp.MustCompile(`^[a-zA-Z0-9.-]+/v[a-zA-Z0-9]+$`)
	reName       = regexp.MustCompile(`^[a-zA-Z0-9.-]+$`)

	regexCache sync.Map // string -> *regexp.Regexp
)

// compiledRegex returns a cached compiled regular expression (patterns are linted first).
func compiledRegex(expr string) *regexp.Regexp {
	if re, ok := regexCache.Load(expr); ok {
		return re.(*regexp.Regexp)
	}
	re, err := regexp.Compile(expr)
	if err != nil {
		re = regexp.MustCompile(`$^`) // matches nothing
	}
	regexCache.Store(expr, re)
	return re
}

// -------------------------
// Lint — entry point
// -------------------------

// Lint parses a YAML byte slice, unmarshals it into PatternAsCode,
// and validates all fields against the schema rules.
func Lint(data []byte) (*PatternAsCode, error) {
	if len(data) == 0 {
		return nil, lintErr("yaml input is empty")
	}

	var p PatternAsCode
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, lintErr("pattern definition is not valid yaml: %v", err)
	}

	if err := lintAPIVersion(p.APIVersion); err != nil {
		return nil, err
	}
	if err := lintKind(p.Kind); err != nil {
		return nil, err
	}
	if err := lintMetadata(p.Metadata); err != nil {
		return nil, err
	}
	if err := lintSpec(&p.Spec); err != nil {
		return nil, err
	}

	return &p, nil
}

// -------------------------
// Root fields
// -------------------------

func lintAPIVersion(version string) error {
	if version == "" {
		return lintErr("apiVersion is empty")
	}
	if !reAPIVersion.MatchString(version) {
		return lintErr("'%s' is not a valid apiVersion. Expected format: '<domain>/v<version>' (e.g. kubepattern.dev/v1)", version)
	}
	return nil
}

func lintKind(kind string) error {
	if kind == "" {
		return lintErr("kind is empty")
	}
	if kind != "PatternAsCode" && kind != "Pattern" {
		return lintErr("kind must be 'PatternAsCode' or 'Pattern', found: %s", kind)
	}
	return nil
}

// -------------------------
// Metadata
// -------------------------

func lintMetadata(m Metadata) error {
	if m.Name == "" {
		return lintErr("metadata.name is empty")
	}
	if !reName.MatchString(m.Name) {
		return lintErr("metadata.name contains invalid characters. Expected format: [a-zA-Z0-9.-]+")
	}
	return nil
}

// -------------------------
// Spec
// -------------------------

func lintSpec(spec *Spec) error {
	if spec.DisplayName == "" {
		return lintErr("spec.displayName is empty")
	}
	if spec.Category == "" {
		return lintErr("spec.category is empty")
	}
	if err := lintSeverity(spec.Severity); err != nil {
		return err
	}

	if spec.Message == "" {
		return lintErr("spec.message is empty")
	}

	if err := lintFor(spec.For); err != nil {
		return err
	}

	if err := lintTarget(spec.Target); err != nil {
		return err
	}

	depIDs := make(map[string]struct{}, len(spec.Dependencies))
	for i, dep := range spec.Dependencies {
		if err := lintDependency(i, dep); err != nil {
			return err
		}
		if _, exists := depIDs[dep.ID]; exists {
			return lintErr("spec.dependencies[%d].id '%s' is not unique", i, dep.ID)
		}
		depIDs[dep.ID] = struct{}{}
	}

	if err := lintRelationships(spec.Relationships, depIDs); err != nil {
		return err
	}

	return lintAnchorChains(spec)
}

// lintAnchorChains requires the "[@]" anchors of each side to form a single nested chain:
// the target's anchors across its filters and all relationships, and each relationship's
// dependency anchors. Anchors on independent arrays would bind their cartesian product,
// which is empty (no target or no match at all) as soon as one of the arrays is.
// When the target is examined element by element, every relationship must read the
// element: a target without it has no binding and is not examined, so a relationship that
// ignores the element would silently never be evaluated for it.
// The paths were already checked for syntax.
func lintAnchorChains(spec *Spec) error {
	targetPaths := fieldpath.ParseAll(spec.TargetPaths())
	if a, b, found := fieldpath.AnchorConflict(targetPaths); found {
		return lintErr("spec: the target paths anchor two independent arrays, '%s' and '%s'; '[@]' anchors of the target must be nested in one array (a[@].b[@])", a, b)
	}
	element := fieldpath.DeepestAnchor(targetPaths)
	r := spec.Relationships
	for _, group := range []struct {
		name string
		rels []Relationship
	}{{"matchAll", r.MatchAll}, {"matchAny", r.MatchAny}, {"matchNone", r.MatchNone}} {
		for i, rel := range group.rels {
			if a, b, found := fieldpath.AnchorConflict(fieldpath.ParseAll(rel.DependencyPaths())); found {
				return lintErr("spec.relationships.%s[%d]: the dependency paths anchor two independent arrays, '%s' and '%s'; '[@]' anchors of a dependency must be nested in one array (a[@].b[@])", group.name, i, a, b)
			}
			if element != "" && !readsElement(rel, element) {
				return lintErr("spec.relationships.%s[%d] does not read the target element '%s': in a Pattern whose target is examined element by element, every relationship must read that element (a target without it is not examined, so this relationship would never be evaluated for it); move this relationship to its own Pattern", group.name, i, element)
			}
		}
	}
	return nil
}

func readsElement(rel Relationship, anchor string) bool {
	for _, p := range fieldpath.ParseAll(rel.TargetPaths()) {
		if p.Reaches(anchor) {
			return true
		}
	}
	return false
}

func lintFor(f string) error {
	if f == "" {
		return nil
	}
	d, err := time.ParseDuration(f)
	if err != nil {
		return lintErr("spec.for '%s' is not a valid duration (e.g. 30m, 1h, 24h)", f)
	}
	if d < 0 {
		return lintErr("spec.for '%s' must not be negative", f)
	}
	return nil
}

// ForDuration returns spec.for as a duration; an empty or invalid value (rejected by the linter) is 0.
func (s Spec) ForDuration() time.Duration {
	d, err := time.ParseDuration(s.For)
	if err != nil || d < 0 {
		return 0
	}
	return d
}

func lintSeverity(s Severity) error {
	switch s {
	case SeverityLow, SeverityMedium, SeverityHigh, SeverityCritical:
		return nil
	case "":
		return lintErr("spec.severity is empty")
	default:
		return lintErr("spec.severity must be one of [LOW, MEDIUM, HIGH, CRITICAL], found: %s", s)
	}
}

// -------------------------
// Target & Dependencies
// -------------------------

func lintTarget(t Target) error {
	if err := lintSelector("spec.target", t.Kind, t.APIVersion, t.PluralName, t.Category, t.Kinds); err != nil {
		return err
	}
	if err := lintFilters("target", t.Filters, true); err != nil {
		return err
	}
	return nil
}

func lintDependency(index int, d Dependency) error {
	ctx := fmt.Sprintf("spec.dependencies[%d]", index)
	if d.ID == "" {
		return lintErr("%s.id is empty", ctx)
	}
	if d.FromTarget != nil {
		if d.Kind != "" || d.APIVersion != "" || d.PluralName != "" || d.Category != "" || len(d.Kinds) > 0 {
			return lintErr("%s.fromTarget cannot be combined with kind, apiVersion, plural, category or kinds", ctx)
		}
		ft := d.FromTarget
		if (ft.APIVersionPath == "") == (ft.APIVersion == "") || ft.KindPath == "" {
			return lintErr("%s.fromTarget needs kindPath and exactly one of apiVersionPath or apiVersion", ctx)
		}
		// "[@]" reads the apiVersion, kind and plural of each derived type from the same
		// array element; "[*]" would pair values of different elements.
		for _, p := range ft.Paths() {
			if err := lintPath(ctx+".fromTarget", p, true); err != nil {
				return err
			}
			if fieldpath.MustParse(p).HasWildcard() {
				return lintErr("%s.fromTarget: '[*]' is not allowed in path '%s'; use '[@]' to read every element", ctx, p)
			}
		}
		if a, b, found := fieldpath.AnchorConflict(fieldpath.ParseAll(ft.Paths())); found {
			return lintErr("%s.fromTarget: the paths anchor two independent arrays, '%s' and '%s'", ctx, a, b)
		}
	} else if err := lintSelector(ctx, d.Kind, d.APIVersion, d.PluralName, d.Category, d.Kinds); err != nil {
		return err
	}
	if err := lintFilters(fmt.Sprintf("dependencies[%d]", index), d.Filters, false); err != nil {
		return err
	}
	return nil
}

// lintSelector validates the resource-type selector of a target or a dependency: exactly
// one of kind (+ apiVersion + plural), category, or a kinds list.
func lintSelector(ctx, kind, apiVersion, plural, category string, kinds []ResourceRef) error {
	forms := 0
	if kind != "" || apiVersion != "" || plural != "" {
		forms++
	}
	if category != "" {
		forms++
	}
	if len(kinds) > 0 {
		forms++
	}
	if forms == 0 {
		return lintErr("%s.kind is empty", ctx)
	}
	if forms > 1 {
		return lintErr("%s must use only one of kind, category or kinds", ctx)
	}
	if len(kinds) > 0 {
		for i, r := range kinds {
			if err := lintRef(fmt.Sprintf("%s.kinds[%d]", ctx, i), r); err != nil {
				return err
			}
		}
		return nil
	}
	return lintRef(ctx, ResourceRef{Kind: kind, APIVersion: apiVersion, PluralName: plural, Category: category})
}

func lintRef(ctx string, r ResourceRef) error {
	if r.Category != "" {
		if r.Kind != "" || r.APIVersion != "" || r.PluralName != "" {
			return lintErr("%s.category cannot be combined with kind, apiVersion or plural", ctx)
		}
		return nil
	}
	if r.Kind == "" {
		return lintErr("%s.kind is empty", ctx)
	}
	if r.APIVersion == "" {
		return lintErr("%s.apiVersion is empty", ctx)
	}
	if r.Kind == KindWildcard {
		if r.PluralName != "" {
			return lintErr("%s.plural must be empty when kind is '*'", ctx)
		}
		return nil
	}
	if r.PluralName == "" {
		return lintErr("%s.pluralName is empty", ctx)
	}
	return nil
}

// lintPath checks the syntax of a path. Element anchors "[@]" are allowed only where the
// engine binds them (target paths and relationship paths), and never after a "[*]".
func lintPath(ctx, raw string, allowAnchor bool) error {
	p, err := fieldpath.Parse(raw)
	if err != nil {
		return lintErr("%s: %v", ctx, err)
	}
	if p.HasAnchor() {
		if !allowAnchor {
			return lintErr("%s: '[@]' is not allowed in path '%s' (only target filters and relationship paths bind array elements)", ctx, raw)
		}
		if p.WildcardBeforeAnchor() {
			return lintErr("%s: '[*]' cannot precede '[@]' in path '%s'", ctx, raw)
		}
	}
	return nil
}

// -------------------------
// Filters
// -------------------------

func lintFilters(context string, f Filters, allowAnchor bool) error {
	groups := []struct {
		name  string
		items []FilterCondition
	}{
		{"matchAll", f.MatchAll},
		{"matchAny", f.MatchAny},
		{"matchNone", f.MatchNone},
	}

	for _, g := range groups {
		for i, cond := range g.items {
			if err := lintFilterCondition(context, g.name, i, cond, allowAnchor); err != nil {
				return err
			}
		}
	}
	return nil
}

func lintFilterCondition(context string, group string, index int, c FilterCondition, allowAnchor bool) error {
	if c.Path == "" {
		return lintErr("spec.%s.filters.%s[%d].path is empty", context, group, index)
	}
	ctx := fmt.Sprintf("spec.%s.filters.%s[%d]", context, group, index)
	if err := lintPath(ctx+".path", c.Path, allowAnchor); err != nil {
		return err
	}
	if err := lintFilterOperator(context, group, index, c.Operator); err != nil {
		return err
	}
	if c.ValuesFrom != "" {
		if c.Operator != FilterEquals {
			return lintErr("%s.valuesFrom is supported only with operator EQUALS", ctx)
		}
		if len(c.Values) > 0 {
			return lintErr("%s must set either values or valuesFrom, not both", ctx)
		}
		return lintPath(ctx+".valuesFrom", c.ValuesFrom, allowAnchor)
	}
	if err := lintFilterValues(context, group, index, c.Operator, c.Values); err != nil {
		return err
	}
	return nil
}

func lintFilterOperator(context string, group string, index int, op FilterOperator) error {
	switch op {
	case
		FilterEquals,
		FilterExists,
		FilterGreaterThan,
		FilterGreaterOrEqual,
		FilterLessThan,
		FilterLessOrEqual,
		FilterArraySizeEquals,
		FilterArraySizeGreaterThan,
		FilterArraySizeGreaterOrEqual,
		FilterArraySizeLessThan,
		FilterArraySizeLessOrEqual,
		FilterIsEmpty:
		return nil
	case "":
		return lintErr("spec.%s.filters.%s[%d].operator is empty", context, group, index)
	default:
		return lintErr("spec.%s.filters.%s[%d].operator '%s' is not valid", context, group, index, op)
	}
}

func lintFilterValues(context string, group string, index int, op FilterOperator, values []string) error {
	// EXISTS and IS_EMPTY do not require values
	if op == FilterExists || op == FilterIsEmpty {
		if len(values) > 0 {
			return lintErr("spec.%s.filters.%s[%d].values should be empty for operator %s", context, group, index, op)
		}
		return nil
	}
	if len(values) == 0 {
		return lintErr("spec.%s.filters.%s[%d].values is empty for operator %s", context, group, index, op)
	}
	return nil
}

// -------------------------
// Relationships
// -------------------------

func lintRelationships(rels Relationships, depIDs map[string]struct{}) error {
	groups := []struct {
		name  string
		items []Relationship
	}{
		{"matchAll", rels.MatchAll},
		{"matchAny", rels.MatchAny},
		{"matchNone", rels.MatchNone},
	}

	for _, g := range groups {
		for i, rel := range g.items {
			if err := lintRelationship(g.name, i, rel, depIDs); err != nil {
				return err
			}
		}
	}
	return nil
}

func lintRelationship(group string, index int, rel Relationship, depIDs map[string]struct{}) error {
	if rel.With == "" {
		return lintErr("spec.relationships.%s[%d].with is empty", group, index)
	}
	if _, exists := depIDs[rel.With]; !exists {
		return lintErr("spec.relationships.%s[%d].with '%s' does not match any dependency id", group, index, rel.With)
	}
	if err := lintRelationshipType(group, index, rel.Type); err != nil {
		return err
	}

	ctx := fmt.Sprintf("spec.relationships.%s[%d]", group, index)
	switch rel.Type {
	case RelationshipCustom:
		if len(rel.Criteria) == 0 {
			return lintErr("%s of type 'custom' must have at least one criteria", ctx)
		}
		if rel.SelectorPath != "" || rel.EmptySelector != "" {
			return lintErr("%s.selectorPath and emptySelector are only valid for types selects and selectedBy", ctx)
		}
	case RelationshipOwns, RelationshipOwnedBy:
		// These types leverage graph knowledge / standardized k8s relations. They shouldn't have criteria.
		if len(rel.Criteria) > 0 {
			return lintErr("spec.relationships.%s[%d] of type '%s' must not declare criteria", group, index, rel.Type)
		}
		if rel.SelectorPath != "" || rel.EmptySelector != "" {
			return lintErr("%s.selectorPath and emptySelector are only valid for types selects and selectedBy", ctx)
		}
	case RelationshipSelects, RelationshipSelectedBy:
		// The selector decides the match; criteria are optional extra conditions.
		if err := lintPath(ctx+".selectorPath", rel.Selector(), true); err != nil {
			return err
		}
		switch rel.EmptySelector {
		case "", EmptySelectorAll, EmptySelectorNone:
		default:
			return lintErr("%s.emptySelector '%s' is not valid. Expected: all, none", ctx, rel.EmptySelector)
		}
	}
	for i, c := range rel.Criteria {
		if err := lintCriteria(group, index, i, c); err != nil {
			return err
		}
	}

	return nil
}

func lintRelationshipType(group string, index int, t RelationshipType) error {
	switch t {
	case RelationshipCustom, RelationshipOwns, RelationshipOwnedBy, RelationshipSelects, RelationshipSelectedBy:
		return nil
	case "":
		return lintErr("spec.relationships.%s[%d].type is empty", group, index)
	default:
		return lintErr("spec.relationships.%s[%d].type '%s' is not valid. Expected: custom, owns, ownedBy, selects, selectedBy", group, index, t)
	}
}

func lintCriteria(group string, relIndex int, index int, c Criteria) error {
	ctx := fmt.Sprintf("spec.relationships.%s[%d].criteria[%d]", group, relIndex, index)
	if err := lintSide(ctx, "target", c.TargetPath, c.TargetValue, c.TargetDefault); err != nil {
		return err
	}
	if err := lintSide(ctx, "dependency", c.DependencyPath, c.DependencyValue, c.DependencyDefault); err != nil {
		return err
	}
	if c.TargetValue != nil && c.DependencyValue != nil {
		return lintErr("%s compares two literals", ctx)
	}
	if err := lintCriteriaOperator(group, relIndex, index, c.Operator); err != nil {
		return err
	}
	if err := lintDefault(ctx+".targetDefault", c.TargetDefault); err != nil {
		return err
	}
	if err := lintDefault(ctx+".dependencyDefault", c.DependencyDefault); err != nil {
		return err
	}
	if err := lintTransform(ctx+".targetTransform", c.TargetTransform); err != nil {
		return err
	}
	if err := lintTransform(ctx+".dependencyTransform", c.DependencyTransform); err != nil {
		return err
	}
	return nil
}

// lintSide checks one side of a criterion: a path, or a literal value (without default).
func lintSide(ctx, side, path string, value *string, def *ValueDefault) error {
	switch {
	case path == "" && value == nil:
		return lintErr("%s.%sPath is empty", ctx, side)
	case path != "" && value != nil:
		return lintErr("%s must set either %sPath or %sValue, not both", ctx, side, side)
	case value != nil && def != nil:
		return lintErr("%s.%sDefault needs a %sPath", ctx, side, side)
	case path != "":
		if err := lintPath(ctx+"."+side+"Path", path, true); err != nil {
			return err
		}
		// With "[*]" the default would apply only when no element has a value, not to each
		// element that lacks one: the elements without a value would silently not count.
		if def != nil && fieldpath.MustParse(path).HasWildcard() {
			return lintErr("%s.%sDefault cannot be used with '[*]' in %sPath '%s': it would apply only when no element has a value; use '[@]' to default each element", ctx, side, side, path)
		}
	}
	return nil
}

func lintDefault(ctx string, d *ValueDefault) error {
	if d == nil {
		return nil
	}
	if (d.Path == "") == (d.Value == nil) {
		return lintErr("%s must set exactly one of path or value", ctx)
	}
	if d.Path != "" {
		if err := lintPath(ctx+".path", d.Path, true); err != nil {
			return err
		}
		if fieldpath.MustParse(d.Path).HasWildcard() {
			return lintErr("%s.path '%s' cannot contain '[*]': the values of different elements would not be correlated; use '[@]'", ctx, d.Path)
		}
	}
	return nil
}

func lintTransform(ctx string, t *Transform) error {
	if t == nil {
		return nil
	}
	set := 0
	if t.APIGroup {
		set++
	}
	if t.Split != nil {
		set++
		if t.Split.Separator == "" {
			return lintErr("%s.split.separator is empty", ctx)
		}
	}
	if t.Regex != "" {
		set++
		re, err := regexp.Compile(t.Regex)
		if err != nil {
			return lintErr("%s.regex is not a valid regular expression: %v", ctx, err)
		}
		if re.NumSubexp() != 1 {
			return lintErr("%s.regex must have exactly one capture group, found %d", ctx, re.NumSubexp())
		}
	}
	if t.Lowercase {
		set++
	}
	if set != 1 {
		return lintErr("%s must set exactly one of apiGroup, split, regex or lowercase", ctx)
	}
	return nil
}

func lintCriteriaOperator(group string, relIndex int, index int, op CriteriaOperator) error {
	switch op {
	case CriteriaEquals:
		return nil
	case CriteriaContains, CriteriaLabelSelector:
		// Declared in the API but not evaluated by the engine (label selection is the
		// selects / selectedBy relationship types).
		return lintErr("spec.relationships.%s[%d].criteria[%d].operator '%s' is not implemented yet. Supported: EQUALS", group, relIndex, index, op)
	case "":
		return lintErr("spec.relationships.%s[%d].criteria[%d].operator is empty", group, relIndex, index)
	default:
		return lintErr("spec.relationships.%s[%d].criteria[%d].operator '%s' is not valid. Expected: EQUALS", group, relIndex, index, op)
	}
}

// ApplyTransform applies a (linted) transform to a value; ok is false when the value is dropped.
func ApplyTransform(t *Transform, v string) (string, bool) {
	switch {
	case t == nil:
		return v, true
	case t.APIGroup:
		if i := strings.LastIndex(v, "/"); i >= 0 {
			return v[:i], true
		}
		return "", true
	case t.Split != nil:
		parts := strings.Split(v, t.Split.Separator)
		i := t.Split.Index
		if i < 0 {
			i += len(parts)
		}
		if i < 0 || i >= len(parts) {
			return "", false
		}
		return parts[i], true
	case t.Regex != "":
		m := compiledRegex(t.Regex).FindStringSubmatch(v)
		if m == nil {
			return "", false
		}
		return m[1], true
	case t.Lowercase:
		return strings.ToLower(v), true
	}
	return v, true
}
