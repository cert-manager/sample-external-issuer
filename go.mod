module github.com/cert-manager/sample-external-issuer

go 1.13

require (
	github.com/go-logr/zapr v0.2.0 // indirect
	github.com/go-logr/logr v0.2.1-0.20200730175230-ee2de8da5be6
	github.com/jetstack/cert-manager v1.0.4
	github.com/onsi/ginkgo v1.12.1
	github.com/onsi/gomega v1.10.1
	gonum.org/v1/netlib v0.0.0-20190331212654-76723241ea4e // indirect
	k8s.io/apimachinery v0.19.0
	k8s.io/client-go v0.19.0
	sigs.k8s.io/controller-runtime v0.6.2
	sigs.k8s.io/structured-merge-diff v1.0.1-0.20191108220359-b1b620dd3f06 // indirect
)
