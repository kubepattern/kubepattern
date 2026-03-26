package registry

import "context"

// Fetcher defines the interface for retrieving PatternAsCode definitions.
// It returns a map where the key is a unique identifier (e.g., filename or CRD name)
// and the value is the raw YAML/JSON byte content of the pattern.
type Fetcher interface {
	ReadAllDefinitions(ctx context.Context) (map[string][]byte, error)
}
