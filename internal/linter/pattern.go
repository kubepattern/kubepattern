package linter

import (
	"encoding/json"
	"fmt"
	"regexp"
)

// -------------------------
// LintError
// -------------------------

// LintError represents a validation failure when checking a pattern definition.
type LintError struct {
	Message string
}

// Error implements the error interface for LintError.
func (e *LintError) Error() string {
	return fmt.Sprintf("malformed pattern: %s", e.Message)
}

// lintErr is a helper to create a formatted LintError.
func lintErr(msg string, args ...any) error {
	return &LintError{Message: fmt.Sprintf(msg, args...)}
}

// -------------------------
// Regex
// -------------------------

var (
	// reVersion matches the '<domain>/v<number>' format.
	reVersion = regexp.MustCompile(`^[a-zA-Z0-9.-]+/v[a-zA-Z0-9]+$`)
	// reName matches alphanumeric strings including dots and dashes.
	reName = regexp.MustCompile(`^[a-zA-Z0-9.-]+$`)
	// reCategory matches alphanumeric strings including dashes.
	reCategory = regexp.MustCompile(`^[a-zA-Z0-9-]+$`)
	// reURL ensures a string starts with a standard web protocol.
	reURL = regexp.MustCompile(`^https?://`)
)

// -------------------------
// Helpers
// -------------------------

// isEmpty checks if a string is empty or contains the literal string "null".
func isEmpty(s string) bool {
	return s == "" || s == "null"
}

// keys returns a slice of keys from a map with string-based keys.
func keys[K ~string, V struct{}](m map[K]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, string(k))
	}
	return out
}

// getString performs a type assertion to safely retrieve a string from a map.
func getString(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}

// getBool performs a type assertion to safely retrieve a boolean from a map.
func getBool(m map[string]any, key string) bool {
	v, _ := m[key].(bool)
	return v
}

// -------------------------
// Lint — entry point
// -------------------------

// Lint parses a JSON string and validates it against the Pattern schema requirements.
// It returns a LintError if any field fails validation.
func Lint(jsonStr string) error {
	if jsonStr == "" {
		return lintErr("json string is empty")
	}

	var root map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &root); err != nil {
		return lintErr("pattern definition is not a valid json")
	}

	// Validate top-level required fields
	if err := lintVersion(getString(root, "version")); err != nil {
		return err
	}
	if err := lintKind(getString(root, "kind")); err != nil {
		return err
	}

	// Extract and validate nested Metadata object
	metadata, _ := root["metadata"].(map[string]any)
	if err := lintMetadata(metadata); err != nil {
		return err
	}

	// Extract and validate nested Spec object
	spec, _ := root["spec"].(map[string]any)
	if err := lintSpec(spec); err != nil {
		return err
	}

	return nil
}

// -------------------------
// Root fields
// -------------------------

// lintVersion validates the API version format.
func lintVersion(version string) error {
	if isEmpty(version) {
		return lintErr("version is null or empty")
	}
	if !reVersion.MatchString(version) {
		return lintErr(
			"'%s' is not a valid version. Expected format: '<domain>/v<version>' (es. kubepattern.it/v1).",
			version,
		)
	}
	return nil
}

// lintKind ensures the kind is strictly set to 'Pattern'.
func lintKind(kind string) error {
	if isEmpty(kind) {
		return lintErr("kind is null or empty")
	}
	if kind != "Pattern" {
		return lintErr("kind must be 'Pattern', found: %s", kind)
	}
	return nil
}

// -------------------------
// Metadata
// -------------------------

// lintMetadata runs a suite of checks against the metadata fields.
func lintMetadata(metadata map[string]any) error {
	if metadata == nil {
		return lintErr("metadata is null or empty")
	}

	checks := []func() error{
		func() error { return lintMetadataName(getString(metadata, "name")) },
		func() error { return lintMetadataDisplayName(getString(metadata, "displayName")) },
		func() error { return lintMetadataPatternType(getString(metadata, "patternType")) },
		func() error { return lintMetadataSeverity(getString(metadata, "severity")) },
		func() error { return lintMetadataURL("registryUrl", getString(metadata, "registryUrl")) },
	}

	for _, check := range checks {
		if err := check(); err != nil {
			return err
		}
	}
	return nil
}

func lintMetadataName(name string) error {
	if isEmpty(name) {
		return lintErr("metadata.name is null or empty")
	}
	if !reName.MatchString(name) {
		return lintErr("metadata.name contains invalid characters. Correct format: [a-zA-Z0-9.-]+")
	}
	return nil
}

func lintMetadataDisplayName(displayName string) error {
	if isEmpty(displayName) {
		return lintErr("metadata.displayName is null or empty")
	}
	return nil
}

func lintMetadataPatternType(patternType string) error {
	if isEmpty(patternType) {
		return lintErr("metadata.patternType is null or empty")
	}
	return nil
}

