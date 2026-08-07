//go:build !nowebhook

package connections

import (
	ctrl "sigs.k8s.io/controller-runtime"
)

// RegisterWebhooks registers the connection secret deletion validation webhook.
func RegisterWebhooks(mgr ctrl.Manager) error {
	if err := (&DeletionValidator{
		Client: mgr.GetAPIReader(),
		Name:   "connection-secret-deletion-validator",
	}).SetupWithManager(mgr); err != nil {
		return err
	}

	return nil
}
