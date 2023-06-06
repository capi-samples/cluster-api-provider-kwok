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

package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	"github.com/spf13/pflag"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"
	"k8s.io/utils/pointer"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	_ "sigs.k8s.io/kwok/pkg/kwokctl/runtime/binary"
	_ "sigs.k8s.io/kwok/pkg/kwokctl/runtime/compose"
	_ "sigs.k8s.io/kwok/pkg/kwokctl/runtime/kind"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expclusterv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"

	bootstrapv1 "github.com/capi-samples/cluster-api-provider-kwok/api/bootstrap/v1alpha1"
	controlplanev1 "github.com/capi-samples/cluster-api-provider-kwok/api/controlplane/v1alpha1"
	infrav1 "github.com/capi-samples/cluster-api-provider-kwok/api/infrastructure/v1alpha1"

	//bootstrapcontroller "github.com/capi-samples/cluster-api-provider-kwok/internal/controller/bootstrap"
	controlplanecontroller "github.com/capi-samples/cluster-api-provider-kwok/internal/controller/controlplane"
	infracontroller "github.com/capi-samples/cluster-api-provider-kwok/internal/controller/infrastructure"
	"github.com/capi-samples/cluster-api-provider-kwok/pkg/consts"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")

	// flags.
	metricsBindAddr             string
	enableLeaderElection        bool
	leaderElectionLeaseDuration time.Duration
	leaderElectionRenewDeadline time.Duration
	leaderElectionRetryPeriod   time.Duration
	watchFilterValue            string
	profilerAddress             string
	syncPeriod                  time.Duration
	//webhookPort                 int
	//webhookCertDir              string
	healthAddr string

	controlPlaneConcurrency int
	clusterConcurrency      int
	machineConcurrency      int
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(infrav1.AddToScheme(scheme))
	utilruntime.Must(controlplanev1.AddToScheme(scheme))
	utilruntime.Must(bootstrapv1.AddToScheme(scheme))
	utilruntime.Must(clusterv1.AddToScheme(scheme))
	utilruntime.Must(expclusterv1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

// initFlags initializes the flags.
func initFlags(fs *pflag.FlagSet) {
	fs.StringVar(&metricsBindAddr, "metrics-bind-addr", ":8080",
		"The address the metric endpoint binds to.")

	fs.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")

	fs.DurationVar(&leaderElectionLeaseDuration, "leader-elect-lease-duration", consts.DefaultLeaderElectLeaseDuration,
		"Interval at which non-leader candidates will wait to force acquire leadership (duration string)")

	fs.DurationVar(&leaderElectionRenewDeadline, "leader-elect-renew-deadline", consts.DefaultLeaderElectRenewDeadline,
		"Duration that the leading controller manager will retry refreshing leadership before giving up (duration string)")

	fs.DurationVar(&leaderElectionRetryPeriod, "leader-elect-retry-period", consts.DefaultLeaderElectRetryPeriod,
		"Duration the LeaderElector clients should wait between tries of actions (duration string)")

	fs.StringVar(&watchFilterValue, "watch-filter", "",
		fmt.Sprintf("Label value that the controller watches to reconcile cluster-api objects. Label key is always %s. If unspecified, the controller watches for all cluster-api objects.", clusterv1.WatchLabel)) //nolint:lll

	fs.StringVar(&profilerAddress, "profiler-address", "",
		"Bind address to expose the pprof profiler (e.g. localhost:6060)")

	fs.IntVar(&controlPlaneConcurrency, "controlplane-concurrency", 1,
		"Number of control plane resources to process simultaneously")

	fs.IntVar(&clusterConcurrency, "cluster-concurrency", 1,
		"Number of cluster resources to process simultaneously")

	fs.IntVar(&clusterConcurrency, "machine-concurrency", 1,
		"Number of machine resources to process simultaneously")

	fs.DurationVar(&syncPeriod, "sync-period", consts.DefaultSyncPeriod,
		"The minimum interval at which watched resources are reconciled (e.g. 15m)")

	// fs.IntVar(&webhookPort, "webhook-port", consts.DefaultWebhookPort, "Webhook Server port")

	// fs.StringVar(&webhookCertDir, "webhook-cert-dir", "/tmp/k8s-webhook-server/serving-certs/",
	// 	"Webhook cert dir, only used when webhook-port is specified.")

	fs.StringVar(&healthAddr, "health-addr", ":9440",
		"The address the health endpoint binds to.")
}

func main() {
	initFlags(pflag.CommandLine)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()

	ctrl.SetLogger(klogr.New())
	ctx := ctrl.SetupSignalHandler()

	if profilerAddress != "" {
		klog.Infof("Profiler listening for requests at %s", profilerAddress)

		go func() {
			klog.Info(http.ListenAndServe(profilerAddress, nil))
		}()
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsBindAddr,
		LeaderElection:     enableLeaderElection,
		LeaderElectionID:   "kwok-provider.cluster.x-k8s.io",
		LeaseDuration:      &leaderElectionLeaseDuration,
		RenewDeadline:      &leaderElectionRenewDeadline,
		RetryPeriod:        &leaderElectionRetryPeriod,
		SyncPeriod:         &syncPeriod,
		ClientDisableCacheFor: []client.Object{
			&corev1.Secret{},
			&corev1.ConfigMap{},
		},
		HealthProbeBindAddress: healthAddr,
		//Port
		//CertDir
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	setupProbes(mgr)
	setupReconcilers(ctx, mgr)
	//setupWebhooks(mgr)

	setupLog.Info("starting manager")

	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func setupProbes(mgr ctrl.Manager) {
	//TODO: change to webhook ports when we have them
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}
}

func setupReconcilers(ctx context.Context, mgr ctrl.Manager) {
	if err := (&infracontroller.KwokClusterReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(ctx, mgr, controller.Options{MaxConcurrentReconciles: clusterConcurrency, RecoverPanic: pointer.Bool(true)}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "KwokCluster")
		os.Exit(1)
	}
	//if err := (&infracontroller.KwokMachineReconciler{
	// 	Client: mgr.GetClient(),
	// 	Scheme: mgr.GetScheme(),
	// }).SetupWithManager(mgr); err != nil {
	// 	setupLog.Error(err, "unable to create controller", "controller", "KwokMachine")
	// 	os.Exit(1)
	// }
	if err := (&controlplanecontroller.KwokControlPlaneReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(ctx, mgr, controller.Options{MaxConcurrentReconciles: controlPlaneConcurrency, RecoverPanic: pointer.Bool(true)}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "KwokControlPlane")
		os.Exit(1)
	}
	// if err := (&bootstrapcontroller.KwokConfigReconciler{
	// 	Client: mgr.GetClient(),
	// 	Scheme: mgr.GetScheme(),
	// }).SetupWithManager(mgr); err != nil {
	// 	setupLog.Error(err, "unable to create controller", "controller", "KwokConfig")
	// 	os.Exit(1)
	// }
}
