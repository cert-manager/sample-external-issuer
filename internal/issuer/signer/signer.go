package signer

import sampleissuerapi "github.com/cert-manager/sample-external-issuer/api/v1alpha1"

type HealthChecker interface {
	Check() error
}

type HealthCheckerBuilder func(*sampleissuerapi.IssuerSpec, map[string][]byte) (HealthChecker, error)
