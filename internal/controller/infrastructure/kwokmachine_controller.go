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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	controlplanev1alpha1 "github.com/capi-samples/cluster-api-provider-kwok/api/controlplane/v1alpha1"
	infrastructurev1alpha1 "github.com/capi-samples/cluster-api-provider-kwok/api/infrastructure/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// KwokMachineReconciler reconciles a KwokMachine object
type KwokMachineReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=kwokmachines,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=kwokmachines/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=kwokmachines/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the KwokMachine object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *KwokMachineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)

	kwokMachine := &infrastructurev1alpha1.KwokMachine{}
	err := r.Get(ctx, req.NamespacedName, kwokMachine)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Fetch the machine
	machine, err := util.GetOwnerMachine(ctx, r.Client, kwokMachine.ObjectMeta)
	if err != nil {
		logger.Error(err, "Failed to retrieve owner Machine from the API Server")

		return ctrl.Result{}, err
	}

	if machine == nil {
		logger.Info("Machine Controller has not yet set OwnerRef")

		return ctrl.Result{Requeue: true}, nil
	}

	logger = logger.WithValues("machine", machine.Name)

	if kwokMachine.Status.FailureMessage != nil ||
		kwokMachine.Status.FailureReason != nil {
		// TODO(vadasambar): add better log msg
		// with more details (machine name, failure msg/reason etc.,)
		logger.Info("KwokMachine has failed status")
		return ctrl.Result{}, nil
	}

	// Fetch the Cluster.
	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, machine.ObjectMeta)
	if err != nil {
		logger.Info("Machine is missing cluster label or cluster does not exist")
		return ctrl.Result{}, nil
	}

	if annotations.IsPaused(cluster, kwokMachine) {
		logger.Info("Machine or linked Cluster is marked as paused. Won't reconcile")
		return ctrl.Result{}, nil
	}

	logger = logger.WithValues("cluster", klog.KObj(cluster))

	if cluster.Status.InfrastructureReady == false {
		logger.Info("Cluster is not ready yet")
		return ctrl.Result{}, nil
	}

	if machine.Spec.Bootstrap.DataSecretName == nil {
		logger.Info("Bootstrap data secret is not present")
		return ctrl.Result{}, nil
	}

	infraCluster, err := r.getInfraCluster(ctx, &logger, cluster, kwokMachine)

	if err != nil {
		return ctrl.Result{}, errors.New("error getting infra provider cluster or control plane object")
	}
	if infraCluster == nil {
		logger.Info("KwokCluster or KwokManagedControlPlane is not ready yet")
		return ctrl.Result{}, nil
	}

	if !machine.ObjectMeta.DeletionTimestamp.IsZero() {
		err := r.Delete(context.Background(), kwokMachine)
		if err != nil {
			logger.Info("Error deleting KwokMachine", "kwokMachine", kwokMachine.GetName())
			return ctrl.Result{Requeue: true}, nil
		} else {
			return ctrl.Result{}, nil
		}
	}

	// kwokMachine

	return ctrl.Result{}, nil
}

func (r *KwokMachineReconciler) getInfraCluster(ctx context.Context, log *logr.Logger, cluster *clusterv1.Cluster, kwokMachine *infrastructurev1alpha1.KwokMachine) (runtime.Object, error) {

	if cluster.Spec.ControlPlaneRef != nil && cluster.Spec.ControlPlaneRef.Kind == "KwokControlPlane" {
		controlPlane := &controlplanev1alpha1.KwokControlPlane{}
		controlPlaneName := client.ObjectKey{
			Namespace: kwokMachine.Namespace,
			Name:      cluster.Spec.ControlPlaneRef.Name,
		}

		if err := r.Get(ctx, controlPlaneName, controlPlane); err != nil {
			// KwokControlPlane is not ready
			return nil, nil //nolint:nilerr
		}

		return controlPlane, nil
	}

	kwokCluster := &infrastructurev1alpha1.KwokCluster{}

	infraClusterName := client.ObjectKey{
		Namespace: kwokCluster.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}

	if err := r.Client.Get(ctx, infraClusterName, kwokCluster); err != nil {
		// KwokCluster is not ready
		return nil, nil //nolint:nilerr
	}

	return kwokCluster, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *KwokMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1alpha1.KwokMachine{}).
		Complete(r)
}
