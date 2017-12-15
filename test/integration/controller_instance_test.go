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

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	osb "github.com/pmorie/go-open-service-broker-client/v2"
	fakeosb "github.com/pmorie/go-open-service-broker-client/v2/fake"

	// avoid error `servicecatalog/v1beta1 is not enabled`
	_ "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/install"

	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1beta1"
	"github.com/kubernetes-incubator/service-catalog/test/util"
)

// TestCreateServiceInstanceNonExistentClusterServiceClassOrPlan tests that a ServiceInstance gets
// a Failed condition when the service class or service plan it references does not exist.
func TestCreateServiceInstanceNonExistentClusterServiceClassOrPlan(t *testing.T) {
	cases := []struct {
		name                string
		classExternalName   string
		planExternalName    string
		classK8sName        string
		planK8sName         string
		expectedErrorReason string
	}{
		{
			name:                "existent external class and plan name",
			classExternalName:   testClusterServiceClassName,
			planExternalName:    testClusterServicePlanName,
			expectedErrorReason: "",
		},
		{
			name:                "non-existent external class name",
			classExternalName:   "nothereclass",
			planExternalName:    testClusterServicePlanName,
			expectedErrorReason: "ReferencesNonexistentServiceClass",
		},
		{
			name:                "non-existent external plan name",
			classExternalName:   testClusterServiceClassName,
			planExternalName:    "nothereplan",
			expectedErrorReason: "ReferencesNonexistentServicePlan",
		},
		{
			name:                "non-existent external class and plan name",
			classExternalName:   "nothereclass",
			planExternalName:    "nothereplan",
			expectedErrorReason: "ReferencesNonexistentServiceClass",
		},
		{
			name:                "existent k8s class and plan name",
			classK8sName:        testClusterServiceClassGUID,
			planK8sName:         testPlanExternalID,
			expectedErrorReason: "",
		},
		{
			name:                "non-existent k8s class name",
			classK8sName:        "nothereclass",
			planK8sName:         testPlanExternalID,
			expectedErrorReason: "ReferencesNonexistentServiceClass",
		},
		{
			name:                "non-existent k8s plan name",
			classK8sName:        testClusterServiceClassGUID,
			planK8sName:         "nothereplan",
			expectedErrorReason: "ReferencesNonexistentServicePlan",
		},
		{
			name:                "non-existent k8s class and plan name",
			classK8sName:        "nothereclass",
			planK8sName:         "nothereplan",
			expectedErrorReason: "ReferencesNonexistentServiceClass",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ct := &controllerTest{
				t:      t,
				broker: getTestBroker(),
				instance: func() *v1beta1.ServiceInstance {
					i := getTestInstance()
					i.Spec.PlanReference.ClusterServiceClassExternalName = tc.classExternalName
					i.Spec.PlanReference.ClusterServicePlanExternalName = tc.planExternalName
					i.Spec.PlanReference.ClusterServiceClassName = tc.classK8sName
					i.Spec.PlanReference.ClusterServicePlanName = tc.planK8sName
					return i
				}(),
				skipVerifyingInstanceSuccess: tc.expectedErrorReason != "",
			}
			ct.run(func(ct *controllerTest) {
				status := v1beta1.ConditionTrue
				if tc.expectedErrorReason != "" {
					status = v1beta1.ConditionFalse
				}
				condition := v1beta1.ServiceInstanceCondition{
					Type:   v1beta1.ServiceInstanceConditionReady,
					Status: status,
					Reason: tc.expectedErrorReason,
				}
				if err := util.WaitForInstanceCondition(ct.client, testNamespace, testInstanceName, condition); err != nil {
					t.Fatalf("error waiting for instance condition: %v", err)
				}
			})
		})
	}
}

