package linter

import (
	"fmt"
	"regexp"

	"gopkg.in/yaml.v3"
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
}

type Target struct {
	Kind        string  `yaml:"kind"`
	APIVersion  string  `yaml:"apiVersion"`
	PluralName  string  `yaml:"plural"`
	Filters     Filters `yaml:"filters,omitempty"`
	EmitOnEmpty bool    `yaml:"emitOnEmpty"`
}

type Dependency struct {
	ID         string  `yaml:"id"`
	Kind       string  `yaml:"kind"`
	APIVersion string  `yaml:"apiVersion"`
	PluralName string  `yaml:"plural"`
	Filters    Filters `yaml:"filters,omitempty"`
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
}

type Criteria struct {
	TargetPath     string           `yaml:"targetPath"`
	DependencyPath string           `yaml:"dependencyPath"`
	Operator       CriteriaOperator `yaml:"operator"`
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
)

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

	return nil
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
	if t.Kind == "" {
		return lintErr("spec.target.kind is empty")
	}
	if t.APIVersion == "" {
		return lintErr("spec.target.apiVersion is empty")
	}

	if t.PluralName == "" {
		return lintErr("spec.target.pluralName is empty")
	}

	if err := lintFilters("target", t.Filters); err != nil {
		return err
	}
	return nil
}

func lintDependency(index int, d Dependency) error {
	if d.ID == "" {
		return lintErr("spec.dependencies[%d].id is empty", index)
	}
	if d.Kind == "" {
		return lintErr("spec.dependencies[%d].kind is empty", index)
	}
	if d.APIVersion == "" {
		return lintErr("spec.dependencies[%d].apiVersion is empty", index)
	}

	if d.PluralName == "" {
		return lintErr("spec.dependencies[%d].pluralName is empty", index)
	}

	if err := lintFilters(fmt.Sprintf("dependencies[%d]", index), d.Filters); err != nil {
		return err
	}
	return nil
}

// -------------------------
// Filters
// -------------------------

func lintFilters(context string, f Filters) error {
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
			if err := lintFilterCondition(context, g.name, i, cond); err != nil {
				return err
			}
		}
	}
	return nil
}

func lintFilterCondition(context string, group string, index int, c FilterCondition) error {
	if c.Path == "" {
		return lintErr("spec.%s.filters.%s[%d].path is empty", context, group, index)
	}
	if err := lintFilterOperator(context, group, index, c.Operator); err != nil {
		return err
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

	switch rel.Type {
	case RelationshipCustom:
		if len(rel.Criteria) == 0 {
			return lintErr("spec.relationships.%s[%d] of type 'custom' must have at least one criteria", group, index)
		}
		for i, c := range rel.Criteria {
			if err := lintCriteria(group, index, i, c); err != nil {
				return err
			}
		}
	case RelationshipOwns, RelationshipOwnedBy, RelationshipSelects, RelationshipSelectedBy:
		// These types leverage graph knowledge / standardized k8s relations. They shouldn't have criteria.
		if len(rel.Criteria) > 0 {
			return lintErr("spec.relationships.%s[%d] of type '%s' must not declare criteria", group, index, rel.Type)
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
	if c.TargetPath == "" {
		return lintErr("spec.relationships.%s[%d].criteria[%d].targetPath is empty", group, relIndex, index)
	}
	if c.DependencyPath == "" {
		return lintErr("spec.relationships.%s[%d].criteria[%d].dependencyPath is empty", group, relIndex, index)
	}
	if err := lintCriteriaOperator(group, relIndex, index, c.Operator); err != nil {
		return err
	}
	return nil
}

func lintCriteriaOperator(group string, relIndex int, index int, op CriteriaOperator) error {
	switch op {
	case CriteriaEquals, CriteriaContains, CriteriaLabelSelector:
		return nil
	case "":
		return lintErr("spec.relationships.%s[%d].criteria[%d].operator is empty", group, relIndex, index)
	default:
		return lintErr("spec.relationships.%s[%d].criteria[%d].operator '%s' is not valid. Expected: EQUALS, CONTAINS, LABEL_SELECTOR", group, relIndex, index, op)
	}
}
