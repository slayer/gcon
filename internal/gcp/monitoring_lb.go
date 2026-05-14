package gcp

import (
	"context"
	"fmt"
	"sort"
	"time"

	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// LBMetrics aggregates the time-series data the Observability tab renders
// for a single HTTP(S) load balancer.
type LBMetrics struct {
	RequestCount    []DataPoint // requests/sec
	RequestCount4xx []DataPoint // raw 4xx rate (requests/sec)
	RequestCount5xx []DataPoint // raw 5xx rate (requests/sec)
	Latency50       []DataPoint // total latency p50 (ms)
	Latency95       []DataPoint // total latency p95 (ms)
	Latency99       []DataPoint // total latency p99 (ms)
	BackendLat50    []DataPoint // backend latency p50 (ms)
	BackendLat95    []DataPoint // backend latency p95 (ms)
	BackendLat99    []DataPoint // backend latency p99 (ms)
	RequestBytes    []DataPoint // bytes/sec
	ResponseBytes   []DataPoint // bytes/sec
	LastFetch       time.Time
}

// lbFilter builds a Cloud Monitoring filter scoped to a single LB. The
// resourceType must be a single Cloud Monitoring resource string (e.g.
// "https_lb_rule" for global external Application LBs or
// "internal_http_lb_rule" for internal Application LBs). GCP's filter
// language explicitly disallows `OR` between resource.type values
// ("Within the 'resource' prefix, OR can only be used to connect a list
// of 'labels' restrictions"), so callers must pick exactly one resource
// type per fetch.
func lbFilter(resourceType, forwardingRuleName, metricType string) string {
	return fmt.Sprintf( //nolint:gocritic // GCP filter syntax requires double quotes
		`resource.type = "%s" AND resource.labels.forwarding_rule_name = "%s" AND metric.type = "%s"`,
		resourceType, forwardingRuleName, metricType,
	)
}

// lbFilterWithIntLabel narrows lbFilter by a single integer-typed metric
// label. The LB metric "response_code_class" is typed as an integer
// (4, 5, …), not a string ("4xx", "5xx") — Cloud Monitoring rejects
// quoted values with: "The label is typed as an integer, but the
// supplied value cannot be parsed as an integer."
func lbFilterWithIntLabel(resourceType, forwardingRuleName, metricType, labelKey string, labelValue int) string {
	return fmt.Sprintf( //nolint:gocritic // GCP filter syntax requires double quotes
		`%s AND metric.labels.%s = %d`,
		lbFilter(resourceType, forwardingRuleName, metricType), labelKey, labelValue,
	)
}

// fetchLBMetric is the rate/sum/mean fetcher mirrored from fetchCloudRunMetric.
func (c *MonitoringClient) fetchLBMetric(ctx context.Context, filter string, duration time.Duration, aligner monitoringpb.Aggregation_Aligner, reducer monitoringpb.Aggregation_Reducer) ([]DataPoint, error) {
	endTime := time.Now()
	startTime := endTime.Add(-duration)

	req := &monitoringpb.ListTimeSeriesRequest{
		Name:   fmt.Sprintf("projects/%s", c.projectID),
		Filter: filter,
		Interval: &monitoringpb.TimeInterval{
			StartTime: timestamppb.New(startTime),
			EndTime:   timestamppb.New(endTime),
		},
		Aggregation: &monitoringpb.Aggregation{
			AlignmentPeriod:    durationpb.New(60 * time.Second),
			PerSeriesAligner:   aligner,
			CrossSeriesReducer: reducer,
		},
	}

	points, err := c.collectDataPoints(ctx, req)
	if err != nil {
		return nil, err
	}
	sort.Slice(points, func(i, j int) bool {
		return points[i].Timestamp.Before(points[j].Timestamp)
	})
	return points, nil
}

const lbMetricRequestCount = "loadbalancing.googleapis.com/https/request_count"

// GetLBRequestCount fetches per-second request count for an HTTP(S) LB
// keyed by forwarding-rule name. ALIGN_RATE + REDUCE_SUM aggregate across
// all per-backend time series into one rate-of-requests series.
func (c *MonitoringClient) GetLBRequestCount(ctx context.Context, resourceType, forwardingRuleName string, duration time.Duration) ([]DataPoint, error) {
	filter := lbFilter(resourceType, forwardingRuleName, lbMetricRequestCount)
	return c.fetchLBMetric(ctx, filter, duration, monitoringpb.Aggregation_ALIGN_RATE, monitoringpb.Aggregation_REDUCE_SUM)
}

// GetLBRequestCountByCodeClass narrows request_count by the integer
// response_code_class label (2 = 2xx, 3 = 3xx, 4 = 4xx, 5 = 5xx). Used
// to compute error rate.
func (c *MonitoringClient) GetLBRequestCountByCodeClass(ctx context.Context, resourceType, forwardingRuleName string, codeClass int, duration time.Duration) ([]DataPoint, error) {
	filter := lbFilterWithIntLabel(resourceType, forwardingRuleName, lbMetricRequestCount, "response_code_class", codeClass)
	return c.fetchLBMetric(ctx, filter, duration, monitoringpb.Aggregation_ALIGN_RATE, monitoringpb.Aggregation_REDUCE_SUM)
}

const lbMetricRequestLatencies = "loadbalancing.googleapis.com/https/total_latencies"

// GetLBRequestLatencies fetches p50, p95, and p99 of total request latency
// for the LB. Total latency = backend latency + LB overhead. Values are
// returned in milliseconds (the GCP metric is already in ms — no scale).
func (c *MonitoringClient) GetLBRequestLatencies(ctx context.Context, resourceType, forwardingRuleName string, duration time.Duration) (p50, p95, p99 []DataPoint, err error) {
	filter := lbFilter(resourceType, forwardingRuleName, lbMetricRequestLatencies)

	p50, err = c.fetchLBPercentile(ctx, filter, duration, monitoringpb.Aggregation_ALIGN_PERCENTILE_50)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("fetch p50 total latency: %w", err)
	}
	p95, err = c.fetchLBPercentile(ctx, filter, duration, monitoringpb.Aggregation_ALIGN_PERCENTILE_95)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("fetch p95 total latency: %w", err)
	}
	p99, err = c.fetchLBPercentile(ctx, filter, duration, monitoringpb.Aggregation_ALIGN_PERCENTILE_99)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("fetch p99 total latency: %w", err)
	}
	return p50, p95, p99, nil
}

