package controllers

import (
	"context"
	"errors"
	"fmt"
	"testing"

	logrtesting "github.com/go-logr/logr/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	sampleissuerapi "github.com/cert-manager/sample-external-issuer/api/v1alpha1"
	"github.com/cert-manager/sample-external-issuer/internal/issuer/signer"
	issuerutil "github.com/cert-manager/sample-external-issuer/internal/issuer/util"
)

type fakeHealthChecker struct {
	errCheck error
}

func (o *fakeHealthChecker) Check() error {
	return o.errCheck
}

func TestIssuerReconcile(t *testing.T) {
	type testCase struct {
		kind                         string
		name                         types.NamespacedName
		objects                      []client.Object
		healthCheckerBuilder         signer.HealthCheckerBuilder
		clusterResourceNamespace     string
		expectedResult               ctrl.Result
		expectedError                error
		expectedReadyConditionStatus sampleissuerapi.ConditionStatus
	}

	tests := map[string]testCase{
		"success-issuer": {
			kind: "Issuer",
			name: types.NamespacedName{Namespace: "ns1", Name: "issuer1"},
			objects: []client.Object{
				&sampleissuerapi.Issuer{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "issuer1",
						Namespace: "ns1",
					},
					Spec: sampleissuerapi.IssuerSpec{
						AuthSecretName: "issuer1-credentials",
					},
					Status: sampleissuerapi.IssuerStatus{
						Conditions: []sampleissuerapi.IssuerCondition{
							{
								Type:   sampleissuerapi.IssuerConditionReady,
								Status: sampleissuerapi.ConditionUnknown,
							},
						},
					},
				},
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "issuer1-credentials",
						Namespace: "ns1",
					},
				},
			},
			healthCheckerBuilder: func(*sampleissuerapi.IssuerSpec, map[string][]byte) (signer.HealthChecker, error) {
				return &fakeHealthChecker{}, nil
			},
			expectedReadyConditionStatus: sampleissuerapi.ConditionTrue,
			expectedResult:               ctrl.Result{RequeueAfter: defaultHealthCheckInterval},
		},
		"success-clusterissuer": {
			kind: "ClusterIssuer",
			name: types.NamespacedName{Name: "clusterissuer1"},
			objects: []client.Object{
				&sampleissuerapi.ClusterIssuer{
					ObjectMeta: metav1.ObjectMeta{
						Name: "clusterissuer1",
					},
					Spec: sampleissuerapi.IssuerSpec{
						AuthSecretName: "clusterissuer1-credentials",
					},
					Status: sampleissuerapi.IssuerStatus{
						Conditions: []sampleissuerapi.IssuerCondition{
							{
								Type:   sampleissuerapi.IssuerConditionReady,
								Status: sampleissuerapi.ConditionUnknown,
							},
						},
					},
				},
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "clusterissuer1-credentials",
						Namespace: "kube-system",
					},
				},
			},
			healthCheckerBuilder: func(*sampleissuerapi.IssuerSpec, map[string][]byte) (signer.HealthChecker, error) {
				return &fakeHealthChecker{}, nil
			},
			clusterResourceNamespace:     "kube-system",
			expectedReadyConditionStatus: sampleissuerapi.ConditionTrue,
			expectedResult:               ctrl.Result{RequeueAfter: defaultHealthCheckInterval},
		},
		"issuer-kind-unrecognised": {
			kind: "UnrecognizedType",
			name: types.NamespacedName{Namespace: "ns1", Name: "issuer1"},
		},
		"issuer-not-found": {
			name: types.NamespacedName{Namespace: "ns1", Name: "issuer1"},
		},
		"issuer-missing-ready-condition": {
			name: types.NamespacedName{Namespace: "ns1", Name: "issuer1"},
			objects: []client.Object{
				&sampleissuerapi.Issuer{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "issuer1",
						Namespace: "ns1",
					},
				},
			},
			expectedReadyConditionStatus: sampleissuerapi.ConditionUnknown,
		},
		"issuer-missing-secret": {
			name: types.NamespacedName{Namespace: "ns1", Name: "issuer1"},
			objects: []client.Object{
				&sampleissuerapi.Issuer{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "issuer1",
						Namespace: "ns1",
					},
					Spec: sampleissuerapi.IssuerSpec{
						AuthSecretName: "issuer1-credentials",
					},
					Status: sampleissuerapi.IssuerStatus{
						Conditions: []sampleissuerapi.IssuerCondition{
							{
								Type:   sampleissuerapi.IssuerConditionReady,
								Status: sampleissuerapi.ConditionUnknown,
							},
						},
					},
				},
			},
			expectedError:                errGetAuthSecret,
			expectedReadyConditionStatus: sampleissuerapi.ConditionFalse,
		},
		"issuer-failing-healthchecker-builder": {
			name: types.NamespacedName{Namespace: "ns1", Name: "issuer1"},
			objects: []client.Object{
				&sampleissuerapi.Issuer{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "issuer1",
						Namespace: "ns1",
					},
					Spec: sampleissuerapi.IssuerSpec{
						AuthSecretName: "issuer1-credentials",
					},
					Status: sampleissuerapi.IssuerStatus{
						Conditions: []sampleissuerapi.IssuerCondition{
							{
								Type:   sampleissuerapi.IssuerConditionReady,
								Status: sampleissuerapi.ConditionUnknown,
							},
						},
					},
				},
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "issuer1-credentials",
						Namespace: "ns1",
					},
				},
			},
			healthCheckerBuilder: func(*sampleissuerapi.IssuerSpec, map[string][]byte) (signer.HealthChecker, error) {
				return nil, errors.New("simulated health checker builder error")
			},
			expectedError:                errHealthCheckerBuilder,
			expectedReadyConditionStatus: sampleissuerapi.ConditionFalse,
		},
		"issuer-failing-healthchecker-check": {
			name: types.NamespacedName{Namespace: "ns1", Name: "issuer1"},
			objects: []client.Object{
				&sampleissuerapi.Issuer{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "issuer1",
						Namespace: "ns1",
					},
					Spec: sampleissuerapi.IssuerSpec{
						AuthSecretName: "issuer1-credentials",
					},
					Status: sampleissuerapi.IssuerStatus{
						Conditions: []sampleissuerapi.IssuerCondition{
							{
								Type:   sampleissuerapi.IssuerConditionReady,
								Status: sampleissuerapi.ConditionUnknown,
							},
						},
					},
				},
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "issuer1-credentials",
						Namespace: "ns1",
					},
				},
			},
			healthCheckerBuilder: func(*sampleissuerapi.IssuerSpec, map[string][]byte) (signer.HealthChecker, error) {
				return &fakeHealthChecker{errCheck: errors.New("simulated health check error")}, nil
			},
			expectedError:                errHealthCheckerCheck,
			expectedReadyConditionStatus: sampleissuerapi.ConditionFalse,
		},
	}

	scheme := runtime.NewScheme()
	require.NoError(t, sampleissuerapi.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			eventRecorder := record.NewFakeRecorder(100)
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tc.objects...).
				Build()
			if tc.kind == "" {
				tc.kind = "Issuer"
			}
			controller := IssuerReconciler{
				Kind:                     tc.kind,
				Client:                   fakeClient,
				Scheme:                   scheme,
				HealthCheckerBuilder:     tc.healthCheckerBuilder,
				ClusterResourceNamespace: tc.clusterResourceNamespace,
				recorder:                 eventRecorder,
			}

			issuerBefore, err := controller.newIssuer()
			if err == nil {
				if err := fakeClient.Get(context.TODO(), tc.name, issuerBefore); err != nil {
					require.NoError(t, client.IgnoreNotFound(err), "unexpected error from fake client")
				}
			}

			result, reconcileErr := controller.Reconcile(
				ctrl.LoggerInto(context.TODO(), &logrtesting.TestLogger{T: t}),
				reconcile.Request{NamespacedName: tc.name},
			)

			var actualEvents []string
			for {
				select {
				case e := <-eventRecorder.Events:
					actualEvents = append(actualEvents, e)
					continue
				default:
					break
				}
				break
			}

			if tc.expectedError != nil {
				assertErrorIs(t, tc.expectedError, reconcileErr)
			} else {
				assert.NoError(t, reconcileErr)
			}

			assert.Equal(t, tc.expectedResult, result, "Unexpected result")

			// For tests where the target {Cluster}Issuer exists, we perform some further checks,
			// otherwise exit early.
			issuerAfter, err := controller.newIssuer()
			if err == nil {
				if err := fakeClient.Get(context.TODO(), tc.name, issuerAfter); err != nil {
					require.NoError(t, client.IgnoreNotFound(err), "unexpected error from fake client")
				}
			}
			if issuerAfter == nil {
				return
			}

			// If the CR is unchanged after the Reconcile then we expect no
			// Events and need not perform any further checks.
			// NB: controller-runtime FakeClient updates the Resource version.
			if issuerBefore.GetResourceVersion() == issuerAfter.GetResourceVersion() {
				assert.Empty(t, actualEvents, "Events should only be created if the {Cluster}Issuer is modified")
				return
			}
			_, issuerStatusAfter, err := issuerutil.GetSpecAndStatus(issuerAfter)
			require.NoError(t, err)

			condition := issuerutil.GetReadyCondition(issuerStatusAfter)

			if tc.expectedReadyConditionStatus != "" {
				if assert.NotNilf(
					t,
					condition,
					"Ready condition was expected but not found: tc.expectedReadyConditionStatus == %v",
					tc.expectedReadyConditionStatus,
				) {
					verifyIssuerReadyCondition(t, tc.expectedReadyConditionStatus, condition)
				}
			} else {
				assert.Nil(t, condition, "Unexpected Ready condition")
			}

			// Event checks
			if condition != nil {
				// The desired Event behaviour is as follows:
				//
				// * An Event should always be generated when the Ready condition is set.
				// * Event contents should match the status and message of the condition.
				// * Event type should be Warning if the Reconcile failed (temporary error)
				// * Event type should be warning if the condition status is failed (permanent error)
				expectedEventType := corev1.EventTypeNormal
				if reconcileErr != nil || condition.Status == sampleissuerapi.ConditionFalse {
					expectedEventType = corev1.EventTypeWarning
				}
				// If there was a Reconcile error, there will be a retry and
				// this should be reflected in the Event message.
				eventMessage := condition.Message
				if reconcileErr != nil {
					eventMessage = fmt.Sprintf("Temporary error. Retrying: %v", reconcileErr)
				}
				// Each Reconcile should only emit a single Event
				assert.Equal(
					t,
					[]string{fmt.Sprintf("%s %s %s", expectedEventType, sampleissuerapi.EventReasonIssuerReconciler, eventMessage)},
					actualEvents,
					"expected a single event matching the condition",
				)
			} else {
				assert.Empty(t, actualEvents, "Found unexpected Events without a corresponding Ready condition")
			}
		})
	}
}

func verifyIssuerReadyCondition(t *testing.T, status sampleissuerapi.ConditionStatus, condition *sampleissuerapi.IssuerCondition) {
	assert.Equal(t, status, condition.Status, "unexpected condition status")
}
