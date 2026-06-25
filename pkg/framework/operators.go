package framework

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type OperatorInstallOptions struct {
	Name                string
	Namespace           string
	Channel             string
	CatalogSource       string
	CatalogSourceNS     string
	StartingCSV         string
	InstallPlanApproval string
	AllNamespaces       bool // Use all-namespaces OperatorGroup
}

func (f *Framework) InstallOperator(ctx context.Context, opts OperatorInstallOptions) error {
	if _, err := f.CreateNamespace(ctx, opts.Namespace); err != nil {
		return fmt.Errorf("failed to create namespace: %w", err)
	}

	if err := f.ensureOperatorGroupWithMode(ctx, opts.Namespace, opts.AllNamespaces); err != nil {
		return fmt.Errorf("failed to create operator group: %w", err)
	}

	subscription := &unstructured.Unstructured{
		Object: map[string]interface{}{
			keyAPIVersion: "operators.coreos.com/v1alpha1",
			keyKind:       kindSubscription,
			keyMetadata:   map[string]interface{}{keyName: opts.Name, keyNamespace: opts.Namespace},
			keySpec: map[string]interface{}{
				"channel":             opts.Channel,
				keyName:               opts.Name,
				"source":              opts.CatalogSource,
				"sourceNamespace":     opts.CatalogSourceNS,
				"installPlanApproval": opts.InstallPlanApproval,
			},
		},
	}
	if opts.StartingCSV != "" {
		if err := unstructured.SetNestedField(subscription.Object, opts.StartingCSV, "spec", "startingCSV"); err != nil {
			return fmt.Errorf("failed to set startingCSV: %w", err)
		}
	}

	if err := f.Client.Create(ctx, subscription); err != nil {
		return fmt.Errorf("failed to create subscription: %w", err)
	}

	return nil
}

func (f *Framework) ensureOperatorGroupWithMode(ctx context.Context, namespace string, allNamespaces bool) error {
	ogList := &unstructured.UnstructuredList{}
	ogList.SetGroupVersionKind(schema.GroupVersionKind{Group: operatorsAPIGroup, Version: "v1", Kind: "OperatorGroupList"})

	if err := f.Client.List(ctx, ogList, client.InNamespace(namespace)); err != nil {
		return err
	}

	// Delete any existing OperatorGroups that don't match the desired mode
	for i := range ogList.Items {
		existing := &ogList.Items[i]
		targets, _, _ := unstructured.NestedStringSlice(existing.Object, "spec", "targetNamespaces")

		existingIsAllNS := len(targets) == 0
		if existingIsAllNS != allNamespaces {
			fmt.Printf("[Operator] Deleting stale OperatorGroup %s (allNamespaces=%v, want=%v)\n",
				existing.GetName(), existingIsAllNS, allNamespaces)

			if err := f.Client.Delete(ctx, existing); err != nil {
				return fmt.Errorf("failed to delete stale OperatorGroup: %w", err)
			}
		} else {
			return nil
		}
	}

	spec := map[string]interface{}{}
	if !allNamespaces {
		spec["targetNamespaces"] = []interface{}{namespace}
	}

	og := &unstructured.Unstructured{
		Object: map[string]interface{}{
			keyAPIVersion: "operators.coreos.com/v1",
			keyKind:       "OperatorGroup",
			keyMetadata:   map[string]interface{}{keyName: namespace + "-og", keyNamespace: namespace},
			keySpec:       spec,
		},
	}

	return f.Client.Create(ctx, og)
}

func (f *Framework) UninstallOperator(ctx context.Context, name, namespace string) error {
	subscription := &unstructured.Unstructured{}
	subscription.SetGroupVersionKind(schema.GroupVersionKind{Group: operatorsAPIGroup, Version: apiV1alpha1, Kind: kindSubscription})
	subscription.SetName(name)
	subscription.SetNamespace(namespace)

	if err := client.IgnoreNotFound(f.Client.Delete(ctx, subscription)); err != nil {
		return fmt.Errorf("failed to delete subscription: %w", err)
	}

	csvList := &unstructured.UnstructuredList{}
	csvList.SetGroupVersionKind(schema.GroupVersionKind{Group: operatorsAPIGroup, Version: apiV1alpha1, Kind: "ClusterServiceVersionList"})

	if err := f.Client.List(ctx, csvList, client.InNamespace(namespace)); err != nil {
		return fmt.Errorf("failed to list CSVs: %w", err)
	}

	for _, csv := range csvList.Items {
		if err := f.Client.Delete(ctx, &csv); err != nil {
			return fmt.Errorf("failed to delete CSV %s: %w", csv.GetName(), err)
		}
	}

	return nil
}

