//go:build !nowebhook

package hardwareprofile

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

//+kubebuilder:webhook:path=/validate-hardwareprofile-deletion,mutating=false,failurePolicy=ignore,sideEffects=None,groups=infrastructure.opendatahub.io,resources=hardwareprofiles,verbs=delete,versions=v1,name=hardwareprofile-deletion-validator.opendatahub.io,admissionReviewVersions=v1
//nolint:lll

// DeletionValidator implements a validating admission webhook that prevents deletion
// of HardwareProfiles still referenced by workloads (Notebooks, InferenceServices,
// LLMInferenceServices). Fix for RHOAIENG-50566.
type DeletionValidator struct {
	Client client.Reader
	Name   string
}

var _ admission.Handler = &DeletionValidator{}

func (v *DeletionValidator) SetupWithManager(mgr ctrl.Manager) error {
	hookServer := mgr.GetWebhookServer()
	hookServer.Register("/validate-hardwareprofile-deletion", &webhook.Admission{
		Handler:        v,
		LogConstructor: webhookutils.NewWebhookLogConstructor(v.Name),
	})
	return nil
}

func (v *DeletionValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	log := logf.FromContext(ctx)

	if req.Operation != admissionv1.Delete {
		return admission.Allowed(fmt.Sprintf("Operation %s on %s allowed", req.Operation, req.Kind.Kind))
	}

	refs, err := webhookutils.FindWorkloadsReferencingHWP(ctx, v.Client, req.Name, req.Namespace)
	if err != nil {
		log.Error(err, "Failed to check for referencing workloads",
			"hardwareProfile", req.Name, "namespace", req.Namespace)
		// failurePolicy: Ignore means Kubernetes will allow the request if we error
		return admission.Errored(http.StatusInternalServerError, err)
	}

	if len(refs) > 0 {
		msg := fmt.Sprintf(
			"Cannot delete HardwareProfile '%s' in namespace '%s': still referenced by %s. "+
				"Remove the hardware profile annotation from these workloads or delete them first.",
			req.Name, req.Namespace, webhookutils.FormatReferencingWorkloads(refs),
		)
		log.Info("Denied HardwareProfile deletion", "hardwareProfile", req.Name,
			"namespace", req.Namespace, "referencingWorkloads", len(refs))
		return admission.Denied(msg)
	}

	return admission.Allowed("No active workloads reference this HardwareProfile")
}
