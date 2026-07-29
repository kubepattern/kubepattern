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

package controller

import (
	"context"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	kubepatternv1 "kubepattern-go/api/v1"
	"kubepattern-go/internal/analysis"
	"kubepattern-go/internal/cluster"
	"kubepattern-go/internal/kube"
)

// PatternReconciler reconciles a Pattern object
type PatternReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// KubeClient performs the dynamic, cross-GVK cluster reads a Pattern's
	// target/dependencies describe at runtime. It is a single long-lived
	// instance shared across every Reconcile call (built once in cmd/main.go).
	KubeClient *kube.Client

	// SaveInNamespace and TargetNamespace configure where emitted Smells are
	// persisted; see kube.NewSmellWriter.
	SaveInNamespace bool
	TargetNamespace string

	// RequeueInterval is how often a Pattern is re-evaluated. Analysis is
	// re-run on this cadence rather than in response to arbitrary cluster
	// changes, since a Pattern's targets/dependencies are dynamic GVKs not
	// known until the Pattern itself is read.
	RequeueInterval time.Duration
}

// +kubebuilder:rbac:groups=kubepattern.dev,resources=patterns,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubepattern.dev,resources=patterns/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kubepattern.dev,resources=patterns/finalizers,verbs=update
// +kubebuilder:rbac:groups=kubepattern.dev,resources=smells,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=*,resources=*,verbs=get;list;watch

// Reconcile fetches a Pattern's target and dependency resources from across
// the cluster, evaluates it against the current state of the graph, and
// writes/prunes the resulting Smells. It always requeues after
// RequeueInterval so patterns are re-evaluated continuously, mirroring the
// cadence of the CronJob this controller replaces.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.24.1/pkg/reconcile
func (r *PatternReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	pattern := &kubepatternv1.Pattern{}
	if err := r.Get(ctx, req.NamespacedName, pattern); err != nil {
		if apierrors.IsNotFound(err) {
			// Pattern was deleted; nothing to reconcile. Its previously emitted
			// Smells are orphaned intentionally — pruning them would require a
			// finalizer, which is left as a follow-up.
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	requestResources := []kube.Resource{{
		APIVersion: pattern.Spec.Target.APIVersion,
		Kind:       pattern.Spec.Target.Kind,
		Resource:   pattern.Spec.Target.PluralName,
	}}
	for _, dependency := range pattern.Spec.Dependencies {
		requestResources = append(requestResources, kube.Resource{
			APIVersion: dependency.APIVersion,
			Kind:       dependency.Kind,
			Resource:   dependency.PluralName,
		})
	}

	resources, err := r.KubeClient.FetchSelectedWithInheritance(requestResources, ctx)
	if err != nil {
		// Missing RBAC or a transient discovery error should not crash the
		// manager — log and retry on the normal cadence rather than hot-looping.
		log.Error(err, "Failed to fetch cluster resources for pattern, will retry", "pattern", pattern.Name)
		return ctrl.Result{RequeueAfter: r.RequeueInterval}, nil
	}

	graph := cluster.NewGraph()
	graph.Build(resources)

	smellWriter := kube.NewSmellWriter(r.KubeClient, r.SaveInNamespace, r.TargetNamespace)
	engine := analysis.NewEngine(graph, smellWriter)

	if err := engine.Run(ctx, pattern); err != nil {
		log.Error(err, "Analysis run failed for pattern, will retry", "pattern", pattern.Name)
		return ctrl.Result{RequeueAfter: r.RequeueInterval}, nil
	}

	log.Info("Reconciled pattern", "pattern", pattern.Name, "nodes", len(graph.GetNodes()))
	return ctrl.Result{RequeueAfter: r.RequeueInterval}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PatternReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubepatternv1.Pattern{}).
		Named("pattern").
		Complete(r)
}
