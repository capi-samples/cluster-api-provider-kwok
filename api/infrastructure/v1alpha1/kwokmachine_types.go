/*
Copyright 2023 The Kubernetes Authors..

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"time"

	sharedv1 "github.com/capi-samples/cluster-api-provider-kwok/api/shared/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/errors"
)

const (
	// KwokMachineFinalizer allows the controller to clean up resources on delete.
	KwokMachineFinalizer = "kwokmachine.infrastructure.cluster.x-k8s.io"
)

// KwokMachineSpec defines the desired state of KwokMachine
type KwokMachineSpec struct {
	// ProviderID is the unique identifier as specified by the cloud provider.
	ProviderID *string `json:"providerID,omitempty"`

	// SimulationConfig holds the configuration options for changing the behavior of the simulation.
	//+optional
	SimulationConfig *sharedv1.SimulationConfig `json:"simulationConfig,omitempty"`
}

// KwokMachineStatus defines the observed state of KwokMachine
type KwokMachineStatus struct {
	// Ready is true when the provider resource is ready.
	// +optional
	// +kubebuilder:default=false
	Ready bool `json:"ready"`

	// FailureReason will be set in the event that there is a terminal problem
	// reconciling the Machine and will contain a succinct value suitable
	// for machine interpretation.
	// +optional
	FailureReason *errors.MachineStatusError `json:"failureReason,omitempty"`

	// FailureMessage will be set in the event that there is a terminal problem
	// reconciling the Machine and will contain a more verbose string suitable
	// for logging and human consumption.
	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`

	// Conditions defines current service state of the MicrovmMachine.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`

	// LastReconcileTime is the duration of the last reconcile loop.
	//+optional
	LastReconcileDuration time.Duration `json:"lastreconcileduration,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// KwokMachine is the Schema for the kwokmachines API
type KwokMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KwokMachineSpec   `json:"spec,omitempty"`
	Status KwokMachineStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// KwokMachineList contains a list of KwokMachine
type KwokMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KwokMachine `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KwokMachine{}, &KwokMachineList{})
}
