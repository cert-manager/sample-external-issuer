MAKEFLAGS += --warn-undefined-variables --no-builtin-rules
SHELL := /usr/bin/env bash
.SHELLFLAGS := -uo pipefail -c
.DELETE_ON_ERROR:
.SUFFIXES:

# FORCE is a helper target to force a file to be rebuilt whenever its
# target is invoked.
FORCE:

BINDIR := _bin
SOURCES := $(shell find . -not \( -path "./$(BINDIR)/*" -prune \) -name "*.go" | cat -)

$(BINDIR) $(BINDIR)/scratch $(BINDIR)/tools $(BINDIR)/downloaded $(BINDIR)/downloaded/tools $(BINDIR)/bin:
	@mkdir -p $@

include make/tools.mk

# ==================================
# Actions
# ==================================

# The version which will be reported by the --version argument of each binary
# and which will be used as the Docker image tag
VERSION ?= $(shell git describe --tags)
GIT_COMMIT ?= $(shell git rev-parse "HEAD^{commit}" 2>/dev/null)

# The Docker repository name, overridden in CI.
DOCKER_REGISTRY ?= ghcr.io
DOCKER_IMAGE_NAME ?= cert-manager/sample-external-issuer/controller

# Image URL to use all building/pushing image targets
IMG ?= ${DOCKER_REGISTRY}/${DOCKER_IMAGE_NAME}:${VERSION}

GOFLAGS := -trimpath

GOLDFLAGS := -w -s \
	-X github.com/cert-manager/sample-external-issuer/internal/version.Version=$(VERSION) \
    -X github.com/cert-manager/sample-external-issuer/internal/version.GitCommit=$(GIT_COMMIT)

# Kind
K8S_CLUSTER_NAME := sample-external-issuer-e2e

# cert-manager
CERT_MANAGER_VERSION ?= 1.9.0

INSTALL_YAML ?= $(BINDIR)/yaml/install.yaml

.DEFAULT_GOAL := all
.PHONY: all
all: manager

# Run tests
.PHONY: test
test: update-all | $(DEPENDS_ON_GO)
	$(GO) test ./... -coverprofile cover.out

# Run against the configured Kubernetes cluster in ~/.kube/config
.PHONY: run
ARGS ?= # default empty
run: update-all | $(DEPENDS_ON_GO)
	$(GO) run ./main.go $(ARGS)

# Build manager binary
.PHONY: manager
manager: $(BINDIR)/bin/manager

$(BINDIR)/bin/manager: $(SOURCES) $(DEPENDS_ON_GO) | $(BINDIR)/bin
	GOOS=linux GOARCH=amd64 $(GO) build -o $@ $(GOFLAGS) -ldflags '$(GOLDFLAGS)' main.go

.PHONY: update-all
update-all: generate fmt vet manifests

# Run go fmt against code
fmt: $(SOURCES) | $(DEPENDS_ON_GO)
	$(GO) fmt ./...

# Run go vet against code
vet: $(SOURCES) | $(DEPENDS_ON_GO)
	$(GO) vet ./...

# Generate code
generate: $(SOURCES) | $(CONTROLLER_GEN)
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

# Generate manifests e.g. CRD, RBAC etc.
manifests: $(SOURCES) | $(CONTROLLER_GEN)
	$(CONTROLLER_GEN) crd:trivialVersions=true rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

# Install CRDs into a cluster
.PHONY: install
install: manifests
	$(KUBECTL) kustomize config/crd | $(KUBECTL) apply -f -

# Uninstall CRDs from a cluster
.PHONY: uninstall
uninstall: manifests
	$(KUBECTL) kustomize config/crd | $(KUBECTL) delete -f -

# TODO(wallrj): .PHONY ensures that the install file is always regenerated,
# because I this really depends on the checksum of the Docker image and all the
# base Kustomize files.
.PHONY: $(INSTALL_YAML)
$(INSTALL_YAML): | $(KUSTOMIZE) $(BINDIR)/scratch
	mkdir -p $(dir $@)
	rm -rf $(BINDIR)/scratch/kustomize
	mkdir -p $(BINDIR)/scratch/kustomize
	LDIR=`realpath --relative-to="$(BINDIR)/scratch/kustomize" ./config/default` && \
	LKUSTOMIZE=`realpath $(KUSTOMIZE)` && \
	cd $(BINDIR)/scratch/kustomize && \
	$$LKUSTOMIZE create --resources $$LDIR && \
	$$LKUSTOMIZE edit set image controller=$(IMG)
	$(KUSTOMIZE) build $(BINDIR)/scratch/kustomize > $@

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
.PHONY: deploy
deploy: $(INSTALL_YAML)
	kubectl apply -f $(INSTALL_YAML)

# Build the docker image
.PHONY: docker-build
docker-build:
	$(CTR) build \
		--build-arg GOFLAGS="$(GOFLAGS)" \
		--build-arg GOLDFLAGS="$(GOLDFLAGS)" \
		--tag $(IMG) \
		--file Dockerfile \
		${CURDIR}

# Push the docker image
.PHONY: docker-push
docker-push:
	$(CTR) push $(IMG)

# ==================================
# E2E testing
# ==================================
.PHONY: kind-cluster
kind-cluster: | $(KIND) ## Use Kind to create a Kubernetes cluster for E2E tests
	$(KIND) get clusters | grep $(K8S_CLUSTER_NAME) || $(KIND) create cluster --name $(K8S_CLUSTER_NAME)

.PHONY: kind-load
kind-load: | $(KIND) ## Load all the Docker images into Kind
	$(KIND) load docker-image --name $(K8S_CLUSTER_NAME) $(IMG)

.PHONY: kind-export-logs
kind-export-logs: | $(KIND)
	$(KIND) export logs --name $(K8S_CLUSTER_NAME) $(E2E_ARTIFACTS_DIRECTORY)


.PHONY: deploy-cert-manager
deploy-cert-manager: | $(KUBECTL) ## Deploy cert-manager in the configured Kubernetes cluster in ~/.kube/config
	$(KUBECTL) apply --filename=https://github.com/cert-manager/cert-manager/releases/download/v$(CERT_MANAGER_VERSION)/cert-manager.yaml
	$(KUBECTL) wait --for=condition=Available --timeout=300s apiservice v1.cert-manager.io

.PHONY: e2e
e2e: | $(KUBECTL)
	$(KUBECTL) apply --filename config/samples

	$(KUBECTL) wait --for=condition=Ready --timeout=5s issuers.sample-issuer.example.com issuer-sample
	$(KUBECTL) wait --for=condition=Ready --timeout=5s certificaterequests.cert-manager.io issuer-sample
	$(KUBECTL) wait --for=condition=Ready --timeout=5s certificates.cert-manager.io certificate-by-issuer

	$(KUBECTL) wait --for=condition=Ready --timeout=5s clusterissuers.sample-issuer.example.com clusterissuer-sample
	$(KUBECTL) wait --for=condition=Ready --timeout=5s certificaterequests.cert-manager.io clusterissuer-sample
	$(KUBECTL) wait --for=condition=Ready --timeout=5s certificates.cert-manager.io certificate-by-clusterissuer

	$(KUBECTL) delete --filename config/samples