// TestCreateServiceInstanceNonExistsentClusterServiceBroker tests creating a
// ServiceInstance whose broker does not exist.
func TestCreateServiceInstanceNonExistentClusterServiceBroker(t *testing.T) {
	ct := &controllerTest{
		t:                            t,
		instance:                     getTestInstance(),
		skipVerifyingInstanceSuccess: true,
		preCreateInstance: func(ct *controllerTest) {
			serviceClass := &v1beta1.ClusterServiceClass{
				ObjectMeta: metav1.ObjectMeta{Name: testClusterServiceClassGUID},
				Spec: v1beta1.ClusterServiceClassSpec{
					ClusterServiceBrokerName: testClusterServiceBrokerName,
					ExternalID:               testClusterServiceClassGUID,
					ExternalName:             testClusterServiceClassName,
					Description:              "a test service",
					Bindable:                 true,
				},
			}
			if _, err := ct.client.ClusterServiceClasses().Create(serviceClass); err != nil {
				t.Fatalf("error creating ClusterServiceClass: %v", err)
			}

			if err := util.WaitForClusterServiceClassToExist(ct.client, testClusterServiceClassGUID); err != nil {
				t.Fatalf("error waiting for ClusterServiceClass to exist: %v", err)
			}

			servicePlan := &v1beta1.ClusterServicePlan{
				ObjectMeta: metav1.ObjectMeta{Name: testPlanExternalID},
				Spec: v1beta1.ClusterServicePlanSpec{
					ClusterServiceBrokerName: testClusterServiceBrokerName,
					ExternalID:               testPlanExternalID,
					ExternalName:             testClusterServicePlanName,
					Description:              "a test plan",
					ClusterServiceClassRef: v1beta1.ClusterObjectReference{
						Name: testClusterServiceClassGUID,
					},
				},
			}
			if _, err := ct.client.ClusterServicePlans().Create(servicePlan); err != nil {
				t.Fatalf("error creating ClusterServicePlan: %v", err)
			}
			if err := util.WaitForClusterServicePlanToExist(ct.client, testPlanExternalID); err != nil {
				t.Fatalf("error waiting for ClusterServicePlan to exist: %v", err)
			}
		},
	}
	ct.run(func(ct *controllerTest) {
		if err := util.WaitForInstanceCondition(ct.client, testNamespace, testInstanceName, v1beta1.ServiceInstanceCondition{
			Type:   v1beta1.ServiceInstanceConditionReady,
			Status: v1beta1.ConditionFalse,
			Reason: "ReferencesNonexistentBroker",
		}); err != nil {
			t.Fatalf("error waiting for instance reconciliation to fail: %v", err)
		}
	})
}

// TestCreateServiceInstanceWithAuthError tests creating a SerivceInstance when
// the secret containing the broker authorization info cannot be found.
func TestCreateServiceInstanceWithAuthError(t *testing.T) {
	ct := &controllerTest{
		t: t,
		broker: func() *v1beta1.ClusterServiceBroker {
			b := getTestBroker()
			b.Spec.AuthInfo = &v1beta1.ServiceBrokerAuthInfo{
				Basic: &v1beta1.BasicAuthConfig{
					SecretRef: &v1beta1.ObjectReference{
						Namespace: testNamespace,
						Name:      "secret-name",
					},
				},
			}
			return b
		}(),
		instance:                     getTestInstance(),
		skipVerifyingInstanceSuccess: true,
		preCreateBroker: func(ct *controllerTest) {
			prependGetSecretReaction(ct.kubeClient, "secret-name", map[string][]byte{
				"username": []byte("user"),
				"password": []byte("pass"),
			})
		},
		preCreateInstance: func(ct *controllerTest) {
			prependGetSecretNotFoundReaction(ct.kubeClient)
		},
	}
	ct.run(func(ct *controllerTest) {
		if err := util.WaitForInstanceCondition(ct.client, testNamespace, testInstanceName, v1beta1.ServiceInstanceCondition{
			Type:   v1beta1.ServiceInstanceConditionReady,
			Status: v1beta1.ConditionFalse,
			Reason: "ErrorGettingAuthCredentials",
		}); err != nil {
			t.Fatalf("error waiting for instance reconciliation to fail: %v", err)
		}
	})
}

