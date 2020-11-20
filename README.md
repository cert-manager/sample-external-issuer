# sample-external-issuer

This is an example of an [External Issuer] for cert-manager.

## How to write your own external issuer

If you are writing an external issuer you may find it helpful to review the code and the commits in this repository
and to follow the steps below,
replacing references to `sample-external-issuer` with the name of your project.

### Prerequisites

You will need the following command line tools installed on your PATH:

* [Git](https://git-scm.com/)
* [Golang v1.13+](https://golang.org/)
* [Docker v17.03+](https://docs.docker.com/install/)
* [Kind v0.9.0+](https://kind.sigs.k8s.io/docs/user/quick-start/)
* [Kubectl v1.11.3+](https://kubernetes.io/docs/tasks/tools/install-kubectl/)
* [Kubebuilder v2.3.1+](https://book.kubebuilder.io/quick-start.html#installation)
* [Kustomize v3.8.1+](https://kustomize.io/)

You may also want to read: the [Kubebuilder Book] and the [cert-manager Concepts Documentation].

### Create a test cluster

We will need a Kubernetes cluster on which to test our issuer and we can quickly create one using `kind`.

```
kind create cluster
```

This will update your KUBECONFIG file with the URL and credentials for the test cluster.
You can explore it using `kubectl`

```
kubectl get nodes
```

This should show you details of a single node.

### Create a repository

We need a Git repository to track changes to the issuer code.
You can start by creating a repository on GitHub or you can create it locally.

```
mkdir sample-external-issuer
cd sample-external-issuer
git init
```

### Initialise a Go mod file

A Go project needs a `go.mod` file which defines the root name of your Go packages.

```
go mod init github.com/cert-manager/sample-external-issuer
```

## Links

[External Issuer]: https://cert-manager.io/docs/contributing/external-issuers
[cert-manager Concepts Documentation]: https://cert-manager.io/docs/concepts
[Kubebuilder Book]: https://book.kubebuilder.io
