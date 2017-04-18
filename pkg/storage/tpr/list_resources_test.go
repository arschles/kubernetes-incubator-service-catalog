/*
Copyright 2017 The Kubernetes Authors.

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

package tpr

import (
	"testing"

	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestStripNamespacesFromList(t *testing.T) {
	lst := v1alpha1.BrokerList{
		Items: []v1alpha1.Broker{
			v1alpha1.Broker{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "testns1",
					Name:      "test1",
				},
			},
			v1alpha1.Broker{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "testns2",
					Name:      "test2",
				},
			},
		},
	}
	if err := stripNamespacesFromList(&lst); err != nil {
		t.Fatalf("removing namespaces from list (%s)", err)
	}
	for i, item := range lst.Items {
		if item.Namespace != "" {
			t.Errorf("item %d has a non-empty namespace %s", i, item.Namespace)
		}
	}
}

func TestGetAllNamespaces(t *testing.T) {
	const (
		ns1Name = "ns1"
	)
	cl := newFakeCoreRESTClient()
	nsList, err := getAllNamespaces(cl)
	if err != nil {
		t.Fatalf("getting all namespaces (%s)", err)
	}
	if len(nsList.Items) != 0 {
		t.Fatalf("expected 0 namespaces, got %d", len(nsList.Items))
	}
	cl.storage[ns1Name] = newTypedStorage()
	nsList, err = getAllNamespaces(cl)
	if err != nil {
		t.Fatalf("getting all namespaces (%s)", err)
	}
	if len(nsList.Items) != 1 {
		t.Fatalf("expected 1 namespace, got %d", len(nsList.Items))
	}
	if nsList.Items[0].Name != ns1Name {
		t.Fatalf("expected namespace with name %s, got %s instead", ns1Name, nsList.Items[0].Name)
	}
}