const lbMetricBackendLatencies = "loadbalancing.googleapis.com/https/backend_latencies"

// GetLBBackendLatencies fetches p50, p95, and p99 of backend-only latency
// (origin response time, excludes LB-introduced overhead). Values in ms.
func (c *MonitoringClient) GetLBBackendLatencies(ctx context.Context, resourceType, forwardingRuleName string, duration time.Duration) (p50, p95, p99 []DataPoint, err error) {
	filter := lbFilter(resourceType, forwardingRuleName, lbMetricBackendLatencies)

	p50, err = c.fetchLBPercentile(ctx, filter, duration, monitoringpb.Aggregation_ALIGN_PERCENTILE_50)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("fetch p50 backend latency: %w", err)
	}
	p95, err = c.fetchLBPercentile(ctx, filter, duration, monitoringpb.Aggregation_ALIGN_PERCENTILE_95)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("fetch p95 backend latency: %w", err)
	}
	p99, err = c.fetchLBPercentile(ctx, filter, duration, monitoringpb.Aggregation_ALIGN_PERCENTILE_99)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("fetch p99 backend latency: %w", err)
	}
	return p50, p95, p99, nil
}

const (
	lbMetricRequestBytes  = "loadbalancing.googleapis.com/https/request_bytes_count"
	lbMetricResponseBytes = "loadbalancing.googleapis.com/https/response_bytes_count"
)

// GetLBRequestBytes returns request-bytes/sec rate-aligned series.
func (c *MonitoringClient) GetLBRequestBytes(ctx context.Context, resourceType, forwardingRuleName string, duration time.Duration) ([]DataPoint, error) {
	filter := lbFilter(resourceType, forwardingRuleName, lbMetricRequestBytes)
	return c.fetchLBMetric(ctx, filter, duration, monitoringpb.Aggregation_ALIGN_RATE, monitoringpb.Aggregation_REDUCE_SUM)
}

// GetLBResponseBytes returns response-bytes/sec rate-aligned series.
func (c *MonitoringClient) GetLBResponseBytes(ctx context.Context, resourceType, forwardingRuleName string, duration time.Duration) ([]DataPoint, error) {
	filter := lbFilter(resourceType, forwardingRuleName, lbMetricResponseBytes)
	return c.fetchLBMetric(ctx, filter, duration, monitoringpb.Aggregation_ALIGN_RATE, monitoringpb.Aggregation_REDUCE_SUM)
}

// fetchLBPercentile is the percentile aligner variant for distribution metrics.
func (c *MonitoringClient) fetchLBPercentile(ctx context.Context, filter string, duration time.Duration, aligner monitoringpb.Aggregation_Aligner) ([]DataPoint, error) {
	endTime := time.Now()
	startTime := endTime.Add(-duration)

	req := &monitoringpb.ListTimeSeriesRequest{
		Name:   fmt.Sprintf("projects/%s", c.projectID),
		Filter: filter,
		Interval: &monitoringpb.TimeInterval{
			StartTime: timestamppb.New(startTime),
			EndTime:   timestamppb.New(endTime),
		},
		Aggregation: &monitoringpb.Aggregation{
			AlignmentPeriod:    durationpb.New(60 * time.Second),
			PerSeriesAligner:   aligner,
			CrossSeriesReducer: monitoringpb.Aggregation_REDUCE_MEAN,
		},
	}

	points, err := c.collectDataPoints(ctx, req)
	if err != nil {
		return nil, err
	}
	sort.Slice(points, func(i, j int) bool {
		return points[i].Timestamp.Before(points[j].Timestamp)
	})
	return points, nil
}
