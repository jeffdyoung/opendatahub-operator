package v1alpha1

import (
	"github.com/opendatahub-io/opendatahub-operator/v2/api/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// component name
	SparkOperatorComponentName = "sparkoperator"

	// SparkOperatorInstanceName is the name of the component instance singleton
	// value should match what is set in the kubebuilder markers for XValidation defined below
	SparkOperatorInstanceName = "default-" + "sparkoperator"

	// kubernetes kind of the component
	SparkOperatorKind = "SparkOperator"
)

// Check that the component implements common.PlatformObject.
var _ common.PlatformObject = (*SparkOperator)(nil)

type SparkOperatorCommonSpec struct {
	// TODO:
	// new component spec shared with DSC api
	// ( refer/define here if applicable to the new component )
}

type SparkOperatorSpec struct {
	SparkOperatorCommonSpec `json:",inline"`

	// TODO:
	// new component spec exposed only to internal api
	// ( refer/define here if applicable to the new component )
}

// SparkOperatorCommonStatus defines the shared observed state of SparkOperator
type SparkOperatorCommonStatus struct {
	common.ComponentReleaseStatus `json:",inline"`
}

// SparkOperatorStatus defines the observed state of SparkOperator
type SparkOperatorStatus struct {
	common.Status             `json:",inline"`
	SparkOperatorCommonStatus `json:",inline"`
}

// default kubebuilder markers for the component
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:validation:XValidation:rule="self.metadata.name == 'default-sparkoperator'",message="SparkOperator name must be default-sparkoperator"
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`,description="Ready"
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].reason`,description="Reason"

// SparkOperator is the Schema for the SparkOperators API
type SparkOperator struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SparkOperatorSpec   `json:"spec,omitempty"`
	Status SparkOperatorStatus `json:"status,omitempty"`
}

// GetStatus retrieves the status of the SparkOperator component
func (c *SparkOperator) GetStatus() *common.Status {
	return &c.Status.Status
}

func (c *SparkOperator) GetConditions() []common.Condition {
	return c.Status.GetConditions()
}

func (c *SparkOperator) SetConditions(conditions []common.Condition) {
	c.Status.SetConditions(conditions)
}

func (c *SparkOperator) GetReleaseStatus() *[]common.ComponentRelease {
	return &c.Status.Releases
}

func (c *SparkOperator) SetReleaseStatus(releases []common.ComponentRelease) {
	c.Status.Releases = releases
}

// +kubebuilder:object:root=true

// SparkOperatorList contains a list of SparkOperator
type SparkOperatorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SparkOperator `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SparkOperator{}, &SparkOperatorList{})
}

type DSCSparkOperator struct {
	common.ManagementSpec   `json:",inline"`
	SparkOperatorCommonSpec `json:",inline"`
}

// DSCSparkOperatorStatus contains the observed state of the SparkOperator exposed in the DSC instance
type DSCSparkOperatorStatus struct {
	common.ManagementSpec      `json:",inline"`
	*SparkOperatorCommonStatus `json:",inline"`
}
