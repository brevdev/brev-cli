package ids

import "github.com/brevdev/dev-plane/pkg/prefixid"

type (
	CloudCredID             prefixid.PrefixID
	InstanceID              prefixid.PrefixID
	CloudProviderInstanceID prefixid.PrefixID
	CloudProviderID         string
)

type HealthCheckID prefixid.PrefixID

type CreditID prefixid.PrefixID

type LimitID prefixid.PrefixID
