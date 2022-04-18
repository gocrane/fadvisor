package datasource

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/gocrane/crane/pkg/common"
	"github.com/gocrane/fadvisor/pkg/metricnaming"
)

var _ RealTime = &RealTimeDataProxy{}

type RealTimeDataProxy struct {
	sync.Mutex
	realtimeProviders map[DataSourceType]RealTime
}

// NewRealTimeDataProxy return a proxy for all realtime datasource-providers, now it has no selecting policy configurable.
// Default policy is traversing all datasource-providers one by one until no error return.
func NewRealTimeDataProxy(realtimeProviders map[DataSourceType]RealTime) *RealTimeDataProxy {
	return &RealTimeDataProxy{
		realtimeProviders: realtimeProviders,
	}
}

func (r *RealTimeDataProxy) QueryLatestTimeSeries(ctx context.Context, metricNamer metricnaming.MetricNamer) ([]*common.TimeSeries, error) {
	var errs []error
	for _, provider := range r.getSortedProviders() {
		res, err := provider.QueryLatestTimeSeries(ctx, metricNamer)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		return res, nil
	}
	return nil, fmt.Errorf("no realtime data source is available now, errs: %+v", errs)
}

func (r *RealTimeDataProxy) RegisterRealTimeProvider(name DataSourceType, provider RealTime) {
	r.Lock()
	defer r.Unlock()
	r.realtimeProviders[name] = provider
}

func (r *RealTimeDataProxy) DeleteRealTimeProvider(name DataSourceType) {
	r.Lock()
	defer r.Unlock()
	delete(r.realtimeProviders, name)
}

func (r *RealTimeDataProxy) getSortedProviders() []RealTime {
	r.Lock()
	defer r.Unlock()
	var names []string
	var providers []RealTime
	for name := range r.realtimeProviders {
		names = append(names, string(name))
	}
	sort.Strings(names)
	for _, name := range names {
		providers = append(providers, r.realtimeProviders[DataSourceType(name)])
	}
	return providers
}

var _ History = &HistoryDataProxy{}

type HistoryDataProxy struct {
	sync.Mutex
	historyProviders map[DataSourceType]History
}

// NewHistoryDataProxy return a proxy for all history datasource-providers, now it has no selecting policy configurable.
// Default policy is traversing all datasource-providers one by one until no error return.
func NewHistoryDataProxy(historyProviders map[DataSourceType]History) *HistoryDataProxy {
	return &HistoryDataProxy{
		historyProviders: historyProviders,
	}
}

func (h *HistoryDataProxy) QueryTimeSeries(ctx context.Context, metricNamer metricnaming.MetricNamer, startTime time.Time, endTime time.Time, step time.Duration) ([]*common.TimeSeries, error) {
	var errs []error
	for _, provider := range h.getSortedProviders() {
		res, err := provider.QueryTimeSeries(ctx, metricNamer, startTime, endTime, step)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		return res, nil
	}
	return nil, fmt.Errorf("no history data source is available now, errs: %+v", errs)
}

func (h *HistoryDataProxy) RegisterHistoryProvider(name DataSourceType, provider History) {
	h.Lock()
	defer h.Unlock()
	if h.historyProviders == nil {
		h.historyProviders = make(map[DataSourceType]History)
	}
	h.historyProviders[name] = provider
}

func (h *HistoryDataProxy) DeleteHistoryProvider(name DataSourceType) {
	h.Lock()
	defer h.Unlock()
	delete(h.historyProviders, name)
}

func (h *HistoryDataProxy) getSortedProviders() []History {
	h.Lock()
	defer h.Unlock()
	var names []string
	var providers []History
	for name := range h.historyProviders {
		names = append(names, string(name))
	}
	sort.Strings(names)
	for _, name := range names {
		providers = append(providers, h.historyProviders[DataSourceType(name)])
	}
	return providers
}
