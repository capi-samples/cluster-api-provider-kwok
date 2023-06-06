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
	// KwokClusterFinalizer allows the controller to clean up resources on delete.
	KwokClusterFinalizer = "kwokcluster.infrastructure.cluster.x-k8s.io"
)

// KwokClusterSpec defines the desired state of KwokCluster
type KwokClusterSpec struct {
	//BindAddress is the address to use in the kubeconfig.
	//+optional
	BindAddress string `json:"bindAddress,omitempty"`

	// Runtime is the kwok runtime to use.
	// +kubebuilder:default=docker
	Runtime string `json:"runtime,omitempty"`

	// WorkingDir is the directory to use for the kwok runtime. If using kind
	// you will need to mount this as an extra volume.
	// +kubebuilder:default=/kwok
	WorkingDir string `json:"workingDir,omitempty"`

	// ControlPlaneEndpoint represents the endpoint used to communicate with the control plane.
	// +optional
	ControlPlaneEndpoint clusterv1.APIEndpoint `json:"controlPlaneEndpoint"`

	// SimulationConfig holds the configuration options for changing the behavior of the simulation.
	//+optional
	SimulationConfig *sharedv1.SimulationConfig `json:"simulationConfig,omitempty"`
}

// KwokClusterStatus defines the observed state of KwokCluster
type KwokClusterStatus struct {
	// Ready indicates that the cluster is ready.
	// +optional
	// +kubebuilder:default=false
	Ready bool `json:"ready"`

	// Conditions defines current service state of the KwokCluster.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`

	// FailureDomains is a list of the failure domains that CAPI should spread the machines across.
	FailureDomains clusterv1.FailureDomains `json:"failureDomains,omitempty"`

	// LastReconcileTime is the duration of the last reconcile loop.
	//+optional
	LastReconcileDuration time.Duration `json:"lastreconcileduration,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// KwokCluster is the Schema for the kwokclusters API
type KwokCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KwokClusterSpec   `json:"spec,omitempty"`
	Status KwokClusterStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// KwokClusterList contains a list of KwokCluster
type KwokClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KwokCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KwokCluster{}, &KwokClusterList{})
}
