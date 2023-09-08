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
	"crypto/x509"
	"encoding/pem"
	"time"

	capi "k8s.io/api/certificates/v1beta1"

	sampleissuerapi "github.com/cert-manager/sample-external-issuer/api/v1alpha1"
	"github.com/cert-manager/sample-external-issuer/internal/controllers"
)

func ExampleHealthCheckerFromIssuerAndSecretData(*sampleissuerapi.IssuerSpec, map[string][]byte) (controllers.HealthChecker, error) {
	return &exampleSigner{}, nil
}

func ExampleSignerFromIssuerAndSecretData(*sampleissuerapi.IssuerSpec, map[string][]byte) (controllers.Signer, error) {
	return &exampleSigner{}, nil
}

type exampleSigner struct {
}

func (o *exampleSigner) Check() error {
	return nil
}

var (
	keyPEM = []byte(`
-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEAyyD731YDc88ZkdbtaQyi65adORoenBB0FvR6JLmuupm0bqhH
erNzHbFdKkPfES7NTkALGYId0AfA6Y9zpnPmfYO4BIDGQaVlk9A00p77PZovA6dQ
aEyHiTwSwH0/3hXGe0M32Lk30EUyhF6dU1/DFgoGMRtd2Tf+Z121fyyEB5AEtn1I
Xkwkb5/BXukxRH5jGjm+o1usE8CyKFFwnT+gtILKJ7DNzqpIkFQBe9wXr0z/nEFl
GjtR21iF1amdY6dNIG9dPw6IQl6Swz6zWUCvh9rIFKJqPfknwrtiD6s8d38H2Xv3
dNAQFM2cRKCz4iR5KVqUOFwyD9tOg2e6lMUXHwIDAQABAoIBAFIPpTF4ojRq+j18
wrSpsjfSxPmIn80UqJGNerrTeM9RwR7jRN1BGcRpHuYwPTHH4pE2NkW71ydvunOg
zGv2bqtOR00qaO2kUAEDIBPmvkEIxO2I7mb0Y90BM+Int2GVEnZBlZIsYWv2SI5J
Wu2PxlRlAFNeZu+WO2Su6t/RsBUNVSUOjFhbT2zTQtwinalD2pIE5WCrnvEFpAeK
bhAsL3Vht6clGDYk5INYxTLnbiLwSl7Dl38/q8/D+hJNFe7XlQ2X5cOrShvtUarP
Q1L3RlQTXq7kyx6PJ3tyQvQiBVSd7jb3bxUZwxfRC5sZ2dROVOSLI/NVSR7aooAn
De4yEeECgYEA7xnJ8Zbv5qP/3Vvuzq2264xGLleboKLAV8Br3AWy8+OTpUH2DI5n
exLR4FSLS4n48E9GmdY9FsgNYOGTlQDDuYYB1jDFQqGtKo4YkiYxLrC2o7JI8I0p
XRRiPAXtzY00bF1dw8vA689zEnf9XvovjPmLWUuUqjdXv7z+v8F3XPUCgYEA2XxU
zMqcy211vH1zz+PYy7JQAaRFZ7eoZH0vDOO4deFcPZp1LpjRzYbRI/EB9vAWAovt
FKVtZW5+rrJ/sd4ZB0/jkxahUj5r49ELicxFmWgBzzJRfQ/RZXpjh4S40Qz9zptG
e6HbbeVRiEA6ZKpOBZHG3wtSaeik9g2ZOlve10MCgYATlZEs8KgFxDkY8IbG9wOc
l4jIEvT0W2BVz7UF+JGH2IQnbReyP5fKROhb75DZRxvU0yl9QEcQrqIp5VApTD67
23YbDTObGZMNgUYR8n7kzCSpk9jVmzpgHWNOd03bIE3C8oLTnsTWi89pG9rtBKEQ
cwAu+DndF1tgoSJcooQcYQKBgFmacPGi9HCXm29aHHHlRLe/sljKzlGKCFXGgbEE
zUW74J382hSln6LWzanKLO4JQngwIDBma6jjmkvtfNDSWWt6zZ8XLsXMs/S7ds6C
G5a1lDFCYPJupu3xO7pkwyRV/ue1b5eWOuqPFUVWePhqdhSzV8UjTAQYdoZtWdkC
atAzAoGBAODs3qUjU25kqnUEJS8P3llrNZQunMRWhD1eVovyVgsJkRTNaF2nKrjW
EM9qDV5Wq1hIKW93f9lVKhbj7dMkRQrHNz1ToAuvKFsWnL7mW89FdbHqZpNfomQU
Cs+nfLWBLLXpABSikgdzD27/w436jk5nqRr7/Jh9WDEO7roHOneY
-----END RSA PRIVATE KEY-----
`)
	certPEM = []byte(`
-----BEGIN CERTIFICATE-----
MIICyDCCAbCgAwIBAgIBADANBgkqhkiG9w0BAQsFADAVMRMwEQYDVQQDEwprdWJl
cm5ldGVzMB4XDTIwMDQwNzEzMzgxMFoXDTMwMDQwNTEzMzgxMFowFTETMBEGA1UE
AxMKa3ViZXJuZXRlczCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAMsg
+99WA3PPGZHW7WkMouuWnTkaHpwQdBb0eiS5rrqZtG6oR3qzcx2xXSpD3xEuzU5A
CxmCHdAHwOmPc6Zz5n2DuASAxkGlZZPQNNKe+z2aLwOnUGhMh4k8EsB9P94VxntD
N9i5N9BFMoRenVNfwxYKBjEbXdk3/mddtX8shAeQBLZ9SF5MJG+fwV7pMUR+Yxo5
vqNbrBPAsihRcJ0/oLSCyiewzc6qSJBUAXvcF69M/5xBZRo7UdtYhdWpnWOnTSBv
XT8OiEJeksM+s1lAr4fayBSiaj35J8K7Yg+rPHd/B9l793TQEBTNnESgs+IkeSla
lDhcMg/bToNnupTFFx8CAwEAAaMjMCEwDgYDVR0PAQH/BAQDAgKkMA8GA1UdEwEB
/wQFMAMBAf8wDQYJKoZIhvcNAQELBQADggEBAD7h2TrMPlAl22BQija0EMKEokWL
2ZNL4+l8F6go/epU1QQYS6PmWBqySvhK2aek65LaWaowLUzUey70k/f9oJvGBo6W
AJvBJ6eBVSuiEid6FW7gj/+gAKEC2vd78zs3QmusCCISO6h1dDTQS0swyS/HBBVx
1T33EWRlxdF42vTHMzO8bEUJlokBxvWvkzkjpAfuJQ1MuVTfkbuJFeIVER2xtLL7
ai85UCdnGfwgzKGx1URCjcE67oKuUQDiulXk4bnQT2Zbj0IcEHcB4XAeuYuYJdB4
YcXl/jdU/2nHdY6r7m6xIapxs0hdDMF/lML2SszUIukZw73NJp3x7L9enCY=
-----END CERTIFICATE-----
`)
	duration = time.Hour * 24 * 365
)

func (o *exampleSigner) Sign(certTemplate *x509.Certificate) ([]byte, error) {
	key, err := parseKey(keyPEM)
	if err != nil {
		return nil, err
	}
	cert, err := parseCert(certPEM)
	if err != nil {
		return nil, err
	}

	ca := &CertificateAuthority{
		Certificate: cert,
		PrivateKey:  key,
		Backdate:    5 * time.Minute,
	}

	crtDER, err := ca.Sign(certTemplate, PermissiveSigningPolicy{
		TTL: duration,
		Usages: []capi.KeyUsage{
			capi.UsageServerAuth,
		},
	})
	if err != nil {
		return nil, err
	}

	return pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: crtDER,
	}), nil
}
