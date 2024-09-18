/*
Copyright 2023 The cert-manager Authors.

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
	"github.com/cert-manager/issuer-lib/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status"
// +kubebuilder:printcolumn:name="Reason",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].reason"
// +kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message"
// +kubebuilder:printcolumn:name="LastTransition",type="string",type="date",JSONPath=".status.conditions[?(@.type==\"Ready\")].lastTransitionTime"
// +kubebuilder:printcolumn:name="ObservedGeneration",type="integer",JSONPath=".status.conditions[?(@.type==\"Ready\")].observedGeneration"
// +kubebuilder:printcolumn:name="Generation",type="integer",JSONPath=".metadata.generation"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// SampleClusterIssuer is the Schema for the clusterissuers API
type SampleClusterIssuer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IssuerSpec            `json:"spec,omitempty"`
	Status v1alpha1.IssuerStatus `json:"status,omitempty"`
}

func (vi *SampleClusterIssuer) GetStatus() *v1alpha1.IssuerStatus {
	return &vi.Status
}

func (vi *SampleClusterIssuer) GetIssuerTypeIdentifier() string {
	return "sampleclusterissuers.sample-issuer.example.com"
}

var _ v1alpha1.Issuer = &SampleClusterIssuer{}

//+kubebuilder:object:root=true

// SampleClusterIssuerList contains a list of ClusterIssuer
type SampleClusterIssuerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SampleClusterIssuer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SampleClusterIssuer{}, &SampleClusterIssuerList{})
}
