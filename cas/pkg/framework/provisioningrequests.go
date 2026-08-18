package framework

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	ProvisioningRequestGroup   = "autoscaling.x-k8s.io"
	ProvisioningRequestVersion = "v1"
	ProvisioningRequestKind    = "ProvisioningRequest"
)

// ProvisioningRequestConfig holds parameters for constructing a ProvisioningRequest.
type ProvisioningRequestConfig struct {
	Name              string
	Namespace         string
	ProvisioningClass string
	PodTemplateName   string
	PodCount          int64
}

func NewProvisioningRequest(cfg ProvisioningRequestConfig) *unstructured.Unstructured {
	if cfg.PodCount < 1 {
		cfg.PodCount = 1
	}
	if cfg.PodTemplateName == "" {
		cfg.PodTemplateName = cfg.Name + "-pod-template"
	}

	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": ProvisioningRequestGroup + "/" + ProvisioningRequestVersion,
			"kind":       ProvisioningRequestKind,
			"metadata": map[string]interface{}{
				"name":      cfg.Name,
				"namespace": cfg.Namespace,
			},
			"spec": map[string]interface{}{
				"provisioningClassName": cfg.ProvisioningClass,
				"podSets": []interface{}{
					map[string]interface{}{
						"count": cfg.PodCount,
						"podTemplateRef": map[string]interface{}{
							"name": cfg.PodTemplateName,
						},
					},
				},
			},
		},
	}
}
