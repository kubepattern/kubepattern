package linter

import (
	"encoding/json"
	"fmt"
	"regexp"
)

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
	reVersion  = regexp.MustCompile(`^[a-zA-Z0-9.-]+/v[a-zA-Z0-9]+$`)
	reName     = regexp.MustCompile(`^[a-zA-Z0-9.-]+$`)
	reCategory = regexp.MustCompile(`^[a-zA-Z0-9-]+$`)
	reURL      = regexp.MustCompile(`^https?://`)
)

// -------------------------
// Helpers
// -------------------------

func isEmpty(s string) bool {
	return s == "" || s == "null"
}

func keys[K ~string, V struct{}](m map[K]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, string(k))
	}
	return out
}

func getString(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}

func getBool(m map[string]any, key string) bool {
	v, _ := m[key].(bool)
	return v
}

// -------------------------
// Lint — entry point
// -------------------------

func Lint(jsonStr string) error {
	if jsonStr == "" {
		return lintErr("json string is empty")
	}

	var root map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &root); err != nil {
		return lintErr("pattern definition is not a valid json")
	}

	if err := lintVersion(getString(root, "version")); err != nil {
		return err
	}
	if err := lintKind(getString(root, "kind")); err != nil {
		return err
	}

	metadata, _ := root["metadata"].(map[string]any)
	if err := lintMetadata(metadata); err != nil {
		return err
	}

	spec, _ := root["spec"].(map[string]any)
	if err := lintSpec(spec); err != nil {
		return err
	}

	return nil
}

// -------------------------
// Root fields
// -------------------------

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

func lintResource(resource map[string]any) error {
	if resource == nil {
		return lintErr("resource is null or empty")
	}

	checks := []func() error{
		func() error { return lintResourceKind(getString(resource, "kind")) },
		func() error { return lintResourceId(getString(resource, "id")) },
		func() error { return lintResourceLeader(getBool(resource, "leader")) },
		func() error { return lintResourceFilters(resource["filters"].(map[string]any)) },
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
	return nil
}

// -------------------------
// Filters
// -------------------------

func lintResourceFilters(filters map[string]any) error {
	if filters == nil {
		return nil // filters è opzionale
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
	return nil
}
