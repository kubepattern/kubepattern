/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// Severity is the impact level of a detected smell.
// +kubebuilder:validation:Enum=LOW;MEDIUM;HIGH;CRITICAL
type Severity string

const (
	SeverityLow      Severity = "LOW"
	SeverityMedium   Severity = "MEDIUM"
	SeverityHigh     Severity = "HIGH"
	SeverityCritical Severity = "CRITICAL"
)

// FilterOperator is the comparison applied by a FilterCondition.
// +kubebuilder:validation:Enum=EQUALS;IS_EMPTY;EXISTS;GREATER_THAN;GREATER_OR_EQUAL;LESS_THAN;LESS_OR_EQUAL;ARRAY_SIZE_EQUALS;ARRAY_SIZE_GREATER_THAN;ARRAY_SIZE_GREATER_OR_EQUAL;ARRAY_SIZE_LESS_THAN;ARRAY_SIZE_LESS_OR_EQUAL
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

// RelationshipType describes how a dependency relates to the target resource.
// +kubebuilder:validation:Enum=custom;owns;ownedBy;selects;selectedBy
type RelationshipType string

const (
	RelationshipCustom     RelationshipType = "custom"
	RelationshipOwns       RelationshipType = "owns"
	RelationshipOwnedBy    RelationshipType = "ownedBy"
	RelationshipSelects    RelationshipType = "selects"
	RelationshipSelectedBy RelationshipType = "selectedBy"
)

// CriteriaOperator is the comparison applied by a custom Relationship's Criteria.
// +kubebuilder:validation:Enum=EQUALS;CONTAINS;LABEL_SELECTOR
type CriteriaOperator string

const (
	CriteriaEquals        CriteriaOperator = "EQUALS"
	CriteriaContains      CriteriaOperator = "CONTAINS"
	CriteriaLabelSelector CriteriaOperator = "LABEL_SELECTOR"
)

// FilterCondition is a single predicate evaluated against a candidate resource.
type FilterCondition struct {
	// path is a dot-separated path into the candidate resource (e.g. spec.replicas).
	// +required
	// +kubebuilder:validation:MinLength=1
	Path string `json:"path"`

	// operator is the comparison applied at path.
	// +required
	Operator FilterOperator `json:"operator"`

	// values are the operands compared against path. Required for every operator
	// except EXISTS and IS_EMPTY, which must not set it.
	// +optional
	Values []string `json:"values,omitempty"`
}

// Filters groups FilterConditions with AND/OR/NOT semantics.
type Filters struct {
	// matchAll requires every condition to hold (AND).
	// +optional
	MatchAll []FilterCondition `json:"matchAll,omitempty"`

	// matchAny requires at least one condition to hold (OR).
	// +optional
	MatchAny []FilterCondition `json:"matchAny,omitempty"`

	// matchNone requires no condition to hold (NOT).
	// +optional
	MatchNone []FilterCondition `json:"matchNone,omitempty"`
}

// Target identifies the resource kind a Pattern evaluates.
type Target struct {
	// kind is the Kubernetes Kind of the target resource (e.g. Pod, Deployment).
	// +required
	// +kubebuilder:validation:MinLength=1
	Kind string `json:"kind"`

	// apiVersion is the group/version of the target resource (e.g. apps/v1).
	// +required
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9.-]+/v[a-zA-Z0-9]+$`
	APIVersion string `json:"apiVersion"`

	// plural is the plural resource name used to list the target (e.g. deployments).
	// +required
	// +kubebuilder:validation:MinLength=1
	PluralName string `json:"plural"`

	// filters narrow which target candidates are evaluated.
	// +optional
	Filters Filters `json:"filters,omitempty"`

	// emitOnEmpty causes a synthetic smell to be emitted when no target
	// candidates are found in the cluster (absence patterns).
	// +optional
	// +kubebuilder:default=false
	EmitOnEmpty bool `json:"emitOnEmpty,omitempty"`
}

// Dependency identifies a related resource kind used to evaluate Relationships.
type Dependency struct {
	// id uniquely identifies this dependency within the pattern; referenced by
	// relationships.*.with.
	// +required
	// +kubebuilder:validation:MinLength=1
	ID string `json:"id"`

	// kind is the Kubernetes Kind of the dependency resource.
	// +required
	// +kubebuilder:validation:MinLength=1
	Kind string `json:"kind"`

	// apiVersion is the group/version of the dependency resource (e.g. apps/v1).
	// +required
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9.-]+/v[a-zA-Z0-9]+$`
	APIVersion string `json:"apiVersion"`

	// plural is the plural resource name used to list the dependency (e.g. services).
	// +required
	// +kubebuilder:validation:MinLength=1
	PluralName string `json:"plural"`

	// filters narrow which dependency candidates are evaluated.
	// +optional
	Filters Filters `json:"filters,omitempty"`
}