// TestCreateServiceInstanceWithParameters tests creating a ServiceInstance
// with parameters.
func TestCreateServiceInstanceWithParameters(t *testing.T) {
	type secretDef struct {
		name string
		data map[string][]byte
	}
	cases := []struct {
		name           string
		params         map[string]interface{}
		paramsFrom     []v1beta1.ParametersFromSource
		secrets        []secretDef
		expectedParams map[string]interface{}
		expectedError  bool
	}{
		{
			name:           "no params",
			expectedParams: nil,
		},
		{
			name: "plain params",
			params: map[string]interface{}{
				"Name": "test-param",
				"Args": map[string]interface{}{
					"first":  "first-arg",
					"second": "second-arg",
				},
			},
			expectedParams: map[string]interface{}{
				"Name": "test-param",
				"Args": map[string]interface{}{
					"first":  "first-arg",
					"second": "second-arg",
				},
			},
		},
		{
			name: "secret params",
			paramsFrom: []v1beta1.ParametersFromSource{
				{
					SecretKeyRef: &v1beta1.SecretKeyReference{
						Name: "secret-name",
						Key:  "secret-key",
					},
				},
			},
			secrets: []secretDef{
				{
					name: "secret-name",
					data: map[string][]byte{
						"secret-key": []byte(`{"A":"B","C":{"D":"E","F":"G"}}`),
					},
				},
			},
			expectedParams: map[string]interface{}{
				"A": "B",
				"C": map[string]interface{}{
					"D": "E",
					"F": "G",
				},
			},
		},
		{
			name: "plain and secret params",
			params: map[string]interface{}{
				"Name": "test-param",
				"Args": map[string]interface{}{
					"first":  "first-arg",
					"second": "second-arg",
				},
			},
			paramsFrom: []v1beta1.ParametersFromSource{
				{
					SecretKeyRef: &v1beta1.SecretKeyReference{
						Name: "secret-name",
						Key:  "secret-key",
					},
				},
			},
			secrets: []secretDef{
				{
					name: "secret-name",
					data: map[string][]byte{
						"secret-key": []byte(`{"A":"B","C":{"D":"E","F":"G"}}`),
					},
				},
			},
			expectedParams: map[string]interface{}{
				"Name": "test-param",
				"Args": map[string]interface{}{
					"first":  "first-arg",
					"second": "second-arg",
				},
				"A": "B",
				"C": map[string]interface{}{
					"D": "E",
					"F": "G",
				},
			},
		},
		{
			name: "missing secret",
			paramsFrom: []v1beta1.ParametersFromSource{
				{
					SecretKeyRef: &v1beta1.SecretKeyReference{
						Name: "secret-name",
						Key:  "secret-key",
					},
				},
			},
			expectedError: true,
		},
		{
			name: "missing secret key",
			paramsFrom: []v1beta1.ParametersFromSource{
				{
					SecretKeyRef: &v1beta1.SecretKeyReference{
						Name: "secret-name",
						Key:  "other-secret-key",
					},
				},
			},
			secrets: []secretDef{
				{
					name: "secret-name",
					data: map[string][]byte{
						"secret-key": []byte(`bad`),
					},
				},
			},
			expectedError: true,
		},
		{
			name: "empty secret data",
			paramsFrom: []v1beta1.ParametersFromSource{
				{
					SecretKeyRef: &v1beta1.SecretKeyReference{
						Name: "secret-name",
						Key:  "secret-key",
					},
				},
			},
			secrets: []secretDef{
				{
					name: "secret-name",
					data: map[string][]byte{},
				},
			},
			expectedError: true,
		},
		{
			name: "bad secret data",
			paramsFrom: []v1beta1.ParametersFromSource{
				{
					SecretKeyRef: &v1beta1.SecretKeyReference{
						Name: "secret-name",
						Key:  "secret-key",
					},
				},
			},
			secrets: []secretDef{
				{
					name: "secret-name",
					data: map[string][]byte{
						"secret-key": []byte(`bad`),
					},
				},
			},
			expectedError: true,
		},
		{
			name: "no params in secret data",
			paramsFrom: []v1beta1.ParametersFromSource{
				{
					SecretKeyRef: &v1beta1.SecretKeyReference{
						Name: "secret-name",
						Key:  "secret-key",
					},
				},
			},
			secrets: []secretDef{
				{
					name: "secret-name",
					data: map[string][]byte{
						"secret-key": []byte(`{}`),
					},
				},
			},
			expectedParams: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ct := &controllerTest{
				t:      t,
				broker: getTestBroker(),
				instance: func() *v1beta1.ServiceInstance {
					i := getTestInstance()
					if tc.params != nil {
						i.Spec.Parameters = convertParametersIntoRawExtension(t, tc.params)
					}
					i.Spec.ParametersFrom = tc.paramsFrom
					return i
				}(),
				skipVerifyingInstanceSuccess: tc.expectedError,
				setup: func(ct *controllerTest) {
					for _, secret := range tc.secrets {
						prependGetSecretReaction(ct.kubeClient, secret.name, secret.data)
					}
				},
			}
			ct.run(func(ct *controllerTest) {
				if tc.expectedError {
					if err := util.WaitForInstanceCondition(ct.client, testNamespace, testInstanceName, v1beta1.ServiceInstanceCondition{
						Type:   v1beta1.ServiceInstanceConditionReady,
						Status: v1beta1.ConditionFalse,
						Reason: "ErrorWithParameters",
					}); err != nil {
						t.Fatalf("error waiting for instance reconciliation to fail: %v", err)
					}
				} else {
					brokerAction := getLastBrokerAction(t, ct.osbClient, fakeosb.ProvisionInstance)
					if e, a := tc.expectedParams, brokerAction.Request.(*osb.ProvisionRequest).Parameters; !reflect.DeepEqual(e, a) {
						t.Fatalf("unexpected diff in provision parameters: expected %v, got %v", e, a)
					}
				}
			})
		})
	}
}