func (f *Framework) WaitForOperatorCSVReady(ctx context.Context, namespace string, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, 5*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		csvList := &unstructured.UnstructuredList{}
		csvList.SetGroupVersionKind(schema.GroupVersionKind{Group: operatorsAPIGroup, Version: apiV1alpha1, Kind: "ClusterServiceVersionList"})

		if err := f.Client.List(ctx, csvList, client.InNamespace(namespace)); err != nil || len(csvList.Items) == 0 {
			return false, nil //nolint:nilerr
		}

		for _, csv := range csvList.Items {
			phase, found, _ := unstructured.NestedString(csv.Object, "status", "phase")
			if !found || phase != "Succeeded" {
				return false, nil
			}
		}

		return true, nil
	})
}

func (f *Framework) GetSubscription(ctx context.Context, name, namespace string) (*unstructured.Unstructured, error) {
	subscription := &unstructured.Unstructured{}
	subscription.SetGroupVersionKind(schema.GroupVersionKind{Group: operatorsAPIGroup, Version: apiV1alpha1, Kind: kindSubscription})
	err := f.Client.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, subscription)

	return subscription, err
}

func (f *Framework) IsOperatorSubscribed(ctx context.Context, name, namespace string) (bool, error) {
	_, err := f.GetSubscription(ctx, name, namespace)
	if err != nil {
		if client.IgnoreNotFound(err) == nil {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

// Operator namespaces.
const (
	VPANamespace      = "openshift-vertical-pod-autoscaler"
	HPANamespace      = "openshift-machine-api" // backward comp
	CASNamespace      = "openshift-machine-api"
	CRONamespace      = "clusterresourceoverride-operator"
	CMANamespace      = "openshift-keda"
	AutoNodeNamespace = "openshift-machine-api"
)

// Common unstructured object map keys used across the framework package.
const (
	keyAPIVersion = "apiVersion"
	keyKind       = "kind"
	keyMetadata   = "metadata"
	keyName       = "name"
	keyNamespace  = "namespace"
	keySpec       = "spec"
	labelApp      = "app"
	labelK8sApp   = "k8s-app"
	apiV1alpha1   = "v1alpha1"

	operatorsAPIGroup      = "operators.coreos.com"
	kindSubscription       = "Subscription"
	catalogRedHatOperators = "redhat-operators"
	channelStable          = "stable"
	kedaOperator           = "keda-operator"
	kedaMetricsAPIServer   = "keda-metrics-apiserver"
)

// OperatorInfo contains information about an operator.
type OperatorInfo struct {
	Name          string
	Namespace     string // Namespace where subscription is created
	PodsNamespace string // Namespace where operator pods run (if different)
	Labels        map[string]string
	PackageName   string // OLM package name for installation
	CatalogSource string // Catalog source (e.g., "redhat-operators")
	Channel       string // Channel (e.g., "stable")
	AllNamespaces bool   // If true, OperatorGroup watches all namespaces
}

var Operators = map[string]OperatorInfo{
	"vpa": {
		Name:          "vertical-pod-autoscaler-operator",
		Namespace:     VPANamespace,
		Labels:        map[string]string{labelK8sApp: "vertical-pod-autoscaler-operator"},
		PackageName:   "vertical-pod-autoscaler",
		CatalogSource: catalogRedHatOperators,
		Channel:       channelStable,
	},
	"cas": {
		Name:          "cluster-autoscaler-operator",
		Namespace:     CASNamespace,
		Labels:        map[string]string{labelK8sApp: "cluster-autoscaler-operator"},
		PackageName:   "cluster-autoscaler",
		CatalogSource: catalogRedHatOperators,
		Channel:       channelStable,
	},
	"cro": {
		Name:          "clusterresourceoverride-operator",
		Namespace:     CRONamespace,
		Labels:        map[string]string{"clusterresourceoverride.operator": "true"},
		PackageName:   "clusterresourceoverride",
		CatalogSource: catalogRedHatOperators,
		Channel:       channelStable,
	},
	"cma": {
		Name:          "custom-metrics-autoscaler-operator",
		Namespace:     CMANamespace,
		PodsNamespace: "",
		Labels:        map[string]string{keyName: "custom-metrics-autoscaler-operator"},
		PackageName:   "openshift-custom-metrics-autoscaler-operator",
		CatalogSource: catalogRedHatOperators,
		Channel:       channelStable,
	},

	// KEDA sub-components installed by CMA operator
	kedaOperator: {
		Name:      kedaOperator,
		Namespace: CMANamespace,
		Labels:    map[string]string{labelApp: kedaOperator},
	},
	kedaMetricsAPIServer: {
		Name:      kedaMetricsAPIServer,
		Namespace: CMANamespace,
		Labels:    map[string]string{labelApp: kedaMetricsAPIServer},
	},
	"keda-admission": {
		Name:      "keda-admission-webhooks",
		Namespace: CMANamespace,
		Labels:    map[string]string{"app": "keda-admission-webhooks"},
	},
}

// IsOperatorInstalled checks if an operator is installed.
func (f *Framework) IsOperatorInstalled(ctx context.Context, operatorKey string) (bool, error) {
	op, ok := Operators[operatorKey]
	if !ok {
		return false, fmt.Errorf("unknown operator: %s", operatorKey)
	}

	pods, err := f.ListPods(ctx, op.Namespace, op.Labels)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}

		return false, err
	}

	return len(pods.Items) > 0, nil
}

// WaitForOperatorReady waits for an operator to be ready.
func (f *Framework) WaitForOperatorReady(ctx context.Context, operatorKey string, timeout time.Duration) error {
	op, ok := Operators[operatorKey]
	if !ok {
		return fmt.Errorf("unknown operator: %s", operatorKey)
	}

	ns := op.Namespace
	if op.PodsNamespace != "" {
		ns = op.PodsNamespace
	}

	return f.WaitForPodsWithLabel(ctx, ns, op.Labels, 1, timeout)
}

// GetOperatorPods returns all pods for an operator.
func (f *Framework) GetOperatorPods(ctx context.Context, operatorKey string) (*corev1.PodList, error) {
	op, ok := Operators[operatorKey]
	if !ok {
		return nil, fmt.Errorf("unknown operator: %s", operatorKey)
	}

	ns := op.Namespace
	if op.PodsNamespace != "" {
		ns = op.PodsNamespace
	}

	return f.ListPods(ctx, ns, op.Labels)
}

// CheckOperatorHealth performs a health check on an operator.
func (f *Framework) CheckOperatorHealth(ctx context.Context, operatorKey string) error {
	op, ok := Operators[operatorKey]
	if !ok {
		return fmt.Errorf("unknown operator: %s", operatorKey)
	}

	ns := op.Namespace
	if op.PodsNamespace != "" {
		ns = op.PodsNamespace
	}

	pods, err := f.ListPods(ctx, ns, op.Labels)
	if err != nil {
		return fmt.Errorf("failed to list operator pods: %w", err)
	}

	if len(pods.Items) == 0 {
		return fmt.Errorf("no pods found for operator %s", operatorKey)
	}

	for _, pod := range pods.Items {
		if !isPodReady(&pod) {
			return fmt.Errorf("pod %s is not ready", pod.Name)
		}
	}

	return nil
}

// InstallOperatorByKey installs an operator using predefined settings from Operators map.
func (f *Framework) InstallOperatorByKey(ctx context.Context, operatorKey string) error {
	op, ok := Operators[operatorKey]
	if !ok {
		return fmt.Errorf("unknown operator: %s", operatorKey)
	}

	if op.PackageName == "" {
		return fmt.Errorf("operator %s is built-in and cannot be installed via catalog", operatorKey)
	}

	opts := OperatorInstallOptions{
		Name:                op.PackageName,
		Namespace:           op.Namespace,
		Channel:             op.Channel,
		CatalogSource:       op.CatalogSource,
		CatalogSourceNS:     "openshift-marketplace",
		InstallPlanApproval: "Automatic",
		AllNamespaces:       op.AllNamespaces,
	}

	return f.InstallOperator(ctx, opts)
}

// GetOperatorPodsNamespace returns the namespace where operator pods run.
func GetOperatorPodsNamespace(operatorKey string) string {
	op, ok := Operators[operatorKey]
	if !ok {
		return ""
	}

	if op.PodsNamespace != "" {
		return op.PodsNamespace
	}

	return op.Namespace
}

// UninstallOperatorByKey uninstalls an operator using predefined settings.
func (f *Framework) UninstallOperatorByKey(ctx context.Context, operatorKey string) error {
	op, ok := Operators[operatorKey]
	if !ok {
		return fmt.Errorf("unknown operator: %s", operatorKey)
	}

	if op.PackageName == "" {
		return fmt.Errorf("operator %s is built-in and cannot be uninstalled", operatorKey)
	}

	return f.UninstallOperator(ctx, op.PackageName, op.Namespace)
}
