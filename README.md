# sample-external-issuer

This is an example of an [External Issuer] for cert-manager.

## Demo

You can run the sample-external-issuer on a local cluster with this command:

```
make IMG=controller:0.0.0 kind-cluster deploy-cert-manager docker-build kind-load deploy e2e
```

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

### Creating a CertificateRequest controller

We now need a controller to handle [cert-manager CertificateRequest resources](https://cert-manager.io/docs/concepts/certificaterequest/).
This controller will watch for `CertificateRequest` resources and attempt to sign their attached x509 certificate signing requests (CSR).
Your external issuer will likely interact with your certificate authority using a REST API,
so this is the controller where we will eventually need to instantiate an HTTP client,
directly or via an API wrapper library.
And we will need to get the configuration and credentials for this from the `Issuer` or `ClusterIssuer` referred to by the `CertificateRequest`.

Start by copying the `controllers/issuer_controller.go` to `controllers/certificaterequest_controller.go`
and modifying its code and comments to refer to `CertificateRequest` rather than `Issuer`.

NOTE: You will need to import the [cert-manager V1 API](https://cert-manager.io/docs/reference/api-docs/)
and this in turn will pull in a number of transitive dependencies of cert-manager which we will deal with shortly.

```
import (
...
    cmapi "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"
...
)

```

Next edit `main.go` and register the new `CertificateRequestReconciler` in the same way that the `IssuerReconciler` is registered.
You will also need to add the cert-manager API types to the `Scheme`:

```

func init() {
...
    _ = cmapi.AddToScheme(scheme)
...
}
```

The `Scheme` is how the controller-runtime client knows how to decode and encode the API resources from the Kubernetes API server.
So it is important to add all the API types that are used in your issuer.

Finally run `make generate manifests` again to update all the generated code.

NOTE: You may encounter a dependency conflict between the version of controller-runtime used by cert-manager and the version installed by Kubebuilder.
We have to use the cert-manager version and that in turn requires a newer version of the `zapr` logging library.
Add the following line to `go.mod`:

```
github.com/go-logr/zapr v0.2.0 // indirect
```

#### Get the CertificateRequest

The `CertificateRequestReconciler` is triggered by changes to any `CertificateRequest` resource in the cluster.
The `Reconcile` function is called with the name of the object that changed, and
the first thing we need to do is to `GET` the complete object from the Kubernetes API server.

The `Reconcile` function may occasionally be triggered with the names of deleted resources,
so we have to handle that case gracefully.

Explore the unit-tests in `controllers/certificaterequest_test.go` and try running the tests and seeing them fail
*before* updating the controller code.
These are table-driven tests  which will execute the `Reconcile` function many times,
with inputs supplied as function arguments and also certain inputs that come from the parent object.
It also uses a `FakeClient` which can be primed with a collection of Kubernetes API objects.
The test will check the output of the `Reconcile` function for errors and later we will make it check the changes that have been made to the supplied API objects.

In the implementation we are careful to `return Result{}, nil` when the `CertificateRequest` is not found.
This tells controller-runtime *do not retry*.
Other error types are assumed to be temporary errors and are returned.

NOTE: If you return an `error`, controller-runtime will retry with an increasing backoff,
so it is very important to distinguish between temporary and permanent errors.

#### Ignore foreign CertificateRequest

We only want to reconcile `CertificateRequest` resources that are configured for our external issuer.
So the next piece of controller logic attempts to exit early if `CertificateRequest.Spec.IssuerRef` does not refer to our particular `Issuer` or `ClusterIssuer` types.

As before explore the unit-tests and see how we modify the success case, where the `IssuerRef` does refer to one of our types.
And then we add some error cases where the `Group` or the `Kind` are unrecognised.

Also note how in the implementation we use the `Scheme.New`  method to verify the `Kind`.
This later will allow us to easily handle both `Issuer` and `ClusterIssuer` references.

If there is a mismatch in the `IssuerRef` we ignore the `CertificateRequest`.

#### Set the CertificateRequest Ready condition

The [External Issuer] documentation says the following:

 It is important to update the condition status of the `CertificateRequest` to a ready state,
 as this is what is used to signal to higher order controllers, such as the Certificate controller, that the resource is ready to be consumed.
 Conversely, if the `CertificateRequest` fails, it is important to mark the resource as such, as this will also be used to signal to higher order controllers.

So now we need to ensure that our issuer always sets one of the [strongly defined conditions](https://cert-manager.io/docs/concepts/certificaterequest/#conditions)
on all the `CertificateRequest` referring to our `Group`.

Study the changes and the additional tests.
Note that the first thing we do is check whether the `Ready` condition is already `true` in which case we can exit early.
Note also the use of a `defer` function which ensures that the condition is always set and that it is always set to false if an error has occurred.

#### Get the Issuer or ClusterIssuer

The `Issuer` or `ClusterIssuer` for the `CertificateRequest` will usually contain configuration that you will need to connect to your certificate authority API.
It may also contain a reference to a `Secret` containing credentials which you will use to authenticate with with your certificate authority API.

So now we attempt to `GET` the `Issuer` or `ClusterIssuer` and to do this we need to derive a resource name.
An `Issuer` has both a name and a namespace.
A `ClusterIssuer` is [cluster scoped](https://kubernetes.io/docs/concepts/overview/working-with-objects/namespaces/#not-all-objects-are-in-a-namespace) and does not have a namespace.
So we check which type we have in order to derive the correct name.

If the `GET` request fails, we return the error so as to trigger the retry-with-backoff behaviour (described above).
This allows for situations where the `CertificateRequest` may have been created before the corresponding `Issuer`.
This case is demonstrated in the unit tests.

#### Check the Issuer or ClusterIssuer Ready condition

An issuer will often perform some initialisation when it is first created,
for example it might create a private key and CA certificate and store those somewhere,
and such operations take time.
So we give the `Issuer` and `ClusterIssuer` resources their own Ready conditions which the `IssuerReconciler` can set to signal that the initialization is complete and that the issuer is ready and healthy.

The `CertificateRequestReconciler` should then wait for the `Issuer` to be Ready before progressing further.


#### Get the Issuer or ClusterIssuer credentials from a Secret

The API for your CA may require some configuration and credentials and the obvious place to store these is in a Kubernetes `Secret`.
We extend the `IssuerSpec` to include a `URL` field and a `AuthSecretName`, which is the name of a `Secret`.
As usual run `make generate manifests` after modifying the API source files:

```
make generate manifests
```

NOTE: The namespace of that Secret is deliberately not specified here,
because that would breach a security boundary and potentially allow someone who has permission to create `Issuer` resources,
to make the controller access secrets in another namespace which that person would not normally have access to.

For this reason, the Secret for an Issuer MUST be in the same namespace as the Issuer.
The Secret for a ClusterIssuer MUST be in a namespace defined by cluster administrator,
but that is a little more complicated and for now we will concentrate on Issuer Secrets.

Both the `IssuerReconciler` and the `CertificateRequestReconciler` are updated to `GET` the `Secret` referred to by the `Issuer`.

Add a new [Kubebuilder RBAC Marker](https://book.kubebuilder.io/reference/markers/rbac.html) to both controllers,
permitting them read-only access to `Secret` resources.

```
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
```

Then run `make manifests` to regenerate the RBAC configuration in `config/`.

Add the `corev1` types to the `Scheme` in the unit-tests.

NOTE: It has already been added to the `main.go` `Scheme` as part of the `clientgoscheme`.

Write a test to check that if the `GET` `Secret` operation fails,
the error is returned and triggers a retry-with-backoff.
This important because the `Secret` may not exist at the time the `Issuer` or `CertificateRequest` is created.

NOTE: Ideally, we would `WATCH` for the particular `Secret` and trigger the reconciliation when it becomes available.
And that may be a future enhancement to this project.

In the case of the `CertificateRequestReconciler` we need to deal with both `Issuer` and `ClusterIssuer` types,
so we modify the `issuerutil` function to allow us to extract an `IssuerSpec` from either of those types.


#### Issuer health checks

An issuer that connects to a certificate authority API may want to perform periodic health checks and sanity checks,
to ensure that the API server is responding and if not,
to set update the `Ready` condition of the `Issuer` to false, and log a meaningful error message with the condition.
This will give early warning of problems with the configuration or with the API,
rather than waiting a for `CertificateRequest` to fail before being alerted to the problem.

Start with an `Interface` describing the health check operation.
For example:

```
type HealthChecker interface {
    Check() error
}
```

We don't need to implement it yet,
we just need to plug that into the `IssuerReconciler` and add a fake implementation to the tests
so that we can check how the reconciler behaves when the health checks fail.

And since we can't know the `Issuer` configuration or credentials until we begin reconciling,
we need to describe a constructor function type which can build a `HealthChecker` from an `IssuerSpec`  and some `Secret` data.

```
 type HealthCheckerBuilder func(*sampleissuerapi.IssuerSpec, map[string][]byte) (HealthChecker, error)
```

This will be supplied as an `IssuerReconciler` field, and can be easily faked in the unit-tests.

And finally, since we want the health checks to be performed periodically,
we need to make controller-runtime retry reconciling regularly, even when the current reconcile succeeds.
We do this by setting the `Result.RequeueAfter` field of the returned result.


### Sign the CertificateRequest

Now we turn back to the `CertificateRequestReconciler` and think about how we want it to handle the certificate signing request (CSR).

Let's once again assume that the issuer will connect to a certificate authority API.
We extend the `signer` package with a new simple `Interface` and a factory function definition
(for the same reasons given about in the Issuer Health Checks section):

```
type Signer interface {
    Sign([]byte) ([]byte, error)
}

type SignerBuilder func(*sampleissuerapi.IssuerSpec, map[string][]byte) (Signer, error)
```

We don't need to implement it yet,
we just need to plug that into the `CertificateRequestReconciler` and add a fake implementation to the tests
so that we can check how the reconciler behaves when `Sign` fails.

If `Sign` succeeds it returns the bytes of a signed certificate which we then use as the value for `CertificateRequest.Status.Certificate`.
And we add a unit-test for this case.

In the unit-tests, we can use a simple byte string for the certificate, but in E2E later we will use real ceritificate signing requests and real certificates.

#### An example signer

For the purposes of this example external issuer,
we will implement an `exampleSigner` which implements both the `HealthChecker` and the `Signer` interfaces, and
which signs the CSR using a static in-memory CA certificate.

In `internal/issuer/signer/signer.go` you will see that we:
decode the supplied CSR bytes,
and then sign the certificate using some libraries that were copied from the Kubernetes project.
This simple implementation is just sufficient to allow us (later) to perform some E2E tests with cert-manager.

In your external issuer, this is where you will plug in your CA client library,
or where you will instantiate an HTTP client and connect to your API.

Notice also that we add two concrete factory functions which are supplied to the `IssuerReconciler` and `CertificateRequestReconciler` in `main.go`.

#### What about the ClusterIssuerReconciler?

We have so far abandoned development of the `ClusterIssuerReconciler`, and that's because we want to re-use the `IssuerReconciler` rather than duplicating everything.

So here we delete the skaffolded `controllers/clusterissuer_controller.go` and update the `issuer_controller.go` to handle both types.

As well as juggling the code to handle both types, we:
aggregate the Kubebuilder RBAC annotations, and
add a new command line flag which allows us to set a `--cluster-resource-namespace`.

The `--cluster-resource-namespace` is the namespace where the issuer will look for `Secret` resources referred to by a `ClusterIssuer`,
since `ClusterIssuer` is cluster-scoped.
The default value of the flag is the namespace where the issuer is running in the cluster.

#### End-to-end tests

Now our issuer is almost feature complete and it should be possible to write an end-to-end test that
deploys a cert-manager `Certificate`
referring to an external `Issuer` and check that a signed `Certificate` is saved to the expected secret.

We can make such a test easier by tidying up the `Makefile` and adding some new targets
which will help create a test cluster and to help install cert-manager.

We can write a simple end-to-end test which deploys a `Certificate` manifest and waits for it to be ready.

```
kubectl apply --filename config/samples
kubectl wait --for=condition=Ready --timeout=5s issuers.sample-issuer.example.com issuer-sample
kubectl wait --for=condition=Ready --timeout=5s  certificates.cert-manager.io certificate-by-issuer
```

You can of course write more complete tests than this,
but this is a good start and demonstrates that the issuer is doing what we hoped it would do.


## Links

[External Issuer]: https://cert-manager.io/docs/contributing/external-issuers
[cert-manager Concepts Documentation]: https://cert-manager.io/docs/concepts
[Kubebuilder Book]: https://book.kubebuilder.io
[Kubebuilder Markers]: https://book.kubebuilder.io/reference/markers.html
