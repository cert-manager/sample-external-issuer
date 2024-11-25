/*
Copyright 2023 The cert-manager Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	cmapi "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	sampleissuerv1alpha1 "github.com/cert-manager/sample-external-issuer/api/v1alpha1"
	"github.com/cert-manager/sample-external-issuer/internal/controllers"
	"github.com/cert-manager/sample-external-issuer/internal/signer"
	"github.com/cert-manager/sample-external-issuer/internal/version"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

const inClusterNamespacePath = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

type options struct {
	metricsAddr              string
	probeAddr                string
	enableLeaderElection     bool
	clusterResourceNamespace string
	printVersion             bool
	disableApprovedCheck     bool
}

func main() {
	opts := options{}
	flag.StringVar(&opts.metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&opts.probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&opts.enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&opts.clusterResourceNamespace, "cluster-resource-namespace", "", "The namespace for secrets in which cluster-scoped resources are found.")
	flag.BoolVar(&opts.printVersion, "version", false, "Print version to stdout and exit")
	flag.BoolVar(&opts.disableApprovedCheck, "disable-approved-check", false,
		"Disables waiting for CertificateRequests to have an approved condition before signing.")

	// Options for configuring logging
	loggerOpts := zap.Options{}
	loggerOpts.BindFlags(flag.CommandLine)

	flag.Parse()

	logr := zap.New(zap.UseFlagOptions(&loggerOpts))

	klog.SetLogger(logr)
	ctrl.SetLogger(logr)

	logr.Info("Version", "version", version.Version)

	if opts.printVersion {
		return
	}

	if err := Main(logr, opts); err != nil {
		logr.Error(err, "error running manager")
		os.Exit(1)
	}
}

func Main(
	logr klog.Logger,
	opts options,
) error {
	setupLog := logr.WithName("setup")

	if err := getInClusterNamespace(&opts.clusterResourceNamespace); err != nil {
		if errors.Is(err, errNotInCluster) {
			return fmt.Errorf("please supply --cluster-resource-namespace: %w", err)
		} else {
			return fmt.Errorf("unexpected error while getting in-cluster Namespace: %w", err)
		}
	}

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(cmapi.AddToScheme(scheme))
	utilruntime.Must(sampleissuerv1alpha1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme

	setupLog.Info(
		"starting",
		"version", version.Version,
		"enable-leader-election", opts.enableLeaderElection,
		"metrics-addr", opts.metricsAddr,
		"cluster-resource-namespace", opts.clusterResourceNamespace,
	)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: server.Options{
			BindAddress: opts.metricsAddr,
		},
		WebhookServer: webhook.NewServer(webhook.Options{
			Port: 9443,
		}),
		HealthProbeBindAddress: opts.probeAddr,
		LeaderElection:         opts.enableLeaderElection,
		LeaderElectionID:       "54c549fd.sample-external-issuer",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		return fmt.Errorf("unable to start manager: %w", err)
	}

	ctx, cancel := context.WithCancel(ctrl.SetupSignalHandler())
	defer cancel()

	if err = (&controllers.Issuer{
		HealthCheckerBuilder:     signer.ExampleHealthCheckerFromIssuerAndSecretData,
		SignerBuilder:            signer.ExampleSignerFromIssuerAndSecretData,
		ClusterResourceNamespace: opts.clusterResourceNamespace,
	}).SetupWithManager(ctx, mgr); err != nil {
		return fmt.Errorf("unable to create Signer controllers: %w", err)
	}

	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return fmt.Errorf("unable to set up health check: %w", err)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return fmt.Errorf("unable to set up ready check: %w", err)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		return fmt.Errorf("problem running manager: %w", err)
	}

	return nil
}

var errNotInCluster = errors.New("not running in-cluster")

// Copied from controller-runtime/pkg/leaderelection
func getInClusterNamespace(clusterResourceNamespace *string) error {
	if *clusterResourceNamespace != "" {
		return nil
	}

	// Check whether the namespace file exists.
	// If not, we are not running in cluster so can't guess the namespace.
	_, err := os.Stat(inClusterNamespacePath)
	if os.IsNotExist(err) {
		return errNotInCluster
	} else if err != nil {
		return fmt.Errorf("error checking namespace file: %w", err)
	}

	// Load the namespace file and return its content
	namespace, err := os.ReadFile(inClusterNamespacePath)
	if err != nil {
		return fmt.Errorf("error reading namespace file: %w", err)
	}
	*clusterResourceNamespace = string(namespace)

	return nil
}
