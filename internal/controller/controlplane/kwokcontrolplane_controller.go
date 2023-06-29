/*
Copyright 2023 The Kubernetes Authors..

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

package controlplane

import (
	"context"
	"fmt"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	controlplanev1 "github.com/capi-samples/cluster-api-provider-kwok/api/controlplane/v1alpha1"
	infrav1 "github.com/capi-samples/cluster-api-provider-kwok/api/infrastructure/v1alpha1"
	"github.com/capi-samples/cluster-api-provider-kwok/pkg/scope"
	"github.com/capi-samples/cluster-api-provider-kwok/pkg/services"
	"github.com/capi-samples/cluster-api-provider-kwok/pkg/services/kwok/cluster"
	"github.com/go-logr/logr"
)

// KwokControlPlaneReconciler reconciles a KwokControlPlane object
type KwokControlPlaneReconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	WatchFilterValue string
}

//+kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=kwokcontrolplanes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=kwokcontrolplanes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=kwokcontrolplanes/finalizers,verbs=update
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=kwokclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=kwokclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *KwokControlPlaneReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, reterr error) {
	logger := log.FromContext(ctx)

	kwokControlPlane := &controlplanev1.KwokControlPlane{}
	err := r.Get(ctx, req.NamespacedName, kwokControlPlane)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Fetch the Cluster.
	cluster, err := util.GetOwnerCluster(ctx, r.Client, kwokControlPlane.ObjectMeta)
	if err != nil {
		logger.Error(err, "Failed to retrieve owner Cluster from the API Server")

		return ctrl.Result{}, err
	}

	if cluster == nil {
		logger.Info("Cluster Controller has not yet set OwnerRef")

		return ctrl.Result{Requeue: true}, nil
	}

	logger = logger.WithValues("cluster", cluster.Name)

	kwokCluster := &infrav1.KwokCluster{}
	kwokClusterRef := types.NamespacedName{
		Name:      cluster.Spec.InfrastructureRef.Name,
		Namespace: cluster.Spec.InfrastructureRef.Namespace,
	}

	if err := r.Get(ctx, kwokClusterRef, kwokCluster); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get kwok cluster ref: %w", err)
	}

	if annotations.IsPaused(cluster, kwokControlPlane) {
		logger.Info("Reconciliation is paused for this object")

		return ctrl.Result{}, nil
	}

	cpScope, err := scope.NewControlPlaneScope(scope.ControlPlaneScopeParams{
		Client:         r.Client,
		Cluster:        cluster,
		KwokCluster:    kwokCluster,
		ControlPlane:   kwokControlPlane,
		ControllerName: strings.ToLower(kwokControlPlane.Kind),
		Logger:         &logger,
	})
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to create scope: %w", err)
	}

	defer func() {
		//TODO: update conditions if needed
		if err := cpScope.Close(); err != nil {
			reterr = err
		}
	}()

	if !kwokControlPlane.ObjectMeta.DeletionTimestamp.IsZero() {
		// Handle deletion reconciliation loop.
		return r.reconcileDelete(ctx, cpScope)
	}

	// Handle normal reconciliation loop.
	return r.reconcileNormal(ctx, cpScope)

}

func (r *KwokControlPlaneReconciler) reconcileNormal(ctx context.Context, cpScope *scope.ControlPlaneScope) (res ctrl.Result, reterr error) {
	cpScope.Logger.Info("Reconciling KwokControlPlane")

	if controllerutil.AddFinalizer(cpScope.ControlPlane, controlplanev1.KwokControlPlaneFinalizer) {
		if err := cpScope.PatchObject(); err != nil {
			return ctrl.Result{}, err
		}
	}

	reconcilers := []services.ReconcilerWithResult{
		cluster.NewService(cpScope),
	}

	for _, r := range reconcilers {
		res, err := r.Reconcile(ctx)
		if err != nil {
			cpScope.Logger.Error(err, "Reconcile error")
			//record.Warnf(clusterScope.GCPCluster, "GCPClusterReconcile", "Reconcile error - %v", err)
			return ctrl.Result{}, err
		}
		if res.Requeue || res.RequeueAfter > 0 {
			return res, nil
		}
	}

	return reconcile.Result{}, nil
}

func (r *KwokControlPlaneReconciler) reconcileDelete(ctx context.Context, cpScope *scope.ControlPlaneScope) (res ctrl.Result, reterr error) {
	cpScope.Logger.Info("Reconciling KwokControlPlane delete")

	reconcilers := []services.ReconcilerWithResult{
		cluster.NewService(cpScope),
	}

	for _, r := range reconcilers {
		res, err := r.Delete(ctx)
		if err != nil {
			cpScope.Logger.Error(err, "Reconcile error")
			//record.Warnf(clusterScope.GCPCluster, "GCPClusterReconcile", "Reconcile error - %v", err)
			return ctrl.Result{}, err
		}
		if res.Requeue || res.RequeueAfter > 0 {
			return res, nil
		}
	}

	controllerutil.RemoveFinalizer(cpScope.ControlPlane, controlplanev1.KwokControlPlaneFinalizer)

	return reconcile.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *KwokControlPlaneReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	logger := log.FromContext(ctx)

	controlPlane := &controlplanev1.KwokControlPlane{}
	c, err := ctrl.NewControllerManagedBy(mgr).
		For(controlPlane).
		WithOptions(options).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(logger, r.WatchFilterValue)).
		Build(r)

	if err != nil {
		return fmt.Errorf("failed setting up the KwokControlPlane controller manager: %w", err)
	}

	if err = c.Watch(
		source.Kind(mgr.GetCache(), &clusterv1.Cluster{}),
		handler.EnqueueRequestsFromMapFunc(util.ClusterToInfrastructureMapFunc(ctx, controlPlane.GroupVersionKind(), mgr.GetClient(), &controlplanev1.KwokControlPlane{})),
		predicates.ClusterUnpausedAndInfrastructureReady(logger),
	); err != nil {
		return fmt.Errorf("failed adding a watch for ready clusters: %w", err)
	}

	if err = c.Watch(
		source.Kind(mgr.GetCache(), &infrav1.KwokCluster{}),
		handler.EnqueueRequestsFromMapFunc(r.kwokClusterToKwokControlPlane(ctx, &logger)),
	); err != nil {
		return fmt.Errorf("failed adding a watch for KwokCluster")
	}

	return nil
}

func (r *KwokControlPlaneReconciler) kwokClusterToKwokControlPlane(ctx context.Context, logger *logr.Logger) handler.MapFunc {
	return func(ctx context.Context, o client.Object) []ctrl.Request {
		kwokCluster, ok := o.(*infrav1.KwokCluster)
		if !ok {
			logger.Error(fmt.Errorf("expected a KwokCluster but got a %T", o), "Expected KwokCluster")
			return nil
		}

		if !kwokCluster.ObjectMeta.DeletionTimestamp.IsZero() {
			logger.V(2).Info("KwokCluster has a deletion timestamp, skipping mapping")
			return nil
		}

		cluster, err := util.GetOwnerCluster(ctx, r.Client, kwokCluster.ObjectMeta)
		if err != nil {
			logger.Error(err, "failed to get owning cluster")
			return nil
		}
		if cluster == nil {
			logger.V(2).Info("Owning cluster not set on KwokCluster, skipping mapping")
			return nil
		}

		controlPlaneRef := cluster.Spec.ControlPlaneRef
		if controlPlaneRef == nil || controlPlaneRef.Kind != "KwokControlPlane" {
			logger.V(2).Info("ControlPlaneRef is nil or not KwokControlPlane, skipping mapping")
			return nil
		}

		return []ctrl.Request{
			{
				NamespacedName: types.NamespacedName{
					Name:      controlPlaneRef.Name,
					Namespace: controlPlaneRef.Namespace,
				},
			},
		}
	}
}
