package cluster

import (
	"github.com/capi-samples/cluster-api-provider-kwok/pkg/scope"
)

type Service struct {
	scope *scope.ControlPlaneScope
}

func NewService(scope *scope.ControlPlaneScope) *Service {
	return &Service{
		scope: scope,
	}
}
