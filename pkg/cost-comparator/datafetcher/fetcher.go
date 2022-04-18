package datafetcher

import (
	"context"
	"time"

	"github.com/gocrane/crane/pkg/common"
)

// Fetcher fetch data from datasource and other cluster data, it adapt the different data source
// You can implement your Fetcher.
type Fetcher interface {
	MetricFetcher
}

// MetricFetcher fetches data from datasource metric, it adapt the different data source.
type MetricFetcher interface {
	// ContainerCPUUsed return container cpu used metric, result is a map, key is namespace/podname/containername
	ContainerCPUUsed(ctx context.Context, start time.Time, end time.Time, step time.Duration) (map[string]*common.TimeSeries, error)
	// ContainerRAMUsed return container ram used metric unit bytes, result is a map, key is namespace/podname/containername
	ContainerRAMUsed(ctx context.Context, start time.Time, end time.Time, step time.Duration) (map[string]*common.TimeSeries, error)

	// PodCPUUsed return pod cpu used metric, result is a map, key is namespace/podname
	PodCPUUsed(ctx context.Context, start time.Time, end time.Time, step time.Duration) (map[string]*common.TimeSeries, error)
	// PodRAMUsed return ram used metric unit bytes, result is a map, key is namespace/podname
	PodRAMUsed(ctx context.Context, start time.Time, end time.Time, step time.Duration) (map[string]*common.TimeSeries, error)

	// WorkloadCPUUsed return workload cpu used metric for, result is a map, key is namespace/workloadkind/workloadname
	WorkloadCPUUsed(ctx context.Context, start time.Time, end time.Time, step time.Duration) (map[string]*common.TimeSeries, error)
	// WorkloadRAMUsed return workload ram used metric unit bytes, result is a map, key is namespace/workloadkind/workloadname
	WorkloadRAMUsed(ctx context.Context, start time.Time, end time.Time, step time.Duration) (map[string]*common.TimeSeries, error)

	// NodeCPUUsed return node cpu used metric, result is a map, key is nodename
	NodeCPUUsed(ctx context.Context, start time.Time, end time.Time, step time.Duration) (map[string]*common.TimeSeries, error)
	// NodeRAMUsed return node ram used metric unit bytes, result is a map, key is nodename
	NodeRAMUsed(ctx context.Context, start time.Time, end time.Time, step time.Duration) (map[string]*common.TimeSeries, error)
}
