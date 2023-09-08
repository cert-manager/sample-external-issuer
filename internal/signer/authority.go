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

package signer

import (
	"crypto"
	"crypto/rand"
	"crypto/x509"
	"fmt"
	"time"
)

// CertificateAuthority implements a certificate authority that supports policy
// based signing. It's used by the signing controller.
type CertificateAuthority struct {
	// RawCert is an optional field to determine if signing cert/key pairs have changed
	RawCert []byte
	// RawKey is an optional field to determine if signing cert/key pairs have changed
	RawKey []byte

	Certificate *x509.Certificate
	PrivateKey  crypto.Signer
	Backdate    time.Duration
	Now         func() time.Time
}

// Sign signs a certificate request, applying a SigningPolicy and returns a DER
// encoded x509 certificate.
func (ca *CertificateAuthority) Sign(certTemplate *x509.Certificate, policy SigningPolicy) ([]byte, error) {
	now := time.Now()
	if ca.Now != nil {
		now = ca.Now()
	}

	nbf := now.Add(-ca.Backdate)
	if !nbf.Before(ca.Certificate.NotAfter) {
		return nil, fmt.Errorf("the signer has expired: NotAfter=%v", ca.Certificate.NotAfter)
	}

	if err := policy.apply(certTemplate); err != nil {
		return nil, err
	}

	if !certTemplate.NotAfter.Before(ca.Certificate.NotAfter) {
		certTemplate.NotAfter = ca.Certificate.NotAfter
	}
	if !now.Before(ca.Certificate.NotAfter) {
		return nil, fmt.Errorf("refusing to sign a certificate that expired in the past")
	}

	der, err := x509.CreateCertificate(rand.Reader, certTemplate, ca.Certificate, certTemplate.PublicKey, ca.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign certificate: %v", err)
	}

	return der, nil
}
