package controllers

import (
	"context"
	"errors"
	"testing"

	logrtesting "github.com/go-logr/logr/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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
			}
			result, err := controller.Reconcile(
				ctrl.LoggerInto(context.TODO(), logrtesting.NewTestLogger(t)),
				reconcile.Request{NamespacedName: tc.name},
			)
			if tc.expectedError != nil {
				assertErrorIs(t, tc.expectedError, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tc.expectedResult, result, "Unexpected result")

			if tc.expectedReadyConditionStatus != "" {
				issuer, err := controller.newIssuer()
				require.NoError(t, err)
				require.NoError(t, fakeClient.Get(context.TODO(), tc.name, issuer))
				_, issuerStatus, err := issuerutil.GetSpecAndStatus(issuer)
				require.NoError(t, err)
				assertIssuerHasReadyCondition(t, tc.expectedReadyConditionStatus, issuerStatus)
			}
		})
	}
}

func assertIssuerHasReadyCondition(t *testing.T, status sampleissuerapi.ConditionStatus, issuerStatus *sampleissuerapi.IssuerStatus) {
	condition := issuerutil.GetReadyCondition(issuerStatus)
	if !assert.NotNil(t, condition, "Ready condition not found") {
		return
	}
	assert.Equal(t, issuerReadyConditionReason, condition.Reason, "unexpected condition reason")
	assert.Equal(t, status, condition.Status, "unexpected condition status")
}
