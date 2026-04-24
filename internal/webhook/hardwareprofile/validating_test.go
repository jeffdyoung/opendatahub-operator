//go:build !nowebhook

package hardwareprofile_test

import (
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/opendatahub-io/opendatahub-operator/v2/internal/webhook/envtestutil"
	hardwareprofilewebhook "github.com/opendatahub-io/opendatahub-operator/v2/internal/webhook/hardwareprofile"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster/gvk"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/fakeclient"
	testscheme "github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/scheme"
	webhookutils "github.com/opendatahub-io/opendatahub-operator/v2/pkg/webhook"

	. "github.com/onsi/gomega"
)

const deletionTestHWPName = "test-hwp"

func hwpAdmissionRequest(t *testing.T, op admissionv1.Operation) admission.Request {
	t.Helper()
	hwp := envtestutil.NewHardwareProfile(deletionTestHWPName, hwpNamespace)
	return envtestutil.NewAdmissionRequest(
		t,
		op,
		hwp,
		gvk.HardwareProfile,
		metav1.GroupVersionResource{
			Group:    gvk.HardwareProfile.Group,
			Version:  gvk.HardwareProfile.Version,
			Resource: "hardwareprofiles",
		},
	)
}

func workloadScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s, err := testscheme.New()
	if err != nil {
		t.Fatalf("failed to create scheme: %v", err)
	}
	s.AddKnownTypeWithName(gvk.Notebook, &unstructured.Unstructured{})
	s.AddKnownTypeWithName(gvk.Notebook.GroupVersion().WithKind("NotebookList"), &unstructured.UnstructuredList{})
	s.AddKnownTypeWithName(gvk.InferenceServices, &unstructured.Unstructured{})
	s.AddKnownTypeWithName(gvk.InferenceServices.GroupVersion().WithKind("InferenceServiceList"), &unstructured.UnstructuredList{})
	s.AddKnownTypeWithName(gvk.LLMInferenceServiceV1Alpha1, &unstructured.Unstructured{})
	s.AddKnownTypeWithName(gvk.LLMInferenceServiceV1Alpha1.GroupVersion().WithKind("LLMInferenceServiceList"), &unstructured.UnstructuredList{})
	s.AddKnownTypeWithName(gvk.LLMInferenceServiceV1Alpha2, &unstructured.Unstructured{})
	s.AddKnownTypeWithName(gvk.LLMInferenceServiceV1Alpha2.GroupVersion().WithKind("LLMInferenceServiceList"), &unstructured.UnstructuredList{})
	return s
}

func newWorkload(kind, apiVersion, name, namespace string, anns map[string]string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetKind(kind)
	obj.SetAPIVersion(apiVersion)
	obj.SetName(name)
	obj.SetNamespace(namespace)
	obj.SetAnnotations(anns)
	return obj
}

func newTerminatingWorkload(kind, apiVersion, name, namespace string, anns map[string]string) *unstructured.Unstructured {
	obj := newWorkload(kind, apiVersion, name, namespace, anns)
	now := metav1.Now()
	obj.SetDeletionTimestamp(&now)
	obj.SetFinalizers([]string{"test-finalizer"})
	return obj
}

