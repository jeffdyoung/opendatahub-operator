//go:build !nowebhook

package connections_test

import (
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	connectionswebhook "github.com/opendatahub-io/opendatahub-operator/v2/internal/webhook/connections"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/webhook/envtestutil"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster/gvk"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/metadata/annotations"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/fakeclient"
	testscheme "github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/scheme"

	. "github.com/onsi/gomega"
)

const (
	secretName      = "test-secret"
	secretNamespace = "test-ns"
)

func secretAdmissionRequest(t *testing.T, op admissionv1.Operation) admission.Request {
	t.Helper()
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: secretNamespace,
		},
	}
	return envtestutil.NewAdmissionRequest(
		t,
		op,
		secret,
		gvk.Secret,
		metav1.GroupVersionResource{
			Group:    "",
			Version:  "v1",
			Resource: "secrets",
		},
	)
}

func connectionWorkloadScheme(t *testing.T) *runtime.Scheme {
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

func newConnectionWorkload(kind, apiVersion, name, namespace string, anns map[string]string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetKind(kind)
	obj.SetAPIVersion(apiVersion)
	obj.SetName(name)
	obj.SetNamespace(namespace)
	obj.SetAnnotations(anns)
	return obj
}

func newTerminatingConnectionWorkload(kind, apiVersion, name, namespace string, anns map[string]string) *unstructured.Unstructured {
	obj := newConnectionWorkload(kind, apiVersion, name, namespace, anns)
	now := metav1.Now()
	obj.SetDeletionTimestamp(&now)
	obj.SetFinalizers([]string{"test-finalizer"})
	return obj
}

func TestConnectionSecretDeletionValidator(t *testing.T) {
	t.Parallel()

	connAnnotation := secretNamespace + "/" + secretName

	cases := []struct {
		name         string
		existingObjs []client.Object
		req          admission.Request
		allowed      bool
		msgContains  string
	}{
		{
			name:         "Allows deletion when no workloads reference the secret",
			existingObjs: nil,
			req:          secretAdmissionRequest(t, admissionv1.Delete),
			allowed:      true,
		},
		{
			name: "Denies deletion when a Notebook references the secret",
			existingObjs: []client.Object{
				newConnectionWorkload("Notebook", "kubeflow.org/v1", "my-notebook", secretNamespace, map[string]string{
					annotations.Connection: connAnnotation,
				}),
			},
			req:         secretAdmissionRequest(t, admissionv1.Delete),
			allowed:     false,
			msgContains: "my-notebook",
		},
		{
			name: "Denies deletion when an InferenceService references the secret",
			existingObjs: []client.Object{
				newConnectionWorkload("InferenceService", "serving.kserve.io/v1beta1", "my-isvc", secretNamespace, map[string]string{
					annotations.Connection: connAnnotation,
				}),
			},
			req:         secretAdmissionRequest(t, admissionv1.Delete),
			allowed:     false,
			msgContains: "my-isvc",
		},
		{
			name: "Allows deletion when referencing workload is terminating",
			existingObjs: []client.Object{
				newTerminatingConnectionWorkload("Notebook", "kubeflow.org/v1", "terminating-notebook", secretNamespace, map[string]string{
					annotations.Connection: connAnnotation,
				}),
			},
			req:     secretAdmissionRequest(t, admissionv1.Delete),
			allowed: true,
		},
		{
			name: "Allows deletion when workload references a different secret",
			existingObjs: []client.Object{
				newConnectionWorkload("Notebook", "kubeflow.org/v1", "other-notebook", secretNamespace, map[string]string{
					annotations.Connection: secretNamespace + "/other-secret",
				}),
			},
			req:     secretAdmissionRequest(t, admissionv1.Delete),
			allowed: true,
		},
		{
			name: "Denies deletion when secret is one of multiple connections",
			existingObjs: []client.Object{
				newConnectionWorkload("Notebook", "kubeflow.org/v1", "multi-conn-nb", secretNamespace, map[string]string{
					annotations.Connection: secretNamespace + "/other-secret," + connAnnotation,
				}),
			},
			req:         secretAdmissionRequest(t, admissionv1.Delete),
			allowed:     false,
			msgContains: "multi-conn-nb",
		},
		{
			name: "Allows deletion when workload is in a different namespace",
			existingObjs: []client.Object{
				newConnectionWorkload("Notebook", "kubeflow.org/v1", "other-ns-notebook", "other-ns", map[string]string{
					annotations.Connection: "other-ns/" + secretName,
				}),
			},
			req:     secretAdmissionRequest(t, admissionv1.Delete),
			allowed: true,
		},
		{
			name:         "Allows non-DELETE operations",
			existingObjs: nil,
			req:          secretAdmissionRequest(t, admissionv1.Create),
			allowed:      true,
		},
		{
			name: "Denies deletion listing multiple referencing workloads",
			existingObjs: []client.Object{
				newConnectionWorkload("Notebook", "kubeflow.org/v1", "notebook-1", secretNamespace, map[string]string{
					annotations.Connection: connAnnotation,
				}),
				newConnectionWorkload("InferenceService", "serving.kserve.io/v1beta1", "isvc-1", secretNamespace, map[string]string{
					annotations.Connection: connAnnotation,
				}),
			},
			req:         secretAdmissionRequest(t, admissionv1.Delete),
			allowed:     false,
			msgContains: "notebook-1",
		},
	}

	s := connectionWorkloadScheme(t)

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			ctx := t.Context()

			cli, err := fakeclient.New(fakeclient.WithScheme(s), fakeclient.WithObjects(tc.existingObjs...))
			g.Expect(err).ShouldNot(HaveOccurred())

			validator := &connectionswebhook.DeletionValidator{
				Client: cli,
				Name:   "test-connection-secret-deletion-validator",
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
