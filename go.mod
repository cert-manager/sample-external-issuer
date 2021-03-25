module github.com/cert-manager/sample-external-issuer

go 1.15

require (
	github.com/go-logr/logr v0.3.0
	github.com/jetstack/cert-manager v1.0.4
	github.com/stretchr/testify v1.6.1
	k8s.io/api v0.20.2
	k8s.io/apimachinery v0.20.2
	k8s.io/client-go v0.20.2
	sigs.k8s.io/controller-runtime v0.8.3
)
