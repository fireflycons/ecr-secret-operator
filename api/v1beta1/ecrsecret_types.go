/*
Copyright 2023.

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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ECRSecretSpec defines the desired state of ECRSecret
type ECRSecretSpec struct {
	// +kubebuilder:validation:Pattern=`^\d{12}\.dkr.ecr.(ap|ca|eu|sa|us(-gov)?)-(east|northeast|southeast|north|south|southeast|central|west)-\d\.amazonaws\.com$`
	Registry string `json:"registry,omitempty"`
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`
	SecretName string `json:"secretName,omitempty"`
}

// ECRSecretStatus defines the observed state of ECRSecret
type ECRSecretStatus struct {
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ECRSecret is the Schema for the ecrsecrets API
type ECRSecret struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ECRSecretSpec   `json:"spec,omitempty"`
	Status ECRSecretStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ECRSecretList contains a list of ECRSecret
type ECRSecretList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ECRSecret `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ECRSecret{}, &ECRSecretList{})
}
