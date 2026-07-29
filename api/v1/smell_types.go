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

// SmellPatternRef identifies the Pattern that produced a Smell.
type SmellPatternRef struct {
	// name is the name of the Pattern that produced this smell.
	// +optional
	Name string `json:"name,omitempty"`

	// version is the apiVersion of the Pattern that produced this smell.
	// +optional
	Version string `json:"version,omitempty"`
}

// SmellTarget identifies the resource that triggered a Smell.
type SmellTarget struct {
	// apiVersion is the group/version of the target resource.
	// +required
	APIVersion string `json:"apiVersion"`

	// kind is the Kind of the target resource.
	// +required
	Kind string `json:"kind"`

	// name is the name of the target resource.
	// +required
	Name string `json:"name"`

	// namespace is the namespace of the target resource, empty for cluster-scoped resources.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// uid is the UID of the target resource.
	// +required
	UID string `json:"uid"`
}

// SmellSpec defines the desired state of Smell
type SmellSpec struct {
	// name is the smell name.
	// +required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// category groups related smells (e.g. "zombie-resources", "misconfiguration").
	// +required
	// +kubebuilder:validation:MinLength=1
	Category string `json:"category"`

	// reference points to external documentation for this smell.
	// +optional
	Reference string `json:"reference,omitempty"`

	// pattern identifies the Pattern that produced this smell.
	// +optional
	Pattern SmellPatternRef `json:"pattern,omitempty"`

	// message is the human-readable description of the detected issue.
	// +optional
	Message string `json:"message,omitempty"`

	// severity is the impact level of the detected smell.
	// +optional
	Severity Severity `json:"severity,omitempty"`

	// suppress marks this smell as a known false positive to be ignored.
	// +optional
	// +kubebuilder:default=false
	Suppress bool `json:"suppress,omitempty"`

	// target identifies the resource that triggered this smell.
	// +required
	Target SmellTarget `json:"target"`
}

// SmellStatus defines the observed state of Smell.
type SmellStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// conditions represent the current state of the Smell resource.
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
// +kubebuilder:resource:shortName=smell
// +kubebuilder:printcolumn:name="Severity",type="string",JSONPath=".spec.severity",description="Smell severity (LOW, MEDIUM, HIGH, CRITICAL)"
// +kubebuilder:printcolumn:name="Target Kind",type="string",JSONPath=".spec.target.kind",description="Kind of the target resource"
// +kubebuilder:printcolumn:name="Target Name",type="string",JSONPath=".spec.target.name",description="Name of the target resource"
// +kubebuilder:printcolumn:name="Target Namespace",type="string",JSONPath=".spec.target.namespace",description="Namespace of the target resource",priority=1
// +kubebuilder:printcolumn:name="Message",type="string",JSONPath=".spec.message",description="Smell message",priority=1
// +kubebuilder:printcolumn:name="Suppressed",type="boolean",JSONPath=".spec.suppress",description="Indicates if the smell is ignored",priority=1

// Smell is the Schema for the smells API
type Smell struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of Smell
	// +required
	Spec SmellSpec `json:"spec"`

	// status defines the observed state of Smell
	// +optional
	Status SmellStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// SmellList contains a list of Smell
type SmellList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Smell `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &Smell{}, &SmellList{})
		return nil
	})
}
