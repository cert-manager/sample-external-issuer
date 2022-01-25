module github.com/cert-manager/sample-external-issuer

go 1.17

require (
	github.com/go-logr/logr v1.2.0
	github.com/jetstack/cert-manager v1.7.0-beta.0
	github.com/stretchr/testify v1.7.0
	k8s.io/api v0.23.1
	k8s.io/apimachinery v0.23.1
	k8s.io/client-go v0.23.1
	sigs.k8s.io/controller-runtime v0.11.0
)
