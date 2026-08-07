//go:build !nowebhook

package webhookutils

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster/gvk"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/metadata/annotations"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/resources"
)

const (
	HardwareProfileNameAnnotation      = "opendatahub.io/hardware-profile-name"
	HardwareProfileNamespaceAnnotation = "opendatahub.io/hardware-profile-namespace"
)

// WorkloadReference identifies a workload that references a resource.
type WorkloadReference struct {
	Kind      string
	Name      string
	Namespace string
}

func (r WorkloadReference) String() string {
	return fmt.Sprintf("%s '%s/%s'", r.Kind, r.Namespace, r.Name)
}

func workloadGVKs() []schema.GroupVersionKind {
	return []schema.GroupVersionKind{
		gvk.Notebook,
		gvk.InferenceServices,
		gvk.LLMInferenceServiceV1Alpha1,
		gvk.LLMInferenceServiceV1Alpha2,
	}
}

// FindWorkloadsReferencingHWP lists all active (non-terminating) workloads that reference
// the given HardwareProfile by name and namespace. It searches across all namespaces
// because cross-namespace HWP references are supported.
func FindWorkloadsReferencingHWP(ctx context.Context, cli client.Reader, hwpName, hwpNamespace string) ([]WorkloadReference, error) {
	log := logf.FromContext(ctx)
	var refs []WorkloadReference

	for _, workloadGVK := range workloadGVKs() {
		list := &metav1.PartialObjectMetadataList{}
		list.SetGroupVersionKind(workloadGVK)

		if err := cli.List(ctx, list); err != nil {
			if meta.IsNoMatchError(err) {
				log.V(1).Info("CRD not installed, skipping workload type", "gvk", workloadGVK)
				continue
			}
			return nil, fmt.Errorf("failed to list %s: %w", workloadGVK.Kind, err)
		}

		for i := range list.Items {
			item := &list.Items[i]

			if !item.GetDeletionTimestamp().IsZero() {
				continue
			}

			profileName := resources.GetAnnotation(item, HardwareProfileNameAnnotation)
			if profileName != hwpName {
				continue
			}

			profileNamespace := resources.GetAnnotation(item, HardwareProfileNamespaceAnnotation)
			if profileNamespace == "" {
				profileNamespace = item.GetNamespace()
			}
			if profileNamespace != hwpNamespace {
				continue
			}

			refs = append(refs, WorkloadReference{
				Kind:      workloadGVK.Kind,
				Name:      item.GetName(),
				Namespace: item.GetNamespace(),
			})
		}
	}

	return refs, nil
}

// FindWorkloadsReferencingSecret lists all active (non-terminating) workloads in the given
// namespace that reference the specified connection secret. Connection secrets are
// namespace-scoped so only workloads in the same namespace are checked.
func FindWorkloadsReferencingSecret(ctx context.Context, cli client.Reader, secretName, secretNamespace string) ([]WorkloadReference, error) {
	log := logf.FromContext(ctx)
	var refs []WorkloadReference

	for _, workloadGVK := range workloadGVKs() {
		list := &metav1.PartialObjectMetadataList{}
		list.SetGroupVersionKind(workloadGVK)

		if err := cli.List(ctx, list, client.InNamespace(secretNamespace)); err != nil {
			if meta.IsNoMatchError(err) {
				log.V(1).Info("CRD not installed, skipping workload type", "gvk", workloadGVK)
				continue
			}
			return nil, fmt.Errorf("failed to list %s in namespace %s: %w", workloadGVK.Kind, secretNamespace, err)
		}

		for i := range list.Items {
			item := &list.Items[i]

			if !item.GetDeletionTimestamp().IsZero() {
				continue
			}

			connAnnotation := resources.GetAnnotation(item, annotations.Connection)
			if connAnnotation == "" {
				continue
			}

			if referencesSecret(connAnnotation, secretName, secretNamespace) {
				refs = append(refs, WorkloadReference{
					Kind:      workloadGVK.Kind,
					Name:      item.GetName(),
					Namespace: item.GetNamespace(),
				})
			}
		}
	}

	return refs, nil
}

// referencesSecret checks if a connection annotation value references the given secret.
// The annotation format is "namespace/secretName" with comma-separated entries.
func referencesSecret(annotationValue, secretName, secretNamespace string) bool {
	target := secretNamespace + "/" + secretName
	for _, part := range strings.Split(annotationValue, ",") {
		if strings.TrimSpace(part) == target {
			return true
		}
	}
	return false
}

// FormatReferencingWorkloads builds a human-readable list of workload references
// for use in admission denial messages.
func FormatReferencingWorkloads(refs []WorkloadReference) string {
	if len(refs) == 0 {
		return ""
	}
	parts := make([]string, len(refs))
	for i, ref := range refs {
		parts[i] = ref.String()
	}
	return strings.Join(parts, ", ")
}
