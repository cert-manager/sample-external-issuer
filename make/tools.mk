CTR=docker

KIND_VERSION := 0.14.0
CONTROLLER_GEN_VERSION := 0.5.0
VENDORED_GO_VERSION := 1.18.3
KUBECTL_VERSION := 1.24.3
KUSTOMIZE_VERSION := 4.5.2

.PHONY: tools
tools: $(BINDIR)/tools/kubectl $(BINDIR)/tools/kind $(BINDIR)/tools/controller-gen

# When switching branches which use different versions of the tools, we
# need a way to re-trigger the symlinking from $(BINDIR)/downloaded to $(BINDIR)/tools.
$(BINDIR)/scratch/%_VERSION: FORCE | $(BINDIR)/scratch
	@test "$($*_VERSION)" == "$(shell cat $@ 2>/dev/null)" || echo $($*_VERSION) > $@

# The reason we don't use "go env GOOS" or "go env GOARCH" is that the "go"
# binary may not be available in the PATH yet when the Makefiles are
# evaluated. HOST_OS and HOST_ARCH only support Linux, *BSD and macOS (M1
# and Intel).
HOST_OS := $(shell uname -s | tr A-Z a-z)
HOST_ARCH = $(shell uname -m)
ifeq (x86_64, $(HOST_ARCH))
	HOST_ARCH = amd64
endif

# --silent = don't print output like progress meters
# --show-error = but do print errors when they happen
# --fail = exit with a nonzero error code without the response from the server when there's an HTTP error
# --location = follow redirects from the server
# --retry = the number of times to retry a failed attempt to connect
# --retry-connrefused = retry even if the initial connection was refused
CURL = curl --silent --show-error --fail --location --retry 10 --retry-connrefused

######
# Go #
######

GO = go

# DEPENDS_ON_GO is a target that is set as an order-only prerequisite in
# any target that calls $(GO), e.g.:
#
#     $(BINDIR)/tools/crane: $(DEPENDS_ON_GO)
#         $(GO) build -o $(BINDIR)/tools/crane
#
# DEPENDS_ON_GO is empty most of the time, except when running "make
# vendor-go" or when "make vendor-go" was previously run, in which case
# DEPENDS_ON_GO is set to $(BINDIR)/tools/go, since $(BINDIR)/tools/go is a
# prerequisite of any target depending on Go when "make vendor-go" was run.
DEPENDS_ON_GO := $(if $(findstring vendor-go,$(MAKECMDGOALS))$(shell [ -f $(BINDIR)/tools/go ] && echo yes), $(BINDIR)/tools/go,)
ifneq ($(DEPENDS_ON_GO),)
export GOROOT := $(PWD)/$(BINDIR)/tools/goroot
export PATH := $(PWD)/$(BINDIR)/tools/goroot/bin:$(PATH)
GO := $(PWD)/$(BINDIR)/tools/go
endif

GOBUILD=CGO_ENABLED=$(CGO_ENABLED) GOMAXPROCS=$(GOBUILDPROCS) $(GO) build
GOTEST=CGO_ENABLED=$(CGO_ENABLED) $(GO) test

GOTESTSUM=CGO_ENABLED=$(CGO_ENABLED) ./$(BINDIR)/tools/gotestsum

.PHONY: vendor-go
## By default, this Makefile uses the system's Go. You can use a "vendored"
## version of Go that will get downloaded by running this command once. To
## disable vendoring, run "make unvendor-go". When vendoring is enabled,
## you will want to set the following:
##
##     export PATH="$PWD/$(BINDIR)/tools:$PATH"
##     export GOROOT="$PWD/$(BINDIR)/tools/goroot"
vendor-go: $(BINDIR)/tools/go

.PHONY: unvendor-go
unvendor-go: $(BINDIR)/tools/go
	rm -rf $(BINDIR)/tools/go $(BINDIR)/tools/goroot

