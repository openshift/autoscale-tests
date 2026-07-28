package autoscaler

import (
	"context"
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2" //nolint:staticcheck
	. "github.com/onsi/gomega"    //nolint:staticcheck

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"

	caov1 "github.com/openshift/cluster-autoscaler-operator/pkg/apis/autoscaling/v1"

	"github.com/openshift/autoscale-tests/cas/pkg/framework"
)

const (
	provisioningRequestFeatureGate = "ProvisioningRequestAvailable"
	enableProvisioningRequestsArg  = "--enable-provisioning-requests"
)

var _ = Describe("ProvisioningRequest should", framework.LabelAutoscaler, Serial, func() {
	var (
		client runtimeclient.Client
		ctx    context.Context
		err    error
	)

	BeforeEach(func() {
		ctx = context.Background()

		client, err = framework.LoadClient()
		Expect(err).NotTo(HaveOccurred(), "Failed to create Kubernetes client")

		framework.SkipIfNotDevPreviewOrTechPreview(ctx, client)

		crd := &unstructured.Unstructured{}
		crd.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "apiextensions.k8s.io",
			Version: "v1",
			Kind:    "CustomResourceDefinition",
		})
		if err := client.Get(ctx, runtimeclient.ObjectKey{
			Name: "provisioningrequests.autoscaling.x-k8s.io",
		}, crd); err != nil {
			Skip("ProvisioningRequest CRD not found, feature not available in this build")
		}
	})

	Context("have proper infrastructure on DevPreviewNoUpgrade clusters", func() {

		It("have the ProvisioningRequest CRD registered", func() {
			crd := &unstructured.Unstructured{}
			crd.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "apiextensions.k8s.io",
				Version: "v1",
				Kind:    "CustomResourceDefinition",
			})
			err := client.Get(ctx, runtimeclient.ObjectKey{
				Name: "provisioningrequests.autoscaling.x-k8s.io",
			}, crd)
			Expect(err).NotTo(HaveOccurred(), "ProvisioningRequest CRD should exist")
			klog.Infof("ProvisioningRequest CRD found: %s", crd.GetName())
		})

		It("have correct feature gate annotations on the ProvisioningRequest CRD", func() {
			crd := &unstructured.Unstructured{}
			crd.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "apiextensions.k8s.io",
				Version: "v1",
				Kind:    "CustomResourceDefinition",
			})
			err := client.Get(ctx, runtimeclient.ObjectKey{
				Name: "provisioningrequests.autoscaling.x-k8s.io",
			}, crd)
			Expect(err).NotTo(HaveOccurred())

			annotations := crd.GetAnnotations()
			Expect(annotations).To(HaveKeyWithValue(
				"feature-gate.release.openshift.io/"+provisioningRequestFeatureGate, "true"),
				"CRD should have ProvisioningRequestAvailable feature gate annotation")
			Expect(annotations).To(HaveKeyWithValue(
				"release.openshift.io/feature-set", "DevPreviewNoUpgrade"),
				"CRD should have DevPreviewNoUpgrade feature-set annotation")
			klog.Info("ProvisioningRequest CRD annotations verified")
		})

		It("have the operator ClusterRole for provisioning requests", func() {
			cr := &rbacv1.ClusterRole{}
			err := client.Get(ctx, runtimeclient.ObjectKey{
				Name: "cluster-autoscaler-operator-prov-req",
			}, cr)
			Expect(err).NotTo(HaveOccurred(), "Operator ProvisioningRequest ClusterRole should exist")

			By("Verifying correct API group in rules")
			hasProvReqRule := false
			for _, rule := range cr.Rules {
				for _, group := range rule.APIGroups {
					if group == framework.ProvisioningRequestGroup {
						hasProvReqRule = true
						break
					}
				}
			}
			Expect(hasProvReqRule).To(BeTrue(),
				"ClusterRole should have rules for autoscaling.x-k8s.io API group")

			By("Verifying feature gate annotations")
			Expect(cr.Annotations).To(HaveKeyWithValue(
				"feature-gate.release.openshift.io/"+provisioningRequestFeatureGate, "true"))
			Expect(cr.Annotations).To(HaveKeyWithValue(
				"release.openshift.io/feature-set", "DevPreviewNoUpgrade"))

			klog.Infof("Operator ClusterRole %q verified with correct rules and annotations", cr.Name)
		})

		It("have the autoscaler ClusterRole for provisioning requests", func() {
			cr := &rbacv1.ClusterRole{}
			err := client.Get(ctx, runtimeclient.ObjectKey{
				Name: "cluster-autoscaler-prov-req",
			}, cr)
			Expect(err).NotTo(HaveOccurred(), "Autoscaler ProvisioningRequest ClusterRole should exist")

			By("Verifying correct API group in rules")
			hasProvReqRule := false
			for _, rule := range cr.Rules {
				for _, group := range rule.APIGroups {
					if group == framework.ProvisioningRequestGroup {
						hasProvReqRule = true
						break
					}
				}
			}
			Expect(hasProvReqRule).To(BeTrue(),
				"ClusterRole should have rules for autoscaling.x-k8s.io API group")

			By("Verifying feature gate annotations")
			Expect(cr.Annotations).To(HaveKeyWithValue(
				"feature-gate.release.openshift.io/"+provisioningRequestFeatureGate, "true"))
			Expect(cr.Annotations).To(HaveKeyWithValue(
				"release.openshift.io/feature-set", "DevPreviewNoUpgrade"))

			klog.Infof("Autoscaler ClusterRole %q verified with correct rules and annotations", cr.Name)
		})

		It("have the operator ClusterRoleBinding for provisioning requests", func() {
			crb := &rbacv1.ClusterRoleBinding{}
			err := client.Get(ctx, runtimeclient.ObjectKey{
				Name: "cluster-autoscaler-operator-prov-req",
			}, crb)
			Expect(err).NotTo(HaveOccurred(), "Operator ProvisioningRequest ClusterRoleBinding should exist")

			Expect(crb.RoleRef.Name).To(Equal("cluster-autoscaler-operator-prov-req"))
			Expect(crb.RoleRef.Kind).To(Equal("ClusterRole"))

			hasSA := false
			for _, subject := range crb.Subjects {
				if subject.Kind == "ServiceAccount" &&
					subject.Name == "cluster-autoscaler-operator" &&
					subject.Namespace == framework.MachineAPINamespace {
					hasSA = true
					break
				}
			}
			Expect(hasSA).To(BeTrue(),
				"ClusterRoleBinding should reference cluster-autoscaler-operator ServiceAccount")
			klog.Infof("Operator ClusterRoleBinding %q verified", crb.Name)
		})

		It("have the autoscaler ClusterRoleBinding for provisioning requests", func() {
			crb := &rbacv1.ClusterRoleBinding{}
			err := client.Get(ctx, runtimeclient.ObjectKey{
				Name: "cluster-autoscaler-prov-req",
			}, crb)
			Expect(err).NotTo(HaveOccurred(), "Autoscaler ProvisioningRequest ClusterRoleBinding should exist")

			Expect(crb.RoleRef.Name).To(Equal("cluster-autoscaler-prov-req"))
			Expect(crb.RoleRef.Kind).To(Equal("ClusterRole"))

			hasSA := false
			for _, subject := range crb.Subjects {
				if subject.Kind == "ServiceAccount" &&
					subject.Name == "cluster-autoscaler" &&
					subject.Namespace == framework.MachineAPINamespace {
					hasSA = true
					break
				}
			}
			Expect(hasSA).To(BeTrue(),
				"ClusterRoleBinding should reference cluster-autoscaler ServiceAccount")
			klog.Infof("Autoscaler ClusterRoleBinding %q verified", crb.Name)
		})
	})

	Context("pass the enable flag to the cluster-autoscaler", func() {

		var (
			clusterAutoscaler *caov1.ClusterAutoscaler
			cleanupCA         bool
		)

		BeforeEach(func() {
			By("Ensuring a ClusterAutoscaler resource exists")
			existingCA, getErr := framework.GetClusterAutoscaler(client, "default")
			if getErr != nil {
				if !apierrors.IsNotFound(getErr) {
					Expect(getErr).NotTo(HaveOccurred())
				}
				By("Creating a ClusterAutoscaler for the test")
				clusterAutoscaler = &caov1.ClusterAutoscaler{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "default",
						Namespace: framework.MachineAPINamespace,
					},
					TypeMeta: metav1.TypeMeta{
						Kind:       "ClusterAutoscaler",
						APIVersion: "autoscaling.openshift.io/v1",
					},
					Spec: caov1.ClusterAutoscalerSpec{
						ResourceLimits: &caov1.ResourceLimits{},
						ScaleDown: &caov1.ScaleDownConfig{
							Enabled: true,
						},
					},
				}
				Expect(client.Create(ctx, clusterAutoscaler)).To(Succeed(),
					"Failed to create ClusterAutoscaler")
				cleanupCA = true
			} else {
				clusterAutoscaler = existingCA
				cleanupCA = false
			}
		})

		AfterEach(func() {
			if cleanupCA && clusterAutoscaler != nil {
				By("Cleaning up test ClusterAutoscaler")
				_ = client.Delete(ctx, clusterAutoscaler)
				Eventually(func() bool {
					_, err := framework.GetClusterAutoscaler(client, "default")
					return apierrors.IsNotFound(err)
				}, framework.WaitMedium, pollingInterval).Should(BeTrue())
			}
		})

		It("have --enable-provisioning-requests=true in the cluster-autoscaler deployment args", func() {
			By("Waiting for the cluster-autoscaler deployment to be ready")
			caDeploymentName := "cluster-autoscaler-" + clusterAutoscaler.Name
			var caDeployment appsv1.Deployment
			Eventually(func() error {
				if err := client.Get(ctx, runtimeclient.ObjectKey{
					Name:      caDeploymentName,
					Namespace: framework.MachineAPINamespace,
				}, &caDeployment); err != nil {
					return fmt.Errorf("cluster-autoscaler deployment %q not found: %w", caDeploymentName, err)
				}
				if caDeployment.Status.ReadyReplicas < 1 {
					return fmt.Errorf("cluster-autoscaler not ready yet (ready=%d)", caDeployment.Status.ReadyReplicas)
				}
				return nil
			}, framework.WaitMedium, pollingInterval).Should(Succeed(),
				"cluster-autoscaler deployment should be running")

			By("Checking for --enable-provisioning-requests=true in container args")
			found := false
			for _, container := range caDeployment.Spec.Template.Spec.Containers {
				for _, arg := range container.Args {
					if strings.Contains(arg, enableProvisioningRequestsArg) &&
						strings.Contains(arg, "true") {
						found = true
						break
					}
				}
				if !found {
					for _, arg := range container.Command {
						if strings.Contains(arg, enableProvisioningRequestsArg) &&
							strings.Contains(arg, "true") {
							found = true
							break
						}
					}
				}
			}
			Expect(found).To(BeTrue(),
				"cluster-autoscaler deployment should have %s=true in args", enableProvisioningRequestsArg)
			klog.Infof("cluster-autoscaler deployment has %s=true", enableProvisioningRequestsArg)
		})
	})

	Context("support CRUD operations on ProvisioningRequest objects", func() {

		It("create a ProvisioningRequest successfully", func() {
			prName := fmt.Sprintf("e2e-provreq-create-%d", time.Now().UnixNano())

			podTemplate := newTestPodTemplate(prName + "-pod-template")
			Expect(client.Create(ctx, podTemplate)).To(Succeed())
			defer func() {
				_ = client.Delete(ctx, podTemplate)
			}()

			pr := framework.NewProvisioningRequest(framework.ProvisioningRequestConfig{
				Name:              prName,
				Namespace:         framework.MachineAPINamespace,
				ProvisioningClass: "check-capacity.autoscaling.x-k8s.io",
				PodTemplateName:   podTemplate.Name,
				PodCount:          1,
			})
			Expect(client.Create(ctx, pr)).To(Succeed(),
				"Should be able to create a ProvisioningRequest")
			defer func() {
				_ = client.Delete(ctx, pr)
			}()

			klog.Infof("ProvisioningRequest %q created successfully", prName)
		})

		It("get a ProvisioningRequest after creation", func() {
			prName := fmt.Sprintf("e2e-provreq-get-%d", time.Now().UnixNano())

			podTemplate := newTestPodTemplate(prName + "-pod-template")
			Expect(client.Create(ctx, podTemplate)).To(Succeed())
			defer func() {
				_ = client.Delete(ctx, podTemplate)
			}()

			pr := framework.NewProvisioningRequest(framework.ProvisioningRequestConfig{
				Name:              prName,
				Namespace:         framework.MachineAPINamespace,
				ProvisioningClass: "check-capacity.autoscaling.x-k8s.io",
				PodTemplateName:   podTemplate.Name,
				PodCount:          1,
			})
			Expect(client.Create(ctx, pr)).To(Succeed())
			defer func() {
				_ = client.Delete(ctx, pr)
			}()

			By("Getting the ProvisioningRequest")
			fetched := &unstructured.Unstructured{}
			fetched.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   framework.ProvisioningRequestGroup,
				Version: framework.ProvisioningRequestVersion,
				Kind:    framework.ProvisioningRequestKind,
			})
			Expect(client.Get(ctx, runtimeclient.ObjectKey{
				Name:      prName,
				Namespace: framework.MachineAPINamespace,
			}, fetched)).To(Succeed())

			Expect(fetched.GetName()).To(Equal(prName))

			provClass, _, _ := unstructured.NestedString(fetched.Object, "spec", "provisioningClassName")
			Expect(provClass).To(Equal("check-capacity.autoscaling.x-k8s.io"))

			klog.Infof("ProvisioningRequest %q fetched with provisioningClassName=%s", prName, provClass)
		})

		It("list ProvisioningRequests in a namespace", func() {
			prNames := make([]string, 3)
			var podTemplates []*corev1.PodTemplate
			for i := range prNames {
				prNames[i] = fmt.Sprintf("e2e-provreq-list-%d-%d", i, time.Now().UnixNano())
				pt := newTestPodTemplate(prNames[i] + "-pod-template")
				Expect(client.Create(ctx, pt)).To(Succeed())
				podTemplates = append(podTemplates, pt)

				pr := framework.NewProvisioningRequest(framework.ProvisioningRequestConfig{
					Name:              prNames[i],
					Namespace:         framework.MachineAPINamespace,
					ProvisioningClass: "check-capacity.autoscaling.x-k8s.io",
					PodTemplateName:   pt.Name,
					PodCount:          1,
				})
				Expect(client.Create(ctx, pr)).To(Succeed())
			}
			defer func() {
				for _, name := range prNames {
					pr := &unstructured.Unstructured{}
					pr.SetGroupVersionKind(schema.GroupVersionKind{
						Group:   framework.ProvisioningRequestGroup,
						Version: framework.ProvisioningRequestVersion,
						Kind:    framework.ProvisioningRequestKind,
					})
					pr.SetName(name)
					pr.SetNamespace(framework.MachineAPINamespace)
					_ = client.Delete(ctx, pr)
				}
				for _, pt := range podTemplates {
					_ = client.Delete(ctx, pt)
				}
			}()

			By("Listing ProvisioningRequests")
			prList := &unstructured.UnstructuredList{}
			prList.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   framework.ProvisioningRequestGroup,
				Version: framework.ProvisioningRequestVersion,
				Kind:    framework.ProvisioningRequestKind + "List",
			})
			Expect(client.List(ctx, prList,
				runtimeclient.InNamespace(framework.MachineAPINamespace))).To(Succeed())
			Expect(len(prList.Items)).To(BeNumerically(">=", 3),
				"Should list at least 3 ProvisioningRequests we created")

			klog.Infof("Listed %d ProvisioningRequests in namespace %s",
				len(prList.Items), framework.MachineAPINamespace)
		})

		It("delete a ProvisioningRequest", func() {
			prName := fmt.Sprintf("e2e-provreq-del-%d", time.Now().UnixNano())

			podTemplate := newTestPodTemplate(prName + "-pod-template")
			Expect(client.Create(ctx, podTemplate)).To(Succeed())
			defer func() {
				_ = client.Delete(ctx, podTemplate)
			}()

			pr := framework.NewProvisioningRequest(framework.ProvisioningRequestConfig{
				Name:              prName,
				Namespace:         framework.MachineAPINamespace,
				ProvisioningClass: "check-capacity.autoscaling.x-k8s.io",
				PodTemplateName:   podTemplate.Name,
				PodCount:          1,
			})
			Expect(client.Create(ctx, pr)).To(Succeed())

			By("Deleting the ProvisioningRequest")
			Expect(client.Delete(ctx, pr)).To(Succeed())

			By("Verifying deletion")
			fetched := &unstructured.Unstructured{}
			fetched.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   framework.ProvisioningRequestGroup,
				Version: framework.ProvisioningRequestVersion,
				Kind:    framework.ProvisioningRequestKind,
			})
			err := client.Get(ctx, runtimeclient.ObjectKey{
				Name:      prName,
				Namespace: framework.MachineAPINamespace,
			}, fetched)
			Expect(apierrors.IsNotFound(err)).To(BeTrue(),
				"ProvisioningRequest should not exist after deletion")
			klog.Infof("ProvisioningRequest %q deleted and confirmed gone", prName)
		})
	})

	Context("process ProvisioningRequests via the cluster-autoscaler", func() {

		var (
			clusterAutoscaler *caov1.ClusterAutoscaler
			cleanupCA         bool
		)

		BeforeEach(func() {
			existingCA, getErr := framework.GetClusterAutoscaler(client, "default")
			if getErr != nil {
				if !apierrors.IsNotFound(getErr) {
					Expect(getErr).NotTo(HaveOccurred())
				}
				clusterAutoscaler = &caov1.ClusterAutoscaler{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "default",
						Namespace: framework.MachineAPINamespace,
					},
					TypeMeta: metav1.TypeMeta{
						Kind:       "ClusterAutoscaler",
						APIVersion: "autoscaling.openshift.io/v1",
					},
					Spec: caov1.ClusterAutoscalerSpec{
						ResourceLimits: &caov1.ResourceLimits{},
						ScaleDown: &caov1.ScaleDownConfig{
							Enabled: true,
						},
					},
				}
				Expect(client.Create(ctx, clusterAutoscaler)).To(Succeed())
				cleanupCA = true
			} else {
				clusterAutoscaler = existingCA
				cleanupCA = false
			}

			By("Waiting for cluster-autoscaler to be ready")
			caDeploymentName := "cluster-autoscaler-" + clusterAutoscaler.Name
			Eventually(func() error {
				var caDeployment appsv1.Deployment
				if err := client.Get(ctx, runtimeclient.ObjectKey{
					Name:      caDeploymentName,
					Namespace: framework.MachineAPINamespace,
				}, &caDeployment); err != nil {
					return fmt.Errorf("cluster-autoscaler deployment %q not found: %w", caDeploymentName, err)
				}
				if caDeployment.Status.ReadyReplicas < 1 {
					return fmt.Errorf("cluster-autoscaler not ready")
				}
				return nil
			}, framework.WaitMedium, pollingInterval).Should(Succeed())
		})

		AfterEach(func() {
			if cleanupCA && clusterAutoscaler != nil {
				_ = client.Delete(ctx, clusterAutoscaler)
				Eventually(func() bool {
					_, err := framework.GetClusterAutoscaler(client, "default")
					return apierrors.IsNotFound(err)
				}, framework.WaitMedium, pollingInterval).Should(BeTrue())
			}
		})

		It("set status conditions on a check-capacity ProvisioningRequest", func() {
			prName := fmt.Sprintf("e2e-provreq-status-%d", time.Now().UnixNano())

			podTemplate := newTestPodTemplate(prName + "-pod-template")
			Expect(client.Create(ctx, podTemplate)).To(Succeed())
			defer func() {
				_ = client.Delete(ctx, podTemplate)
			}()

			pr := framework.NewProvisioningRequest(framework.ProvisioningRequestConfig{
				Name:              prName,
				Namespace:         framework.MachineAPINamespace,
				ProvisioningClass: "check-capacity.autoscaling.x-k8s.io",
				PodTemplateName:   podTemplate.Name,
				PodCount:          1,
			})
			Expect(client.Create(ctx, pr)).To(Succeed())
			defer func() {
				_ = client.Delete(ctx, pr)
			}()

			By("Waiting for autoscaler to set status conditions on the ProvisioningRequest")
			Eventually(func() bool {
				fetched := &unstructured.Unstructured{}
				fetched.SetGroupVersionKind(schema.GroupVersionKind{
					Group:   framework.ProvisioningRequestGroup,
					Version: framework.ProvisioningRequestVersion,
					Kind:    framework.ProvisioningRequestKind,
				})
				if err := client.Get(ctx, runtimeclient.ObjectKey{
					Name:      prName,
					Namespace: framework.MachineAPINamespace,
				}, fetched); err != nil {
					return false
				}
				conditions, found, _ := unstructured.NestedSlice(fetched.Object, "status", "conditions")
				return found && len(conditions) > 0
			}, framework.WaitOverMedium, pollingInterval).Should(BeTrue(),
				"Autoscaler should eventually set status conditions on ProvisioningRequest")

			fetched := &unstructured.Unstructured{}
			fetched.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   framework.ProvisioningRequestGroup,
				Version: framework.ProvisioningRequestVersion,
				Kind:    framework.ProvisioningRequestKind,
			})
			Expect(client.Get(ctx, runtimeclient.ObjectKey{
				Name:      prName,
				Namespace: framework.MachineAPINamespace,
			}, fetched)).To(Succeed())

			conditions, _, _ := unstructured.NestedSlice(fetched.Object, "status", "conditions")
			klog.Infof("ProvisioningRequest %q has %d status conditions:", prName, len(conditions))
			for _, c := range conditions {
				if cond, ok := c.(map[string]interface{}); ok {
					klog.Infof("  type=%v status=%v reason=%v message=%v",
						cond["type"], cond["status"], cond["reason"], cond["message"])
				}
			}
		})

		It("accept a ProvisioningRequest with atomic-scale-up class", func() {
			prName := fmt.Sprintf("e2e-provreq-atomic-%d", time.Now().UnixNano())

			podTemplate := newTestPodTemplate(prName + "-pod-template")
			Expect(client.Create(ctx, podTemplate)).To(Succeed())
			defer func() {
				_ = client.Delete(ctx, podTemplate)
			}()

			pr := framework.NewProvisioningRequest(framework.ProvisioningRequestConfig{
				Name:              prName,
				Namespace:         framework.MachineAPINamespace,
				ProvisioningClass: "atomic-scale-up.autoscaling.x-k8s.io",
				PodTemplateName:   podTemplate.Name,
				PodCount:          1,
			})
			Expect(client.Create(ctx, pr)).To(Succeed())
			defer func() {
				_ = client.Delete(ctx, pr)
			}()

			By("Verifying the ProvisioningRequest exists in the API")
			fetched := &unstructured.Unstructured{}
			fetched.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   framework.ProvisioningRequestGroup,
				Version: framework.ProvisioningRequestVersion,
				Kind:    framework.ProvisioningRequestKind,
			})
			Expect(client.Get(ctx, runtimeclient.ObjectKey{
				Name:      prName,
				Namespace: framework.MachineAPINamespace,
			}, fetched)).To(Succeed())
			Expect(fetched.GetName()).To(Equal(prName))

			klog.Infof("ProvisioningRequest %q with atomic-scale-up class accepted", prName)
		})
	})
})

func newTestPodTemplate(name string) *corev1.PodTemplate {
	return &corev1.PodTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: framework.MachineAPINamespace,
		},
		Template: corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "pause",
						Image: "registry.k8s.io/pause:3.9",
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("100m"),
								corev1.ResourceMemory: resource.MustParse("128Mi"),
							},
						},
					},
				},
			},
		},
	}
}
