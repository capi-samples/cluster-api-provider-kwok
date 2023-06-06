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

	sharedv1 "github.com/capi-samples/cluster-api-provider-kwok/api/shared/v1alpha1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// KwokConfigSpec defines the desired state of KwokConfig
type KwokConfigSpec struct {
	// SimulationConfig holds the configuration options for changing the behavior of the simulation.
	//+optional
	SimulationConfig *sharedv1.SimulationConfig `json:"simulationConfig,omitempty"`
}

// KwokConfigStatus defines the observed state of KwokConfig
type KwokConfigStatus struct {
	// LastReconcileTime is the duration of the last reconcile loop.
	//+optional
	LastReconcileDuration time.Duration `json:"lastreconcileduration,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// KwokConfig is the Schema for the kwokconfigs API
type KwokConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KwokConfigSpec   `json:"spec,omitempty"`
	Status KwokConfigStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// KwokConfigList contains a list of KwokConfig
type KwokConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KwokConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KwokConfig{}, &KwokConfigList{})
}
