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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kubepatternv1 "kubepattern-go/api/v1"
)

const (
	testDepID        = "dep1"
	pathMetadataName = "metadata.name"
)

// validTestPattern returns a Pattern that satisfies every cross-field rule
// checked by validatePattern, so individual tests can mutate a single field
// to trigger exactly one violation.
func validTestPattern() *kubepatternv1.Pattern {
	return &kubepatternv1.Pattern{
		ObjectMeta: metav1.ObjectMeta{Name: "valid-pattern"},
		Spec: kubepatternv1.PatternSpec{
			DisplayName: "Valid Pattern",
			Category:    "test-category",
			Severity:    kubepatternv1.SeverityLow,
			Message:     "test message",
			Target: kubepatternv1.Target{
				Kind:       "Deployment",
				APIVersion: "apps/v1",
				PluralName: "deployments",
			},
			Dependencies: []kubepatternv1.Dependency{
				{ID: testDepID, Kind: "Service", APIVersion: "v1", PluralName: "services"},
			},
			Relationships: kubepatternv1.Relationships{
				MatchAll: []kubepatternv1.Relationship{
					{With: testDepID, Type: kubepatternv1.RelationshipSelectedBy},
				},
			},
		},
	}
}

var _ = Describe("Pattern Webhook", func() {
	var validator PatternCustomValidator

	BeforeEach(func() {
		validator = PatternCustomValidator{}
	})

	Context("When creating or updating Pattern under Validating Webhook", func() {
		It("should admit a well-formed pattern", func() {
			Expect(validator.ValidateCreate(ctx, validTestPattern())).Error().NotTo(HaveOccurred())
		})

		It("should deny duplicate dependency ids", func() {
			obj := validTestPattern()
			obj.Spec.Dependencies = append(obj.Spec.Dependencies, kubepatternv1.Dependency{
				ID: testDepID, Kind: "ConfigMap", APIVersion: "v1", PluralName: "configmaps",
			})
			Expect(validator.ValidateCreate(ctx, obj)).Error().To(MatchError(ContainSubstring("Duplicate value")))
		})

		It("should deny a relationship referencing an unknown dependency", func() {
			obj := validTestPattern()
			obj.Spec.Relationships.MatchAll[0].With = "does-not-exist"
			Expect(validator.ValidateCreate(ctx, obj)).Error().To(MatchError(ContainSubstring("does not match any dependency id")))
		})

		It("should deny a custom relationship without criteria", func() {
			obj := validTestPattern()
			obj.Spec.Relationships.MatchAll[0].Type = kubepatternv1.RelationshipCustom
			Expect(validator.ValidateCreate(ctx, obj)).Error().To(MatchError(ContainSubstring("must have at least one criteria")))
		})

		It("should deny a standardized relationship with unexpected criteria", func() {
			obj := validTestPattern()
			obj.Spec.Relationships.MatchAll[0].Criteria = []kubepatternv1.Criteria{
				{TargetPath: pathMetadataName, DependencyPath: pathMetadataName, Operator: kubepatternv1.CriteriaEquals},
			}
			Expect(validator.ValidateCreate(ctx, obj)).Error().To(MatchError(ContainSubstring("must not be set for relationship type")))
		})

		It("should deny an EQUALS filter without values", func() {
			obj := validTestPattern()
			obj.Spec.Target.Filters.MatchAll = []kubepatternv1.FilterCondition{
				{Path: pathMetadataName, Operator: kubepatternv1.FilterEquals},
			}
			Expect(validator.ValidateCreate(ctx, obj)).Error().To(MatchError(ContainSubstring("must not be empty for operator EQUALS")))
		})

		It("should deny an EXISTS filter with values", func() {
			obj := validTestPattern()
			obj.Spec.Target.Filters.MatchAll = []kubepatternv1.FilterCondition{
				{Path: pathMetadataName, Operator: kubepatternv1.FilterExists, Values: []string{"unexpected"}},
			}
			Expect(validator.ValidateCreate(ctx, obj)).Error().To(MatchError(ContainSubstring("must be empty for operator EXISTS")))
		})

		It("should validate updates using the same rules", func() {
			oldObj := validTestPattern()
			newObj := validTestPattern()
			newObj.Spec.Dependencies = append(newObj.Spec.Dependencies, kubepatternv1.Dependency{
				ID: testDepID, Kind: "ConfigMap", APIVersion: "v1", PluralName: "configmaps",
			})
			Expect(validator.ValidateUpdate(ctx, oldObj, newObj)).Error().To(HaveOccurred())
		})
	})
})
