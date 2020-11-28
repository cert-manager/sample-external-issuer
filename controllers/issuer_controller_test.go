package controllers

import (
	"context"
	"testing"

	logrtesting "github.com/go-logr/logr/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	sampleissuerapi "github.com/cert-manager/sample-external-issuer/api/v1alpha1"
	issuerutil "github.com/cert-manager/sample-external-issuer/internal/issuer/util"
)

func TestIssuerReconcile(t *testing.T) {
	type testCase struct {
		name                         types.NamespacedName
		objects                      []runtime.Object
		expectedResult               ctrl.Result
		expectedError                error
		expectedReadyConditionStatus sampleissuerapi.ConditionStatus
	}
	tests := map[string]testCase{
		"success": {
			name: types.NamespacedName{Namespace: "ns1", Name: "issuer1"},
			objects: []runtime.Object{
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
			expectedReadyConditionStatus: sampleissuerapi.ConditionTrue,
		},
		"issuer-not-found": {
			name: types.NamespacedName{Namespace: "ns1", Name: "issuer1"},
		},
		"issuer-missing-ready-condition": {
			name: types.NamespacedName{Namespace: "ns1", Name: "issuer1"},
			objects: []runtime.Object{
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
			objects: []runtime.Object{
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
	}

	scheme := runtime.NewScheme()
	require.NoError(t, sampleissuerapi.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			fakeClient := fake.NewFakeClientWithScheme(scheme, tc.objects...)
			controller := IssuerReconciler{
				Client: fakeClient,
				Log:    logrtesting.TestLogger{T: t},
				Scheme: scheme,
			}
			result, err := controller.Reconcile(reconcile.Request{NamespacedName: tc.name})
			if tc.expectedError != nil {
				assertErrorIs(t, tc.expectedError, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tc.expectedResult, result, "Unexpected result")

			if tc.expectedReadyConditionStatus != "" {
				var issuer sampleissuerapi.Issuer
				require.NoError(t, fakeClient.Get(context.TODO(), tc.name, &issuer))
				assertIssuerHasReadyCondition(t, tc.expectedReadyConditionStatus, &issuer)
			}
		})
	}
}

func assertIssuerHasReadyCondition(t *testing.T, status sampleissuerapi.ConditionStatus, issuer *sampleissuerapi.Issuer) {
	condition := issuerutil.GetReadyCondition(&issuer.Status)
	if !assert.NotNil(t, condition, "Ready condition not found") {
		return
	}
	assert.Equal(t, issuerReadyConditionReason, condition.Reason, "unexpected condition reason")
	assert.Equal(t, status, condition.Status, "unexpected condition status")
}
