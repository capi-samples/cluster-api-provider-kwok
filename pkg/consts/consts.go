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

package consts

import (
	"time"
)

const (
	// DefaultLeaderElectLeaseDuration is the default duration that non-leader candidates will
	// wait to force acquire leadership.
	DefaultLeaderElectLeaseDuration = 15 * time.Second

	// DefaultLeaderElectRenewDeadline is the default duration that the acting master will retry
	// refreshing leadership before giving up.
	DefaultLeaderElectRenewDeadline = 10 * time.Second

	// DefaultLeaderElectRetryPeriod is the default duration the LeaderElector clients should wait
	// between tries of actions.
	DefaultLeaderElectRetryPeriod = 2 * time.Second

	// DefaultWebhookPort is the default port that the webhook server serves at.
	DefaultWebhookPort = 9443

	// DefaultSyncPeriod is the default resync period for the controller manager's cache.
	DefaultSyncPeriod = 10 * time.Minute
)
