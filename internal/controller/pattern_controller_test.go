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
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kubepatternv1 "kubepattern-go/api/v1"
	"kubepattern-go/internal/kube"
)

var _ = Describe("Pattern Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			resourceName = "test-resource"
			deployName   = "test-deployment"
			namespace    = "default"
		)

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name: resourceName,
		}
		pattern := &kubepatternv1.Pattern{}
		deployment := &appsv1.Deployment{}

		BeforeEach(func() {
			By("creating a Deployment for the pattern to target")
			replicas := int32(1)
			deploy := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      deployName,
					Namespace: namespace,
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: &replicas,
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": deployName},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{"app": deployName},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{{
								Name:  "app",
								Image: "nginx:latest",
							}},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, deploy)).To(Succeed())
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: deployName, Namespace: namespace}, deployment)).To(Succeed())

			By("creating the custom resource for the Kind Pattern")
			err := k8sClient.Get(ctx, typeNamespacedName, pattern)
			if err != nil && errors.IsNotFound(err) {
				resource := &kubepatternv1.Pattern{
					ObjectMeta: metav1.ObjectMeta{
						Name: resourceName,
					},
					Spec: kubepatternv1.PatternSpec{
						DisplayName: "Test Pattern",
						Category:    "test-category",
						Severity:    kubepatternv1.SeverityLow,
						Message:     "found deployment {{target.metadata.name}}",
						Target: kubepatternv1.Target{
							Kind:       "Deployment",
							APIVersion: "apps/v1",
							PluralName: "deployments",
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			By("Cleanup the specific resource instance Pattern")
			resource := &kubepatternv1.Pattern{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())

			By("Cleanup the Deployment")
			Expect(k8sClient.Delete(ctx, deployment)).To(Succeed())

			By("Cleanup any Smell produced by the reconcile")
			smell := &kubepatternv1.Smell{}
			smellName := fmt.Sprintf("%s-%s", resourceName, deployment.GetUID())
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: smellName, Namespace: namespace}, smell); err == nil {
				Expect(k8sClient.Delete(ctx, smell)).To(Succeed())
			}
		})

		It("should emit a Smell for the matching Deployment", func() {
			By("Reconciling the created resource")
			kubeClient, err := kube.NewClient(cfg)
			Expect(err).NotTo(HaveOccurred())

			controllerReconciler := &PatternReconciler{
				Client:          k8sClient,
				Scheme:          k8sClient.Scheme(),
				KubeClient:      kubeClient,
				SaveInNamespace: true,
				TargetNamespace: namespace,
				RequeueInterval: time.Minute,
			}

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(time.Minute))

			By("verifying the Smell was created with the expected fields")
			smell := &kubepatternv1.Smell{}
			smellName := fmt.Sprintf("%s-%s", resourceName, deployment.GetUID())
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: smellName, Namespace: namespace}, smell)
			}).Should(Succeed())

			Expect(smell.Spec.Category).To(Equal("test-category"))
			Expect(smell.Spec.Severity).To(Equal(kubepatternv1.SeverityLow))
			Expect(smell.Spec.Message).To(Equal("found deployment " + deployName))
			Expect(smell.Spec.Target.Kind).To(Equal("Deployment"))
			Expect(smell.Spec.Target.Name).To(Equal(deployName))
			Expect(smell.Spec.Target.Namespace).To(Equal(namespace))
		})

		It("should return NotFound gracefully for a deleted Pattern", func() {
			kubeClient, err := kube.NewClient(cfg)
			Expect(err).NotTo(HaveOccurred())

			controllerReconciler := &PatternReconciler{
				Client:          k8sClient,
				Scheme:          k8sClient.Scheme(),
				KubeClient:      kubeClient,
				SaveInNamespace: true,
				TargetNamespace: namespace,
				RequeueInterval: time.Minute,
			}

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: "does-not-exist"},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeZero())
		})
	})
})
