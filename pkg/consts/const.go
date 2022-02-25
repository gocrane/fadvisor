package consts

import "time"

const (
	CraneNamespace = "crane-system"

	CostExporterName = "fadvisor"

	// DefaultLeaseDuration is the default LeaseDuration for leader election.
	DefaultLeaseDuration = 15 * time.Second
	// DefaultRenewDeadline is the default RenewDeadline for leader election.
	DefaultRenewDeadline = 10 * time.Second
	// DefaultRetryPeriod is the default RetryPeriod for leader election.
	DefaultRetryPeriod = 5 * time.Second
)

const (
	GB = 1024 * 1024 * 1024
)
