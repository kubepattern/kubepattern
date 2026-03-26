package analysis

import "kubepattern-go/internal/linter"

// Smell represents a detected architectural issue on a target resource.
type Smell struct {
	// CRDName is the deterministic Kubernetes resource name: {pattern-name}-{target-uid}
	CRDName        string
	PatternName    string
	PatternVersion string
	Name           string
	Category       string
	Severity       linter.Severity
	Message        string
	Reference      string
	Suppress       bool
	Target         SmellTarget
}

// SmellTarget holds the identifying information of the resource that triggered the smell.
type SmellTarget struct {
	APIVersion string
	Kind       string
	Name       string
	Namespace  string
	UID        string
}