// TestUpdateServiceInstanceChangePlans tests changing plans for an existing
// ServiceInstance.
func TestUpdateServiceInstanceChangePlans(t *testing.T) {
	otherPlanName := "otherplanname"
	otherPlanID := "other-plan-id"
	cases := []struct {
		name             string
		useExternalNames bool
	}{
		{
			name:             "external",
			useExternalNames: true,
		},
		{
			name:             "k8s",
			useExternalNames: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ct := &controllerTest{
				t:      t,
				broker: getTestBroker(),
				instance: func() *v1beta1.ServiceInstance {
					i := getTestInstance()
					if !tc.useExternalNames {
						i.Spec.ClusterServiceClassExternalName = ""
						i.Spec.ClusterServicePlanExternalName = ""
						i.Spec.ClusterServiceClassName = testClusterServiceClassGUID
						i.Spec.ClusterServicePlanName = testPlanExternalID
					}
					return i
				}(),
				setup: func(ct *controllerTest) {
					catalogResponse := ct.osbClient.CatalogReaction.(*fakeosb.CatalogReaction).Response
					catalogResponse.Services[0].PlanUpdatable = truePtr()
					catalogResponse.Services[0].Plans = append(catalogResponse.Services[0].Plans, osb.Plan{
						Name:        otherPlanName,
						Free:        truePtr(),
						ID:          otherPlanID,
						Description: "another test plan",
					})
				},
			}
			ct.run(func(ct *controllerTest) {
				if tc.useExternalNames {
					ct.instance.Spec.ClusterServicePlanExternalName = otherPlanName
				} else {
					ct.instance.Spec.ClusterServicePlanName = otherPlanID
				}

				if _, err := ct.client.ServiceInstances(testNamespace).Update(ct.instance); err != nil {
					t.Fatalf("error updating Instance: %v", err)
				}

				if err := util.WaitForInstanceReconciledGeneration(ct.client, testNamespace, testInstanceName, ct.instance.Status.ReconciledGeneration+1); err != nil {
					t.Fatalf("error waiting for instance to reconcile: %v", err)
				}

				brokerAction := getLastBrokerAction(t, ct.osbClient, fakeosb.UpdateInstance)
				request := brokerAction.Request.(*osb.UpdateInstanceRequest)
				if request.PlanID == nil {
					t.Fatalf("plan ID not sent in update instance request: request = %+v", request)
				}
				if e, a := otherPlanID, *request.PlanID; e != a {
					t.Fatalf("unexpected plan ID: expected %s, got %s", e, a)
				}
			})
		})
	}
}

