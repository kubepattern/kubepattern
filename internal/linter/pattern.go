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
	RelationshipCustom    RelationshipType = "custom"
	RelationshipOwnership RelationshipType = "ownership"
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
	Version  string   `yaml:"version"`
	Kind     string   `yaml:"kind"`
	Metadata Metadata `yaml:"metadata"`
	Spec     Spec     `yaml:"spec"`
}

type Metadata struct {
	Name        string   `yaml:"name"`
	DisplayName string   `yaml:"displayName"`
	PatternType string   `yaml:"patternType"`
	Severity    Severity `yaml:"severity"`
	Reference   string   `yaml:"reference"`
}

type Spec struct {
	Message       string         `yaml:"message"`
	Resources     []Resource     `yaml:"resources"`
	Relationships []Relationship `yaml:"relationships"`
	MinRequired   int            `yaml:"minRequired"`
}

type Resource struct {
	ID         string  `yaml:"id"`
	Kind       string  `yaml:"kind"`
	APIVersion string  `yaml:"apiVersion"`
	Leader     bool    `yaml:"leader"`
	Filters    Filters `yaml:"filters"`
}

type Filters struct {
	MatchAll  []FilterCondition `yaml:"matchAll"`
	MatchAny  []FilterCondition `yaml:"matchAny"`
	MatchNone []FilterCondition `yaml:"matchNone"`
}

type FilterCondition struct {
	Key      string         `yaml:"key"`
	Operator FilterOperator `yaml:"operator"`
	Values   []string       `yaml:"values"`
}

type Relationship struct {
	Type      RelationshipType `yaml:"type"`
	Required  bool             `yaml:"required"`
	Shared    bool             `yaml:"shared"`
	Resources RelationshipSide `yaml:"resources"`
}

type RelationshipSide struct {
	From     string     `yaml:"from"`
	To       string     `yaml:"to"`
	Criteria []Criteria `yaml:"criteria"`
}

