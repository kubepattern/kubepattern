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
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	kubepatternv1 "kubepattern-go/api/v1"
)

// nolint:unused
// log is for logging in this package.
var patternlog = logf.Log.WithName("pattern-resource")

// SetupPatternWebhookWithManager registers the webhook for Pattern in the manager.
func SetupPatternWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &kubepatternv1.Pattern{}).
		WithValidator(&PatternCustomValidator{}).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// NOTE: If you want to customise the 'path', use the flags '--defaulting-path' or '--validation-path'.
// +kubebuilder:webhook:path=/validate-kubepattern-dev-v1-pattern,mutating=false,failurePolicy=fail,sideEffects=None,groups=kubepattern.dev,resources=patterns,verbs=create;update,versions=v1,name=vpattern-v1.kb.io,admissionReviewVersions=v1

// PatternCustomValidator struct is responsible for validating the Pattern resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type PatternCustomValidator struct {
	// TODO(user): Add more fields as needed for validation
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type Pattern.
func (v *PatternCustomValidator) ValidateCreate(_ context.Context, obj *kubepatternv1.Pattern) (admission.Warnings, error) {
	patternlog.Info("Validation for Pattern upon creation", "name", obj.GetName())

	return nil, validatePattern(obj)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type Pattern.
func (v *PatternCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj *kubepatternv1.Pattern) (admission.Warnings, error) {
	patternlog.Info("Validation for Pattern upon update", "name", newObj.GetName())

	return nil, validatePattern(newObj)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type Pattern.
func (v *PatternCustomValidator) ValidateDelete(_ context.Context, obj *kubepatternv1.Pattern) (admission.Warnings, error) {
	patternlog.Info("Validation for Pattern upon deletion", "name", obj.GetName())

	return nil, nil
}

// validatePattern checks the cross-field rules of a Pattern's spec that the CRD's
// OpenAPI schema cannot express on its own (dependency id uniqueness, relationship
// references, and filter operator/values pairing). Everything expressible as a
// structural schema constraint (required fields, enums, regexes) is already
// enforced by the markers on PatternSpec and its nested types.
func validatePattern(p *kubepatternv1.Pattern) error {
	var allErrs field.ErrorList
	specPath := field.NewPath("spec")

	depIDs := make(map[string]struct{}, len(p.Spec.Dependencies))
	for i, dep := range p.Spec.Dependencies {
		depPath := specPath.Child("dependencies").Index(i)
		if _, exists := depIDs[dep.ID]; exists {
			allErrs = append(allErrs, field.Duplicate(depPath.Child("id"), dep.ID))
		}
		depIDs[dep.ID] = struct{}{}
		allErrs = append(allErrs, validateFilters(depPath.Child("filters"), dep.Filters)...)
	}

	allErrs = append(allErrs, validateFilters(specPath.Child("target", "filters"), p.Spec.Target.Filters)...)

	relPath := specPath.Child("relationships")
	allErrs = append(allErrs, validateRelationshipGroup(relPath.Child("matchAll"), p.Spec.Relationships.MatchAll, depIDs)...)
	allErrs = append(allErrs, validateRelationshipGroup(relPath.Child("matchAny"), p.Spec.Relationships.MatchAny, depIDs)...)
	allErrs = append(allErrs, validateRelationshipGroup(relPath.Child("matchNone"), p.Spec.Relationships.MatchNone, depIDs)...)

	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(
		kubepatternv1.GroupVersion.WithKind("Pattern").GroupKind(),
		p.Name, allErrs)
}

// validateFilters checks the operator/values pairing of every FilterCondition
// across a Filters' matchAll/matchAny/matchNone groups.
func validateFilters(path *field.Path, f kubepatternv1.Filters) field.ErrorList {
	var allErrs field.ErrorList
	allErrs = append(allErrs, validateFilterConditions(path.Child("matchAll"), f.MatchAll)...)
	allErrs = append(allErrs, validateFilterConditions(path.Child("matchAny"), f.MatchAny)...)
	allErrs = append(allErrs, validateFilterConditions(path.Child("matchNone"), f.MatchNone)...)
	return allErrs
}

// validateFilterConditions checks that EXISTS/IS_EMPTY conditions carry no values
// and every other operator carries at least one.
func validateFilterConditions(path *field.Path, conditions []kubepatternv1.FilterCondition) field.ErrorList {
	var allErrs field.ErrorList
	for i, c := range conditions {
		valuesPath := path.Index(i).Child("values")
		switch c.Operator {
		case kubepatternv1.FilterExists, kubepatternv1.FilterIsEmpty:
			if len(c.Values) > 0 {
				allErrs = append(allErrs, field.Forbidden(valuesPath,
					fmt.Sprintf("must be empty for operator %s", c.Operator)))
			}
		default:
			if len(c.Values) == 0 {
				allErrs = append(allErrs, field.Required(valuesPath,
					fmt.Sprintf("must not be empty for operator %s", c.Operator)))
			}
		}
	}
	return allErrs
}

// validateRelationshipGroup checks that each Relationship's "with" references a
// declared dependency id, and that criteria is set only for the "custom" type.
func validateRelationshipGroup(path *field.Path, rels []kubepatternv1.Relationship, depIDs map[string]struct{}) field.ErrorList {
	var allErrs field.ErrorList
	for i, rel := range rels {
		relPath := path.Index(i)

		if _, exists := depIDs[rel.With]; !exists {
			allErrs = append(allErrs, field.Invalid(relPath.Child("with"), rel.With,
				"does not match any dependency id"))
		}

		switch rel.Type {
		case kubepatternv1.RelationshipCustom:
			if len(rel.Criteria) == 0 {
				allErrs = append(allErrs, field.Required(relPath.Child("criteria"),
					"must have at least one criteria for relationship type 'custom'"))
			}
		default:
			if len(rel.Criteria) > 0 {
				allErrs = append(allErrs, field.Forbidden(relPath.Child("criteria"),
					fmt.Sprintf("must not be set for relationship type %q", rel.Type)))
			}
		}
	}
	return allErrs
}
