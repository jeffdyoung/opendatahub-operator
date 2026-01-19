package sparkoperator

import (
	"github.com/opendatahub-io/opendatahub-operator/v2/api/common"
	componentApi "github.com/opendatahub-io/opendatahub-operator/v2/api/components/v1alpha1"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/status"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/types"
	odhdeploy "github.com/opendatahub-io/opendatahub-operator/v2/pkg/deploy"
)

const (
	ComponentName = componentApi.SparkOperatorComponentName

	ReadyConditionType = componentApi.SparkOperatorKind + status.ReadySuffix
)

var (
	ManifestsSourcePath = map[common.Platform]string{
		cluster.SelfManagedRhoai: "overlays/rhoai",
		cluster.ManagedRhoai:     "overlays/rhoai",
		cluster.OpenDataHub:      "overlays/odh",
	}

	// imageParamMap maps kustomize parameter names to environment variable names for image overrides.
	imageParamMap = map[string]string{
		"odh-spark-operator-controller-image": "RELATED_IMAGE_ODH_SPARK_OPERATOR_IMAGE",
	}

	// conditionTypes defines the list of conditions that contribute to the component's readiness.
	conditionTypes = []string{
		status.ConditionDeploymentsAvailable,
	}
)

// manifestPath returns the manifest location for the SparkOperator component.
func manifestPath(p common.Platform) types.ManifestInfo {
	return types.ManifestInfo{
		Path:       odhdeploy.DefaultManifestPath,
		ContextDir: ComponentName,
		SourcePath: ManifestsSourcePath[p],
	}
}
