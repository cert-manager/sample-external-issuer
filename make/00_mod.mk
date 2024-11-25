# Copyright 2023 The cert-manager Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

go_header_file := $(CURDIR)/make/config/boilerplate.go.txt

repo_name := github.com/cert-manager/sample-external-issuer

kind_cluster_name := sample-external-issuer
kind_cluster_config := $(bin_dir)/scratch/kind_cluster.yaml

build_names := manager

go_manager_main_dir := .
go_manager_mod_dir := .
go_manager_ldflags := -X $(repo_name)/pkg/internal/version.Version=$(VERSION)
oci_manager_base_image_flavor := static
oci_manager_image_name := ghcr.io/cert-manager/sample-external-issuer/controller
oci_manager_image_tag := $(VERSION)
oci_manager_image_name_development := cert-manager.local/sample-external-issuer

deploy_name := sample-external-issuer
deploy_namespace := sample-external-issuer-system

api_docs_outfile := docs/api/api.md
api_docs_package := $(repo_name)/api/v1alpha1
api_docs_branch := main

helm_chart_source_dir := deploy/charts/sample-external-issuer
helm_chart_name := sample-external-issuer
helm_chart_version := $(VERSION)
helm_labels_template_name := sample-external-issuer.labels
helm_docs_use_helm_tool := 1
helm_generate_schema := 1
helm_verify_values := 1

golangci_lint_config := .golangci.yaml

define helm_values_mutation_function
$(YQ) \
	'( .image.repository = "$(oci_manager_image_name)" ) | \
	( .image.tag = "$(oci_manager_image_tag)" )' \
	$1 --inplace
endef
