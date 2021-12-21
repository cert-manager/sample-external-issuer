MAKEFLAGS += --warn-undefined-variables
SHELL := bash
.SHELLFLAGS := -eu -o pipefail -c
.DELETE_ON_ERROR:
.SUFFIXES:
.ONESHELL:

# The version which will be reported by the --version argument of each binary
# and which will be used as the Docker image tag
VERSION ?= $(shell git describe --tags)
# The Docker repository name, overridden in CI.
DOCKER_REGISTRY ?= ghcr.io
DOCKER_IMAGE_NAME ?= cert-manager/sample-external-issuer/controller
# Image URL to use all building/pushing image targets
IMG ?= ${DOCKER_REGISTRY}/${DOCKER_IMAGE_NAME}:${VERSION}
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

# BIN is the directory where tools will be installed
export BIN ?= ${CURDIR}/bin

OS := $(shell go env GOOS)
ARCH := $(shell go env GOARCH)

# Kind
KIND_VERSION := 0.9.0
KIND := ${BIN}/kind-${KIND_VERSION}
K8S_CLUSTER_NAME := sample-external-issuer-e2e

# cert-manager
CERT_MANAGER_VERSION ?= 1.3.0

# Controller tools
CONTROLLER_GEN_VERSION := 0.5.0
CONTROLLER_GEN := ${BIN}/controller-gen-${CONTROLLER_GEN_VERSION}

INSTALL_YAML ?= build/install.yaml

.PHONY: all
all: manager

# Run tests
.PHONY: test
test: generate fmt vet manifests
	go test ./... -coverprofile cover.out

# Build manager binary
.PHONY: manager
manager: generate fmt vet
	go build -o bin/manager main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
.PHONY: run
run: generate fmt vet manifests
	go run ./main.go

# Install CRDs into a cluster
.PHONY: install
install: manifests
	kustomize build config/crd | kubectl apply -f -

# Uninstall CRDs from a cluster
.PHONY: uninstall
uninstall: manifests
	kustomize build config/crd | kubectl delete -f -

# TODO(wallrj): .PHONY ensures that the install file is always regenerated,
# because I this really depends on the checksum of the Docker image and all the
# base Kustomize files.
.PHONY: ${INSTALL_YAML}
${INSTALL_YAML}:
	mkdir -p $(dir ${INSTALL_YAML})
	TMP_DIR=$$(mktemp -d -p ${CURDIR})
	trap "rm -rf $${TMP_DIR}" EXIT
	pushd $${TMP_DIR}
	kustomize create --resources ../config/default
	kustomize edit set image controller=${IMG}
	popd
	kustomize build $${TMP_DIR} > ${INSTALL_YAML}

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
.PHONY: deploy
deploy: ${INSTALL_YAML}
	 kubectl apply -f ${INSTALL_YAML}

# Generate manifests e.g. CRD, RBAC etc.
.PHONY: manifests
manifests: ${CONTROLLER_GEN}
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

# Run go fmt against code
.PHONY: fmt
fmt:
	go fmt ./...

# Run go vet against code
.PHONY: vet
vet:
	go vet ./...

# Generate code
generate: ${CONTROLLER_GEN}
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

# Build the docker image
.PHONY: docker-build
docker-build:
	docker build \
		--build-arg VERSION=$(VERSION) \
		--tag ${IMG} \
		--file Dockerfile \
		${CURDIR}

# Push the docker image
.PHONY: docker-push
docker-push:
	docker push ${IMG}

${CONTROLLER_GEN}: | ${BIN}
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d)
	trap "rm -rf $${CONTROLLER_GEN_TMP_DIR}" EXIT
	cd $${CONTROLLER_GEN_TMP_DIR}
	go mod init tmp
	GOBIN=$${CONTROLLER_GEN_TMP_DIR} go get sigs.k8s.io/controller-tools/cmd/controller-gen@v${CONTROLLER_GEN_VERSION}
	mv $${CONTROLLER_GEN_TMP_DIR}/controller-gen ${CONTROLLER_GEN}


# ==================================
# E2E testing
# ==================================
.PHONY: kind-cluster
kind-cluster: ## Use Kind to create a Kubernetes cluster for E2E tests
kind-cluster: ${KIND}
	 ${KIND} get clusters | grep ${K8S_CLUSTER_NAME} || ${KIND} create cluster --name ${K8S_CLUSTER_NAME}

.PHONY: kind-load
kind-load: ## Load all the Docker images into Kind
	${KIND} load docker-image --name ${K8S_CLUSTER_NAME} ${IMG}

.PHONY: kind-export-logs
kind-export-logs:
	${KIND} export logs --name ${K8S_CLUSTER_NAME} ${E2E_ARTIFACTS_DIRECTORY}


.PHONY: deploy-cert-manager
deploy-cert-manager: ## Deploy cert-manager in the configured Kubernetes cluster in ~/.kube/config
	kubectl apply --filename=https://github.com/jetstack/cert-manager/releases/download/v${CERT_MANAGER_VERSION}/cert-manager.yaml
	kubectl wait --for=condition=Available --timeout=300s apiservice v1.cert-manager.io

.PHONY: e2e
e2e:
	kubectl apply --filename config/samples

	kubectl wait --for=condition=Ready --timeout=5s issuers.sample-issuer.example.com issuer-sample
	kubectl wait --for=condition=Ready --timeout=5s  certificaterequests.cert-manager.io issuer-sample
	kubectl wait --for=condition=Ready --timeout=5s  certificates.cert-manager.io certificate-by-issuer

	kubectl wait --for=condition=Ready --timeout=5s clusterissuers.sample-issuer.example.com clusterissuer-sample
	kubectl wait --for=condition=Ready --timeout=5s  certificaterequests.cert-manager.io clusterissuer-sample
	kubectl wait --for=condition=Ready --timeout=5s  certificates.cert-manager.io certificate-by-clusterissuer

	kubectl delete --filename config/samples

# ==================================
# Download: tools in ${BIN}
# ==================================
${BIN}:
	mkdir -p ${BIN}

${KIND}: ${BIN}
	curl -sSL -o ${KIND} https://github.com/kubernetes-sigs/kind/releases/download/v${KIND_VERSION}/kind-${OS}-${ARCH}
	chmod +x ${KIND}