.PHONY: which-go
## Print the version and path of go which will be used for building and
## testing in Makefile commands. Vendored go will have a path in ./bin
which-go: |  $(DEPENDS_ON_GO)
	@$(GO) version
	@echo "go binary used for above version information: $(GO)"

# In Prow, the pod has the folder "$(BINDIR)/downloaded" mounted into the
# container. For some reason, even though the permissions are correct,
# binaries that are mounted with hostPath can't be executed. When in CI, we
# copy the binaries to work around that. Using $(LN) is only required when
# dealing with binaries. Other files and folders can be symlinked.
#
# Details on how "$(BINDIR)/downloaded" gets cached are available in the
# description of the PR https://github.com/jetstack/testing/pull/651.
#
# We use "printenv CI" instead of just "ifeq ($(CI),)" because otherwise we
# would get "warning: undefined variable 'CI'".
ifeq ($(shell printenv CI),)
LN := ln -f -s
else
LN := cp -f -r
endif

# The "_" in "_go "prevents "go mod tidy" from trying to tidy the vendored
# goroot.
$(BINDIR)/tools/go: $(BINDIR)/downloaded/tools/_go-$(VENDORED_GO_VERSION)-$(HOST_OS)-$(HOST_ARCH)/goroot/bin/go $(BINDIR)/tools/goroot $(BINDIR)/scratch/VENDORED_GO_VERSION | $(BINDIR)/tools
	cd $(dir $@) && $(LN) $(patsubst $(BINDIR)/%,../%,$<) .
	@touch $@

$(BINDIR)/tools/goroot: $(BINDIR)/downloaded/tools/_go-$(VENDORED_GO_VERSION)-$(HOST_OS)-$(HOST_ARCH)/goroot $(BINDIR)/scratch/VENDORED_GO_VERSION | $(BINDIR)/tools
	@rm -rf $(BINDIR)/tools/goroot
	cd $(dir $@) && $(LN) $(patsubst $(BINDIR)/%,../%,$<) .
	@touch $@

$(BINDIR)/downloaded/tools/_go-$(VENDORED_GO_VERSION)-%/goroot $(BINDIR)/downloaded/tools/_go-$(VENDORED_GO_VERSION)-%/goroot/bin/go: $(BINDIR)/downloaded/tools/go-$(VENDORED_GO_VERSION)-%.tar.gz
	@mkdir -p $(dir $@)
	rm -rf $(BINDIR)/downloaded/tools/_go-$(VENDORED_GO_VERSION)-$*/goroot
	tar xzf $< -C $(BINDIR)/downloaded/tools/_go-$(VENDORED_GO_VERSION)-$*
	mv $(BINDIR)/downloaded/tools/_go-$(VENDORED_GO_VERSION)-$*/go $(BINDIR)/downloaded/tools/_go-$(VENDORED_GO_VERSION)-$*/goroot

$(BINDIR)/downloaded/tools/go-$(VENDORED_GO_VERSION)-%.tar.gz: | $(BINDIR)/downloaded/tools
	$(CURL) https://go.dev/dl/go$(VENDORED_GO_VERSION).$*.tar.gz -o $@

##################
# controller-gen #
##################

CONTROLLER_GEN:=$(BINDIR)/tools/controller-gen

$(BINDIR)/tools/controller-gen: $(BINDIR)/downloaded/tools/controller-gen@$(CONTROLLER_GEN_VERSION) $(BINDIR)/scratch/CONTROLLER_GEN_VERSION | $(BINDIR)/tools
	@cd $(dir $@) && $(LN) $(patsubst $(BINDIR)/%,../%,$<) $(notdir $@)

$(BINDIR)/downloaded/tools/controller-gen@$(CONTROLLER_GEN_VERSION): $(DEPENDS_ON_GO) | $(BINDIR)/downloaded/tools
	GOBIN=$(PWD)/$(dir $@) $(GO) install sigs.k8s.io/controller-tools/cmd/controller-gen@v$(CONTROLLER_GEN_VERSION)
	@mv $(subst @$(CONTROLLER_GEN_VERSION),,$@) $@