// TestUpdateServiceInstanceChangePlansToNonexistentPlan tests changing plans
// to a non-existent plan.
func TestUpdateServiceInstanceChangePlansToNonexistentPlan(t *testing.T) {
	cases := []struct {
		name             string
		useExternalNames bool
	}{
		{
			name:             "external",
			useExternalNames: true,
		},
		{
			name:             "k8s",
			useExternalNames: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ct := &controllerTest{
				t:      t,
				broker: getTestBroker(),
				instance: func() *v1beta1.ServiceInstance {
					i := getTestInstance()
					if !tc.useExternalNames {
						i.Spec.ClusterServiceClassExternalName = ""
						i.Spec.ClusterServicePlanExternalName = ""
						i.Spec.ClusterServiceClassName = testClusterServiceClassGUID
						i.Spec.ClusterServicePlanName = testPlanExternalID
					}
					return i
				}(),
				setup: func(ct *controllerTest) {
					ct.osbClient.CatalogReaction.(*fakeosb.CatalogReaction).Response.Services[0].PlanUpdatable = truePtr()
				},
			}
			ct.run(func(ct *controllerTest) {
				if tc.useExternalNames {
					ct.instance.Spec.ClusterServicePlanExternalName = "other-plan-name"
				} else {
					ct.instance.Spec.ClusterServicePlanName = "other-plan-id"
				}

				if _, err := ct.client.ServiceInstances(testNamespace).Update(ct.instance); err != nil {
					t.Fatalf("error updating Instance: %v", err)
				}

				if err := util.WaitForInstanceCondition(ct.client, testNamespace, testInstanceName, v1beta1.ServiceInstanceCondition{
					Type:   v1beta1.ServiceInstanceConditionReady,
					Status: v1beta1.ConditionFalse,
					Reason: "ReferencesNonexistentServicePlan",
				}); err != nil {
					t.Fatalf("error waiting for instance reconciliation to fail: %v", err)
				}

			})
		})
	}
}

