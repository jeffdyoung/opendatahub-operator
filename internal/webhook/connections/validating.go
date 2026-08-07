//go:build !nowebhook

package connections

import (
	"context"
	"fmt"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	webhookutils "github.com/opendatahub-io/opendatahub-operator/v2/pkg/webhook"
)

// CEL matchConditions on the webhook configuration pre-filter to only
// admit connection secrets (those with connection-type annotations).
// Non-connection secrets never reach this handler.

// NOTE: matchConditions (CEL pre-filter for connection secrets) is added via kustomize patch
// in config/webhook/patches/connection-secret-match-conditions.yaml because controller-gen
// does not support the matchConditions marker field.
//+kubebuilder:webhook:path=/validate-connection-secret-deletion,mutating=false,failurePolicy=ignore,sideEffects=None,groups="",resources=secrets,verbs=delete,versions=v1,name=connection-secret-deletion-validator.opendatahub.io,admissionReviewVersions=v1
//nolint:lll

// DeletionValidator implements a validating admission webhook that prevents deletion
// of connection secrets still referenced by workloads (Notebooks, InferenceServices,
// LLMInferenceServices). Fix for RHOAIENG-50566.
type DeletionValidator struct {
	Client client.Reader
	Name   string
}

var _ admission.Handler = &DeletionValidator{}

func (v *DeletionValidator) SetupWithManager(mgr ctrl.Manager) error {
	hookServer := mgr.GetWebhookServer()
	hookServer.Register("/validate-connection-secret-deletion", &webhook.Admission{
		Handler:        v,
		LogConstructor: webhookutils.NewWebhookLogConstructor(v.Name),
	})
	return nil
}

func (v *DeletionValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	log := logf.FromContext(ctx)

	if req.Operation != admissionv1.Delete {
		return admission.Allowed(fmt.Sprintf("Operation %s on Secret allowed", req.Operation))
	}

	refs, err := webhookutils.FindWorkloadsReferencingSecret(ctx, v.Client, req.Name, req.Namespace)
	if err != nil {
		log.Error(err, "Failed to check for referencing workloads",
			"secret", req.Name, "namespace", req.Namespace)
		return admission.Errored(http.StatusInternalServerError, err)
	}

	if len(refs) > 0 {
		msg := fmt.Sprintf(
			"Cannot delete connection secret '%s' in namespace '%s': still referenced by %s. "+
				"Remove the connection annotation from these workloads or delete them first.",
			req.Name, req.Namespace, webhookutils.FormatReferencingWorkloads(refs),
		)
		log.Info("Denied connection secret deletion", "secret", req.Name,
			"namespace", req.Namespace, "referencingWorkloads", len(refs))
		return admission.Denied(msg)
	}

	return admission.Allowed("No active workloads reference this connection secret")
}
