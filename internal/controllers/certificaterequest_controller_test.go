package controllers

import (
	"context"
	"errors"
	"testing"

	logrtesting "github.com/go-logr/logr/testing"
	cmutil "github.com/jetstack/cert-manager/pkg/api/util"
	cmapi "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/jetstack/cert-manager/pkg/apis/meta/v1"
	cmgen "github.com/jetstack/cert-manager/test/unit/gen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	sampleissuerapi "github.com/cert-manager/sample-external-issuer/api/v1alpha1"
	"github.com/cert-manager/sample-external-issuer/internal/issuer/signer"
)

type fakeSigner struct {
	errSign error
}

func (o *fakeSigner) Sign([]byte) ([]byte, error) {
	return []byte("fake signed certificate"), o.errSign
}

func TestCertificateRequestReconcile(t *testing.T) {
	type testCase struct {
		name                         types.NamespacedName
		objects                      []client.Object
		signerBuilder                signer.SignerBuilder
		clusterResourceNamespace     string
		expectedResult               ctrl.Result
		expectedError                error
		expectedReadyConditionStatus cmmeta.ConditionStatus
		expectedReadyConditionReason string
		expectedCertificate          []byte
	}
	tests := map[string]testCase{
		"success-issuer": {
			name: types.NamespacedName{Namespace: "ns1", Name: "cr1"},
			objects: []client.Object{
				cmgen.CertificateRequest(
					"cr1",
					cmgen.SetCertificateRequestNamespace("ns1"),
					cmgen.SetCertificateRequestIssuer(cmmeta.ObjectReference{
						Name:  "issuer1",
						Group: sampleissuerapi.GroupVersion.Group,
						Kind:  "Issuer",
					}),
					cmgen.SetCertificateRequestStatusCondition(cmapi.CertificateRequestCondition{
						Type:   cmapi.CertificateRequestConditionApproved,
						Status: cmmeta.ConditionTrue,
					}),
					cmgen.SetCertificateRequestStatusCondition(cmapi.CertificateRequestCondition{
						Type:   cmapi.CertificateRequestConditionReady,
						Status: cmmeta.ConditionUnknown,
					}),
				),
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
								Status: sampleissuerapi.ConditionTrue,
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
			signerBuilder: func(*sampleissuerapi.IssuerSpec, map[string][]byte) (signer.Signer, error) {
				return &fakeSigner{}, nil
			},
			expectedReadyConditionStatus: cmmeta.ConditionTrue,
			expectedReadyConditionReason: cmapi.CertificateRequestReasonIssued,
			expectedCertificate:          []byte("fake signed certificate"),
		},
		"success-cluster-issuer": {
			name: types.NamespacedName{Namespace: "ns1", Name: "cr1"},
			objects: []client.Object{
				cmgen.CertificateRequest(
					"cr1",
					cmgen.SetCertificateRequestNamespace("ns1"),
					cmgen.SetCertificateRequestIssuer(cmmeta.ObjectReference{
						Name:  "clusterissuer1",
						Group: sampleissuerapi.GroupVersion.Group,
						Kind:  "ClusterIssuer",
					}),
					cmgen.SetCertificateRequestStatusCondition(cmapi.CertificateRequestCondition{
						Type:   cmapi.CertificateRequestConditionApproved,
						Status: cmmeta.ConditionTrue,
					}),
					cmgen.SetCertificateRequestStatusCondition(cmapi.CertificateRequestCondition{
						Type:   cmapi.CertificateRequestConditionReady,
						Status: cmmeta.ConditionUnknown,
					}),
				),
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
								Status: sampleissuerapi.ConditionTrue,
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
			signerBuilder: func(*sampleissuerapi.IssuerSpec, map[string][]byte) (signer.Signer, error) {
				return &fakeSigner{}, nil
			},
			clusterResourceNamespace:     "kube-system",
			expectedReadyConditionStatus: cmmeta.ConditionTrue,
			expectedReadyConditionReason: cmapi.CertificateRequestReasonIssued,
			expectedCertificate:          []byte("fake signed certificate"),
		},
		"certificaterequest-not-found": {
			name: types.NamespacedName{Namespace: "ns1", Name: "cr1"},
		},
		"issuer-ref-foreign-group": {
			name: types.NamespacedName{Namespace: "ns1", Name: "cr1"},
			objects: []client.Object{
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
		"certificaterequest-already-ready": {
			name: types.NamespacedName{Namespace: "ns1", Name: "cr1"},
			objects: []client.Object{
				cmgen.CertificateRequest(
					"cr1",
					cmgen.SetCertificateRequestNamespace("ns1"),
					cmgen.SetCertificateRequestIssuer(cmmeta.ObjectReference{
						Name:  "issuer1",
						Group: sampleissuerapi.GroupVersion.Group,
						Kind:  "Issuer",
					}),
					cmgen.SetCertificateRequestStatusCondition(cmapi.CertificateRequestCondition{
						Type:   cmapi.CertificateRequestConditionApproved,
						Status: cmmeta.ConditionTrue,
					}),
					cmgen.SetCertificateRequestStatusCondition(cmapi.CertificateRequestCondition{
						Type:   cmapi.CertificateRequestConditionReady,
						Status: cmmeta.ConditionTrue,
					}),
				),
			},
		},
		"certificaterequest-missing-ready-condition": {
			name: types.NamespacedName{Namespace: "ns1", Name: "cr1"},
			objects: []client.Object{
				cmgen.CertificateRequest(
					"cr1",
					cmgen.SetCertificateRequestNamespace("ns1"),
					cmgen.SetCertificateRequestIssuer(cmmeta.ObjectReference{
						Name:  "issuer1",
						Group: sampleissuerapi.GroupVersion.Group,
						Kind:  "Issuer",
					}),
					cmgen.SetCertificateRequestStatusCondition(cmapi.CertificateRequestCondition{
						Type:   cmapi.CertificateRequestConditionApproved,
						Status: cmmeta.ConditionTrue,
					}),
				),
			},
			expectedReadyConditionStatus: cmmeta.ConditionFalse,
			expectedReadyConditionReason: cmapi.CertificateRequestReasonPending,
		},
		"issuer-ref-unknown-kind": {
			name: types.NamespacedName{Namespace: "ns1", Name: "cr1"},
			objects: []client.Object{
				cmgen.CertificateRequest(
					"cr1",
					cmgen.SetCertificateRequestNamespace("ns1"),
					cmgen.SetCertificateRequestIssuer(cmmeta.ObjectReference{
						Name:  "issuer1",
						Group: sampleissuerapi.GroupVersion.Group,
						Kind:  "ForeignKind",
					}),
					cmgen.SetCertificateRequestStatusCondition(cmapi.CertificateRequestCondition{
						Type:   cmapi.CertificateRequestConditionApproved,
						Status: cmmeta.ConditionTrue,
					}),
					cmgen.SetCertificateRequestStatusCondition(cmapi.CertificateRequestCondition{
						Type:   cmapi.CertificateRequestConditionReady,
						Status: cmmeta.ConditionUnknown,
					}),
				),
			},
			expectedReadyConditionStatus: cmmeta.ConditionFalse,
			expectedReadyConditionReason: cmapi.CertificateRequestReasonFailed,
		},
		"issuer-not-found": {
			name: types.NamespacedName{Namespace: "ns1", Name: "cr1"},
			objects: []client.Object{
				cmgen.CertificateRequest(
					"cr1",
					cmgen.SetCertificateRequestNamespace("ns1"),
					cmgen.SetCertificateRequestIssuer(cmmeta.ObjectReference{
						Name:  "issuer1",
						Group: sampleissuerapi.GroupVersion.Group,
						Kind:  "Issuer",
					}),
					cmgen.SetCertificateRequestStatusCondition(cmapi.CertificateRequestCondition{
						Type:   cmapi.CertificateRequestConditionApproved,
						Status: cmmeta.ConditionTrue,
					}),
					cmgen.SetCertificateRequestStatusCondition(cmapi.CertificateRequestCondition{
						Type:   cmapi.CertificateRequestConditionReady,
						Status: cmmeta.ConditionUnknown,
					}),
				),
			},
			expectedError:                errGetIssuer,
			expectedReadyConditionStatus: cmmeta.ConditionFalse,
			expectedReadyConditionReason: cmapi.CertificateRequestReasonPending,
		},
		"clusterissuer-not-found": {
			name: types.NamespacedName{Namespace: "ns1", Name: "cr1"},
			objects: []client.Object{
				cmgen.CertificateRequest(
					"cr1",
					cmgen.SetCertificateRequestNamespace("ns1"),
					cmgen.SetCertificateRequestIssuer(cmmeta.ObjectReference{
						Name:  "clusterissuer1",
						Group: sampleissuerapi.GroupVersion.Group,
						Kind:  "ClusterIssuer",
					}),
					cmgen.SetCertificateRequestStatusCondition(cmapi.CertificateRequestCondition{
						Type:   cmapi.CertificateRequestConditionApproved,
						Status: cmmeta.ConditionTrue,
					}),
					cmgen.SetCertificateRequestStatusCondition(cmapi.CertificateRequestCondition{
						Type:   cmapi.CertificateRequestConditionReady,
						Status: cmmeta.ConditionUnknown,
					}),
				),
			},
			expectedError:                errGetIssuer,
			expectedReadyConditionStatus: cmmeta.ConditionFalse,
			expectedReadyConditionReason: cmapi.CertificateRequestReasonPending,
		},
		"issuer-not-ready": {
			name: types.NamespacedName{Namespace: "ns1", Name: "cr1"},
			objects: []client.Object{
				cmgen.CertificateRequest(
					"cr1",
					cmgen.SetCertificateRequestNamespace("ns1"),
					cmgen.SetCertificateRequestIssuer(cmmeta.ObjectReference{
						Name:  "issuer1",
						Group: sampleissuerapi.GroupVersion.Group,
						Kind:  "Issuer",
					}),
					cmgen.SetCertificateRequestStatusCondition(cmapi.CertificateRequestCondition{
						Type:   cmapi.CertificateRequestConditionApproved,
						Status: cmmeta.ConditionTrue,
					}),
					cmgen.SetCertificateRequestStatusCondition(cmapi.CertificateRequestCondition{
						Type:   cmapi.CertificateRequestConditionReady,
						Status: cmmeta.ConditionUnknown,
					}),
				),
				&sampleissuerapi.Issuer{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "issuer1",
						Namespace: "ns1",
					},
					Status: sampleissuerapi.IssuerStatus{
						Conditions: []sampleissuerapi.IssuerCondition{
							{
								Type:   sampleissuerapi.IssuerConditionReady,
								Status: sampleissuerapi.ConditionFalse,
							},
						},
					},
				},
			},
			expectedError:                errIssuerNotReady,
			expectedReadyConditionStatus: cmmeta.ConditionFalse,
			expectedReadyConditionReason: cmapi.CertificateRequestReasonPending,
		},
		"issuer-secret-not-found": {
			name: types.NamespacedName{Namespace: "ns1", Name: "cr1"},
			objects: []client.Object{
				cmgen.CertificateRequest(
					"cr1",
					cmgen.SetCertificateRequestNamespace("ns1"),
					cmgen.SetCertificateRequestIssuer(cmmeta.ObjectReference{
						Name:  "issuer1",
						Group: sampleissuerapi.GroupVersion.Group,
						Kind:  "Issuer",
					}),
					cmgen.SetCertificateRequestStatusCondition(cmapi.CertificateRequestCondition{
						Type:   cmapi.CertificateRequestConditionApproved,
						Status: cmmeta.ConditionTrue,
					}),
					cmgen.SetCertificateRequestStatusCondition(cmapi.CertificateRequestCondition{
						Type:   cmapi.CertificateRequestConditionReady,
						Status: cmmeta.ConditionUnknown,
					}),
				),
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
								Status: sampleissuerapi.ConditionTrue,
							},
						},
					},
				},
			},
			expectedError:                errGetAuthSecret,
			expectedReadyConditionStatus: cmmeta.ConditionFalse,
			expectedReadyConditionReason: cmapi.CertificateRequestReasonPending,
		},
		"signer-builder-error": {
			name: types.NamespacedName{Namespace: "ns1", Name: "cr1"},
			objects: []client.Object{
				cmgen.CertificateRequest(
					"cr1",
					cmgen.SetCertificateRequestNamespace("ns1"),
					cmgen.SetCertificateRequestIssuer(cmmeta.ObjectReference{
						Name:  "issuer1",
						Group: sampleissuerapi.GroupVersion.Group,
						Kind:  "Issuer",
					}),
					cmgen.SetCertificateRequestStatusCondition(cmapi.CertificateRequestCondition{
						Type:   cmapi.CertificateRequestConditionApproved,
						Status: cmmeta.ConditionTrue,
					}),
					cmgen.SetCertificateRequestStatusCondition(cmapi.CertificateRequestCondition{
						Type:   cmapi.CertificateRequestConditionReady,
						Status: cmmeta.ConditionUnknown,
					}),
				),
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
								Status: sampleissuerapi.ConditionTrue,
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
			signerBuilder: func(*sampleissuerapi.IssuerSpec, map[string][]byte) (signer.Signer, error) {
				return nil, errors.New("simulated signer builder error")
			},
			expectedError:                errSignerBuilder,
			expectedReadyConditionStatus: cmmeta.ConditionFalse,
			expectedReadyConditionReason: cmapi.CertificateRequestReasonPending,
		},
		"signer-error": {
			name: types.NamespacedName{Namespace: "ns1", Name: "cr1"},
			objects: []client.Object{
				cmgen.CertificateRequest(
					"cr1",
					cmgen.SetCertificateRequestNamespace("ns1"),
					cmgen.SetCertificateRequestIssuer(cmmeta.ObjectReference{
						Name:  "issuer1",
						Group: sampleissuerapi.GroupVersion.Group,
						Kind:  "Issuer",
					}),
					cmgen.SetCertificateRequestStatusCondition(cmapi.CertificateRequestCondition{
						Type:   cmapi.CertificateRequestConditionApproved,
						Status: cmmeta.ConditionTrue,
					}),
					cmgen.SetCertificateRequestStatusCondition(cmapi.CertificateRequestCondition{
						Type:   cmapi.CertificateRequestConditionReady,
						Status: cmmeta.ConditionUnknown,
					}),
				),
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
								Status: sampleissuerapi.ConditionTrue,
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
			signerBuilder: func(*sampleissuerapi.IssuerSpec, map[string][]byte) (signer.Signer, error) {
				return &fakeSigner{errSign: errors.New("simulated sign error")}, nil
			},
			expectedError:                errSignerSign,
			expectedReadyConditionStatus: cmmeta.ConditionFalse,
			expectedReadyConditionReason: cmapi.CertificateRequestReasonPending,
		},
		"request-not-approved": {
			name: types.NamespacedName{Namespace: "ns1", Name: "cr1"},
			objects: []client.Object{
				cmgen.CertificateRequest(
					"cr1",
					cmgen.SetCertificateRequestNamespace("ns1"),
					cmgen.SetCertificateRequestIssuer(cmmeta.ObjectReference{
						Name:  "issuer1",
						Group: sampleissuerapi.GroupVersion.Group,
						Kind:  "Issuer",
					}),
					cmgen.SetCertificateRequestStatusCondition(cmapi.CertificateRequestCondition{
						Type:   cmapi.CertificateRequestConditionReady,
						Status: cmmeta.ConditionUnknown,
					}),
				),
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
								Status: sampleissuerapi.ConditionTrue,
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
			signerBuilder: func(*sampleissuerapi.IssuerSpec, map[string][]byte) (signer.Signer, error) {
				return &fakeSigner{}, nil
			},
			expectedCertificate: nil,
		},
		"request-denied": {
			name: types.NamespacedName{Namespace: "ns1", Name: "cr1"},
			objects: []client.Object{
				cmgen.CertificateRequest(
					"cr1",
					cmgen.SetCertificateRequestNamespace("ns1"),
					cmgen.SetCertificateRequestIssuer(cmmeta.ObjectReference{
						Name:  "issuer1",
						Group: sampleissuerapi.GroupVersion.Group,
						Kind:  "Issuer",
					}),
					cmgen.SetCertificateRequestStatusCondition(cmapi.CertificateRequestCondition{
						Type:   cmapi.CertificateRequestConditionDenied,
						Status: cmmeta.ConditionTrue,
					}),
					cmgen.SetCertificateRequestStatusCondition(cmapi.CertificateRequestCondition{
						Type:   cmapi.CertificateRequestConditionReady,
						Status: cmmeta.ConditionUnknown,
					}),
				),
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
								Status: sampleissuerapi.ConditionTrue,
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
			signerBuilder: func(*sampleissuerapi.IssuerSpec, map[string][]byte) (signer.Signer, error) {
				return &fakeSigner{}, nil
			},
			expectedCertificate: nil,
		},
	}

	scheme := runtime.NewScheme()
	require.NoError(t, sampleissuerapi.AddToScheme(scheme))
	require.NoError(t, cmapi.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tc.objects...).
				Build()
			controller := CertificateRequestReconciler{
				Client:                   fakeClient,
				Scheme:                   scheme,
				ClusterResourceNamespace: tc.clusterResourceNamespace,
				SignerBuilder:            tc.signerBuilder,
			}
			result, err := controller.Reconcile(
				ctrl.LoggerInto(context.TODO(), &logrtesting.TestLogger{T: t}),
				reconcile.Request{NamespacedName: tc.name},
			)
			if tc.expectedError != nil {
				assertErrorIs(t, tc.expectedError, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tc.expectedResult, result, "Unexpected result")

			var cr cmapi.CertificateRequest
			err = fakeClient.Get(context.TODO(), tc.name, &cr)
			require.NoError(t, client.IgnoreNotFound(err), "unexpected error from fake client")
			if err == nil {
				if tc.expectedReadyConditionStatus != "" {
					assertCertificateRequestHasReadyCondition(t, tc.expectedReadyConditionStatus, tc.expectedReadyConditionReason, &cr)
				}
				assert.Equal(t, tc.expectedCertificate, cr.Status.Certificate)
			}
		})
	}
}

func assertErrorIs(t *testing.T, expectedError, actualError error) {
	if !assert.Error(t, actualError) {
		return
	}
	assert.Truef(t, errors.Is(actualError, expectedError), "unexpected error type. expected: %v, got: %v", expectedError, actualError)
}

func assertCertificateRequestHasReadyCondition(t *testing.T, status cmmeta.ConditionStatus, reason string, cr *cmapi.CertificateRequest) {
	condition := cmutil.GetCertificateRequestCondition(cr, cmapi.CertificateRequestConditionReady)
	if !assert.NotNil(t, condition, "Ready condition not found") {
		return
	}
	assert.Equal(t, status, condition.Status, "unexpected condition status")
	validReasons := sets.NewString(
		cmapi.CertificateRequestReasonPending,
		cmapi.CertificateRequestReasonFailed,
		cmapi.CertificateRequestReasonIssued,
	)
	assert.Contains(t, validReasons, reason, "unexpected condition reason")
	assert.Equal(t, reason, condition.Reason, "unexpected condition reason")
}
