<p align="center">
  <img src="https://raw.githubusercontent.com/cert-manager/cert-manager/d53c0b9270f8cd90d908460d69502694e1838f5f/logo/logo-small.png" height="256" width="256" alt="cert-manager project logo" />
</p>

# sample-external-issuer

External issuers extend [cert-manager](https://cert-manager.io/) to issue certificates using APIs and services
which aren't built into the cert-manager core.

This repository provides an example of an [External Issuer] built using the [issuer-lib] library.

## Install

```console
kubectl apply -f https://github.com/cert-manager/sample-external-issuer/releases/download/v0.1.0/install.yaml
```

## Demo

You can run the sample-external-issuer on a local cluster with this command:

```console
make kind-cluster deploy-cert-manager docker-build kind-load deploy e2e
```

## How to write your own external issuer

If you are writing an external issuer you may find it helpful to review the sample code in this repository
and to follow the steps below, replacing references to `sample-external-issuer` with the name of your project.

### Prerequisites

You will need the following command line tools installed on your PATH:

* [Git](https://git-scm.com/)
* [Golang v1.20+](https://golang.org/)
* [Docker v17.03+](https://docs.docker.com/install/)
* [Kind v0.18.0+](https://kind.sigs.k8s.io/docs/user/quick-start/)
* [Kubectl v1.26.3+](https://kubernetes.io/docs/tasks/tools/install-kubectl/)
* [Kubebuilder v3.9.1+](https://book.kubebuilder.io/quick-start.html#installation)
* [Kustomize v3.8.1+](https://kustomize.io/)

You may also want to read: the [Kubebuilder Book] and the [cert-manager Concepts Documentation] for further background
information.

### Create a test cluster

We will need a Kubernetes cluster on which to test our issuer and we can quickly create one using `kind`.

```console
kind create cluster
```

This will update your KUBECONFIG file with the URL and credentials for the test cluster.
You can explore it using `kubectl`

```console
kubectl get nodes
```

This should show you details of a single node.

### Copy the sample-external-issuer code

We need a Git repository to track changes to the issuer code.
You can start by creating a repository on GitHub or you can create it locally.

```console
mkdir my-external-issuer
cd my-external-issuer
git clone https://github.com/cert-manager/sample-external-issuer.git .
git remote rm origin
git remote add origin https://github.com/<username>/my-external-issuer.git
```

### Run the controller-manager

With all these tools in place and with the project initialised you should now be able to run the issuer for the first time.

```console
make run
```

This will compile and run the issuer locally and it will connect to the test cluster and log some startup messages.
We will add more to it in the next steps.

### Creating MyIssuer and MyClusterIssuer custom resources

An [External Issuer] must implement two custom resources for compatibility with cert-manager: `MyIssuer` and `MyClusterIssuer`

NOTE: It is important to understand the [Concept of Issuers] before proceeding.

The `MyIssuer` and `MyClusterIssuer` custom resources can be defined in the `api/v1alpha1` directory.
Use the `SampleIssuer` and `SampleClusterIssuer` definitions as a starting point.

Additionally, the group, version and kind of the custom resources must be customised to be unique to your issuer:

* `group` is the name given to a collection of custom resource APIs
* `kind` is the name of an individual resource in that group
* `version` allows you to create multiple versions of your APIs as they evolve, whilst providing backwards compatibility for clients using older API versions

After modifying the API source files you should always regenerate all generated code and configuration,
as follows:

```console
make generate manifests
```

You should see a number of new and modified files, reflecting the changes you made to the API source files.

#### Issuer health checks

An issuer that connects to a certificate authority API may want to perform periodic health checks and sanity checks,
to ensure that the API server is responding and if not,
to set update the `Ready` condition of the `Issuer` to false, and log a meaningful error message with the condition.
This will give early warning of problems with the configuration or with the API,
rather than waiting a for `CertificateRequest` to fail before being alerted to the problem.
Additionally, this implements the "Circuit Breaker" pattern, it makes all the `CertificateRequest` wait until the `Issuer`
is healthy again.

The health check is implemented in the `Check` function in the `./internal/controllers/signer.go` file.

TODO: issuer-lib does not yet support performing the health checks periodically.
There should be some return value for the `Check` function so we can make controller-runtime retry reconciling regularly, even when the current reconcile succeeds.

See [the issuer-lib README](https://github.com/cert-manager/issuer-lib?tab=readme-ov-file#how-it-works) for more information.

### Sign the cert-manager CertificateRequest resources and kubernetes CertificateSigningRequest resources

The `Sign` function in the `./internal/controllers/signer.go` file is used by the CertificateRequest and CertificateSigningRequest reconcilers
to create signed x509 certificates for the provided x509 certificate signing requests.

If `Sign` succeeds it returns the bytes of a signed certificate which we then use as the value for `CertificateRequest.Status.Certificate`.
If it returns a normal error, the Sign function will be retried as long as we have not spent more than the configured MaxRetryDuration after the certificate request was created.

See [the issuer-lib README](https://github.com/cert-manager/issuer-lib?tab=readme-ov-file#how-it-works) for more information.

#### Get the Issuer or ClusterIssuer credentials from a Secret

The API for your CA may require some configuration and credentials and the obvious place to store these is in a Kubernetes `Secret`.
We extend the `IssuerSpec` to include a `URL` field and a `AuthSecretName`, which is the name of a `Secret`.
As usual run `make generate manifests` after modifying the API source files:

```console
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

```go
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

### Logging and Events

We want to make it easy to debug problems with the issuer,
so in addition to setting Conditions on the Issuer, ClusterIssuer and CertificateRequest,
we can provide more feedback to the user by logging Kubernetes Events.
You may want to read more about [Application Introspection and Debugging][] before continuing.

[Application Introspection and Debugging]: https://kubernetes.io/docs/tasks/debug-application-cluster/debug-application-introspection/

Kubernetes Events are saved to the API server on a best-effort basis,
they are (usually) associated with some other Kubernetes resource,
and they are temporary; old Events are periodically purged from the API server.
This allows tools such as `kubectl describe <resource-kind> <resource-name>` to show not only the resource details,
but also a table of the recent events associated with that resource.

The aim is to produce helpful debug output that looks like this:

```
$ kubectl describe clusterissuers.sample-issuer.example.com clusterissuer-sample
...
    Type:                  Ready
Events:
  Type     Reason            Age                From                    Message
  ----     ------            ----               ----                    -------
  Normal   IssuerReconciler  13s                sample-external-issuer  First seen
  Warning  IssuerReconciler  13s (x3 over 13s)  sample-external-issuer  Temporary error. Retrying: failed to get Secret containing Issuer credentials, secret name: sample-external-issuer-system/clusterissuer-sample-credentials, reason: Secret "clusterissuer-sample-credentials" not found
  Normal   IssuerReconciler  13s (x3 over 13s)  sample-external-issuer  Success
```
And this:

```
$ kubectl describe certificaterequests.cert-manager.io issuer-sample
...
Events:
  Type     Reason                        Age   From                    Message
  ----     ------                        ----  ----                    -------
  Normal   CertificateRequestReconciler  23m   sample-external-issuer  Initialising Ready condition
  Warning  CertificateRequestReconciler  23m   sample-external-issuer  Temporary error. Retrying: error getting issuer: Issuer.sample-issuer.example.com "issuer-sample" not found
  Normal   CertificateRequestReconciler  23m   sample-external-issuer  Signed

```

First add [record.EventRecorder][] attributes to the `IssuerReconciler` and to the `CertificateRequestReconciler`.
And then in the Reconciler code, you can then generate an event by executing `r.recorder.Eventf(...)` whenever a significant change is made to the resource.

[record.EventRecorder]: https://pkg.go.dev/k8s.io/client-go/tools/record#EventRecorder

You can also write unit tests to verify the Reconciler events by using a [record.FakeRecorder][].

[record.FakeRecorder]: https://pkg.go.dev/k8s.io/client-go/tools/record#FakeRecorder

See [PR 10: Generate Kubernetes Events](https://github.com/cert-manager/sample-external-issuer/pull/10) for an example of how you might generate events in your issuer.

### End-to-end tests

Now our issuer is almost feature complete and it should be possible to write an end-to-end test that
deploys a cert-manager `Certificate`
referring to an external `Issuer` and check that a signed `Certificate` is saved to the expected secret.

We can make such a test easier by tidying up the `Makefile` and adding some new targets
which will help create a test cluster and to help install cert-manager.

We can write a simple end-to-end test which deploys a `Certificate` manifest and waits for it to be ready.

```console
kubectl apply --filename config/samples
kubectl wait --for=condition=Ready --timeout=5s sampleissuers.sample-issuer.example.com sampleissuer-sample
kubectl wait --for=condition=Ready --timeout=5s  certificates.cert-manager.io certificate-by-sampleissuer
```

You can of course write more complete tests than this,
but this is a good start and demonstrates that the issuer is doing what we hoped it would do.

Run the tests as follows:

```bash
# Create a Kind cluster along with cert-manager.
make kind-cluster deploy-cert-manager

# Wait for cert-manager to start...

# Build and install sample-external-issuer and run the E2E tests.
# This step can be run iteratively when ever you make changes to the code or to the installation manifests.
make docker-build kind-load deploy e2e
```

#### Continuous Integration

You should configure a CI system to automatically run the unit-tests when the code changes.
See the `.github/workflows/`  directory for some examples of using GitHub Actions
which are triggered by changes to pull request branches and by any changes to the master branch.

The E2E tests can be executed with GitHub Actions too.
The GitHub Actions Ubuntu runner has Docker installed and is capable of running a Kind cluster for the E2E tests.
The Kind cluster logs can be saved in the event of an E2E test failure,
and uploaded as a GitHub Actions artifact,
to make it easier to diagnose E2E test failures.

## Security considerations

We use a [Distroless Docker Image][] as our Docker base image,
and we configure our `manager` process to run as `USER: nonroot:nonroot`.
This limits the privileges of the `manager` process in the cluster.

The [kube-rbac-proxy][] sidecar Docker image also uses a non-root user by default (since v0.7.0).

Additionally we [Configure a Security Context][] for the manager Pod.
We set `runAsNonRoot`, which ensure that the Kubelet will validate the image at runtime
to ensure that it does not run as UID 0 (root) and fail to start the container if it does.

## Notes for cert-manager Maintainers

### Release Process

Visit the [GitHub New Release Page][] and fill in the form.
Here are some example values:

 * Tag Version: `v0.1.0-alpha.0`, `v0.1.0` for example.
 * Target: `main`
 * Release Title: `Release v0.1.0-alpha.2`
 * Description: (optional) a short summary of the changes since the last release.

Click the `Publish release` button to trigger the automated release process:

* A Docker image will be generated and published to `ghcr.io/cert-manager/sample-external-issuer/controller` with the chosen tag.
* An `install.yaml` file will be generated and attached to the release.

## Links

[External Issuer]: https://cert-manager.io/docs/contributing/external-issuers
[issuer-lib]: https://github.com/cert-manager/issuer-lib
[cert-manager Concepts Documentation]: https://cert-manager.io/docs/concepts
[Kubebuilder Book]: https://book.kubebuilder.io
[Kubebuilder Markers]: https://book.kubebuilder.io/reference/markers.html
[Distroless Docker Image]: https://github.com/GoogleContainerTools/distroless
[Configure a Security Context]: https://kubernetes.io/docs/tasks/configure-pod-container/security-context/
[kube-rbac-proxy]: https://github.com/brancz/kube-rbac-proxy
[GitHub New Release Page]: https://github.com/cert-manager/sample-external-issuer/releases/new
