package controllers

import (
	"errors"
	"testing"

	logrtesting "github.com/go-logr/logr/testing"
	cmapi "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/jetstack/cert-manager/pkg/apis/meta/v1"
	cmgen "github.com/jetstack/cert-manager/test/unit/gen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	sampleissuerapi "github.com/cert-manager/sample-external-issuer/api/v1alpha1"
)

func TestCertificateRequestReconcile(t *testing.T) {
	type testCase struct {
		name           types.NamespacedName
		objects        []runtime.Object
		expectedResult ctrl.Result
		expectedError  error
	}
	tests := map[string]testCase{
		"success": {
			name: types.NamespacedName{Namespace: "ns1", Name: "cr1"},
			objects: []runtime.Object{
				cmgen.CertificateRequest(
					"cr1",
					cmgen.SetCertificateRequestNamespace("ns1"),
					cmgen.SetCertificateRequestIssuer(cmmeta.ObjectReference{
						Name:  "issuer1",
						Group: sampleissuerapi.GroupVersion.Group,
						Kind:  "Issuer",
					}),
				),
			},
		},
		"certificaterequest-not-found": {
			name: types.NamespacedName{Namespace: "ns1", Name: "cr1"},
		},
		"issuer-ref-foreign-group": {
			name: types.NamespacedName{Namespace: "ns1", Name: "cr1"},
			objects: []runtime.Object{
				cmgen.CertificateRequest(
					"cr1",
					cmgen.SetCertificateRequestNamespace("ns1"),
					cmgen.SetCertificateRequestIssuer(cmmeta.ObjectReference{
						Name:  "issuer1",
						Group: "foreign-issuer.example.com",
					}),
				),
			},
		},
		"issuer-ref-unknown-kind": {
			name: types.NamespacedName{Namespace: "ns1", Name: "cr1"},
			objects: []runtime.Object{
				cmgen.CertificateRequest(
					"cr1",
					cmgen.SetCertificateRequestNamespace("ns1"),
					cmgen.SetCertificateRequestIssuer(cmmeta.ObjectReference{
						Name:  "issuer1",
						Group: sampleissuerapi.GroupVersion.Group,
						Kind:  "ForeignKind",
					}),
				),
			},
		},
	}

	scheme := runtime.NewScheme()
	require.NoError(t, sampleissuerapi.AddToScheme(scheme))
	require.NoError(t, cmapi.AddToScheme(scheme))

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			controller := CertificateRequestReconciler{
				Client: fake.NewFakeClientWithScheme(scheme, tc.objects...),
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
		})
	}
}

func assertErrorIs(t *testing.T, expectedError, actualError error) {
	if !assert.Error(t, actualError) {
		return
	}
	assert.Truef(t, errors.Is(actualError, expectedError), "unexpected error type. expected: %v, got: %v", expectedError, actualError)
}