type Criteria struct {
	From     string           `yaml:"from"`
	To       string           `yaml:"to"`
	Operator CriteriaOperator `yaml:"operator"`
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
	reVersion = regexp.MustCompile(`^[a-zA-Z0-9.-]+/v[a-zA-Z0-9]+$`)
	reName    = regexp.MustCompile(`^[a-zA-Z0-9.-]+$`)
	reURL     = regexp.MustCompile(`^https?://`)
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

	if err := lintVersion(p.Version); err != nil {
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

func lintVersion(version string) error {
	if version == "" {
		return lintErr("version is empty")
	}
	if !reVersion.MatchString(version) {
		return lintErr("'%s' is not a valid version. Expected format: '<domain>/v<version>' (e.g. kubepattern.dev/v1)", version)
	}
	return nil
}

func lintKind(kind string) error {
	if kind == "" {
		return lintErr("kind is empty")
	}
	if kind != "PatternAsCode" && kind != "Pattern" {
		return lintErr("kind must be 'PatternAsCode', found: %s", kind)
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
	if m.DisplayName == "" {
		return lintErr("metadata.displayName is empty")
	}
	if m.PatternType == "" {
		return lintErr("metadata.patternType is empty")
	}
	if err := lintSeverity(m.Severity); err != nil {
		return err
	}
	return nil
}

func lintSeverity(s Severity) error {
	switch s {
	case SeverityLow, SeverityMedium, SeverityHigh, SeverityCritical:
		return nil
	case "":
		return lintErr("metadata.severity is empty")
	default:
		return lintErr("metadata.severity must be one of [LOW, MEDIUM, HIGH, CRITICAL], found: %s", s)
	}
}

// -------------------------
// Spec
// -------------------------

func lintSpec(spec *Spec) error {
	if spec.Message == "" {
		return lintErr("spec.message is empty")
	}
	if err := lintResources(spec.Resources); err != nil {
		return err
	}

	// Build the set of valid resource IDs for relationship validation
	resourceIDs := make(map[string]struct{}, len(spec.Resources))
	for _, r := range spec.Resources {
		resourceIDs[r.ID] = struct{}{}
	}

	if err := lintRelationships(spec.Relationships, resourceIDs); err != nil {
		return err
	}
	return nil
}

// -------------------------
// Resources
// -------------------------

func lintResources(resources []Resource) error {
	if len(resources) == 0 {
		return lintErr("spec.resources is empty")
	}

	leaderCount := 0
	for i, r := range resources {
		if err := lintResource(i, r); err != nil {
			return err
		}
		if r.Leader {
			leaderCount++
		}
	}

	// If no leader declared, the first resource is the implicit leader — no error.
	// If more than one leader declared, that is ambiguous.
	if leaderCount > 1 {
		return lintErr("spec.resources has %d resources with leader: true — exactly one is allowed", leaderCount)
	}

	return nil
}

func lintResource(index int, r Resource) error {
	if r.ID == "" {
		return lintErr("spec.resources[%d].id is empty", index)
	}
	if r.Kind == "" {
		return lintErr("spec.resources[%d].kind is empty", index)
	}
	if r.APIVersion == "" {
		return lintErr("spec.resources[%d].apiVersion is empty", index)
	}
	if err := lintFilters(index, r.Filters); err != nil {
		return err
	}
	return nil
}

// -------------------------
// Filters
// -------------------------

func lintFilters(resourceIndex int, f Filters) error {
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
			if err := lintFilterCondition(resourceIndex, g.name, i, cond); err != nil {
				return err
			}
		}
	}
	return nil
}

func lintFilterCondition(resourceIndex int, group string, index int, c FilterCondition) error {
	if c.Key == "" {
		return lintErr("spec.resources[%d].filters.%s[%d].key is empty", resourceIndex, group, index)
	}
	if err := lintFilterOperator(resourceIndex, group, index, c.Operator); err != nil {
		return err
	}
	if err := lintFilterValues(resourceIndex, group, index, c.Operator, c.Values); err != nil {
		return err
	}
	return nil
}

func lintFilterOperator(resourceIndex int, group string, index int, op FilterOperator) error {
	switch op {
	case
		FilterEquals,
		FilterExists,
		FilterIsEmpty:
		return nil
	case "":
		return lintErr("spec.resources[%d].filters.%s[%d].operator is empty", resourceIndex, group, index)
	default:
		return lintErr("spec.resources[%d].filters.%s[%d].operator '%s' is not valid", resourceIndex, group, index, op)
	}
}

func lintFilterValues(resourceIndex int, group string, index int, op FilterOperator, values []string) error {
	// EXISTS and IS_EMPTY do not require values
	if op == FilterExists || op == FilterIsEmpty {
		return nil
	}
	if len(values) == 0 {
		return lintErr("spec.resources[%d].filters.%s[%d].values is empty for operator %s", resourceIndex, group, index, op)
	}
	return nil
}

// -------------------------
// Relationships
// -------------------------

func lintRelationships(relationships []Relationship, resourceIDs map[string]struct{}) error {
	for i, rel := range relationships {
		if err := lintRelationship(i, rel, resourceIDs); err != nil {
			return err
		}
	}
	return nil
}

func lintRelationship(index int, rel Relationship, resourceIDs map[string]struct{}) error {
	if err := lintRelationshipType(index, rel.Type); err != nil {
		return err
	}
	if err := lintRelationshipSide(index, rel.Resources, resourceIDs); err != nil {
		return err
	}

	switch rel.Type {
	case RelationshipCustom:
		// custom requires at least one criterion
		if len(rel.Resources.Criteria) == 0 {
			return lintErr("spec.relationships[%d] of type 'custom' must have at least one criteria", index)
		}
		for i, c := range rel.Resources.Criteria {
			if err := lintCriteria(index, i, c); err != nil {
				return err
			}
		}
	case RelationshipOwnership:
		// ownership uses the pre-built graph edges — criteria are not needed
		if len(rel.Resources.Criteria) > 0 {
			return lintErr("spec.relationships[%d] of type 'ownership' must not declare criteria", index)
		}
	}

	return nil
}

func lintRelationshipType(index int, t RelationshipType) error {
	switch t {
	case RelationshipCustom, RelationshipOwnership:
		return nil
	case "":
		return lintErr("spec.relationships[%d].type is empty", index)
	default:
		return lintErr("spec.relationships[%d].type '%s' is not valid. Expected: custom, ownership", index, t)
	}
}

func lintRelationshipSide(index int, side RelationshipSide, resourceIDs map[string]struct{}) error {
	if side.From == "" {
		return lintErr("spec.relationships[%d].resources.from is empty", index)
	}
	if side.To == "" {
		return lintErr("spec.relationships[%d].resources.to is empty", index)
	}
	if _, ok := resourceIDs[side.From]; !ok {
		return lintErr("spec.relationships[%d].resources.from '%s' does not match any resource id", index, side.From)
	}
	if _, ok := resourceIDs[side.To]; !ok {
		return lintErr("spec.relationships[%d].resources.to '%s' does not match any resource id", index, side.To)
	}
	if side.From == side.To {
		return lintErr("spec.relationships[%d].resources.from and to must be different", index)
	}
	return nil
}

func lintCriteria(relIndex int, index int, c Criteria) error {
	if c.From == "" {
		return lintErr("spec.relationships[%d].criteria[%d].from is empty", relIndex, index)
	}
	if c.To == "" {
		return lintErr("spec.relationships[%d].criteria[%d].to is empty", relIndex, index)
	}
	if err := lintCriteriaOperator(relIndex, index, c.Operator); err != nil {
		return err
	}
	return nil
}

func lintCriteriaOperator(relIndex int, index int, op CriteriaOperator) error {
	switch op {
	case CriteriaEquals, CriteriaContains, CriteriaLabelSelector:
		return nil
	case "":
		return lintErr("spec.relationships[%d].criteria[%d].operator is empty", relIndex, index)
	default:
		return lintErr("spec.relationships[%d].criteria[%d].operator '%s' is not valid. Expected: EQUALS, CONTAINS, LABEL_SELECTOR", relIndex, index, op)
	}
}

// -------------------------
// Helpers
// -------------------------

// LeaderResource returns the leader resource from the pattern.
// If none is explicitly marked, the first resource is returned as an implicit leader.
func LeaderResource(p *PatternAsCode) *Resource {
	for i := range p.Spec.Resources {
		if p.Spec.Resources[i].Leader {
			return &p.Spec.Resources[i]
		}
	}
	if len(p.Spec.Resources) > 0 {
		return &p.Spec.Resources[0]
	}
	return nil
}
