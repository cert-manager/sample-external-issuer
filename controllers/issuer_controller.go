/*
Copyright 2020 The cert-manager Authors

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

package controllers

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sampleissuerapi "github.com/cert-manager/sample-external-issuer/api/v1alpha1"
	issuerutil "github.com/cert-manager/sample-external-issuer/internal/issuer/util"
)

const (
	issuerReadyConditionReason = "sample-issuer.IssuerController.Reconcile"
)

var (
	errGetAuthSecret = errors.New("failed to get Secret containing Issuer credentials")
)

// IssuerReconciler reconciles a Issuer object
type IssuerReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=sample-issuer.example.com,resources=issuers,verbs=get;list;watch
// +kubebuilder:rbac:groups=sample-issuer.example.com,resources=issuers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

func (r *IssuerReconciler) Reconcile(req ctrl.Request) (result ctrl.Result, err error) {
	ctx := context.Background()
	log := r.Log.WithValues("issuer", req.NamespacedName)

	// Get the Issuer
	var issuer sampleissuerapi.Issuer
	if err := r.Get(ctx, req.NamespacedName, &issuer); err != nil {
		if err := client.IgnoreNotFound(err); err != nil {
			return ctrl.Result{}, fmt.Errorf("unexpected get error: %v", err)
		}
		log.Info("Not found. Ignoring.")
		return ctrl.Result{}, nil
	}

	// Always attempt to update the Ready condition
	defer func() {
		if err != nil {
			issuerutil.SetReadyCondition(&issuer.Status, sampleissuerapi.ConditionFalse, issuerReadyConditionReason, err.Error())
		}
		if updateErr := r.Status().Update(ctx, &issuer); updateErr != nil {
			err = utilerrors.NewAggregate([]error{err, updateErr})
			result = ctrl.Result{}
		}
	}()

	if ready := issuerutil.GetReadyCondition(&issuer.Status); ready == nil {
		issuerutil.SetReadyCondition(&issuer.Status, sampleissuerapi.ConditionUnknown, issuerReadyConditionReason, "First seen")
		return ctrl.Result{}, nil
	}

	secretName := types.NamespacedName{
		Name:      issuer.Spec.AuthSecretName,
		Namespace: issuer.Namespace,
	}
	var secret corev1.Secret
	if err := r.Get(ctx, secretName, &secret); err != nil {
		return ctrl.Result{}, fmt.Errorf("%w, secret name: %s, reason: %v", errGetAuthSecret, secretName, err)
	}

	issuerutil.SetReadyCondition(&issuer.Status, sampleissuerapi.ConditionTrue, issuerReadyConditionReason, "Success")
	return ctrl.Result{}, nil
}

func (r *IssuerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&sampleissuerapi.Issuer{}).
		Complete(r)
}
