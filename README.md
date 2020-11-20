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

### Initialise a Kubebuilder project

```
kubebuilder init  --domain example.com --owner 'The cert-manager Authors'
```

This will create multiple directories and files containing a Makefile and configuration for building and deploying your project.
Notably:
* `config/`: containing various `kustomize` configuration files.
* `Dockerfile`: which is used to statically compile the issuer and package it as a "distroless" Docker image.
* `main.go`: which is the issuer main entry point.

### Run the controller-manager

With all these tools in place and with the project initialised you should now be able to run the issuer for the first time.


```
make run
```

This will compile and run the issuer locally and it will connect to the test cluster and log some startup messages.
We will add more to it in the next steps.


### Creating Issuer and ClusterIssuer custom resources

An [External Issuer] must implement two custom resources for compatibility with cert-manager: `Issuer` and `ClusterIssuer`

NOTE: It is important to understand the [Concept of Issuers] before proceeding.

We create the custom resource definitions (CRDs) using `kubebuilder` as follows:

```
kubebuilder create api --group sample-issuer --kind Issuer --version v1alpha1
```

```
kubebuilder create api --group sample-issuer --kind ClusterIssuer --version v1alpha1 --namespaced=false
```

NOTE: You will be prompted to create APIs and controllers. Answer `y` to all.

The `group` is the name given to a collection of custom resource APIs, and
the `kind` is the name of an individual resource in that group, and
the `version` allows you to create multiple versions of your APIs as the evolve,
whilst providing backwards compatibility for clients that still use older API versions.

These commands will have created some boilerplate files and directories: `api/` and `controllers/`,
which we now need to edit as follows:

* `api/v1alpha1/{cluster}issuer_types.go`:
   Add [Kubebuilder CRD Markers](https://book.kubebuilder.io/reference/markers/crd.html) to allow modification of IssuerStatus
   as a [Status Subresource](https://book-v1.book.kubebuilder.io/basics/status_subresource.html): `// +kubebuilder:subresource:status`

* `api/v1alpha1/clusterissuer_types.go`:
   Remove the `ClusterIssuerSpec` and `ClusterIssuerStatus` and replace them with `IssuerSpec` and `IssuerStatus`.
   This is because both types of issuers share the same configuration and status reporting.

* `controllers/{cluster}issuer_controller.go`:
   Edit the [Kubebuilder RBAC Markers](https://book.kubebuilder.io/reference/markers/rbac.html).
   The controller should not have write permission to the Issuer or ClusterIssuer.
   It should only be permitted to modify the Status subresource.

* `api/v1alpha1/{cluster}issuer_types.go`:
   And finally, remove any placeholder comments from the API files.

After modifying [Kubebuilder Markers] and API source files you should always regenerate all generated code and configuration,
as follows:

```
make generate manifests
```

You should see a number of new and modified files, reflecting the changes you made to the API source files and to the markers.

## Links

[External Issuer]: https://cert-manager.io/docs/contributing/external-issuers
[cert-manager Concepts Documentation]: https://cert-manager.io/docs/concepts
[Kubebuilder Book]: https://book.kubebuilder.io
[Kubebuilder Markers]: https://book.kubebuilder.io/reference/markers.html
