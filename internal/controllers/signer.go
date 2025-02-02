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

package controllers

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"time"

	"github.com/cert-manager/cert-manager/pkg/util/pki"
	issuerapi "github.com/cert-manager/issuer-lib/api/v1alpha1"
	"github.com/cert-manager/issuer-lib/controllers"
	"github.com/cert-manager/issuer-lib/controllers/signer"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sampleissuerapi "github.com/cert-manager/sample-external-issuer/api/v1alpha1"
)

var (
	errGetAuthSecret        = errors.New("failed to get Secret containing Issuer credentials")
	errHealthCheckerBuilder = errors.New("failed to build the healthchecker")
	errHealthCheckerCheck   = errors.New("healthcheck failed")

	errSignerBuilder = errors.New("failed to build the signer")
	errSignerSign    = errors.New("failed to sign")
)

type HealthChecker interface {
	Check() error
}

type HealthCheckerBuilder func(*sampleissuerapi.IssuerSpec, map[string][]byte) (HealthChecker, error)

type Signer interface {
	Sign(*x509.Certificate) ([]byte, error)
}

type SignerBuilder func(*sampleissuerapi.IssuerSpec, map[string][]byte) (Signer, error)

type Issuer struct {
	HealthCheckerBuilder     HealthCheckerBuilder
	SignerBuilder            SignerBuilder
	ClusterResourceNamespace string

	client client.Client
}

// +kubebuilder:rbac:groups=sample-issuer.example.com,resources=sampleclusterissuers;sampleissuers,verbs=get;list;watch
// +kubebuilder:rbac:groups=sample-issuer.example.com,resources=sampleclusterissuers/status;sampleissuers/status,verbs=patch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups=cert-manager.io,resources=certificaterequests,verbs=get;list;watch
// +kubebuilder:rbac:groups=cert-manager.io,resources=certificaterequests/status,verbs=patch
// +kubebuilder:rbac:groups=certificates.k8s.io,resources=certificatesigningrequests,verbs=get;list;watch
// +kubebuilder:rbac:groups=certificates.k8s.io,resources=certificatesigningrequests/status,verbs=patch
// +kubebuilder:rbac:groups=certificates.k8s.io,resources=signers,verbs=sign,resourceNames=sampleclusterissuers.sample-issuer.example.com/*;sampleissuers.sample-issuer.example.com/*

func (s Issuer) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	s.client = mgr.GetClient()

	return (&controllers.CombinedController{
		IssuerTypes:        []issuerapi.Issuer{&sampleissuerapi.SampleIssuer{}},
		ClusterIssuerTypes: []issuerapi.Issuer{&sampleissuerapi.SampleClusterIssuer{}},

		FieldOwner:       "sampleissuer.cert-manager.io",
		MaxRetryDuration: 1 * time.Minute,

		Sign:          s.Sign,
		Check:         s.Check,
		EventRecorder: mgr.GetEventRecorderFor("sampleissuer.cert-manager.io"),
	}).SetupWithManager(ctx, mgr)
}

func (o *Issuer) getIssuerDetails(issuerObject issuerapi.Issuer) (*sampleissuerapi.IssuerSpec, string, error) {
	switch t := issuerObject.(type) {
	case *sampleissuerapi.SampleIssuer:
		return &t.Spec, issuerObject.GetNamespace(), nil
	case *sampleissuerapi.SampleClusterIssuer:
		return &t.Spec, o.ClusterResourceNamespace, nil
	default:
		// A permanent error will cause the Issuer to not retry until the
		// Issuer is updated.
		return nil, "", signer.PermanentError{
			Err: fmt.Errorf("unexpected issuer type: %t", issuerObject),
		}
	}
}

func (o *Issuer) getSecretData(ctx context.Context, issuerSpec *sampleissuerapi.IssuerSpec, namespace string) (map[string][]byte, error) {
	secretName := types.NamespacedName{
		Namespace: namespace,
		Name:      issuerSpec.AuthSecretName,
	}

	var secret corev1.Secret
	if err := o.client.Get(ctx, secretName, &secret); err != nil {
		return nil, fmt.Errorf("%w, secret name: %s, reason: %v", errGetAuthSecret, secretName, err)
	}

	checker, err := o.HealthCheckerBuilder(issuerSpec, secret.Data)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errHealthCheckerBuilder, err)
	}

	if err := checker.Check(); err != nil {
		return nil, fmt.Errorf("%w: %v", errHealthCheckerCheck, err)
	}

	return secret.Data, nil
}

// Check checks that the CA it is available. Certificate requests will not be
// processed until this check passes.
func (o *Issuer) Check(ctx context.Context, issuerObject issuerapi.Issuer) error {
	issuerSpec, namespace, err := o.getIssuerDetails(issuerObject)
	if err != nil {
		return err
	}

	_, err = o.getSecretData(ctx, issuerSpec, namespace)
	return err
}

// Sign returns a signed certificate for the supplied CertificateRequestObject (a cert-manager CertificateRequest resource or
// a kubernetes CertificateSigningRequest resource). The CertificateRequestObject contains a GetRequest method that returns
// a certificate template that can be used as a starting point for the generated certificate.
// The Sign method should return a PEMBundle containing the signed certificate and any intermediate certificates (see the PEMBundle docs for more information).
// If the Sign method returns an error, the issuance will be retried until the MaxRetryDuration is reached.
// Special errors and cases can be found in the issuer-lib README: https://github.com/cert-manager/issuer-lib/tree/main?tab=readme-ov-file#how-it-works
func (o *Issuer) Sign(ctx context.Context, cr signer.CertificateRequestObject, issuerObject issuerapi.Issuer) (signer.PEMBundle, error) {
	issuerSpec, namespace, err := o.getIssuerDetails(issuerObject)
	if err != nil {
		// Returning an IssuerError will change the status of the Issuer to Failed too.
		return signer.PEMBundle{}, signer.IssuerError{
			Err: err,
		}
	}

	secretData, err := o.getSecretData(ctx, issuerSpec, namespace)
	if err != nil {
		// Returning an IssuerError will change the status of the Issuer to Failed too.
		return signer.PEMBundle{}, signer.IssuerError{
			Err: err,
		}
	}

	certTemplate, _, _, err := cr.GetRequest()
	if err != nil {
		return signer.PEMBundle{}, err
	}

	signerObj, err := o.SignerBuilder(issuerSpec, secretData)
	if err != nil {
		return signer.PEMBundle{}, fmt.Errorf("%w: %v", errSignerBuilder, err)
	}

	signed, err := signerObj.Sign(certTemplate)
	if err != nil {
		return signer.PEMBundle{}, fmt.Errorf("%w: %v", errSignerSign, err)
	}

	bundle, err := pki.ParseSingleCertificateChainPEM(signed)
	if err != nil {
		return signer.PEMBundle{}, err
	}

	return signer.PEMBundle(bundle), nil
}
