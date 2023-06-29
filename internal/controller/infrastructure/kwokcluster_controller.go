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

package controller

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	kruntime "sigs.k8s.io/kwok/pkg/kwokctl/runtime"

	controlplanev1 "github.com/capi-samples/cluster-api-provider-kwok/api/controlplane/v1alpha1"
	infrav1 "github.com/capi-samples/cluster-api-provider-kwok/api/infrastructure/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
)

// KwokClusterReconciler reconciles a KwokCluster object
type KwokClusterReconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	WatchFilterValue string
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=kwokclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=kwokclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=kwokclusters/finalizers,verbs=update
//+kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=kwokcontrolplanes;kwokcontrolplanes/status,verbs=get;list;watch
//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *KwokClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	// Get the KwokCluster
	kwokCluster := &infrav1.KwokCluster{}
	err := r.Get(ctx, req.NamespacedName, kwokCluster)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Fetch the Cluster.
	cluster, err := util.GetOwnerCluster(ctx, r.Client, kwokCluster.ObjectMeta)
	if err != nil {
		return reconcile.Result{}, err
	}
	if cluster == nil {
		log.Info("Cluster Controller has not yet set OwnerRef")
		return reconcile.Result{}, nil
	}

	var timerCh <-chan time.Time
	if kwokCluster.Spec.SimulationConfig != nil && kwokCluster.Spec.SimulationConfig.Reconcile.Latency.Duration != 0 {
		timerCh = time.After(kwokCluster.Spec.SimulationConfig.Reconcile.Latency.Duration)
	}

	if annotations.IsPaused(cluster, kwokCluster) {
		log.Info("KwokCluster or linked Cluster is marked as paused. Won't reconcile")
		return reconcile.Result{}, nil
	}

	log = log.WithValues("cluster", cluster.Name)

	controlPlane := &controlplanev1.KwokControlPlane{}
	controlPlaneRef := types.NamespacedName{
		Name:      cluster.Spec.ControlPlaneRef.Name,
		Namespace: cluster.Spec.ControlPlaneRef.Namespace,
	}

	if err := r.Get(ctx, controlPlaneRef, controlPlane); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get control plane ref: %w", err)
	}

	log = log.WithValues("controlPlane", controlPlaneRef.Name)

	patchHelper, err := patch.NewHelper(kwokCluster, r.Client)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to init patch helper: %w", err)
	}

	// Check the runtime is valid
	runtime := "docker"
	if kwokCluster.Spec.Runtime != "" {
		runtime = kwokCluster.Spec.Runtime
	}

	log = log.WithValues("runtime", runtime)

	_, ok := kruntime.DefaultRegistry.Get(runtime)
	if !ok {
		return reconcile.Result{}, fmt.Errorf("runtime %q not found", runtime)
	}

	// Set the values from the managed control plane
	kwokCluster.Status.Ready = true
	kwokCluster.Spec.ControlPlaneEndpoint = controlPlane.Spec.ControlPlaneEndpoint

	if err := patchHelper.Patch(ctx, kwokCluster); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to patch KwokCluster: %w", err)
	}

	if kwokCluster.Spec.SimulationConfig != nil && kwokCluster.Spec.SimulationConfig.Reconcile.Latency.Duration != 0 {
		<-timerCh
	}

	log.Info("Successfully reconciled KwokCluster")

	return reconcile.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *KwokClusterReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	//log := ctrl.LoggerFrom(ctx)

	kwokCluster := &infrav1.KwokCluster{}

	_, err := ctrl.NewControllerManagedBy(mgr).
		For(kwokCluster).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(ctrl.LoggerFrom(ctx), r.WatchFilterValue)).Build(r)
	if err != nil {
		return fmt.Errorf("error creating controller: %w", err)
	}

	// Add a watch for clusterv1.Cluster unpaise
	// if err = controller.Watch(
	// 	&source.Kind{Type: &clusterv1.Cluster{}},
	// 	handler.EnqueueRequestsFromMapFunc(util.ClusterToInfrastructureMapFunc(ctx, infrav1.GroupVersion.WithKind("KwokCluster"), mgr.GetClient(), &infrav1.KwokCluster{})),
	// 	predicates.ClusterUnpaused(log),
	// ); err != nil {
	// 	return fmt.Errorf("failed adding a watch for ready clusters: %w", err)
	// }

	// Add a watch for KwokControlPlane
	// if err = controller.Watch(
	// 	&source.Kind{Type: &controlplanev1.KwokControlPlane{}},
	// 	handler.EnqueueRequestsFromMapFunc(r.kwokControlPlaneToKwokCluster(ctx, &log)),
	// ); err != nil {
	// 	return fmt.Errorf("failed adding watch on KwokControlPlane: %w", err)
	// }

	return nil
}

func (r *KwokClusterReconciler) kwokControlPlaneToKwokCluster(ctx context.Context, log *logr.Logger) handler.MapFunc {
	return func(ctx context.Context, o client.Object) []ctrl.Request {
		kwokControlPlane, ok := o.(*controlplanev1.KwokControlPlane)
		if !ok {
			log.Error(errors.Errorf("expected an KwokControlPlane, got %T instead", o), "failed to map KwokControlPlane")
			return nil
		}

		log := log.WithValues("objectMapper", "kwokcpTokwokc", "kwokcontrolplane", klog.KRef(kwokControlPlane.Namespace, kwokControlPlane.Name))

		if !kwokControlPlane.ObjectMeta.DeletionTimestamp.IsZero() {
			log.Info("KwokControlPlane has a deletion timestamp, skipping mapping")
			return nil
		}

		if kwokControlPlane.Spec.ControlPlaneEndpoint.IsZero() {
			log.V(2).Info("KwokControlPlane has no control plane endpoint, skipping mapping")
			return nil
		}

		cluster, err := util.GetOwnerCluster(ctx, r.Client, kwokControlPlane.ObjectMeta)
		if err != nil {
			log.Error(err, "failed to get owning cluster")
			return nil
		}
		if cluster == nil {
			log.Info("no owning cluster, skipping mapping")
			return nil
		}

		kwokClusterRef := cluster.Spec.InfrastructureRef
		if kwokClusterRef == nil {
			log.Info("InfrastructureRef is nil, skipping mapping")
			return nil
		}

		return []ctrl.Request{
			{
				NamespacedName: types.NamespacedName{
					Name:      kwokClusterRef.Name,
					Namespace: kwokClusterRef.Namespace,
				},
			},
		}
	}
}
