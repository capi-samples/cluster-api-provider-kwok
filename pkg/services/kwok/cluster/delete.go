package cluster

import (
	"context"
	"errors"
	"os"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/kwok/pkg/kwokctl/runtime"
)

func (s *Service) Delete(ctx context.Context) (ctrl.Result, error) {
	logger := s.scope.Logger
	logger.Info("Reconciling KwokControlPlane delete")

	rt, err := runtime.DefaultRegistry.Load(ctx, s.scope.Name(), s.scope.WorkDir())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			logger.V(2).Info("Cluster does not exists, no action")

			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	logger.Info("Cluster is stopping")
	start := time.Now()
	err = rt.Down(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}
	logger.Info("Cluster is stopped",
		"elapsed", time.Since(start),
	)

	start = time.Now()
	logger.Info("Cluster is deleting")
	err = rt.Uninstall(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}
	logger.Info("Cluster is deleted",
		"elapsed", time.Since(start),
	)

	return ctrl.Result{}, nil
}