// TestUpdateServiceInstanceUpdateParameters tests updating the parameters
// of an existing ServiceInstance.
func TestUpdateServiceInstanceUpdateParameters(t *testing.T) {
	cases := []struct {
		name                        string
		createdWithParams           bool
		createdWithParamsFromSecret bool
		updateParams                bool
		updateParamsFromSecret      bool
		updateSecret                bool
		deleteParams                bool
		deleteParamsFromSecret      bool
	}{
		{
			name:              "add param",
			createdWithParams: false,
			updateParams:      true,
		},
		{
			name:              "update param",
			createdWithParams: true,
			updateParams:      true,
		},
		{
			name:              "delete param",
			createdWithParams: true,
			deleteParams:      true,
		},
		{
			name:                        "add param with secret",
			createdWithParams:           false,
			createdWithParamsFromSecret: true,
			updateParams:                true,
		},
		{
			name:                        "update param with secret",
			createdWithParams:           true,
			createdWithParamsFromSecret: true,
			updateParams:                true,
		},
		{
			name:                        "delete param with secret",
			createdWithParams:           true,
			createdWithParamsFromSecret: true,
			deleteParams:                true,
		},
		{
			name: "add secret param",
			createdWithParamsFromSecret: false,
			updateParamsFromSecret:      true,
		},
		{
			name: "update secret param",
			createdWithParamsFromSecret: true,
			updateParamsFromSecret:      true,
		},
		{
			name: "delete secret param",
			createdWithParamsFromSecret: true,
			deleteParamsFromSecret:      true,
		},
		{
			name:                        "add secret param with plain param",
			createdWithParams:           true,
			createdWithParamsFromSecret: false,
			updateParamsFromSecret:      true,
		},
		{
			name:                        "update secret param with plain param",
			createdWithParams:           true,
			createdWithParamsFromSecret: true,
			updateParamsFromSecret:      true,
		},
		{
			name:                        "delete secret param with plain param",
			createdWithParams:           true,
			createdWithParamsFromSecret: true,
			deleteParamsFromSecret:      true,
		},
		{
			name: "update secret",
			createdWithParamsFromSecret: true,
			updateSecret:                true,
		},
		{
			name:                        "update secret with plain param",
			createdWithParams:           true,
			createdWithParamsFromSecret: true,
			updateSecret:                true,
		},
		{
			name:                        "add plain and secret param",
			createdWithParams:           false,
			createdWithParamsFromSecret: false,
			updateParams:                true,
			updateParamsFromSecret:      true,
		},
		{
			name:                        "update plain and secret param",
			createdWithParams:           true,
			createdWithParamsFromSecret: true,
			updateParams:                true,
			updateParamsFromSecret:      true,
		},
		{
			name:                        "delete plain and secret param",
			createdWithParams:           true,
			createdWithParamsFromSecret: true,
			deleteParams:                true,
			deleteParamsFromSecret:      true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ct := &controllerTest{
				t:      t,
				broker: getTestBroker(),
				instance: func() *v1beta1.ServiceInstance {
					i := getTestInstance()
					if tc.createdWithParams {
						i.Spec.Parameters = convertParametersIntoRawExtension(t,
							map[string]interface{}{
								"param-key": "param-value",
							})
					}
					if tc.createdWithParamsFromSecret {
						i.Spec.ParametersFrom = []v1beta1.ParametersFromSource{
							{
								SecretKeyRef: &v1beta1.SecretKeyReference{
									Name: "secret-name",
									Key:  "secret-key",
								},
							},
						}
					}
					return i
				}(),
				setup: func(ct *controllerTest) {
					prependGetSecretReaction(ct.kubeClient, "secret-name", map[string][]byte{
						"secret-key": []byte(`{"secret-param-key":"secret-param-value"}`),
					})
					prependGetSecretReaction(ct.kubeClient, "other-secret-name", map[string][]byte{
						"other-secret-key": []byte(`{"other-secret-param-key":"other-secret-param-value"}`),
					})
				},
			}
			ct.run(func(ct *controllerTest) {
				if tc.updateParams {
					ct.instance.Spec.Parameters = convertParametersIntoRawExtension(t,
						map[string]interface{}{
							"param-key": "new-param-value",
						})
				} else if tc.deleteParams {
					ct.instance.Spec.Parameters = nil
				}

				if tc.updateParamsFromSecret {
					ct.instance.Spec.ParametersFrom = []v1beta1.ParametersFromSource{
						{
							SecretKeyRef: &v1beta1.SecretKeyReference{
								Name: "other-secret-name",
								Key:  "other-secret-key",
							},
						},
					}
				} else if tc.deleteParamsFromSecret {
					ct.instance.Spec.ParametersFrom = nil
				}

				if tc.updateSecret {
					prependGetSecretReaction(ct.kubeClient, "secret-name", map[string][]byte{
						"secret-key": []byte(`{"new-secret-param-key":"new-secret-param-value"}`),
					})
					ct.instance.Spec.UpdateRequests++
				}

				if _, err := ct.client.ServiceInstances(testNamespace).Update(ct.instance); err != nil {
					t.Fatalf("error updating Instance: %v", err)
				}

				if err := util.WaitForInstanceReconciledGeneration(ct.client, testNamespace, testInstanceName, ct.instance.Status.ReconciledGeneration+1); err != nil {
					t.Fatalf("error waiting for instance to reconcile: %v", err)
				}

				expectedParameters := make(map[string]interface{})

				if tc.updateParams {
					expectedParameters["param-key"] = "new-param-value"
				} else if tc.createdWithParams && !tc.deleteParams {
					expectedParameters["param-key"] = "param-value"
				}

				if tc.updateParamsFromSecret {
					expectedParameters["other-secret-param-key"] = "other-secret-param-value"
				} else if tc.updateSecret {
					expectedParameters["new-secret-param-key"] = "new-secret-param-value"
				} else if tc.createdWithParamsFromSecret && !tc.deleteParamsFromSecret {
					expectedParameters["secret-param-key"] = "secret-param-value"
				}

				brokerAction := getLastBrokerAction(t, ct.osbClient, fakeosb.UpdateInstance)
				request := brokerAction.Request.(*osb.UpdateInstanceRequest)
				if e, a := expectedParameters, request.Parameters; !reflect.DeepEqual(e, a) {
					t.Fatalf("unexpected parameters: expected %v, got %v", e, a)
				}
			})
		})
	}
}