// Criteria is a single predicate used by a "custom" Relationship to compare a
// target resource against a dependency candidate.
type Criteria struct {
	// targetPath is a dot-separated path into the target resource.
	// +required
	// +kubebuilder:validation:MinLength=1
	TargetPath string `json:"targetPath"`

	// dependencyPath is a dot-separated path into the dependency candidate.
	// +required
	// +kubebuilder:validation:MinLength=1
	DependencyPath string `json:"dependencyPath"`

	// operator is the comparison applied between targetPath and dependencyPath.
	// +required
	Operator CriteriaOperator `json:"operator"`
}

// Relationship constrains how a dependency must relate to the target resource.
type Relationship struct {
	// with is the id of the Dependency this relationship applies to.
	// +required
	// +kubebuilder:validation:MinLength=1
	With string `json:"with"`

	// type is the kind of relationship checked against the graph, or "custom"
	// to evaluate criteria directly.
	// +required
	Type RelationshipType `json:"type"`

	// criteria is required when type is "custom", and must be empty for the
	// standardized relationship types (owns, ownedBy, selects, selectedBy).
	// +optional
	Criteria []Criteria `json:"criteria,omitempty"`
}

// Relationships groups Relationships with AND/OR/NOT semantics.
type Relationships struct {
	// matchAll requires every relationship to hold (AND).
	// +optional
	MatchAll []Relationship `json:"matchAll,omitempty"`

	// matchAny requires at least one relationship to hold (OR).
	// +optional
	MatchAny []Relationship `json:"matchAny,omitempty"`

	// matchNone requires no relationship to hold (NOT).
	// +optional
	MatchNone []Relationship `json:"matchNone,omitempty"`
}

// PatternSpec defines the desired state of Pattern
type PatternSpec struct {
	// displayName is the human-readable name of the pattern.
	// +required
	// +kubebuilder:validation:MinLength=1
	DisplayName string `json:"displayName"`

	// category groups related patterns (e.g. "zombie-resources", "misconfiguration").
	// +required
	// +kubebuilder:validation:MinLength=1
	Category string `json:"category"`

	// severity is the impact level assigned to smells emitted by this pattern.
	// +required
	Severity Severity `json:"severity"`

	// reference points to external documentation for this pattern.
	// +optional
	Reference string `json:"reference,omitempty"`

	// message is the templated text used for smells emitted by this pattern.
	// Supports {{target.metadata.name}}, {{target.metadata.namespace}},
	// {{target.metadata.uid}}, {{target.kind}} and {{target.apiVersion}} placeholders.
	// +required
	// +kubebuilder:validation:MinLength=1
	Message string `json:"message"`

	// target identifies the resource kind this pattern evaluates.
	// +required
	Target Target `json:"target"`

	// dependencies are the related resource kinds used to evaluate relationships.
	// +optional
	Dependencies []Dependency `json:"dependencies,omitempty"`

	// relationships constrain how dependencies must relate to the target for a
	// smell to be emitted.
	// +optional
	Relationships Relationships `json:"relationships,omitempty"`
}

// PatternStatus defines the observed state of Pattern.
type PatternStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// conditions represent the current state of the Pattern resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Standard condition types include:
	// - "Available": the resource is fully functional
	// - "Progressing": the resource is being created or updated
	// - "Degraded": the resource failed to reach or maintain its desired state
	//
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=pat

// Pattern is the Schema for the patterns API
type Pattern struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of Pattern
	// +required
	Spec PatternSpec `json:"spec"`

	// status defines the observed state of Pattern
	// +optional
	Status PatternStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// PatternList contains a list of Pattern
type PatternList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Pattern `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &Pattern{}, &PatternList{})
		return nil
	})
}