#############
# kustomize #
#############

KUSTOMIZE:=$(BINDIR)/tools/kustomize

$(BINDIR)/tools/kustomize: $(BINDIR)/downloaded/tools/kustomize@$(KUSTOMIZE_VERSION) $(BINDIR)/scratch/KUSTOMIZE_VERSION | $(BINDIR)/tools
	@cd $(dir $@) && $(LN) $(patsubst $(BINDIR)/%,../%,$<) $(notdir $@)

$(BINDIR)/downloaded/tools/kustomize@$(KUSTOMIZE_VERSION): $(DEPENDS_ON_GO) | $(BINDIR)/downloaded/tools
	GOBIN=$(PWD)/$(dir $@) $(GO) install sigs.k8s.io/kustomize/kustomize/v4@v$(KUSTOMIZE_VERSION)
	@mv $(subst @$(KUSTOMIZE_VERSION),,$@) $@

###########
# kubectl #
###########

KUBECTL:=$(BINDIR)/tools/kubectl

KUBECTL_linux_amd64_SHA256SUM=8a45348bdaf81d46caf1706c8bf95b3f431150554f47d444ffde89e8cdd712c1
KUBECTL_darwin_amd64_SHA256SUM=c585f970cdf76adab9cd5c32c29aa67fe4c1a6e3a5af113598524f94fe46a7c2
KUBECTL_darwin_arm64_SHA256SUM=653a31a944b220660684de8b963ddc7c4d5b2792d172f9ce30acb3f6a88e3c82

$(BINDIR)/tools/kubectl: $(BINDIR)/downloaded/tools/kubectl_$(KUBECTL_VERSION)_$(HOST_OS)_$(HOST_ARCH) $(BINDIR)/scratch/KUBECTL_VERSION | $(BINDIR)/tools
	@cd $(dir $@) && $(LN) $(patsubst $(BINDIR)/%,../%,$<) $(notdir $@)

$(BINDIR)/downloaded/tools/kubectl_$(KUBECTL_VERSION)_$(HOST_OS)_$(HOST_ARCH): | $(BINDIR)/downloaded/tools
	$(CURL) https://storage.googleapis.com/kubernetes-release/release/v$(KUBECTL_VERSION)/bin/$(HOST_OS)/$(HOST_ARCH)/kubectl -o $@
	./hack/util/checkhash.sh $@ $(KUBECTL_$(HOST_OS)_$(HOST_ARCH)_SHA256SUM)
	chmod +x $@

########
# kind #
########

KIND:=$(BINDIR)/tools/kind

KIND_linux_amd64_SHA256SUM=af5e8331f2165feab52ec2ae07c427c7b66f4ad044d09f253004a20252524c8b
KIND_darwin_amd64_SHA256SUM=fdf7209e5f92651ee5d55b78eb4ee5efded0d28c3f3ab8a4a163b6ffd92becfd
KIND_darwin_arm64_SHA256SUM=bdbb6e94bc8c846b0a6a1df9f962fe58950d92b26852fd6ebdc48fabb229932c

$(BINDIR)/tools/kind: $(BINDIR)/downloaded/tools/kind_$(KIND_VERSION)_$(HOST_OS)_$(HOST_ARCH) $(BINDIR)/scratch/KIND_VERSION | $(BINDIR)/tools
	@cd $(dir $@) && $(LN) $(patsubst $(BINDIR)/%,../%,$<) $(notdir $@)

$(BINDIR)/downloaded/tools/kind_$(KIND_VERSION)_%: | $(BINDIR)/downloaded/tools $(BINDIR)/tools
	$(CURL) -sSfL https://github.com/kubernetes-sigs/kind/releases/download/v$(KIND_VERSION)/kind-$(subst _,-,$*) -o $@
	./hack/util/checkhash.sh $@ $(KIND_$*_SHA256SUM)
	chmod +x $@
