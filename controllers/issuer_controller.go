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
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sampleissuerapi "github.com/cert-manager/sample-external-issuer/api/v1alpha1"
	issuerutil "github.com/cert-manager/sample-external-issuer/internal/issuer/util"
)

const (
	issuerReadyConditionReason = "sample-issuer.IssuerController.Reconcile"
)

// IssuerReconciler reconciles a Issuer object
type IssuerReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=sample-issuer.example.com,resources=issuers,verbs=get;list;watch
// +kubebuilder:rbac:groups=sample-issuer.example.com,resources=issuers/status,verbs=get;update;patch

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

	issuerutil.SetReadyCondition(&issuer.Status, sampleissuerapi.ConditionTrue, issuerReadyConditionReason, "Success")
	return ctrl.Result{}, nil
}

func (r *IssuerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&sampleissuerapi.Issuer{}).
		Complete(r)
}
