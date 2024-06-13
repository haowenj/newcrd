/*
Copyright 2024.

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

// NewDepSpec defines the desired state of NewDep
type NewDepSpec struct {
	// Image image name <name:tag>
	Image *string `json:"image,omitempty"`
	// Replica number of pods
	Replica *int32 `json:"replica,omitempty"`
}

// NewDepStatus defines the observed state of NewDep
type NewDepStatus struct {
	// RealReplica number of pods
	RealReplica int32 `json:"realReplica,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
//+kubebuilder:printcolumn:JSONPath=".status.realReplica",name=RealReplica,type=integer

// NewDep is the Schema for the newdeps API
type NewDep struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NewDepSpec   `json:"spec,omitempty"`
	Status NewDepStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// NewDepList contains a list of NewDep
type NewDepList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NewDep `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NewDep{}, &NewDepList{})
}