func TestHardwareProfileDeletionValidator(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		existingObjs []client.Object
		req          admission.Request
		allowed      bool
		msgContains  string
	}{
		{
			name:         "Allows deletion when no workloads reference the HWP",
			existingObjs: nil,
			req:          hwpAdmissionRequest(t, admissionv1.Delete),
			allowed:      true,
		},
		{
			name: "Denies deletion when a Notebook references the HWP in the same namespace",
			existingObjs: []client.Object{
				newWorkload("Notebook", "kubeflow.org/v1", "my-notebook", hwpNamespace, map[string]string{
					webhookutils.HardwareProfileNameAnnotation: deletionTestHWPName,
				}),
			},
			req:         hwpAdmissionRequest(t, admissionv1.Delete),
			allowed:     false,
			msgContains: "my-notebook",
		},
		{
			name: "Denies deletion when a Notebook references the HWP cross-namespace",
			existingObjs: []client.Object{
				newWorkload("Notebook", "kubeflow.org/v1", "cross-ns-notebook", "other-ns", map[string]string{
					webhookutils.HardwareProfileNameAnnotation:      deletionTestHWPName,
					webhookutils.HardwareProfileNamespaceAnnotation: hwpNamespace,
				}),
			},
			req:         hwpAdmissionRequest(t, admissionv1.Delete),
			allowed:     false,
			msgContains: "cross-ns-notebook",
		},
		{
			name: "Allows deletion when referencing Notebook is terminating",
			existingObjs: []client.Object{
				newTerminatingWorkload("Notebook", "kubeflow.org/v1", "terminating-notebook", hwpNamespace, map[string]string{
					webhookutils.HardwareProfileNameAnnotation: deletionTestHWPName,
				}),
			},
			req:     hwpAdmissionRequest(t, admissionv1.Delete),
			allowed: true,
		},
		{
			name: "Denies deletion when an InferenceService references the HWP",
			existingObjs: []client.Object{
				newWorkload("InferenceService", "serving.kserve.io/v1beta1", "my-isvc", hwpNamespace, map[string]string{
					webhookutils.HardwareProfileNameAnnotation: deletionTestHWPName,
				}),
			},
			req:         hwpAdmissionRequest(t, admissionv1.Delete),
			allowed:     false,
			msgContains: "my-isvc",
		},
		{
			name: "Allows deletion when workload references a different HWP",
			existingObjs: []client.Object{
				newWorkload("Notebook", "kubeflow.org/v1", "other-notebook", hwpNamespace, map[string]string{
					webhookutils.HardwareProfileNameAnnotation: "other-hwp",
				}),
			},
			req:     hwpAdmissionRequest(t, admissionv1.Delete),
			allowed: true,
		},
		{
			name: "Allows deletion when workload references same name but different namespace",
			existingObjs: []client.Object{
				newWorkload("Notebook", "kubeflow.org/v1", "other-ns-notebook", "other-ns", map[string]string{
					webhookutils.HardwareProfileNameAnnotation:      deletionTestHWPName,
					webhookutils.HardwareProfileNamespaceAnnotation: "different-ns",
				}),
			},
			req:     hwpAdmissionRequest(t, admissionv1.Delete),
			allowed: true,
		},
		{
			name:         "Allows non-DELETE operations",
			existingObjs: nil,
			req:          hwpAdmissionRequest(t, admissionv1.Create),
			allowed:      true,
		},
		{
			name: "Denies deletion listing multiple referencing workloads",
			existingObjs: []client.Object{
				newWorkload("Notebook", "kubeflow.org/v1", "notebook-1", hwpNamespace, map[string]string{
					webhookutils.HardwareProfileNameAnnotation: deletionTestHWPName,
				}),
				newWorkload("Notebook", "kubeflow.org/v1", "notebook-2", hwpNamespace, map[string]string{
					webhookutils.HardwareProfileNameAnnotation: deletionTestHWPName,
				}),
			},
			req:         hwpAdmissionRequest(t, admissionv1.Delete),
			allowed:     false,
			msgContains: "notebook-1",
		},
	}

	s := workloadScheme(t)

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			ctx := t.Context()

			cli, err := fakeclient.New(fakeclient.WithScheme(s), fakeclient.WithObjects(tc.existingObjs...))
			g.Expect(err).ShouldNot(HaveOccurred())

			validator := &hardwareprofilewebhook.DeletionValidator{
				Client: cli,
				Name:   "test-hwp-deletion-validator",
			}

			resp := validator.Handle(ctx, tc.req)
			g.Expect(resp.Allowed).To(Equal(tc.allowed))
			if !tc.allowed {
				g.Expect(resp.Result.Message).ToNot(BeEmpty(), "Expected error message when request is denied")
				if tc.msgContains != "" {
					g.Expect(resp.Result.Message).To(ContainSubstring(tc.msgContains))
				}
			}
		})
	}
}

func TestFormatReferencingWorkloads(t *testing.T) {
	t.Parallel()

	t.Run("empty list returns empty string", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)
		g.Expect(webhookutils.FormatReferencingWorkloads(nil)).To(Equal(""))
	})

	t.Run("single reference", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)
		refs := []webhookutils.WorkloadReference{
			{Kind: "Notebook", Name: "nb-1", Namespace: "ns-1"},
		}
		g.Expect(webhookutils.FormatReferencingWorkloads(refs)).To(Equal("Notebook 'ns-1/nb-1'"))
	})

	t.Run("multiple references", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)
		refs := []webhookutils.WorkloadReference{
			{Kind: "Notebook", Name: "nb-1", Namespace: "ns-1"},
			{Kind: "InferenceService", Name: "isvc-1", Namespace: "ns-2"},
		}
		result := webhookutils.FormatReferencingWorkloads(refs)
		g.Expect(result).To(ContainSubstring("Notebook 'ns-1/nb-1'"))
		g.Expect(result).To(ContainSubstring("InferenceService 'ns-2/isvc-1'"))
	})
}
