package services

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
)

type Reconciler interface {
	Reconcile(ctx context.Context) error
	Delete(ctx context.Context) error
}

// ReconcilerWithResult is a generic interface used by components offering a type of service.
type ReconcilerWithResult interface {
	Reconcile(ctx context.Context) (ctrl.Result, error)
	Delete(ctx context.Context) (ctrl.Result, error)
}
