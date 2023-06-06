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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	sharedv1 "github.com/capi-samples/cluster-api-provider-kwok/api/shared/v1alpha1"
)

const (
	// KwokControlPlaneFinalizer allows the controller to clean up resources on delete.
	KwokControlPlaneFinalizer = "kwok.controleplane.cluster.x-k8s.io"
)

// KwokControlPlaneSpec defines the desired state of KwokControlPlane
type KwokControlPlaneSpec struct {
	// ControlPlaneEndpoint represents the endpoint used to communicate with the control plane.
	// +optional
	ControlPlaneEndpoint clusterv1.APIEndpoint `json:"controlPlaneEndpoint"`
	// InfrastructureRef is a required reference to a custom resource
	// offered by an infrastructure provider.
	//InfrastructureRef corev1.ObjectReference `json:"infrastructureRef"`

	// SimulationConfig holds the configuration options for changing the behavior of the simulation.
	//+optional
	SimulationConfig *sharedv1.SimulationConfig `json:"simulationConfig,omitempty"`
}

// KwokControlPlaneStatus defines the observed state of KwokControlPlane
type KwokControlPlaneStatus struct {
	// LastReconcileTime is the duration of the last reconcile loop.
	//+optional
	LastReconcileDuration time.Duration `json:"lastreconcileduration,omitempty"`
	// Initialized denotes whether or not the control plane has the
	// uploaded kubernetes config-map.
	// +optional
	Initialized bool `json:"initialized"`
	// Ready denotes that the KwokControlPlane API Server is ready to
	// receive requests and that the VPC infra is ready.
	// +kubebuilder:default=false
	Ready bool `json:"ready"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// KwokControlPlane is the Schema for the kwokcontrolplanes API
type KwokControlPlane struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KwokControlPlaneSpec   `json:"spec,omitempty"`
	Status KwokControlPlaneStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// KwokControlPlaneList contains a list of KwokControlPlane
type KwokControlPlaneList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KwokControlPlane `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KwokControlPlane{}, &KwokControlPlaneList{})
}