// TestCreateServiceInstanceWithInvalidParameters tests creating a ServiceInstance
// with invalid parameters.
func TestCreateServiceInstanceWithInvalidParameters(t *testing.T) {
	ct := &controllerTest{
		t:      t,
		broker: getTestBroker(),
	}
	ct.run(func(ct *controllerTest) {
		instance := getTestInstance()
		instance.Spec.Parameters = convertParametersIntoRawExtension(t,
			map[string]interface{}{
				"Name": "test-param",
				"Args": map[string]interface{}{
					"first":  "first-arg",
					"second": "second-arg",
				},
			})
		instance.Spec.Parameters.Raw[0] = 0x21
		if _, err := ct.client.ServiceInstances(instance.Namespace).Create(instance); err == nil {
			t.Fatalf("expected instance to fail to be created due to invalid parameters")
		}
	})
}

// TimeoutError is an error sent back in a url.Error from the broker when
// the request has timed out at the network layer.
type TimeoutError string

// Timeout returns true since TimeoutError indicates that there was a timeout.
// This method is so that TimeoutError implements the url.timeout interface.
func (e TimeoutError) Timeout() bool {
	return true
}

// Error returns the TimeoutError as a string
func (e TimeoutError) Error() string {
	return string(e)
}

// TestCreateServiceInstanceWithProvisionFailure tests creating a ServiceInstance
// with various failure results in response to the provision request.
func TestCreateServiceInstanceWithProvisionFailure(t *testing.T) {
	cases := []struct {
		name                     string
		statusCode               int
		nonHttpResponseError     error
		conditionReason          string
		expectFailCondition      bool
		triggersOrphanMitigation bool
	}{
		{
			name:                "Status OK",
			statusCode:          http.StatusOK,
			conditionReason:     "ClusterServiceBrokerReturnedFailure",
			expectFailCondition: true,
		},
		{
			name:                     "Status Created",
			statusCode:               http.StatusCreated,
			conditionReason:          "ClusterServiceBrokerReturnedFailure",
			expectFailCondition:      true,
			triggersOrphanMitigation: true,
		},
		{
			name:                     "other 2xx",
			statusCode:               http.StatusNoContent,
			conditionReason:          "ClusterServiceBrokerReturnedFailure",
			expectFailCondition:      true,
			triggersOrphanMitigation: true,
		},
		{
			name:                "3XX",
			statusCode:          300,
			conditionReason:     "ClusterServiceBrokerReturnedFailure",
			expectFailCondition: true,
		},
		{
			name:                     "Status Request Timeout",
			statusCode:               http.StatusRequestTimeout,
			conditionReason:          "ClusterServiceBrokerReturnedFailure",
			expectFailCondition:      true,
			triggersOrphanMitigation: true,
		},
		{
			name:                "other 4XX",
			statusCode:          400,
			conditionReason:     "ClusterServiceBrokerReturnedFailure",
			expectFailCondition: true,
		},
		{
			name:                     "5XX",
			statusCode:               500,
			conditionReason:          "ClusterServiceBrokerReturnedFailure",
			expectFailCondition:      true,
			triggersOrphanMitigation: true,
		},
		{
			name:                 "non-url transport error",
			nonHttpResponseError: fmt.Errorf("non-url error"),
			conditionReason:      "ErrorCallingProvision",
		},
		{
			name: "non-timeout url error",
			nonHttpResponseError: &url.Error{
				Op:  "Put",
				URL: "https://fakebroker.com/v2/service_instances/instance_id",
				Err: fmt.Errorf("non-timeout error"),
			},
			conditionReason: "ErrorCallingProvision",
		},
		{
			name: "network timeout",
			nonHttpResponseError: &url.Error{
				Op:  "Put",
				URL: "https://fakebroker.com/v2/service_instances/instance_id",
				Err: TimeoutError("timeout error"),
			},
			conditionReason:          "ErrorCallingProvision",
			expectFailCondition:      true,
			triggersOrphanMitigation: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ct := &controllerTest{
				t:                            t,
				broker:                       getTestBroker(),
				instance:                     getTestInstance(),
				skipVerifyingInstanceSuccess: true,
				setup: func(ct *controllerTest) {
					reactionError := tc.nonHttpResponseError
					if reactionError == nil {
						reactionError = osb.HTTPStatusCodeError{
							StatusCode:   tc.statusCode,
							ErrorMessage: strPtr("error message"),
							Description:  strPtr("response description"),
						}
					}
					ct.osbClient.ProvisionReaction = &fakeosb.ProvisionReaction{
						Error: reactionError,
					}
				},
			}
			ct.run(func(ct *controllerTest) {
				var condition v1beta1.ServiceInstanceCondition
				if tc.expectFailCondition {
					condition = v1beta1.ServiceInstanceCondition{
						Type:   v1beta1.ServiceInstanceConditionFailed,
						Status: v1beta1.ConditionTrue,
						Reason: tc.conditionReason,
					}
				} else {
					condition = v1beta1.ServiceInstanceCondition{
						Type:   v1beta1.ServiceInstanceConditionReady,
						Status: v1beta1.ConditionFalse,
						Reason: tc.conditionReason,
					}
				}
				if err := util.WaitForInstanceCondition(ct.client, testNamespace, testInstanceName, condition); err != nil {
					t.Fatalf("error waiting for instance condition: %v", err)
				}

				if tc.expectFailCondition {
					if err := util.WaitForInstanceReconciledGeneration(ct.client, ct.instance.Namespace, ct.instance.Name, 1); err != nil {
						t.Fatalf("error waiting for instance reconciliation to complete: %v", err)
					}
				}

				expectedLastActionType := fakeosb.ProvisionInstance
				if tc.triggersOrphanMitigation {
					expectedLastActionType = fakeosb.DeprovisionInstance
				}
				// Ensure that the last request made to the broker was of the expected type
				getLastBrokerAction(t, ct.osbClient, expectedLastActionType)

				if tc.triggersOrphanMitigation {
					instance, err := ct.client.ServiceInstances(ct.instance.Namespace).Get(ct.instance.Name, metav1.GetOptions{})
					if err != nil {
						t.Fatalf("error getting instance %s/%s back", ct.instance.Namespace, ct.instance.Name)
					}
					util.AssertServiceInstanceCondition(
						t,
						instance,
						v1beta1.ServiceInstanceConditionReady,
						v1beta1.ConditionFalse,
						"OrphanMitigationSuccessful",
					)
					if e, a := v1beta1.ServiceInstanceDeprovisionStatusSucceeded, instance.Status.DeprovisionStatus; e != a {
						t.Fatalf("unexpected deprovision status: expected %v, got %v", e, a)
					}
				}
			})
		})
	}
}