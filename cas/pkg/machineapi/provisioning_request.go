package autoscaler

import (
	"context"
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2" //nolint:staticcheck
	. "github.com/onsi/gomega"    //nolint:staticcheck

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog"
	"k8s.io/utils/ptr"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"

	configv1 "github.com/openshift/api/config/v1"
	machinev1 "github.com/openshift/api/machine/v1beta1"
	caov1 "github.com/openshift/cluster-autoscaler-operator/pkg/apis/autoscaling/v1"
	caov1beta1 "github.com/openshift/cluster-autoscaler-operator/pkg/apis/autoscaling/v1beta1"
	annotationsutil "github.com/openshift/machine-api-operator/pkg/util/machineset"

	"github.com/openshift/autoscale-tests/cas/pkg/framework"
)

var _ = Describe("ProvisioningRequest should", framework.LabelAutoscaler, framework.LabelDisruptive, Serial, func() {
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

	Context("have proper infrastructure", func() {

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

		It("have the ClusterRoles for provisioning requests", func() {
			By("Checking operator ClusterRole")
			cr := &rbacv1.ClusterRole{}
			err := client.Get(ctx, runtimeclient.ObjectKey{
				Name: "cluster-autoscaler-operator-prov-req",
			}, cr)
			Expect(err).NotTo(HaveOccurred(), "Operator ProvisioningRequest ClusterRole should exist")

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

			By("Checking autoscaler ClusterRole")
			cr2 := &rbacv1.ClusterRole{}
			err = client.Get(ctx, runtimeclient.ObjectKey{
				Name: "cluster-autoscaler-prov-req",
			}, cr2)
			Expect(err).NotTo(HaveOccurred(), "Autoscaler ProvisioningRequest ClusterRole should exist")

			hasProvReqRule = false
			for _, rule := range cr2.Rules {
				for _, group := range rule.APIGroups {
					if group == framework.ProvisioningRequestGroup {
						hasProvReqRule = true
						break
					}
				}
			}
			Expect(hasProvReqRule).To(BeTrue(),
				"ClusterRole should have rules for autoscaling.x-k8s.io API group")

			klog.Info("Both ProvisioningRequest ClusterRoles verified")
		})

		It("have the ClusterRoleBindings for provisioning requests", func() {
			By("Checking operator ClusterRoleBinding")
			crb := &rbacv1.ClusterRoleBinding{}
			err := client.Get(ctx, runtimeclient.ObjectKey{
				Name: "cluster-autoscaler-operator-prov-req",
			}, crb)
			Expect(err).NotTo(HaveOccurred(), "Operator ProvisioningRequest ClusterRoleBinding should exist")
			Expect(crb.RoleRef.Name).To(Equal("cluster-autoscaler-operator-prov-req"))

			By("Checking autoscaler ClusterRoleBinding")
			crb2 := &rbacv1.ClusterRoleBinding{}
			err = client.Get(ctx, runtimeclient.ObjectKey{
				Name: "cluster-autoscaler-prov-req",
			}, crb2)
			Expect(err).NotTo(HaveOccurred(), "Autoscaler ProvisioningRequest ClusterRoleBinding should exist")
			Expect(crb2.RoleRef.Name).To(Equal("cluster-autoscaler-prov-req"))

			klog.Info("Both ProvisioningRequest ClusterRoleBindings verified")
		})
	})

	Context("scale up a MachineSet via atomic-scale-up ProvisioningRequest", func() {

		var (
			clusterAutoscaler *caov1.ClusterAutoscaler
			machineAutoscaler *caov1beta1.MachineAutoscaler
			machineSet        *machinev1.MachineSet
			podTemplate       *corev1.PodTemplate
			workload          *batchv1.Job
			targetedNodeLabel string
			cleanupCA         bool
		)

		BeforeEach(func() {
			By("Checking platform supports scale from zero")
			clusterInfra, infraErr := framework.GetInfrastructure(ctx, client)
			Expect(infraErr).NotTo(HaveOccurred(), "Failed to get cluster infrastructure object")

			platform := clusterInfra.Status.PlatformStatus.Type
			switch platform {
			case configv1.AWSPlatformType, configv1.GCPPlatformType, configv1.AzurePlatformType, configv1.OpenStackPlatformType, configv1.VSpherePlatformType, configv1.NutanixPlatformType:
				klog.Infof("Platform is %v", platform)
			default:
				Skip(fmt.Sprintf("Platform %v does not support autoscaling from/to zero, skipping.", platform))
			}

			By("Ensuring a ClusterAutoscaler resource exists")
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
						ResourceLimits: &caov1.ResourceLimits{
							MaxNodesTotal: ptr.To[int32](100),
						},
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

			By("Verifying cluster-autoscaler has --enable-provisioning-requests=true")
			var caDeployment appsv1.Deployment
			Eventually(func() error {
				return client.Get(ctx, runtimeclient.ObjectKey{
					Name:      "cluster-autoscaler-" + clusterAutoscaler.Name,
					Namespace: framework.MachineAPINamespace,
				}, &caDeployment)
			}, framework.WaitMedium, pollingInterval).Should(Succeed(),
				"cluster-autoscaler deployment should exist")

			provReqEnabled := false
			for _, container := range caDeployment.Spec.Template.Spec.Containers {
				for _, arg := range container.Args {
					if strings.Contains(arg, "--enable-provisioning-requests") && strings.Contains(arg, "true") {
						provReqEnabled = true
						break
					}
				}
			}
			if !provReqEnabled {
				Skip("cluster-autoscaler does not have --enable-provisioning-requests=true flag")
			}

			By("Creating a new MachineSet with 0 replicas")
			machineSetParams := framework.BuildMachineSetParams(ctx, client, 0)
			targetedNodeLabel = "machine.openshift.io/provreq-e2e-worker"
			machineSetParams.Labels[targetedNodeLabel] = ""
			machineSet, err = framework.CreateMachineSet(client, machineSetParams)
			Expect(err).ToNot(HaveOccurred(), "Failed to create MachineSet with 0 replicas")

			framework.WaitForMachineSet(ctx, client, machineSet.GetName())

			By("Waiting for scale-from-zero annotations on the MachineSet")
			Eventually(func() (map[string]string, error) {
				ms, err := framework.GetMachineSet(ctx, client, machineSet.GetName())
				if err != nil {
					return nil, err
				}
				return ms.Annotations, nil
			}, framework.WaitMedium, pollingInterval).Should(SatisfyAll(
				HaveKey(annotationsutil.CpuKeyDeprecated),
				HaveKey(annotationsutil.MemoryKeyDeprecated),
			), "MachineSet should have scale-from-zero capacity annotations")

			By("Creating a MachineAutoscaler for the MachineSet")
			machineAutoscaler = &caov1beta1.MachineAutoscaler{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "provreq-e2e-",
					Namespace:    framework.MachineAPINamespace,
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "MachineAutoscaler",
					APIVersion: "autoscaling.openshift.io/v1beta1",
				},
				Spec: caov1beta1.MachineAutoscalerSpec{
					MaxReplicas: 3,
					MinReplicas: 0,
					ScaleTargetRef: caov1beta1.CrossVersionObjectReference{
						Name:       machineSet.Name,
						Kind:       "MachineSet",
						APIVersion: "machine.openshift.io/v1beta1",
					},
				},
			}
			Expect(client.Create(ctx, machineAutoscaler)).To(Succeed(),
				"Failed to create MachineAutoscaler")

			By("Creating a PodTemplate for the ProvisioningRequest")
			podTemplate = &corev1.PodTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:      machineSet.Name + "-pod-template",
					Namespace: framework.MachineAPINamespace,
				},
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Affinity: &corev1.Affinity{
							NodeAffinity: &corev1.NodeAffinity{
								RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
									NodeSelectorTerms: []corev1.NodeSelectorTerm{
										{
											MatchExpressions: []corev1.NodeSelectorRequirement{
												{
													Key:      targetedNodeLabel,
													Operator: corev1.NodeSelectorOpExists,
												},
											},
										},
									},
								},
							},
						},
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
			Expect(client.Create(ctx, podTemplate)).To(Succeed(),
				"Failed to create PodTemplate")
		})

		AfterEach(func() {
			By("Cleaning up test resources")
			if workload != nil {
				_ = client.Delete(ctx, workload, runtimeclient.PropagationPolicy(metav1.DeletePropagationBackground))
			}
			if podTemplate != nil {
				_ = client.Delete(ctx, podTemplate)
			}
			if machineAutoscaler != nil {
				_ = client.Delete(ctx, machineAutoscaler)
			}
			if machineSet != nil {
				_ = client.Delete(ctx, machineSet)
				framework.WaitForMachineSetsDeleted(ctx, client, machineSet)
			}
			if cleanupCA && clusterAutoscaler != nil {
				_ = client.Delete(ctx, clusterAutoscaler)
				Eventually(func() bool {
					_, err := framework.GetClusterAutoscaler(client, "default")
					return apierrors.IsNotFound(err)
				}, framework.WaitMedium, pollingInterval).Should(BeTrue())
			}
		})

		It("scale up the MachineSet when a ProvisioningRequest with atomic-scale-up is created", func() {
			prName := fmt.Sprintf("e2e-provreq-scaleup-%d", time.Now().UnixNano())
			expectedReplicas := int32(1)

			By("Creating an atomic-scale-up ProvisioningRequest")
			pr := framework.NewProvisioningRequest(framework.ProvisioningRequestConfig{
				Name:              prName,
				Namespace:         framework.MachineAPINamespace,
				ProvisioningClass: "atomic-scale-up.autoscaling.x-k8s.io",
				PodTemplateName:   podTemplate.Name,
				PodCount:          int64(expectedReplicas),
			})
			Expect(client.Create(ctx, pr)).To(Succeed(),
				"Should be able to create ProvisioningRequest")
			defer func() {
				_ = client.Delete(ctx, pr)
			}()

			klog.Infof("Created ProvisioningRequest %s with class atomic-scale-up targeting PodTemplate %s",
				prName, podTemplate.Name)

			By("Creating workload pods targeting the MachineSet to ensure scale-up")
			workload = framework.NewWorkLoad(expectedReplicas, resource.MustParse("128Mi"),
				fmt.Sprintf("provreq-workload-%d", time.Now().Unix()),
				autoscalingTestLabel, "provisioning-request",
				corev1.NodeSelectorRequirement{
					Key:      targetedNodeLabel,
					Operator: corev1.NodeSelectorOpExists,
				})
			Expect(client.Create(ctx, workload)).To(Succeed(), "Failed to create workload")

			By("Waiting for the MachineSet to scale up")
			Eventually(func() bool {
				ms, err := framework.GetMachineSet(ctx, client, machineSet.GetName())
				Expect(err).ToNot(HaveOccurred(), "Failed to get MachineSet %s", machineSet.GetName())

				By(fmt.Sprintf("Waiting for machineSet replicas to scale out. Current replicas are %v, expected %v.",
					*ms.Spec.Replicas, expectedReplicas))

				return *ms.Spec.Replicas == expectedReplicas
			}, framework.WaitLong, pollingInterval).Should(BeTrue(),
				"MachineSet %s failed to scale out to %d replicas", machineSet.GetName(), expectedReplicas)

			By("Checking ProvisioningRequest status")
			prStatus := &unstructured.Unstructured{}
			prStatus.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   framework.ProvisioningRequestGroup,
				Version: framework.ProvisioningRequestVersion,
				Kind:    framework.ProvisioningRequestKind,
			})
			if err := client.Get(ctx, runtimeclient.ObjectKey{
				Name:      prName,
				Namespace: framework.MachineAPINamespace,
			}, prStatus); err == nil {
				conditions, found, _ := unstructured.NestedSlice(prStatus.Object, "status", "conditions")
				if found && len(conditions) > 0 {
					for _, c := range conditions {
						if cond, ok := c.(map[string]interface{}); ok {
							klog.Infof("ProvisioningRequest %s condition: type=%v status=%v",
								prName, cond["type"], cond["status"])
						}
					}
				} else {
					klog.Warningf("ProvisioningRequest %s has no status conditions yet", prName)
				}
			}

			By("Waiting for Machines in the MachineSet to become Running")
			framework.WaitForMachineSet(ctx, client, machineSet.GetName())

			klog.Infof("Scale-up verified: MachineSet %s scaled to %d replicas",
				machineSet.GetName(), expectedReplicas)
		})
	})
})