// lintMetadataSeverity restricts severity to a specific set of uppercase keywords.
func lintMetadataSeverity(severity string) error {
	if isEmpty(severity) {
		return lintErr("metadata.severity is null or empty")
	}
	if severity != "LOW" && severity != "MEDIUM" && severity != "HIGH" && severity != "CRITICAL" {
		return lintErr("metadata.severity must be one of [LOW, MEDIUM, HIGH, CRITICAL], found: %s", severity)
	}

	return nil
}

func lintMetadataURL(field, url string) error {
	if !isEmpty(url) && !reURL.MatchString(url) {
		return lintErr("metadata.%s must be a valid URL starting with https://", field)
	}
	return nil
}

// -------------------------
// Spec
// -------------------------

// lintSpec validates the core functional definition of the Pattern.
func lintSpec(spec map[string]any) error {
	if spec == nil {
		return lintErr("spec is null or empty")
	}

	checks := []func() error{
		func() error { return lintSpecMessage(getString(spec, "message")) },
		func() error { return lintSpecTopology(getString(spec, "topology")) },
		func() error { return lintSpecResources(spec["resources"]) },
	}

	for _, check := range checks {
		if err := check(); err != nil {
			return err
		}
	}

	return nil
}

// lintSpecResources iterates through the resource list and validates each entry.
func lintSpecResources(raw any) error {
	items, ok := raw.([]any)
	if !ok || len(items) == 0 {
		return lintErr("spec.resources is null or empty")
	}

	for i, item := range items {
		resource, ok := item.(map[string]any)
		if !ok {
			return lintErr("spec.resources[%d] is not a valid object", i)
		}
		if err := lintResource(resource); err != nil {
			return err
		}
	}

	return nil
}

func lintSpecMessage(msg string) error {
	if isEmpty(msg) {
		return lintErr("spec.message is null or empty")
	}

	return nil
}

// lintSpecTopology ensures the topology matches known deployment patterns.
func lintSpecTopology(topology string) error {
	if isEmpty(topology) {
		return lintErr("spec.topology is null or empty")
	}

	if topology != "SINGLE" && topology != "LEADER_FOLLOWER" {
		return lintErr("spec.topology must be 'SINGLE', 'LEADER_FOLLOWER', found: %s", topology)
	}

	return nil
}

func lintResources(resources map[string]any) error {
	if resources == nil {
		return lintErr("resources is null or empty")
	}

	return nil
}

// lintResource validates individual resource requirements and their filters.
func lintResource(resource map[string]any) error {
	if resource == nil {
		return lintErr("resource is null or empty")
	}

	checks := []func() error{
		func() error { return lintResourceKind(getString(resource, "kind")) },
		func() error { return lintResourceId(getString(resource, "id")) },
		func() error { return lintResourceLeader(getBool(resource, "leader")) },
		func() error {
			// Filters are optional, only lint if present
			if f, ok := resource["filters"].(map[string]any); ok {
				return lintResourceFilters(f)
			}
			return nil
		},
	}

	for _, check := range checks {
		if err := check(); err != nil {
			return err
		}
	}

	return nil
}

func lintResourceKind(kind string) error {
	if isEmpty(kind) {
		return lintErr("resource.kind is null or empty")
	}
	return nil
}

func lintResourceId(id string) error {
	if isEmpty(id) {
		return lintErr("resource.id is null or empty")
	}
	return nil
}

func lintResourceLeader(leader bool) error {
	return nil // Placeholder for future logic
}

// -------------------------
// Filters
// -------------------------

// lintResourceFilters handles the validation of matchAll, matchAny, and matchNone logic groups.
func lintResourceFilters(filters map[string]any) error {
	if filters == nil {
		return nil // Filters are optional
	}

	groups := []string{"matchAll", "matchAny", "matchNone"}
	for _, group := range groups {
		raw, exists := filters[group]
		if !exists {
			continue
		}

		items, ok := raw.([]any)
		if !ok {
			return lintErr("resource.filters.%s is not a valid array", group)
		}

		for i, item := range items {
			condition, ok := item.(map[string]any)
			if !ok {
				return lintErr("resource.filters.%s[%d] is not a valid object", group, i)
			}
			if err := lintFilterCondition(group, i, condition); err != nil {
				return err
			}
		}
	}

	return nil
}

// lintFilterCondition validates the key-operator-value triplet of a filter.
func lintFilterCondition(group string, index int, condition map[string]any) error {
	checks := []func() error{
		func() error { return lintFilterKey(group, index, getString(condition, "key")) },
		func() error { return lintFilterOperator(group, index, getString(condition, "operator")) },
		func() error {
			return lintFilterValues(group, index, condition["values"], getString(condition, "operator"))
		},
	}

	for _, check := range checks {
		if err := check(); err != nil {
			return err
		}
	}

	return nil
}

func lintFilterKey(group string, index int, key string) error {
	if isEmpty(key) {
		return lintErr("resource.filters.%s[%d].key is null or empty", group, index)
	}
	return nil
}

func lintFilterOperator(group string, index int, operator string) error {
	if isEmpty(operator) {
		return lintErr("resource.filters.%s[%d].operator is null or empty", group, index)
	}
	return nil
}

func lintFilterValues(group string, index int, raw any, operator string) error {
	return nil // Placeholder for value-type validation based on operator
}
