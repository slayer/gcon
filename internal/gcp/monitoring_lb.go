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

// lbFilter builds a Cloud Monitoring filter scoped to a single HTTP(S)
// load balancer keyed by forwarding-rule name. The filter matches both
// external (https_lb_rule) and internal (internal_http_lb_rule) resource
// types so external HTTPS, internal HTTPS, and external HTTP load
// balancers all resolve through a single helper.
func lbFilter(forwardingRuleName, metricType string) string {
	return fmt.Sprintf( //nolint:gocritic // GCP filter syntax requires double quotes
		`(resource.type = "https_lb_rule" OR resource.type = "internal_http_lb_rule") AND resource.labels.forwarding_rule_name = "%s" AND metric.type = "%s"`,
		forwardingRuleName, metricType,
	)
}

// lbFilterWithLabel narrows lbFilter by a single metric label (e.g. a
// response_code_class label for error breakdowns).
func lbFilterWithLabel(forwardingRuleName, metricType, labelKey, labelValue string) string {
	return fmt.Sprintf( //nolint:gocritic // GCP filter syntax requires double quotes
		`%s AND metric.labels.%s = "%s"`,
		lbFilter(forwardingRuleName, metricType), labelKey, labelValue,
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
