package cluster

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/cluster-api/util/kubeconfig"
	"sigs.k8s.io/cluster-api/util/record"
	"sigs.k8s.io/cluster-api/util/secret"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/kwok/pkg/config"
	"sigs.k8s.io/kwok/pkg/kwokctl/runtime"
	"sigs.k8s.io/kwok/pkg/utils/format"
)

func (s *Service) Reconcile(ctx context.Context) (ctrl.Result, error) {
	logger := s.scope.Logger
	logger.Info("Reconciling KwokControlPlane")

	kwokctlConfiguration := config.GetKwokctlConfiguration(ctx)

	buildRuntime, ok := runtime.DefaultRegistry.Get(s.scope.Runtime())
	if !ok {
		return ctrl.Result{}, fmt.Errorf("runtime %q not found", s.scope.Runtime())
	}
	rt, err := buildRuntime(s.scope.Name(), s.scope.WorkDir())
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("runtime %v not available: %w", s.scope.Runtime(), err)
	}

	_, err = rt.Config(ctx)
	if err == nil {
		logger.Info("Cluster already exists")

		ready, err := rt.Ready(ctx)
		if err != nil {
			logger.Error(err, "Failed to check cluster status")
			return ctrl.Result{}, err
		}
		if ready {
			logger.Info("Cluster is already ready")
			return ctrl.Result{}, nil
		}
	} else {
		start := time.Now()
		logger.Info("Cluster is creating")

		err = rt.SetConfig(ctx, kwokctlConfiguration)
		if err != nil {
			logger.Error(err, "Failed to set config")
			return ctrl.Result{}, err
		}
		err = rt.Save(ctx)
		if err != nil {
			logger.Error(err, "Failed to save config", err)
			return ctrl.Result{}, err
		}

		err = rt.Install(ctx)
		if err != nil {
			logger.Error(err, "Failed to setup config")
			return ctrl.Result{}, err
		}
		logger.Info("Cluster is created",
			"elapsed", time.Since(start),
		)
	}

	if err := s.reconcileKubeconfig(ctx, rt); err != nil {
		return ctrl.Result{}, fmt.Errorf("reconciling kubeconfig: %w", err)
	}

	start := time.Now()
	logger.Info("Cluster is starting")
	err = rt.Up(ctx)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to start cluster %q: %w", s.scope.Name(), err)
	}
	logger.Info("Cluster is started",
		"elapsed", time.Since(start),
	)

	s.scope.ControlPlane.Status.Initialized = true

	return ctrl.Result{}, nil
}

func (s *Service) reconcileKubeconfig(ctx context.Context, rt runtime.Runtime) error {
	logger := s.scope.Logger

	logger.Info("Reconciling kubeconfig for cluster", "cluster", s.scope.Name())

	clusterRef := types.NamespacedName{
		Name:      s.scope.Cluster.Name,
		Namespace: s.scope.Cluster.Namespace,
	}

	configSecret, err := secret.GetFromNamespacedName(ctx, s.scope.Client, clusterRef, secret.Kubeconfig)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrap(err, "failed to get kubeconfig secret")
		}

		if createErr := s.createKubeconfigSecret(ctx, &clusterRef, rt); createErr != nil {
			return fmt.Errorf("creating kubeconfig secret: %w", err)
		}
	} else {
		logger.V(2).Info("kubeconfig secret already exists", "name", configSecret.Name, "namespace", configSecret.Namespace)
	}

	return nil
}

func (s *Service) createKubeconfigSecret(ctx context.Context, clusterRef *types.NamespacedName, rt runtime.Runtime) error {
	config, err := rt.Config(ctx)
	if err != nil {
		return fmt.Errorf("getting kwock runtime config: %w", err)
	}
	conf := &config.Options

	controllerOwnerRef := *metav1.NewControllerRef(s.scope.ControlPlane, s.scope.Cluster.Spec.ControlPlaneRef.GroupVersionKind())

	clusterName := s.scope.Name()
	userName := fmt.Sprintf("%s-capf-admin", clusterName)
	contextName := fmt.Sprintf("%s@%s", userName, clusterName)

	//TODO: do we need to allow changing to https
	scheme := "http"
	address := s.scope.ClusterAddress()

	//pkiPath := filepath.Join(s.scope.WorkDir(), runtime.PkiName)
	//adminKeyPath := path.Join(pkiPath, "admin.key")
	//adminCertPath := path.Join(pkiPath, "admin.crt")

	cfg := &api.Config{
		APIVersion: api.SchemeGroupVersion.Version,
		Clusters: map[string]*api.Cluster{
			clusterName: {
				Server: scheme + "://" + address + ":" + format.String(conf.KubeApiserverPort),
			},
		},
		Contexts: map[string]*api.Context{
			contextName: {
				Cluster:  clusterName,
				AuthInfo: userName,
			},
		},
		CurrentContext: contextName,
	}

	out, err := clientcmd.Write(*cfg)
	if err != nil {
		return errors.Wrap(err, "failed to serialize config to yaml")
	}

	kubeconfigSecret := kubeconfig.GenerateSecretWithOwner(*clusterRef, out, controllerOwnerRef)
	if err := s.scope.Client.Create(ctx, kubeconfigSecret); err != nil {
		return errors.Wrap(err, "failed to create kubeconfig secret")
	}

	record.Eventf(s.scope.ControlPlane, "SucessfulCreateKubeconfig", "Created kubeconfig for cluster %q", s.scope.Name())
	return nil
}
