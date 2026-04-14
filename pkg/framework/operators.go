package framework

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// Operator namespaces
const (
	VPANamespace      = "openshift-vertical-pod-autoscaler"
	HPANamespace      = "openshift-machine-api"
	CRONamespace      = "clusterresourceoverride-operator"
	CMANamespace      = "openshift-keda"
	AutoNodeNamespace = "openshift-machine-api"
)

// OperatorInfo contains information about an operator
type OperatorInfo struct {
	Name      string
	Namespace string
	Labels    map[string]string
}

var Operators = map[string]OperatorInfo{
	"vpa": {
		Name:      "vertical-pod-autoscaler-operator",
		Namespace: VPANamespace,
		Labels:    map[string]string{"k8s-app": "vertical-pod-autoscaler-operator"},
	},
	"hpa": {
		Name:      "horizontal-pod-autoscaler",
		Namespace: HPANamespace,
		Labels:    map[string]string{"k8s-app": "cluster-autoscaler-operator"},
	},
	"cro": {
		Name:      "clusterresourceoverride-operator",
		Namespace: CRONamespace,
		Labels:    map[string]string{"app": "clusterresourceoverride-operator"},
	},
	"cma": {
		Name:      "custom-metrics-autoscaler-operator",
		Namespace: CMANamespace,
		Labels:    map[string]string{"name": "custom-metrics-autoscaler-operator"},
	},
	"autonode": {
		Name:      "machine-api-operator",
		Namespace: AutoNodeNamespace,
		Labels:    map[string]string{"app": "machine-api-operator"},
	},
}

// IsOperatorInstalled checks if an operator is installed
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

// WaitForOperatorReady waits for an operator to be ready
func (f *Framework) WaitForOperatorReady(ctx context.Context, operatorKey string, timeout time.Duration) error {
	op, ok := Operators[operatorKey]
	if !ok {
		return fmt.Errorf("unknown operator: %s", operatorKey)
	}

	return f.WaitForPodsWithLabel(ctx, op.Namespace, op.Labels, 1, timeout)
}

// GetOperatorPods returns all pods for an operator
func (f *Framework) GetOperatorPods(ctx context.Context, operatorKey string) (*corev1.PodList, error) {
	op, ok := Operators[operatorKey]
	if !ok {
		return nil, fmt.Errorf("unknown operator: %s", operatorKey)
	}

	return f.ListPods(ctx, op.Namespace, op.Labels)
}

// CheckOperatorHealth performs a health check on an operator
func (f *Framework) CheckOperatorHealth(ctx context.Context, operatorKey string) error {
	op, ok := Operators[operatorKey]
	if !ok {
		return fmt.Errorf("unknown operator: %s", operatorKey)
	}

	pods, err := f.ListPods(ctx, op.Namespace, op.Labels)
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
